//go:build integration

package pass2_test

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2"
	core "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2"
)

// message returns one fixture message from address, with a List-Id when listID is set, whose text part
// is text, and whose HTML part is html. It is sent a minute before the message whose identifier's
// number is one less, so the pass, which reads newest first (ADR-0126), reads a test's messages in the
// order of their numbers.
func message(id, address string, listID bool, text, html string) fake.Message {
	n, err := strconv.Atoi(strings.TrimLeft(id, "abcdefghijklmnopqrstuvwxyz"))
	if err != nil {
		panic("a fixture message's identifier ends in its number: " + id)
	}
	m := fake.Message{
		Metadata: mail.MessageMetadata{
			ID: id, ThreadID: "t" + id, From: mail.Address{Email: address}, Subject: marker.Field("subject"),
			Date: mail.UnixMilli(1_700_000_000_000 - int64(n)*60_000),
		},
		Body: mail.MessageBody{Text: text, HTML: html},
	}
	if listID {
		m.Metadata.ListID = "list.example"
	}
	return m
}

// drive runs the pass until it is done, failing the test on a step that fails.
func drive(t *testing.T, w *world) {
	t.Helper()
	w.open(t)
	for range 50 {
		s, err := w.pass.Next(context.Background())
		if err != nil {
			t.Fatalf("a step failed: %v", err)
		}
		if s.Done {
			return
		}
	}
	t.Fatal("the pass did not end within 50 steps")
}

// runEvents returns the kinds of the account's second-pass run events, in order.
func runEvents(t *testing.T, account string) []string {
	t.Helper()
	rows, err := superuser(t).Query(context.Background(), `SELECT e.kind FROM job_run_events AS e
		JOIN job_runs AS r ON r.account_id = e.account_id AND r.run_id = e.run_id
		WHERE e.account_id = $1 AND r.pass = 'pass2' ORDER BY e.seq`, account)
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			t.Fatal(err)
		}
		out = append(out, k)
	}
	return out
}

// VERIFICATIONS' rows for the scan gate over stored messages, D2's part. Over a fixture corpus the first
// pass filled, a restricted sender's message whose every other input asks for a scan is skipped as
// restricted and its body never asked for, a high-volume sender's list message is skipped by the gate
// with its reason, and every other message is scanned, its flags, content rules, scanner version and
// revision recorded. Every decision is recorded with its reason from the first evaluation, each
// flagged message adds one prior hit to its sender, the run is recorded with a progress event per page,
// and the pass sets its completion flag (ADR-0093, ADR-0008, ADR-0009, ADR-0117).
func TestAPassDecidesAndRecordsEveryWaitingMessage(t *testing.T) {
	news := func(i int) string { return fmt.Sprintf("n%d@news.example", i) }
	messages := []fake.Message{
		message("m01", "noreply@bank.example", false, marker.Body("bank")+" "+code, ""),
		message("m02", news(1), true, marker.Body("newsa"), ""),
		message("m03", news(2), true, marker.Body("newsb"), ""),
		message("m04", news(3), true, marker.Body("newsc"), ""),
		message("m05", "orders@shop.example", false, marker.Body("shop")+" "+code, ""),
		message("m06", "hello@shop.example", false, "", "<p>"+marker.Body("shophtml")+" Welcome.</p>"),
	}
	w := realWorldOf(t, 2, messages)

	drive(t, w)

	got := inspectPostgres(t, postgres.URL(t), w.account)
	want := map[string]stored{
		"m01": {Domain: "bank.example", State: "skipped_restricted", Flags: []string{}, Rules: []string{}, Decision: "SKIP", Reason: "restricted"},
		"m02": {Domain: "news.example", State: "skipped_gate", Flags: []string{}, Rules: []string{}, Decision: "SKIP", Reason: "high_volume_no_hits"},
		"m03": {Domain: "news.example", State: "skipped_gate", Flags: []string{}, Rules: []string{}, Decision: "SKIP", Reason: "high_volume_no_hits"},
		"m04": {Domain: "news.example", State: "skipped_gate", Flags: []string{}, Rules: []string{}, Decision: "SKIP", Reason: "high_volume_no_hits"},
		"m05": {
			Domain: "shop.example", State: "scanned", Flags: []string{"mfa_code"}, Rules: []string{"mfa.trigger_window"},
			Version: 1, Revision: "a-revision", Decision: "SCAN", Reason: "default",
		},
		"m06": {
			Domain: "shop.example", State: "scanned", Flags: []string{}, Rules: []string{},
			Version: 1, Revision: "a-revision", Decision: "SCAN", Reason: "prior_hit",
		},
	}
	if diff := cmp.Diff(want, got.Messages, compare.Options); diff != "" {
		t.Errorf("the messages as the pass left them (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"m05", "m06"}, w.fetched, compare.Options); diff != "" {
		t.Errorf("the bodies asked for (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]int64{"shop.example": 1}, got.Hits, compare.Options); diff != "" {
		t.Errorf("the senders' prior hits (-want +got):\n%s", diff)
	}
	if !got.Ended {
		t.Errorf("the pass did not set its completion flag")
	}
	if diff := cmp.Diff([]string{"start", "progress", "progress", "progress", "finish"}, runEvents(t, w.account), compare.Options); diff != "" {
		t.Errorf("the run's events (-want +got):\n%s", diff)
	}
	latest, _ := got.latest()
	wantProgress := core.Progress{}
	wantProgress.Checkpoint.Page, wantProgress.Checkpoint.After = 3, core.Position{SentAt: 1_700_000_000_000 - 6*60_000, ID: "m06"}
	wantProgress.Counters.Pages, wantProgress.Counters.Decided, wantProgress.Counters.Scanned, wantProgress.Counters.Skipped = 3, 6, 2, 4
	if latest.State != "succeeded" || latest.Progress != wantProgress {
		t.Errorf("the latest run is %+v, want one that succeeded at %+v", latest, wantProgress)
	}
}

// VERIFICATIONS' row for the second pass reading newest first. Six messages wait, sent in an order their
// identifiers do not follow, two of them in the same minute. The first page of two scans the two newest
// and leaves the four older ones waiting, so the newest mail is readable first, and the pass then reads
// the rest newest first, the two sent in the same minute by identifier, highest first (ADR-0126).
func TestTheSecondPassReadsTheNewestWaitingMessagesFirst(t *testing.T) {
	at := func(minute int64, m fake.Message) fake.Message {
		m.Metadata.Date = mail.UnixMilli(1_700_000_000_000 + minute*60_000)
		return m
	}
	w := realWorldOf(t, 2, []fake.Message{
		at(3, message("m01", "a@shop.example", false, marker.Body("one"), "")),
		at(5, message("m02", "b@shop.example", false, marker.Body("two"), "")),
		at(1, message("m03", "c@shop.example", false, marker.Body("three"), "")),
		at(4, message("m04", "d@shop.example", false, marker.Body("four"), "")),
		at(1, message("m05", "e@shop.example", false, marker.Body("five"), "")),
		at(2, message("m06", "f@shop.example", false, marker.Body("six"), "")),
	})
	w.step(t)
	states := func() map[string]string {
		out := map[string]string{}
		for id, m := range w.inspect(t).Messages {
			out[id] = m.State
		}
		return out
	}
	afterOne := map[string]string{"m01": "pending", "m02": "scanned", "m03": "pending", "m04": "scanned", "m05": "pending", "m06": "pending"}
	if diff := cmp.Diff(afterOne, states(), compare.Options); diff != "" {
		t.Errorf("the scan states after the first page (-want +got):\n%s", diff)
	}
	for range 10 {
		if w.step(t) {
			break
		}
	}
	if diff := cmp.Diff([]string{"m02", "m04", "m01", "m06", "m05", "m03"}, w.fetched, compare.Options); diff != "" {
		t.Errorf("the bodies asked for (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for a sender's first hit reaching its next message. On one page, a shop message
// holding a code is scanned first. A high-volume news sender's list message after it is still skipped,
// since the shop's hit is not the news sender's. The news sender's next message carries no List-Id, so
// the gate scans it by default and finds a code. The news sender's list messages after it would be
// skipped by the high-volume rule, and are scanned for the prior hit instead, so no skip outlives the
// sender's first hit and no hit reaches another sender (ADR-0094).
func TestASendersFirstHitReachesItsNextMessage(t *testing.T) {
	messages := []fake.Message{
		message("m01", "orders@shop.example", false, marker.Body("shop")+" "+code, ""),
		message("m02", "a@news.example", true, marker.Body("before"), ""),
		message("m03", "b@news.example", false, marker.Body("first")+" "+code, ""),
		message("m04", "c@news.example", true, marker.Body("second"), ""),
		message("m05", "d@news.example", true, marker.Body("third"), ""),
	}
	w := realWorldOf(t, 5, messages)

	drive(t, w)

	got := inspectPostgres(t, postgres.URL(t), w.account)
	reasons := map[string]string{}
	for id, m := range got.Messages {
		reasons[id] = m.Reason
	}
	want := map[string]string{"m01": "default", "m02": "high_volume_no_hits", "m03": "default", "m04": "prior_hit", "m05": "prior_hit"}
	if diff := cmp.Diff(want, reasons, compare.Options); diff != "" {
		t.Errorf("the gate's reasons (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]int64{"news.example": 1, "shop.example": 1}, got.Hits, compare.Options); diff != "" {
		t.Errorf("the senders' prior hits (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for scanning the released part of a body. The scan reads the body as it is
// released, the conversion of its HTML part when it has one and its text part in the literal form
// otherwise, so a code in the HTML part flags the message, a code in a text-only body flags it, and
// a clean HTML part is the whole of what is scanned, the text part beside it never fetched
// (ADR-0131).
func TestTheReleasedPartOfABodyIsScanned(t *testing.T) {
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("texta")+" "+code, "<p>"+marker.Body("htmla")+" Welcome.</p>"),
		message("m02", "b@shop.example", false, marker.Body("textb")+" Welcome.", "<p>"+marker.Body("htmlb")+" "+code+"</p>"),
		message("m03", "c@shop.example", false, marker.Body("textc")+" "+code, ""),
		message("m04", "d@shop.example", false, marker.Body("textd")+" Welcome.", "<p>"+marker.Body("htmld")+" Welcome.</p>"),
	}
	w := realWorldOf(t, 4, messages)

	drive(t, w)

	flags := map[string][]string{}
	for id, m := range inspectPostgres(t, postgres.URL(t), w.account).Messages {
		flags[id] = m.Flags
	}
	want := map[string][]string{"m01": {}, "m02": {"mfa_code"}, "m03": {"mfa_code"}, "m04": {}}
	if diff := cmp.Diff(want, flags, compare.Options); diff != "" {
		t.Errorf("the content flags (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for a body refused for good. A body whose rendered part the port refuses as past
// the bound, an HTML part and a text part, and one within the bound the conversion refuses, nesting
// more elements than the parser accepts, are each
// recorded refused with the bound they were refused under, never scanned, with a failed item of kind
// message, class refused and disposition refused, counted as refused, and the pass goes on to scan
// the next message and end. A later run reads none of them again, so none is fetched again (ADR-0131,
// ADR-0132).
func TestABodyRefusedIsRecordedRefusedAndNotFetchedAgain(t *testing.T) {
	over := strings.Repeat("a", mail.MaxBodyBytes)
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("text"), "<p>"+over+"</p>"),
		message("m02", "b@shop.example", false, marker.Body("plain")+over, ""),
		message("m03", "c@shop.example", false, marker.Body("text"), strings.Repeat("<div>", 600)+marker.Body("nested")+strings.Repeat("</div>", 600)),
		message("m04", "d@shop.example", false, marker.Body("next"), ""),
	}
	w := realWorldOf(t, 4, messages)
	inner := w.deps.Body
	fetched := map[string]int{}
	w.deps.Body = func(ctx context.Context, id string) (mail.MessageBody, error) {
		fetched[id]++
		return inner(ctx, id)
	}

	drive(t, w)

	got := inspectPostgres(t, postgres.URL(t), w.account)
	for _, id := range []string{"m01", "m02", "m03"} {
		if st := got.Messages[id]; st.State != "refused" || st.Decision != "SCAN" || len(st.Flags) != 0 {
			t.Errorf("message %s is %+v, want refused with its scan decision recorded and no flags", id, st)
		}
	}
	if got.Messages["m04"].State != "scanned" || !got.Ended {
		t.Errorf("the pass did not go on to scan the next message and end: %+v, ended %v", got.Messages["m04"], got.Ended)
	}
	type item struct {
		ID, Class, Disposition string
		Bound                  int32
	}
	var items []item
	rows, err := superuser(t).Query(context.Background(), `SELECT f.item_id, f.error_class, f.disposition, m.refused_bound
		FROM job_run_failures AS f JOIN messages AS m ON m.account_id = f.account_id AND m.message_id = f.item_id
		WHERE f.account_id = $1 AND f.item_kind = 'message' ORDER BY f.item_id`, w.account)
	if err != nil {
		t.Fatal(err)
	}
	var it item
	if _, err := pgx.ForEachRow(rows, []any{&it.ID, &it.Class, &it.Disposition, &it.Bound}, func() error { items = append(items, it); return nil }); err != nil {
		t.Fatal(err)
	}
	wantItems := []item{
		{ID: "m01", Class: "refused", Disposition: "refused", Bound: mail.MaxBodyBytes},
		{ID: "m02", Class: "refused", Disposition: "refused", Bound: mail.MaxBodyBytes},
		{ID: "m03", Class: "refused", Disposition: "refused", Bound: mail.MaxBodyBytes},
	}
	if diff := cmp.Diff(wantItems, items, compare.Options); diff != "" {
		t.Errorf("the refused messages' failed items (-want +got):\n%s", diff)
	}
	if r, ok := got.latest(); !ok || r.Progress.Counters.Refused != 3 || r.Progress.Counters.Pending != 0 {
		t.Errorf("the run's counters are %+v, want three refused and none pending", r.Progress.Counters)
	}

	if _, err := superuser(t).Exec(context.Background(), "UPDATE account_state SET backfill_pass2_complete = false WHERE account_id = $1", w.account); err != nil {
		t.Fatal(err)
	}
	before := maps.Clone(fetched)
	drive(t, w)
	if diff := cmp.Diff(before, fetched, compare.Options); diff != "" {
		t.Errorf("a later run fetched a body again (-before +after):\n%s", diff)
	}
}

// VERIFICATIONS' row for a refusal's return to pending. The run-start step returns a body refused
// under another size bound than the port's to pending and reopens the pass, so the next run fetches it
// again under the bound in force and records its new bound, and leaves a refusal under the bound in
// force alone. A rule restricting a refused message's sender, added and then lifted, leaves the
// message refused through the delisting transition, which returns only the messages skipped as
// restricted, so it is not fetched again. The sender's other message waits while the rule is added,
// is skipped as restricted, and returns to pending at the lift, so the transition runs for the
// sender's domain (ADR-0132, ADR-0136, ADR-0037).
func TestARefusalReturnsToPendingOnlyWhenItsBoundChanges(t *testing.T) {
	over := "<p>" + strings.Repeat("a", mail.MaxBodyBytes) + "</p>"
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("bound"), over),
		message("m02", "b@news.example", false, marker.Body("restricted"), over),
		message("m03", "c@other.example", false, marker.Body("kept"), over),
		message("m04", "d@news.example", false, marker.Body("skipped"), "<p>small</p>"),
	}
	w := realWorldOf(t, 3, messages)
	inner := w.deps.Body
	fetched := map[string]int{}
	w.deps.Body = func(ctx context.Context, id string) (mail.MessageBody, error) {
		fetched[id]++
		return inner(ctx, id)
	}
	bound := func(id string) *int32 {
		t.Helper()
		var b *int32
		if err := superuser(t).QueryRow(context.Background(), "SELECT refused_bound FROM messages WHERE account_id = $1 AND message_id = $2", w.account, id).Scan(&b); err != nil {
			t.Fatal(err)
		}
		return b
	}

	drive(t, w)
	if _, err := superuser(t).Exec(context.Background(), "UPDATE messages SET refused_bound = 1024 WHERE account_id = $1 AND message_id = 'm01'", w.account); err != nil {
		t.Fatal(err)
	}
	if _, err := superuser(t).Exec(context.Background(), "UPDATE messages SET scan_state = 'pending', content_flags = '{}', rule_ids = '{}', scanned_at = NULL, "+
		"scanner_version = NULL, scanner_revision = NULL WHERE account_id = $1 AND message_id = 'm04'", w.account); err != nil {
		t.Fatal(err)
	}
	w.added = []string{"news.example"}
	r, err := pass2.Reopen(context.Background(), w.deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if r.Refusals != 1 {
		t.Errorf("the run-start step returned %d refusals to pending, want the one made under another bound", r.Refusals)
	}
	if got := inspectPostgres(t, postgres.URL(t), w.account); got.Ended || got.Messages["m01"].State != "pending" || got.Messages["m03"].State != "refused" {
		t.Errorf("after the run-start step the pass is ended %v, m01 %q and m03 %q, want reopened, pending and refused",
			got.Ended, got.Messages["m01"].State, got.Messages["m03"].State)
	}
	drive(t, w)
	if b := bound("m01"); b == nil || *b != mail.MaxBodyBytes {
		t.Errorf("m01's bound after it was refused again is %v, want %d", b, mail.MaxBodyBytes)
	}

	w.added = nil
	if _, err := superuser(t).Exec(context.Background(), "UPDATE account_state SET backfill_pass2_complete = false WHERE account_id = $1", w.account); err != nil {
		t.Fatal(err)
	}
	drive(t, w)
	if st := inspectPostgres(t, postgres.URL(t), w.account).Messages["m02"]; st.State != "refused" {
		t.Errorf("after a rule restricting its sender was added and lifted m02 is %q, want still refused", st.State)
	}
	if b := bound("m02"); b == nil || *b != mail.MaxBodyBytes {
		t.Errorf("m02's bound after its sender's rule was lifted is %v, want %d", b, mail.MaxBodyBytes)
	}
	if st := inspectPostgres(t, postgres.URL(t), w.account).Messages["m04"]; st.State != "scanned" {
		t.Errorf("after its sender's rule was lifted m04 is %q, want scanned once it returned to pending", st.State)
	}
	if diff := cmp.Diff(map[string]int{"m01": 2, "m02": 1, "m03": 1, "m04": 2}, fetched, compare.Options); diff != "" {
		t.Errorf("the bodies asked for (-want +got):\n%s", diff)
	}
}

// What the pass does when a body fetch fails. A body throttled once is asked for again and recorded as
// recovered, a message the provider no longer has is recorded gone and one it keeps failing abandoned,
// both left pending while the pass goes on and ends, and a refused credential stops the run with the
// message recorded (ADR-0117, ADR-0025).
func TestAFailedBodyFetch(t *testing.T) {
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("throttled"), ""),
		message("m02", "b@shop.example", false, marker.Body("gone"), ""),
		message("m03", "c@shop.example", false, marker.Body("failing"), ""),
		message("m04", "d@shop.example", false, marker.Body("fine"), ""),
	}
	w := realWorldOf(t, 4, messages)
	inner := w.deps.Body
	throttled := false
	w.deps.Body = func(ctx context.Context, id string) (mail.MessageBody, error) {
		switch {
		case id == "m01" && !throttled:
			throttled = true
			return mail.MessageBody{}, mail.ThrottleError{Signal: mail.ThrottleSignal{RetryAfterMillis: 1, HasRetryAfter: true}}
		case id == "m02":
			return mail.MessageBody{}, fmt.Errorf("fake: %w", mail.ErrNotFound)
		case id == "m03":
			return mail.MessageBody{}, fmt.Errorf("fake: %w", mail.ErrProvider)
		}
		return inner(ctx, id)
	}

	drive(t, w)

	got := inspectPostgres(t, postgres.URL(t), w.account)
	states := map[string]string{}
	for id, m := range got.Messages {
		states[id] = m.State
	}
	if diff := cmp.Diff(map[string]string{"m01": "scanned", "m02": "pending", "m03": "pending", "m04": "scanned"}, states, compare.Options); diff != "" {
		t.Errorf("the scan states (-want +got):\n%s", diff)
	}
	want := map[string][]string{"m01": {"recovered"}, "m02": {"gone"}, "m03": {"abandoned"}}
	if diff := cmp.Diff(want, got.Items, compare.Options); diff != "" {
		t.Errorf("the failed items (-want +got):\n%s", diff)
	}

	stopped := realWorldOf(t, 4, messages[:1])
	stopped.deps.Body = func(context.Context, string) (mail.MessageBody, error) {
		return mail.MessageBody{}, fmt.Errorf("fake: %w", mail.ErrAuthentication)
	}
	stopped.open(t)
	if _, err := stopped.pass.Next(context.Background()); !errors.Is(err, mail.ErrAuthentication) {
		t.Errorf("Next returned %v, want the refused credential", err)
	}
	got = inspectPostgres(t, postgres.URL(t), stopped.account)
	if latest, _ := got.latest(); latest.State != "failed" || !slices.Equal(got.Items["m01"], []string{"abandoned"}) {
		t.Errorf("after a refused credential the run is %+v with items %v, want failed with the message abandoned", latest, got.Items)
	}
}

// releaseReason returns why the Redaction Gate releases or withholds a stored message's body under the
// world's policy, from the stored flags and scan state, as the API server decides it (ADR-0002).
func releaseReason(t *testing.T, w *world, id string, st stored) redact.Reason {
	t.Helper()
	states := map[string]sensitivity.ScanState{
		"pending": sensitivity.Pending(), "scanned": sensitivity.Scanned(),
		"skipped_gate": sensitivity.SkippedGate(), "skipped_restricted": sensitivity.SkippedRestricted(),
	}
	flags := sensitivity.Flags(slices.Contains(st.Flags, "mfa_code"), slices.Contains(st.Flags, "login_link"))
	var from string
	for _, m := range w.messages {
		if m.Metadata.ID == id {
			from = m.Metadata.From.Email
		}
	}
	return redact.Decide(classify.Prepare(policyListing(t, w.delisted).For(w.account), lookups), from, flags, states[st.State]).Reason()
}

// VERIFICATIONS' row for removing a fixture sender from the sensitive list. A run stops after the page
// that skipped the bank's messages as restricted. The bank's rule is removed, as any path might remove
// it, and the next run's delisting transition returns the bank's messages skipped as restricted to
// pending scan before its first page, and starts over, since they sit before its checkpoint. A body
// requested in that state is denied as pending its content scan rather than as a restricted sender's. Once the pass scans
// them, a clean body is releasable and a body holding a code is withheld for its flag (ADR-0037).
func TestARemovedRuleReturnsItsSendersMessagesToPendingScan(t *testing.T) {
	messages := []fake.Message{
		message("m01", "alerts@bank.example", false, marker.Body("bankclean")+" Your statement is ready.", ""),
		message("m02", "codes@bank.example", false, marker.Body("bankcode")+" "+code, ""),
		message("m03", "a@shop.example", false, marker.Body("shop"), ""),
		message("m04", "b@shop.example", false, marker.Body("shoplater"), ""),
	}
	w := realWorldOf(t, 2, messages)
	w.open(t)
	if _, err := w.pass.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := inspectPostgres(t, postgres.URL(t), w.account)
	if r := releaseReason(t, w, "m01", before.Messages["m01"]); r != redact.SkippedRestricted {
		t.Fatalf("before the removal the bank's body is denied as %v, want skipped as restricted", r)
	}

	w.delisted = true
	w.open(t)

	marked := inspectPostgres(t, postgres.URL(t), w.account)
	for _, id := range []string{"m01", "m02"} {
		st := marked.Messages[id]
		if st.State != "pending" {
			t.Errorf("after the transition message %s is %+v, want it pending scan", id, st)
		}
		if r := releaseReason(t, w, id, st); r != redact.PendingScan {
			t.Errorf("after the transition message %s is denied as %v, want pending its content scan", id, r)
		}
	}
	if latest, _ := marked.latest(); latest.Progress.Checkpoint != (core.Checkpoint{}) {
		t.Errorf("after the transition the run starts from %+v, want the first waiting message", latest.Progress.Checkpoint)
	}
	for {
		s, err := w.pass.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if s.Done {
			break
		}
	}
	after := inspectPostgres(t, postgres.URL(t), w.account)
	if r := releaseReason(t, w, "m01", after.Messages["m01"]); r != redact.Released {
		t.Errorf("once scanned the bank's clean body is decided %v, want released", r)
	}
	if r := releaseReason(t, w, "m02", after.Messages["m02"]); r != redact.ContentFlagged {
		t.Errorf("once scanned the bank's body holding a code is decided %v, want withheld for its flag", r)
	}
	if diff := cmp.Diff([]string{"m01", "m02", "m03", "m04"}, w.fetched, compare.Options); diff != "" {
		t.Errorf("the bodies asked for, the bank's only after the removal (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for removing a fixture sender from the sensitive list, through the scan state
// alone. Rules for the shop and for kept.example are added before the pass reaches their messages, so
// the bank's, the shop's and kept.example's messages are skipped as restricted while the newsletter's
// message holding a code is scanned with its flag. A rule then added for the newsletter leaves that
// verdict as it is, since the index stores no class to restrict (ADR-0136). The bank's, the shop's and
// the newsletter's rules are then lifted, and the next run's transition returns the bank's and the
// shop's messages skipped as restricted to pending scan, leaves kept.example's skipped, since its rule
// is still in place, and leaves the newsletter's scanned verdict and its flag as they were (ADR-0037).
func TestTheTransitionReturnsOnlyTheSkippedMessagesOfDelistedSenders(t *testing.T) {
	messages := []fake.Message{
		message("m01", "alerts@bank.example", false, marker.Body("bank"), ""),
		message("m02", "a@shop.example", false, marker.Body("shop"), ""),
		message("m03", "b@kept.example", false, marker.Body("kept"), ""),
		message("m04", "news@news.example", false, marker.Body("news")+" "+code, ""),
	}
	w := realWorldOf(t, 4, messages)
	w.added = []string{"shop.example", "kept.example"}
	w.open(t)
	if _, err := w.pass.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	state := func(id string) stored { return inspectPostgres(t, postgres.URL(t), w.account).Messages[id] }
	for _, id := range []string{"m01", "m02", "m03"} {
		if st := state(id); st.State != "skipped_restricted" {
			t.Fatalf("under the rules message %s is %+v, want it skipped as restricted", id, st)
		}
	}
	flagged := state("m04")
	if flagged.State != "scanned" || !slices.Equal(flagged.Flags, []string{"mfa_code"}) {
		t.Fatalf("the newsletter's message is %+v, want it scanned with its code flagged", flagged)
	}

	w.added = []string{"shop.example", "kept.example", "news.example"}
	w.open(t)
	if diff := cmp.Diff(flagged, state("m04"), compare.Options); diff != "" {
		t.Errorf("a rule added for the newsletter changed its scanned message (-want +got):\n%s", diff)
	}

	w.delisted, w.added = true, []string{"kept.example"}
	w.open(t)
	for id, want := range map[string]string{"m01": "pending", "m02": "pending", "m03": "skipped_restricted"} {
		if st := state(id); st.State != want {
			t.Errorf("after the lifts message %s is %+v, want it %s", id, st, want)
		}
	}
	if diff := cmp.Diff(flagged, state("m04"), compare.Options); diff != "" {
		t.Errorf("the lifts changed the newsletter's scanned message (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for removing a fixture sender from the sensitive list, a clean verdict. The
// shop's message is scanned clean before a rule restricts the shop, and the rule is then lifted. The
// message keeps its verdict through both, since the transition reads only the messages skipped as
// restricted (ADR-0037, ADR-0136). Its clean verdict carries no flag, so no check of the schema would
// refuse to return it to pending, and only the transition's own reading keeps it.
func TestTheTransitionLeavesACleanScannedVerdict(t *testing.T) {
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("shopclean"), ""),
		message("m02", "news@news.example", false, marker.Body("news"), ""),
	}
	w := realWorldOf(t, 2, messages)
	w.open(t)
	if _, err := w.pass.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	scanned := inspectPostgres(t, postgres.URL(t), w.account).Messages["m01"]
	if scanned.State != "scanned" || len(scanned.Flags) != 0 {
		t.Fatalf("the shop's message is %+v, want it scanned with no flag", scanned)
	}
	w.added = []string{"shop.example"}
	w.open(t)
	w.added = nil
	w.open(t)
	if diff := cmp.Diff(scanned, inspectPostgres(t, postgres.URL(t), w.account).Messages["m01"], compare.Options); diff != "" {
		t.Errorf("the shop's clean verdict after its rule was added and lifted (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for removing a fixture sender from the sensitive list, a rule added after the
// first pass. A rule listing the shop is added before the second pass reaches its messages, so the run
// skips the first of them as restricted, and only its skip state says the rule held it, since the
// index stores no class (ADR-0136). The rule is removed before the pass ends, and the next run's
// delisting transition finds the skipped message by its skip state, returns it to pending scan and
// starts over, and the pass scans it (ADR-0037).
func TestARuleAddedAndRemovedDuringThePassReachesTheTransition(t *testing.T) {
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("shopone"), ""),
		message("m02", "b@shop.example", false, marker.Body("shoptwo"), ""),
		message("m03", "c@other.example", false, marker.Body("other"), ""),
	}
	w := realWorldOf(t, 1, messages)
	load := func(rows ...policy.Row) classify.Policy {
		p, err := policy.Load(rows)
		if err != nil {
			t.Fatal(err)
		}
		return classify.Prepare(p.For(w.account), lookups)
	}
	w.deps.Policy = fixed(load(policy.Row{ID: "rule.shop", Class: policy.Restricted, DomainSuffixes: []string{"shop.example"}}))
	p, err := pass2.Open(context.Background(), w.deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := inspectPostgres(t, postgres.URL(t), w.account).Messages["m01"]; st.State != "skipped_restricted" {
		t.Fatalf("under the added rule m01 is %+v, want it skipped as restricted", st)
	}

	w.deps.Policy = fixed(load())
	p, err = pass2.Open(context.Background(), w.deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if st := inspectPostgres(t, postgres.URL(t), w.account).Messages["m01"]; st.State != "pending" {
		t.Errorf("after the removal m01 is %+v, want pending scan", st)
	}
	for {
		s, err := p.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if s.Done {
			break
		}
	}

	end := inspectPostgres(t, postgres.URL(t), w.account)
	if st := end.Messages["m01"]; st.State != "scanned" || st.Reason == "restricted" {
		t.Errorf("after the removal and the pass's end m01 is %+v, want scanned", st)
	}
	if diff := cmp.Diff([]string{"m01", "m02", "m03"}, w.fetched, compare.Options); diff != "" {
		t.Errorf("the bodies asked for, the shop's only after the removal (-want +got):\n%s", diff)
	}
}

// A second run of the transition finds nothing more to mark, so a run that marked nothing keeps its
// checkpoint. A rule still in place delists nothing (ADR-0037).
func TestTheTransitionMarksNothingTwice(t *testing.T) {
	messages := []fake.Message{
		message("m01", "alerts@bank.example", false, marker.Body("bank"), ""),
		message("m02", "a@shop.example", false, marker.Body("shop"), ""),
	}
	w := realWorldOf(t, 1, messages)
	w.open(t)
	if _, err := w.pass.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	w.open(t)
	if w.lastOpenDelisted {
		t.Errorf("a run under an unchanged policy delisted messages")
	}
	w.delisted = true
	w.open(t)
	if !w.lastOpenDelisted {
		t.Fatalf("a run after the removal delisted nothing")
	}
	w.open(t)
	if w.lastOpenDelisted {
		t.Errorf("a second run after the removal delisted the same messages again")
	}
	if latest, _ := inspectPostgres(t, postgres.URL(t), w.account).latest(); latest.Progress.Checkpoint.Page != 0 {
		t.Errorf("the run after the transition starts from %+v", latest.Progress.Checkpoint)
	}
}

// VERIFICATIONS' row for a change of scanner, the run-start step's part. Every subject stored unmasked
// under another scanner, more of them than one batch holds, is masked again from the store under the
// scanner in force, with the pair recorded and a masking event for each mask, and with no provider
// call, a subject the new scanner masks reading masked. A subject stored masked is left for the first
// pass to fetch again, so the step reopens the second pass and marks it to start over, and a second
// step under the same scanner masks nothing more (ADR-0120).
func TestTheRunStartMasksTheSubjectsStoredUnmaskedAgainFromTheStore(t *testing.T) {
	w := realWorldOf(t, 2, nil)
	conn := superuser(t)
	const unmasked = 5003
	must := func(sql string, args ...any) {
		t.Helper()
		if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
			t.Fatal(err)
		}
	}
	must(`INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, subject, subject_masked, sent_at, has_attachments,
			scan_state, subject_scanner_version, subject_scanner_revision)
		SELECT $1, 'm' || lpad(n::text, 5, '0'), 't' || n, 'news@news.example', 'news.example',
			$3 || CASE WHEN n % 1000 = 0 THEN ' Your verification code is 419283' ELSE ' number ' || n END, false, now(), false,
			'skipped_restricted', 1, 'a-revision'
		FROM generate_series(1, $2::int) AS n`, w.account, unmasked, marker.Field("subject"))
	must(`INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, subject, subject_masked, sent_at, has_attachments,
			scan_state, subject_scanner_version, subject_scanner_revision)
		VALUES ($1, 'x1', 'tx1', 'news@news.example', 'news.example', $2 || ' Sign in ██████', true, now(), false, 'skipped_restricted', 1, 'a-revision')`,
		w.account, marker.Field("subject"))
	must("UPDATE account_state SET backfill_pass2_complete = true, backfill_pass2_restart = false WHERE account_id = $1", w.account)

	deps := w.deps
	deps.Scanner = scannerAt(t, "revision-1")
	r, err := pass2.Reopen(t.Context(), deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if r.Subjects != unmasked {
		t.Errorf("the run-start step masked %d subjects again, want the %d stored unmasked", r.Subjects, unmasked)
	}
	type row struct {
		Subject  string
		Masked   bool
		Revision string
		Events   int
	}
	read := func(id string) row {
		t.Helper()
		var got row
		err := conn.QueryRow(t.Context(), `SELECT m.subject, m.subject_masked, m.subject_scanner_revision,
				(SELECT count(*) FROM masking_events AS e WHERE e.account_id = m.account_id AND e.message_id = m.message_id
					AND e.scanner_version = 1 AND e.scanner_revision = 'revision-1')
			FROM messages AS m WHERE m.account_id = $1 AND m.message_id = $2`, w.account, id).Scan(&got.Subject, &got.Masked, &got.Revision, &got.Events)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	for id, want := range map[string]row{
		"m00001": {Subject: marker.Field("subject") + " number 1", Revision: "revision-1"},
		"m03000": {Subject: marker.Field("subject") + " Your verification code is ██████", Masked: true, Revision: "revision-1", Events: 1},
		"m05003": {Subject: marker.Field("subject") + " number 5003", Revision: "revision-1"},
		"x1":     {Subject: marker.Field("subject") + " Sign in ██████", Masked: true, Revision: "a-revision"},
	} {
		if diff := cmp.Diff(want, read(id), compare.Options); diff != "" {
			t.Errorf("message %s after the run-start step (-want +got):\n%s", id, diff)
		}
	}
	var stale, events int
	if err := conn.QueryRow(t.Context(), `SELECT count(*) FILTER (WHERE subject_scanner_revision <> 'revision-1'),
			(SELECT count(*) FROM masking_events WHERE account_id = $1 AND scanner_revision = 'revision-1')
		FROM messages WHERE account_id = $1`, w.account).Scan(&stale, &events); err != nil {
		t.Fatal(err)
	}
	if stale != 1 || events != 5 {
		t.Errorf("after the run-start step %d subjects are stale and %d masking events are recorded under the new scanner, want the masked one stale and the five codes' events", stale, events)
	}
	var ended, restart bool
	if err := conn.QueryRow(t.Context(), "SELECT backfill_pass2_complete, backfill_pass2_restart FROM account_state WHERE account_id = $1", w.account).
		Scan(&ended, &restart); err != nil {
		t.Fatal(err)
	}
	if ended || !restart {
		t.Errorf("after the run-start step the second pass is ended %v and marked to start over %v, want it reopened and marked", ended, restart)
	}
	again, err := pass2.Reopen(t.Context(), deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if again.Subjects != 0 {
		t.Errorf("a second run-start step under the same scanner masked %d subjects again, want none", again.Subjects)
	}
}

// VERIFICATIONS' row for a change of scanner, the second pass's guard. A gate skip stored with a
// masked subject was decided without the signal that subject carries, so the second pass returns it to
// pending before its first page, while a skip whose subject is unmasked stands. The state arises when
// another job kind stores a subject unmasked under another scanner pair after the run-start step, and
// the first pass then fetches it and masks it again. Here it is set directly (ADR-0120, ADR-0093).
func TestAGateSkipWhoseSubjectIsMaskedReturnsToPendingAtTheSecondPassesStart(t *testing.T) {
	w := realWorldOf(t, 2, []fake.Message{
		message("s1", "news@news.example", true, marker.Body("one"), ""),
		message("s2", "news@news.example", true, marker.Body("two"), ""),
	})
	conn := superuser(t)
	if _, err := conn.Exec(t.Context(), `UPDATE messages SET scan_state = 'skipped_gate', subject_masked = (message_id = 's1')
		WHERE account_id = $1`, w.account); err != nil {
		t.Fatal(err)
	}
	p, err := pass2.Open(t.Context(), w.deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if p.Run() == "" {
		t.Fatalf("the second pass did not run")
	}
	got := map[string]string{}
	rows, err := conn.Query(t.Context(), "SELECT message_id, scan_state FROM messages WHERE account_id = $1", w.account)
	if err != nil {
		t.Fatal(err)
	}
	var id, state string
	if _, err := pgx.ForEachRow(rows, []any{&id, &state}, func() error { got[id] = state; return nil }); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(map[string]string{"s1": "pending", "s2": "skipped_gate"}, got, compare.Options); diff != "" {
		t.Errorf("the scan states before the first page (-want +got):\n%s", diff)
	}
}
