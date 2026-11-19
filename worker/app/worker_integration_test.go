//go:build integration

package app_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	neturl "net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/app"
)

// both is every job kind of the worker, sorted, as jobs returns them.
var both = []string{"backfill", "sync"}

// mailOf returns a Fake for the account holding n messages from distinct senders of one shop, in
// pages of two.
func mailOf(t *testing.T, account string, n int) *fake.Fake {
	t.Helper()
	var messages []fake.Message
	for i := range n {
		messages = append(messages, fake.Message{
			Metadata: mail.MessageMetadata{
				ID: fmt.Sprintf("m%d", i), ThreadID: fmt.Sprintf("t%d", i), From: mail.Address{Email: fmt.Sprintf("orders%d@shop.example", i)},
				Subject: marker.Field("subject"), Date: mail.UnixMilli(1_700_000_000_000 + int64(i)*60_000),
			},
			Body: mail.MessageBody{Text: "Your order has shipped."},
		})
	}
	f, err := fake.New(fake.Config{Account: account, PageSize: 2}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// forget removes an account and everything recorded for it in one transaction, as account removal
// does, so a run writing for the account meanwhile cannot leave a row behind it. A transaction the
// database ends as a deadlock with such a run is made again.
func (w *world) forget(account string) {
	w.t.Helper()
	var err error
	for range 5 {
		if err = w.remove(account); err == nil {
			return
		}
	}
	w.t.Fatalf("removing %s: %v", account, err)
}

// remove is one attempt of forget.
func (w *world) remove(account string) error {
	return pgx.BeginFunc(w.t.Context(), w.conn, func(tx pgx.Tx) error {
		if _, err := tx.Exec(w.t.Context(), "LOCK TABLE accounts, account_state, rate_state, rate_grants IN EXCLUSIVE MODE"); err != nil {
			return err
		}
		for _, table := range []string{
			"masking_events", "scan_gate_decisions", "job_run_events", "job_run_failures", "job_runs", "messages", "senders",
			"rate_grants", "rate_state", "account_state", "accounts",
		} {
			if _, err := tx.Exec(w.t.Context(), "DELETE FROM "+table+" WHERE account_id = $1", account); err != nil {
				return err
			}
		}
		return nil
	})
}

// passesEnded returns whether the account's backfill passes have ended.
func (w *world) passesEnded(account string) bool {
	w.t.Helper()
	var first, second bool
	if err := w.conn.QueryRow(w.t.Context(), "SELECT backfill_pass1_complete, backfill_pass2_complete FROM account_state WHERE account_id = $1", account).Scan(&first, &second); err != nil {
		w.t.Fatal(err)
	}
	return first && second
}

// ticked returns how many of the account's delta sync ticks succeeded.
func (w *world) ticked(account string) int {
	w.t.Helper()
	var n int
	if err := w.conn.QueryRow(w.t.Context(), "SELECT count(*) FROM job_runs WHERE account_id = $1 AND workload = 'sync' AND state = 'succeeded'", account).Scan(&n); err != nil {
		w.t.Fatal(err)
	}
	return n
}

// VERIFICATIONS' rows for the quiet start and the worker's readiness, the Session's part and each
// listing's. Each job kind's listing is asked at once, while the Session's first reload is held up
// behind a lock on the accounts, and answers that it is not due, so nothing is logged as an error and
// the worker reports not ready. Once the lock goes, the Session's reload wakes each listing and the
// worker reports ready within seconds (ADR-0119, ADR-0142).
func TestAStartLogsNoErrorAndIsReadyOnlyOnceTheSessionLoads(t *testing.T) {
	w := newWorld(t)
	w.connect("personal", "personal-token", true)
	tx, err := w.conn.Begin(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(t.Context(), "LOCK TABLE accounts IN ACCESS EXCLUSIVE MODE"); err != nil {
		t.Fatal(err)
	}
	w.start("--account_reload_interval=1h", "--reload_interval=1h")
	w.scrape(
		`mediated_mailbox_job_runs_finished_total{component="worker",job_kind="backfill",outcome="not_due"} 1`,
		`mediated_mailbox_job_runs_finished_total{component="worker",job_kind="sync",outcome="not_due"} 1`,
	)
	if code, body := w.probe("/readyz"); code == http.StatusOK {
		t.Errorf("the worker reported ready before its Session loaded: %d %q", code, body)
	}
	if err := tx.Commit(t.Context()); err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	w.ready()
	if took := time.Since(start); took > 5*time.Second {
		t.Errorf("the worker took %s to report ready after its Session could load", took)
	}
	if errs := w.records("ERROR"); len(errs) != 0 {
		t.Errorf("a start whose Session loads logged errors: %+v", errs)
	}
}

// VERIFICATIONS' row for the worker's readiness, the part of its pools. Once the database refuses the
// worker's connections and ends those open, the Session's next reload finds it gone and the worker
// reports not ready naming the pool, and once the database admits them again the pool's own attempt to
// connect again reports it ready, with no reload due for an hour. Its health probe answers 200
// throughout (ADR-0142).
func TestTheWorkerIsNotReadyWhileItsWorkFindsTheDatabaseGone(t *testing.T) {
	w := newWorld(t)
	w.start("--log_level=debug", "--account_reload_interval=1s")
	w.ready()
	var database string
	if err := w.conn.QueryRow(t.Context(), "SELECT current_database()").Scan(&database); err != nil {
		t.Fatal(err)
	}
	must(t, w.conn, fmt.Sprintf("REVOKE CONNECT ON DATABASE %q FROM PUBLIC", database))
	t.Cleanup(func() { restore(t, fmt.Sprintf("GRANT CONNECT ON DATABASE %q TO PUBLIC", database)) })
	// Only the worker's own connections end. The other test packages share the server under pgrun, each
	// on a database of its own, so a connection outside this database is not the test's to end.
	must(t, w.conn, "SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1 AND usename = $2", database, role)
	w.eventually(15*time.Second, "the worker reporting its pool not ready", func() bool {
		code, body := w.probe("/readyz")
		return code == http.StatusServiceUnavailable && strings.Contains(body, ": the database could not serve the latest statement or connection attempt")
	})
	if code, body := w.probe("/healthz"); code != http.StatusOK {
		t.Errorf("with the database refusing connections the health probe answers %d %q, want 200", code, body)
	}
	must(t, w.conn, fmt.Sprintf("GRANT CONNECT ON DATABASE %q TO PUBLIC", database))
	w.ready()
}

// VERIFICATIONS' row for probes a state the operator fixes does not fail, for the worker. With no
// account, with accounts whose stored credential cannot be opened, whose provider refuses the
// credential on every call, whose provider needs an OAuth client and that names none, or whose
// client's secret does not open, and with an account whose policy was never imported, each job
// kind's runs fail or find nothing to do, and both probes answer 200 throughout (ADR-0142).
func TestNoAccountOrAnAccountThatCannotConnectFailsNoProbe(t *testing.T) {
	probes := func(t *testing.T, w *world) {
		t.Helper()
		for _, path := range []string{"/readyz", "/healthz"} {
			if code, body := w.probe(path); code != http.StatusOK {
				t.Errorf("%s answered %d %q, want 200", path, code, body)
			}
		}
	}
	t.Run("no account", func(t *testing.T) {
		w := newWorld(t)
		w.start("--account_reload_interval=1s", "--reload_interval=1s")
		w.ready()
		time.Sleep(2 * time.Second)
		probes(t, w)
	})
	t.Run("accounts that cannot connect", func(t *testing.T) {
		w := newWorld(t)
		w.connect("refused", "refused-token", false)
		w.schedule("refused", func(fake.Call) error { return mail.ErrAuthentication })
		w.connect("unopenable", "unopenable-token", false)
		must(t, w.conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'unopenable'", []byte("not a sealed value"))
		// An account that names no OAuth client, so none is stored for it, and one connecting through
		// a client whose secret is sealed to another key, so it does not open.
		must(t, w.conn, "INSERT INTO accounts (account_id, provider) VALUES ('clientless', 'gmail')")
		must(t, w.conn, "INSERT INTO account_state (account_id, credential) VALUES ('clientless', $1)",
			sealed(t, w.keys, "clientless-token", seal.AccountCredential("clientless")))
		must(t, w.conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('elsewhere', 'gmail', 'elsewhere-id', $1)",
			sealed(t, generate(t), "elsewhere-secret", seal.ClientSecret("elsewhere")))
		must(t, w.conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('unopened-client', 'gmail', 'elsewhere')")
		must(t, w.conn, "INSERT INTO account_state (account_id, credential) VALUES ('unopened-client', $1)",
			sealed(t, w.keys, "unopened-client-token", seal.AccountCredential("unopened-client")))
		w.start("--account_reload_interval=1s", "--reload_interval=1s", "--sync.sync_interval=1s")
		w.ready()
		w.eventually(10*time.Second, "a refused call", func() bool { return len(w.records("ERROR")) > 0 })
		time.Sleep(2 * time.Second)
		probes(t, w)
	})
	t.Run("an account whose policy was never imported", func(t *testing.T) {
		w := newWorld(t)
		w.connect("personal", "personal-token", true)
		w.start("--account_reload_interval=1s", "--reload_interval=1s")
		w.ready()
		time.Sleep(2 * time.Second)
		probes(t, w)
	})
}

// VERIFICATIONS' row for each reload of the worker's Session waking its job kinds. With every listing
// asked again only once an hour, an account connected after the start gets its jobs of both job kinds
// within seconds of the Session's next reload, every second here, so each job kind's plan is told of
// each reload (ADR-0090, ADR-0119).
func TestEachSessionReloadWakesTheJobKinds(t *testing.T) {
	w := newWorld(t)
	w.connect("personal", "personal-token", true)
	w.start("--account_reload_interval=1s", "--reload_interval=1h")
	w.eventually(10*time.Second, "the account listed at the start getting its jobs", func() bool { return slices.Equal(both, w.jobs("personal")) })
	w.connect("work", "work-token", true)
	w.eventually(4*time.Second, "the account connected after the start getting its jobs", func() bool { return slices.Equal(both, w.jobs("work")) })
}

// VERIFICATIONS' row for a policy load that fails. While the Session's policy load fails, an account
// newly listed gets no job, since its rules were never read, an account no longer listed has its jobs
// dropped, and an account whose policy loaded keeps them. Once the policy loads, the newly listed
// account gets its jobs (ADR-0119, ADR-0041).
func TestNoAccountIsListedBeforeItsPolicyLoads(t *testing.T) {
	w := newWorld(t)
	w.connect("personal", "personal-token", true)
	w.connect("work", "work-token", true)
	w.start("--account_reload_interval=1s", "--reload_interval=1h")
	w.eventually(10*time.Second, "both accounts getting their jobs", func() bool {
		return slices.Equal(both, w.jobs("personal")) && slices.Equal(both, w.jobs("work"))
	})
	revoke(t, w.conn, "SELECT ON policy_rules")
	w.connect("new", "new-token", true)
	w.forget("work")
	w.eventually(5*time.Second, "the jobs of the account no longer listed being dropped", func() bool { return len(w.jobs("work")) == 0 })
	time.Sleep(2 * time.Second)
	if got := w.jobs("new"); len(got) != 0 {
		t.Errorf("an account listed while the policy failed to load has jobs of %v", got)
	}
	if got := w.jobs("personal"); !slices.Equal(both, got) {
		t.Errorf("an account whose policy loaded has jobs of %v, want both job kinds'", got)
	}
	must(t, w.conn, "GRANT SELECT ON policy_rules TO "+role)
	w.eventually(5*time.Second, "the newly listed account getting its jobs once the policy loads", func() bool { return slices.Equal(both, w.jobs("new")) })
}

// waiting is a mailbox whose every call waits for its context to end, as a call held up by the
// provider would, and tells which operations were cancelled.
type waiting struct {
	*fake.Fake
	mu        sync.Mutex
	entered   []string
	cancelled []string
}

func (p *waiting) wait(ctx context.Context, op string) error {
	p.mu.Lock()
	p.entered = append(p.entered, op)
	p.mu.Unlock()
	<-ctx.Done()
	p.mu.Lock()
	defer p.mu.Unlock()
	p.cancelled = append(p.cancelled, op)
	return ctx.Err()
}

// waited returns how many calls are waiting or have waited.
func (p *waiting) waited() int {
	p.mu.Lock()
	defer p.mu.Unlock()
	return len(p.entered)
}

func (p *waiting) ops() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	return slices.Sorted(slices.Values(p.cancelled))
}

func (p *waiting) EnumerateAll(ctx context.Context, _ mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	return mail.Page[mail.MessageMetadata]{}, p.wait(ctx, "enumerate")
}

func (p *waiting) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	return "", p.wait(ctx, "cursor")
}

func (p *waiting) ListThreads(ctx context.Context, _ mail.Query, _ mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	return mail.Page[mail.ThreadMetadata]{}, p.wait(ctx, "threads")
}

// VERIFICATIONS' row for an account removed while its runs run. A backfill run and a delta sync tick
// held up inside a call to the provider are both cancelled within one reload of the Session once the
// account is removed, and the account's jobs are dropped (ADR-0119).
func TestADroppedAccountsRunsAreCancelled(t *testing.T) {
	w := newWorld(t)
	f, err := fake.New(fake.Config{Account: "personal"})
	if err != nil {
		t.Fatal(err)
	}
	p := &waiting{Fake: f}
	w.port("personal", p)
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=1s", "--reload_interval=1h")
	w.eventually(10*time.Second, "both job kinds' runs waiting inside a call to the provider", func() bool { return p.waited() == 2 })
	w.forget("personal")
	w.eventually(5*time.Second, "both runs being cancelled", func() bool { return len(p.ops()) == 2 })
	if got := p.ops(); !slices.Contains(got, "enumerate") || !slices.Contains(got, "cursor") {
		t.Errorf("the calls cancelled are %v, want backfill's enumeration and delta sync's cursor read", got)
	}
	w.eventually(5*time.Second, "the account's jobs being dropped", func() bool { return len(w.jobs("personal")) == 0 })
}

// VERIFICATIONS' row for a new account backfilled with no manual step. An account connected while the
// worker runs is backfilled from the Session's next reload, both passes run to their end over its
// mail, and delta sync ticks it (ADR-0119, ADR-0133).
func TestANewAccountIsBackfilledWithNoManualStep(t *testing.T) {
	w := newWorld(t)
	w.start("--account_reload_interval=1s", "--reload_interval=1h", "--sync.sync_interval=1s")
	w.ready()
	f := mailOf(t, "personal", 5)
	w.port("personal", f)
	w.connect("personal", "personal-token", false)
	w.eventually(20*time.Second, "the new account's backfill ending", func() bool { return w.passesEnded("personal") })
	var stored int
	if err := w.conn.QueryRow(t.Context(), "SELECT count(*) FROM messages WHERE account_id = 'personal'").Scan(&stored); err != nil || stored != 5 {
		t.Errorf("the index holds %d of the account's messages (%v), want 5", stored, err)
	}
	w.eventually(10*time.Second, "the new account's first tick", func() bool { return w.ticked("personal") > 0 })
	// A message arriving once backfill has ended is added, masked and scanned by delta sync's tick.
	if err := f.Deliver(fake.Message{
		Metadata: mail.MessageMetadata{ID: "late", ThreadID: "late", From: mail.Address{Email: "orders@shop.example"}, Subject: marker.Field("late"), Date: mail.UnixMilli(time.Now().UnixMilli())},
		Body:     mail.MessageBody{Text: "Your order has shipped."},
	}); err != nil {
		t.Fatal(err)
	}
	w.eventually(20*time.Second, "the late message scanned by a tick", func() bool {
		var state string
		err := w.conn.QueryRow(t.Context(), "SELECT scan_state FROM messages WHERE account_id = 'personal' AND message_id = 'late'").Scan(&state)
		return err == nil && state == "scanned"
	})
	// Both passes and the tick masked and scanned with the process's one scanner, built from the
	// default scanner section under its known revision (TestTheScannerSectionBuildsTheSharedScanner).
	var revisions []string
	rows, err := w.conn.Query(t.Context(), "SELECT DISTINCT coalesce(subject_scanner_revision, '') || ' ' || coalesce(scanner_revision, '') FROM messages WHERE account_id = 'personal'")
	if err != nil {
		t.Fatal(err)
	}
	if revisions, err = pgx.CollectRows(rows, pgx.RowTo[string]); err != nil {
		t.Fatal(err)
	}
	const shared = "c9fa3ff13948e4f89ac094b91a27455c"
	if want := []string{shared + " " + shared}; !slices.Equal(want, revisions) {
		t.Errorf("the account's messages were masked and scanned under the revisions %q, want %q", revisions, want)
	}
}

// rotating rotates the token the account's latest mailbox was built over to rotated, and records an
// attempt at the provider's authentication endpoint on it, at the first call it lets through, as a
// provider that rotates the credential at a refresh does (ADR-0082, ADR-0097).
func (w *world) rotating(account, rotated string, attempt mail.AuthAttempt) fake.Schedule {
	var once sync.Once
	return func(fake.Call) error {
		once.Do(func() {
			tokens := w.built(account)
			tokens[len(tokens)-1].Rotate([]byte(rotated))
			tokens[len(tokens)-1].Attempted(attempt)
		})
		return nil
	}
}

// lastAuthentication reads the account's last authentication attempt as the account's state row
// records it.
func (w *world) lastAuthentication(account string) (*time.Time, *string) {
	w.t.Helper()
	var at *time.Time
	var outcome *string
	if err := w.conn.QueryRow(w.t.Context(), "SELECT last_auth_at, last_auth_outcome FROM account_state WHERE account_id = $1", account).Scan(&at, &outcome); err != nil {
		w.t.Fatal(err)
	}
	return at, outcome
}

// VERIFICATIONS' rows for forcing a rotation and restarting, and for failing a unit of work after an
// authentication attempt, for each job kind. Each job kind calls the provider through a unit of work,
// whose end writes the credential its token rotated to back and records the token's latest attempt,
// so a worker started afterwards reads the rotated credential (ADR-0082, ADR-0089, ADR-0097).
func TestEachJobKindsUnitOfWorkWritesBackAndRecords(t *testing.T) {
	for _, c := range []struct {
		name  string
		ended bool
		done  func(w *world) bool
	}{
		{"backfill", false, func(w *world) bool { return w.passesEnded("personal") }},
		{"delta sync", true, func(w *world) bool { return w.ticked("personal") > 0 }},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.port("personal", mailOf(t, "personal", 3))
			attempt := mail.AuthAttempt{At: 1_790_000_000_000, Outcome: mail.AuthSucceeded}
			w.schedule("personal", w.rotating("personal", "personal-rotated", attempt))
			w.connect("personal", "personal-token", c.ended)
			w.start("--account_reload_interval=1h", "--reload_interval=1h")
			// The unit of work's end records the attempt after the work it ends is made durable.
			w.eventually(20*time.Second, "the job kind's work and its unit of work's end", func() bool {
				at, _ := w.lastAuthentication("personal")
				return c.done(w) && at != nil
			})
			at, outcome := w.lastAuthentication("personal")
			if at == nil || at.UnixMilli() != int64(attempt.At) || outcome == nil || *outcome != "succeeded" {
				t.Errorf("the account's state row records the attempt %v %v, want the token's", at, outcome)
			}
			w.halt()

			restarted := &world{t: t, conn: w.conn, keys: w.keys, logs: &lockedBuffer{}, ports: w.ports, schedules: map[string]fake.Schedule{}, tokens: map[string][]*fake.Token{}}
			restarted.start("--account_reload_interval=1h", "--reload_interval=1h")
			restarted.eventually(10*time.Second, "a token made for the account after the restart", func() bool { return len(restarted.built("personal")) > 0 })
			if got := string(restarted.built("personal")[0].Credential()); got != "personal-rotated" {
				t.Errorf("after a restart the account's token holds %q, want the rotated credential", got)
			}
		})
	}
}

// VERIFICATIONS' row for failing the write-back of a rotated credential, for each job kind. A
// write-back that fails is logged by the connections, which keep the rotated credential in memory, and
// fails the job kind's run, so the failure is loud (ADR-0082).
func TestAFailedWriteBackFailsTheJobKindsRun(t *testing.T) {
	for _, c := range []struct {
		name, kind string
		ended      bool
	}{{"backfill", "backfill", false}, {"delta sync", "sync", true}} {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.port("personal", mailOf(t, "personal", 1))
			w.schedule("personal", w.rotating("personal", "personal-rotated", mail.AuthAttempt{}))
			w.connect("personal", "personal-token", c.ended)
			revoke(t, w.conn, "UPDATE (credential) ON account_state")
			w.start("--account_reload_interval=1h", "--reload_interval=1h")
			w.eventually(20*time.Second, "the run failing at its write-back", func() bool {
				for _, r := range w.records("ERROR") {
					if r.JobKind == c.kind && r.Account == "personal" && strings.Contains(r.Error, "writing back the rotated credential") {
						return true
					}
				}
				return false
			})
			var logged bool
			for _, r := range w.records("ERROR") {
				logged = logged || r.Msg == "a rotated credential was not written back, so a restart before a later write-back lands loses the account's access"
			}
			if !logged {
				t.Errorf("the failed write-back was not logged:\n%s", w.logs.String())
			}
		})
	}
}

// VERIFICATIONS' row for a credential replaced while the worker runs, for each job kind. Once the
// provider refuses the credential a token holds, the call that was refused reads the account's row
// again and, finding the credential the operator stored since, is made once more with a token made
// from it, so each job kind's work goes on (ADR-0090, ADR-0106).
func TestARefusedCallUsesTheCredentialTheOperatorStored(t *testing.T) {
	w := newWorld(t)
	w.port("personal", mailOf(t, "personal", 3))
	w.connect("personal", "old-token", false)
	refused := func(fake.Call) error {
		tokens := w.built("personal")
		if string(tokens[len(tokens)-1].Credential()) == "old-token" {
			return mail.ErrAuthentication
		}
		return nil
	}
	w.schedule("personal", refused)
	// A tick that failed is tried again at the ticker's next tick, every two seconds here.
	w.start("--account_reload_interval=1h", "--reload_interval=1h", "--sync.sync_interval=2s")
	w.eventually(10*time.Second, "a refused call", func() bool { return len(w.records("ERROR")) > 0 })
	must(t, w.conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, w.keys, "new-token", seal.AccountCredential("personal")))
	w.eventually(20*time.Second, "backfill ending and delta sync ticking over the stored credential", func() bool {
		return w.passesEnded("personal") && w.ticked("personal") > 0
	})
}

// VERIFICATIONS' rows for an account's token made with its own client, for the worker's job kinds.
// Two accounts connecting through two clients each get a token made from their own client's
// identifier and secret and their own credential (ADR-0106).
func TestEachAccountIsServedThroughItsOwnClient(t *testing.T) {
	w := newWorld(t)
	must(t, w.conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('employer', 'gmail', 'employer-id', $1)",
		sealed(t, w.keys, "employer-secret", seal.ClientSecret("employer")))
	w.connect("personal", "personal-token", true)
	w.connect("work", "work-token", true)
	must(t, w.conn, "UPDATE accounts SET oauth_client = 'employer' WHERE account_id = 'work'")
	w.start("--account_reload_interval=1h", "--reload_interval=1h")
	w.eventually(10*time.Second, "both accounts ticking", func() bool { return w.ticked("personal") > 0 && w.ticked("work") > 0 })
	got := map[string]string{}
	for _, account := range []string{"personal", "work"} {
		tok := w.built(account)[0]
		got[account] = tok.Client().ID + " " + tok.Client().Secret + " " + string(tok.Credential())
	}
	want := map[string]string{"personal": "household-id household-secret personal-token", "work": "employer-id employer-secret work-token"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the client and credential each account's token was made from (-want +got):\n%s", diff)
	}
}

// keyID returns the identifier of the key the sealed value names in its header.
func keyID(t *testing.T, sealedValue []byte) seal.KeyID {
	t.Helper()
	parts, err := seal.Split(sealedValue)
	if err != nil {
		t.Fatal(err)
	}
	return parts.KeyID
}

// VERIFICATIONS' rows for the re-seal delta sync runs, with an account and with none. Delta sync's
// listing re-seals every account credential and OAuth client secret the connections opened with a key
// other than the current one, whether or not an account is listed, and sets the key-scan series from
// what it found, which reads 0 once each is sealed to the current key (ADR-0092, ADR-0103).
func TestDeltaSyncsListingReSealsWhatWasOpenedWithAnOldKey(t *testing.T) {
	for _, c := range []struct {
		name     string
		accounts []string
	}{{"with an account", []string{"personal"}}, {"with no account", nil}} {
		t.Run(c.name, func(t *testing.T) {
			old := newWorld(t)
			for _, a := range c.accounts {
				old.connect(a, a+"-token", true)
			}
			w := &world{t: t, conn: old.conn, keys: generate(t), logs: &lockedBuffer{}, ports: old.ports, schedules: old.schedules, tokens: old.tokens}
			w.privates = []string{old.keys.privateFile, w.keys.privateFile}
			w.start("--account_reload_interval=1h", "--reload_interval=1h")
			want := []string{`mediated_mailbox_client_secret_on_old_key{client="household",component="worker",job_kind="sync"} 0`}
			for _, a := range c.accounts {
				want = append(want, `mediated_mailbox_credential_on_old_key{account="`+a+`",component="worker",job_kind="sync"} 0`)
			}
			w.scrape(want...)
			var secret []byte
			if err := w.conn.QueryRow(t.Context(), "SELECT client_secret FROM oauth_clients WHERE name = $1", household).Scan(&secret); err != nil {
				t.Fatal(err)
			}
			if keyID(t, secret) != w.keys.public.KeyID() {
				t.Error("the client secret is not sealed to the current key")
			}
			for _, a := range c.accounts {
				var credential []byte
				if err := w.conn.QueryRow(t.Context(), "SELECT credential FROM account_state WHERE account_id = $1", a).Scan(&credential); err != nil {
					t.Fatal(err)
				}
				if keyID(t, credential) != w.keys.public.KeyID() {
					t.Errorf("%s's credential is not sealed to the current key", a)
				}
			}
		})
	}
}

// VERIFICATIONS' rows for the series between ticks, the attribution of each job kind's series, the
// reload-failure alarm and the performance measurements. The worker runs until stopped serving every
// series each job kind emits, each labelled with its job kind, among them the key-scan, cursor-gap,
// unclassified-sender and backlog series, and the lease series of each job kind's own caller in its
// own class, beside the Session's reload and policy series once for the process, the scheduler's
// series of each listing and each account job, each pool's and the Go runtime's (ADR-0103, ADR-0117,
// ADR-0125, ADR-0077).
func TestTheProbesServeEverySeriesBetweenRuns(t *testing.T) {
	w := newWorld(t)
	w.port("personal", mailOf(t, "personal", 3))
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=1h", "--reload_interval=1h")
	w.scrape(
		`mediated_mailbox_credential_on_old_key{account="personal",component="worker",job_kind="sync"} 0`,
		`mediated_mailbox_client_secret_on_old_key{client="household",component="worker",job_kind="sync"} 0`,
		`mediated_mailbox_sync_cursor_gaps_total{account="personal",component="worker",job_kind="sync"} 0`,
		`mediated_mailbox_unclassified_senders_total{account="personal",component="worker",job_kind="backfill"} 0`,
		`mediated_mailbox_index_messages_total{account="personal",component="worker",job_kind="backfill",stage="indexed"} 3`,
		`mediated_mailbox_policy_reload_failed{component="worker"} 0`,
		`mediated_mailbox_ratelimit_granted_total{account="personal",class="batch",component="worker",job_kind="backfill"}`,
		`mediated_mailbox_ratelimit_granted_total{account="personal",class="sync",component="worker",job_kind="sync"}`,
		`mediated_mailbox_ratelimit_lease_wait_seconds_count{class="sync",component="worker",job_kind="sync"}`,
		`mediated_mailbox_ratelimit_lease_wait_seconds_count{class="batch",component="worker",job_kind="backfill"}`,
		`mediated_mailbox_body_processing_duration_seconds_count{component="worker",job_kind="backfill",step="scan"}`,
		`mediated_mailbox_session_reload_duration_seconds_count{component="worker"}`,
		`mediated_mailbox_policy_reload_duration_seconds_count{component="worker"}`,
		`mediated_mailbox_job_run_duration_seconds_count{component="worker",job="reload",job_kind="backfill"}`,
		`mediated_mailbox_job_run_duration_seconds_count{component="worker",job="reload",job_kind="sync"}`,
		`mediated_mailbox_job_run_duration_seconds_count{component="worker",job="account",job_kind="backfill"}`,
		`mediated_mailbox_job_run_duration_seconds_count{component="worker",job="account",job_kind="sync"}`,
		`mediated_mailbox_db_pool_max_connections{component="worker",job_kind="backfill"} 4`,
		`mediated_mailbox_db_pool_max_connections{component="worker",job_kind="sync"} 4`,
		`mediated_mailbox_db_statement_duration_seconds_count{component="worker",job_kind="backfill",statement="SetTransactionAccount"}`,
		// Each job kind's statements run on its own pool, timed under its job kind, delta sync's cursor
		// read among them (ADR-0119).
		`mediated_mailbox_db_statement_duration_seconds_count{component="worker",job_kind="sync",statement="SyncCursor"}`,
		"go_goroutines",
		"process_start_time_seconds",
	)
	w.ready()
	if code, body := w.probe("/healthz"); code != http.StatusOK || body != "ok\n" {
		t.Errorf("/healthz answered %d %q, want 200 ok", code, body)
	}
	_, body := w.probe("/metrics")
	for line := range strings.Lines(body) {
		if strings.HasPrefix(line, "mediated_mailbox_policy_reload_failed{") && strings.Contains(line, "job_kind") {
			t.Errorf("the reload-failure series is served under a job kind: %s", line)
		}
	}
}

// VERIFICATIONS' row for an account's series going with its work. An account's backfill backlog
// series goes once its second pass ends, and its backfill and delta sync series go once its jobs are
// dropped (ADR-0104, ADR-0119).
func TestAnAccountsSeriesGoWithItsWork(t *testing.T) {
	w := newWorld(t)
	w.port("personal", mailOf(t, "personal", 3))
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=1s", "--reload_interval=1h")
	unclassified := `mediated_mailbox_unclassified_senders_total{account="personal",component="worker",job_kind="backfill"}`
	gaps := `mediated_mailbox_sync_cursor_gaps_total{account="personal",component="worker",job_kind="sync"}`
	w.eventually(20*time.Second, "the account's backfill ending", func() bool { return w.passesEnded("personal") })
	w.scrape(unclassified, gaps)
	// The pass's end is made durable before its run removes the series, so the series goes soon after.
	w.eventually(5*time.Second, "backfill's backlog series going once the second pass ended", func() bool {
		return !w.served(`mediated_mailbox_scan_backlog{account="personal",component="worker",job_kind="backfill"}`)
	})
	w.forget("personal")
	backlog := `mediated_mailbox_scan_backlog{account="personal",component="worker",job_kind="sync"}`
	w.eventually(5*time.Second, "the dropped account's series going", func() bool {
		return !w.served(unclassified) && !w.served(gaps) && !w.served(backlog)
	})
}

// panicking is a mailbox whose every call panics, as a bug in the code a run calls would.
type panicking struct{ *fake.Fake }

func (panicking) EnumerateAll(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	panic("a bug in a backfill page")
}

func (panicking) CurrentCursor(context.Context) (mail.Cursor, error) {
	panic("a bug in a tick")
}

func (panicking) ListThreads(context.Context, mail.Query, mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	panic("a bug in a tick")
}

// bodyPanicking is a mailbox whose body read panics, as a bug in the code a page of the second pass
// runs would. Every other call is the Fake's.
type bodyPanicking struct{ *fake.Fake }

func (bodyPanicking) GetMessageBody(context.Context, string, mail.BodyPart) (mail.MessageBody, error) {
	panic("a bug in the second pass")
}

// failedRun returns the last error the account's run of the workload and pass recorded, and whether
// one is recorded failed.
func (w *world) failedRun(account, workload, pass string) (string, bool) {
	w.t.Helper()
	var last string
	err := w.conn.QueryRow(w.t.Context(), "SELECT coalesce(last_error, '') FROM job_runs WHERE account_id = $1 AND workload = $2 AND pass = $3 AND state = 'failed' LIMIT 1",
		account, workload, pass).Scan(&last)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false
	}
	if err != nil {
		w.t.Fatal(err)
	}
	return last, true
}

// VERIFICATIONS' row for a job's run that panics, for the job kinds on the worker's spec. A panic in
// a page of backfill's first pass, in one of its second pass and in a tick is recovered in the run
// that raised it and recorded as that run's failure with its stack, logged, and every other
// account's jobs go on (ADR-0119, ADR-0117).
func TestAPanicInARunFailsThatRunAlone(t *testing.T) {
	w := newWorld(t)
	f, err := fake.New(fake.Config{Account: "broken"})
	if err != nil {
		t.Fatal(err)
	}
	w.port("broken", panicking{f})
	w.port("bodies", bodyPanicking{mailOf(t, "bodies", 2)})
	w.port("personal", mailOf(t, "personal", 3))
	w.connect("broken", "broken-token", false)
	w.connect("bodies", "bodies-token", false)
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=1h", "--reload_interval=1h")
	w.eventually(20*time.Second, "the other account's work", func() bool { return w.passesEnded("personal") && w.ticked("personal") > 0 })
	for _, c := range []struct{ account, workload, pass, message string }{
		{"broken", "backfill", "pass1", "a bug in a backfill page"},
		{"bodies", "backfill", "pass2", "a bug in the second pass"},
		{"broken", "sync", "tick", "a bug in a tick"},
	} {
		w.eventually(20*time.Second, "the "+c.workload+" "+c.pass+" run of "+c.account+" recorded failed", func() bool {
			_, failed := w.failedRun(c.account, c.workload, c.pass)
			return failed
		})
		if last, _ := w.failedRun(c.account, c.workload, c.pass); !strings.Contains(last, c.message) || !strings.Contains(last, "goroutine") {
			t.Errorf("the %s %s run of %s is recorded failed with %q, want the panic and its stack", c.workload, c.pass, c.account, last)
		}
	}
	panicked := map[string]bool{}
	for _, r := range w.records("ERROR") {
		if r.Account == "broken" && strings.Contains(r.Error, "panic") {
			panicked[r.JobKind] = true
		}
	}
	if diff := cmp.Diff(map[string]bool{"backfill": true, "sync": true}, panicked, compare.Options); diff != "" {
		t.Errorf("the job kinds whose run of the broken account was logged failed by its panic (-want +got):\n%s", diff)
	}
	w.ready()
}

// editing is a mailbox that runs edit once, before the second page of its enumeration.
type editing struct {
	*fake.Fake
	mu    sync.Mutex
	calls int
	edit  func()
}

func (e *editing) EnumerateAll(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	e.mu.Lock()
	e.calls++
	second := e.calls == 2
	e.mu.Unlock()
	if second {
		e.edit()
	}
	return e.Fake.EnumerateAll(ctx, token)
}

// VERIFICATIONS' rows for a policy edit during a running pass. Each page takes the policy the
// worker's Session has in force at its entry, so a rule restricting the shop, added and loaded by the
// Session while the first pass is between its pages, reaches the second pass of the same run, which
// skips both of the shop's messages as restricted (ADR-0119, ADR-0041).
func TestEachPageTakesThePolicyInForce(t *testing.T) {
	w := newWorld(t)
	t.Cleanup(func() { restore(t, "DELETE FROM policy_rules") })
	messages := []fake.Message{
		{Metadata: mail.MessageMetadata{ID: "m1", ThreadID: "t1", From: mail.Address{Email: "a@shop.example"}, Subject: marker.Field("first"), Date: 1_700_000_000_000}},
		{Metadata: mail.MessageMetadata{ID: "m2", ThreadID: "t2", From: mail.Address{Email: "b@news.example"}, Subject: marker.Field("second"), Date: 1_700_000_060_000}},
		{Metadata: mail.MessageMetadata{ID: "m3", ThreadID: "t3", From: mail.Address{Email: "c@shop.example"}, Subject: marker.Field("third"), Date: 1_700_000_120_000}},
		{Metadata: mail.MessageMetadata{ID: "m4", ThreadID: "t4", From: mail.Address{Email: "d@news.example"}, Subject: marker.Field("fourth"), Date: 1_700_000_180_000}},
	}
	f, err := fake.New(fake.Config{Account: "personal", PageSize: 2}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	// The edit runs on the job's goroutine, so it writes over a connection of its own and reports its
	// failure to the test rather than failing it from there.
	editor := superuser(t)
	edited := make(chan error, 1)
	w.port("personal", &editing{Fake: f, edit: func() {
		_, err := editor.Exec(context.Background(), "INSERT INTO policy_rules (rule_id, class, domain_suffix, source, created_by) VALUES ('rule.shop', 'restricted', '{shop.example}', 'operator', 'operator')")
		edited <- err
		// The Session reloads every half second, so the rule is in force before the second page.
		time.Sleep(1500 * time.Millisecond)
	}})
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=500ms", "--reload_interval=1h")
	w.eventually(20*time.Second, "the account's backfill ending", func() bool { return w.passesEnded("personal") })
	if err := <-edited; err != nil {
		t.Fatalf("adding the rule: %v", err)
	}
	rows, err := w.conn.Query(t.Context(), "SELECT message_id, scan_state FROM messages WHERE account_id = 'personal'")
	if err != nil {
		t.Fatal(err)
	}
	skipped := map[string]bool{}
	var id, state string
	if _, err := pgx.ForEachRow(rows, []any{&id, &state}, func() error {
		skipped[id] = state == "skipped_restricted"
		return nil
	}); err != nil {
		t.Fatal(err)
	}
	want := map[string]bool{"m1": true, "m2": false, "m3": true, "m4": false}
	if diff := cmp.Diff(want, skipped, compare.Options); diff != "" {
		t.Errorf("the messages the run skipped as restricted under the rule added between its pages (-want +got):\n%s", diff)
	}
}

// VERIFICATIONS' row for a public key matching none of the private keys, for the worker on its spec.
// The start loads the keys when the connections are built and refuses a public key that matches none
// of the private keys, before anything serves (ADR-0088).
func TestAPublicKeyMatchingNoPrivateKeyRefusesTheStart(t *testing.T) {
	w := newWorld(t)
	w.keys.publicFile = generate(t).publicFile
	err := runWorker(t, w.flags(), nil, &lockedBuffer{})
	if err == nil || !strings.Contains(err.Error(), "matches none of the private keys") {
		t.Errorf("the start returned %v, want the keyring's refusal", err)
	}
}

// VERIFICATIONS' row for the database driver's own variables. The worker connects as its
// configuration names, whatever PGHOST, PGDATABASE and PGSSLMODE name, so it reports ready (ADR-0078).
func TestTheDriversVariablesNameNothingTheWorkerConnectsTo(t *testing.T) {
	w := newWorld(t)
	done := make(chan error, 1)
	ctx, stop := context.WithCancel(context.Background())
	environ := []string{"PGHOST=elsewhere.example", "PGDATABASE=another", "PGSSLMODE=verify-full"}
	go func() { done <- runWorkerContext(ctx, w.flags(), environ, w.logs) }()
	t.Cleanup(func() {
		stop()
		if err := <-done; err != nil {
			t.Errorf("the worker ended with %v", err)
		}
	})
	w.eventually(20*time.Second, "the worker serving", func() bool { w.probes = probeAddress(w.logs.String()); return w.probes != "" })
	w.ready()
}

// VERIFICATIONS' row for the log level, for the worker on its spec. Under log_level set to warn the
// worker writes every effective value with its source, serves and reports ready, and writes no record
// at info past the configuration, so its serving record is held back. With no log_level it writes its
// serving record at info, so the run under warn holds back a record the worker writes (ADR-0122).
func TestTheConfiguredLevelGovernsTheLog(t *testing.T) {
	w := newWorld(t)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	probes := ln.Addr().String()
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	ctx, stop := context.WithCancel(context.Background())
	done := make(chan error, 1)
	list := fake.Adapters(w.mailbox, gmail)
	go func() {
		done <- process.RunContext(ctx, w.flags("--log_level=warn", "--probe_listen="+probes), nil, w.logs, io.Discard, app.Spec(), connections.WithAdapters(list))
	}()
	// The serving record names the probes' address at info, which warn holds back, so the probes listen
	// where the flag puts them and the readiness probe is asked until it answers 200.
	w.eventually(20*time.Second, "the worker under warn reporting ready", func() bool {
		req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, "http://"+probes+"/readyz", http.NoBody)
		if err != nil {
			t.Fatal(err)
		}
		res, err := http.DefaultClient.Do(req)
		if err != nil {
			return false
		}
		return res.Body.Close() == nil && res.StatusCode == http.StatusOK
	})
	stop()
	if err := <-done; err != nil {
		t.Fatalf("the worker under warn ended with %v", err)
	}
	var sources []string
	for _, r := range w.records("") {
		switch {
		case r.Msg == "configuration":
			sources = append(sources, r.Path+" "+r.Source)
		case r.Level == "INFO" || r.Level == "DEBUG":
			t.Errorf("under warn the worker wrote %q at %s", r.Msg, r.Level)
		}
	}
	if !slices.Contains(sources, "log_level flag --log_level") || !slices.Contains(sources, "sync.concurrency default") {
		t.Errorf("under warn the configuration written is %v, want every value with its source", sources)
	}

	loud := newWorld(t)
	loud.start()
	if !slices.ContainsFunc(loud.records("INFO"), func(r logRecord) bool { return r.Msg == "serving" }) {
		t.Errorf("with no log_level the worker wrote no serving record at info:\n%s", loud.logs.String())
	}
}

// bodyMarkers returns every body marker found in the account's rows of every table that has an
// account column, each row read whole as JSON, and in logs.
func bodyMarkers(t *testing.T, conn *pgx.Conn, account, logs string) []string {
	t.Helper()
	rows, err := conn.Query(t.Context(), `SELECT table_name FROM information_schema.columns
		WHERE table_schema = 'public' AND column_name = 'account_id' ORDER BY table_name`)
	if err != nil {
		t.Fatal(err)
	}
	tables, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	if len(tables) < 10 {
		t.Fatalf("the search reached only the tables %v", tables)
	}
	var found []string
	for _, table := range tables {
		rows, err := conn.Query(t.Context(), "SELECT row_to_json(r)::text FROM "+pgx.Identifier{table}.Sanitize()+" AS r WHERE r.account_id = $1", account)
		if err != nil {
			t.Fatal(err)
		}
		values, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		for _, v := range values {
			if strings.Contains(v, marker.BodyPrefix) {
				found = append(found, table+": "+v)
			}
		}
	}
	for line := range strings.Lines(logs) {
		if strings.Contains(line, marker.BodyPrefix) {
			found = append(found, "log: "+line)
		}
	}
	return found
}

// VERIFICATIONS' row for no body text reaching the index or the logs, for each job kind built from
// the worker's spec. Backfill's second pass and delta sync's tick fetch and scan bodies holding a
// marker, a code among them, logging at debug, and no marker reaches any row of the account or any
// log record (ADR-0009).
func TestNoBodyTextReachesTheIndexOrTheLogs(t *testing.T) {
	w := newWorld(t)
	var messages []fake.Message
	for i, body := range []string{marker.Body("plain"), marker.Body("code") + " Your verification code is 419283."} {
		messages = append(messages, fake.Message{
			Metadata: mail.MessageMetadata{
				ID: fmt.Sprintf("m%d", i), ThreadID: fmt.Sprintf("t%d", i), From: mail.Address{Email: fmt.Sprintf("orders%d@shop.example", i)},
				Subject: marker.Field("subject"), Date: mail.UnixMilli(time.Now().Add(-time.Hour).UnixMilli() + int64(i)),
			},
			Body: mail.MessageBody{Text: body},
		})
	}
	f, err := fake.New(fake.Config{Account: "personal"}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	w.port("personal", f)
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=1h", "--reload_interval=1h", "--log_level=debug")
	w.eventually(20*time.Second, "backfill ending and delta sync ticking", func() bool { return w.passesEnded("personal") && w.ticked("personal") > 0 })
	var scanned int
	if err := w.conn.QueryRow(t.Context(), "SELECT count(*) FROM messages WHERE account_id = 'personal' AND scan_state = 'scanned'").Scan(&scanned); err != nil || scanned != 2 {
		t.Fatalf("%d of the account's bodies were scanned (%v), want both", scanned, err)
	}
	w.halt()
	if found := bodyMarkers(t, w.conn, "personal", w.logs.String()); len(found) != 0 {
		t.Errorf("body text reached the index or the logs:\n%s", strings.Join(found, "\n"))
	}
}

// VERIFICATIONS' row for a schema the chain cannot run on, for the worker's own start on its spec.
// Started over a database that has not applied the chain's newest migration, or whose compatibility
// floor is above the chain, the worker refuses to start before it serves, since each job kind's pool
// is checked before its Build gets it, and over the chain's own schema, or a newer one whose floor
// the chain meets, it serves (ADR-0140, ADR-0142).
func TestTheWorkerRefusesASchemaItCannotRunOn(t *testing.T) {
	keys := generate(t)
	postgres.SchemaChecks(t, "../../db/migrations", func(ctx context.Context, pool *pgxpool.Pool) error {
		c := pool.Config().ConnConfig
		url := fmt.Sprintf("postgres://%s:%s@%s/%s", c.User, neturl.QueryEscape(c.Password), net.JoinHostPort(c.Host, fmt.Sprint(c.Port)), c.Database)
		args := append(databaseFlags(t, url),
			"--credential.public_key_file="+keys.publicFile, "--credential.private_key_files=["+keys.privateFile+"]", "--probe_listen=127.0.0.1:0")
		running, stop := context.WithTimeout(ctx, 20*time.Second)
		defer stop()
		logs := &lockedBuffer{}
		done := make(chan error, 1)
		go func() { done <- runWorkerContext(running, args, nil, logs) }()
		for {
			select {
			case err := <-done:
				return err
			case <-time.After(20 * time.Millisecond):
			}
			if probeAddress(logs.String()) != "" {
				stop()
				if err := <-done; err != nil {
					return fmt.Errorf("the worker served and then ended with %w", err)
				}
				return nil
			}
		}
	}, role)
}

// VERIFICATIONS' row for a tick's token, for the worker on its spec. Each tick is a unit of work over
// the token the connections hold for the account, so three ticks are served by one token between
// them, and once the operator replaces the stored credential and the Session reloads, the next tick
// is served by a token made from the new one (ADR-0090, ADR-0142).
func TestTicksHoldTheirTokenUntilTheCredentialIsReplaced(t *testing.T) {
	w := newWorld(t)
	w.connect("personal", "first-token", true)
	w.start("--account_reload_interval=1s", "--reload_interval=1h", "--sync.sync_interval=1s")
	w.eventually(20*time.Second, "three ticks", func() bool { return w.ticked("personal") >= 3 })
	first := w.built("personal")
	for _, tok := range first[1:] {
		if tok != first[0] {
			t.Fatalf("the ticks were served by %d tokens, want one", len(slices.Compact(first)))
		}
	}
	must(t, w.conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", sealed(t, w.keys, "second-token", seal.AccountCredential("personal")))
	w.eventually(10*time.Second, "a tick over the replaced credential", func() bool {
		tokens := w.built("personal")
		return string(tokens[len(tokens)-1].Credential()) == "second-token"
	})
}

// verdicts returns the scanner revision each of the account's verdicts was made under, by message.
func (w *world) verdicts(account string) map[string]string {
	w.t.Helper()
	rows, err := w.conn.Query(w.t.Context(), "SELECT message_id, coalesce(scanner_revision, '') FROM messages WHERE account_id = $1", account)
	if err != nil {
		w.t.Fatal(err)
	}
	out := map[string]string{}
	var id, revision string
	if _, err := pgx.ForEachRow(rows, []any{&id, &revision}, func() error {
		out[id] = revision
		return nil
	}); err != nil {
		w.t.Fatal(err)
	}
	return out
}

// every returns a map holding revision for each of the account's three messages.
func every(revision string) map[string]string {
	return map[string]string{"m0": revision, "m1": revision, "m2": revision}
}

// knownRevision is the default scanner section's revision (TestTheScannerSectionBuildsTheSharedScanner).
const knownRevision = "c9fa3ff13948e4f89ac094b91a27455c"

// VERIFICATIONS' row for the run-start step, backfill's job on the worker's spec. The step is made
// once for each job's life, so verdicts made under another scanner stay as they are through the job's
// later asks, every second here, and an account no longer served and then served again gets a job of
// its own, whose step returns them to pending and scans them again under the worker's scanner
// (ADR-0119, ADR-0121, ADR-0120).
func TestTheRunStartStepIsMadeOnceForEachJobsLife(t *testing.T) {
	w := newWorld(t)
	w.port("personal", mailOf(t, "personal", 3))
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=500ms", "--reload_interval=1s")
	w.eventually(20*time.Second, "the account's backfill ending", func() bool { return w.passesEnded("personal") })
	must(t, w.conn, "UPDATE messages SET scanner_revision = 'another-revision' WHERE account_id = 'personal'")
	time.Sleep(3 * time.Second)
	if diff := cmp.Diff(every("another-revision"), w.verdicts("personal"), compare.Options); diff != "" {
		t.Errorf("the verdicts after the job's later asks, which make no run-start step (-want +got):\n%s", diff)
	}
	stored := sealed(t, w.keys, "personal-token", seal.AccountCredential("personal"))
	must(t, w.conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	w.eventually(5*time.Second, "the account no longer served losing its jobs", func() bool { return len(w.jobs("personal")) == 0 })
	must(t, w.conn, "UPDATE account_state SET credential = $1 WHERE account_id = 'personal'", stored)
	w.eventually(20*time.Second, "the run-start step of the account served again", func() bool {
		return cmp.Equal(every(knownRevision), w.verdicts("personal")) && w.passesEnded("personal")
	})
}

// VERIFICATIONS' row for the run-start step, its failure. A step that fails is made again at the
// job's next ask, since it counts as made only once it has succeeded. A worker whose step fails once,
// the application user briefly lacking a grant the step needs, returns the verdicts made under another
// scanner to pending at the next ask, and they are scanned again under its scanner (ADR-0121).
func TestAFailedRunStartStepIsMadeAgainAtTheNextAsk(t *testing.T) {
	w := newWorld(t)
	w.port("personal", mailOf(t, "personal", 3))
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=1h", "--reload_interval=1h")
	w.eventually(20*time.Second, "the account's backfill ending", func() bool { return w.passesEnded("personal") })
	w.halt()
	must(t, w.conn, "UPDATE messages SET scanner_revision = 'another-revision' WHERE account_id = 'personal'")
	revoke(t, w.conn, "UPDATE (scanned_at) ON messages")

	again := &world{t: t, conn: w.conn, keys: w.keys, logs: &lockedBuffer{}, ports: w.ports, schedules: map[string]fake.Schedule{}, tokens: map[string][]*fake.Token{}}
	again.start("--account_reload_interval=1h", "--reload_interval=1h")
	again.eventually(20*time.Second, "the run whose run-start step lacked a grant failing", func() bool {
		for _, r := range again.records("ERROR") {
			if r.JobKind == "backfill" && r.Account == "personal" {
				return true
			}
		}
		return false
	})
	must(t, w.conn, "GRANT UPDATE (scanned_at) ON messages TO "+role)
	again.eventually(20*time.Second, "the next ask making the failed step again", func() bool {
		return cmp.Equal(every(knownRevision), again.verdicts("personal"))
	})
}

// holding is a mailbox whose body reads after the first hundred wait for their context to end, as a
// provider slow to answer would, so the second pass's first page is made durable and its next one
// waits.
type holding struct {
	*fake.Fake
	mu    sync.Mutex
	reads int
}

func (h *holding) GetMessageBody(ctx context.Context, id string, part mail.BodyPart) (mail.MessageBody, error) {
	h.mu.Lock()
	h.reads++
	held := h.reads > 100
	h.mu.Unlock()
	if held {
		<-ctx.Done()
		return mail.MessageBody{}, ctx.Err()
	}
	return h.Fake.GetMessageBody(ctx, id, part)
}

// VERIFICATIONS' row for an account's series going with its work, the backlog's part. An account
// whose second pass is under way, its backlog served after the pass's first page while its next page
// waits on the provider, has the backlog series removed once its job is dropped, not only once the
// pass ends (ADR-0104, ADR-0119).
func TestADroppedAccountsBacklogGoesWithItsJob(t *testing.T) {
	w := newWorld(t)
	w.port("personal", &holding{Fake: mailOf(t, "personal", 101)})
	w.connect("personal", "personal-token", false)
	w.start("--account_reload_interval=500ms", "--reload_interval=1h")
	backlog := `mediated_mailbox_scan_backlog{account="personal",component="worker",job_kind="backfill"} 1`
	w.scrape(backlog)
	w.forget("personal")
	w.eventually(5*time.Second, "the dropped account's backlog series going", func() bool {
		return !w.served(`mediated_mailbox_scan_backlog{account="personal",component="worker",job_kind="backfill"}`)
	})
}

// backfillHeld is a mailbox whose enumeration waits for its context to end, so backfill's first
// pass never makes a page durable and the account's second pass never ends, while every call a tick
// makes is the Fake's.
type backfillHeld struct{ *fake.Fake }

func (backfillHeld) EnumerateAll(ctx context.Context, _ mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	<-ctx.Done()
	return mail.Page[mail.MessageMetadata]{}, ctx.Err()
}

// recentMail returns a Fake for the account holding a message from each sender, dated an hour ago,
// inside the first window a tick reconciles an account with no cursor over.
func recentMail(t *testing.T, account string, senders ...string) *fake.Fake {
	t.Helper()
	recent := mail.UnixMilli(time.Now().Add(-time.Hour).UnixMilli())
	var messages []fake.Message
	for i, from := range senders {
		id := fmt.Sprintf("m%d", i+1)
		messages = append(messages, fake.Message{
			Metadata: mail.MessageMetadata{ID: id, ThreadID: id, From: mail.Address{Email: from}, Subject: "Your order", Date: recent + mail.UnixMilli(i)},
			Body:     mail.MessageBody{Text: "Thanks for your order."},
		})
	}
	f, err := fake.New(fake.Config{Account: account}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// D4's parts of VERIFICATIONS' rows for the unclassified-sender volume and for scanning once
// backfill's second pass has ended, for delta sync on the worker's spec. Each tick adds what it
// counted of senders it could not classify to the account's series, and once the account's second
// pass has ended sets its scan backlog series to what still waits after its scanning, two of four
// here at two decisions a tick. An account whose second pass has not ended has no backlog series
// from delta sync, since backfill emits its own until then (O2, ADR-0104).
func TestEachTickFeedsItsUnclassifiedAndBacklogSeries(t *testing.T) {
	w := newWorld(t)
	senders := []string{"no-address-at-all", "orders@shop.example", "news@paper.example", "alerts@bank.example"}
	w.port("ended", recentMail(t, "ended", senders...))
	w.port("early", backfillHeld{recentMail(t, "early", senders...)})
	w.connect("ended", "ended-token", true)
	w.connect("early", "early-token", false)
	w.start("--account_reload_interval=1h", "--reload_interval=1h", "--sync.decisions_per_tick=2")
	w.scrape(
		`mediated_mailbox_unclassified_senders_total{account="ended",component="worker",job_kind="sync"} 1`,
		`mediated_mailbox_unclassified_senders_total{account="early",component="worker",job_kind="sync"} 1`,
		`mediated_mailbox_scan_backlog{account="ended",component="worker",job_kind="sync"} 2`,
	)
	if w.served(`mediated_mailbox_scan_backlog{account="early",component="worker",job_kind="sync"}`) {
		t.Error("delta sync serves a backlog for an account whose second pass has not ended")
	}
}

// VERIFICATIONS' row for a body's rendered part, for delta sync's tick on the worker's spec. A tick
// that scans a body whose rendered part the provider holds apart asks once more for that part and
// scans it (ADR-0131).
func TestATickScansABodyHeldApart(t *testing.T) {
	w := newWorld(t)
	f, err := fake.New(fake.Config{Account: "personal"}, fake.Message{
		Metadata:  mail.MessageMetadata{ID: "apart", ThreadID: "apart", From: mail.Address{Email: "orders@shop.example"}, Subject: "Your order", Date: mail.UnixMilli(time.Now().Add(-time.Hour).UnixMilli())},
		Body:      mail.MessageBody{HTML: "<p>Your verification code is 419283.</p>"},
		HeldApart: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	w.port("personal", f)
	w.connect("personal", "personal-token", true)
	w.start("--account_reload_interval=1h", "--reload_interval=1h")
	w.eventually(20*time.Second, "the body held apart scanned by a tick", func() bool {
		var state string
		err := w.conn.QueryRow(t.Context(), "SELECT scan_state FROM messages WHERE account_id = 'personal' AND message_id = 'apart'").Scan(&state)
		return err == nil && state == "scanned"
	})
}

// VERIFICATIONS' row for the call size a tick's metadata reads take, for delta sync on the worker's
// spec. A tick asking for the metadata of more messages than one call fits splits them into calls of
// at most the call size the unit's mailbox gives, so no call costs more than the hard cap allows
// (ADR-0023).
func TestATicksMetadataCallsFitTheCallSize(t *testing.T) {
	w := newWorld(t)
	// A rate profile declaring a small budget gives a call size well below the messages delivered.
	f, err := fake.New(fake.Config{Account: "personal", BudgetPerSecond: 5})
	if err != nil {
		t.Fatal(err)
	}
	w.port("personal", f)
	var mu sync.Mutex
	most := 0
	w.schedule("personal", func(c fake.Call) error {
		if c.Op.Operation == mail.OpGetMessageMetadata {
			mu.Lock()
			most = max(most, c.Op.Messages)
			mu.Unlock()
		}
		return nil
	})
	w.connect("personal", "personal-token", true)
	w.start("--account_reload_interval=1h", "--reload_interval=1h", "--sync.sync_interval=1s")
	w.eventually(10*time.Second, "the first tick", func() bool { return w.ticked("personal") > 0 })
	const delivered = 40
	for i := range delivered {
		if err := f.Deliver(fake.Message{
			Metadata: mail.MessageMetadata{ID: fmt.Sprintf("d%d", i), ThreadID: fmt.Sprintf("d%d", i), From: mail.Address{Email: "orders@shop.example"}, Subject: "Your order", Date: mail.UnixMilli(time.Now().UnixMilli())},
			Body:     mail.MessageBody{Text: "Thanks."},
		}); err != nil {
			t.Fatal(err)
		}
	}
	w.eventually(20*time.Second, "a tick adding every delivered message", func() bool {
		var n int
		return w.conn.QueryRow(t.Context(), "SELECT count(*) FROM messages WHERE account_id = 'personal'").Scan(&n) == nil && n == delivered
	})
	size := f.MetadataCallSize(delivered)
	mu.Lock()
	defer mu.Unlock()
	if size >= delivered {
		t.Fatalf("the fake fits all %d identifiers in one call, so the test splits nothing", delivered)
	}
	if most == 0 || most > size {
		t.Errorf("the largest metadata call named %d identifiers, want at most the call size %d", most, size)
	}
}

// VERIFICATIONS' row for each unit of work taking the policy in force, for delta sync's ticks on the
// worker's spec. A rule restricting the shop, added and loaded by the Session after an account's first
// tick, reaches the account's next tick, which skips the shop's new message as restricted (ADR-0041,
// ADR-0119).
func TestEachTickTakesThePolicyInForce(t *testing.T) {
	w := newWorld(t)
	t.Cleanup(func() { restore(t, "DELETE FROM policy_rules") })
	f, err := fake.New(fake.Config{Account: "personal"})
	if err != nil {
		t.Fatal(err)
	}
	w.port("personal", f)
	w.connect("personal", "personal-token", true)
	w.start("--account_reload_interval=500ms", "--reload_interval=1h", "--sync.sync_interval=1s")
	w.eventually(10*time.Second, "the first tick", func() bool { return w.ticked("personal") > 0 })
	must(t, w.conn, "INSERT INTO policy_rules (rule_id, class, domain_suffix, source, created_by) VALUES ('rule.shop', 'restricted', '{shop.example}', 'operator', 'operator')")
	time.Sleep(1500 * time.Millisecond)
	if err := f.Deliver(fake.Message{
		Metadata: mail.MessageMetadata{ID: "after", ThreadID: "after", From: mail.Address{Email: "orders@shop.example"}, Subject: "Your order", Date: mail.UnixMilli(time.Now().UnixMilli())},
		Body:     mail.MessageBody{Text: "Your verification code is 419283."},
	}); err != nil {
		t.Fatal(err)
	}
	w.eventually(20*time.Second, "the shop's new message decided", func() bool {
		var state string
		err := w.conn.QueryRow(t.Context(), "SELECT scan_state FROM messages WHERE account_id = 'personal' AND message_id = 'after'").Scan(&state)
		return err == nil && state != "pending"
	})
	var state string
	if err := w.conn.QueryRow(t.Context(), "SELECT scan_state FROM messages WHERE account_id = 'personal' AND message_id = 'after'").Scan(&state); err != nil || state != "skipped_restricted" {
		t.Errorf("the shop's message after the rule is %q (%v), want skipped_restricted", state, err)
	}
}

// VERIFICATIONS' rows for forcing a rotation and for an attempt recorded at a unit of work's end, for
// a tick that fails. A tick whose provider call fails after its token rotated still ends its unit of
// work, so the rotated credential is written back and the attempt recorded (ADR-0082, ADR-0097).
func TestAFailedTickStillEndsItsUnitOfWork(t *testing.T) {
	w := newWorld(t)
	attempt := mail.AuthAttempt{At: 1_790_000_000_000, Outcome: mail.AuthSucceeded}
	rotate := w.rotating("personal", "personal-rotated", attempt)
	w.schedule("personal", func(c fake.Call) error {
		if err := rotate(c); err != nil {
			return err
		}
		return mail.ErrProvider
	})
	w.connect("personal", "personal-token", true)
	before := w.storedCredential("personal")
	w.start("--account_reload_interval=1h", "--reload_interval=1h")
	w.eventually(20*time.Second, "the failed tick's unit of work ending", func() bool {
		at, _ := w.lastAuthentication("personal")
		return at != nil
	})
	if bytes.Equal(before, w.storedCredential("personal")) {
		t.Error("the credential the failed tick's token rotated to was not written back")
	}
	if w.ticked("personal") != 0 {
		t.Error("a tick whose every provider call failed was recorded as succeeded")
	}
}

// storedCredential returns the sealed credential the account's state row holds.
func (w *world) storedCredential(account string) []byte {
	w.t.Helper()
	var b []byte
	if err := w.conn.QueryRow(w.t.Context(), "SELECT credential FROM account_state WHERE account_id = $1", account).Scan(&b); err != nil {
		w.t.Fatal(err)
	}
	return b
}

// VERIFICATIONS' row for an attempt recorded at a unit of work's end, its failure, for each job kind.
// A recording of the token's latest attempt that fails fails the job kind's run, so the failure is
// loud (ADR-0097).
func TestAFailedRecordingFailsTheJobKindsRun(t *testing.T) {
	for _, c := range []struct {
		name, kind string
		ended      bool
	}{{"backfill", "backfill", false}, {"delta sync", "sync", true}} {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t)
			w.port("personal", mailOf(t, "personal", 1))
			w.schedule("personal", w.rotating("personal", "personal-token", mail.AuthAttempt{At: 1_790_000_000_000, Outcome: mail.AuthSucceeded}))
			w.connect("personal", "personal-token", c.ended)
			revoke(t, w.conn, "UPDATE (last_auth_at, last_auth_outcome) ON account_state")
			w.start("--account_reload_interval=1h", "--reload_interval=1h")
			w.eventually(20*time.Second, "the run failing at recording the attempt", func() bool {
				for _, r := range w.records("ERROR") {
					if r.JobKind == c.kind && r.Account == "personal" && strings.Contains(r.Error, "recording the last authentication attempt") {
						return true
					}
				}
				return false
			})
		})
	}
}

// VERIFICATIONS' row for a body the conversion refuses, for delta sync's tick on the worker's spec. A
// tick whose body's HTML nests more elements than the conversion accepts records the message refused
// and never scans it (ADR-0132).
func TestATickRecordsABodyTheConversionRefusesAsRefused(t *testing.T) {
	w := newWorld(t)
	f, err := fake.New(fake.Config{Account: "personal"}, fake.Message{
		Metadata: mail.MessageMetadata{ID: "deep", ThreadID: "deep", From: mail.Address{Email: "big@news.example"}, Subject: "News", Date: mail.UnixMilli(time.Now().Add(-time.Hour).UnixMilli())},
		Body:     mail.MessageBody{HTML: strings.Repeat("<div>", 600) + "Your verification code is 419283." + strings.Repeat("</div>", 600)},
	})
	if err != nil {
		t.Fatal(err)
	}
	w.port("personal", f)
	w.connect("personal", "personal-token", true)
	w.start("--account_reload_interval=1h", "--reload_interval=1h")
	w.eventually(20*time.Second, "the body the conversion refuses decided", func() bool {
		var state string
		err := w.conn.QueryRow(t.Context(), "SELECT scan_state FROM messages WHERE account_id = 'personal' AND message_id = 'deep'").Scan(&state)
		return err == nil && state != "pending"
	})
	var state string
	if err := w.conn.QueryRow(t.Context(), "SELECT scan_state FROM messages WHERE account_id = 'personal' AND message_id = 'deep'").Scan(&state); err != nil || state != "refused" {
		t.Errorf("the body the conversion refuses is %q (%v), want refused", state, err)
	}
}
