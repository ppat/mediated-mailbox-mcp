// Package fake is the provider fake. It implements the Provider Port in memory for tests, and it
// passes the contract suite in provider/contract like every other implementation (ADR-0043).
//
// A Fake holds one account's mailbox. Beyond the port it offers what a test needs to play the
// provider's own part, delivering mail, removing it and expiring change cursors. Throttle wraps any
// implementation of the port with a schedule of refusals, which makes a Fake the rate limiter's
// simulated provider that throttles on schedule, the same contract plus a schedule.
//
// It is test-only. The import list for non-test code refuses it, so no deployable's running code
// can reach it.
package fake

import (
	"context"
	"errors"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/ppat/mediated-mailbox-mcp/core/authorize"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
)

// Config sets up a Fake.
type Config struct {
	// Account is the account the mailbox belongs to. Every value the Fake returns carries it.
	Account string
	// PageSize is how many messages a page of EnumerateAll holds, and how many threads a page of
	// ListThreads holds. Zero means 100.
	PageSize int
	// BudgetPerSecond is the ceiling the Fake's rate profile declares. Zero means 100.
	BudgetPerSecond float64
}

// Message is one message as the Fake holds it.
//
// Metadata.ID and Metadata.ThreadID must be set, and the Fake keeps them. The Fake sets the account
// on both values and the message identifier on the body. It derives the snippet, the first 100
// characters of the text part, and the size, when either is left zero, as a provider would.
type Message struct {
	Metadata mail.MessageMetadata
	Body     mail.MessageBody
}

// Fake is an in-memory mailbox implementing the Provider Port. It is safe for concurrent use.
type Fake struct {
	mu       sync.Mutex
	account  string
	pageSize int
	profile  Profile
	seq      int
	byID     map[string]*stored
	labels   map[string]bool
	history  []change
	horizon  int
}

type stored struct {
	seq  int
	meta mail.MessageMetadata
	body mail.MessageBody
}

type changeKind uint8

const (
	added changeKind = iota + 1
	modified
	removed
)

type change struct {
	id   string
	kind changeKind
}

var _ mail.Port[context.Context] = (*Fake)(nil)

// New returns a Fake holding messages, in the order given.
func New(cfg Config, messages ...Message) (*Fake, error) {
	if cfg.Account == "" {
		return nil, errors.New("fake: the account is empty")
	}
	f := &Fake{
		account:  cfg.Account,
		pageSize: cfg.PageSize,
		profile:  Profile{budget: cfg.BudgetPerSecond},
		byID:     map[string]*stored{},
		labels:   map[string]bool{mail.Inbox: true, mail.Trash: true, mail.Spam: true},
	}
	if f.pageSize <= 0 {
		f.pageSize = 100
	}
	if f.profile.budget <= 0 {
		f.profile.budget = 100
	}
	for _, m := range messages {
		if err := f.insert(m); err != nil {
			return nil, err
		}
	}
	return f, nil
}

// Deliver adds a message to the mailbox, as new mail arriving. Delta sync sees it as added.
func (f *Fake) Deliver(m Message) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := f.insert(m); err != nil {
		return err
	}
	f.history = append(f.history, change{id: m.Metadata.ID, kind: added})
	return nil
}

// Remove deletes a message from the mailbox, as a deletion made outside this system. Delta sync sees
// it as removed.
func (f *Fake) Remove(id string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if _, ok := f.byID[id]; !ok {
		return fmt.Errorf("fake: message %q: %w", id, mail.ErrNotFound)
	}
	delete(f.byID, id)
	f.history = append(f.history, change{id: id, kind: removed})
	return nil
}

// ExpireCursors makes every cursor issued before now one the Fake cannot calculate changes from, as
// a provider does with a cursor that has aged out (ADR-0018).
func (f *Fake) ExpireCursors() {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.horizon = len(f.history)
}

func (f *Fake) insert(m Message) error {
	meta, body := m.Metadata, m.Body
	switch {
	case meta.ID == "":
		return errors.New("fake: a message has no identifier")
	case meta.ThreadID == "":
		return fmt.Errorf("fake: message %q has no thread", meta.ID)
	}
	if _, ok := f.byID[meta.ID]; ok {
		return fmt.Errorf("fake: message %q is already in the mailbox", meta.ID)
	}
	meta = clone(meta)
	meta.AccountID = f.account
	body.AccountID, body.MessageID = f.account, meta.ID
	if meta.Snippet == "" {
		meta.Snippet = snippet(body.Text)
	}
	if meta.SizeBytes == 0 {
		meta.SizeBytes = int64(len(meta.From.Email) + len(meta.Subject) + len(body.Text) + len(body.HTML))
	}
	for _, l := range meta.Labels {
		f.labels[l] = true
	}
	f.seq++
	f.byID[meta.ID] = &stored{seq: f.seq, meta: meta, body: body}
	return nil
}

func snippet(text string) string {
	runes := []rune(text)
	if len(runes) > 100 {
		runes = runes[:100]
	}
	return string(runes)
}

// ListThreads implements the port.
func (f *Fake) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	if err := ctx.Err(); err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	if !q.Valid() {
		return mail.Page[mail.ThreadMetadata]{}, fmt.Errorf("fake: the query is not valid: %w", mail.ErrInvalid)
	}
	start, err := parseToken(page, "t")
	if err != nil {
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out mail.Page[mail.ThreadMetadata]
	for _, th := range f.threads() {
		first := th[0].seq
		if first < start || !slices.ContainsFunc(th, func(s *stored) bool { return matches(q, s.meta) }) {
			continue
		}
		if len(out.Items) == f.pageSize {
			out.Next = mail.PageToken("t" + strconv.Itoa(first))
			break
		}
		out.Items = append(out.Items, f.thread(th))
	}
	return out, nil
}

// GetThreadMetadata implements the port.
func (f *Fake) GetThreadMetadata(ctx context.Context, threadID string) (mail.ThreadMetadata, error) {
	if err := ctx.Err(); err != nil {
		return mail.ThreadMetadata{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, th := range f.threads() {
		if th[0].meta.ThreadID == threadID {
			return f.thread(th), nil
		}
	}
	return mail.ThreadMetadata{}, fmt.Errorf("fake: thread %q: %w", threadID, mail.ErrNotFound)
}

// GetMessageMetadata implements the port.
func (f *Fake) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out []mail.MessageMetadata
	for _, id := range ids {
		if s, ok := f.byID[id]; ok {
			out = append(out, clone(s.meta))
		}
	}
	return out, nil
}

// GetMessageBody implements the port.
func (f *Fake) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	if err := ctx.Err(); err != nil {
		return mail.MessageBody{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	s, ok := f.byID[id]
	if !ok {
		return mail.MessageBody{}, fmt.Errorf("fake: message %q: %w", id, mail.ErrNotFound)
	}
	return s.body, nil
}

// ListLabels implements the port.
func (f *Fake) ListLabels(ctx context.Context) ([]mail.Label, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	paths := make([]string, 0, len(f.labels))
	for p := range f.labels {
		paths = append(paths, p)
	}
	sort.Strings(paths)
	out := make([]mail.Label, len(paths))
	for i, p := range paths {
		out[i] = mail.Label{AccountID: f.account, Path: p}
	}
	return out, nil
}

// EnsureLabel implements the port.
func (f *Fake) EnsureLabel(ctx context.Context, path string) (mail.Label, error) {
	if err := ctx.Err(); err != nil {
		return mail.Label{}, err
	}
	if path == "" {
		return mail.Label{}, fmt.Errorf("fake: the label path is empty: %w", mail.ErrInvalid)
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	f.labels[path] = true
	return mail.Label{AccountID: f.account, Path: path}, nil
}

// Mutate implements the port. It applies every op it can and reports the rest, so the per-op
// outcomes the port promises are observable.
func (f *Fake) Mutate(ctx context.Context, ops []mail.MutationOp) (mail.MutationResult, error) {
	if err := ctx.Err(); err != nil {
		return mail.MutationResult{}, err
	}
	for i, op := range ops {
		if !op.Built() {
			return mail.MutationResult{}, fmt.Errorf("fake: op %d was not built by a constructor: %w", i, mail.ErrInvalid)
		}
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	result := mail.MutationResult{Errors: make([]error, len(ops))}
	for i, op := range ops {
		result.Errors[i] = f.apply(op)
	}
	return result, nil
}

func (f *Fake) apply(op mail.MutationOp) error {
	s, ok := f.byID[op.MessageID()]
	if !ok {
		return fmt.Errorf("fake: message %q: %w", op.MessageID(), mail.ErrNotFound)
	}
	// Only a label an op adds must exist (ADR-0010). Removing one the message or the account lacks
	// changes nothing and succeeds.
	if add := op.AddLabel(); add != "" && !f.labels[add] {
		return fmt.Errorf("fake: label %q: %w", add, mail.ErrNotFound)
	}
	before := clone(s.meta)
	switch op.Verb() {
	case authorize.Label, authorize.Unlabel, authorize.Move:
		s.meta.Labels = relabel(s.meta.Labels, op.AddLabel(), op.RemoveLabel())
	case authorize.Archive:
		s.meta.Labels = relabel(s.meta.Labels, "", mail.Inbox)
	case authorize.Trash:
		s.meta.Labels = relabel(s.meta.Labels, mail.Trash, mail.Inbox)
	case authorize.Spam:
		s.meta.Labels = relabel(s.meta.Labels, mail.Spam, mail.Inbox)
	case authorize.MarkRead:
		s.meta.Flags.Read = true
	case authorize.Star:
		s.meta.Flags.Starred = true
	default:
		return fmt.Errorf("fake: verb %s: %w", op.Verb(), mail.ErrInvalid)
	}
	if !slices.Equal(before.Labels, s.meta.Labels) || before.Flags != s.meta.Flags {
		f.history = append(f.history, change{id: s.meta.ID, kind: modified})
	}
	return nil
}

func relabel(labels []string, add, remove string) []string {
	out := slices.DeleteFunc(slices.Clone(labels), func(l string) bool { return l == remove })
	if add != "" && !slices.Contains(out, add) {
		out = append(out, add)
	}
	return out
}

// CurrentCursor implements the port.
func (f *Fake) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	return cursor(len(f.history)), nil
}

// ChangesSince implements the port. It returns every change after the cursor in one change set.
func (f *Fake) ChangesSince(ctx context.Context, c mail.Cursor) (mail.ChangeSet, error) {
	if err := ctx.Err(); err != nil {
		return mail.ChangeSet{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	n, err := strconv.Atoi(strings.TrimPrefix(string(c), "h"))
	if err != nil || !strings.HasPrefix(string(c), "h") || n < f.horizon || n > len(f.history) {
		return mail.ChangeSet{}, fmt.Errorf("fake: cursor %q: %w", c, mail.ErrCursorGap)
	}
	var order []string
	kinds := map[string]map[changeKind]bool{}
	for _, ch := range f.history[n:] {
		if kinds[ch.id] == nil {
			kinds[ch.id] = map[changeKind]bool{}
			order = append(order, ch.id)
		}
		kinds[ch.id][ch.kind] = true
	}
	out := mail.ChangeSet{AccountID: f.account, Next: cursor(len(f.history))}
	for _, id := range order {
		switch k := kinds[id]; {
		case k[removed]:
			out.Removed = append(out.Removed, id)
		case k[added]:
			out.Added = append(out.Added, id)
		default:
			out.Modified = append(out.Modified, id)
		}
	}
	return out, nil
}

func cursor(n int) mail.Cursor { return mail.Cursor("h" + strconv.Itoa(n)) }

// EnumerateAll implements the port.
func (f *Fake) EnumerateAll(ctx context.Context, page mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	if err := ctx.Err(); err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	start, err := parseToken(page, "m")
	if err != nil {
		return mail.Page[mail.MessageMetadata]{}, err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	var out mail.Page[mail.MessageMetadata]
	for _, s := range f.ordered() {
		if s.seq < start {
			continue
		}
		if len(out.Items) == f.pageSize {
			out.Next = mail.PageToken("m" + strconv.Itoa(s.seq))
			break
		}
		out.Items = append(out.Items, clone(s.meta))
	}
	return out, nil
}

// RateProfile implements the port.
func (f *Fake) RateProfile() mail.RateLimitProfile[context.Context] { return f.profile }

// parseToken reads a page token the Fake issued, which is prefix followed by the sequence number to
// resume from. The empty token starts at the beginning.
func parseToken(page mail.PageToken, prefix string) (int, error) {
	if page == "" {
		return 0, nil
	}
	n, err := strconv.Atoi(strings.TrimPrefix(string(page), prefix))
	if err != nil || !strings.HasPrefix(string(page), prefix) || n <= 0 {
		return 0, fmt.Errorf("fake: page token %q: %w", page, mail.ErrInvalid)
	}
	return n, nil
}

// ordered returns the stored messages in the order they entered the mailbox.
func (f *Fake) ordered() []*stored {
	out := make([]*stored, 0, len(f.byID))
	for _, s := range f.byID {
		out = append(out, s)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].seq < out[j].seq })
	return out
}

// threads returns the stored messages grouped by thread, each thread oldest first, the threads in
// the order their first message entered the mailbox.
func (f *Fake) threads() [][]*stored {
	var ids []string
	groups := map[string][]*stored{}
	for _, s := range f.ordered() {
		if groups[s.meta.ThreadID] == nil {
			ids = append(ids, s.meta.ThreadID)
		}
		groups[s.meta.ThreadID] = append(groups[s.meta.ThreadID], s)
	}
	out := make([][]*stored, len(ids))
	for i, id := range ids {
		th := groups[id]
		sort.SliceStable(th, func(a, b int) bool { return th[a].meta.Date < th[b].meta.Date })
		out[i] = th
	}
	return out
}

func (f *Fake) thread(th []*stored) mail.ThreadMetadata {
	out := mail.ThreadMetadata{AccountID: f.account, ID: th[0].meta.ThreadID}
	for _, s := range th {
		out.Messages = append(out.Messages, clone(s.meta))
	}
	return out
}

func matches(q mail.Query, m mail.MessageMetadata) bool {
	switch q.Kind() {
	case mail.QueryAll:
		return true
	case mail.QueryAfter:
		return m.Date >= q.Instant()
	case mail.QueryBefore:
		return m.Date < q.Instant()
	case mail.QueryInLabel:
		return slices.Contains(m.Labels, q.Label())
	case mail.QueryFrom:
		return strings.EqualFold(m.From.Email, q.Address())
	case mail.QueryAnd:
		for _, o := range q.Operands() {
			if !matches(o, m) {
				return false
			}
		}
		return true
	default:
		return false
	}
}

// clone copies m so neither the caller nor the Fake can change the other's value through a shared
// slice.
func clone(m mail.MessageMetadata) mail.MessageMetadata {
	m.To = slices.Clone(m.To)
	m.Cc = slices.Clone(m.Cc)
	m.Labels = slices.Clone(m.Labels)
	m.AttachmentNames = slices.Clone(m.AttachmentNames)
	return m
}

// Profile is the Fake's rate profile. A call weighs one unit per message it names, and at least one.
type Profile struct {
	budget float64
}

// Cost implements the rate profile.
func (p Profile) Cost(op mail.ProviderOp) mail.OpCost {
	return mail.OpCost{Weight: float64(max(op.Messages, 1)), OpsCount: op.Messages}
}

// BudgetPerSecond implements the rate profile.
func (p Profile) BudgetPerSecond() float64 { return p.budget }

// ParseThrottle implements the rate profile. The Fake's throttles are the port's own ThrottleError.
func (p Profile) ParseThrottle(err error) (mail.ThrottleSignal, bool) { return mail.Throttled(err) }

// RefreshLimits implements the rate profile. The Fake's limits are fixed.
func (p Profile) RefreshLimits(ctx context.Context) error { return ctx.Err() }
