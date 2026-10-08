//go:build integration

package tick_test

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	coreindex "github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/sync/internal/tick"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// revoke revokes a grant of delta sync's role for the rest of the test, and grants it again when the
// test ends, so the tests after it see the role as the migration chain made it.
func revoke(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "REVOKE "+privilege+" FROM "+role)
	url := postgres.URL(t)
	t.Cleanup(func() {
		ctx := context.Background()
		c, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := c.Close(ctx); err != nil {
				t.Error(err)
			}
		}()
		if _, err := c.Exec(ctx, "GRANT "+privilege+" TO "+role); err != nil {
			t.Error(err)
		}
	})
}

// VERIFICATIONS' row for a tick applying the provider's changes. An account with no cursor is
// reconciled over the first window inside its first tick, which stores the cursor it took first and
// records the window with no gap. The next tick applies what changed since, each new message
// classified and its subject masked with a masking event naming the scanner it was masked under, a
// changed message's labels and flags set, and a removed message removed with its sender's statistics.
// The cursor advances to the provider's, its write time recorded, and the tick's counters record the
// change sets. Before backfill's second pass has ended nothing is decided or scanned (ADR-0018,
// ADR-0104, ADR-0105).
func TestATickAppliesTheChangesSinceItsCursor(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, false)
	f := mailbox(t, account,
		message("m0", "old@shop.example", now.Add(-8*day)),
		message("m1", "orders@shop.example", now.Add(-6*day), mail.Inbox),
		message("m5", "once@gone.example", now.Add(-day), mail.Inbox),
	)
	p := &direct{port: f}
	d := deps(t, syncPool(t), p, listing(t, "bank.example"), account)

	first := mustTick(t, d, account)

	cursor, written := cursorOf(t, conn, account)
	if cursor != "h0" || written == nil {
		t.Fatalf("the first tick stored the cursor %q written at %v, want the provider's h0 with its write time", cursor, written)
	}
	if got := mapsKeys(index(t, conn, account)); !slices.Equal(got, []string{"m1", "m5"}) {
		t.Errorf("the first tick indexed %v, want the two messages of the first window", got)
	}
	if first.Gap || first.Recovery != "" {
		t.Errorf("the first tick reported a gap %v and recovery %q, want neither", first.Gap, first.Recovery)
	}
	other := message("m2", "security@security.example", now, mail.Inbox)
	other.Metadata.Subject = marker.Field("codesubject") + " Your code is 419283"
	deliver(t, f,
		other,
		message("m3", "alerts@bank.example", now),
		message("m4", "no-domain", now),
	)
	label(t, f, "m1", "Receipts")
	if err := f.Remove("m5"); err != nil {
		t.Fatal(err)
	}

	second := mustTick(t, d, account)

	got := index(t, conn, account)
	want := map[string]stored{
		"m1": {Labels: []string{mail.Inbox, "Receipts"}, Read: true, Subject: marker.Field("subject"), Class: "normal", State: "pending", Flags: []string{}, SubjectStampRevision: "a-revision"},
		"m2": {Labels: []string{mail.Inbox}, Subject: marker.Field("codesubject") + " Your code is ██████", Masked: true, Class: "normal", State: "pending", Flags: []string{}, SubjectStampRevision: "a-revision"},
		"m3": {Labels: []string{}, Subject: marker.Field("subject"), Class: "restricted", Rule: "rule.bank", State: "pending", Flags: []string{}, SubjectStampRevision: "a-revision"},
		"m4": {Labels: []string{}, Subject: marker.Field("subject"), Class: "restricted", State: "pending", Flags: []string{}, SubjectStampRevision: "a-revision"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the index after the second tick (-want +got):\n%s", diff)
	}
	if n := count(t, conn, "SELECT count(*) FROM masking_events WHERE account_id = $1 AND message_id = 'm2' AND scanner_revision = 'a-revision'", account); n != 1 {
		t.Errorf("the code's subject has %d masking events under the scanner, want 1", n)
	}
	if n := count(t, conn, "SELECT count(*) FROM senders WHERE account_id = $1 AND domain = 'gone.example'", account); n != 0 {
		t.Errorf("the removed message's sender keeps %d statistics rows, want none", n)
	}
	if n := count(t, conn, "SELECT message_count FROM senders WHERE account_id = $1 AND domain = 'shop.example'", account); n != 1 {
		t.Errorf("the shop's statistics count %d messages, want 1", n)
	}
	if second.Unclassified != 1 {
		t.Errorf("the tick counted %d unclassified senders, want the one with no domain", second.Unclassified)
	}
	if c, _ := cursorOf(t, conn, account); c != "h6" {
		t.Errorf("the cursor reads %q after the second tick, want the provider's h6", c)
	}
	if n := count(t, conn, "SELECT count(*) FROM scan_gate_decisions WHERE account_id = $1", account); n != 0 || second.Scanning {
		t.Errorf("the tick decided %d messages before the second pass ended, want none", n)
	}
	r := runs(t, conn, account)
	if len(r) != 2 || r[0].Pass != "tick" || r[1].Pass != "tick" || r[0].State != "succeeded" || r[1].State != "succeeded" {
		t.Fatalf("the runs are %+v, want two ticks that succeeded", r)
	}
	if diff := cmp.Diff(map[string]any{
		"added": 0.0, "modified": 0.0, "removed": 0.0, "decided": 0.0, "scanned": 0.0, "skipped": 0.0, "pending": 0.0,
		"window_start": now.Add(-7 * day).Format(time.RFC3339), "window_end": now.Format(time.RFC3339), "reconciled": 2.0,
	}, r[0].Counters, compare.Options); diff != "" {
		t.Errorf("the first tick's counters (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]any{
		"added": 3.0, "modified": 1.0, "removed": 1.0, "decided": 0.0, "scanned": 0.0, "skipped": 0.0, "pending": 0.0,
	}, r[1].Counters, compare.Options); diff != "" {
		t.Errorf("the second tick's counters (-want +got):\n%s", diff)
	}
}

// mapsKeys returns a map's keys, sorted.
func mapsKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

// VERIFICATIONS' row for a tick applying the provider's changes, the cursor. A change set whose
// application fails stores nothing of it, and the cursor stays where it was, so the next tick asks for
// the same changes and applies them (ADR-0018).
func TestTheCursorNeverRunsAheadOfTheIndex(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, false)
	f := mailbox(t, account)
	d := deps(t, syncPool(t), &direct{port: f}, listing(t), account)
	mustTick(t, d, account)
	coded := message("m1", "security@security.example", now)
	coded.Metadata.Subject = "Your code is 419283"
	deliver(t, f, coded, message("m2", "orders@shop.example", now))
	revoke(t, conn, "INSERT (account_id, message_id, field, rule_id, tier, scanner_version, scanner_revision) ON masking_events")

	if _, err := tick.Run(t.Context(), d, account); err == nil {
		t.Fatal("the tick succeeded though its change set could not be made durable")
	}

	if c, _ := cursorOf(t, conn, account); c != "h0" {
		t.Errorf("the cursor reads %q after the failed tick, want h0, where it was", c)
	}
	if got := index(t, conn, account); len(got) != 0 {
		t.Errorf("the failed tick stored %v, want nothing of its change set", mapsKeys(got))
	}
	if r := runs(t, conn, account); r[len(r)-1].State != "failed" {
		t.Errorf("the failed tick is recorded %q, want failed", r[len(r)-1].State)
	}
	must(t, conn, "GRANT INSERT (account_id, message_id, field, rule_id, tier, scanner_version, scanner_revision) ON masking_events TO "+role)

	mustTick(t, d, account)

	if got := mapsKeys(index(t, conn, account)); !slices.Equal(got, []string{"m1", "m2"}) {
		t.Errorf("the next tick indexed %v, want both messages of the change set", got)
	}
	if c, _ := cursorOf(t, conn, account); c != "h2" {
		t.Errorf("the cursor reads %q, want h2", c)
	}
}

// VERIFICATIONS' row for invalidating the sync cursor, D4's part, the detection and the bounded
// recovery. A gap starts a recovery recorded as a run of its own, which takes the current cursor,
// lists the window from an hour before the last cursor's write time to now, applies every message of
// the threads it finds and none older, stores the cursor and records its window and the messages it
// reconciled. The next tick asks for changes from the new cursor with no gap (ADR-0018, ADR-0105).
func TestAGapIsRecoveredOverTheWindowSinceTheLastCursor(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, false)
	f := mailbox(t, account, message("m1", "orders@shop.example", now.Add(-2*day), mail.Inbox))
	p := &direct{port: f}
	d := deps(t, syncPool(t), p, listing(t), account)
	mustTick(t, d, account)
	written := now.Add(-3 * time.Hour)
	must(t, conn, "UPDATE account_state SET sync_cursor_at = $1 WHERE account_id = $2", written, account)
	deliver(t, f,
		message("m2", "early@shop.example", written.Add(-2*time.Hour)),
		message("m3", "inside@shop.example", written.Add(-30*time.Minute)),
		message("m4", "later@shop.example", written.Add(time.Hour)),
	)
	label(t, f, "m1", "Receipts")
	f.ExpireCursors()
	gaps := prometheus.NewRegistry()
	metrics, err := tick.NewMetrics(gaps)
	if err != nil {
		t.Fatal(err)
	}

	r := mustTick(t, d, account)
	metrics.Count(account, r)

	if !r.Gap || r.Recovery == "" {
		t.Fatalf("the tick reported gap %v and recovery %q, want a gap and its recovery", r.Gap, r.Recovery)
	}
	got := index(t, conn, account)
	if diff := cmp.Diff([]string{"m1", "m3", "m4"}, mapsKeys(got), compare.Options); diff != "" {
		t.Errorf("the recovery indexed (-want +got):\n%s", diff)
	}
	if slices.Contains(got["m1"].Labels, "Receipts") {
		t.Errorf("a message older than the window was listed, its labels now %v", got["m1"].Labels)
	}
	all := runs(t, conn, account)
	recovery := all[len(all)-1]
	if recovery.ID != r.Recovery || recovery.Pass != "gap_recovery" || recovery.State != "succeeded" {
		t.Fatalf("the latest run is %+v, want the recovery %s succeeded", recovery, r.Recovery)
	}
	if diff := cmp.Diff(map[string]any{
		"window_start": written.Add(-time.Hour).Format(time.RFC3339), "window_end": now.Format(time.RFC3339), "reconciled": 2.0,
	}, recovery.Counters, compare.Options); diff != "" {
		t.Errorf("the recovery's counters (-want +got):\n%s", diff)
	}
	if c, _ := cursorOf(t, conn, account); c != "h5" {
		t.Errorf("the cursor reads %q after the recovery, want the provider's current h5", c)
	}
	if v := gauge(t, gaps, "mediated_mailbox_sync_cursor_gaps_total", account); v != 1 {
		t.Errorf("the gap series reads %v, want 1", v)
	}
	deliver(t, f, message("m6", "next@shop.example", now))

	next := mustTick(t, d, account)

	if next.Gap {
		t.Error("the tick after the recovery found a gap again")
	}
	if _, ok := index(t, conn, account)["m6"]; !ok {
		t.Error("the tick after the recovery did not apply the next change")
	}
}

// gauge returns the value of the series name with the account label in reg.
func gauge(t *testing.T, reg *prometheus.Registry, name, account string) float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "account" && l.GetValue() == account {
					if c := m.GetCounter(); c != nil {
						return c.GetValue()
					}
					return m.GetGauge().GetValue()
				}
			}
		}
	}
	t.Fatalf("no series %s for %s", name, account)
	return 0
}

// A recovery that fails is recorded as failed and stores nothing of its cursor, so the next tick meets
// the same gap, counts it again and recovers over the same window (ADR-0105).
func TestAFailedRecoveryIsTriedAgainByTheNextTick(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, false)
	f := mailbox(t, account)
	p := &direct{port: f}
	d := deps(t, syncPool(t), p, listing(t), account)
	mustTick(t, d, account)
	must(t, conn, "UPDATE account_state SET sync_cursor_at = $1 WHERE account_id = $2", now.Add(-time.Hour), account)
	deliver(t, f, message("m1", "orders@shop.example", now))
	f.ExpireCursors()
	p.failThreads = fmt.Errorf("listing: %w", mail.ErrProvider)

	r, err := tick.Run(t.Context(), d, account)

	if err == nil || !r.Gap {
		t.Fatalf("the tick returned %v with gap %v, want the failed recovery's error and a gap", err, r.Gap)
	}
	if c, _ := cursorOf(t, conn, account); c != "h0" {
		t.Errorf("the cursor reads %q after the failed recovery, want h0", c)
	}
	failed := runs(t, conn, account)
	states := []string{}
	for _, run := range failed[1:] {
		states = append(states, run.Pass+" "+run.State)
	}
	if diff := cmp.Diff([]string{"tick failed", "gap_recovery failed"}, states, compare.Options); diff != "" {
		t.Errorf("the runs after the first tick (-want +got):\n%s", diff)
	}

	again := mustTick(t, d, account)

	if !again.Gap || again.Recovery == "" {
		t.Errorf("the next tick reported gap %v and recovery %q, want the same gap recovered", again.Gap, again.Recovery)
	}
	if _, ok := index(t, conn, account)["m1"]; !ok {
		t.Error("the recovery that succeeded did not apply the window's message")
	}
}

// waiting returns a normal sender's message whose body is text, carrying a body marker and, with
// coded, a one-time code.
func waiting(id, address string, coded bool) fake.Message {
	m := message(id, address, now.Add(-time.Hour))
	if coded {
		m.Body.Text += " " + code
	}
	return m
}

// VERIFICATIONS' row for scanning what waits once backfill's second pass has ended. A tick over an
// account whose second pass has ended decides every waiting message with the gate and records the
// decision with its reason, scans a body the gate selects and records its verdict and its sender's
// hit, skips a restricted sender's message without asking for its body, and leaves a body the
// conversion refuses or the provider no longer has waiting, with a failed item. The backlog it reports
// counts what waits after it (ADR-0104, ADR-0093, ADR-0008, ADR-0017).
func TestOnceTheSecondPassHasEndedATickScansWhatWaits(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, true)
	refused := message("m5", "big@news.example", now.Add(-time.Hour))
	refused.Body = mail.MessageBody{HTML: "<p>" + marker.Body("refused") + strings.Repeat(" padding", 512*1024/8) + "</p>"}
	f := mailbox(t, account,
		waiting("m1", "orders@shop.example", true),
		waiting("m2", "hello@shop.example", false),
		waiting("m3", "alerts@bank.example", true),
		waiting("m4", "gone@news.example", false),
		refused,
	)
	p := &direct{port: f, failBody: map[string]error{"m4": fmt.Errorf("body: %w", mail.ErrNotFound)}}
	d := deps(t, syncPool(t), p, listing(t, "bank.example"), account)

	r := mustTick(t, d, account)

	got := index(t, conn, account)
	states := map[string]string{}
	for id, s := range got {
		states[id] = s.State + " " + s.Decision + " " + s.Reason + " " + strings.Join(s.Flags, ",")
	}
	want := map[string]string{
		"m1": "scanned SCAN recent_small mfa_code",
		"m2": "scanned SCAN recent_small ",
		"m3": "skipped_restricted SKIP restricted ",
		"m4": "pending SCAN recent_small ",
		"m5": "pending SCAN low_volume ",
	}
	if diff := cmp.Diff(want, states, compare.Options); diff != "" {
		t.Errorf("each message's state, decision, reason and flags (-want +got):\n%s", diff)
	}
	if slices.Contains(p.bodies, "m3") {
		t.Error("the restricted sender's body was asked for")
	}
	if got["m1"].Version != 1 || got["m1"].Revision != "a-revision" {
		t.Errorf("the verdict records scanner %d %q, want 1 a-revision", got["m1"].Version, got["m1"].Revision)
	}
	if n := count(t, conn, "SELECT scan_hit_count FROM senders WHERE account_id = $1 AND domain = 'shop.example'", account); n != 1 {
		t.Errorf("the shop has %d prior hits, want 1", n)
	}
	items := count(t, conn, `SELECT count(*) FROM job_run_failures WHERE account_id = $1 AND item_kind = 'message'
		AND ((item_id = 'm4' AND error_class = 'gone' AND disposition = 'gone') OR (item_id = 'm5' AND error_class = 'validation' AND disposition = 'abandoned'))`, account)
	if items != 2 {
		t.Errorf("found %d of the two failed items, the gone body and the refused one", items)
	}
	if !r.Scanning || r.Backlog != 2 {
		t.Errorf("the tick reported scanning %v with a backlog of %d, want scanning and the two left waiting", r.Scanning, r.Backlog)
	}
	last := runs(t, conn, account)
	if c := last[len(last)-1].Counters; c["decided"] != 5.0 || c["scanned"] != 2.0 || c["skipped"] != 1.0 || c["pending"] != 2.0 {
		t.Errorf("the tick's counters are %v, want five decided, two scanned, one skipped and two pending", c)
	}
}

// Before backfill's second pass has ended, a tick adds what arrived and leaves it waiting, so the
// second pass decides it and the gate never reads a cold index (ADR-0104, ADR-0017).
func TestBeforeTheSecondPassHasEndedATickScansNothing(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, false)
	f := mailbox(t, account, waiting("m1", "orders@shop.example", true))
	p := &direct{port: f}
	d := deps(t, syncPool(t), p, listing(t), account)

	r := mustTick(t, d, account)

	if r.Scanning || len(p.bodies) != 0 {
		t.Errorf("the tick scanned %v and asked for bodies %v, want neither", r.Scanning, p.bodies)
	}
	if s := index(t, conn, account)["m1"]; s.State != "pending" || s.Decision != "" {
		t.Errorf("the message is %q with decision %q, want pending and undecided", s.State, s.Decision)
	}
}

// A tick decides at most its bound of waiting messages and records where it stopped, the next tick
// goes on from there, and the tick that reaches the end starts the next read from the first waiting
// message again, where a message left waiting is asked for once more (ADR-0104).
func TestATickDecidesABoundedNumberAndTheNextGoesOn(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, true)
	var messages []fake.Message
	for i := range 5 {
		messages = append(messages, waiting(fmt.Sprintf("m%d", i+1), fmt.Sprintf("from%c@shop.example", 'a'+i), false))
	}
	f := mailbox(t, account, messages...)
	p := &direct{port: f, failBody: map[string]error{"m1": fmt.Errorf("body: %w", mail.ErrProvider)}}
	d := deps(t, syncPool(t), p, listing(t), account)
	d.Decisions = 2

	var after []string
	var asked [][]string
	for range 4 {
		p.bodies = nil
		mustTick(t, d, account)
		r := runs(t, conn, account)
		after = append(after, fmt.Sprint(r[len(r)-1].Checkpoint["after"]))
		asked = append(asked, p.bodies)
	}

	if diff := cmp.Diff([]string{"m2", "m4", "", ""}, after, compare.Options); diff != "" {
		t.Errorf("where each tick's scanning stopped (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([][]string{{"m1", "m2"}, {"m3", "m4"}, {"m5"}, {"m1"}}, asked, compare.Options); diff != "" {
		t.Errorf("the bodies each tick asked for (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for removing a fixture sender from the sensitive list, D4's part. Once the
// second pass has ended, a tick compares the messages stored as restricted or skipped as restricted
// with the policy it holds, returns a delisted sender's messages to pending with a normal class and
// no rule, and scans them in the same tick from the first waiting message (ADR-0037, ADR-0104).
func TestATickScansADelistedSendersMessages(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, true)
	f := mailbox(t, account, waiting("m1", "alerts@bank.example", true), waiting("m2", "orders@shop.example", false))
	p := &direct{port: f}
	listed := deps(t, syncPool(t), p, listing(t, "bank.example"), account)
	mustTick(t, listed, account)
	if s := index(t, conn, account)["m1"]; s.State != "skipped_restricted" {
		t.Fatalf("the bank's message is %q under the policy listing it, want skipped_restricted", s.State)
	}
	delisted := deps(t, syncPool(t), p, listing(t), account)

	mustTick(t, delisted, account)

	s := index(t, conn, account)["m1"]
	if s.Class != "normal" || s.Rule != "" || s.State != "scanned" || !slices.Equal(s.Flags, []string{"mfa_code"}) {
		t.Errorf("the delisted sender's message is %+v, want normal with no rule and scanned with its code flagged", s)
	}
	r := runs(t, conn, account)
	if n := count(t, conn, "SELECT count(*) FROM job_run_events WHERE account_id = $1 AND run_id = $2 AND kind = 'retry' AND detail LIKE 'the delisting transition returned 1 messages%'", account, r[len(r)-1].ID); n != 1 {
		t.Errorf("the tick recorded %d delisting events, want 1", n)
	}
}

// Once backfill's second pass has ended, a tick restricts the stored class of a sender a rule added
// since restricts, with the rule, and rebuilds its statistics, leaving the scan state it had. A tick
// under the same policy then marks nothing more (ADR-0113).
func TestATickRestrictsTheStoredClassOfAnAddedRulesSender(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, true)
	f := mailbox(t, account, waiting("m1", "orders@shop.example", false), waiting("m2", "news@news.example", false))
	p := &direct{port: f}
	mustTick(t, deps(t, syncPool(t), p, listing(t), account), account)
	if s := index(t, conn, account)["m1"]; s.Class != "normal" || s.State != "scanned" {
		t.Fatalf("the shop's message is %+v under no rule, want normal and scanned", s)
	}
	added := deps(t, syncPool(t), p, listing(t, "shop.example"), account)

	mustTick(t, added, account)

	stored := index(t, conn, account)
	if s := stored["m1"]; s.Class != "restricted" || s.Rule != "rule.shop" || s.State != "scanned" {
		t.Errorf("the shop's message is %+v, want restricted by rule.shop and still scanned", s)
	}
	if s := stored["m2"]; s.Class != "normal" || s.Rule != "" {
		t.Errorf("the news sender's message is %+v, want normal with no rule", s)
	}
	if n := count(t, conn, "SELECT count(*) FROM senders WHERE account_id = $1 AND domain = 'shop.example' AND sender_class = 'restricted'", account); n != 1 {
		t.Errorf("found %d restricted statistics for the shop, want 1", n)
	}
	marked, err := tick.NewPostgres(syncPool(t)).List(context.Background(), account, func(normal []string) []coreindex.Listing {
		return coreindex.Listed(listing(t, "shop.example").For(account), lookups, normal)
	})
	if err != nil || marked != 0 {
		t.Errorf("a second comparison marked %d messages with error %v, want none", marked, err)
	}
}

// A throttled body fetch ends the tick's scanning for the account. The message is recorded as a failed
// item, and nothing from it on is decided. It and every message after it stay waiting with no gate
// decision, since the gate would decide them without the hits their unscanned bodies hold. The tick
// still succeeds with its checkpoint at the last message decided, so the next tick asks for the
// throttled body first and goes on (ADR-0104).
func TestAThrottledBodyStopsTheAccountsScanning(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, true)
	f := mailbox(t, account, waiting("m1", "a@first.example", false), waiting("m2", "b@second.example", false),
		waiting("m3", "c@third.example", false), waiting("m4", "d@fourth.example", false))
	p := &direct{port: f, failBody: map[string]error{"m2": mail.ThrottleError{}}}
	d := deps(t, syncPool(t), p, listing(t), account)

	r := mustTick(t, d, account)

	if diff := cmp.Diff([]string{"m1", "m2"}, p.bodies, compare.Options); diff != "" {
		t.Errorf("the bodies asked for (-want +got):\n%s", diff)
	}
	decisions := map[string]string{}
	for id, s := range index(t, conn, account) {
		decisions[id] = s.State + " " + s.Decision
	}
	want := map[string]string{"m1": "scanned SCAN", "m2": "pending ", "m3": "pending ", "m4": "pending "}
	if diff := cmp.Diff(want, decisions, compare.Options); diff != "" {
		t.Errorf("each message's state and decision, nothing decided from the throttled message on (-want +got):\n%s", diff)
	}
	if n := count(t, conn, "SELECT count(*) FROM job_run_failures WHERE account_id = $1 AND item_id = 'm2' AND error_class = 'throttled'", account); n != 1 {
		t.Errorf("found %d throttled items for m2, want 1", n)
	}
	if after := r.Progress.Checkpoint.After; after != "m1" {
		t.Errorf("the tick's checkpoint is after %q, want m1, the last message decided", after)
	}
	delete(p.failBody, "m2")
	p.bodies = nil

	mustTick(t, d, account)

	if diff := cmp.Diff([]string{"m2", "m3", "m4"}, p.bodies, compare.Options); diff != "" {
		t.Errorf("the bodies the next tick asked for (-want +got):\n%s", diff)
	}
}

// Every call a tick makes through Leased spends from the account's budget in the sync class
// (ADR-0025).
func TestATickSpendsInTheSyncClass(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, true)
	f := mailbox(t, account, waiting("m1", "orders@shop.example", false))
	reg := prometheus.NewRegistry()
	metrics, err := lease.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	limiter := lease.New(syncPool(t), f.RateProfile().BudgetPerSecond(), metrics)
	d := deps(t, syncPool(t), tick.Leased{Limiter: limiter, Port: f, Account: account}, listing(t), account)

	mustTick(t, d, account)

	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	classes := map[string]float64{}
	for _, fam := range families {
		if fam.GetName() != "mediated_mailbox_ratelimit_granted_total" {
			continue
		}
		for _, m := range fam.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "class" {
					classes[l.GetValue()] += m.GetCounter().GetValue()
				}
			}
		}
	}
	if classes["sync"] == 0 || classes["batch"] != 0 || classes["interactive"] != 0 {
		t.Errorf("the tick was granted %v by class, want the sync class alone", classes)
	}
	if s := index(t, conn, account)["m1"]; s.State != "scanned" {
		t.Errorf("the message is %q, want scanned through the leased calls", s.State)
	}
}

// partial is a Provider whose thread listing leaves out the thread hidden names, as a search that
// misses a message the mailbox holds would, and fails the first listing asked for with a page token
// while failLater is set.
type partial struct {
	*direct
	hidden    string
	failLater bool
}

func (p *partial) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	if page != "" && p.failLater {
		p.failLater = false
		return mail.Page[mail.ThreadMetadata]{}, fmt.Errorf("listing: %w", mail.ErrProvider)
	}
	got, err := p.direct.ListThreads(ctx, q, page)
	got.Items = slices.DeleteFunc(slices.Clone(got.Items), func(th mail.ThreadMetadata) bool { return th.ID == p.hidden })
	return got, err
}

// VERIFICATIONS' row for a gap recovery's removal of what the provider no longer holds. A message
// dated in the window that the provider removed during the gap is removed with its sender's
// statistics, once a listing read to its last page left it out and the provider no longer returns it
// by its identifier. A message the listing left out but the provider still holds stays, a recovery
// whose listing failed on a later page removes nothing, and a message dated before the window that
// was removed during the gap stays, the residual ADR-0105 states (ADR-0105).
func TestARecoveryRemovesWhatTheProviderNoLongerHolds(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, false)
	written := now.Add(-3 * time.Hour)
	f := mailbox(t, account,
		message("m1", "gone@removed.example", written.Add(-30*time.Minute)),
		message("m2", "kept@hidden.example", written.Add(-20*time.Minute)),
		message("m3", "old@before.example", now.Add(-2*day)),
		message("m4", "four@shop.example", written.Add(-10*time.Minute)),
		message("m5", "five@shop.example", written.Add(-5*time.Minute)),
	)
	p := &partial{direct: &direct{port: f}, hidden: "tm2"}
	d := deps(t, syncPool(t), p, listing(t), account)
	p.hidden = ""
	mustTick(t, d, account)
	p.hidden, p.failLater = "tm2", true
	must(t, conn, "UPDATE account_state SET sync_cursor_at = $1 WHERE account_id = $2", written, account)
	for _, id := range []string{"m1", "m3"} {
		if err := f.Remove(id); err != nil {
			t.Fatal(err)
		}
	}
	f.ExpireCursors()

	if _, err := tick.Run(t.Context(), d, account); err == nil {
		t.Fatal("the tick succeeded though its recovery's listing failed")
	}

	if diff := cmp.Diff([]string{"m1", "m2", "m3", "m4", "m5"}, mapsKeys(index(t, conn, account)), compare.Options); diff != "" {
		t.Errorf("the index after the failed recovery (-want +got):\n%s", diff)
	}

	r := mustTick(t, d, account)

	if diff := cmp.Diff([]string{"m2", "m3", "m4", "m5"}, mapsKeys(index(t, conn, account)), compare.Options); diff != "" {
		t.Errorf("the index after the recovery (-want +got):\n%s", diff)
	}
	if n := count(t, conn, "SELECT count(*) FROM senders WHERE account_id = $1 AND domain = 'removed.example'", account); n != 0 {
		t.Errorf("the removed message's sender keeps %d statistics rows, want none", n)
	}
	all := runs(t, conn, account)
	recovery := all[len(all)-1]
	if recovery.ID != r.Recovery || recovery.State != "succeeded" || recovery.Counters["reconciled"] != 1.0 {
		t.Errorf("the latest run is %+v, want the recovery %s succeeded having reconciled the one removal", recovery, r.Recovery)
	}
}

// cancelling is a Provider that cancels the tick's context at the first call of the operation at, as
// a process stopped there would be.
type cancelling struct {
	*direct
	at     mail.Operation
	cancel context.CancelFunc
}

func (c *cancelling) ChangesSince(ctx context.Context, cur mail.Cursor) (mail.ChangeSet, error) {
	if c.at == mail.OpChangesSince {
		c.cancel()
		return mail.ChangeSet{}, ctx.Err()
	}
	return c.direct.ChangesSince(ctx, cur)
}

func (c *cancelling) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	if c.at == mail.OpListThreads {
		c.cancel()
		return mail.Page[mail.ThreadMetadata]{}, ctx.Err()
	}
	return c.direct.ListThreads(ctx, q, page)
}

// VERIFICATIONS' row for a tick or a recovery a stopped process left running. A tick stopped part way,
// and separately a gap recovery stopped part way with the tick that started it, stay recorded as
// running only until the account's next tick, which records each as failed, saying it stopped before
// it recorded its end, and records itself as succeeded (ADR-0103, ADR-0022).
func TestATickRecordsAStoppedRunAsFailed(t *testing.T) {
	for _, c := range []struct {
		name string
		at   mail.Operation
		want []string
	}{
		{"a tick stopped part way", mail.OpChangesSince, []string{"tick failed", "tick succeeded"}},
		{"a recovery stopped part way", mail.OpListThreads, []string{"tick failed", "gap_recovery failed", "tick succeeded", "gap_recovery succeeded"}},
	} {
		t.Run(c.name, func(t *testing.T) {
			conn := superuser(t)
			account := newAccount(t, conn, false)
			f := mailbox(t, account, message("m1", "orders@shop.example", now.Add(-time.Hour)))
			d := deps(t, syncPool(t), &direct{port: f}, listing(t), account)
			mustTick(t, d, account)
			if c.at == mail.OpListThreads {
				deliver(t, f, message("m2", "next@shop.example", now))
				f.ExpireCursors()
			}
			ctx, cancel := context.WithCancel(t.Context())
			stopped := d
			stopped.Provider = &cancelling{direct: &direct{port: f}, at: c.at, cancel: cancel}
			if _, err := tick.Run(ctx, stopped, account); err == nil {
				t.Fatal("the stopped tick succeeded")
			}

			mustTick(t, d, account)

			var states []string
			for _, r := range runs(t, conn, account)[1:] {
				states = append(states, r.Pass+" "+r.State)
			}
			if diff := cmp.Diff(c.want, states, compare.Options); diff != "" {
				t.Errorf("the runs after the first tick (-want +got):\n%s", diff)
			}
			if n := count(t, conn, "SELECT count(*) FROM job_runs WHERE account_id = $1 AND state = 'failed' AND last_error = 'the run stopped before it recorded its end'", account); n != len(c.want)/2 {
				t.Errorf("found %d runs recorded as stopped, want %d", n, len(c.want)/2)
			}
		})
	}
}
