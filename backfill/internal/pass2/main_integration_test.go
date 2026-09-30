//go:build integration

package pass2_test

import (
	"context"
	"encoding/json"
	"fmt"
	"maps"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	pass1core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass1"
	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/pass2"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass2"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
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

// superuser returns a connection that bypasses row-level security, to write what account setup would
// write and to read what a check reads.
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

var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

// The senders a generated mailbox draws from. The first is listed by the policy until a world delists
// it, and the last has no domain the classifier can read.
var senderDomains = []string{"bank.example", "news.example", "shop.example", ""}

const listedDomain = "bank.example"

// gateConfig is the gate's thresholds in these tests, lowered so a mailbox of a few messages reaches
// the high-volume skip: a sender of more than two messages whose message carries a List-Id and whose
// sender has no prior hit is skipped, and nothing is small or recent enough to be scanned for it.
var gateConfig = scangate.Config{NoReplyLocalParts: []string{"noreply"}, SmallBytes: 1, RecentAgeMillis: 1, LowVolume: 1, HighVolume: 2}

// code is a one-time code the scanner flags, in a body or a subject.
const code = "Your verification code is 419283."

// setup is what a world is built from, kept whole by the failing-case store, so every field is a
// plain value.
type setup struct {
	// PageSize is how many waiting messages a page of the second pass reads.
	PageSize int
	Messages []drawn
}

// drawn is one message of a generated mailbox.
type drawn struct {
	// Sender indexes senderDomains.
	Sender int
	// SubjectCode puts a one-time code in the subject, BodyCode one in the body.
	SubjectCode, BodyCode bool
	// HTML sends the body as an HTML part alone, and otherwise as a text part alone.
	HTML bool
	// ListID gives the message a list identifier.
	ListID bool
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

// mailbox returns the generated mailbox as the provider fake's messages. Every body carries a body
// marker, so a search of anything the pass stores or logs can find it.
func mailbox(s setup) []fake.Message {
	var out []fake.Message
	for i, d := range s.Messages {
		id := fmt.Sprintf("m%03d", i+1)
		domain := senderDomains[d.Sender%len(senderDomains)]
		address := "from" + tag(i) + "@" + domain
		if domain == "" {
			address = "from" + tag(i)
		}
		subject := marker.Field("subject" + tag(i))
		if d.SubjectCode {
			subject += " " + code
		}
		text := marker.Body("body"+tag(i)) + " Thanks for your order."
		if d.BodyCode {
			text += " " + code
		}
		m := fake.Message{Metadata: mail.MessageMetadata{
			ID: id, ThreadID: "t" + id, From: mail.Address{Email: address}, Subject: subject,
			Date: mail.UnixMilli(1_700_000_000_000 + int64(i)*60_000),
		}}
		if d.ListID {
			m.Metadata.ListID = "list." + domain
		}
		if d.HTML {
			m.Body.HTML = "<p>" + text + "</p>"
		} else {
			m.Body.Text = text
		}
		out = append(out, m)
	}
	return out
}

// policyListing returns the policy listing the bank, or listing nothing once delisted.
func policyListing(t tb, delisted bool) policy.Snapshot {
	t.Helper()
	var rows []policy.Row
	if !delisted {
		rows = append(rows, policy.Row{ID: "rule.bank", Class: policy.Restricted, DomainSuffixes: []string{listedDomain}})
	}
	p, err := policy.Load(rows)
	if err != nil {
		t.Fatalf("loading the policy: %v", err)
	}
	return p
}

func scanner(t tb) scan.Scanner {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatalf("building the scanner: %v", err)
	}
	return s
}

// state is what the index holds for one account, read as plain values, the same way from the model and
// from PostgreSQL, so the checks never reach inside either (ADR-0045).
type state struct {
	Ended    bool
	Runs     []run
	Messages map[string]stored
	// Hits are the senders' prior hits, by domain.
	Hits map[string]int64
	// Items are the failed items recorded, by the message's identifier.
	Items map[string][]string
}

type run struct {
	ID, State string
	Progress  core.Progress
}

// stored is one message's position relative to the scanner, and the gate's recorded decision.
type stored struct {
	Domain, Class, State string
	Flags, Rules         []string
	Version              int
	Revision             string
	Decision, Reason     string
}

// latest returns the latest run, and false when there is none.
func (s state) latest() (run, bool) {
	if len(s.Runs) == 0 {
		return run{}, false
	}
	return s.Runs[len(s.Runs)-1], true
}

// seedPostgres creates the account and runs the first pass over its mailbox for real, so the second
// pass starts from what the first stores (ADR-0017).
func seedPostgres(t testing.TB, account string, f mail.Port[context.Context]) {
	t.Helper()
	conn := superuser(t)
	for _, q := range []string{
		"INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')",
		"INSERT INTO account_state (account_id) VALUES ($1)",
	} {
		if _, err := conn.Exec(context.Background(), q, account); err != nil {
			t.Fatal(err)
		}
	}
	n := 0
	deps := pass1.Deps{
		Store: pass1.NewPostgres(backfillPool(t)), Fetch: f.EnumerateAll,
		Policy: policyListing(t, false).For(account), Scanner: scanner(t), Lookups: lookups,
		RunID: func() string { n++; return fmt.Sprintf("first%d", n) }, Now: time.Now,
	}
	p, err := pass1.Open(context.Background(), deps, account)
	if err != nil {
		t.Fatal(err)
	}
	for {
		s, err := p.Next(context.Background())
		if err != nil {
			t.Fatalf("seeding the first pass: %v", err)
		}
		if s.Done {
			return
		}
	}
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
	s := state{Messages: map[string]stored{}, Hits: map[string]int64{}, Items: map[string][]string{}}
	if err := conn.QueryRow(ctx, "SELECT backfill_pass2_complete FROM account_state WHERE account_id = $1", account).Scan(&s.Ended); err != nil {
		t.Fatalf("reading the completion flag: %v", err)
	}
	rows, err := conn.Query(ctx, `SELECT run_id, state, checkpoint, counters FROM job_runs
		WHERE account_id = $1 AND workload = 'backfill' AND pass = 'pass2' ORDER BY started_at, run_id`, account)
	if err != nil {
		t.Fatalf("reading the runs: %v", err)
	}
	var r run
	var cp, ct []byte
	_, err = pgx.ForEachRow(rows, []any{&r.ID, &r.State, &cp, &ct}, func() error {
		var c struct {
			Page  int    `json:"page"`
			After string `json:"after"`
		}
		var n struct{ Pages, Decided, Pending, Scanned, Skipped int }
		if err := json.Unmarshal(cp, &c); err != nil {
			return err
		}
		if err := json.Unmarshal(ct, &n); err != nil {
			return err
		}
		r.Progress = core.Progress{
			Checkpoint: core.Checkpoint{Page: c.Page, After: c.After},
			Counters:   core.Counters{Pages: n.Pages, Decided: n.Decided, Pending: n.Pending, Scanned: n.Scanned, Skipped: n.Skipped},
		}
		s.Runs = append(s.Runs, r)
		return nil
	})
	if err != nil {
		t.Fatalf("reading the runs: %v", err)
	}
	rows, err = conn.Query(ctx, `SELECT m.message_id, m.from_domain::text, m.sender_class, m.scan_state, m.content_flags, m.rule_ids,
			coalesce(m.scanner_version, 0), coalesce(m.scanner_revision, ''), coalesce(d.decision, ''), coalesce(d.reason, '')
		FROM messages AS m LEFT JOIN scan_gate_decisions AS d ON d.account_id = m.account_id AND d.message_id = m.message_id
		WHERE m.account_id = $1`, account)
	if err != nil {
		t.Fatalf("reading the messages: %v", err)
	}
	var id string
	var m stored
	_, err = pgx.ForEachRow(rows, []any{&id, &m.Domain, &m.Class, &m.State, &m.Flags, &m.Rules, &m.Version, &m.Revision, &m.Decision, &m.Reason}, func() error {
		s.Messages[id] = m
		return nil
	})
	if err != nil {
		t.Fatalf("reading the messages: %v", err)
	}
	var domain string
	var hits int64
	rows, err = conn.Query(ctx, "SELECT domain::text, scan_hit_count FROM senders WHERE account_id = $1 AND scan_hit_count > 0", account)
	if err != nil {
		t.Fatalf("reading the senders: %v", err)
	}
	if _, err := pgx.ForEachRow(rows, []any{&domain, &hits}, func() error { s.Hits[domain] = hits; return nil }); err != nil {
		t.Fatalf("reading the senders: %v", err)
	}
	var disposition string
	rows, err = conn.Query(ctx, "SELECT item_id, disposition FROM job_run_failures WHERE account_id = $1 AND item_kind = 'message' ORDER BY seq", account)
	if err != nil {
		t.Fatalf("reading the failed items: %v", err)
	}
	if _, err := pgx.ForEachRow(rows, []any{&id, &disposition}, func() error { s.Items[id] = append(s.Items[id], disposition); return nil }); err != nil {
		t.Fatalf("reading the failed items: %v", err)
	}
	return s
}

// memory is the in-memory model of the Store, which the crash harness searches and reduces against
// (ADR-0069). Each method makes its whole change or none, as a transaction does, and a method called
// with a cancelled context changes nothing. It starts from what the first pass stores.
type memory struct {
	ended    bool
	runs     []run
	messages map[string]*memoryMessage
	volume   map[string]int64
	hits     map[string]int64
	items    map[string][]string
}

type memoryMessage struct {
	in core.Message
	st stored
}

var _ pass2.Store = (*memory)(nil)

// newMemory returns the model holding the mailbox as the first pass stores it, each sender classified
// under the policy listing the bank and each subject masked.
func newMemory(t tb, messages []fake.Message) *memory {
	t.Helper()
	var metadata []mail.MessageMetadata
	for _, m := range messages {
		metadata = append(metadata, m.Metadata)
	}
	page := pass1core.Decide(metadata, policyListing(t, false).For("personal"), scanner(t), lookups)
	m := &memory{messages: map[string]*memoryMessage{}, volume: map[string]int64{}, hits: map[string]int64{}, items: map[string][]string{}}
	for _, d := range page.Messages {
		m.messages[d.ID] = &memoryMessage{
			in: core.Message{ID: d.ID, From: d.From.Email, Domain: d.Domain, SubjectMasked: d.SubjectMasked, ListID: d.ListID != "", SizeBytes: d.SizeBytes, SentAt: d.Date},
			st: stored{Domain: d.Domain, Class: string(d.Class), State: "pending", Flags: []string{}, Rules: []string{}},
		}
		m.volume[d.Domain]++
	}
	return m
}

func (m *memory) State(ctx context.Context, _ string) (bool, bool, pass1core.Latest[core.Progress], error) {
	if err := ctx.Err(); err != nil {
		return false, false, pass1core.Latest[core.Progress]{}, err
	}
	if len(m.runs) == 0 {
		return true, m.ended, pass1core.Latest[core.Progress]{}, nil
	}
	r := m.runs[len(m.runs)-1]
	states := map[string]pass1core.RunState{"running": pass1core.Running, "succeeded": pass1core.Succeeded, "failed": pass1core.Failed}
	return true, m.ended, pass1core.Latest[core.Progress]{Found: true, RunID: r.ID, State: states[r.State], Progress: r.Progress}, nil
}

func (m *memory) end(id, st string) {
	for i := range m.runs {
		if m.runs[i].ID == id && m.runs[i].State == "running" {
			m.runs[i].State = st
		}
	}
}

func (m *memory) progress(id string, at core.Progress) {
	for i := range m.runs {
		if m.runs[i].ID == id {
			m.runs[i].Progress = at
		}
	}
}

func (m *memory) Start(ctx context.Context, _, runID string, start pass1core.Start[core.Progress]) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if start.Abandon {
		m.end(start.ResumedFrom, "failed")
	}
	m.runs = append(m.runs, run{ID: runID, State: "running", Progress: start.From})
	return nil
}

func (m *memory) Delist(ctx context.Context, _, runID string, delisted func([]string) []string, restart core.Progress) (int, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	restricted := map[string]bool{}
	for _, msg := range m.messages {
		if msg.st.Class == "restricted" || msg.st.State == "skipped_restricted" {
			restricted[msg.st.Domain] = true
		}
	}
	marked := 0
	for _, d := range delisted(slices.Sorted(maps.Keys(restricted))) {
		for _, msg := range m.messages {
			if msg.st.Domain == d && (msg.st.Class == "restricted" || msg.st.State == "skipped_restricted") {
				msg.st.Class, msg.st.State = "normal", "pending"
				marked++
			}
		}
	}
	if marked > 0 {
		m.progress(runID, restart)
	}
	return marked, nil
}

func (m *memory) Pending(ctx context.Context, _, after string, n int) ([]core.Message, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	var out []core.Message
	for _, id := range slices.Sorted(maps.Keys(m.messages)) {
		msg := m.messages[id]
		if id > after && msg.st.State == "pending" && len(out) < n {
			c := msg.in
			c.SenderVolume, c.SenderHits = m.volume[msg.st.Domain], m.hits[msg.st.Domain]
			out = append(out, c)
		}
	}
	return out, nil
}

func (m *memory) Commit(ctx context.Context, _, runID string, p core.Page, items []pass1.Item, at core.Progress) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	for _, o := range p.Outcomes {
		if !o.Verdict.Decided() {
			continue
		}
		msg := m.messages[o.ID]
		msg.st.Decision, msg.st.Reason = "SKIP", o.Verdict.Reason().String()
		switch {
		case o.Scanned != nil:
			msg.st.Decision, msg.st.State = "SCAN", "scanned"
			msg.st.Flags, msg.st.Rules = flagNames(o.Scanned), slices.Clone(o.Scanned.Rules)
			msg.st.Version, msg.st.Revision = o.Scanned.Version, o.Scanned.Revision
		case !o.Verdict.Scans():
			msg.st.State = o.Verdict.State().String()
		default:
			msg.st.Decision = "SCAN"
		}
	}
	for d, n := range p.Hits() {
		m.hits[d] += n
	}
	for _, it := range items {
		m.items[it.ID] = append(m.items[it.ID], it.Disposition)
	}
	m.progress(runID, at)
	return nil
}

// flagNames returns a scan's content flags as the index stores them.
func flagNames(s *core.Scanned) []string {
	out := []string{}
	if s.Flags.MFACode() {
		out = append(out, "mfa_code")
	}
	if s.Flags.LoginLink() {
		out = append(out, "login_link")
	}
	return out
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

func (m *memory) Event(ctx context.Context, _, _ string, _ pass1.Event) error {
	return ctx.Err()
}

func (m *memory) Failure(ctx context.Context, _, _ string, it pass1.Item) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	m.items[it.ID] = append(m.items[it.ID], it.Disposition)
	return nil
}

func (m *memory) Backlog(ctx context.Context, _ string) (int64, error) {
	if err := ctx.Err(); err != nil {
		return 0, err
	}
	var n int64
	for _, msg := range m.messages {
		if msg.st.State == "pending" {
			n++
		}
	}
	return n, nil
}

// inspect returns the model's state.
func (m *memory) inspect(tb) state {
	s := state{Ended: m.ended, Runs: slices.Clone(m.runs), Messages: map[string]stored{}, Hits: maps.Clone(m.hits), Items: map[string][]string{}}
	for id, msg := range m.messages {
		s.Messages[id] = msg.st
	}
	for id, it := range m.items {
		s.Items[id] = slices.Clone(it)
	}
	for d, n := range s.Hits {
		if n == 0 {
			delete(s.Hits, d)
		}
	}
	return s
}

// world is pass 2 over one account, its store, the provider fake it fetches bodies from and the process
// that runs it. A crash drops the process, and recovery opens a new one.
type world struct {
	ctx     context.Context
	account string
	store   pass2.Store
	inspect func(t tb) state
	deps    pass2.Deps
	pass    *pass2.Pass
	// messages is the mailbox the provider fake holds.
	messages []fake.Message
	// delisted removes the bank's rule from the policy the next process loads.
	delisted bool
	// throttle is how many body fetches the provider throttles next.
	throttle int
	// cut crashes the process inside the next page, once it has fetched a body and before its commit
	// ends, and cutAfter once its commit has ended and before the run records anything more.
	cut, cutAfter bool
	cancel        context.CancelFunc
	// What the world was told. Decided are the messages the index held decided after the last step
	// that made a page durable, and reported is the progress that step reported.
	decided  map[string]string
	reported core.Progress
	// fetched lists the messages whose body the provider returned, restrictedFetch those fetched while
	// the policy listed their sender, and crashes counts the crashes.
	fetched         []string
	restrictedFetch []string
	crashes         int
	runs            int
	pageSize        int
	// lastOpenDelisted is set when the last recovery's delisting transition marked messages, and
	// delistApplied once a run opened under the policy without the bank's rule.
	lastOpenDelisted, delistApplied bool
}

// newWorld builds a world over store for the mailbox s generates. body turns the throttled fake into
// the body fetch the pass uses.
func newWorld(t tb, s setup, account string, messages []fake.Message, store pass2.Store, inspect func(tb) state, body func(mail.Port[context.Context]) pass2.Body) *world {
	t.Helper()
	f, err := fake.New(fake.Config{Account: account, BudgetPerSecond: 10_000}, messages...)
	if err != nil {
		t.Fatalf("building the mailbox: %v", err)
	}
	w := &world{ctx: context.Background(), account: account, store: store, inspect: inspect, messages: messages, pageSize: max(s.PageSize, 1), decided: map[string]string{}}
	throttled := fake.Throttle(f, func(fake.Call) error {
		if w.throttle > 0 {
			w.throttle--
			return mail.ThrottleError{Signal: mail.ThrottleSignal{RetryAfterMillis: 1, HasRetryAfter: true}}
		}
		return nil
	}, time.Now)
	inner := body(throttled)
	w.deps = pass2.Deps{
		Store: cutting{Store: store, w: w},
		Body: func(ctx context.Context, id string) (mail.MessageBody, error) {
			b, err := inner(ctx, id)
			if err == nil {
				w.fetched = append(w.fetched, id)
				if w.restricted(id) {
					w.restrictedFetch = append(w.restrictedFetch, id)
				}
			}
			if w.cut {
				w.cut = false
				w.cancel()
			}
			return b, err
		},
		Lookups: lookups, Gate: gateConfig, Scanner: scanner(t), PageSize: w.pageSize,
		RunID: func() string {
			w.runs++
			return fmt.Sprintf("run%d", w.runs)
		},
		Now: time.Now,
	}
	return w
}

// restricted reports whether the policy the running process loaded lists the message's sender.
func (w *world) restricted(id string) bool {
	for _, m := range w.messages {
		if m.Metadata.ID == id {
			return classify.Classify(w.deps.Policy, m.Metadata.From.Email, lookups).Class().Restricted()
		}
	}
	return false
}

// open opens a run, the recovery path, loading the policy as it stands.
func (w *world) open(t tb) {
	t.Helper()
	w.deps.Policy = policyListing(t, w.delisted).For(w.account)
	before := w.inspect(t)
	p, err := pass2.Open(w.ctx, w.deps, w.account)
	if err != nil {
		t.Fatalf("opening a run: %v", err)
	}
	w.pass = p
	w.lastOpenDelisted = false
	if w.delisted && p.Run() != "" {
		w.delistApplied = true
	}
	after := w.inspect(t)
	for id, m := range before.Messages {
		if m.Class == "restricted" && after.Messages[id].Class == "normal" || m.State == "skipped_restricted" && after.Messages[id].State == "pending" {
			w.lastOpenDelisted = true
			delete(w.decided, id)
		}
	}
	if w.lastOpenDelisted {
		w.reported = core.Restart(w.reported)
	}
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
	if at := w.pass.Progress(); at != before {
		w.reported = at
		w.decided = map[string]string{}
		for id, m := range w.inspect(t).Messages {
			if m.State != "pending" {
				w.decided[id] = m.State
			}
		}
	}
	if err != nil {
		w.pass = nil
		return false
	}
	return s.Done
}

// The crash points of the second pass. A crash stops the process between two steps, inside a page
// once a body is fetched and before its commit ends, or inside a page once its commit has ended and
// before the run records anything more.
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
		before := w.reported
		w.step(t)
		if at == insidePage && !w.cut && w.reported != before {
			t.Errorf("a page whose process was stopped before its commit ended was made durable")
		}
		w.cut, w.cutAfter = false, false
	}
	w.pass = nil
}

// cutting is the world's store, which stops the process once a commit has ended when the world's crash
// point asks for it.
type cutting struct {
	pass2.Store
	w *world
}

func (c cutting) Commit(ctx context.Context, account, runID string, p core.Page, items []pass1.Item, at core.Progress) error {
	err := c.Store.Commit(ctx, account, runID, p, items, at)
	if err == nil && c.w.cutAfter {
		c.w.cutAfter = false
		c.w.cancel()
	}
	return err
}

// persistence checks that every decision the world was told is durable is intact after recovery, that
// the recovered run starts from exactly the progress last reported, or from the start when the
// recovery's delisting transition returned messages to pending scan, and that no earlier run is left
// running.
func (w *world) persistence(t tb) {
	t.Helper()
	got := w.inspect(t)
	for id, st := range w.decided {
		if got.Messages[id].State != st {
			t.Errorf("message %s was %s before the crash and is %s after it", id, st, got.Messages[id].State)
		}
	}
	if got.Ended {
		return
	}
	latest, ok := got.latest()
	if !ok || latest.State != "running" {
		t.Fatalf("after recovery the latest run is %+v, want a running one", latest)
	}
	if latest.Progress != w.reported {
		t.Errorf("the recovered run starts from %+v, want %+v", latest.Progress, w.reported)
	}
	for _, r := range got.Runs[:len(got.Runs)-1] {
		if r.State == "running" {
			t.Errorf("run %s is still recorded as running after a later run started", r.ID)
		}
	}
}

// progress drives the pass until it is done and checks what it left. No body of a sender the policy
// listed was fetched. Every message is decided, a listed or unreadable sender's as skipped restricted
// with its reason, and every other either scanned, with a flag exactly when its body holds a code, or
// skipped by the gate. Every decision is recorded with a decision row that agrees with it. Each sender's
// prior hits count its flagged messages once. The pass is recorded as ended with its last run
// succeeded and none left running, and no crash cost more than a page of bodies fetched again.
func (w *world) progress(t tb) {
	t.Helper()
	w.throttle = 0
	limit := 2*(len(w.messages)/w.pageSize+2) + 4
	done := false
	for range limit {
		if done = w.step(t); done {
			break
		}
	}
	if !done {
		t.Fatalf("the pass did not end within %d steps", limit)
	}
	if len(w.restrictedFetch) > 0 {
		t.Errorf("the bodies of %v were fetched while the policy listed their sender", w.restrictedFetch)
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
	if latest, ok := got.latest(); !ok || latest.State != "succeeded" {
		t.Errorf("the latest run is %+v, want one that succeeded", latest)
	}
	final := policyListing(t, w.delistApplied).For(w.account)
	hits := map[string]int64{}
	scanned := 0
	for _, m := range w.messages {
		id, st := m.Metadata.ID, got.Messages[m.Metadata.ID]
		restricted := classify.Classify(final, m.Metadata.From.Email, lookups).Class().Restricted()
		flagged := st.State == "scanned" && slices.Contains(st.Flags, "mfa_code")
		hasCode := strings.Contains(m.Body.Text+m.Body.HTML, code)
		switch {
		case restricted && (st.State != "skipped_restricted" || st.Decision != "SKIP" || st.Reason != "restricted"):
			t.Errorf("message %s of a restricted sender is %+v, want skipped as restricted", id, st)
		case !restricted && st.State != "scanned" && st.State != "skipped_gate":
			t.Errorf("message %s of a normal sender is %+v, want scanned or skipped by the gate", id, st)
		case st.State == "scanned" && (st.Decision != "SCAN" || flagged != hasCode || st.Version != scan.Version || st.Revision != "a-revision"):
			t.Errorf("message %s is %+v, want a scan decision and a code flag %v", id, st, hasCode)
		case st.State == "skipped_gate" && (st.Decision != "SKIP" || st.Reason != "high_volume_no_hits"):
			t.Errorf("message %s is %+v, want skipped by the high-volume rule", id, st)
		}
		if flagged {
			hits[st.Domain]++
		}
		if st.State == "scanned" {
			scanned++
		}
	}
	if !maps.Equal(hits, got.Hits) {
		t.Errorf("the senders' prior hits are %v, want %v, one for each flagged message", got.Hits, hits)
	}
	if rework := len(w.fetched) - scanned; rework > w.crashes*w.pageSize {
		t.Errorf("%d bodies were fetched for %d scanned messages across %d crashes, more than a page of rework a crash", len(w.fetched), scanned, w.crashes)
	}
}

// realWorld returns a world over PostgreSQL, for an account of its own that the first pass has filled,
// fetching bodies through the real rate limiter.
func realWorld(t testing.TB, s setup) *world {
	t.Helper()
	return realWorldOf(t, s.PageSize, mailbox(s))
}

// realWorldOf returns a world over PostgreSQL holding messages, read pageSize at a time.
func realWorldOf(t testing.TB, pageSize int, messages []fake.Message) *world {
	t.Helper()
	s := setup{PageSize: pageSize}
	account := fmt.Sprintf("account%d", accounts.Add(1))
	f, err := fake.New(fake.Config{Account: account, PageSize: 50, BudgetPerSecond: 10_000}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	seedPostgres(t, account, f)
	pool := backfillPool(t)
	limiter := lease.New(pool, 10_000, nil)
	url := postgres.URL(t)
	return newWorld(t, s, account, messages, pass2.NewPostgres(pool), func(t tb) state { return inspectPostgres(t, url, account) },
		func(p mail.Port[context.Context]) pass2.Body {
			return func(ctx context.Context, id string) (mail.MessageBody, error) {
				return pass1.Call(ctx, limiter, p, account, mail.OpGetMessageBody, func(ctx context.Context) (mail.MessageBody, error) {
					return p.GetMessageBody(ctx, id)
				})
			}
		})
}

// modelWorld returns a world over the in-memory model, fetching from the fake directly.
func modelWorld(t tb, s setup) *world {
	t.Helper()
	messages := mailbox(s)
	m := newMemory(t, messages)
	return newWorld(t, s, "personal", messages, m, m.inspect, func(p mail.Port[context.Context]) pass2.Body { return p.GetMessageBody })
}
