//go:build integration

package main

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// mediator returns a pool connecting as the mediator's role, under which row-level security applies
// as it does in production.
func mediator(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=mediated_mailbox_mediate"
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// superuser returns a connection that bypasses row-level security, to write the rows the tests
// stage.
func superuser(t *testing.T) *pgx.Conn {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
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

// must runs a statement and fails the test on an error.
func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// newAccounts creates two accounts named for the test.
func newAccounts(t *testing.T, conn *pgx.Conn) []string {
	t.Helper()
	prefix := "acct-" + strings.ToLower(rand.Text()[:8])
	accounts := []string{prefix + "-a", prefix + "-b"}
	for _, account := range accounts {
		must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", account)
	}
	return accounts
}

// The draft-to-approved transition is not the mediator's to make. Its role can neither write a plan's
// status or its approval nor insert a plan already approved, so no operation added to the client
// surface could perform it (ADR-0020, ADR-0021, ADR-0030).
func TestTheMediatorCannotApproveAPlan(t *testing.T) {
	conn := superuser(t)
	accounts := newAccounts(t, conn)
	must(t, conn, `INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES (gen_random_uuid(), $1, 'DRAFT', '{}')`, accounts[0])
	statements := []string{
		"UPDATE reorg_plans SET status = 'APPROVED'",
		"UPDATE reorg_plans SET approved_at = now(), approved_by = 'mediator'",
		"INSERT INTO reorg_plans (plan_id, account_id, status, plan) VALUES (gen_random_uuid(), current_setting('app.account'), 'APPROVED', '{}')",
	}
	for _, sql := range statements {
		err := func() (err error) {
			tx, err := mediator(t).Begin(t.Context())
			if err != nil {
				return err
			}
			defer func() { err = errors.Join(err, tx.Rollback(context.Background())) }()
			if _, err := tx.Exec(t.Context(), "SELECT set_config('app.account', $1, true)", accounts[0]); err != nil {
				return err
			}
			_, err = tx.Exec(t.Context(), sql)
			return err
		}()
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) || pgErr.Code != "42501" {
			t.Errorf("%s as the mediator: %v, want permission denied", sql, err)
		}
	}
}

// scanner returns the scanner the application ships.
func scanner(t *testing.T) scan.Scanner {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// keys returns a generated keyring and the public key it seals to.
func keys(t *testing.T) (*open.Keyring, seal.PublicKey) {
	t.Helper()
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
	return ring, private.PublicKey()
}

// connect writes an account's state row holding a sealed credential.
func connect(t *testing.T, conn *pgx.Conn, public seal.PublicKey, account string) {
	t.Helper()
	credential, err := public.Seal([]byte(account+"-token"), seal.AccountCredential(account))
	if err != nil {
		t.Fatal(err)
	}
	must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ($1, $2)", account, credential)
}

// logBuffer collects what the mediator logs.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (b *logBuffer) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.Write(p)
}

func (b *logBuffer) String() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.buf.String()
}

// debugLogger writes JSON lines to b at debug, the most detailed level a deployment can configure, so
// a detail line carrying what must never be logged is caught too (ADR-0119).
func debugLogger(b *logBuffer) *slog.Logger {
	return slog.New(slog.NewJSONHandler(b, &slog.HandlerOptions{Level: slog.LevelDebug}))
}

// served returns the serving state over the mediator's pool and a registry of one echo operation
// behind it, as the composition root builds them.
func served(t *testing.T, pool *pgxpool.Pool, ring *open.Keyring, metrics *prometheus.Registry, log *slog.Logger) (*serving, service.Registry) {
	t.Helper()
	reg, err := service.NewRegistry(nil, service.Operation{
		Name: "echo", Description: "Returns its arguments.", Effect: service.Read,
		Path:   "/api/accounts/{account_id}/echo",
		Input:  json.RawMessage(`{"type":"object","properties":{"account_id":{"type":"string"}},"required":["account_id"]}`),
		Output: json.RawMessage(`{"type":"object"}`),
		Handle: func(_ context.Context, account string, _ json.RawMessage) (json.RawMessage, error) {
			return json.Marshal(account)
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	s, err := newServing(t.Context(), pool, ring, metrics, log)
	if err != nil {
		t.Fatal(err)
	}
	s.registry = reg
	return s, reg
}

// serves reports whether the registry runs an operation for account.
func serves(t *testing.T, reg service.Registry, account string) bool {
	t.Helper()
	_, err := reg.Call(t.Context(), "echo", json.RawMessage(`{"account_id":"`+account+`"}`))
	return err == nil
}

// D3's part of VERIFICATIONS' row for reloading the account snapshot, for the accounts the mediator
// serves. An account connected while the mediator runs is served from the next reload, with its
// policy loaded on a reload whose policy load succeeds, and one removed is dropped. A reload whose read fails keeps the
// accounts served and is logged, and one that lists no account serves none (ADR-0090).
func TestTheMediatorServesTheAccountsEachReloadLists(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	accounts := newAccounts(t, conn)
	first, second := accounts[0], accounts[1]
	connect(t, conn, public, first)
	must(t, conn, "DELETE FROM accounts WHERE account_id = $1", second)
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
	var log logBuffer
	s, reg := served(t, mediator(t), ring, prometheus.NewRegistry(), debugLogger(&log))

	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !serves(t, reg, first) || serves(t, reg, second) {
		t.Fatalf("after the first load, %s served %v and %s served %v", first, serves(t, reg, first), second, serves(t, reg, second))
	}

	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", second)
	connect(t, conn, public, second)
	if serves(t, reg, second) {
		t.Fatal("an account connected after the load is served before a reload")
	}
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !serves(t, reg, second) {
		t.Error("an account connected while the mediator runs is not served after the next reload")
	}
	if s.policy(second).RestrictsAll() {
		t.Error("an account served after a reload has no policy of its own loaded")
	}
	if got := s.accounts(); len(got) != 2 {
		t.Errorf("the accounts listing holds %v", got)
	}

	must(t, conn, "DELETE FROM account_state WHERE account_id = $1", first)
	must(t, conn, "DELETE FROM accounts WHERE account_id = $1", first)
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if serves(t, reg, first) || !serves(t, reg, second) {
		t.Errorf("after removing %s, it is served %v and %s %v", first, serves(t, reg, first), second, serves(t, reg, second))
	}

	revoke(t, conn, "SELECT ON accounts")
	if err := s.reload(t.Context()); !errors.Is(err, accountload.ErrUntrustedRead) {
		t.Errorf("a reload whose read fails returned %v", err)
	}
	if !serves(t, reg, second) {
		t.Error("a reload whose read failed dropped the account served")
	}
	grantBack(t, conn, "SELECT ON accounts")

	must(t, conn, "DELETE FROM account_state")
	must(t, conn, "DELETE FROM accounts")
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if serves(t, reg, second) {
		t.Error("a reload listing no account still serves one")
	}
	if !strings.Contains(log.String(), "so the previous one stays") {
		t.Errorf("the failed read was not logged:\n%s", log.String())
	}
}

// A serving mediator reloads on its interval without a restart, and a failed reload on the schedule
// is logged and keeps what it served.
func TestTheMediatorReloadsOnItsInterval(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	var log logBuffer
	metrics := prometheus.NewRegistry()
	s, reg := served(t, mediator(t), ring, metrics, debugLogger(&log))
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	token := filepath.Join(t.TempDir(), "token")
	if err := os.WriteFile(token, []byte("s3cret"), 0o600); err != nil {
		t.Fatal(err)
	}
	c := Configuration{TLSAtIngress: true, TokenFile: token, AccountReloadInterval: 20 * time.Millisecond}
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	surfaceListener, probeListener := loopback(t), loopback(t)
	go func() {
		done <- serve(ctx, c, surfaceListener, probeListener, s, metrics, slog.New(slog.DiscardHandler))
	}()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	account := newAccounts(t, conn)[0]
	connect(t, conn, public, account)
	deadline := time.Now().Add(5 * time.Second)
	for !serves(t, reg, account) {
		if time.Now().After(deadline) {
			t.Fatal("an account connected while the mediator serves was never served")
		}
		time.Sleep(10 * time.Millisecond)
	}
	revoke(t, conn, "SELECT ON accounts")
	deadline = time.Now().Add(5 * time.Second)
	for !strings.Contains(log.String(), "the accounts were not reloaded, so the ones served stay") {
		if time.Now().After(deadline) {
			t.Fatalf("a failed scheduled reload was not logged:\n%s", log.String())
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !serves(t, reg, account) {
		t.Error("a failed scheduled reload dropped the account served")
	}
}

// The metrics endpoint carries F3's rate-state series for each account the mediator serves through
// the Gmail adapter, and only for those, following each reload (ADR-0077).
func TestTheMetricsEndpointCarriesEachServedAccountsRateState(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	accounts := newAccounts(t, conn)
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'fastmail')", accounts[0]+"-other")
	for _, a := range accounts {
		connect(t, conn, public, a)
	}
	pool := mediator(t)
	metrics := prometheus.NewRegistry()
	s, _ := served(t, pool, ring, metrics, slog.New(slog.DiscardHandler))
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	reported := func() []string {
		families, err := metrics.Gather()
		if err != nil {
			t.Fatal(err)
		}
		var out []string
		for _, f := range families {
			if f.GetName() != "mediated_mailbox_ratelimit_account_rate" {
				continue
			}
			for _, m := range f.GetMetric() {
				for _, l := range m.GetLabel() {
					out = append(out, l.GetValue())
				}
			}
		}
		slices.Sort(out)
		return out
	}
	if diff := cmp.Diff(accounts, reported()); diff != "" {
		t.Errorf("the accounts the rate series report (-want +got):\n%s", diff)
	}
	must(t, conn, "DELETE FROM account_state WHERE account_id = $1", accounts[0])
	must(t, conn, "DELETE FROM accounts WHERE account_id = $1", accounts[0])
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(accounts[1:], reported()); diff != "" {
		t.Errorf("after a reload dropped an account, the rate series report (-want +got):\n%s", diff)
	}
}

// reset empties the tables that refer to accounts and the accounts themselves. Every test of the
// package shares one database.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	for _, table := range []string{"reorg_plans", "messages", "audit_log", "account_state", "rate_grants", "rate_state", "policy_rules", "accounts", "oauth_clients"} {
		must(t, conn, "DELETE FROM "+table)
	}
}

// revoke revokes a grant of the mediator's role until grantBack or the end of the test.
func revoke(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "REVOKE "+privilege+" FROM mediated_mailbox_mediate")
	url := postgres.URL(t)
	t.Cleanup(func() {
		ctx := context.Background()
		c, err := pgx.Connect(ctx, url)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := c.Close(ctx); err != nil {
				t.Error(err)
			}
		}()
		if _, err := c.Exec(ctx, "GRANT "+privilege+" TO mediated_mailbox_mediate"); err != nil {
			t.Error(err)
		}
	})
}

// grantBack restores a grant revoke took.
func grantBack(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "GRANT "+privilege+" TO mediated_mailbox_mediate")
}

// reloadFailed returns the values of the policy loader's reload-failure series on the registry the
// mediator serves as its metrics endpoint (ADR-0041, ADR-0077).
func reloadFailed(t *testing.T, metrics *prometheus.Registry) []float64 {
	t.Helper()
	families, err := metrics.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var values []float64
	for _, f := range families {
		if f.GetName() == "mediated_mailbox_policyload_reload_failed" {
			for _, m := range f.GetMetric() {
				values = append(values, m.GetGauge().GetValue())
			}
		}
	}
	return values
}

// A reload whose policy load fails still serves the accounts its snapshot lists, so an account the
// snapshot drops is dropped (ADR-0090). An account the policy was loaded for before keeps that policy,
// and one added meanwhile restricts every sender until a policy load reads its rules (ADR-0041). The
// next reload loads the policy again. The metrics endpoint carries no reload-failure series while no
// account is served, and one series that reads 0 after a load that succeeded and 1 after one that
// failed, which the reload-failure alarm reads (ADR-0077).
func TestAFailedPolicyLoadStillFollowsTheSnapshot(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	metrics := prometheus.NewRegistry()
	s, reg := served(t, mediator(t), ring, metrics, slog.New(slog.DiscardHandler))
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := reloadFailed(t, metrics); len(got) != 0 {
		t.Errorf("with no account served, the metrics endpoint carries the reload-failure series %v", got)
	}

	accounts := newAccounts(t, conn)
	kept, dropped := accounts[0], accounts[1]
	for _, a := range accounts {
		connect(t, conn, public, a)
	}
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if got := reloadFailed(t, metrics); !slices.Equal(got, []float64{0}) {
		t.Errorf("after a policy load that succeeded, the reload-failure series reads %v, want one series at 0", got)
	}

	const policyRead = "SELECT (account_id, rule_id, class, domain_suffix) ON policy_rules"
	revoke(t, conn, policyRead)
	must(t, conn, "DELETE FROM account_state WHERE account_id = $1", dropped)
	must(t, conn, "DELETE FROM accounts WHERE account_id = $1", dropped)
	added := kept + "-added"
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", added)
	connect(t, conn, public, added)
	if err := s.reload(t.Context()); !errors.Is(err, policyload.ErrUntrustedRead) {
		t.Fatalf("a reload whose policy load fails returned %v", err)
	}
	if got := reloadFailed(t, metrics); !slices.Equal(got, []float64{1}) {
		t.Errorf("after a policy load that failed, the reload-failure series reads %v, want one series at 1", got)
	}
	if serves(t, reg, dropped) {
		t.Error("an account the snapshot dropped is still served after the policy load failed")
	}
	if !serves(t, reg, kept) || !serves(t, reg, added) {
		t.Errorf("after the policy load failed, %s is served %v and %s %v", kept, serves(t, reg, kept), added, serves(t, reg, added))
	}
	if s.policy(kept).RestrictsAll() {
		t.Error("an account whose policy was loaded before lost it when a later policy load failed")
	}
	if !s.policy(added).RestrictsAll() {
		t.Error("an account added while the policy load failed is decided under a policy that never read its rules")
	}

	grantBack(t, conn, policyRead)
	if err := s.reload(t.Context()); err != nil {
		t.Fatalf("the next reload returned %v", err)
	}
	if got := reloadFailed(t, metrics); !slices.Equal(got, []float64{0}) {
		t.Errorf("after the retried policy load succeeded, the reload-failure series reads %v, want one series at 0", got)
	}
	if s.policy(added).RestrictsAll() {
		t.Error("the next reload did not load the policy of the account added meanwhile")
	}
}

// The read operations the composition root builds read the serving state it holds and the real clock.
// The accounts listing lists the snapshot's accounts, a message is decided under the policy loaded
// for its account, so a sender the policy lists reads restricted and an unlisted one normal. The
// system status reads a backoff ending in an hour as running, one that ended an hour ago as over, and
// a sync cursor written an hour ago as an hour old, to within thirty seconds.
func TestTheReadOperationsReadTheServingState(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	accounts := newAccounts(t, conn)
	account := accounts[0]
	connect(t, conn, public, account)
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
	for id, from := range map[string]string{"m-bank": "alerts@bank.example", "m-news": "editor@newsletter.example"} {
		must(t, conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at,
			has_attachments, sender_class, scan_state) VALUES ($1, $2, 't', $3, $4, now(), false, 'normal', 'scanned')`,
			account, id, from, from[strings.IndexByte(from, '@')+1:])
	}
	// One account's backoff ends an hour from now and the other's ended an hour ago, and the first
	// account's sync cursor was written an hour ago, so a clock ahead of the real one, behind it or
	// stopped reads at least one of them wrong.
	must(t, conn, `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, backoff_until)
		VALUES ($1, 5, 8, 10, now() + interval '1 hour'), ($2, 5, 8, 10, now() - interval '1 hour')`, account, accounts[1])
	must(t, conn, "UPDATE account_state SET sync_cursor_at = now() - interval '1 hour' WHERE account_id = $1", account)
	pool := mediator(t)
	metrics := prometheus.NewRegistry()
	s, err := newServing(t.Context(), pool, ring, metrics, slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	bodies, _, err := newBodies(s, nil, scanner(t), time.Second, metrics)
	if err != nil {
		t.Fatal(err)
	}
	reg, err := service.NewRegistry(nil, service.Operations(sources(pool, s, bodies))...)
	if err != nil {
		t.Fatal(err)
	}
	s.registry = reg
	if err := s.reload(t.Context()); err != nil {
		t.Fatal(err)
	}

	out, err := reg.Call(t.Context(), "list_accounts", json.RawMessage(`{}`))
	if err != nil {
		t.Fatal(err)
	}
	if want := `{"accounts":[{"account_id":"` + accounts[0] + `","provider":"gmail"},{"account_id":"` + accounts[1] + `","provider":"gmail"}]}`; string(out) != want {
		t.Errorf("list_accounts returned %s, want %s", out, want)
	}
	for id, want := range map[string]string{"m-bank": "restricted false", "m-news": "normal true"} {
		out, err := reg.Call(t.Context(), "get_message", json.RawMessage(`{"account_id":"`+account+`","message_id":"`+id+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		var m struct {
			Sensitivity struct {
				SenderClass string `json:"sender_class"`
			} `json:"sensitivity"`
			BodyAvailable bool `json:"body_available"`
		}
		if err := json.Unmarshal(out, &m); err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%s %v", m.Sensitivity.SenderClass, m.BodyAvailable); got != want {
			t.Errorf("get_message %s reads %s, want %s", id, got, want)
		}
	}
	type status struct {
		Sync struct {
			CursorAgeSeconds float64 `json:"cursor_age_seconds"`
		} `json:"sync"`
		Rate struct {
			InBackoff bool `json:"in_backoff"`
		} `json:"rate"`
	}
	read := func(id string) (status, string) {
		out, err := reg.Call(t.Context(), "get_system_status", json.RawMessage(`{"account_id":"`+id+`"}`))
		if err != nil {
			t.Fatal(err)
		}
		var st status
		if err := json.Unmarshal(out, &st); err != nil {
			t.Fatal(err)
		}
		return st, string(out)
	}
	ahead, text := read(account)
	if !ahead.Rate.InBackoff {
		t.Errorf("a backoff recorded to end in an hour reads as over: %s", text)
	}
	if age := ahead.Sync.CursorAgeSeconds; age < 3600-30 || age > 3600+30 {
		t.Errorf("a sync cursor written an hour ago reads %v seconds old: %s", age, text)
	}
	if past, text := read(accounts[1]); past.Rate.InBackoff {
		t.Errorf("a backoff that ended an hour ago reads as running: %s", text)
	}
}
