//go:build integration

package tick_test

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// tb is what the helpers below need of a test, which a testing.T and a rapid.T both are.
type tb interface {
	Helper()
	Errorf(format string, args ...any)
	Fatalf(format string, args ...any)
}

// The role delta sync connects as, so row-level security and its grants apply as in production.
const role = "mediated_mailbox_sync"

var (
	pools    atomic.Pointer[pgxpool.Pool]
	accounts atomic.Int64
	// runIDs numbers the runs of every test, so two ticks of one account never share an identifier.
	runIDs atomic.Int64
)

// syncPool returns the package's pool connecting as role, shared by every test.
func syncPool(t testing.TB) *pgxpool.Pool {
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

// superuser returns a connection that bypasses row-level security, to write what account setup and
// backfill would write and to read what a check reads.
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

func must(t tb, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(context.Background(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// newAccount writes a fresh account with a state row, its second pass of backfill ended or not, and
// returns its identifier. Every test takes accounts of its own, so tests share one database.
func newAccount(t tb, conn *pgx.Conn, secondEnded bool) string {
	t.Helper()
	id := fmt.Sprintf("acct%d", accounts.Add(1))
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", id)
	must(t, conn, "INSERT INTO account_state (account_id, backfill_pass1_complete, backfill_pass2_complete) VALUES ($1, $2, $2)", id, secondEnded)
	return id
}

var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

// listing returns the policy listing each of domains as restricted.
func listing(t tb, domains ...string) policy.Snapshot {
	t.Helper()
	var rows []policy.Row
	for _, d := range domains {
		rows = append(rows, policy.Row{ID: "rule." + strings.Split(d, ".")[0], Class: policy.Restricted, DomainSuffixes: []string{d}})
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

// now is the clock every tick in these tests reads, a fixed instant, so windows and ages are exact.
var now = time.Date(2026, 9, 20, 12, 0, 0, 0, time.UTC)

const day = 24 * time.Hour

// at returns an instant as the canonical model holds it.
func at(t time.Time) mail.UnixMilli { return mail.UnixMilli(t.UnixMilli()) }

// code is a one-time code the scanner flags, in a body or a subject.
const code = "Your verification code is 419283."

// message returns a message from address dated when, in a thread of its own, its body carrying a body
// marker so a search of anything a tick stores or logs can find it.
func message(id, address string, when time.Time, labels ...string) fake.Message {
	return fake.Message{
		Metadata: mail.MessageMetadata{
			ID: id, ThreadID: "t" + id, From: mail.Address{Email: address}, Subject: marker.Field("subject"),
			Date: at(when), Labels: labels,
		},
		Body: mail.MessageBody{Text: marker.Body("body") + " Thanks for your order."},
	}
}

// mailbox returns a provider fake for the account holding messages.
func mailbox(t tb, account string, messages ...fake.Message) *fake.Fake {
	t.Helper()
	f, err := fake.New(fake.Config{Account: account, PageSize: 2}, messages...)
	if err != nil {
		t.Fatalf("building the fake: %v", err)
	}
	return f
}

// direct is a Provider that calls a port with no lease, so a test reaches the tick's logic without the
// rate limiter. It counts the bodies asked for, and fails the calls a test sets it to fail.
type direct struct {
	port mail.Port[context.Context]
	// bodies are the messages whose bodies were asked for, in order.
	bodies []string
	// failBody fails the body fetch of a message with the error it maps it to.
	failBody map[string]error
	// failThreads fails the next listing of threads with the error, once.
	failThreads error
}

func (d *direct) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	return d.port.CurrentCursor(ctx)
}

func (d *direct) ChangesSince(ctx context.Context, c mail.Cursor) (mail.ChangeSet, error) {
	return d.port.ChangesSince(ctx, c)
}

func (d *direct) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	return d.port.GetMessageMetadata(ctx, ids)
}

func (d *direct) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	if err := d.failThreads; err != nil {
		d.failThreads = nil
		return mail.Page[mail.ThreadMetadata]{}, err
	}
	return d.port.ListThreads(ctx, q, page)
}

func (d *direct) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	d.bodies = append(d.bodies, id)
	if err, ok := d.failBody[id]; ok {
		return mail.MessageBody{}, err
	}
	return d.port.GetMessageBody(ctx, id)
}

// gate is the gate's thresholds in these tests, the defaults, under which a message of a sender of
// fewer than twenty messages is scanned.
var gate = scangate.DefaultConfig()

// deps returns a tick's dependencies over the account through pool and p, under the
// policy, at the fixed clock.
func deps(t tb, pool *pgxpool.Pool, p tick.Provider, pol policy.Snapshot, account string) tick.Deps {
	t.Helper()
	return tick.Deps{
		Store: tick.NewPostgres(pool), Provider: p,
		Policy: pol.For(account), Lookups: lookups, Gate: gate, Scanner: scanner(t),
		FirstWindow: 7 * day, Decisions: 200, PageSize: 2,
		RunID: func() string { return fmt.Sprintf("%s-run%d", account, runIDs.Add(1)) },
		Now:   func() time.Time { return now },
	}
}

// stored is one message as the index holds it.
type stored struct {
	Labels               []string
	Read                 bool
	Subject              string
	Masked               bool
	Class, Rule, State   string
	Flags                []string
	Decision, Reason     string
	Version              int
	Revision             string
	SubjectStampRevision string
}

// index reads the account's messages, by identifier.
func index(t tb, conn *pgx.Conn, account string) map[string]stored {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT m.message_id, m.labels, coalesce((m.flags->>'read')::boolean, false), coalesce(m.subject, ''),
			m.subject_masked, m.sender_class, coalesce(m.class_rule_id, ''), m.scan_state, m.content_flags,
			coalesce(d.decision, ''), coalesce(d.reason, ''), coalesce(m.scanner_version, 0), coalesce(m.scanner_revision, ''),
			coalesce(m.subject_scanner_revision, '')
		FROM messages AS m LEFT JOIN scan_gate_decisions AS d ON d.account_id = m.account_id AND d.message_id = m.message_id
		WHERE m.account_id = $1`, account)
	if err != nil {
		t.Fatalf("reading the messages: %v", err)
	}
	out := map[string]stored{}
	var id string
	var s stored
	_, err = pgx.ForEachRow(rows, []any{
		&id, &s.Labels, &s.Read, &s.Subject, &s.Masked, &s.Class, &s.Rule, &s.State, &s.Flags,
		&s.Decision, &s.Reason, &s.Version, &s.Revision, &s.SubjectStampRevision,
	}, func() error {
		out[id] = s
		return nil
	})
	if err != nil {
		t.Fatalf("reading the messages: %v", err)
	}
	return out
}

// runRow is one of the account's runs as job_runs holds it.
type runRow struct {
	ID, Pass, State string
	Checkpoint      map[string]any
	Counters        map[string]any
}

// runs reads the account's delta sync runs, oldest first.
func runs(t tb, conn *pgx.Conn, account string) []runRow {
	t.Helper()
	rows, err := conn.Query(context.Background(), `SELECT run_id, pass, state, coalesce(checkpoint, '{}'), counters FROM job_runs
		WHERE account_id = $1 AND workload = 'sync' ORDER BY started_at, run_id`, account)
	if err != nil {
		t.Fatalf("reading the runs: %v", err)
	}
	var out []runRow
	var r runRow
	var cp, ct []byte
	_, err = pgx.ForEachRow(rows, []any{&r.ID, &r.Pass, &r.State, &cp, &ct}, func() error {
		r.Checkpoint, r.Counters = map[string]any{}, map[string]any{}
		if err := json.Unmarshal(cp, &r.Checkpoint); err != nil {
			return err
		}
		if err := json.Unmarshal(ct, &r.Counters); err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	if err != nil {
		t.Fatalf("reading the runs: %v", err)
	}
	return out
}

// cursorOf reads the account's stored cursor and its write time.
func cursorOf(t tb, conn *pgx.Conn, account string) (string, *time.Time) {
	t.Helper()
	var c *string
	var written *time.Time
	if err := conn.QueryRow(context.Background(), "SELECT sync_cursor, sync_cursor_at FROM account_state WHERE account_id = $1", account).Scan(&c, &written); err != nil {
		t.Fatalf("reading the cursor: %v", err)
	}
	if c == nil {
		return "", written
	}
	return *c, written
}

// count returns the single number a query reads.
func count(t tb, conn *pgx.Conn, sql string, args ...any) int {
	t.Helper()
	var n int
	if err := conn.QueryRow(context.Background(), sql, args...).Scan(&n); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return n
}

// texts returns the text a query reads from each row, in order.
func texts(t tb, conn *pgx.Conn, sql string, args ...any) []string {
	t.Helper()
	rows, err := conn.Query(context.Background(), sql, args...)
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	out, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
	return out
}

// mustTick runs one tick and fails the test on an error.
func mustTick(t tb, d tick.Deps, account string) tick.Result {
	t.Helper()
	r, err := tick.Run(context.Background(), d, account)
	if err != nil {
		t.Fatalf("the tick failed: %v", err)
	}
	return r
}

// label adds a label to a message at the fake, as a change made outside the system.
func label(t tb, f *fake.Fake, id, path string) {
	t.Helper()
	if _, err := f.EnsureLabel(context.Background(), path); err != nil {
		t.Fatalf("creating label %s: %v", path, err)
	}
	op, err := mail.LabelOp(id, path)
	if err != nil {
		t.Fatalf("building an op: %v", err)
	}
	read, err := mail.MarkReadOp(id)
	if err != nil {
		t.Fatalf("building an op: %v", err)
	}
	res, err := f.Mutate(context.Background(), []mail.MutationOp{op, read})
	if err != nil || errors.Join(res.Errors...) != nil {
		t.Fatalf("labelling %s: %v %v", id, err, res.Errors)
	}
}

// deliver delivers messages to the fake, as new mail arriving.
func deliver(t tb, f *fake.Fake, messages ...fake.Message) {
	t.Helper()
	for _, m := range messages {
		if err := f.Deliver(m); err != nil {
			t.Fatalf("delivering %s: %v", m.Metadata.ID, err)
		}
	}
}
