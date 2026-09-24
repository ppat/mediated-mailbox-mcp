package fake_test

import (
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/contract"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
)

type changes struct{ Added, Modified, Removed []string }

func changesSince(t *testing.T, f *fake.Fake, c mail.Cursor) (changes, error) {
	t.Helper()
	got, err := f.ChangesSince(t.Context(), c)
	return changes{got.Added, got.Modified, got.Removed}, err
}

func cursor(t *testing.T, f *fake.Fake) mail.Cursor {
	t.Helper()
	c, err := f.CurrentCursor(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func delivered(key string) fake.Message {
	r := fixture.Receipt()
	return fake.Message{
		Metadata: mail.MessageMetadata{ID: key, ThreadID: key, From: mail.Address{Email: r.FromAddress}, Subject: r.Subject},
		Body:     mail.MessageBody{Text: r.Body},
	}
}

// Mail the test delivers is added, mail it removes is removed, and a message both delivered and
// removed after the cursor is only removed.
func TestDeliveryAndRemovalAreChanges(t *testing.T) {
	f := newFake(t, contract.Account, contract.Mailbox())
	c := cursor(t, f)
	for _, step := range []error{
		f.Deliver(delivered("late")),
		f.Deliver(delivered("brief")),
		f.Remove("brief"),
		f.Remove("bank"),
	} {
		if step != nil {
			t.Fatal(step)
		}
	}
	got, err := changesSince(t, f, c)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(changes{Added: []string{"late"}, Removed: []string{"brief", "bank"}}, got, compare.Options); diff != "" {
		t.Errorf("changes (-want +got):\n%s", diff)
	}
	if err := f.Remove("bank"); !errors.Is(err, mail.ErrNotFound) {
		t.Errorf("removing a removed message returned %v, want an error wrapping %v", err, mail.ErrNotFound)
	}
	if err := f.Deliver(delivered("late")); err == nil {
		t.Error("delivering a message whose identifier the mailbox holds succeeded")
	}
}

// Expiring cursors turns every cursor issued before into a gap, and leaves the current one usable.
func TestExpiredCursorsAreGaps(t *testing.T) {
	f := newFake(t, contract.Account, contract.Mailbox())
	old := cursor(t, f)
	if err := f.Deliver(delivered("late")); err != nil {
		t.Fatal(err)
	}
	f.ExpireCursors()
	if _, err := changesSince(t, f, old); !errors.Is(err, mail.ErrCursorGap) {
		t.Errorf("an expired cursor returned %v, want an error wrapping %v", err, mail.ErrCursorGap)
	}
	got, err := changesSince(t, f, cursor(t, f))
	if err != nil || len(got.Added)+len(got.Modified)+len(got.Removed) != 0 {
		t.Errorf("the current cursor returned %+v, %v, want no changes", got, err)
	}
}

// The fake derives the snippet and the size as a provider would, and keeps any it was given.
func TestTheFakeDerivesSnippetAndSize(t *testing.T) {
	text := "0123456789"
	for range 11 {
		text += "0123456789"
	}
	given := fake.Message{
		Metadata: mail.MessageMetadata{ID: "given", ThreadID: "given", Subject: "s", Snippet: "kept", SizeBytes: 7},
		Body:     mail.MessageBody{Text: text},
	}
	derived := fake.Message{
		Metadata: mail.MessageMetadata{ID: "derived", ThreadID: "derived", Subject: "s", From: mail.Address{Email: "a@b.example"}},
		Body:     mail.MessageBody{Text: text, HTML: "<p>x</p>"},
	}
	f, err := fake.New(fake.Config{Account: "a"}, given, derived)
	if err != nil {
		t.Fatal(err)
	}
	got, err := f.GetMessageMetadata(t.Context(), []string{"given", "derived"})
	if err != nil {
		t.Fatal(err)
	}
	type derivation struct {
		Snippet string
		Size    int64
	}
	want := []derivation{{"kept", 7}, {text[:100], 1 + 11 + 120 + 8}}
	var observed []derivation
	for _, m := range got {
		observed = append(observed, derivation{m.Snippet, m.SizeBytes})
	}
	if diff := cmp.Diff(want, observed, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// A mailbox the fake cannot hold is refused when it is built.
func TestTheFakeRefusesAnInvalidMailbox(t *testing.T) {
	ok := fake.Message{Metadata: mail.MessageMetadata{ID: "m", ThreadID: "t"}}
	cases := []struct {
		name     string
		cfg      fake.Config
		messages []fake.Message
	}{
		{"no account", fake.Config{}, []fake.Message{ok}},
		{"a message with no identifier", fake.Config{Account: "a"}, []fake.Message{{Metadata: mail.MessageMetadata{ThreadID: "t"}}}},
		{"a message with no thread", fake.Config{Account: "a"}, []fake.Message{{Metadata: mail.MessageMetadata{ID: "m"}}}},
		{"two messages with one identifier", fake.Config{Account: "a"}, []fake.Message{ok, ok}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := fake.New(c.cfg, c.messages...); err == nil {
				t.Error("New succeeded")
			}
		})
	}
}

// A value returned by the fake shares nothing with the mailbox, so editing it changes nothing there.
func TestReturnedValuesShareNothing(t *testing.T) {
	f := newFake(t, contract.Account, contract.Mailbox())
	got, err := f.GetMessageMetadata(t.Context(), []string{"bank"})
	if err != nil || len(got) != 1 {
		t.Fatalf("GetMessageMetadata returned %v, %v", got, err)
	}
	want := append([]string(nil), got[0].Labels...)
	got[0].Labels[0] = "edited"
	again, err := f.GetMessageMetadata(t.Context(), []string{"bank"})
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, again[0].Labels, compare.Options); diff != "" {
		t.Errorf("labels after editing a returned value (-want +got):\n%s", diff)
	}
}

// clock is a test clock that advances only when told.
type clock struct{ now time.Time }

func (c *clock) read() time.Time { return c.now }

var signal = mail.ThrottleSignal{RetryAfterMillis: 2000, HasRetryAfter: true, Scope: mail.ScopePerUser}

// outcome is what a caller sees of one call through a schedule.
type outcome struct {
	Throttled bool
	Signal    mail.ThrottleSignal
}

func observe(err error) outcome {
	sig, ok := mail.Throttled(err)
	return outcome{Throttled: ok, Signal: sig}
}

var (
	through   = outcome{}
	throttled = outcome{Throttled: true, Signal: signal}
)

// ThrottleCalls throttles exactly the calls it names, and the calls after them recover.
func TestThrottleCallsThrottlesTheNamedCalls(t *testing.T) {
	c := &clock{now: time.Unix(0, 0)}
	p := fake.Throttle(newFake(t, contract.Account, contract.Mailbox()), fake.ThrottleCalls(2, 3, signal), c.read)
	var got []outcome
	for range 5 {
		_, err := p.ListLabels(t.Context())
		got = append(got, observe(err))
	}
	if diff := cmp.Diff([]outcome{through, throttled, throttled, through, through}, got, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// A throttled call reaches nothing, so a throttled mutation changes nothing.
func TestAThrottledCallHasNoEffect(t *testing.T) {
	f := newFake(t, contract.Account, contract.Mailbox())
	c := &clock{now: time.Unix(0, 0)}
	p := fake.Throttle(f, fake.ThrottleCalls(1, 1, signal), c.read)
	star, err := mail.StarOp("code")
	if err != nil {
		t.Fatal(err)
	}
	before := cursor(t, f)
	if _, err := p.Mutate(t.Context(), []mail.MutationOp{star}); !errors.Is(err, mail.ErrThrottled) {
		t.Fatalf("the throttled Mutate returned %v, want an error wrapping %v", err, mail.ErrThrottled)
	}
	got, err := changesSince(t, f, before)
	if err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(changes{}, got, compare.Options); diff != "" {
		t.Errorf("changes after a throttled mutation (-want +got):\n%s", diff)
	}
}

// Ceiling lets through what fits in the trailing second, throttles the rest without counting it, and
// lets calls through again as the second passes. The fake weighs a call one unit per message it
// names.
func TestCeilingThrottlesAboveTheRate(t *testing.T) {
	c := &clock{now: time.Unix(0, 0)}
	p := fake.Throttle(newFake(t, contract.Account, contract.Mailbox()), fake.Ceiling(5, signal), c.read)
	call := func(messages int) outcome {
		ids := make([]string, messages)
		_, err := p.GetMessageMetadata(t.Context(), ids)
		return observe(err)
	}
	type step struct {
		advance  time.Duration
		messages int
	}
	steps := []step{
		{0, 3},                      // 3 of 5 spent
		{100 * time.Millisecond, 2}, // 5 of 5 spent
		{100 * time.Millisecond, 1}, // would be 6, throttled
		{700 * time.Millisecond, 1}, // still 5 in the second before, throttled
		{150 * time.Millisecond, 1}, // the first call has aged out, 3 of 5 spent
		{0, 2},                      // 5 of 5 spent
		{0, 6},                      // heavier than the ceiling, throttled
		{2 * time.Second, 5},        // everything aged out
	}
	var got []outcome
	for _, s := range steps {
		c.now = c.now.Add(s.advance)
		got = append(got, call(s.messages))
	}
	want := []outcome{through, through, throttled, throttled, through, through, throttled, through}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}

// The same calls at the same times meet the same decisions, so a schedule is deterministic.
func TestASchedulesDecisionsRepeat(t *testing.T) {
	run := func() []outcome {
		c := &clock{now: time.Unix(0, 0)}
		p := fake.Throttle(newFake(t, contract.Account, contract.Mailbox()), fake.Ceiling(3, signal), c.read)
		var out []outcome
		for i := range 20 {
			c.now = c.now.Add(time.Duration(i*37%250) * time.Millisecond)
			_, err := p.GetMessageMetadata(t.Context(), make([]string, i%3))
			out = append(out, observe(err))
		}
		return out
	}
	first := run()
	if diff := cmp.Diff(first, run(), compare.Options); diff != "" {
		t.Errorf("a second run (-first +second):\n%s", diff)
	}
}

// A schedule sees each call's number, operation, cost and time.
func TestAScheduleSeesEachCall(t *testing.T) {
	c := &clock{now: time.Unix(10, 0)}
	var seen []fake.Call
	record := func(call fake.Call) error {
		seen = append(seen, call)
		return nil
	}
	p := fake.Throttle(newFake(t, contract.Account, contract.Mailbox()), record, c.read)
	if _, err := p.GetMessageMetadata(t.Context(), []string{"bank", "code"}); err != nil {
		t.Fatal(err)
	}
	if _, err := p.EnumerateAll(t.Context(), ""); err != nil {
		t.Fatal(err)
	}
	want := []fake.Call{
		{Seq: 1, Op: mail.ProviderOp{Operation: mail.OpGetMessageMetadata, Messages: 2}, Cost: mail.OpCost{Weight: 2, OpsCount: 2}, At: time.Unix(10, 0)},
		{Seq: 2, Op: mail.ProviderOp{Operation: mail.OpEnumerateAll}, Cost: mail.OpCost{Weight: 1}, At: time.Unix(10, 0)},
	}
	if diff := cmp.Diff(want, seen, compare.Options); diff != "" {
		t.Errorf("(-want +got):\n%s", diff)
	}
}
