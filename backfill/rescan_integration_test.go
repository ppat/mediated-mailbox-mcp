//go:build integration

package main

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
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// codeText is a one-time code that the default trigger words catch and the narrow ones below do not.
const codeText = "Your verification code is 419283"

// narrowScanner is a scanner whose only trigger word is otp, so it neither masks nor flags codeText,
// and wideScanner is the default scanner, which does both. Each is built under its own revision, as a
// change to the scanner's section would give it (ADR-0078).
func narrowScanner(t *testing.T) scan.Scanner {
	t.Helper()
	cfg := scan.DefaultConfig()
	cfg.Triggers = map[string][]string{"en": {"otp"}}
	s, err := scan.New(cfg, "narrow-revision")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func wideScanner(t *testing.T) scan.Scanner {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "wide-revision")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// rescanMailbox returns the account's mailbox. The news sender sends three list messages, which the
// lowered gate below skips for volume unless the subject is masked, the first of them with a code in
// its subject. The shop sends a code in a body, a clean body, and a message the test later removes
// from the provider. The bank, which the policy lists, sends a code in a subject. The login sender
// sends a code both scanners flag.
func rescanMailbox(t *testing.T, account string) *fake.Fake {
	t.Helper()
	messages := []struct {
		id, from, subject, body string
		list                    bool
	}{
		{"b1", "alerts@bank.example", marker.Field("bank") + " " + codeText, marker.Body("bank"), false},
		{"n1", "letters@news.example", marker.Field("newsone") + " " + codeText, marker.Body("newsone"), true},
		{"n2", "letters@news.example", marker.Field("newstwo"), marker.Body("newstwo"), true},
		{"n3", "letters@news.example", marker.Field("newsthree"), marker.Body("newsthree"), true},
		{"s1", "orders@shop.example", marker.Field("shopone"), marker.Body("shopone") + " " + codeText + ".", false},
		{"s2", "orders@shop.example", marker.Field("shoptwo"), marker.Body("shoptwo") + " Thanks.", false},
		{"s3", "orders@shop.example", marker.Field("shopthree") + " " + codeText, marker.Body("shopthree"), false},
		{"o1", "sign-in@login.example", marker.Field("login"), marker.Body("login") + " Your OTP is 419283.", false},
	}
	var out []fake.Message
	for i, m := range messages {
		md := mail.MessageMetadata{
			ID: m.id, ThreadID: "t" + m.id, From: mail.Address{Email: m.from}, Subject: m.subject,
			Date: mail.UnixMilli(1_700_000_000_000 + int64(i)*60_000), SizeBytes: 4096,
		}
		if m.list {
			md.ListID = "list.news.example"
		}
		out = append(out, fake.Message{Metadata: md, Body: mail.MessageBody{Text: m.body}})
	}
	f, err := fake.New(fake.Config{Account: account, PageSize: 2}, out...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// runBackfill runs both passes over the account with scanner s, as one backfill run does.
func runBackfill(t *testing.T, pool *pgxpool.Pool, account string, f *fake.Fake, s scan.Scanner, run string) {
	t.Helper()
	first, second := rescanDeps(t, pool, account, f, s, run)
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

// rescanDeps returns both passes' dependencies over the fake with scanner s, under a policy listing the
// bank and a gate lowered so a sender of more than two list messages with no prior hit is skipped. Run
// identifiers start with run.
func rescanDeps(t *testing.T, pool *pgxpool.Pool, account string, f *fake.Fake, s scan.Scanner, run string) (pass1.Deps, pass2.Deps) {
	t.Helper()
	rules, err := policy.Load([]policy.Row{{ID: "rule.bank", Class: policy.Restricted, DomainSuffixes: []string{"bank.example"}}})
	if err != nil {
		t.Fatal(err)
	}
	lookups := classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}
	n := 0
	runID := func() string { n++; return fmt.Sprintf("%s%d", run, n) }
	gate := scangate.Config{NoReplyLocalParts: []string{"noreply"}, SmallBytes: 1, RecentAgeMillis: 1, LowVolume: 1, HighVolume: 2}
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

// rescanRow is one message as a check reads it.
type rescanRow struct {
	Subject           string
	SubjectRevision   string
	State             string
	Flags             []string
	VerdictRevision   string
	Decision, Reason  string
	EventsUnderStamp  int
	WholeSubjectEvent bool
}

func rescanRows(t *testing.T, conn *pgx.Conn, account string) map[string]rescanRow {
	t.Helper()
	rows, err := conn.Query(t.Context(), `SELECT m.message_id, m.subject, coalesce(m.subject_scanner_revision, ''), m.scan_state, m.content_flags,
			coalesce(m.scanner_revision, ''), coalesce(d.decision, ''), coalesce(d.reason, ''),
			(SELECT count(*) FROM masking_events AS e WHERE e.account_id = m.account_id AND e.message_id = m.message_id
				AND e.scanner_version = m.subject_scanner_version AND e.scanner_revision = m.subject_scanner_revision),
			exists(SELECT 1 FROM masking_events AS e WHERE e.account_id = m.account_id AND e.message_id = m.message_id
				AND e.rule_id = 'mask.whole_subject' AND e.scanner_revision = m.subject_scanner_revision)
		FROM messages AS m LEFT JOIN scan_gate_decisions AS d ON d.account_id = m.account_id AND d.message_id = m.message_id
		WHERE m.account_id = $1`, account)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]rescanRow{}
	var id string
	var r rescanRow
	_, err = pgx.ForEachRow(rows, []any{
		&id, &r.Subject, &r.SubjectRevision, &r.State, &r.Flags, &r.VerdictRevision, &r.Decision, &r.Reason,
		&r.EventsUnderStamp, &r.WholeSubjectEvent,
	}, func() error {
		out[id] = r
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func completion(t *testing.T, conn *pgx.Conn, account string) (bool, bool) {
	t.Helper()
	var first, second bool
	if err := conn.QueryRow(t.Context(), "SELECT backfill_pass1_complete, backfill_pass2_complete FROM account_state WHERE account_id = $1", account).
		Scan(&first, &second); err != nil {
		t.Fatal(err)
	}
	return first, second
}

// VERIFICATIONS' row for a change of scanner, the operation that masks and scans again. Both passes
// run under a scanner whose narrow trigger words neither mask nor flag the code, and end. A message is
// then removed at the provider, new mail arrives, and the scanner is changed to the default one.
//
// The next run, before its first pass, returns every scanned message to pending with its verdict
// cleared and its sender's prior hits counted again, and reopens the second pass. Its first pass then
// fails, and the verdicts stay pending. The run after that returns nothing more, and part way through
// its first pass the verdicts are still pending. That first pass masks every stored subject again
// under the new scanner with its masking events, adds the new mail, and masks whole the subject of the
// message the provider no longer has. The second pass then returns the list message the gate skipped
// whose subject is now masked to pending, scans everything waiting under the new scanner, flags the
// codes, and counts each sender's hits once. A run after that finds nothing to do (ADR-0096).
func TestAChangeOfScannerMasksAndScansAgain(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := rescanMailbox(t, "personal")
	runBackfill(t, pool, "personal", f, narrowScanner(t), "narrow")

	before := rescanRows(t, conn, "personal")
	if before["s1"].State != "scanned" || len(before["s1"].Flags) != 0 || before["n1"].State != "skipped_gate" || before["n1"].Subject != marker.Field("newsone")+" "+codeText {
		t.Fatalf("under the narrow scanner the shop's code is %+v and the news code subject %+v, want scanned clean and skipped unmasked", before["s1"], before["n1"])
	}
	if err := f.Remove("s3"); err != nil {
		t.Fatal(err)
	}
	newMail := fake.Message{
		Metadata: mail.MessageMetadata{
			ID: "s4", ThreadID: "ts4", From: mail.Address{Email: "orders@shop.example"}, Subject: marker.Field("shopfour"),
			Date: mail.UnixMilli(1_700_001_000_000), SizeBytes: 4096,
		},
		Body: mail.MessageBody{Text: marker.Body("shopfour") + " " + codeText + "."},
	}
	if err := f.Deliver(newMail); err != nil {
		t.Fatal(err)
	}
	wide := wideScanner(t)
	first, second := rescanDeps(t, pool, "personal", f, wide, "wide")
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	metrics2, err := pass2.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	none := func(context.Context, string) error { return nil }

	// The next run's first pass fails on its first page. Before it ran, the run returned every verdict the
	// narrow scanner made to pending, cleared, with the senders' prior hits counted again, and reopened
	// the second pass, so no stale verdict releases a body while the first pass cannot end.
	refusing := first
	refusing.Fetch = func(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return mail.Page[mail.MessageMetadata]{}, fmt.Errorf("the credential: %w", mail.ErrAuthentication)
	}
	if err := backfillAccount(t.Context(), refusing, second, "personal", metrics1, metrics2, none, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatalf("a run whose first pass the provider refuses ended without an error")
	}
	if r := before["o1"]; r.State != "scanned" || len(r.Flags) == 0 {
		t.Fatalf("under the narrow scanner the login code is %+v, want flagged", r)
	}
	stalePending := func(when string) {
		t.Helper()
		rows := rescanRows(t, conn, "personal")
		for _, id := range []string{"s1", "s2", "s3", "o1"} {
			if r := rows[id]; r.State != "pending" || len(r.Flags) != 0 || r.VerdictRevision != "" {
				t.Errorf("%s, message %s is %+v, want pending with its verdict cleared", when, id, r)
			}
		}
		// A skip whose subject the new scanner masks waits for the re-mask, the window ADR-0096 states.
		if r := rows["n1"]; r.State != "skipped_gate" {
			t.Errorf("%s, the news code message is %+v, want its skip in force until the first pass masks its subject again", when, r)
		}
		for _, domain := range []string{"shop.example", "login.example"} {
			var hits int64
			if err := conn.QueryRow(t.Context(), "SELECT scan_hit_count FROM senders WHERE account_id = 'personal' AND domain = $1", domain).Scan(&hits); err != nil || hits != 0 {
				t.Errorf("%s, %s has %d prior hits (%v), want 0, its flagged verdicts returned to pending", when, domain, hits, err)
			}
		}
	}
	stalePending("while the first pass fails")
	if ended, scanEnded := completion(t, conn, "personal"); ended || scanEnded {
		t.Errorf("after a run whose first pass failed the passes are recorded ended as %v and %v, want both reopened", ended, scanEnded)
	}

	// The next run's first pass resumes and masks every subject again. Part way through it the stale
	// verdicts are still pending.
	if _, err := pass2.Reopen(t.Context(), second, "personal"); err != nil {
		t.Fatal(err)
	}
	p1, err := pass1.Open(t.Context(), first, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if p1.Run() == "" {
		t.Fatalf("the first pass did not run after a change of scanner")
	}
	for page := 0; ; page++ {
		s, err := p1.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if page == 0 {
			stalePending("while the first pass runs")
		}
		if s.Done {
			break
		}
	}
	masked := marker.Field("newsone") + " Your verification code is ██████"
	remasked := rescanRows(t, conn, "personal")
	for id, r := range remasked {
		if r.SubjectRevision != "wide-revision" {
			t.Errorf("message %s's subject is masked under %q, want the new scanner's revision", id, r.SubjectRevision)
		}
	}
	if r := remasked["n1"]; r.Subject != masked || r.EventsUnderStamp != 1 {
		t.Errorf("the news code subject is %+v, want %q with one masking event under the new scanner", r, masked)
	}
	if r := remasked["b1"]; r.Subject != marker.Field("bank")+" Your verification code is ██████" || r.EventsUnderStamp != 1 {
		t.Errorf("the bank's subject is %+v, want its code masked with one masking event under the new scanner", r)
	}
	gone := remasked["s3"]
	if wantSubject := []rune(before["s3"].Subject); gone.Subject != string(repeat('█', len(wantSubject))) || !gone.WholeSubjectEvent || gone.EventsUnderStamp != 1 {
		t.Errorf("the removed message's subject is %+v, want it masked whole with the event of a whole subject", gone)
	}
	var goneItems int
	if err := conn.QueryRow(t.Context(), `SELECT count(*) FROM job_run_failures AS f JOIN job_runs AS r USING (account_id, run_id)
		WHERE f.account_id = 'personal' AND r.pass = 'pass1' AND f.item_kind = 'message' AND f.item_id = 's3'
			AND f.error_class = 'gone' AND f.disposition = 'gone'`).Scan(&goneItems); err != nil || goneItems != 1 {
		t.Errorf("the first pass recorded the removed message as gone %d times (%v), want once", goneItems, err)
	}
	if first, _ := completion(t, conn, "personal"); !first {
		t.Errorf("the reopened first pass did not end")
	}

	// The second pass then returns the skip decided without its subject's signal to pending before its
	// first page, and scans everything waiting, the new mail the first pass added included.
	p, err := pass2.Open(t.Context(), second, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if p.Run() == "" {
		t.Fatalf("the second pass did not run after a change of scanner")
	}
	if r := rescanRows(t, conn, "personal")["n1"]; r.State != "pending" {
		t.Errorf("the news code message is %+v before the first page, want pending", r)
	}
	for _, id := range []string{"b1", "n2", "n3"} {
		if r := rescanRows(t, conn, "personal")[id]; r.State != before[id].State {
			t.Errorf("message %s is %s, want it left %s", id, r.State, before[id].State)
		}
	}
	for {
		s, err := p.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if s.Done {
			break
		}
	}

	after := rescanRows(t, conn, "personal")
	wantStates := map[string]string{
		"b1": "skipped_restricted", "n1": "scanned", "n2": "skipped_gate", "n3": "skipped_gate", "s1": "scanned", "s2": "scanned", "s3": "pending",
		"o1": "scanned", "s4": "scanned",
	}
	for id, want := range wantStates {
		if after[id].State != want {
			t.Errorf("message %s is %+v, want %s", id, after[id], want)
		}
	}
	if r := after["s1"]; !cmp.Equal(r.Flags, []string{"mfa_code"}) || r.VerdictRevision != "wide-revision" {
		t.Errorf("the shop's code is %+v, want flagged under the new scanner", r)
	}
	if r := after["n1"]; r.Decision != "SCAN" || r.Reason != "subject_signal" || r.VerdictRevision != "wide-revision" {
		t.Errorf("the news code message is %+v, want scanned for its subject's signal under the new scanner", r)
	}
	if r := after["s4"]; !cmp.Equal(r.Flags, []string{"mfa_code"}) {
		t.Errorf("the new mail the reopened first pass added is %+v, want scanned and flagged", r)
	}
	for domain, want := range map[string]int64{"shop.example": 2, "login.example": 1} {
		var hits int64
		if err := conn.QueryRow(t.Context(), "SELECT scan_hit_count FROM senders WHERE account_id = 'personal' AND domain = $1", domain).Scan(&hits); err != nil || hits != want {
			t.Errorf("%s has %d prior hits (%v), want %d, each flagged message counted once", domain, hits, err, want)
		}
	}
	if first, second := completion(t, conn, "personal"); !first || !second {
		t.Errorf("the passes end as %v and %v, want both ended", first, second)
	}
	if found := bodyMarkers(t, conn, "personal", ""); len(found) != 0 {
		t.Errorf("fixture body text reached the index while scanning again: %v", found)
	}

	// A run after that finds nothing stale and does nothing.
	var runs int
	count := func() int {
		if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM job_runs WHERE account_id = 'personal'").Scan(&runs); err != nil {
			t.Fatal(err)
		}
		return runs
	}
	n := count()
	runBackfill(t, pool, "personal", f, wide, "again")
	if got := count(); got != n {
		t.Errorf("a run with nothing stale recorded %d runs, want none", got-n)
	}
	if diff := cmp.Diff(after, rescanRows(t, conn, "personal"), compare.Options); diff != "" {
		t.Errorf("a run with nothing stale changed the index (-before +after):\n%s", diff)
	}
}

// repeat returns n copies of r.
func repeat(r rune, n int) []rune {
	out := make([]rune, n)
	for i := range out {
		out[i] = r
	}
	return out
}

// VERIFICATIONS' row for a change of scanner, the second pass following a reopened first. Under the
// narrow scanner the mailbox holds only a restricted sender's message, so no verdict is ever stale. New
// mail with a code arrives and the scanner changes. The run reopens the first pass, which adds the new
// mail, and the second pass runs and scans it, rather than leaving it pending for good (ADR-0096).
func TestAReopenedFirstPassLeadsToASecondPass(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f, err := fake.New(fake.Config{Account: "personal", PageSize: 2}, fake.Message{
		Metadata: mail.MessageMetadata{
			ID: "b1", ThreadID: "tb1", From: mail.Address{Email: "alerts@bank.example"}, Subject: marker.Field("bank"),
			Date: mail.UnixMilli(1_700_000_000_000), SizeBytes: 4096,
		},
		Body: mail.MessageBody{Text: marker.Body("bank")},
	})
	if err != nil {
		t.Fatal(err)
	}
	runBackfill(t, pool, "personal", f, narrowScanner(t), "narrow")
	err = f.Deliver(fake.Message{
		Metadata: mail.MessageMetadata{
			ID: "s9", ThreadID: "ts9", From: mail.Address{Email: "orders@shop.example"}, Subject: marker.Field("shopnine"),
			Date: mail.UnixMilli(1_700_001_000_000), SizeBytes: 4096,
		},
		Body: mail.MessageBody{Text: marker.Body("shopnine") + " " + codeText + "."},
	})
	if err != nil {
		t.Fatal(err)
	}

	runBackfill(t, pool, "personal", f, wideScanner(t), "wide")

	if r := rescanRows(t, conn, "personal")["s9"]; r.State != "scanned" || !cmp.Equal(r.Flags, []string{"mfa_code"}) {
		t.Errorf("the new mail the reopened first pass added is %+v, want scanned and flagged", r)
	}
	if first, second := completion(t, conn, "personal"); !first || !second {
		t.Errorf("the passes end as %v and %v, want both ended", first, second)
	}
}

// VERIFICATIONS' row for a change of scanner, a release that bumps the scanner version while the
// configuration revision stays. Both passes run under the default scanner and end, a message is
// removed at the provider, and the index is set to record an earlier version beside the same revision,
// for every subject, masking event and verdict, as a release bumping the version leaves it. The next
// run returns every verdict to pending, masks every present subject again under the version in force,
// masks whole the subject of the removed message, and scans the verdicts again under it (ADR-0096,
// ADR-0005).
func TestAChangeOfScannerVersionAloneMasksAndScansAgain(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := rescanMailbox(t, "personal")
	wide := wideScanner(t)
	runBackfill(t, pool, "personal", f, wide, "first")
	before := rescanRows(t, conn, "personal")
	if err := f.Remove("s3"); err != nil {
		t.Fatal(err)
	}
	must(t, conn, `UPDATE messages SET subject_scanner_version = 0,
		scanner_version = CASE WHEN scan_state = 'scanned' THEN 0 END WHERE account_id = 'personal'`)
	must(t, conn, "UPDATE masking_events SET scanner_version = 0 WHERE account_id = 'personal'")

	runBackfill(t, pool, "personal", f, wide, "second")

	type versions struct {
		Subject, Verdict int
		State            string
	}
	rows, err := conn.Query(t.Context(), `SELECT message_id, subject_scanner_version, coalesce(scanner_version, -1), scan_state
		FROM messages WHERE account_id = 'personal'`)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]versions{}
	var id string
	var v versions
	if _, err := pgx.ForEachRow(rows, []any{&id, &v.Subject, &v.Verdict, &v.State}, func() error { got[id] = v; return nil }); err != nil {
		t.Fatal(err)
	}
	for id, v := range got {
		if v.Subject != 1 {
			t.Errorf("message %s's subject records version %d, want 1, masked again under the version in force", id, v.Subject)
		}
		if before[id].State == "scanned" && id != "s3" && (v.State != "scanned" || v.Verdict != 1) {
			t.Errorf("message %s is %+v, want scanned again under version 1", id, v)
		}
	}
	after := rescanRows(t, conn, "personal")
	for id, r := range after {
		if id != "s3" && (r.WholeSubjectEvent || r.Subject != before[id].Subject || r.EventsUnderStamp != before[id].EventsUnderStamp) {
			t.Errorf("message %s is %+v, want its subject masked again as it was, with its masks recorded under version 1", id, r)
		}
	}
	if r := after["s3"]; !r.WholeSubjectEvent || got["s3"].State != "pending" {
		t.Errorf("the removed message is %+v and %+v, want its subject masked whole and its verdict pending", r, got["s3"])
	}
}

// runRecord is a run's recorded row and timeline, as the jobs card reads them.
type runRecord struct {
	State, Checkpoint, Counters, Heartbeat, Finished, LastError string
	Events                                                      []string
}

func readRun(t *testing.T, conn *pgx.Conn, runID string) runRecord {
	t.Helper()
	var r runRecord
	err := conn.QueryRow(t.Context(), `SELECT state, checkpoint::text, counters::text, coalesce(heartbeat_at::text, ''),
			coalesce(finished_at::text, ''), coalesce(last_error, '')
		FROM job_runs WHERE account_id = 'personal' AND run_id = $1`, runID).
		Scan(&r.State, &r.Checkpoint, &r.Counters, &r.Heartbeat, &r.Finished, &r.LastError)
	if err != nil {
		t.Fatal(err)
	}
	rows, err := conn.Query(t.Context(), "SELECT kind FROM job_run_events WHERE account_id = 'personal' AND run_id = $1 ORDER BY seq", runID)
	if err != nil {
		t.Fatal(err)
	}
	if r.Events, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		t.Fatal(err)
	}
	return r
}

// VERIFICATIONS' row for a change of scanner, the record of a second pass stopped part way. Its run's
// row and timeline stay exactly as they were through the run-start step that returns its verdicts to
// pending, and the next run of the second pass, which resumes it, starts over from the first waiting
// message from what its checkpoint records, the scanner it scanned under, then scans every verdict
// again (ADR-0096, ADR-0022).
func TestAStoppedSecondPassKeepsItsRecordAndStartsOver(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := rescanMailbox(t, "personal")
	narrowFirst, narrowSecond := rescanDeps(t, pool, "personal", f, narrowScanner(t), "narrow")
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	none := func(context.Context, string) error { return nil }
	if err := passAccount(t.Context(), narrowFirst, "personal", metrics1, none, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	narrowSecond.PageSize = 4
	p, err := pass2.Open(t.Context(), narrowSecond, "personal")
	if err != nil {
		t.Fatal(err)
	}
	for range 2 {
		if s, err := p.Next(t.Context()); err != nil || s.Done {
			t.Fatalf("the second pass's page ended as %+v, %v, want a page made durable", s, err)
		}
	}
	stopped := p.Run()
	before := readRun(t, conn, stopped)

	first, second := rescanDeps(t, pool, "personal", f, wideScanner(t), "wide")
	marked, err := pass2.Reopen(t.Context(), second, "personal")
	if err != nil {
		t.Fatal(err)
	}
	if marked.Verdicts == 0 {
		t.Fatalf("the run-start step returned no verdict to pending")
	}
	if diff := cmp.Diff(before, readRun(t, conn, stopped), compare.Options); diff != "" {
		t.Errorf("the run-start step changed the stopped run's record (-before +after):\n%s", diff)
	}

	if err := passAccount(t.Context(), first, "personal", metrics1, none, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	p, err = pass2.Open(t.Context(), second, "personal")
	if err != nil {
		t.Fatal(err)
	}
	resumed := readRun(t, conn, p.Run())
	if want := `{"page": 0, "after": ""}`; resumed.Checkpoint != want {
		t.Errorf("the resuming run starts from %s, want %s, the first waiting message", resumed.Checkpoint, want)
	}
	if len(resumed.Events) < 2 || resumed.Events[0] != "resume" || resumed.Events[1] != "retry" {
		t.Errorf("the resuming run's timeline is %v, want it to open with resume and the retry that starts the pass over", resumed.Events)
	}

	// The run that started over cleared the mark, so a run resuming it after it stops resumes from its
	// checkpoint rather than starting over again.
	if s, err := p.Next(t.Context()); err != nil || s.Done {
		t.Fatalf("the restarted run's page ended as %+v, %v, want a page made durable", s, err)
	}
	stoppedAgain := readRun(t, conn, p.Run())
	if _, err := pass2.Reopen(t.Context(), second, "personal"); err != nil {
		t.Fatal(err)
	}
	p, err = pass2.Open(t.Context(), second, "personal")
	if err != nil {
		t.Fatal(err)
	}
	again := readRun(t, conn, p.Run())
	if again.Checkpoint != stoppedAgain.Checkpoint {
		t.Errorf("the run resuming the restarted run starts from %s, want its checkpoint %s", again.Checkpoint, stoppedAgain.Checkpoint)
	}
	if len(again.Events) > 1 && again.Events[1] == "retry" {
		t.Errorf("the run resuming the restarted run's timeline is %v, want it to resume without starting over", again.Events)
	}
	for {
		s, err := p.Next(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		if s.Done {
			break
		}
	}
	if r := rescanRows(t, conn, "personal")["s1"]; r.State != "scanned" || !cmp.Equal(r.Flags, []string{"mfa_code"}) || r.VerdictRevision != "wide-revision" {
		t.Errorf("the shop's code is %+v, want scanned again and flagged under the new scanner", r)
	}
}

// VERIFICATIONS' row for a change of scanner, a change reverted before any second pass ran under it.
// Under the narrow scanner the second pass stops part way, past the messages it scanned. The scanner
// changes to the default one, and that run returns those verdicts to pending before its first pass,
// which then fails. The scanner is reverted to the narrow one. The next run finds nothing stale, and
// its second pass, resuming the stopped run, still starts over from the first waiting message, because
// the earlier run's start marked it to, so it scans every verdict the change returned to pending and no
// message is left waiting with both passes ended (ADR-0096).
func TestARevertedChangeOfScannerStillScansWhatItReturnedToPending(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	f := rescanMailbox(t, "personal")
	metrics1, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	metrics2, err := pass2.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	none := func(context.Context, string) error { return nil }
	narrowFirst, narrowSecond := rescanDeps(t, pool, "personal", f, narrowScanner(t), "narrow")
	if err := passAccount(t.Context(), narrowFirst, "personal", metrics1, none, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}
	p, err := pass2.Open(t.Context(), narrowSecond, "personal")
	if err != nil {
		t.Fatal(err)
	}
	for range 3 {
		if s, err := p.Next(t.Context()); err != nil || s.Done {
			t.Fatalf("the second pass's page ended as %+v, %v, want a page made durable", s, err)
		}
	}
	if r := rescanRows(t, conn, "personal")["s1"]; r.State != "scanned" {
		t.Fatalf("the shop's message is %+v before the change, want scanned", r)
	}

	wideFirst, wideSecond := rescanDeps(t, pool, "personal", f, wideScanner(t), "wide")
	wideFirst.Fetch = func(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return mail.Page[mail.MessageMetadata]{}, fmt.Errorf("the credential: %w", mail.ErrAuthentication)
	}
	if err := backfillAccount(t.Context(), wideFirst, wideSecond, "personal", metrics1, metrics2, none, slog.New(slog.DiscardHandler)); err == nil {
		t.Fatalf("a run whose first pass the provider refuses ended without an error")
	}
	if r := rescanRows(t, conn, "personal")["s1"]; r.State != "pending" {
		t.Fatalf("the shop's message is %+v after the change, want pending", r)
	}

	runBackfill(t, pool, "personal", f, narrowScanner(t), "reverted")

	for id, r := range rescanRows(t, conn, "personal") {
		if r.State == "pending" {
			t.Errorf("message %s is left pending after the revert, want every waiting message read", id)
		}
	}
	if r := rescanRows(t, conn, "personal")["o1"]; r.State != "scanned" || r.VerdictRevision != "narrow-revision" {
		t.Errorf("the login code is %+v, want scanned again under the narrow scanner", r)
	}
	if first, second := completion(t, conn, "personal"); !first || !second {
		t.Errorf("the passes end as %v and %v, want both ended", first, second)
	}
}
