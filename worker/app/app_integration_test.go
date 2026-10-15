//go:build integration

package app

import (
	"bytes"
	"context"
	"errors"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/process/dbmetrics"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// measuredPool returns a pool of four connections connecting as the runtime role, measured on registry
// as the worker's composition root measures each job kind's pool (ADR-0125).
func measuredPool(t *testing.T, role string, registry prometheus.Registerer) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + role
	cfg.MaxConns = 4
	p, err := dbmetrics.OpenPool(t.Context(), cfg, registry)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

func sealed(t *testing.T, to seal.PublicKey, plaintext string, c seal.Context) []byte {
	t.Helper()
	b, err := to.Seal([]byte(plaintext), c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// connected writes the household Gmail client and the account personal connected through it with its
// own token, and returns a keyring that opens both.
func connected(t *testing.T) *open.Keyring {
	t.Helper()
	ring, _ := connectedKeys(t)
	return ring
}

// connectedKeys is connected, also returning the public key the keyring opens what it seals.
func connectedKeys(t *testing.T) (*open.Keyring, seal.PublicKey) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	key, err := seal.KEM().GenerateKey()
	if err != nil {
		t.Fatal(err)
	}
	seed, err := key.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	private, err := open.ParsePrivateKey(seed)
	if err != nil {
		t.Fatal(err)
	}
	ring, err := open.NewKeyring(private.PublicKey(), private)
	if err != nil {
		t.Fatal(err)
	}
	public := private.PublicKey()
	for _, table := range []string{
		"masking_events", "scan_gate_decisions", "job_run_events", "job_run_failures", "job_runs", "messages", "senders",
		"rate_grants", "rate_state", "account_state", "accounts", "oauth_clients",
	} {
		must(t, conn, "DELETE FROM "+table)
	}
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'client-id', $1)",
		sealed(t, public, "client-secret", seal.ClientSecret("household")))
	must(t, conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('personal', 'gmail', 'household')")
	must(t, conn, "INSERT INTO account_state (account_id, credential, backfill_pass1_complete, backfill_pass2_complete) VALUES ('personal', $1, true, true)",
		sealed(t, public, "personal-token", seal.AccountCredential("personal")))
	return ring, public
}

// unreachable returns an HTTP client whose every request goes through a proxy nothing listens on, so a
// token refresh or a provider request fails without reaching Google (ADR-0043).
func unreachable(t *testing.T) *http.Client {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	proxy := &url.URL{Scheme: "http", Host: ln.Addr().String()}
	if err := ln.Close(); err != nil {
		t.Fatal(err)
	}
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxy)}}
}

// probe returns the probes' answer on path.
func probe(t *testing.T, address, path string) (int, string) {
	t.Helper()
	res, err := http.Get((&url.URL{Scheme: "http", Host: address, Path: path}).String())
	if err != nil {
		t.Fatal(err)
	}
	body, err := io.ReadAll(res.Body)
	if err := errors.Join(err, res.Body.Close()); err != nil {
		t.Fatal(err)
	}
	return res.StatusCode, string(body)
}

// scrape builds the worker through Assemble and serves it through the host, as its composition root
// does, over the account
// personal connected through the household client with both of its backfill passes ended, and runs it
// until every line of want is served on its metrics endpoint or twenty seconds have passed. It then
// stops the worker, fails the test for each line of want the last scrape did not serve, and checks the
// health probe answers. Each account job's first run reaches the provider through a proxy nothing
// listens on, so delta sync's first tick fails at the token refresh after the limiter granted it one
// lease in its class, and backfill's run finds both passes ended (ADR-0043).
func scrape(t *testing.T, want []string) {
	t.Helper()
	ring := connected(t)
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	h, err := host.New(slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	registry := h.Registerer("worker")
	pools := Pools{
		Backfill: measuredPool(t, "mediated_mailbox_backfill", labelled(registry, backfill.Kind)),
		Sync:     measuredPool(t, "mediated_mailbox_sync", labelled(registry, deltasync.Kind)),
	}
	ec := mustContext(t, pools, ring, scanner, defaults().Section, registry, slog.New(slog.DiscardHandler))
	parts, err := Assemble(h.Context(), Needs{
		Section: defaults().Section, Pools: pools, Context: ec, Client: unreachable(t),
		Registerer: registry, Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	parts.Tasks = append(parts.Tasks, host.Task{Name: "execution context reload", Run: ec.Run})
	for _, check := range parts.Ready {
		if strings.HasSuffix(check.Name, " reload") && check.Check(t.Context()) == nil {
			t.Errorf("the check %s passed before the worker ran", check.Name)
		}
	}
	h.Add("worker", nil, parts)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	done := make(chan error, 1)
	go func() { done <- h.Serve(ctx, host.Listeners{Probes: ln}, host.Serving{}) }()
	var body string
	missing := func() []string {
		var out []string
		for _, line := range want {
			// A line with no value matches a series of that name and labels whatever its value.
			if !strings.Contains(body, "\n"+line+"\n") && (strings.Contains(line, " ") || !strings.Contains(body, "\n"+line+" ")) {
				out = append(out, line)
			}
		}
		return out
	}
	for deadline := time.Now().Add(20 * time.Second); ; {
		var code int
		code, body = probe(t, ln.Addr().String(), "/metrics")
		if code != http.StatusOK {
			t.Fatalf("/metrics answered %d", code)
		}
		if len(missing()) == 0 || time.Now().After(deadline) {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}

	for _, line := range missing() {
		t.Errorf("/metrics does not serve %s:\n%s", line, body)
	}
	// Each job kind's first reload has run by the time its series are served, so the worker is ready:
	// both pools answer as their roles and both reloads succeeded.
	for deadline := time.Now().Add(5 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		code, body := probe(t, ln.Addr().String(), "/readyz")
		if code == http.StatusOK {
			break
		}
		if time.Now().After(deadline) {
			t.Errorf("/readyz answered %d %q, want 200 once each job kind has reloaded", code, body)
			break
		}
	}
	if code, body := probe(t, ln.Addr().String(), "/healthz"); code != http.StatusOK || body != "ok\n" {
		t.Errorf("/healthz answered %d %q, want 200 ok", code, body)
	}
	cancel()
	if err := <-done; err != nil {
		t.Error(err)
	}
}

// D1's part of VERIFICATIONS' rows for the reload-failure alarm. The health probe answers while the
// worker runs backfill, and its metrics endpoint serves the reload-failure series of the worker's one
// policy loader, the execution context's, once and with no job kind, so the alarm reaches the platform
// from a running worker (ADR-0041, ADR-0051, ADR-0077, ADR-0090).
func TestTheProbesServeTheRunsSeries(t *testing.T) {
	scrape(t, []string{`mediated_mailbox_policyload_reload_failed{component="worker"} 0`})
}

// D4's parts of VERIFICATIONS' rows for replacing a key and stopping a series, how the series reach
// the scrape, and for the series between ticks. The worker runs until stopped serving delta sync's
// series between ticks, each labelled with delta sync's job kind. A scrape after the reload and the
// account's first tick finds the scan series of every listed account and OAuth client, the tick's gap
// and unclassified series, the execution context's reload-failure series and the rate limiter's series (ADR-0103,
// ADR-0092, ADR-0077, ADR-0024, ADR-0117).
func TestTheProbesServeEverySeriesBetweenTicks(t *testing.T) {
	scrape(t, []string{
		`mediated_mailbox_credential_on_old_key{account="personal",component="worker",job_kind="sync"} 0`,
		`mediated_mailbox_client_secret_on_old_key{client="household",component="worker",job_kind="sync"} 0`,
		`mediated_mailbox_sync_cursor_gaps_total{account="personal",component="worker",job_kind="sync"} 0`,
		`mediated_mailbox_unclassified_senders_total{account="personal",component="worker",job_kind="sync"} 0`,
		`mediated_mailbox_policyload_reload_failed{component="worker"} 0`,
		`mediated_mailbox_ratelimit_granted_total{account="personal",class="sync",component="worker",job_kind="sync"} 1`,
	})
}

// The worker's metrics endpoint serves the performance measurements once per process for the Go
// runtime, the process and its execution context's reload and policy load, and per job kind for its
// pool, its statements, its reload's duration apart from its account jobs' and its leases' waits
// (ADR-0125).
func TestTheProbesServeThePerformanceSeries(t *testing.T) {
	scrape(t, []string{
		"go_goroutines",
		"go_gc_heap_live_bytes",
		"go_cpu_classes_gc_total_cpu_seconds_total",
		"go_cpu_classes_total_cpu_seconds_total",
		"go_sched_latencies_seconds_count",
		"process_start_time_seconds",
		`mediated_mailbox_db_pool_max_connections{component="worker",job_kind="backfill"} 4`,
		`mediated_mailbox_db_pool_max_connections{component="worker",job_kind="sync"} 4`,
		`mediated_mailbox_db_pool_acquires_total{component="worker",job_kind="sync"}`,
		`mediated_mailbox_db_statement_duration_seconds_count{component="worker",job_kind="sync",statement="Accounts"}`,
		`mediated_mailbox_db_statement_duration_seconds_count{component="worker",job_kind="backfill",statement="SetTransactionAccount"}`,
		`mediated_mailbox_policyload_reload_duration_seconds_count{component="worker"}`,
		`mediated_mailbox_executioncontext_reload_duration_seconds_count{component="worker"}`,
		`mediated_mailbox_job_run_duration_seconds_count{component="worker",job="reload",job_kind="sync"}`,
		`mediated_mailbox_ratelimit_lease_wait_seconds_count{class="sync",component="worker",job_kind="sync"} 1`,
	})
}

// Each job kind's database check answers whether its own pool reaches the database as its role, so
// the worker stops reporting ready when either job kind's pool cannot answer, and a pool that answers
// leaves its own check passing (ADR-0138).
func TestEachJobKindsDatabaseCheckFollowsItsPool(t *testing.T) {
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	pools := Pools{
		Backfill: measuredPool(t, "mediated_mailbox_backfill", labelled(registry, backfill.Kind)),
		Sync:     measuredPool(t, "mediated_mailbox_sync", labelled(registry, deltasync.Kind)),
	}
	parts, err := Assemble(t.Context(), Needs{
		Section: defaults().Section, Pools: pools, Client: unreachable(t),
		Context:    mustContext(t, pools, connected(t), scanner, defaults().Section, registry, slog.New(slog.DiscardHandler)),
		Registerer: registry, Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	checks := map[string]func(context.Context) error{}
	for _, c := range parts.Ready {
		checks[c.Name] = c.Check
	}
	for _, name := range []string{"backfill database", "sync database"} {
		if checks[name] == nil {
			t.Fatalf("the worker hands the host no check named %q", name)
		}
	}
	answers := func() map[string]bool {
		return map[string]bool{
			"backfill": checks["backfill database"](t.Context()) == nil,
			"sync":     checks["sync database"](t.Context()) == nil,
		}
	}
	if diff := cmp.Diff(map[string]bool{"backfill": true, "sync": true}, answers(), compare.Options); diff != "" {
		t.Fatalf("each job kind's database check with both pools open (-want +got):\n%s", diff)
	}
	pools.Backfill.Close()
	if diff := cmp.Diff(map[string]bool{"backfill": false, "sync": true}, answers(), compare.Options); diff != "" {
		t.Errorf("each job kind's database check with backfill's pool closed (-want +got):\n%s", diff)
	}
}

// The worker's execution context tells its loader Gmail authenticates through an OAuth client, so a
// Gmail account that names none is not connected, and no job kind serves it (ADR-0106).
func TestAGmailAccountNamingNoClientIsNotConnected(t *testing.T) {
	ring := connected(t)
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	must(t, conn, "UPDATE accounts SET oauth_client = NULL WHERE account_id = 'personal'")
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	pools := Pools{Sync: measuredPool(t, "mediated_mailbox_sync", registry)}
	ec := mustContext(t, pools, ring, scanner, defaults().Section, registry, slog.New(slog.DiscardHandler))
	if err := ec.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if a, ok := ec.Snapshot().Account("personal"); !ok || a.Connected() {
		t.Errorf("the Gmail account naming no client is listed %v and connected %v, want listed and not connected", ok, a.Connected())
	}
}

// Each reload of the worker's execution context wakes every job kind's reload, so an account the
// context newly lists gets its jobs at once rather than at the job kinds' next ask, which here is an
// hour away (ADR-0090, ADR-0119).
func TestEachContextReloadWakesTheJobKinds(t *testing.T) {
	ring, public := connectedKeys(t)
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	pools := Pools{
		Backfill: measuredPool(t, "mediated_mailbox_backfill", labelled(registry, backfill.Kind)),
		Sync:     measuredPool(t, "mediated_mailbox_sync", labelled(registry, deltasync.Kind)),
	}
	section := defaults().Section
	section.ReloadInterval = time.Hour
	ec := mustContext(t, pools, ring, scanner, section, registry, slog.New(slog.DiscardHandler))
	if err := ec.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	parts, err := Assemble(t.Context(), Needs{
		Section: section, Pools: pools, Context: ec, Client: unreachable(t),
		Registerer: registry, Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- parts.Tasks[0].Run(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	// jobs returns the job kinds holding a job for the account, read from the series the scheduler
	// starts for each job it is given.
	jobs := func(account string) []string {
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		var kinds []string
		for _, f := range families {
			if f.GetName() != "mediated_mailbox_job_last_success_timestamp_seconds" {
				continue
			}
			for _, m := range f.GetMetric() {
				labels := map[string]string{}
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				if labels["account"] == account {
					kinds = append(kinds, labels["job_kind"])
				}
			}
		}
		slices.Sort(kinds)
		return kinds
	}
	for deadline := time.Now().Add(10 * time.Second); len(jobs("personal")) < 2; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the job kinds' first reloads gave the account listed at the start no jobs")
		}
	}
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	must(t, conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('work', 'gmail', 'household')")
	must(t, conn, "INSERT INTO account_state (account_id, credential, backfill_pass1_complete, backfill_pass2_complete) VALUES ('work', $1, true, true)",
		sealed(t, public, "work-token", seal.AccountCredential("work")))

	if err := ec.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	for deadline := time.Now().Add(2 * time.Second); !slices.Equal(jobs("work"), []string{backfill.Kind, deltasync.Kind}); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("an account the execution context's reload newly lists has jobs of %v two seconds later, want both job kinds'", jobs("work"))
		}
	}
}

// lockedBuffer collects log records written from several goroutines.
type lockedBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *lockedBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *lockedBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// A worker that starts before its execution context has loaded logs no error, since each job kind's
// reload asked before the load answers not due, and reports ready only once the context's first load
// has woken each job kind's reload (ADR-0119, ADR-0138).
func TestAStartLogsNoErrorAndIsReadyOnlyOnceTheContextLoads(t *testing.T) {
	ring := connected(t)
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	// No account is listed, so no account job runs and each job kind's reload is the only job it runs.
	must(t, conn, "DELETE FROM account_state")
	must(t, conn, "DELETE FROM accounts")
	scanner, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	registry := prometheus.NewRegistry()
	var log lockedBuffer
	logger := slog.New(slog.NewJSONHandler(&log, &slog.HandlerOptions{Level: slog.LevelDebug}))
	pools := Pools{
		Backfill: measuredPool(t, "mediated_mailbox_backfill", labelled(registry, backfill.Kind)),
		Sync:     measuredPool(t, "mediated_mailbox_sync", labelled(registry, deltasync.Kind)),
	}
	section := defaults().Section
	section.ReloadInterval = time.Hour
	ec := mustContext(t, pools, ring, scanner, section, registry, logger)
	parts, err := Assemble(t.Context(), Needs{
		Section: section, Pools: pools, Context: ec, Client: unreachable(t), Registerer: registry, Logger: logger,
	})
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 2)
	go func() { done <- parts.Tasks[0].Run(ctx) }()
	defer func() {
		cancel()
		for range 2 {
			if err := <-done; err != nil {
				t.Error(err)
			}
		}
	}()
	ready := func() bool {
		for _, c := range parts.Ready {
			if strings.HasSuffix(c.Name, " reload") && c.Check(t.Context()) != nil {
				return false
			}
		}
		return true
	}
	// Each job kind's reload is asked at once when the scheduler starts, before the context loads.
	asked := func() int {
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		n := 0
		for _, f := range families {
			if f.GetName() == "mediated_mailbox_job_runs_finished_total" {
				for _, m := range f.GetMetric() {
					if m.GetCounter().GetValue() > 0 {
						n++
					}
				}
			}
		}
		return n
	}
	for deadline := time.Now().Add(10 * time.Second); asked() < 2; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the job kinds' reloads were not asked within ten seconds of the start")
		}
	}
	if ready() {
		t.Error("the worker reported ready before its execution context loaded")
	}
	go func() { done <- ec.Run(ctx) }()
	for deadline := time.Now().Add(3 * time.Second); !ready(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("the worker was not ready within three seconds of its execution context's first load")
		}
	}
	if strings.Contains(log.String(), `"level":"ERROR"`) {
		t.Errorf("a start whose execution context loads logged an error:\n%s", log.String())
	}
}
