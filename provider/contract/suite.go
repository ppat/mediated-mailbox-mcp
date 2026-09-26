package contract

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// Account is the account the suite asks each harness to build its implementation for.
const Account = "contract"

// maxPages bounds how many pages the suite reads from one listing or change feed before it fails,
// so an implementation that never ends a listing fails rather than hangs.
const maxPages = 10000

// Config sets up a run of the suite.
type Config struct {
	// Harness builds the implementation each case runs against.
	Harness Harness
	// Run is lowercase letters only, and begins the mark of every case in the run. A run against a
	// real provider takes one no earlier run against the account took.
	Run string
	// Base is the date of the oldest message a case adds, and the others follow it a minute apart,
	// the last eight minutes after it. Zero means 2024-09-01T09:00:00Z. A run against a real provider
	// sets it to a few minutes before the run starts, so the messages it adds are the newest the
	// account holds and a listing reaches them first.
	Base mail.UnixMilli
	// Scoped gives the messages of each case that lists threads a label of that case's own, and
	// conjoins every query the case lists with it, so the listing reads only what the case added. A
	// run against a real provider sets it, since a query over the whole account would read every
	// thread there. The label's name is fixed per case, the same in every run, so runs create no new
	// labels. A harness whose runs share an account takes the label off every message a case added
	// once the case ends, so a later run's listing passes over the earlier runs' messages.
	Scoped bool
	// EnumerationPages bounds how many pages one enumeration reads before the case fails. Zero means
	// 10 000. Where the case's messages fall in a full enumeration is the provider's order, which no
	// provider need document, so a run against a real provider sets a bound that fails the case early
	// and plainly if that order puts them behind the account's other mail.
	EnumerationPages int
	// Reserve is how long before the test's deadline the suite stops starting cases, so the cases
	// already run end and clean up before the test binary is stopped. Zero means the suite starts
	// every case.
	Reserve time.Duration
}

// Harness returns a new implementation of the port for account. The suite calls it once per case
// and adds the case's messages through Deliver.
type Harness func(t *testing.T, account string) Implementation

// Implementation is the port under test, with the two things only the provider's side can do to a
// mailbox, so the suite can add mail and require new and deleted mail to reach delta sync
// (ADR-0018). The fake does them with its own test controls, and a real adapter's harness through
// the provider directly, outside the port.
type Implementation struct {
	Port mail.Port[context.Context]
	// Deliver adds m to the mailbox as newly arrived mail and returns the identifier the
	// implementation gave it. m.Metadata.ThreadID is a key, and m joins the thread of the message the
	// same implementation was earlier given with that key. Deliver creates any label m carries that
	// the account lacks.
	Deliver func(m Message) (string, error)
	// Remove deletes the message with the identifier id from the mailbox, as a deletion the provider
	// reports among its changes. It is nil for a harness that cannot delete mail, and the cases that
	// need it are skipped before they add anything.
	Remove func(id string) error
	// Searchable returns once the provider's search finds every message ids names. The suite calls it
	// after adding the messages of a case that lists threads. It is nil for an implementation whose
	// messages are searchable once added.
	Searchable func(ids []string) error
}

// mailboxKind is what a case needs in the mailbox before it runs.
type mailboxKind uint8

const (
	// empty is a case that adds no messages.
	empty mailboxKind = iota
	// seeded is a case that adds the messages of Mailbox.
	seeded
	// scoped is a case that adds the messages of Mailbox and lists threads over them.
	scoped
)

// Run runs every case of the contract against the implementations cfg's harness builds.
func Run(t *testing.T, cfg Config) {
	t.Helper()
	if cfg.Run == "" || strings.Trim(cfg.Run, "abcdefghijklmnopqrstuvwxyz") != "" {
		t.Fatalf("the run %q is not lowercase letters only", cfg.Run)
	}
	if cfg.Base == 0 {
		cfg.Base = defaultBase
	}
	r := &runner{cfg: cfg}
	cases := []struct {
		name    string
		mailbox mailboxKind
		run     func(*testing.T, *subject)
		removes bool
	}{
		{"enumeration returns every message once with what was seeded", seeded, enumerationReturnsEveryMessage, false},
		{"enumeration resumes from any page token", seeded, enumerationResumes, false},
		{"enumeration refuses a page token it did not issue", empty, enumerationRefusesAForeignToken, false},
		{"thread listing resumes from any page token", scoped, threadListingResumes, false},
		{"thread listing refuses a page token it did not issue", empty, threadListingRefusesAForeignToken, false},
		{"message metadata comes back in the order asked, skipping unknown messages", seeded, messageMetadataInOrder, false},
		{"a thread comes back with every message, oldest first", seeded, threadHoldsEveryMessage, false},
		{"an unknown thread is not found", empty, unknownThreadNotFound, false},
		{"a query selects exactly the threads it names", scoped, querySelectsThreads, false},
		{"a query no constructor completed is refused", empty, invalidQueriesRefused, false},
		{"a body comes back exactly as seeded", seeded, bodyAsSeeded, false},
		{"an unknown message's body is not found", empty, unknownBodyNotFound, false},
		{"every label is listed", seeded, everyLabelListed, false},
		{"ensuring a label creates it once", empty, ensureLabelIsIdempotent, false},
		{"each verb changes exactly what it names", empty, verbsChangeWhatTheyName, false},
		{"labelling with a label the account lacks changes nothing", seeded, labelMustExist, false},
		{"removing a label the message or the account lacks changes nothing", seeded, removingAnAbsentLabel, false},
		{"an op on an unknown message fails alone", seeded, unknownMessageOpFails, false},
		{"a batch holding an op no constructor built applies nothing", seeded, unbuiltOpRefused, false},
		{"no activity means no changes", seeded, noActivityNoChanges, false},
		{"a change is reported once, as a modification", seeded, changeReported, false},
		{"delivered mail is added", seeded, deliveryReported, false},
		{"removed mail is removed", seeded, removalReported, true},
		{"a message delivered and then changed is reported once, as added", seeded, changedDeliveryIsAdded, false},
		{"a message delivered and then removed is reported once, as removed", seeded, removedDeliveryIsRemoved, true},
		{"a cursor the implementation cannot read is a gap", empty, foreignCursorIsAGap, false},
		{"the rate profile declares a budget and a cost for every operation", empty, rateProfileDeclares, false},
		{"every call the rate profile sizes fits one second at the hard cap", empty, rateProfileFitsTheHardCap, false},
		{"the rate profile recognises the port's throttle", empty, rateProfileParsesThrottles, false},
		{"a cancelled call fails and changes nothing", seeded, cancelledCallsFail, false},
	}
	for i, c := range cases {
		if r.late(t) {
			t.Errorf("stopped %d cases short of the end, %s before the test's deadline, so the cases already run can clean up", len(cases)-i, cfg.Reserve)
			return
		}
		t.Run(c.name, func(t *testing.T) {
			c.run(t, r.subject(t, c.mailbox, c.removes))
		})
	}
}

// runner hands each case its own mark.
type runner struct {
	cfg  Config
	next int
}

// subject is one case's implementation under test, with the messages the case added to it and the
// identifiers the implementation gave them.
type subject struct {
	runner *runner
	impl   Implementation
	port   mail.Port[context.Context]
	// tag is the case's own text in the run, lowercase letters only, and mark is what the case
	// appends to the subject of every message it adds. A label named from the tag is one no case
	// creates, so its name may differ in every run without the account gaining labels. index is the
	// case's own text, the same in every run.
	tag   string
	index string
	mark  string
	base  mail.UnixMilli
	// scope is the label a scoped case's messages carry, and empty in any other case.
	scope    string
	messages map[string]Message
	ids      map[string]string
	keys     map[string]string
}

// subject builds the implementation for a case and adds the messages the case needs.
func (r *runner) subject(t *testing.T, kind mailboxKind, removes bool) *subject {
	t.Helper()
	r.inTime(t)
	index := sequence(r.next)
	tag := r.cfg.Run + index
	r.next++
	impl := r.cfg.Harness(t, Account)
	s := &subject{
		runner:   r,
		impl:     impl,
		port:     impl.Port,
		tag:      tag,
		index:    index,
		mark:     " " + marker.Field("contract"+tag),
		base:     r.cfg.Base,
		messages: map[string]Message{},
		ids:      map[string]string{},
		keys:     map[string]string{},
	}
	if kind == scoped && r.cfg.Scoped {
		s.scope = marker.Field("scope" + index)
	}
	if removes && impl.Remove == nil {
		t.Skip("removing mail from a real provider's account takes a delete, which nothing in this system makes, so the case runs against the fake alone (ADR-0043)")
	}
	if kind == empty {
		return s
	}
	var ids []string
	for _, m := range mailbox(s.mark, s.base) {
		if s.scope != "" {
			m.Metadata.Labels = append(slices.Clone(m.Metadata.Labels), s.scope)
		}
		ids = append(ids, s.deliver(t, m))
	}
	if kind == scoped && impl.Searchable != nil {
		if err := impl.Searchable(ids); err != nil {
			t.Fatalf("waiting for the provider's search to find the case's messages: %v", err)
		}
	}
	return s
}

// late reports whether less than the reserve remains before the test's deadline.
func (r *runner) late(t *testing.T) bool {
	deadline, ok := t.Deadline()
	return ok && r.cfg.Reserve > 0 && time.Until(deadline) < r.cfg.Reserve
}

// inTime fails the case once less than the reserve remains before the test's deadline. A case that
// fails this way still runs its cleanups, which a test binary stopped at its deadline never does.
// Every page loop and every case's start checks it, so no single case can run past the reserve.
func (r *runner) inTime(t *testing.T) {
	t.Helper()
	if r.late(t) {
		t.Fatalf("stopped %s before the test's deadline, so the case's cleanups run", r.cfg.Reserve)
	}
}

func (s *subject) inTime(t *testing.T) {
	t.Helper()
	s.runner.inTime(t)
}

// sequence writes n as three lowercase letters, so each case's tag differs from every other's in
// the run and none begins another.
func sequence(n int) string {
	out := []byte("aaa")
	for i := len(out) - 1; i >= 0; i-- {
		out[i] = byte('a' + n%26)
		n /= 26
	}
	return string(out)
}

// deliver adds m to the mailbox and records the identifier the implementation gave it.
func (s *subject) deliver(t *testing.T, m Message) string {
	t.Helper()
	key := m.Metadata.ID
	id, err := s.impl.Deliver(m)
	if err != nil {
		t.Fatalf("delivering the message keyed %q: %v", key, err)
	}
	if _, used := s.keys[id]; id == "" || used {
		t.Fatalf("delivering the message keyed %q returned the identifier %q, which is empty or not unique", key, id)
	}
	s.messages[key], s.ids[key], s.keys[id] = m, id, key
	return id
}

// id returns the identifier the implementation gave the message added under key.
func (s *subject) id(t *testing.T, key string) string {
	t.Helper()
	id, ok := s.ids[key]
	if !ok {
		t.Fatalf("the case added no message keyed %q", key)
	}
	return id
}

// key returns the key of a message the case added. A message the case did not add is reported as
// not the case's, and is never compared or printed. A message carrying the case's mark under an
// identifier the case was never given fails the test, since only an implementation that changed an
// identifier or invented a message produces one.
func (s *subject) key(t *testing.T, m mail.MessageMetadata) (string, bool) {
	t.Helper()
	if key, ok := s.keys[m.ID]; ok {
		return key, true
	}
	if strings.HasSuffix(m.Subject, s.mark) {
		t.Fatalf("a message with the subject %q carries this case's mark under an identifier the case was never given", m.Subject)
	}
	return "", false
}

// query returns q, conjoined with the case's label when the case is scoped.
func (s *subject) query(q mail.Query) mail.Query {
	if s.scope == "" {
		return q
	}
	return mail.And(mail.InLabel(s.scope), q)
}

// mailboxKeys are the keys of the messages Mailbox holds, sorted.
func mailboxKeys() []string {
	var out []string
	for _, m := range Mailbox() {
		out = append(out, m.Metadata.ID)
	}
	return slices.Sorted(slices.Values(out))
}

// fits returns how many messages one call of op may name, up to most. It is the most whose cost
// stays within one second's worth at the hard cap, the hard-cap fraction of the budget the profile
// declares (ADR-0024), so the caller learns what fits by asking the profile (ADR-0023).
func fits(p mail.RateLimitProfile[context.Context], op mail.Operation, most int) int {
	limit := mail.HardCapFraction * p.BudgetPerSecond()
	n := 1
	for n < most && p.Cost(mail.ProviderOp{Operation: op, Messages: n + 1}).Weight <= limit {
		n++
	}
	return n
}

// state returns every message the case added that the mailbox still holds, by key, read through
// GetMessageMetadata in calls that fit.
func (s *subject) state(t *testing.T) map[string]mail.MessageMetadata {
	t.Helper()
	ids := make([]string, 0, len(s.ids))
	for _, key := range sortedKeys(s.ids) {
		ids = append(ids, s.ids[key])
	}
	out := map[string]mail.MessageMetadata{}
	size := fits(s.port.RateProfile(), mail.OpGetMessageMetadata, len(ids))
	for chunk := range slices.Chunk(ids, size) {
		got, err := s.port.GetMessageMetadata(t.Context(), chunk)
		if err != nil {
			t.Fatalf("GetMessageMetadata: %v", err)
		}
		for _, m := range got {
			key, ok := s.keys[m.ID]
			if !ok || !slices.Contains(chunk, m.ID) {
				t.Fatalf("GetMessageMetadata returned a message it was not asked for")
			}
			if _, dup := out[key]; dup {
				t.Fatalf("GetMessageMetadata returned the message keyed %q twice", key)
			}
			out[key] = m
		}
	}
	return out
}

// enumerate pages through EnumerateAll from page until it has seen every message keyed in want, or
// the listing ends. It returns the keys of the case's messages on each page, each page's Next, and
// the case's messages it saw, by key. Every other message is passed over.
func (s *subject) enumerate(t *testing.T, page mail.PageToken, want []string) ([][]string, []mail.PageToken, map[string]mail.MessageMetadata) {
	t.Helper()
	var pages [][]string
	var tokens []mail.PageToken
	seen := map[string]mail.MessageMetadata{}
	issued := map[mail.PageToken]bool{}
	limit := s.runner.cfg.EnumerationPages
	if limit <= 0 {
		limit = maxPages
	}
	for range limit {
		s.inTime(t)
		got, err := s.port.EnumerateAll(t.Context(), page)
		if err != nil {
			t.Fatalf("EnumerateAll: %v", err)
		}
		var keys []string
		for _, m := range got.Items {
			key, ours := s.key(t, m)
			if !ours {
				continue
			}
			if _, dup := seen[key]; dup {
				t.Fatalf("the message keyed %q came back twice", key)
			}
			seen[key] = m
			keys = append(keys, key)
		}
		pages = append(pages, keys)
		tokens = append(tokens, got.Next)
		if got.Next == "" || !slices.ContainsFunc(want, func(k string) bool { _, ok := seen[k]; return !ok }) {
			return pages, tokens, seen
		}
		if issued[got.Next] {
			t.Fatalf("EnumerateAll issued one page token twice")
		}
		issued[got.Next] = true
		page = got.Next
	}
	t.Fatalf("EnumerateAll returned %d pages without ending or reaching every message the case added. The order of a full enumeration is the provider's own, and it may put the case's messages behind the account's other mail", limit)
	return nil, nil, nil
}

// seededFields returns what the seed decides of m. The identifiers, the account, the snippet, the
// size and the authentication results are the implementation's, label order carries no meaning,
// and an empty list is no list.
func seededFields(m mail.MessageMetadata) mail.MessageMetadata {
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

// expected returns what the seed decides of every message of Mailbox the case added, by key.
func (s *subject) expected() map[string]mail.MessageMetadata {
	out := map[string]mail.MessageMetadata{}
	for _, key := range mailboxKeys() {
		out[key] = seededFields(s.messages[key].Metadata)
	}
	return out
}

func project(state map[string]mail.MessageMetadata) map[string]mail.MessageMetadata {
	out := map[string]mail.MessageMetadata{}
	for k, m := range state {
		out[k] = seededFields(m)
	}
	return out
}

func sortedKeys[V any](m map[string]V) []string { return slices.Sorted(maps.Keys(m)) }

// seededAs returns the metadata the case added under key.
func (s *subject) seededAs(t *testing.T, key string) mail.MessageMetadata {
	t.Helper()
	m, ok := s.messages[key]
	if !ok {
		t.Fatalf("the case added no message keyed %q", key)
	}
	return m.Metadata
}

func enumerationReturnsEveryMessage(t *testing.T, s *subject) {
	_, _, state := s.enumerate(t, "", mailboxKeys())
	if diff := cmp.Diff(s.expected(), project(state), compare.Options); diff != "" {
		t.Errorf("enumerated messages (-seeded +got):\n%s", diff)
	}
	threads := map[string][]string{}
	for key, m := range state {
		if m.AccountID != Account {
			t.Errorf("message %q carries the account %q, want %q", key, m.AccountID, Account)
		}
		if m.SizeBytes <= 0 {
			t.Errorf("message %q has the size %d", key, m.SizeBytes)
		}
		if bodyMarker := strings.SplitN(s.messages[key].Body.Text, "\n", 2)[0]; !strings.Contains(m.Snippet, bodyMarker) {
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

// enumerationResumes restarts enumeration from page tokens an enumeration issued before it had seen
// every message the case added, and requires the case's messages of the pages after each, no more
// and no fewer. It restarts from the first, the middle and the last of those tokens, so the pages it
// reads grow with the number of pages rather than with its square.
func enumerationResumes(t *testing.T, s *subject) {
	pages, tokens, _ := s.enumerate(t, "", mailboxKeys())
	for _, i := range spread(len(tokens) - 1) {
		token := tokens[i]
		want := slices.Sorted(slices.Values(slices.Concat(pages[i+1:]...)))
		rest, _, _ := s.enumerate(t, token, want)
		if diff := cmp.Diff(want, slices.Sorted(slices.Values(slices.Concat(rest...))), compare.Options); diff != "" {
			t.Errorf("enumerating from the page token after page %d (-the later pages of a full enumeration +got):\n%s", i+1, diff)
		}
	}
}

// spread returns the first, the middle and the last of n indices, each once, and none for none.
func spread(n int) []int {
	if n <= 0 {
		return nil
	}
	var out []int
	for _, i := range []int{0, n / 2, n - 1} {
		if i >= 0 && !slices.Contains(out, i) {
			out = append(out, i)
		}
	}
	return out
}

// threadPages pages through ListThreads for q from page to the end of the listing, returning each
// page's threads by the key of the thread their messages were added in, and each page's Next. It
// checks each thread carries the account and every message of its thread as added. A thread
// holding none of the case's messages is passed over.
func (s *subject) threadPages(t *testing.T, q mail.Query, page mail.PageToken) ([][]string, []mail.PageToken) {
	t.Helper()
	members := map[string][]string{}
	for _, key := range mailboxKeys() {
		thread := s.messages[key].Metadata.ThreadID
		members[thread] = append(members[thread], key)
	}
	var pages [][]string
	var tokens []mail.PageToken
	issued := map[mail.PageToken]bool{}
	for range maxPages {
		s.inTime(t)
		got, err := s.port.ListThreads(t.Context(), q, page)
		if err != nil {
			t.Fatalf("ListThreads: %v", err)
		}
		var threads []string
		for _, th := range got.Items {
			if s.scope != "" && !slices.ContainsFunc(th.Messages, func(m mail.MessageMetadata) bool { return slices.Contains(m.Labels, s.scope) }) {
				t.Errorf("a scoped listing returned a thread none of whose messages carries the case's label")
			}
			var held []string
			for _, m := range th.Messages {
				if key, ours := s.key(t, m); ours {
					held = append(held, key)
				}
			}
			if len(held) == 0 {
				continue
			}
			slices.Sort(held)
			thread := s.messages[held[0]].Metadata.ThreadID
			if th.AccountID != Account {
				t.Errorf("thread %q carries the account %q, want %q", thread, th.AccountID, Account)
			}
			if diff := cmp.Diff(members[thread], held, compare.Options); diff != "" {
				t.Errorf("thread %q's messages (-added +got):\n%s", thread, diff)
			}
			threads = append(threads, thread)
		}
		pages = append(pages, threads)
		tokens = append(tokens, got.Next)
		if got.Next == "" {
			return pages, tokens
		}
		if issued[got.Next] {
			t.Fatalf("ListThreads issued one page token twice")
		}
		issued[got.Next] = true
		page = got.Next
	}
	t.Fatalf("ListThreads returned %d pages without ending", maxPages)
	return nil, nil
}

// listThreads returns the threads ListThreads selects for q, by the key of the thread their messages
// were added in.
func (s *subject) listThreads(t *testing.T, q mail.Query) []string {
	t.Helper()
	pages, _ := s.threadPages(t, s.query(q), "")
	return slices.Sorted(slices.Values(slices.Concat(pages...)))
}

// threadListingResumes restarts thread listing from the first, the middle and the last page token a
// full listing issued, and requires the threads of the pages after each, no more and no fewer.
func threadListingResumes(t *testing.T, s *subject) {
	q := s.query(mail.All())
	pages, tokens := s.threadPages(t, q, "")
	var issued []int
	for i, token := range tokens {
		if token != "" {
			issued = append(issued, i)
		}
	}
	for _, j := range spread(len(issued)) {
		i, token := issued[j], tokens[issued[j]]
		rest, _ := s.threadPages(t, q, token)
		want := slices.Sorted(slices.Values(slices.Concat(pages[i+1:]...)))
		if diff := cmp.Diff(want, slices.Sorted(slices.Values(slices.Concat(rest...))), compare.Options); diff != "" {
			t.Errorf("listing threads from the page token after page %d (-the later pages of a full listing +got):\n%s", i+1, diff)
		}
	}
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
	receipt, bank := s.id(t, "receipt"), s.id(t, "bank")
	got, err := s.port.GetMessageMetadata(t.Context(), []string{receipt, "no-such-message", bank})
	if err != nil {
		t.Fatalf("GetMessageMetadata: %v", err)
	}
	type returned struct {
		ID       string
		Metadata mail.MessageMetadata
	}
	exp := s.expected()
	want := []returned{{receipt, exp["receipt"]}, {bank, exp["bank"]}}
	var observed []returned
	for _, m := range got {
		observed = append(observed, returned{m.ID, seededFields(m)})
	}
	if diff := cmp.Diff(want, observed, compare.Options); diff != "" {
		t.Errorf("GetMessageMetadata (-seeded +got):\n%s", diff)
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

func querySelectsThreads(t *testing.T, s *subject) {
	bank, receipt := s.seededAs(t, "bank").From.Email, s.seededAs(t, "receipt").From.Email
	base := s.base
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
		{"after an instant, selecting a thread through its newest message", mail.After(base + 6*step - margin), []string{"newsletter"}},
		{"before an instant", mail.Before(base + step - margin), []string{"bank"}},
		{"after the instant a message is dated, which it includes", mail.After(base + 5*step), []string{"newsletter", "receipt"}},
		{"before the instant a message is dated, which it excludes", mail.Before(base + step), []string{"bank"}},
		{"a window", mail.And(mail.After(base+2*step-margin), mail.Before(base+4*step-margin)), []string{"alpha", "code"}},
		{"a label and a sender", mail.And(mail.InLabel(mail.Inbox), mail.From(bank)), []string{"bank"}},
		{"a label and a sender no message has together", mail.And(mail.InLabel(finance), mail.From(receipt)), nil},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if diff := cmp.Diff(c.want, s.listThreads(t, c.q), compare.Options); diff != "" {
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
	for _, key := range mailboxKeys() {
		id := s.id(t, key)
		got, err := s.port.GetMessageBody(t.Context(), id)
		if err != nil {
			t.Fatalf("GetMessageBody(%q): %v", key, err)
		}
		want := s.messages[key].Body
		want.AccountID, want.MessageID = Account, id
		if diff := cmp.Diff(want, got, compare.Options); diff != "" {
			t.Errorf("GetMessageBody(%q) (-seeded +got):\n%s", key, diff)
		}
	}
}

func unknownBodyNotFound(t *testing.T, s *subject) {
	if _, err := s.port.GetMessageBody(t.Context(), "no-such-message"); !errors.Is(err, mail.ErrNotFound) {
		t.Errorf("GetMessageBody of an unknown message returned %v, want an error wrapping %v", err, mail.ErrNotFound)
	}
}

// labels returns the path of every label the account has. The account may hold labels the suite
// never made, so a caller checks only for the ones it names and prints no other.
func labels(t *testing.T, s *subject) []string {
	t.Helper()
	got, err := s.port.ListLabels(t.Context())
	if err != nil {
		t.Fatalf("ListLabels: %v", err)
	}
	var paths []string
	for _, l := range got {
		if l.AccountID != Account {
			t.Errorf("a label carries the account %q, want %q", l.AccountID, Account)
		}
		paths = append(paths, l.Path)
	}
	return paths
}

func everyLabelListed(t *testing.T, s *subject) {
	got := labels(t, s)
	for _, want := range []string{mail.Inbox, mail.Trash, mail.Spam, finance, reading, receipts} {
		if !slices.Contains(got, want) {
			t.Errorf("ListLabels lacks %q", want)
		}
	}
}

// ensureLabelIsIdempotent ensures two labels twice each. Their names are the same in every run, since
// the port deletes no label and a new name each run would grow the account's labels without end.
// So the first EnsureLabel creates each label only where the account lacks it, as the fake's fresh
// mailbox always does, and a real account after its first run does not.
func ensureLabelIsIdempotent(t *testing.T, s *subject) {
	ensured := marker.Field("ensuredlabel")
	for _, path := range []string{ensured, ensured + "/" + marker.Field("phaselabel")} {
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

// mutate applies ops and requires each one applied.
func mutate(t *testing.T, s *subject, ops ...mail.MutationOp) {
	t.Helper()
	result, err := s.port.Mutate(t.Context(), ops)
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if diff := cmp.Diff(make([]error, len(ops)), result.Errors, compare.Options); diff != "" {
		t.Errorf("Mutate's outcomes (-each applied +got):\n%s", diff)
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
			// The trashed message is one the case added, and it stays in the trash for the provider
			// to purge (ADR-0043).
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
			s := s.runner.subject(t, seeded, false)
			if _, err := s.port.EnsureLabel(t.Context(), newLabel); err != nil {
				t.Fatalf("EnsureLabel: %v", err)
			}
			mutate(t, s, c.ops(t, s)...)
			want := s.expected()
			c.want(want)
			if diff := cmp.Diff(want, project(s.state(t)), compare.Options); diff != "" {
				t.Errorf("the mailbox after the mutation (-want +got):\n%s", diff)
			}
		})
	}
}

// set edits the message at key in state, sorting its labels as seededFields does.
func set(state map[string]mail.MessageMetadata, key string, edit func(*mail.MessageMetadata)) {
	m := state[key]
	edit(&m)
	state[key] = seededFields(m)
}

// labelMustExist labels with a label named for the case, which no case creates.
func labelMustExist(t *testing.T, s *subject) {
	op := built(t)(mail.LabelOp(s.id(t, "code"), marker.Field("missinglabel"+s.tag)))
	result, err := s.port.Mutate(t.Context(), []mail.MutationOp{op})
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], mail.ErrNotFound) {
		t.Errorf("Mutate's outcomes are %v, want one wrapping %v", result.Errors, mail.ErrNotFound)
	}
	if diff := cmp.Diff(s.expected(), project(s.state(t)), compare.Options); diff != "" {
		t.Errorf("the mailbox after the refused op (-seeded +got):\n%s", diff)
	}
}

// removingAnAbsentLabel removes a label a message lacks, and a label named for the case, which no
// case creates. Only a label an op adds must exist (ADR-0010), so each op applies and changes
// nothing it does not add, and an op on an unknown message is still not found.
func removingAnAbsentLabel(t *testing.T, s *subject) {
	absent := marker.Field("absentlabel" + s.tag)
	code, link := s.id(t, "code"), s.id(t, "link")
	mutate(t, s, built(t)(mail.UnlabelOp(code, finance)), built(t)(mail.UnlabelOp(code, absent)))
	mutate(t, s, built(t)(mail.MoveOp(link, absent, finance)))
	result, err := s.port.Mutate(t.Context(), []mail.MutationOp{built(t)(mail.UnlabelOp("no-such-message", absent))})
	if err != nil {
		t.Fatalf("Mutate: %v", err)
	}
	if len(result.Errors) != 1 || !errors.Is(result.Errors[0], mail.ErrNotFound) {
		t.Errorf("removing an absent label from an unknown message had the outcomes %v, want one wrapping %v", result.Errors, mail.ErrNotFound)
	}
	want := s.expected()
	set(want, "link", func(x *mail.MessageMetadata) { x.Labels = []string{finance} })
	if diff := cmp.Diff(want, project(s.state(t)), compare.Options); diff != "" {
		t.Errorf("the mailbox after removing absent labels (-want +got):\n%s", diff)
	}
	if slices.Contains(labels(t, s), absent) {
		t.Errorf("removing the label %q created it", absent)
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
	want := s.expected()
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
	if diff := cmp.Diff(s.expected(), project(s.state(t)), compare.Options); diff != "" {
		t.Errorf("the mailbox after the refused batch (-seeded +got):\n%s", diff)
	}
}

// drain asks for changes from c until a change set comes back empty, and returns the changes to the
// case's messages together with the cursor it ended at. A change to any other message is passed
// over.
func drain(t *testing.T, s *subject, c mail.Cursor) (changes, mail.Cursor) {
	t.Helper()
	ours := func(ids []string) []string {
		var out []string
		for _, id := range ids {
			if _, ok := s.keys[id]; ok {
				out = append(out, id)
			}
		}
		return out
	}
	var out changes
	for range maxPages {
		s.inTime(t)
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
		out.Added = append(out.Added, ours(got.Added)...)
		out.Modified = append(out.Modified, ours(got.Modified)...)
		out.Removed = append(out.Removed, ours(got.Removed)...)
	}
	t.Fatalf("ChangesSince never came back empty")
	return changes{}, ""
}

type changes struct{ Added, Modified, Removed []string }

func cursorNow(t *testing.T, s *subject) mail.Cursor {
	t.Helper()
	c, err := s.port.CurrentCursor(t.Context())
	if err != nil {
		t.Fatalf("CurrentCursor: %v", err)
	}
	return c
}

func noActivityNoChanges(t *testing.T, s *subject) {
	got, _ := drain(t, s, cursorNow(t, s))
	if diff := cmp.Diff(changes{}, got, compare.Options); diff != "" {
		t.Errorf("changes with no activity (-want +got):\n%s", diff)
	}
}

func changeReported(t *testing.T, s *subject) {
	code := s.id(t, "code")
	c := cursorNow(t, s)
	mutate(t, s, built(t)(mail.StarOp(code)))
	now := cursorNow(t, s)
	got, next := drain(t, s, c)
	if diff := cmp.Diff(changes{Modified: []string{code}}, got, compare.Options); diff != "" {
		t.Errorf("changes after starring one message (-want +got):\n%s", diff)
	}
	again, _ := drain(t, s, next)
	if diff := cmp.Diff(changes{}, again, compare.Options); diff != "" {
		t.Errorf("changes after the reported ones (-want +got):\n%s", diff)
	}
	// A cursor taken after the change is the mailbox as it is now, so nothing follows it.
	fresh, _ := drain(t, s, now)
	if diff := cmp.Diff(changes{}, fresh, compare.Options); diff != "" {
		t.Errorf("changes after a cursor taken once the change was made (-want +got):\n%s", diff)
	}
}

func deliveryReported(t *testing.T, s *subject) {
	c := cursorNow(t, s)
	arrived := s.deliver(t, late(s.mark, s.base))
	got, _ := drain(t, s, c)
	if diff := cmp.Diff(changes{Added: []string{arrived}}, got, compare.Options); diff != "" {
		t.Errorf("changes after one delivery (-want +got):\n%s", diff)
	}
	state := s.state(t)
	if diff := cmp.Diff(seededFields(late(s.mark, s.base).Metadata), seededFields(state["late"]), compare.Options); diff != "" {
		t.Errorf("the delivered message (-as delivered +got):\n%s", diff)
	}
}

func removalReported(t *testing.T, s *subject) {
	bank := s.id(t, "bank")
	c := cursorNow(t, s)
	if err := s.impl.Remove(bank); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got, _ := drain(t, s, c)
	if diff := cmp.Diff(changes{Removed: []string{bank}}, got, compare.Options); diff != "" {
		t.Errorf("changes after one removal (-want +got):\n%s", diff)
	}
	if _, ok := s.state(t)["bank"]; ok {
		t.Error("the removed message's metadata is still returned")
	}
	if _, err := s.port.GetMessageBody(t.Context(), bank); !errors.Is(err, mail.ErrNotFound) {
		t.Errorf("the removed message's body returned %v, want an error wrapping %v", err, mail.ErrNotFound)
	}
}

// changedDeliveryIsAdded delivers a message and changes it after one cursor. It is only added, as
// ChangeSet states.
func changedDeliveryIsAdded(t *testing.T, s *subject) {
	c := cursorNow(t, s)
	arrived := s.deliver(t, late(s.mark, s.base))
	mutate(t, s, built(t)(mail.StarOp(arrived)))
	got, _ := drain(t, s, c)
	if diff := cmp.Diff(changes{Added: []string{arrived}}, got, compare.Options); diff != "" {
		t.Errorf("netted changes (-want +got):\n%s", diff)
	}
}

// removedDeliveryIsRemoved delivers a message and removes it after one cursor. It is only removed,
// as ChangeSet states.
func removedDeliveryIsRemoved(t *testing.T, s *subject) {
	c := cursorNow(t, s)
	gone := s.deliver(t, brief(s.mark, s.base))
	if err := s.impl.Remove(gone); err != nil {
		t.Fatalf("Remove: %v", err)
	}
	got, _ := drain(t, s, c)
	if diff := cmp.Diff(changes{Removed: []string{gone}}, got, compare.Options); diff != "" {
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

// RunProfile runs the contract's rate-profile cases against p alone. A profile is pure, so an
// adapter's tests run it without the credentials its port needs, and a profile that breaks the
// contract fails where every change runs rather than only in the run against the real provider.
func RunProfile(t *testing.T, p mail.RateLimitProfile[context.Context]) {
	t.Helper()
	t.Run("the rate profile declares a budget and a cost for every operation", func(t *testing.T) { profileDeclares(t, p) })
	t.Run("every call the rate profile sizes fits one second at the hard cap", func(t *testing.T) { profileFitsTheHardCap(t, p) })
	t.Run("the rate profile recognises the port's throttle", func(t *testing.T) { profileParsesThrottles(t, p) })
}

func rateProfileDeclares(t *testing.T, s *subject) { profileDeclares(t, s.port.RateProfile()) }

func rateProfileFitsTheHardCap(t *testing.T, s *subject) {
	profileFitsTheHardCap(t, s.port.RateProfile())
}

// profileFitsTheHardCap requires every operation naming no message, and every operation naming one,
// to cost no more than one second's worth at the hard cap, the hard-cap fraction of the budget the
// profile declares (ADR-0023, ADR-0024). A call costing more is refused at lease issuance every time
// it is asked for, so an implementation must size its pages, and allow the smallest call a caller
// can make, within it. A caller splits a larger call by asking the profile what fits.
func profileFitsTheHardCap(t *testing.T, p mail.RateLimitProfile[context.Context]) {
	limit := mail.HardCapFraction * p.BudgetPerSecond()
	for _, op := range operations {
		for _, n := range []int{0, 1} {
			if c := p.Cost(mail.ProviderOp{Operation: op, Messages: n}); c.Weight > limit {
				t.Errorf("Cost of operation %d naming %d messages is %v, past one second's worth at the hard cap, %v", op, n, c.Weight, limit)
			}
		}
	}
}

func rateProfileParsesThrottles(t *testing.T, s *subject) {
	profileParsesThrottles(t, s.port.RateProfile())
}

// profileDeclares requires a positive budget and a positive cost for every operation naming any
// number of messages, none included. The rate limiter refuses a call costing nothing (ADR-0024),
// so an operation priced at nothing could never be issued.
func profileDeclares(t *testing.T, p mail.RateLimitProfile[context.Context]) {
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

func profileParsesThrottles(t *testing.T, p mail.RateLimitProfile[context.Context]) {
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
	cancelledLabel := marker.Field("cancelledlabel" + s.tag)
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	calls := map[string]func() error{
		"ListThreads":        func() error { _, err := s.port.ListThreads(cancelled, mail.All(), ""); return err },
		"GetThreadMetadata":  func() error { _, err := s.port.GetThreadMetadata(cancelled, "no-such-thread"); return err },
		"GetMessageMetadata": func() error { _, err := s.port.GetMessageMetadata(cancelled, []string{code}); return err },
		"GetMessageBody":     func() error { _, err := s.port.GetMessageBody(cancelled, code); return err },
		"ListLabels":         func() error { _, err := s.port.ListLabels(cancelled); return err },
		"EnsureLabel":        func() error { _, err := s.port.EnsureLabel(cancelled, cancelledLabel); return err },
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
	if diff := cmp.Diff(s.expected(), project(s.state(t)), compare.Options); diff != "" {
		t.Errorf("the mailbox after the cancelled calls (-seeded +got):\n%s", diff)
	}
	if slices.Contains(labels(t, s), cancelledLabel) {
		t.Errorf("a cancelled EnsureLabel created its label")
	}
}
