package contract

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// Account is the account the suite asks each harness to build its implementation for.
const Account = "contract"

// Harness returns a new implementation of the port, for account, over a mailbox holding exactly
// mailbox. The suite calls it once per case, and the cases mutate the mailbox, so each call starts
// from the same messages.
type Harness func(t *testing.T, account string, mailbox []Message) Implementation

// Implementation is the port under test, with the two things only the provider's side can do to a
// mailbox, so the suite can require new and deleted mail to reach delta sync (ADR-0018). The fake
// does them with its own test controls, and a real adapter's harness through the provider directly,
// outside the port.
type Implementation struct {
	Port mail.Port[context.Context]
	// Deliver adds m to the mailbox as newly arrived mail. The implementation gives it its own
	// identifier.
	Deliver func(m Message) error
	// Remove deletes the message with the identifier id from the mailbox, as a deletion the provider
	// reports among its changes.
	Remove func(id string) error
}

// Run runs every case of the contract against the implementations h builds.
func Run(t *testing.T, h Harness) {
	t.Helper()
	cases := []struct {
		name string
		run  func(*testing.T, *subject)
	}{
		{"enumeration returns every message once with what was seeded", enumerationReturnsEveryMessage},
		{"enumeration resumes from any page token", enumerationResumes},
		{"enumeration refuses a page token it did not issue", enumerationRefusesAForeignToken},
		{"thread listing resumes from any page token", threadListingResumes},
		{"thread listing refuses a page token it did not issue", threadListingRefusesAForeignToken},
		{"message metadata comes back in the order asked, skipping unknown messages", messageMetadataInOrder},
		{"a thread comes back with every message, oldest first", threadHoldsEveryMessage},
		{"an unknown thread is not found", unknownThreadNotFound},
		{"a query selects exactly the threads it names", querySelectsThreads},
		{"a query no constructor completed is refused", invalidQueriesRefused},
		{"a body comes back exactly as seeded", bodyAsSeeded},
		{"an unknown message's body is not found", unknownBodyNotFound},
		{"every label is listed", everyLabelListed},
		{"ensuring a label creates it once", ensureLabelIsIdempotent},
		{"each verb changes exactly what it names", verbsChangeWhatTheyName},
		{"labelling with a label the account lacks changes nothing", labelMustExist},
		{"an op on an unknown message fails alone", unknownMessageOpFails},
		{"a batch holding an op no constructor built applies nothing", unbuiltOpRefused},
		{"no activity means no changes", noActivityNoChanges},
		{"a change is reported once, as a modification", changeReported},
		{"delivered mail is added and removed mail is removed", deliveryAndRemovalReported},
		{"a message delivered and then changed or removed is reported once", changesAreNetted},
		{"a cursor the implementation cannot read is a gap", foreignCursorIsAGap},
		{"the rate profile declares a budget and a cost for every operation", rateProfileDeclares},
		{"the rate profile recognises the port's throttle", rateProfileParsesThrottles},
		{"a cancelled call fails and changes nothing", cancelledCallsFail},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			c.run(t, newSubject(t, h))
		})
	}
}

// subject is the implementation under test, with the identifiers it gave the mailbox's messages,
// read through enumeration.
type subject struct {
	harness Harness
	impl    Implementation
	port    mail.Port[context.Context]
	ids     map[string]string
}

func newSubject(t *testing.T, h Harness) *subject {
	t.Helper()
	impl := h(t, Account, Mailbox())
	return &subject{harness: h, impl: impl, port: impl.Port}
}

// id returns the identifier the implementation gave the message seeded or delivered under key. It
// reads the mailbox again for a key it has not seen, such as a message delivered since.
func (s *subject) id(t *testing.T, key string) string {
	t.Helper()
	if _, ok := s.ids[key]; !ok {
		s.ids = map[string]string{}
		for k, m := range s.state(t) {
			s.ids[k] = m.ID
		}
	}
	id, ok := s.ids[key]
	if !ok {
		t.Fatalf("the mailbox has no message seeded as %q", key)
	}
	return id
}

// state returns every message in the mailbox, by the key it was seeded under.
func (s *subject) state(t *testing.T) map[string]mail.MessageMetadata {
	t.Helper()
	items, _ := enumerate(t, s.port, "")
	return byKey(t, items)
}

// enumerate pages through EnumerateAll from page, returning the messages and each page's Next. It
// fails the test when the pages do not end within one more page than the mailbox has messages.
func enumerate(t *testing.T, p mail.Port[context.Context], page mail.PageToken) ([]mail.MessageMetadata, []mail.PageToken) {
	t.Helper()
	var items []mail.MessageMetadata
	var tokens []mail.PageToken
	for range len(Mailbox()) + 2 {
		got, err := p.EnumerateAll(t.Context(), page)
		if err != nil {
			t.Fatalf("EnumerateAll(%q): %v", page, err)
		}
		items = append(items, got.Items...)
		tokens = append(tokens, got.Next)
		if got.Next == "" {
			return items, tokens
		}
		page = got.Next
	}
	t.Fatalf("EnumerateAll returned more pages than the mailbox has messages")
	return nil, nil
}

// keys maps each seeded or delivered subject to the key its message was seeded under.
func keys() map[string]string {
	out := map[string]string{}
	for _, m := range append(Mailbox(), late(), brief()) {
		out[m.Metadata.Subject] = m.Metadata.ID
	}
	return out
}

func byKey(t *testing.T, items []mail.MessageMetadata) map[string]mail.MessageMetadata {
	t.Helper()
	subjects := keys()
	out := map[string]mail.MessageMetadata{}
	for _, m := range items {
		key, ok := subjects[m.Subject]
		if !ok {
			t.Fatalf("a message with the subject %q was never seeded", m.Subject)
		}
		if _, dup := out[key]; dup {
			t.Fatalf("the message seeded as %q came back twice", key)
		}
		out[key] = m
	}
	return out
}

// seeded returns what the seed decides of m. The identifiers, the account, the snippet, the size and
// the authentication results are the implementation's, label order carries no meaning, and an empty
// list is no list.
func seeded(m mail.MessageMetadata) mail.MessageMetadata {
	m.AccountID, m.ID, m.ThreadID, m.Snippet, m.SizeBytes = "", "", "", "", 0
	m.AuthResults = mail.AuthResults{}
	m.Labels = slices.Sorted(slices.Values(m.Labels))
	for _, list := range []*[]mail.Address{&m.To, &m.Cc} {
		if len(*list) == 0 {
			*list = nil
		}
	}
	if len(m.Labels) == 0 {
		m.Labels = nil
	}
	if len(m.AttachmentNames) == 0 {
		m.AttachmentNames = nil
	}
	return m
}

// expected returns what the seed decides of every message, by key.
func expected() map[string]mail.MessageMetadata {
	out := map[string]mail.MessageMetadata{}
	for _, m := range Mailbox() {
		out[m.Metadata.ID] = seeded(m.Metadata)
	}
	return out
}

func project(state map[string]mail.MessageMetadata) map[string]mail.MessageMetadata {
	out := map[string]mail.MessageMetadata{}
	for k, m := range state {
		out[k] = seeded(m)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

// seededAs returns the message seeded under key.
func seededAs(t *testing.T, key string) mail.MessageMetadata {
	t.Helper()
	for _, m := range Mailbox() {
		if m.Metadata.ID == key {
			return m.Metadata
		}
	}
	t.Fatalf("the mailbox has no message seeded as %q", key)
	return mail.MessageMetadata{}
}

func enumerationReturnsEveryMessage(t *testing.T, s *subject) {
	items, _ := enumerate(t, s.port, "")
	state := byKey(t, items)
	if diff := cmp.Diff(expected(), project(state), compare.Options); diff != "" {
		t.Errorf("enumerated messages (-seeded +got):\n%s", diff)
	}
	bodies := map[string]string{}
	for _, m := range Mailbox() {
		bodies[m.Metadata.ID] = m.Body.Text
	}
	ids := map[string]bool{}
	threads := map[string][]string{}
	for key, m := range state {
		if m.AccountID != Account {
			t.Errorf("message %q carries the account %q, want %q", key, m.AccountID, Account)
		}
		if m.ID == "" || ids[m.ID] {
			t.Errorf("message %q has the identifier %q, which is empty or not unique", key, m.ID)
		}
		ids[m.ID] = true
		if m.SizeBytes <= 0 {
			t.Errorf("message %q has the size %d", key, m.SizeBytes)
		}
		if bodyMarker := strings.SplitN(bodies[key], "\n", 2)[0]; !strings.Contains(m.Snippet, bodyMarker) {
			t.Errorf("message %q has the snippet %q, which does not hold the start of its body %q", key, m.Snippet, bodyMarker)
		}
		threads[m.ThreadID] = append(threads[m.ThreadID], key)
	}
	var partition [][]string
	for _, members := range threads {
		partition = append(partition, slices.Sorted(slices.Values(members)))
	}
	slices.SortFunc(partition, func(a, b []string) int { return strings.Compare(a[0], b[0]) })
	want := [][]string{{"alpha"}, {"bank"}, {"code"}, {"link"}, {"newsletter", "reply"}, {"receipt"}}
	if diff := cmp.Diff(want, partition, compare.Options); diff != "" {
		t.Errorf("messages grouped by thread (-want +got):\n%s", diff)
	}
}

// enumerationResumes restarts enumeration from each page token a full enumeration issued, and
// requires the messages of the pages after it, no more and no fewer.
func enumerationResumes(t *testing.T, s *subject) {
	var pages [][]string
	var tokens []mail.PageToken
	page := mail.PageToken("")
	for range len(Mailbox()) + 2 {
		got, err := s.port.EnumerateAll(t.Context(), page)
		if err != nil {
			t.Fatalf("EnumerateAll(%q): %v", page, err)
		}
		pages = append(pages, messageIDs(got.Items))
		tokens = append(tokens, got.Next)
		if got.Next == "" {
			break
		}
		page = got.Next
	}
	for i, token := range tokens {
		if token == "" {
			continue
		}
		rest, _ := enumerate(t, s.port, token)
		want := slices.Sorted(slices.Values(slices.Concat(pages[i+1:]...)))
		if diff := cmp.Diff(want, slices.Sorted(slices.Values(messageIDs(rest))), compare.Options); diff != "" {
			t.Errorf("enumerating from %q (-the later pages of a full enumeration +got):\n%s", token, diff)
		}
	}
}

func messageIDs(items []mail.MessageMetadata) []string {
	out := make([]string, len(items))
	for i, m := range items {
		out[i] = m.ID
	}
	return out
}

// threadListingResumes restarts thread listing from each page token a full listing issued, and
// requires the threads of the pages after it, no more and no fewer.
func threadListingResumes(t *testing.T, s *subject) {
	var pages [][]string
	var tokens []mail.PageToken
	page := mail.PageToken("")
	for range len(Mailbox()) + 2 {
		got, err := s.port.ListThreads(t.Context(), mail.All(), page)
		if err != nil {
			t.Fatalf("ListThreads(%q): %v", page, err)
		}
		pages = append(pages, threadIDs(got.Items))
		tokens = append(tokens, got.Next)
		if got.Next == "" {
			break
		}
		page = got.Next
	}
	for i, token := range tokens {
		if token == "" {
			continue
		}
		var rest []string
		page := token
		for range len(Mailbox()) + 2 {
			got, err := s.port.ListThreads(t.Context(), mail.All(), page)
			if err != nil {
				t.Fatalf("ListThreads(%q): %v", page, err)
			}
			rest = append(rest, threadIDs(got.Items)...)
			if got.Next == "" {
				break
			}
			page = got.Next
		}
		want := slices.Sorted(slices.Values(slices.Concat(pages[i+1:]...)))
		if diff := cmp.Diff(want, slices.Sorted(slices.Values(rest)), compare.Options); diff != "" {
			t.Errorf("listing threads from %q (-the later pages of a full listing +got):\n%s", token, diff)
		}
	}
}

func threadIDs(items []mail.ThreadMetadata) []string {
	out := make([]string, len(items))
	for i, th := range items {
		out[i] = th.ID
	}
	return out
}

func threadListingRefusesAForeignToken(t *testing.T, s *subject) {
	if _, err := s.port.ListThreads(t.Context(), mail.All(), "not-a-page-token"); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("ListThreads with a foreign page token returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
}

func enumerationRefusesAForeignToken(t *testing.T, s *subject) {
	if _, err := s.port.EnumerateAll(t.Context(), "not-a-page-token"); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("EnumerateAll with a foreign page token returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
}

func messageMetadataInOrder(t *testing.T, s *subject) {
	state := s.state(t)
	got, err := s.port.GetMessageMetadata(t.Context(), []string{s.id(t, "receipt"), "no-such-message", s.id(t, "bank")})
	if err != nil {
		t.Fatalf("GetMessageMetadata: %v", err)
	}
	want := []mail.MessageMetadata{state["receipt"], state["bank"]}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("GetMessageMetadata (-enumerated +got):\n%s", diff)
	}
}

func threadHoldsEveryMessage(t *testing.T, s *subject) {
	state := s.state(t)
	id := state["reply"].ThreadID
	got, err := s.port.GetThreadMetadata(t.Context(), id)
	if err != nil {
		t.Fatalf("GetThreadMetadata: %v", err)
	}
	want := mail.ThreadMetadata{AccountID: Account, ID: id, Messages: []mail.MessageMetadata{state["newsletter"], state["reply"]}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("GetThreadMetadata (-want +got):\n%s", diff)
	}
}

func unknownThreadNotFound(t *testing.T, s *subject) {
	if _, err := s.port.GetThreadMetadata(t.Context(), "no-such-thread"); !errors.Is(err, mail.ErrNotFound) {
		t.Errorf("GetThreadMetadata of an unknown thread returned %v, want an error wrapping %v", err, mail.ErrNotFound)
	}
}

// listThreads pages through ListThreads and returns the threads by the key of the thread their
// messages were seeded in, checking each thread carries the account and every message of its seed
// thread.
func listThreads(t *testing.T, s *subject, q mail.Query) []string {
	t.Helper()
	seedThread := map[string]string{}
	members := map[string][]string{}
	for _, m := range Mailbox() {
		seedThread[m.Metadata.ID] = m.Metadata.ThreadID
		members[m.Metadata.ThreadID] = append(members[m.Metadata.ThreadID], m.Metadata.ID)
	}
	var out []string
	page := mail.PageToken("")
	for range len(Mailbox()) + 2 {
		got, err := s.port.ListThreads(t.Context(), q, page)
		if err != nil {
			t.Fatalf("ListThreads: %v", err)
		}
		for _, th := range got.Items {
			held := byKey(t, th.Messages)
			thread := seedThread[sortedKeys(held)[0]]
			if th.AccountID != Account {
				t.Errorf("thread %q carries the account %q, want %q", thread, th.AccountID, Account)
			}
			if diff := cmp.Diff(slices.Sorted(slices.Values(members[thread])), sortedKeys(held), compare.Options); diff != "" {
				t.Errorf("thread %q's messages (-seeded +got):\n%s", thread, diff)
			}
			out = append(out, thread)
		}
		if got.Next == "" {
			return slices.Sorted(slices.Values(out))
		}
		page = got.Next
	}
	t.Fatalf("ListThreads returned more pages than the mailbox has messages")
	return nil
}

func querySelectsThreads(t *testing.T, s *subject) {
	bank, receipt := seededAs(t, "bank").From.Email, seededAs(t, "receipt").From.Email
	cases := []struct {
		name string
		q    mail.Query
		want []string
	}{
		{"every message", mail.All(), []string{"alpha", "bank", "code", "link", "newsletter", "receipt"}},
		{"a label", mail.InLabel(finance), []string{"bank"}},
		{"a nested label, selecting the thread through each of its messages", mail.InLabel(reading), []string{"newsletter"}},
		{"the inbox", mail.InLabel(mail.Inbox), []string{"alpha", "bank", "code", "newsletter"}},
		{"a sender, ignoring case", mail.From(strings.ToUpper(receipt)), []string{"receipt"}},
		{"after an instant, selecting a thread through its newest message", mail.After(base + 6*day - hour), []string{"newsletter"}},
		{"before an instant", mail.Before(base + day - hour), []string{"bank"}},
		{"after the instant a message is dated, which it includes", mail.After(base + 5*day), []string{"newsletter", "receipt"}},
		{"before the instant a message is dated, which it excludes", mail.Before(base + day), []string{"bank"}},
		{"a window", mail.And(mail.After(base+2*day-hour), mail.Before(base+4*day-hour)), []string{"alpha", "code"}},
		{"a label and a sender", mail.And(mail.InLabel(mail.Inbox), mail.From(bank)), []string{"bank"}},
		{"a label and a sender no message has together", mail.And(mail.InLabel(finance), mail.From(receipt)), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, listThreads(t, s, c.q), compare.Options); diff != "" {
				t.Errorf("ListThreads (-want +got):\n%s", diff)
			}
		})
	}
}

func invalidQueriesRefused(t *testing.T, s *subject) {
	cases := []struct {
		name string
		q    mail.Query
	}{
		{"the zero query", mail.Query{}},
		{"an And holding the zero query", mail.And(mail.All(), mail.Query{})},
		{"an empty label", mail.InLabel("")},
		{"an empty sender", mail.From("")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := s.port.ListThreads(t.Context(), c.q, ""); !errors.Is(err, mail.ErrInvalid) {
				t.Errorf("ListThreads returned %v, want an error wrapping %v", err, mail.ErrInvalid)
			}
		})
	}
}

func bodyAsSeeded(t *testing.T, s *subject) {
	for _, m := range Mailbox() {
		id := s.id(t, m.Metadata.ID)
		got, err := s.port.GetMessageBody(t.Context(), id)
		if err != nil {
			t.Fatalf("GetMessageBody(%q): %v", m.Metadata.ID, err)
		}
		want := m.Body
		want.AccountID, want.MessageID = Account, id
		if diff := cmp.Diff(want, got, compare.Options); diff != "" {
			t.Errorf("GetMessageBody(%q) (-seeded +got):\n%s", m.Metadata.ID, diff)
		}
	}
}

func unknownBodyNotFound(t *testing.T, s *subject) {
	if _, err := s.port.GetMessageBody(t.Context(), "no-such-message"); !errors.Is(err, mail.ErrNotFound) {
		t.Errorf("GetMessageBody of an unknown message returned %v, want an error wrapping %v", err, mail.ErrNotFound)
	}
}

func labels(t *testing.T, s *subject) []string {
	t.Helper()
	got, err := s.port.ListLabels(t.Context())
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	var paths []string
	for _, l := range got {
		if l.AccountID != Account {
			t.Errorf("label %q carries the account %q, want %q", l.Path, l.AccountID, Account)
		}
		paths = append(paths, l.Path)
	}
	return paths
}

func everyLabelListed(t *testing.T, s *subject) {
	got := labels(t, s)
	for _, want := range []string{mail.Inbox, mail.Trash, mail.Spam, finance, reading, receipts} {
		if !slices.Contains(got, want) {
			t.Errorf("ListLabels returned %q, which lacks %q", got, want)
		}
	}
}

func ensureLabelIsIdempotent(t *testing.T, s *subject) {
	for _, path := range []string{marker.Field("projectlabel"), marker.Field("projectlabel") + "/" + marker.Field("phaselabel")} {
		for range 2 {
			got, err := s.port.EnsureLabel(t.Context(), path)
			if err != nil {
				t.Fatalf("EnsureLabel(%q): %v", path, err)
			}
			if diff := cmp.Diff(mail.Label{AccountID: Account, Path: path}, got, compare.Options); diff != "" {
				t.Errorf("EnsureLabel(%q) (-want +got):\n%s", path, diff)
			}
		}
		listed := labels(t, s)
		if n := len(slices.DeleteFunc(listed, func(l string) bool { return l != path })); n != 1 {
			t.Errorf("after ensuring %q twice, ListLabels lists it %d times, want once", path, n)
		}
	}
	if _, err := s.port.EnsureLabel(t.Context(), ""); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("EnsureLabel of an empty path returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
}

// built returns a function returning the op a constructor built, failing the test when the
// constructor refused it.
func built(t *testing.T) func(mail.MutationOp, error) mail.MutationOp {
	return func(op mail.MutationOp, err error) mail.MutationOp {
		t.Helper()
		if err != nil {
			t.Fatalf("building an op: %v", err)
		}
		return op
	}
}

func verbsChangeWhatTheyName(t *testing.T, s *subject) {
	newLabel := marker.Field("projectlabel")
	cases := []struct {
		name string
		ops  func(t *testing.T, s *subject) []mail.MutationOp
		want func(state map[string]mail.MessageMetadata)
	}{
		{
			"label",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.LabelOp(s.id(t, "code"), newLabel))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "code", func(x *mail.MessageMetadata) { x.Labels = []string{mail.Inbox, newLabel} })
			},
		},
		{
			"unlabel",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.UnlabelOp(s.id(t, "bank"), finance))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "bank", func(x *mail.MessageMetadata) { x.Labels = []string{mail.Inbox} })
			},
		},
		{
			"move",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.MoveOp(s.id(t, "bank"), finance, newLabel))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "bank", func(x *mail.MessageMetadata) { x.Labels = []string{mail.Inbox, newLabel} })
			},
		},
		{
			"label back into the inbox",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.LabelOp(s.id(t, "link"), mail.Inbox))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "link", func(x *mail.MessageMetadata) { x.Labels = []string{mail.Inbox} })
			},
		},
		{
			"archive",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.ArchiveOp(s.id(t, "bank")))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "bank", func(x *mail.MessageMetadata) { x.Labels = []string{finance} })
			},
		},
		{
			"mark read",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.MarkReadOp(s.id(t, "code")))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "code", func(x *mail.MessageMetadata) { x.Flags.Read = true })
			},
		},
		{
			"star",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.StarOp(s.id(t, "code")))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "code", func(x *mail.MessageMetadata) { x.Flags.Starred = true })
			},
		},
		{
			"trash",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.TrashOp(s.id(t, "alpha")))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "alpha", func(x *mail.MessageMetadata) { x.Labels = []string{mail.Trash} })
			},
		},
		{
			"spam",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.SpamOp(s.id(t, "alpha")))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "alpha", func(x *mail.MessageMetadata) { x.Labels = []string{mail.Spam} })
			},
		},
		{
			"mute, which mutes the whole thread and takes it out of the inbox",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.MuteOp(s.id(t, "reply")))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "newsletter", func(x *mail.MessageMetadata) { x.Flags.Muted, x.Labels = true, []string{reading} })
				set(m, "reply", func(x *mail.MessageMetadata) { x.Flags.Muted, x.Labels = true, []string{reading} })
			},
		},
		{
			"two ops on one message, applied in order",
			func(t *testing.T, s *subject) []mail.MutationOp {
				code := s.id(t, "code")
				return []mail.MutationOp{built(t)(mail.LabelOp(code, newLabel)), built(t)(mail.MoveOp(code, newLabel, finance))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "code", func(x *mail.MessageMetadata) { x.Labels = []string{mail.Inbox, finance} })
			},
		},
		{
			"a batch of two verbs on two messages",
			func(t *testing.T, s *subject) []mail.MutationOp {
				return []mail.MutationOp{built(t)(mail.StarOp(s.id(t, "code"))), built(t)(mail.MarkReadOp(s.id(t, "alpha")))}
			},
			func(m map[string]mail.MessageMetadata) {
				set(m, "code", func(x *mail.MessageMetadata) { x.Flags.Starred = true })
				set(m, "alpha", func(x *mail.MessageMetadata) { x.Flags.Read = true })
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			// Each verb starts from the mailbox as seeded.
			s := newSubject(t, s.harness)
			if _, err := s.port.EnsureLabel(t.Context(), newLabel); err != nil {
				t.Fatalf("EnsureLabel: %v", err)
			}
			ops := c.ops(t, s)
			result, err := s.port.Mutate(t.Context(), ops)
			if err != nil {
				t.Fatalf("Mutate: %v", err)
			}
			if diff := cmp.Diff(make([]error, len(ops)), result.Errors, compare.Options); diff != "" {
				t.Errorf("Mutate's outcomes (-each applied +got):\n%s", diff)
			}
			want := expected()
			c.want(want)
			if diff := cmp.Diff(want, project(s.state(t)), compare.Options); diff != "" {
				t.Errorf("the mailbox after the mutation (-want +got):\n%s", diff)
			}
		})
	}
}

// set edits the message at key in state, sorting its labels as seeded does.
func set(state map[string]mail.MessageMetadata, key string, edit func(*mail.MessageMetadata)) {
	m := state[key]
	edit(&m)
	state[key] = seeded(m)
}

func labelMustExist(t *testing.T, s *subject) {
	op := built(t)(mail.LabelOp(s.id(t, "code"), marker.Field("missinglabel")))
	result, err := s.port.Mutate(t.Context(), []mail.MutationOp{op})
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], mail.ErrNotFound) {
		t.Errorf("Mutate's outcomes are %v, want one wrapping %v", result.Errors, mail.ErrNotFound)
	}
	if diff := cmp.Diff(expected(), project(s.state(t)), compare.Options); diff != "" {
		t.Errorf("the mailbox after the refused op (-seeded +got):\n%s", diff)
	}
}

func unknownMessageOpFails(t *testing.T, s *subject) {
	ops := []mail.MutationOp{built(t)(mail.StarOp("no-such-message")), built(t)(mail.StarOp(s.id(t, "code")))}
	result, err := s.port.Mutate(t.Context(), ops)
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if len(result.Errors) != 2 {
		t.Fatalf("Mutate reported %d outcomes for 2 ops", len(result.Errors))
	}
	if !errors.Is(result.Errors[0], mail.ErrNotFound) {
		t.Errorf("the op on an unknown message had the outcome %v, want one wrapping %v", result.Errors[0], mail.ErrNotFound)
	}
	// Whether the other op applied is the implementation's, and the state must agree with the outcome
	// it reported.
	want := expected()
	if result.Errors[1] == nil {
		set(want, "code", func(x *mail.MessageMetadata) { x.Flags.Starred = true })
	}
	if diff := cmp.Diff(want, project(s.state(t)), compare.Options); diff != "" {
		t.Errorf("the mailbox after the batch (-as the outcomes report +got):\n%s", diff)
	}
}

func unbuiltOpRefused(t *testing.T, s *subject) {
	ops := []mail.MutationOp{built(t)(mail.StarOp(s.id(t, "code"))), {}}
	if _, err := s.port.Mutate(t.Context(), ops); !errors.Is(err, mail.ErrInvalid) {
		t.Errorf("Mutate with an op no constructor built returned %v, want an error wrapping %v", err, mail.ErrInvalid)
	}
	if diff := cmp.Diff(expected(), project(s.state(t)), compare.Options); diff != "" {
		t.Errorf("the mailbox after the refused batch (-seeded +got):\n%s", diff)
	}
}

// drain asks for changes from c until a change set comes back empty, and returns them together with
// the cursor it ended at.
func drain(t *testing.T, s *subject, c mail.Cursor) (mail.ChangeSet, mail.Cursor) {
	t.Helper()
	var out mail.ChangeSet
	for range len(Mailbox()) + 2 {
		got, err := s.port.ChangesSince(t.Context(), c)
		if err != nil {
			t.Fatalf("ChangesSince: %v", err)
		}
		if got.AccountID != Account {
			t.Errorf("a change set carries the account %q, want %q", got.AccountID, Account)
		}
		c = got.Next
		if len(got.Added)+len(got.Modified)+len(got.Removed) == 0 {
			return out, c
		}
		out.Added = append(out.Added, got.Added...)
		out.Modified = append(out.Modified, got.Modified...)
		out.Removed = append(out.Removed, got.Removed...)
	}
	t.Fatalf("ChangesSince never came back empty")
	return mail.ChangeSet{}, ""
}

type changes struct{ Added, Modified, Removed []string }

func noActivityNoChanges(t *testing.T, s *subject) {
	c, err := s.port.CurrentCursor(t.Context())
	if err != nil {
		t.Fatalf("CurrentCursor: %v", err)
	}
	got, _ := drain(t, s, c)
	if diff := cmp.Diff(changes{}, changes{got.Added, got.Modified, got.Removed}, compare.Options); diff != "" {
		t.Errorf("changes with no activity (-want +got):\n%s", diff)
	}
}

func changeReported(t *testing.T, s *subject) {
	code := s.id(t, "code")
	c, err := s.port.CurrentCursor(t.Context())
	if err != nil {
		t.Fatalf("CurrentCursor: %v", err)
	}
	if _, err := s.port.Mutate(t.Context(), []mail.MutationOp{built(t)(mail.StarOp(code))}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	now, err := s.port.CurrentCursor(t.Context())
	if err != nil {
		t.Fatalf("CurrentCursor: %v", err)
	}
	got, next := drain(t, s, c)
	if diff := cmp.Diff(changes{Modified: []string{code}}, changes{got.Added, got.Modified, got.Removed}, compare.Options); diff != "" {
		t.Errorf("changes after starring one message (-want +got):\n%s", diff)
	}
	again, _ := drain(t, s, next)
	if diff := cmp.Diff(changes{}, changes{again.Added, again.Modified, again.Removed}, compare.Options); diff != "" {
		t.Errorf("changes after the reported ones (-want +got):\n%s", diff)
	}
	// A cursor taken after the change is the mailbox as it is now, so nothing follows it.
	fresh, _ := drain(t, s, now)
	if diff := cmp.Diff(changes{}, changes{fresh.Added, fresh.Modified, fresh.Removed}, compare.Options); diff != "" {
		t.Errorf("changes after a cursor taken once the change was made (-want +got):\n%s", diff)
	}
}

func deliveryAndRemovalReported(t *testing.T, s *subject) {
	bank := s.id(t, "bank")
	c, err := s.port.CurrentCursor(t.Context())
	if err != nil {
		t.Fatalf("CurrentCursor: %v", err)
	}
	if err := s.impl.Deliver(late()); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	if err := s.impl.Remove(bank); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	arrived := s.id(t, "late")
	got, _ := drain(t, s, c)
	want := changes{Added: []string{arrived}, Removed: []string{bank}}
	if diff := cmp.Diff(want, changes{got.Added, got.Modified, got.Removed}, compare.Options); diff != "" {
		t.Errorf("changes after one delivery and one removal (-want +got):\n%s", diff)
	}
	state := s.state(t)
	if _, ok := state["bank"]; ok {
		t.Error("the removed message is still enumerated")
	}
	if diff := cmp.Diff(seeded(late().Metadata), seeded(state["late"]), compare.Options); diff != "" {
		t.Errorf("the delivered message (-as delivered +got):\n%s", diff)
	}
	if _, err := s.port.GetMessageBody(t.Context(), bank); !errors.Is(err, mail.ErrNotFound) {
		t.Errorf("the removed message's body returned %v, want an error wrapping %v", err, mail.ErrNotFound)
	}
}

// changesAreNetted delivers a message and changes it, and delivers another and removes it, all after
// one cursor. The first is only added and the second only removed, as ChangeSet states.
func changesAreNetted(t *testing.T, s *subject) {
	c, err := s.port.CurrentCursor(t.Context())
	if err != nil {
		t.Fatalf("CurrentCursor: %v", err)
	}
	if err := s.impl.Deliver(late()); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	arrived := s.id(t, "late")
	if _, err := s.port.Mutate(t.Context(), []mail.MutationOp{built(t)(mail.StarOp(arrived))}); err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if err := s.impl.Deliver(brief()); err != nil {
		t.Fatalf("Deliver: %v", err)
	}
	gone := s.id(t, "brief")
	if err := s.impl.Remove(gone); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got, _ := drain(t, s, c)
	want := changes{Added: []string{arrived}, Removed: []string{gone}}
	if diff := cmp.Diff(want, changes{got.Added, got.Modified, got.Removed}, compare.Options); diff != "" {
		t.Errorf("netted changes (-want +got):\n%s", diff)
	}
}

func foreignCursorIsAGap(t *testing.T, s *subject) {
	if _, err := s.port.ChangesSince(t.Context(), "not-a-cursor"); !errors.Is(err, mail.ErrCursorGap) {
		t.Errorf("ChangesSince from a foreign cursor returned %v, want an error wrapping %v", err, mail.ErrCursorGap)
	}
}

var operations = []mail.Operation{
	mail.OpListThreads, mail.OpGetThreadMetadata, mail.OpGetMessageMetadata, mail.OpGetMessageBody,
	mail.OpListLabels, mail.OpEnsureLabel, mail.OpMutate, mail.OpCurrentCursor, mail.OpChangesSince,
	mail.OpEnumerateAll,
}

func rateProfileDeclares(t *testing.T, s *subject) {
	p := s.port.RateProfile()
	if b := p.BudgetPerSecond(); b <= 0 {
		t.Errorf("BudgetPerSecond returned %v, want a positive budget", b)
	}
	for _, op := range operations {
		for _, n := range []int{0, 1, 5} {
			c := p.Cost(mail.ProviderOp{Operation: op, Messages: n})
			if c.Weight <= 0 {
				t.Errorf("Cost of operation %d naming %d messages is %+v, want a positive weight", op, n, c)
			}
		}
	}
	if err := p.RefreshLimits(t.Context()); err != nil {
		t.Errorf("RefreshLimits: %v", err)
	}
}

func rateProfileParsesThrottles(t *testing.T, s *subject) {
	p := s.port.RateProfile()
	signal := mail.ThrottleSignal{RetryAfterMillis: 1500, HasRetryAfter: true, Scope: mail.ScopePerUser}
	type parsed struct {
		Signal    mail.ThrottleSignal
		Throttled bool
	}
	cases := []struct {
		name string
		err  error
		want parsed
	}{
		{"a throttle", mail.ThrottleError{Signal: signal}, parsed{signal, true}},
		{"a wrapped throttle", fmt.Errorf("listing: %w", mail.ThrottleError{Signal: signal}), parsed{signal, true}},
		{"another port error", mail.ErrNotFound, parsed{}},
		{"no error", nil, parsed{}},
	}
	for _, c := range cases {
		sig, ok := p.ParseThrottle(c.err)
		if diff := cmp.Diff(c.want, parsed{sig, ok}, compare.Options); diff != "" {
			t.Errorf("ParseThrottle of %s (-want +got):\n%s", c.name, diff)
		}
	}
}

func cancelledCallsFail(t *testing.T, s *subject) {
	code := s.id(t, "code")
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	calls := map[string]func() error{
		"ListThreads":        func() error { _, err := s.port.ListThreads(cancelled, mail.All(), ""); return err },
		"GetThreadMetadata":  func() error { _, err := s.port.GetThreadMetadata(cancelled, "no-such-thread"); return err },
		"GetMessageMetadata": func() error { _, err := s.port.GetMessageMetadata(cancelled, []string{code}); return err },
		"GetMessageBody":     func() error { _, err := s.port.GetMessageBody(cancelled, code); return err },
		"ListLabels":         func() error { _, err := s.port.ListLabels(cancelled); return err },
		"EnsureLabel":        func() error { _, err := s.port.EnsureLabel(cancelled, marker.Field("cancelledlabel")); return err },
		"Mutate": func() error {
			_, err := s.port.Mutate(cancelled, []mail.MutationOp{built(t)(mail.StarOp(code))})
			return err
		},
		"CurrentCursor": func() error { _, err := s.port.CurrentCursor(cancelled); return err },
		"ChangesSince":  func() error { _, err := s.port.ChangesSince(cancelled, "no-such-cursor"); return err },
		"EnumerateAll":  func() error { _, err := s.port.EnumerateAll(cancelled, ""); return err },
	}
	for _, name := range sortedKeys(calls) {
		if err := calls[name](); !errors.Is(err, context.Canceled) {
			t.Errorf("%s with a cancelled context returned %v, want an error wrapping %v", name, err, context.Canceled)
		}
	}
	if diff := cmp.Diff(expected(), project(s.state(t)), compare.Options); diff != "" {
		t.Errorf("the mailbox after the cancelled calls (-seeded +got):\n%s", diff)
	}
	if slices.Contains(labels(t, s), marker.Field("cancelledlabel")) {
		t.Errorf("a cancelled EnsureLabel created its label")
	}
}
