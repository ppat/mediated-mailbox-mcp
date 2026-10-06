//go:build integration

package pass2_test

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"

	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/redact"
	"github.com/ppat/mediated-mailbox-mcp/core/sensitivity"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// message returns one fixture message from address, with a List-Id when listID is set, whose text part
// is text, and whose HTML part is html.
func message(id, address string, listID bool, text, html string) fake.Message {
	m := fake.Message{
		Metadata: mail.MessageMetadata{
			ID: id, ThreadID: "t" + id, From: mail.Address{Email: address}, Subject: marker.Field("subject"),
			Date: mail.UnixMilli(1_700_000_000_000),
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
// and the pass sets its completion flag (ADR-0093, ADR-0008, ADR-0009, ADR-0022).
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
		"m01": {Domain: "bank.example", Class: "restricted", State: "skipped_restricted", Flags: []string{}, Rules: []string{}, Decision: "SKIP", Reason: "restricted"},
		"m02": {Domain: "news.example", Class: "normal", State: "skipped_gate", Flags: []string{}, Rules: []string{}, Decision: "SKIP", Reason: "high_volume_no_hits"},
		"m03": {Domain: "news.example", Class: "normal", State: "skipped_gate", Flags: []string{}, Rules: []string{}, Decision: "SKIP", Reason: "high_volume_no_hits"},
		"m04": {Domain: "news.example", Class: "normal", State: "skipped_gate", Flags: []string{}, Rules: []string{}, Decision: "SKIP", Reason: "high_volume_no_hits"},
		"m05": {
			Domain: "shop.example", Class: "normal", State: "scanned", Flags: []string{"mfa_code"}, Rules: []string{"mfa.trigger_window"},
			Version: 1, Revision: "a-revision", Decision: "SCAN", Reason: "default",
		},
		"m06": {
			Domain: "shop.example", Class: "normal", State: "scanned", Flags: []string{}, Rules: []string{},
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
	wantProgress.Checkpoint.Page, wantProgress.Checkpoint.After = 3, "m06"
	wantProgress.Counters.Pages, wantProgress.Counters.Decided, wantProgress.Counters.Scanned, wantProgress.Counters.Skipped = 3, 6, 2, 4
	if latest.State != "succeeded" || latest.Progress != wantProgress {
		t.Errorf("the latest run is %+v, want one that succeeded at %+v", latest, wantProgress)
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

// VERIFICATIONS' row for scanning both parts of a body. A code only in the text part of a body whose
// HTML part is clean flags the message, and so does a code only in the HTML part of a body whose text
// part is clean (ADR-0017).
func TestBothPartsOfABodyAreScanned(t *testing.T) {
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("texta")+" "+code, "<p>"+marker.Body("htmla")+" Welcome.</p>"),
		message("m02", "b@shop.example", false, marker.Body("textb")+" Welcome.", "<p>"+marker.Body("htmlb")+" "+code+"</p>"),
		message("m03", "c@shop.example", false, marker.Body("textc")+" Welcome.", "<p>"+marker.Body("htmlc")+" Welcome.</p>"),
	}
	w := realWorldOf(t, 3, messages)

	drive(t, w)

	flags := map[string][]string{}
	for id, m := range inspectPostgres(t, postgres.URL(t), w.account).Messages {
		flags[id] = m.Flags
	}
	want := map[string][]string{"m01": {"mfa_code"}, "m02": {"mfa_code"}, "m03": {}}
	if diff := cmp.Diff(want, flags, compare.Options); diff != "" {
		t.Errorf("the content flags (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for a body the conversion refuses. An HTML part one byte over the conversion's
// limit is never scanned, its message stays pending with a failed item of kind message, class
// validation and disposition abandoned, and the pass goes on to scan the next message and end
// (ADR-0017, ADR-0036, ADR-0074).
func TestABodyTheConversionRefusesStaysPending(t *testing.T) {
	huge := "<p>" + strings.Repeat("a", 512*1024-6) + "</p>"
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("refused"), huge),
		message("m02", "b@shop.example", false, marker.Body("next"), ""),
	}
	w := realWorldOf(t, 2, messages)

	drive(t, w)

	got := inspectPostgres(t, postgres.URL(t), w.account)
	if st := got.Messages["m01"]; st.State != "pending" || st.Decision != "SCAN" {
		t.Errorf("the refused message is %+v, want pending with its scan decision recorded", st)
	}
	if got.Messages["m02"].State != "scanned" || !got.Ended {
		t.Errorf("the pass did not go on to scan the next message and end: %+v, ended %v", got.Messages["m02"], got.Ended)
	}
	var class, disposition string
	err := superuser(t).QueryRow(context.Background(),
		"SELECT error_class, disposition FROM job_run_failures WHERE account_id = $1 AND item_kind = 'message' AND item_id = 'm01'", w.account).
		Scan(&class, &disposition)
	if err != nil || class != "validation" || disposition != "abandoned" {
		t.Errorf("the refused message's failed item is %q, %q (%v), want validation, abandoned", class, disposition, err)
	}
}

// What the pass does when a body fetch fails. A body throttled once is asked for again and recorded as
// recovered, a message the provider no longer has is recorded gone and one it keeps failing abandoned,
// both left pending while the pass goes on and ends, and a refused credential stops the run with the
// message recorded (ADR-0022, ADR-0025).
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
// world's policy, from the stored flags and scan state, as the mediator decides it (ADR-0002).
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
	return redact.Decide(policyListing(t, w.delisted).For(w.account), from, lookups, flags, states[st.State]).Reason()
}

// VERIFICATIONS' row for removing a fixture sender from the sensitive list. A run stops after the page
// that skipped the bank's messages as restricted. The bank's rule is removed, as any path might remove
// it, and the next run's delisting transition returns the bank's messages to a normal sender class and
// to pending scan before its first page, and starts over, since they sit before its checkpoint. They
// stop naming the removed rule as the one that set their class (ADR-0016). A body requested in that
// state is denied as pending its content scan rather than as a restricted sender's. Once the pass scans
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
	rule := "rule.bank"
	if diff := cmp.Diff(map[string]*string{"m01": &rule, "m02": &rule, "m03": nil, "m04": nil}, classRules(t, w.account), compare.Options); diff != "" {
		t.Fatalf("before the removal the rules that set the classes (-want +got):\n%s", diff)
	}

	w.delisted = true
	w.open(t)

	marked := inspectPostgres(t, postgres.URL(t), w.account)
	for _, id := range []string{"m01", "m02"} {
		st := marked.Messages[id]
		if st.Class != "normal" || st.State != "pending" {
			t.Errorf("after the transition message %s is %+v, want a normal sender pending scan", id, st)
		}
		if r := releaseReason(t, w, id, st); r != redact.PendingScan {
			t.Errorf("after the transition message %s is denied as %v, want pending its content scan", id, r)
		}
	}
	if diff := cmp.Diff(map[string]*string{"m01": nil, "m02": nil, "m03": nil, "m04": nil}, classRules(t, w.account), compare.Options); diff != "" {
		t.Errorf("after the transition the rules that set the classes, none (-want +got):\n%s", diff)
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

// TestAnAddedRuleRestrictsTheStoredClasses is ADR-0113's comparison in the second pass. A rule added
// for a sender the index stores as normal, after the pass decided some of its messages, restricts every
// one of its messages stored as normal with the rule, rebuilds the sender's statistics, and leaves each
// scan state and the run's checkpoint as they were. A second comparison marks nothing, and neither does
// one under a policy that never loaded.
func TestAnAddedRuleRestrictsTheStoredClasses(t *testing.T) {
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("shopfirst"), ""),
		message("m02", "b@shop.example", false, marker.Body("shopsecond"), ""),
		message("m03", "c@shop.example", false, marker.Body("shoplater"), ""),
		message("m04", "news@news.example", false, marker.Body("news"), ""),
	}
	w := realWorldOf(t, 2, messages)
	w.open(t)
	if _, err := w.pass.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	before := inspectPostgres(t, postgres.URL(t), w.account)
	checkpoint, _ := before.latest()
	if before.Messages["m01"].State != "scanned" || before.Messages["m03"].State != "pending" {
		t.Fatalf("before the rule the first page left %+v and %+v, want the first scanned and the third pending", before.Messages["m01"], before.Messages["m03"])
	}

	w.added = []string{"shop.example"}
	w.open(t)

	after := inspectPostgres(t, postgres.URL(t), w.account)
	rule := "rule.shop.example"
	if diff := cmp.Diff(map[string]*string{"m01": &rule, "m02": &rule, "m03": &rule, "m04": nil}, classRules(t, w.account), compare.Options); diff != "" {
		t.Errorf("after the comparison the rules that set the classes (-want +got):\n%s", diff)
	}
	for id, st := range after.Messages {
		want := "restricted"
		if id == "m04" {
			want = "normal"
		}
		if st.Class != want || st.State != before.Messages[id].State {
			t.Errorf("after the comparison message %s is %+v, want class %s and the scan state %s it had", id, st, want, before.Messages[id].State)
		}
	}
	var senderClass string
	if err := superuser(t).QueryRow(context.Background(), "SELECT sender_class FROM senders WHERE account_id = $1 AND domain = 'shop.example'", w.account).Scan(&senderClass); err != nil || senderClass != "restricted" {
		t.Errorf("the shop's statistics read %q with error %v, want restricted", senderClass, err)
	}
	if latest, _ := after.latest(); latest.Progress.Checkpoint != checkpoint.Progress.Checkpoint {
		t.Errorf("after the comparison the run resumes from %+v, want %+v, since restricting needs no rescan", latest.Progress.Checkpoint, checkpoint.Progress.Checkpoint)
	}

	store := pass2.NewPostgres(backfillPool(t))
	for name, p := range map[string]policy.Composed{
		"a second comparison":           policyAdding(t, false, w.added).For(w.account),
		"a policy that never loaded":    {},
		"a policy without the new rule": policyListing(t, false).For(w.account),
	} {
		marked, err := store.List(context.Background(), w.account, func(normal []string) []index.Listing {
			return index.Listed(p, lookups, normal)
		})
		if err != nil || marked != 0 {
			t.Errorf("%s marked %d messages with error %v, want none", name, marked, err)
		}
	}
}

// classRules returns the rule each of the account's messages names as the one that set its class, nil
// for none.
func classRules(t testing.TB, account string) map[string]*string {
	t.Helper()
	rows, err := superuser(t).Query(context.Background(), "SELECT message_id, class_rule_id FROM messages WHERE account_id = $1", account)
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]*string{}
	var id string
	var rule pgtype.Text
	_, err = pgx.ForEachRow(rows, []any{&id, &rule}, func() error {
		out[id] = nil
		if rule.Valid {
			r := rule.String
			out[id] = &r
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// VERIFICATIONS' row for removing a fixture sender from the sensitive list, a rule added after the
// first pass. The first pass stores the shop as a normal sender. A rule listing it is added before the
// second pass reaches its messages, so the run restricts their stored class (ADR-0113) and skips the
// first of them as restricted. That message's stored class is then set back to normal, as delta sync
// stores a message under a policy it loaded before the rule, so only its skip state says the rule held
// it. The rule is removed before the pass ends, and the next run's delisting transition finds the
// skipped message by its skip state, returns it to pending scan and starts over, and the pass scans it
// (ADR-0037).
func TestARuleAddedAndRemovedDuringThePassReachesTheTransition(t *testing.T) {
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("shopone"), ""),
		message("m02", "b@shop.example", false, marker.Body("shoptwo"), ""),
		message("m03", "c@other.example", false, marker.Body("other"), ""),
	}
	w := realWorldOf(t, 1, messages)
	load := func(rows ...policy.Row) policy.Composed {
		p, err := policy.Load(rows)
		if err != nil {
			t.Fatal(err)
		}
		return p.For(w.account)
	}
	w.deps.Policy = load(policy.Row{ID: "rule.shop", Class: policy.Restricted, DomainSuffixes: []string{"shop.example"}})
	p, err := pass2.Open(context.Background(), w.deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := p.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	if st := inspectPostgres(t, postgres.URL(t), w.account).Messages["m01"]; st.Class != "restricted" || st.State != "skipped_restricted" {
		t.Fatalf("under the added rule m01 is %+v, want a sender stored restricted and skipped as restricted", st)
	}
	if _, err := superuser(t).Exec(context.Background(), "UPDATE messages SET sender_class = 'normal', class_rule_id = NULL WHERE account_id = $1", w.account); err != nil {
		t.Fatal(err)
	}

	w.deps.Policy = load()
	p, err = pass2.Open(context.Background(), w.deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	if st := inspectPostgres(t, postgres.URL(t), w.account).Messages["m01"]; st.Class != "normal" || st.State != "pending" {
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
