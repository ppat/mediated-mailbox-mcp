//go:build integration

package pass1_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// tb is what the helpers below need of a test, which a testing.T and a rapid.T both are.
type tb interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
	Failed() bool
}

// The role backfill connects as, so row-level security and its grants apply as in production.
const role = "mediated_mailbox_backfill"

var (
	pools    atomic.Pointer[pgxpool.Pool]
	accounts atomic.Int64
)

// backfillPool returns the package's pool connecting as role, shared by every world.
func backfillPool(t testing.TB) *pgxpool.Pool {
	t.Helper()
	if p := pools.Load(); p != nil {
		return p
	}
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + role
	p, err := pgxpool.NewWithConfig(context.Background(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !pools.CompareAndSwap(nil, p) {
		p.Close()
	}
	return pools.Load()
}

// superuser returns a connection that bypasses row-level security, to write what the UI's account
// setup would write and to read what a check reads.
func superuser(t testing.TB) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(context.Background(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	return conn
}

// The senders a generated mailbox draws from. The first is listed by the policy, and the last has no
// domain the classifier can read.
var senderDomains = []string{"bank.example", "news.example", "shop.example", ""}

const listedDomain = "bank.example"

// listedRule is the policy rule that lists listedDomain, which its messages name as the rule that set
// their class (ADR-0016).
const listedRule = "rule.bank"

// setup is what a world is built from, kept whole by the failing-case store, so every field is a
// plain value.
type setup struct {
	// PageSize is how many messages a page of the mailbox's enumeration holds.
	PageSize int
	Messages []drawn
	// Totals has the provider report a total on every page of the enumeration.
	Totals bool
}

// drawn is one message of a generated mailbox.
type drawn struct {
	// Sender indexes senderDomains.
	Sender int
	// Code puts a one-time code in the subject, which masking replaces.
	Code bool
	// Inbox and ListID give the message the inbox label and a list identifier.
	Inbox, ListID bool
}

// tag returns a marker tag for n, lowercase letters only (ADR-0044).
func tag(n int) string {
	s := ""
	for {
		s = string(rune('a'+n%26)) + s
		n /= 26
		if n == 0 {
			return s
		}
	}
}

// expected is one message as the index must hold it, known from how it was generated. ClassRule is
// nil for a class no rule set, which the index holds as NULL.
type expected struct {
	ID, Domain, Subject, Class string
	ClassRule                  *string
	Masks                      int
}

// mailbox returns the generated mailbox as the provider fake's messages, with what the index must hold
// for each.
func mailbox(s setup) ([]fake.Message, []expected) {
	var messages []fake.Message
	var want []expected
	for i, d := range s.Messages {
		id := fmt.Sprintf("m%d", i+1)
		domain := senderDomains[d.Sender%len(senderDomains)]
		address := marker.Field("from"+tag(i)) + "@" + domain
		if domain == "" {
			address = marker.Field("from" + tag(i))
		}
		subject := marker.Field("subject" + tag(i))
		stored, masks := subject, 0
		if d.Code {
			subject += " Your code is 419283"
			stored, masks = subject[:len(subject)-6]+"██████", 1
		}
		m := mail.MessageMetadata{
			ID:       id,
			ThreadID: "t" + id,
			From:     mail.Address{Email: address, Name: marker.Field("name" + tag(i))},
			Subject:  subject,
			Date:     mail.UnixMilli(1_700_000_000_000 + int64(i)*60_000),
		}
		if d.Inbox {
			m.Labels = []string{mail.Inbox}
		}
		if d.ListID {
			m.ListID = marker.Field("list"+tag(i)) + ".news.example"
		}
		class := "normal"
		var rule *string
		if domain == listedDomain || domain == "" {
			class = "restricted"
		}
		if domain == listedDomain {
			listed := listedRule
			rule = &listed
		}
		messages = append(messages, fake.Message{Metadata: m})
		want = append(want, expected{ID: id, Domain: domain, Subject: stored, Class: class, ClassRule: rule, Masks: masks})
	}
	return messages, want
}

// state is what the index holds for one account, read as plain values, the same way from the model and
// from PostgreSQL, so the checks never reach inside either (ADR-0045).
type state struct {
	Ended    bool
	Runs     []run
	Messages map[string]stored
	// Events counts the masking events recorded for each message.
	Events  map[string]int
	Senders map[string]sender
}

type run struct {
	ID, State, ResumedFrom string
	Progress               core.Progress
}

type stored struct {
	Domain, Subject, Class string
	ClassRule              *string
}

type sender struct {
	Count int
	Class string
}

// latest returns the latest run, and false when there is none.
func (s state) latest() (run, bool) {
	if len(s.Runs) == 0 {
		return run{}, false
	}
	return s.Runs[len(s.Runs)-1], true
}

// world is pass 1 over one account, its store, the provider fake it enumerates and the process that
// runs it. A crash drops the process, and recovery opens a new one.
type world struct {
	ctx     context.Context
	account string
	store   pass1.Store
	inspect func(t tb) state
	deps    pass1.Deps
	pass    *pass1.Pass
	want    []expected
	// throttle is how many calls the provider throttles next, and refuse how many calls it refuses
	// next as malformed, as it refuses a page token it no longer honours.
	throttle, refuse int
	// cut crashes the process inside the next page, once it has fetched the page and before its
	// commit ends, and cutAfter once its commit has ended and before the run records anything more.
	cut, cutAfter bool
	cancel        context.CancelFunc
	// What the world was told. Durable are the messages the index held after the last step that made
	// a page durable, and reported is the progress that step reported.
	durable  []string
	reported core.Progress
	// fetched counts the pages the provider returned, committed the pages made durable, and crashes
	// the crashes.
	fetched, committed, crashes int
	// unclassified sums the messages with an unclassified sender the steps counted.
	unclassified int
	runs         int
	pageSize     int
}

// newWorld builds a world over store, enumerating the mailbox s generates. port turns the throttled
// fake into the Fetch the pass uses.
func newWorld(t tb, s setup, account string, store pass1.Store, inspect func(tb) state, port func(mail.Port[context.Context]) pass1.Fetch) *world {
	t.Helper()
	messages, want := mailbox(s)
	f, err := fake.New(fake.Config{Account: account, PageSize: max(s.PageSize, 1), BudgetPerSecond: 10_000, ReportsTotal: s.Totals}, messages...)
	if err != nil {
		t.Fatalf("building the mailbox: %v", err)
	}
	w := &world{ctx: context.Background(), account: account, store: store, inspect: inspect, want: want, pageSize: max(s.PageSize, 1)}
	throttled := fake.Throttle(f, func(fake.Call) error {
		if w.throttle > 0 {
			w.throttle--
			return mail.ThrottleError{Signal: mail.ThrottleSignal{RetryAfterMillis: 1, HasRetryAfter: true}}
		}
		return nil
	}, time.Now)
	inner := port(throttled)
	rules, err := policy.Load([]policy.Row{{ID: listedRule, Class: policy.Restricted, DomainSuffixes: []string{listedDomain}}})
	if err != nil {
		t.Fatalf("loading the policy: %v", err)
	}
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatalf("building the scanner: %v", err)
	}
	w.deps = pass1.Deps{
		Store: cutting{Store: store, w: w},
		Fetch: func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
			if w.refuse > 0 {
				w.refuse--
				return mail.Page[mail.MessageMetadata]{}, fmt.Errorf("the page token %q: %w", token, mail.ErrInvalid)
			}
			page, err := inner(ctx, token)
			if err == nil {
				w.fetched++
			}
			if w.cut {
				w.cut = false
				w.cancel()
			}
			return page, err
		},
		Policy:  rules.For(account),
		Scanner: scanner,
		Lookups: classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
		RunID: func() string {
			w.runs++
			return fmt.Sprintf("run%d", w.runs)
		},
		Now: time.Now,
	}
	return w
}

// open opens a run, the recovery path.
func (w *world) open(t tb) {
	t.Helper()
	p, err := pass1.Open(w.ctx, w.deps, w.account)
	if err != nil {
		t.Fatalf("opening a run: %v", err)
	}
	w.pass = p
}

// step makes one more page durable, opening a run first when no process runs one, as the job is
// started again after a run fails. It reports whether the pass is done.
func (w *world) step(t tb) bool {
	t.Helper()
	if w.pass == nil {
		w.open(t)
	}
	ctx, cancel := context.WithCancel(w.ctx)
	defer cancel()
	w.cancel = cancel
	before := w.pass.Progress()
	s, err := w.pass.Next(ctx)
	// A page whose commit ended is durable, whether or not the step ended well after it.
	if at := w.pass.Progress(); at != before {
		w.committed++
		w.reported = at
		w.durable = slices.Sorted(maps.Keys(w.inspect(t).Messages))
	}
	w.unclassified += s.Unclassified
	if err != nil {
		w.pass = nil
		return false
	}
	return s.Done
}

// The crash points of the backfill target. A crash stops the process between two steps, inside a page
// once it is fetched and before its commit ends, or inside a page once its commit has ended and before
// the run records anything more, such as its end after its last page.
const (
	betweenSteps = 0
	insidePage   = 1
	afterCommit  = 2
)

// crash stops the process at the crash point at.
func (w *world) crash(t tb, at int) {
	t.Helper()
	w.crashes++
	if w.pass != nil && at != betweenSteps {
		w.cut, w.cutAfter = at == insidePage, at == afterCommit
		committed := w.committed
		w.step(t)
		if at == insidePage && w.committed != committed {
			t.Errorf("a page whose process was stopped before its commit ended was made durable")
		}
		w.cut, w.cutAfter = false, false
	}
	w.pass = nil
}

// cutting is the world's store, which stops the process once a commit has ended when the world's crash
// point asks for it.
type cutting struct {
	pass1.Store
	w *world
}

func (c cutting) Commit(ctx context.Context, account, runID string, p core.Page, recovered *pass1.Failure, advance func(int) core.Progress) (pass1.Committed, error) {
	at, err := c.Store.Commit(ctx, account, runID, p, recovered, advance)
	if err == nil && c.w.cutAfter {
		c.w.cutAfter = false
		c.w.cancel()
	}
	return at, err
}

// persistence checks that everything the world was told is durable is intact after recovery, and that
// the recovered run starts from exactly the progress last reported.
func (w *world) persistence(t tb) {
	t.Helper()
	got := w.inspect(t)
	for _, id := range w.durable {
		if _, ok := got.Messages[id]; !ok {
			t.Errorf("message %s was durable before the crash and is gone after it", id)
		}
	}
	w.checkEvents(t, got)
	if got.Ended {
		return
	}
	latest, ok := got.latest()
	if !ok || latest.State != "running" {
		t.Fatalf("after recovery the latest run is %+v, want a running one", latest)
	}
	if diff := cmp.Diff(w.reported, latest.Progress, compare.Options); diff != "" {
		t.Errorf("the recovered run does not start from the progress last reported (-reported +recovered):\n%s", diff)
	}
	for _, r := range got.Runs[:len(got.Runs)-1] {
		if r.State == "running" {
			t.Errorf("run %s is still recorded as running after a later run started", r.ID)
		}
	}
}

// checkEvents checks that each stored message has exactly one masking event per mask its subject
// needed, so no page made durable twice recorded one twice.
func (w *world) checkEvents(t tb, got state) {
	t.Helper()
	for _, m := range w.want {
		if _, ok := got.Messages[m.ID]; ok && got.Events[m.ID] != m.Masks {
			t.Errorf("message %s has %d masking events, want %d", m.ID, got.Events[m.ID], m.Masks)
		}
	}
}

// progress drives the pass until it is done and checks what it left. Every message is indexed once,
// masked and classified, the senders' statistics count every message, the pass is recorded as ended
// with its last run succeeded and none left running, and no crash cost more than one page of rework.
func (w *world) progress(t tb) {
	t.Helper()
	w.throttle = 0
	limit := 2*(len(w.want)/w.pageSize+2) + 2
	done := false
	for range limit {
		if done = w.step(t); done {
			break
		}
	}
	if !done {
		t.Fatalf("the pass did not end within %d steps", limit)
	}
	got := w.inspect(t)
	if !got.Ended {
		t.Errorf("the pass is not recorded as ended")
	}
	for _, r := range got.Runs {
		if r.State == "running" {
			t.Errorf("run %s is left running", r.ID)
		}
	}
	if latest, ok := got.latest(); !ok || latest.State != "succeeded" || latest.Progress.Counters.Messages != len(w.want) {
		t.Errorf("the latest run is %+v, want one that succeeded having added %d messages", latest, len(w.want))
	}
	messages, counts := map[string]stored{}, map[string]sender{}
	for _, m := range w.want {
		messages[m.ID] = stored{Domain: m.Domain, Subject: m.Subject, Class: m.Class, ClassRule: m.ClassRule}
		s := counts[m.Domain]
		s.Count++
		s.Class = m.Class
		counts[m.Domain] = s
	}
	if diff := cmp.Diff(messages, got.Messages, compare.Options); diff != "" {
		t.Errorf("the messages the index holds (-want +got):\n%s", diff)
	}
	w.checkEvents(t, got)
	if diff := cmp.Diff(counts, got.Senders, compare.Options); diff != "" {
		t.Errorf("the senders' statistics (-want +got):\n%s", diff)
	}
	if w.unclassified != w.unclassifiedWant() {
		t.Errorf("the steps counted %d messages with an unclassified sender, want %d, one for each such message", w.unclassified, w.unclassifiedWant())
	}
	if rework := w.fetched - w.committed; rework > w.crashes {
		t.Errorf("%d pages were fetched and %d made durable across %d crashes, more than one page of rework a crash", w.fetched, w.committed, w.crashes)
	}
	if pages := max(1, (len(w.want)+w.pageSize-1)/w.pageSize); w.fetched > pages+w.crashes {
		t.Errorf("the provider returned %d pages for a mailbox of %d across %d crashes, more than one page of rework a crash", w.fetched, pages, w.crashes)
	}
}

// unclassifiedWant counts the mailbox's messages whose sender has no domain the classifier can read.
func (w *world) unclassifiedWant() int {
	n := 0
	for _, m := range w.want {
		if m.Domain == "" {
			n++
		}
	}
	return n
}

// realWorld returns a world over PostgreSQL, for an account of its own, spending through the real rate
// limiter.
func realWorld(t testing.TB, s setup) *world {
	t.Helper()
	conn := superuser(t)
	account := fmt.Sprintf("account%d", accounts.Add(1))
	for _, q := range []string{
		"INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')",
		"INSERT INTO account_state (account_id) VALUES ($1)",
	} {
		if _, err := conn.Exec(context.Background(), q, account); err != nil {
			t.Fatal(err)
		}
	}
	pool := backfillPool(t)
	limiter := lease.New(pool, 10_000, nil)
	url := postgres.URL(t)
	return newWorld(t, s, account, pass1.NewPostgres(pool), func(t tb) state { return inspectPostgres(t, url, account) },
		func(p mail.Port[context.Context]) pass1.Fetch { return pass1.Leased(limiter, p, account) })
}

// inspectPostgres reads the account's state from the database.
func inspectPostgres(t tb, url, account string) state {
	t.Helper()
	ctx := context.Background()
	conn, err := pgx.Connect(ctx, url)
	if err != nil {
		t.Fatalf("connecting: %v", err)
	}
	defer func() {
		if err := conn.Close(ctx); err != nil {
			t.Errorf("closing: %v", err)
		}
	}()
	s := state{Messages: map[string]stored{}, Events: map[string]int{}, Senders: map[string]sender{}}
	if err := conn.QueryRow(ctx, "SELECT backfill_pass1_complete FROM account_state WHERE account_id = $1", account).Scan(&s.Ended); err != nil {
		t.Fatalf("reading the completion flag: %v", err)
	}
	rows, err := conn.Query(ctx, `SELECT run_id, state, coalesce(resumed_from, ''), checkpoint, counters FROM job_runs
		WHERE account_id = $1 AND workload = 'backfill' AND pass = 'pass1' ORDER BY started_at, run_id`, account)
	if err != nil {
		t.Fatalf("reading the runs: %v", err)
	}
	var runs []run
	var cp, ct []byte
	var r run
	_, err = pgx.ForEachRow(rows, []any{&r.ID, &r.State, &r.ResumedFrom, &cp, &ct}, func() error {
		var c struct {
			Page  int    `json:"page"`
			Token string `json:"token"`
		}
		var n struct{ Pages, Messages int }
		if err := json.Unmarshal(cp, &c); err != nil {
			return err
		}
		if err := json.Unmarshal(ct, &n); err != nil {
			return err
		}
		r.Progress = core.Progress{Checkpoint: core.Checkpoint{Page: c.Page, Token: mail.PageToken(c.Token)}, Counters: core.Counters{Pages: n.Pages, Messages: n.Messages}}
		runs = append(runs, r)
		return nil
	})
	if err != nil {
		t.Fatalf("reading the runs: %v", err)
	}
	s.Runs = runs
	var m stored
	var id string
	rows, err = conn.Query(ctx, "SELECT message_id, from_domain::text, subject, sender_class, class_rule_id FROM messages WHERE account_id = $1", account)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	var rule pgtype.Text
	_, err = pgx.ForEachRow(rows, []any{&id, &m.Domain, &m.Subject, &m.Class, &rule}, func() error {
		m.ClassRule = nil
		if rule.Valid {
			r := rule.String
			m.ClassRule = &r
		}
		s.Messages[id] = m
		return nil
	})
	if err != nil {
		t.Fatalf("reading the messages: %v", err)
	}
	var n int
	rows, err = conn.Query(ctx, "SELECT message_id, count(*) FROM masking_events WHERE account_id = $1 GROUP BY message_id", account)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if _, err := pgx.ForEachRow(rows, []any{&id, &n}, func() error { s.Events[id] = n; return nil }); err != nil {
		t.Fatalf("reading the masking events: %v", err)
	}
	var sd sender
	var domain string
	rows, err = conn.Query(ctx, "SELECT domain::text, message_count, sender_class FROM senders WHERE account_id = $1", account)
	if err != nil {
		t.Fatalf("reading: %v", err)
	}
	if _, err := pgx.ForEachRow(rows, []any{&domain, &sd.Count, &sd.Class}, func() error { s.Senders[domain] = sd; return nil }); err != nil {
		t.Fatalf("reading the senders: %v", err)
	}
	return s
}

// memory is the in-memory model of the Store, which the crash harness searches and reduces against
// (ADR-0069). Each method makes its whole change or none, as a transaction does, and a method called
// with a cancelled context changes nothing.
type memory struct {
	ended    bool
	runs     []run
	messages map[string]core.Message
	events   map[string]int
	failures []pass1.Failure
	timeline []pass1.Event
}

var _ pass1.Store = (*memory)(nil)

func newMemory() *memory {
	return &memory{messages: map[string]core.Message{}, events: map[string]int{}}
}

func (m *memory) State(ctx context.Context, _ string) (bool, core.Latest[core.Progress], error) {
	if err := ctx.Err(); err != nil {
		return false, core.Latest[core.Progress]{}, err
	}
	if len(m.runs) == 0 {
		return m.ended, core.Latest[core.Progress]{}, nil
	}
	r := m.runs[len(m.runs)-1]
	states := map[string]core.RunState{"running": core.Running, "succeeded": core.Succeeded, "failed": core.Failed}
	return m.ended, core.Latest[core.Progress]{Found: true, RunID: r.ID, State: states[r.State], Progress: r.Progress}, nil
}

func (m *memory) end(id, st string) {
	for i := range m.runs {
		if m.runs[i].ID == id && m.runs[i].State == "running" {
			m.runs[i].State = st
		}
	}
}

func (m *memory) Start(ctx context.Context, _, runID string, start core.Start[core.Progress]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if start.Abandon {
		m.end(start.ResumedFrom, "failed")
	}
	m.runs = append(m.runs, run{ID: runID, State: "running", ResumedFrom: start.ResumedFrom, Progress: start.From})
	return nil
}

func (m *memory) Commit(ctx context.Context, _, runID string, p core.Page, recovered *pass1.Failure, advance func(int) core.Progress) (pass1.Committed, error) {
	if err := ctx.Err(); err != nil {
		return pass1.Committed{}, err
	}
	var c pass1.Committed
	added := 0
	for _, msg := range p.Messages {
		if _, ok := m.messages[msg.ID]; ok {
			continue
		}
		m.messages[msg.ID] = msg
		m.events[msg.ID] += len(msg.Masks)
		added++
		if msg.Unclassified {
			c.Unclassified++
		}
	}
	if recovered != nil {
		m.failures = append(m.failures, *recovered)
	}
	c.Progress = advance(added)
	for i := range m.runs {
		if m.runs[i].ID == runID {
			m.runs[i].Progress = c.Progress
		}
	}
	return c, nil
}

func (m *memory) Finish(ctx context.Context, _, runID string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.ended = true
	m.end(runID, "succeeded")
	return nil
}

func (m *memory) Fail(ctx context.Context, _, runID, _ string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.end(runID, "failed")
	return nil
}

func (m *memory) Event(ctx context.Context, _, _ string, e pass1.Event) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.timeline = append(m.timeline, e)
	return nil
}

func (m *memory) Failure(ctx context.Context, _, _ string, f pass1.Failure) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.failures = append(m.failures, f)
	return nil
}

// inspect returns the model's state, the senders' statistics taken from the messages it holds, as the
// store rebuilds them.
func (m *memory) inspect(tb) state {
	s := state{Ended: m.ended, Runs: slices.Clone(m.runs), Messages: map[string]stored{}, Events: maps.Clone(m.events), Senders: map[string]sender{}}
	for id, msg := range m.messages {
		st := stored{Domain: msg.Domain, Subject: msg.Subject, Class: string(msg.Class)}
		if msg.ClassRule != "" {
			rule := msg.ClassRule
			st.ClassRule = &rule
		}
		s.Messages[id] = st
		sd := s.Senders[msg.Domain]
		sd.Count++
		if sd.Class != "restricted" {
			sd.Class = string(msg.Class)
		}
		s.Senders[msg.Domain] = sd
	}
	for id, n := range s.Events {
		if n == 0 {
			delete(s.Events, id)
		}
	}
	return s
}

// modelWorld returns a world over the in-memory model, fetching from the fake directly.
func modelWorld(t tb, s setup) *world {
	t.Helper()
	m := newMemory()
	return newWorld(t, s, "personal", m, m.inspect, func(p mail.Port[context.Context]) pass1.Fetch { return p.EnumerateAll })
}
