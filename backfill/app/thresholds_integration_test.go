//go:build integration

package app

import (
	"context"
	"fmt"
	"log/slog"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// Neither set of thresholds below is the default, so a run deciding under the default thresholds
// rather than the ones it is given decides differently from both. Under narrowGate a list sender of
// more than two messages with no prior hit is skipped. widenedGate raises the high-volume mark to five,
// so the news sender's four list messages are scanned and the promotions sender's eight stay skipped.
var (
	narrowGate  = scangate.Config{NoReplyLocalParts: []string{"noreply"}, SmallBytes: 1, RecentAgeMillis: 1, LowVolume: 1, HighVolume: 2}
	widenedGate = scangate.Config{NoReplyLocalParts: []string{"noreply"}, SmallBytes: 1, RecentAgeMillis: 1, LowVolume: 1, HighVolume: 5}
)

// thresholdMailbox returns the account's mailbox, list mail only. The news sender sends four messages,
// the last with a code in its body, and the promotions sender eight clean ones. No subject carries
// anything the scanner masks, so no skip is decided again for its subject's signal.
func thresholdMailbox(t *testing.T, account string) *fake.Fake {
	t.Helper()
	var out []fake.Message
	add := func(id, from, list, body string) {
		n := len(out)
		out = append(out, fake.Message{
			Metadata: mail.MessageMetadata{
				ID: id, ThreadID: "t" + id, From: mail.Address{Email: from}, Subject: marker.Field("subject" + letters(id)),
				Date: mail.UnixMilli(1_700_000_000_000 + int64(n)*60_000), SizeBytes: 4096, ListID: list,
			},
			Body: mail.MessageBody{Text: body},
		})
	}
	for i := 1; i <= 4; i++ {
		body := marker.Body("news"+string(rune('a'+i))) + " Thanks for reading."
		if i == 4 {
			body += " " + codeText + "."
		}
		add(fmt.Sprintf("n%d", i), "letters@news.example", "list.news.example", body)
	}
	for i := 1; i <= 8; i++ {
		add(fmt.Sprintf("p%d", i), "deals@promo.example", "list.promo.example", marker.Body("promo"+string(rune('a'+i)))+" Thanks.")
	}
	f, err := fake.New(fake.Config{Account: account, PageSize: 4}, out...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// letters returns id with each digit spelt as a letter, since a marker tag holds lowercase letters
// only (ADR-0044).
func letters(id string) string {
	out := []rune(id)
	for i, r := range out {
		if r >= '0' && r <= '9' {
			out[i] = 'a' + r - '0'
		}
	}
	return string(out)
}

// thresholdDeps returns both passes' dependencies over the fake with the default scanner, the gate's
// thresholds gate and a policy listing each of restricted. Run identifiers start with run.
func thresholdDeps(t *testing.T, pool *pgxpool.Pool, account string, f *fake.Fake, gate scangate.Config, run string, restricted ...string) (pass1.Deps, pass2.Deps) {
	t.Helper()
	var rows []policy.Row
	for _, d := range restricted {
		rows = append(rows, policy.Row{ID: "rule." + d, Class: policy.Restricted, DomainSuffixes: []string{d}})
	}
	rules, err := policy.Load(rows)
	if err != nil {
		t.Fatal(err)
	}
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	n := 0
	runID := func() string { n++; return fmt.Sprintf("%s%d", run, n) }
	s := wideScanner(t)
	first := pass1.Deps{
		Store: pass1.NewPostgres(pool), Fetch: f.EnumerateAll, Policy: rules.For(account), Scanner: s, Lookups: lookups,
		RunID: runID, Now: time.Now,
	}
	second := pass2.Deps{
		Store: pass2.NewPostgres(pool), Body: f.GetMessageBody, Policy: rules.For(account), Lookups: lookups,
		Gate: gate, Scanner: s, PageSize: 2, RunID: runID, Now: time.Now,
	}
	return first, second
}

// runThresholds runs both passes over the account as one backfill run does, under the gate's
// thresholds gate and a policy listing each of restricted.
func runThresholds(t *testing.T, pool *pgxpool.Pool, account string, f *fake.Fake, gate scangate.Config, run string, restricted ...string) {
	t.Helper()
	first, second := thresholdDeps(t, pool, account, f, gate, run, restricted...)
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	metrics2, err := pass2.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	none := func(context.Context, string) error { return nil }
	if err := backfillAccount(t.Context(), first, second, account, metrics1, metrics2, none, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
}

// restartMark reads whether the account's second pass is marked to start over.
func restartMark(t *testing.T, conn *pgx.Conn, account string) bool {
	t.Helper()
	var mark bool
	if err := conn.QueryRow(t.Context(), "SELECT backfill_pass2_restart FROM account_state WHERE account_id = $1", account).Scan(&mark); err != nil {
		t.Fatal(err)
	}
	return mark
}

// runCount counts the account's recorded runs.
func runCount(t *testing.T, conn *pgx.Conn, account string) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM job_runs WHERE account_id = $1", account).Scan(&n); err != nil {
		t.Fatal(err)
	}
	return n
}

// wantStates checks that each message named is in the state given, with the gate decision and reason
// given.
func wantStates(t *testing.T, when string, rows map[string]rescanRow, ids []string, state, decision, reason string) {
	t.Helper()
	for _, id := range ids {
		if r := rows[id]; r.State != state || r.Decision != decision || r.Reason != reason {
			t.Errorf("%s, message %s is %+v, want %s with the decision %s for %s", when, id, r, state, decision, reason)
		}
	}
}

var (
	newsIDs  = []string{"n1", "n2", "n3", "n4"}
	promoIDs = []string{"p1", "p2", "p3", "p4", "p5", "p6", "p7", "p8"}
)

// VERIFICATIONS' row for a change of the scan gate's thresholds. Both passes run under thresholds
// that skip every list message and end, and a run under the same thresholds changes nothing and
// records no run. The thresholds are then widened so the gate scans the news sender's messages and
// still skips the promotions sender's. The next run, before its first pass, returns exactly the news
// skips to pending, keeping their recorded decision, and records the second pass as not ended and
// marked to start over. Its second pass then scans them and flags the code, and the promotions skips
// stand. A run after that under the widened thresholds changes nothing and records no run
// (ADR-0098).
func TestAChangeOfThresholdsDecidesTheStoredSkipsAgain(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := thresholdMailbox(t, "personal")
	runThresholds(t, pool, "personal", f, narrowGate, "narrow")

	before := rescanRows(t, conn, "personal")
	wantStates(t, "under the narrow thresholds", before, append(append([]string{}, newsIDs...), promoIDs...), "skipped_gate", "SKIP", "high_volume_no_hits")
	if first, second := completion(t, conn, "personal"); !first || !second {
		t.Fatalf("under the narrow thresholds the passes end as %v and %v, want both ended", first, second)
	}

	// A run under the thresholds the skips were made under decides each again as the same skip, so it
	// returns nothing to pending and reopens nothing.
	runs := runCount(t, conn, "personal")
	runThresholds(t, pool, "personal", f, narrowGate, "same")
	if got := runCount(t, conn, "personal"); got != runs {
		t.Errorf("a run under unchanged thresholds recorded %d runs, want none", got-runs)
	}
	if diff := cmp.Diff(before, rescanRows(t, conn, "personal"), compare.Options); diff != "" {
		t.Errorf("a run under unchanged thresholds changed the index (-before +after):\n%s", diff)
	}

	// The run under the widened thresholds, before its first pass, returns exactly the skips they no
	// longer make to pending and reopens the second pass, marked to start over.
	_, second := thresholdDeps(t, pool, "personal", f, widenedGate, "widened")
	reopened, err := pass2.Reopen(t.Context(), second, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if want := (pass2.Reopened{Skips: len(newsIDs)}); reopened != want {
		t.Errorf("the run-start step returned %+v to pending, want %+v", reopened, want)
	}
	rows := rescanRows(t, conn, "personal")
	wantStates(t, "before the first pass", rows, newsIDs, "pending", "SKIP", "high_volume_no_hits")
	wantStates(t, "before the first pass", rows, promoIDs, "skipped_gate", "SKIP", "high_volume_no_hits")
	if first, second := completion(t, conn, "personal"); !first || second {
		t.Errorf("after the run-start step the passes are recorded ended as %v and %v, want the second reopened", first, second)
	}
	if !restartMark(t, conn, "personal") {
		t.Errorf("the run-start step returned skips to pending without marking the second pass to start over")
	}

	runThresholds(t, pool, "personal", f, widenedGate, "widened")
	after := rescanRows(t, conn, "personal")
	wantStates(t, "under the widened thresholds", after, newsIDs, "scanned", "SCAN", "default")
	wantStates(t, "under the widened thresholds", after, promoIDs, "skipped_gate", "SKIP", "high_volume_no_hits")
	if r := after["n4"]; !cmp.Equal(r.Flags, []string{"mfa_code"}) {
		t.Errorf("the news code message is %+v, want flagged once scanned", r)
	}
	if first, second := completion(t, conn, "personal"); !first || !second {
		t.Errorf("under the widened thresholds the passes end as %v and %v, want both ended", first, second)
	}

	// A run after that under the same thresholds finds every skip standing and does nothing.
	runs = runCount(t, conn, "personal")
	runThresholds(t, pool, "personal", f, widenedGate, "again")
	if got := runCount(t, conn, "personal"); got != runs {
		t.Errorf("a run under unchanged thresholds recorded %d runs, want none", got-runs)
	}
	if diff := cmp.Diff(after, rescanRows(t, conn, "personal"), compare.Options); diff != "" {
		t.Errorf("a run under unchanged thresholds changed the index (-before +after):\n%s", diff)
	}
}

// VERIFICATIONS' row for a change of the scan gate's thresholds, thresholds that cannot decide. Every
// stored skip returns to pending at the run's start, and the second pass, unable to decide, leaves
// each pending, so no body stays released under thresholds that decide nothing (ADR-0098, ADR-0093).
func TestThresholdsThatCannotDecideReturnEverySkipToPending(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := thresholdMailbox(t, "personal")
	runThresholds(t, pool, "personal", f, narrowGate, "narrow")

	runThresholds(t, pool, "personal", f, scangate.Config{}, "undecided")
	rows := rescanRows(t, conn, "personal")
	for _, id := range append(append([]string{}, newsIDs...), promoIDs...) {
		if r := rows[id]; r.State != "pending" {
			t.Errorf("under thresholds that cannot decide, message %s is %+v, want pending", id, r)
		}
	}
}

// VERIFICATIONS' row for a change of the scan gate's thresholds, a sender the policy now restricts.
// Its stored skips return to pending at the run's start, and the second pass records each as skipped
// as restricted, while the other sender's skips stand (ADR-0098, ADR-0008).
func TestAGateSkipOfASenderNowRestrictedIsSkippedAsRestricted(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := thresholdMailbox(t, "personal")
	runThresholds(t, pool, "personal", f, narrowGate, "narrow")

	runThresholds(t, pool, "personal", f, narrowGate, "listed", "promo.example")
	rows := rescanRows(t, conn, "personal")
	wantStates(t, "once the policy lists the promotions sender", rows, promoIDs, "skipped_restricted", "SKIP", "restricted")
	wantStates(t, "once the policy lists the promotions sender", rows, newsIDs, "skipped_gate", "SKIP", "high_volume_no_hits")
}

// VERIFICATIONS' row for a change of the scan gate's thresholds, a second pass stopped part way. The
// pass stops past the news skips, the thresholds are widened, and the run resuming it starts over from
// the first waiting message, so the skips returned to pending before its checkpoint are scanned
// rather than left waiting for good (ADR-0098, ADR-0096).
func TestASkipReturnedBeforeAStoppedPassesCheckpointIsScanned(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := thresholdMailbox(t, "personal")
	first, second := thresholdDeps(t, pool, "personal", f, narrowGate, "narrow")
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	none := func(context.Context, string) error { return nil }
	if err := passAccount(t.Context(), first, "personal", metrics1, none, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	p, err := pass2.Open(t.Context(), second, "personal")
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if s, err := p.Next(t.Context()); err != nil || s.Done {
			t.Fatalf("the second pass's page ended as %+v, %v, want a page made durable", s, err)
		}
	}
	if r := readRun(t, conn, p.Run()); r.Checkpoint != `{"page": 3, "after": "p2"}` {
		t.Fatalf("the stopped run's checkpoint is %s, want it past the news skips", r.Checkpoint)
	}

	runThresholds(t, pool, "personal", f, widenedGate, "widened")
	rows := rescanRows(t, conn, "personal")
	wantStates(t, "after the run resuming the stopped pass", rows, newsIDs, "scanned", "SCAN", "default")
	wantStates(t, "after the run resuming the stopped pass", rows, promoIDs, "skipped_gate", "SKIP", "high_volume_no_hits")
}

// VERIFICATIONS' row for a change of the scan gate's thresholds, a sender that has since gained a prior
// hit. The same comparison reads each skip's sender as it now stands, so under unchanged thresholds the
// next run returns that sender's skips to pending and its second pass scans them for the prior hit,
// while the other sender's skips stand (ADR-0098, ADR-0094).
func TestASkipWhoseSenderGainedAPriorHitIsScanned(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := thresholdMailbox(t, "personal")
	runThresholds(t, pool, "personal", f, narrowGate, "narrow")

	// A hit recorded for the news sender by another path, as a scan of its later mail records one.
	must(t, conn, "UPDATE senders SET scan_hit_count = 1 WHERE account_id = 'personal' AND domain = 'news.example'")
	runThresholds(t, pool, "personal", f, narrowGate, "hit")
	rows := rescanRows(t, conn, "personal")
	wantStates(t, "once the news sender has a prior hit", rows, newsIDs, "scanned", "SCAN", "prior_hit")
	wantStates(t, "once the news sender has a prior hit", rows, promoIDs, "skipped_gate", "SKIP", "high_volume_no_hits")
}
