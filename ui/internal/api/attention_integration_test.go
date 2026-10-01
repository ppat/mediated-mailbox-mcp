//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
)

// card is one worth-a-look card as the attention endpoint sends it.
type card struct {
	Rule     string  `json:"rule"`
	What     string  `json:"what"`
	Number   int64   `json:"number"`
	Since    *string `json:"since"`
	Sentence string  `json:"sentence"`
	Link     string  `json:"link"`
}

// cardsUnder seeds the fixture database, runs the statements given as the superuser, and returns the
// personal account's cards under the thresholds.
func cardsUnder(t *testing.T, thresholds attention.Thresholds, statements ...string) []card {
	t.Helper()
	seed(t)
	execute(t, statements...)
	s, _ := serverWith(t, uiPool(t, "mediated_mailbox_ui"), thresholds)
	r := get(t, s.Handler(), "/api/personal/attention")
	if r.status != http.StatusOK {
		t.Fatalf("the attention endpoint answered %d: %s", r.status, r.body)
	}
	var body struct {
		Cards []card `json:"cards"`
	}
	if err := json.Unmarshal(r.body, &body); err != nil {
		t.Fatal(err)
	}
	return body.Cards
}

// execute runs statements as the superuser, each taking the personal account as $1 and the server's
// clock as $2.
func execute(t *testing.T, statements ...string) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	for _, sql := range statements {
		if _, err := conn.Exec(t.Context(), sql, personal, now); err != nil {
			t.Fatalf("%v\n%s", err, sql)
		}
	}
}

// only is the cards of one rule.
func only(cards []card, rule string) []card {
	out := []card{}
	for _, c := range cards {
		if c.Rule == rule {
			out = append(out, c)
		}
	}
	return out
}

func since(s string) *string { return &s }

// serves writes n READ_BODY audit rows at the instant now less the interval, and n DENY_BODY rows beside
// them, which are not serves.
func serves(n int, ago string) []string {
	insert := func(action string) string {
		return `INSERT INTO audit_log (ts, account_id, actor, action, message_id)
			SELECT $2::timestamptz - interval '` + ago + `', $1, 'agent', '` + action + `', 'm-news' FROM generate_series(1, ` + itoa(n) + `)`
	}
	return []string{insert("READ_BODY"), insert("DENY_BODY")}
}

func itoa(n int) string { return strconv.Itoa(n) }

// The clock is 2026-09-10 10:16:04Z, so the 24 hours start at 2026-09-09 10:16:04Z and the baseline is
// the seven whole UTC days from 2026-09-02 to 2026-09-08. Each scenario replaces the seeded audit rows.
func bodyServeScenario(t *testing.T, thresholds attention.Thresholds, recent int, baseline ...string) []card {
	t.Helper()
	statements := []string{`DELETE FROM audit_log WHERE account_id = $1 AND ts <= $2`}
	statements = append(statements, serves(recent, "1 hour")...)
	statements = append(statements, baseline...)
	return only(cardsUnder(t, thresholds, statements...), "body_serves")
}

// baselineDays writes serves at noon on each of the seven baseline days, more the further back the day
// is, 2026-09-08 holding 7 and 2026-09-02 holding 1, so a baseline shifted by a day has another median.
func baselineDays() []string {
	var out []string
	for ago := 1; ago <= 7; ago++ {
		out = append(out, serves(8-ago, itoa(ago)+" days 22 hours 16 minutes 4 seconds")...)
	}
	return out
}

// TestTheBodyServeRuleFiresFarAboveItsMedian drives body serves far above the daily median and requires
// the card, and requires it silent at twice the median, silent when disabled, deaf to body denials, and
// deaf to serves outside the baseline's days (VERIFICATIONS, the body-fetch rate row, M3's part).
//
// The baseline is the seven whole UTC days 2026-09-02 to 2026-09-08. Ten more serves at 01:00 on
// 2026-09-02, before the time of day the 24 hours start at, make that day 11 and the median 5. A rolling
// week, the 7 times 24 hours before the 24 hours start, would leave those ten out and take in the one serve
// at 06:16 on 2026-09-09, a median of 4, so 10 serves tell the two apart: silent against 5, firing against 4.
func TestTheBodyServeRuleFiresFarAboveItsMedian(t *testing.T) {
	// Outside the baseline sit five hundred serves at noon on 2026-09-01 and one in the hours of
	// 2026-09-09 before the 24 hours start, which no median of whole days reads.
	outside := append(serves(500, "8 days 22 hours 16 minutes 4 seconds"), serves(1, "1 day 4 hours")...)
	early := serves(10, "8 days 9 hours 16 minutes 4 seconds")
	baseline := append(append(baselineDays(), early...), outside...)

	got := bodyServeScenario(t, starting(), 40, baseline...)
	want := []card{{
		Rule: "body_serves", What: "Body serves", Number: 40, Since: since("2026-09-10T09:16:04Z"),
		Sentence: "40 bodies were served in 24 hours against a 7-day median of 5. Body-serve volume beyond triage plausibility is the anomaly the design watches for.",
		Link:     "/personal/audit?level=3&range=24h&action=READ_BODY",
	}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("far above the median (-want +got):\n%s", diff)
	}
	if got := bodyServeScenario(t, starting(), 10, baseline...); len(got) != 0 {
		t.Errorf("ten serves against a median of 5 fired %+v", got)
	}
	if got := bodyServeScenario(t, starting(), 11, baseline...); len(got) != 1 || got[0].Number != 11 {
		t.Errorf("eleven serves against a median of 5 fired %+v", got)
	}
	disabled := starting()
	disabled.ServeFactor = 0
	if got := bodyServeScenario(t, disabled, 1000, baseline...); len(got) != 0 {
		t.Errorf("a factor of 0 fired %+v", got)
	}
}

// TestADayWithoutAServeCountsZero puts many serves on three baseline days and none on four, so the
// median is 0 and a single serve fires, as docs/UI.md section 8.1 keeps on purpose. Taking the median of
// the days with a serve alone would read 100 and stay silent.
func TestADayWithoutAServeCountsZero(t *testing.T) {
	baseline := append(serves(100, "2 days 22 hours"), serves(100, "3 days 22 hours")...)
	baseline = append(baseline, serves(100, "4 days 22 hours")...)
	got := bodyServeScenario(t, starting(), 1, baseline...)
	if len(got) != 1 || got[0].Sentence != "1 bodies were served in 24 hours against a 7-day median of 0. Body-serve volume beyond triage plausibility is the anomaly the design watches for." {
		t.Errorf("one serve after four quiet days fired %+v", got)
	}
	if got := bodyServeScenario(t, starting(), 0, baseline...); len(got) != 0 {
		t.Errorf("no serve fired %+v", got)
	}
}

// TestTheSyncGapRuleShowsTheRecovery shows a gap recovery delta sync recorded, counting the ones that
// succeeded within the rule's days and wording the latest, and requires it silent when disabled and for
// a recovery that failed or fell outside the days (VERIFICATIONS, the cursor-invalidation row, M3's
// part).
func TestTheSyncGapRuleShowsTheRecovery(t *testing.T) {
	recoveries := `INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at, counters) VALUES
		($1, 'r-gap-old', 'sync', 'gap_recovery', 'succeeded', $2::timestamptz - interval '8 days', $2::timestamptz - interval '8 days', '{"window_start": "2026-09-01T00:00:00Z", "window_end": "2026-09-01T01:00:00Z", "reconciled": 2}'),
		($1, 'r-gap-failed', 'sync', 'gap_recovery', 'failed', $2::timestamptz - interval '1 hour', $2::timestamptz - interval '50 minutes', '{}'),
		($1, 'r-gap-new', 'sync', 'gap_recovery', 'succeeded', $2::timestamptz - interval '3 hours', $2::timestamptz - interval '2 hours', '{"window_start": "2026-09-09T20:00:00Z", "window_end": "2026-09-10T02:30:00Z", "reconciled": 1204}')`
	got := only(cardsUnder(t, starting(), recoveries), "sync_gap")
	// The seeded recovery r-0911 started two days ago and is the first of the two within seven days.
	want := []card{{
		Rule: "sync_gap", What: "Sync gap", Number: 2, Since: since("2026-09-08T10:16:04Z"),
		Sentence: "Delta sync recovered from a cursor gap on 2026-09-10 08:16Z, re-enumerating a 6h 30m window and reconciling 1,204 messages. A repeated gap means the cadence or the cursor lifetime needs attention.",
		Link:     "/personal/jobs?range=7d&pass=gap_recovery",
	}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the recoveries of seven days (-want +got):\n%s", diff)
	}
	nine := starting()
	nine.GapDays = 9
	got = only(cardsUnder(t, nine, recoveries), "sync_gap")
	if len(got) != 1 || got[0].Number != 3 || *got[0].Since != "2026-09-02T10:16:04Z" || got[0].Link != "/personal/jobs?range=2026-09-01,2026-09-10&pass=gap_recovery" {
		t.Errorf("the recoveries of nine days are %+v", got)
	}
	disabled := starting()
	disabled.GapDays = 0
	if got := only(cardsUnder(t, disabled, recoveries), "sync_gap"); len(got) != 0 {
		t.Errorf("a gap window of 0 fired %+v", got)
	}
	failedOnly := `UPDATE job_runs SET state = 'failed' WHERE account_id = $1 AND pass = 'gap_recovery' AND started_at < $2`
	if got := only(cardsUnder(t, starting(), failedOnly), "sync_gap"); len(got) != 0 {
		t.Errorf("failed recoveries fired %+v", got)
	}
}

// TestTheMaskingRuleCountsEachSenderAndRule fires on the seeded sender masked 21 times under one rule,
// stays silent on a sender at exactly the count, counts every spelling of a domain together, counts no
// event whose message the index no longer holds and no event of a masking a change of scanner replaced,
// and never counts the other account's events.
func TestTheMaskingRuleCountsEachSenderAndRule(t *testing.T) {
	statements := []string{
		`INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class, scan_state) VALUES
		($1, 'm-shop1', 't-shop', 'a@Shop.Example', 'Shop.Example', $2, false, 'normal', 'scanned'),
		($1, 'm-shop2', 't-shop', 'b@shop.example', 'shop.example', $2, false, 'normal', 'scanned')`,
		// Twenty events on the shop under one rule, eleven and nine across the two spellings, is at the count.
		`INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at)
		SELECT $1, CASE WHEN n < 11 THEN 'm-shop1' ELSE 'm-shop2' END, 'subject', 'content.link', 1, $2::timestamptz - make_interval(hours => n + 1)
		FROM generate_series(0, 19) AS n`,
		// Thirty events on a message the index no longer holds count toward no sender.
		`INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at)
		SELECT $1, 'm-removed', 'subject', 'content.link', 1, $2::timestamptz - interval '1 hour' FROM generate_series(1, 30)`,
		// A message of the shop whose subject a change of scanner masked again under the pair (2, r2), with
		// five events of the masking that replaced, under (1, r1), which count toward nothing (ADR-0096).
		`INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class, scan_state, subject_scanner_version, subject_scanner_revision) VALUES
		($1, 'm-shop3', 't-shop', 'c@shop.example', 'shop.example', $2, false, 'normal', 'scanned', 2, 'r2')`,
		`INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at, scanner_version, scanner_revision)
		SELECT $1, 'm-shop3', 'subject', 'content.link', 1, $2::timestamptz - interval '2 hours', 1, 'r1' FROM generate_series(1, 5)`,
		// Events older than seven days count toward nothing.
		`INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at)
		SELECT $1, 'm-shop1', 'subject', 'content.mfa', 1, $2::timestamptz - interval '8 days' FROM generate_series(1, 30)`,
	}
	got := only(cardsUnder(t, starting(), statements...), "masking")
	sender, rule := marker.MarkupField("masksender"), marker.MarkupField("maskrule")
	want := []card{{
		Rule: "masking", What: "Masking", Number: 21, Since: since("2026-09-04T10:16:04Z"),
		Sentence: "Masking fired 21 times on " + sender + " this week, all under " + rule + ". A sender masked this often under one rule is worth checking for an over-mask.",
		Link:     "/personal/masking?level=3&range=7d&rule=%3Cscript%3Emmfieldmarker-maskrule%3C%2Fscript%3E&sender=%3Cscript%3Emmfieldmarker-masksender%3C%2Fscript%3E",
	}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("masking (-want +got):\n%s", diff)
	}
	// One more event on the shop takes it above the count, and both spellings count as one sender.
	more := append(statements, `INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at) VALUES ($1, 'm-shop2', 'subject', 'content.link', 1, $2)`)
	got = only(cardsUnder(t, starting(), more...), "masking")
	// The shop's first event, 20 hours ago, is newer than the seeded sender's, so its card comes first.
	if len(got) != 2 || got[0].Number != 21 || *got[0].Since != "2026-09-09T14:16:04Z" ||
		got[0].Sentence[:40] != "Masking fired 21 times on shop.example t" {
		t.Errorf("the shop above the count is %+v", got)
	}
	disabled := starting()
	disabled.MaskCount = 0
	if got := only(cardsUnder(t, disabled, statements...), "masking"); len(got) != 0 {
		t.Errorf("a masking count of 0 fired %+v", got)
	}
}

// TestTheBacklogRuleFiresAboveItsShare fires on the seeded corpus, one pending message of five, and is
// silent at a share of 20 percent, which is exactly the backlog's, and when disabled.
func TestTheBacklogRuleFiresAboveItsShare(t *testing.T) {
	want := []card{{
		Rule: "backlog", What: "Scan backlog", Number: 1,
		Sentence: "1 messages are pending scan (20.0% of the corpus). Every pending message denies its body until scanned, which reads to the agent like a permission problem.",
		Link:     "/personal/messages?level=3&scan_state=pending",
	}}
	if diff := cmp.Diff(want, only(cardsUnder(t, starting()), "backlog"), compare.Options); diff != "" {
		t.Errorf("backlog (-want +got):\n%s", diff)
	}
	for _, share := range []float64{20, 0} {
		th := starting()
		th.BacklogShare = share
		if got := only(cardsUnder(t, th), "backlog"); len(got) != 0 {
			t.Errorf("a share of %v fired %+v", share, got)
		}
	}
}

// TestCardsComeNewestFirst requires the seeded state's four cards in section 8.1's order, the backlog
// first with no since, then by since, newest first.
func TestCardsComeNewestFirst(t *testing.T) {
	var rules []string
	for _, c := range cardsUnder(t, starting()) {
		rules = append(rules, c.Rule)
	}
	if diff := cmp.Diff([]string{"backlog", "body_serves", "sync_gap", "masking"}, rules, compare.Options); diff != "" {
		t.Errorf("order (-want +got):\n%s", diff)
	}
}

// TestARescanCountsOnlyTheCurrentMasks writes 25 messages sent 200 days ago whose subjects a change of
// scanner masked again an hour ago under (2, r2), after they were masked three days ago under (1, r1).
// The masking card counts the 25 current masks, since the re-mask is masking done this week, and none of
// the 25 masks it replaced, so no subject counts twice (docs/UI.md section 8.1, ADR-0096).
func TestARescanCountsOnlyTheCurrentMasks(t *testing.T) {
	got := only(cardsUnder(t, starting(),
		`INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class, scan_state, subject_scanner_version, subject_scanner_revision)
		SELECT $1, 'm-archive-' || n, 't-archive', 'old@archive.example', 'archive.example', $2::timestamptz - interval '200 days', false, 'normal', 'scanned', 2, 'r2'
		FROM generate_series(1, 25) AS n`,
		`INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at, scanner_version, scanner_revision)
		SELECT $1, 'm-archive-' || n, 'subject', 'content.link', 1, $2::timestamptz - interval '3 days', 1, 'r1' FROM generate_series(1, 25) AS n`,
		`INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at, scanner_version, scanner_revision)
		SELECT $1, 'm-archive-' || n, 'subject', 'content.link', 1, $2::timestamptz - interval '1 hour', 2, 'r2' FROM generate_series(1, 25) AS n`,
	), "masking")
	var archive []card
	for _, c := range got {
		if strings.Contains(c.Sentence, " on archive.example ") {
			archive = append(archive, c)
		}
	}
	if len(archive) != 1 || archive[0].Number != 25 || archive[0].Since == nil || *archive[0].Since != "2026-09-10T09:16:04Z" {
		t.Errorf("the re-masked sender's cards are %+v among %+v", archive, got)
	}
}
