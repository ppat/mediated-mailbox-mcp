//go:build integration

package deltasync

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// The role delta sync connects as, so row-level security and its grants apply as in production.
const role = "mediated_mailbox_sync"

// syncPool returns a pool connecting as role.
func syncPool(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=" + role
	p, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(p.Close)
	return p
}

// superuser returns a connection that bypasses row-level security, to write what the UI's setups
// would write and to break what the tests break.
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

func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// revoke revokes a grant of delta sync's role for the rest of the test, and grants it again when the
// test ends, so the tests after it see the role as the migration chain made it.
func revoke(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "REVOKE "+privilege+" FROM "+role)
	address := postgres.URL(t)
	t.Cleanup(func() {
		ctx := context.Background()
		c, err := pgx.Connect(ctx, address)
		if err != nil {
			t.Error(err)
			return
		}
		defer func() {
			if err := c.Close(ctx); err != nil {
				t.Error(err)
			}
		}()
		if _, err := c.Exec(ctx, "GRANT "+privilege+" TO "+role); err != nil {
			t.Error(err)
		}
	})
}

// reset empties the tables delta sync reads and writes. Every test of the package shares one
// database, and a reload lists every account in it.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	for _, table := range []string{
		"masking_events", "scan_gate_decisions", "job_run_events", "job_run_failures", "job_runs", "messages", "senders",
		"rate_grants", "rate_state", "account_state", "accounts", "oauth_clients",
	} {
		must(t, conn, "DELETE FROM "+table)
	}
}

// keyPair returns a generated private key and its public key.
func keyPair(t *testing.T) (open.PrivateKey, seal.PublicKey) {
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
	return private, private.PublicKey()
}

// keys returns a keyring holding one generated key and the public key it seals to.
func keys(t *testing.T) (*open.Keyring, seal.PublicKey) {
	t.Helper()
	private, public := keyPair(t)
	ring, err := open.NewKeyring(public, private)
	if err != nil {
		t.Fatal(err)
	}
	return ring, public
}

func sealed(t *testing.T, to seal.PublicKey, plaintext string, c seal.Context) []byte {
	t.Helper()
	b, err := to.Seal([]byte(plaintext), c)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// account writes an account, and a state row holding credential unless stateless. An account of a
// provider for which the household client is stored connects through it, as account setup writes it.
func account(t *testing.T, conn *pgx.Conn, id, provider string, credential []byte, stateless bool) {
	t.Helper()
	must(t, conn, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ($1, $2, (SELECT name FROM oauth_clients WHERE name = $3 AND provider = $2))", id, provider, household)
	if !stateless {
		must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ($1, $2)", id, credential)
	}
}

// household is the name of the Gmail client the tests' accounts connect through. It is not the
// provider's name, so nothing finds it by the provider.
const household = "household"

// client writes the household Gmail client, its secret sealed to public, and connects every Gmail
// account that names no client through it.
func client(t *testing.T, conn *pgx.Conn, public seal.PublicKey) {
	t.Helper()
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ($1, $2, $3, $4)",
		household, gmailProvider, "client-id", sealed(t, public, "client-secret", seal.ClientSecret(household)))
	must(t, conn, "UPDATE accounts SET oauth_client = $1 WHERE provider = $2 AND oauth_client IS NULL", household, gmailProvider)
}

func storedCredential(t *testing.T, conn *pgx.Conn, id string) []byte {
	t.Helper()
	var b []byte
	if err := conn.QueryRow(t.Context(), "SELECT credential FROM account_state WHERE account_id = $1", id).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

func storedSecret(t *testing.T, conn *pgx.Conn) []byte {
	t.Helper()
	var b []byte
	if err := conn.QueryRow(t.Context(), "SELECT client_secret FROM oauth_clients WHERE name = $1", household).Scan(&b); err != nil {
		t.Fatal(err)
	}
	return b
}

// gmailSource builds the Gmail token source the job kind builds from an account's credentials, calling
// through client.
func gmailSource(client *http.Client) func(session.Credentials) *gmail.TokenSource {
	return func(c session.Credentials) *gmail.TokenSource {
		return gmail.NewTokenSource(client, gmail.NewCredentials(c.ClientID, c.ClientSecret, c.Credential))
	}
}

// fakes returns the connector whose sources are built with source and whose ports answer each
// account with its fake, refusing an account with none.
func fakes(source func(session.Credentials) *gmail.TokenSource, boxes map[string]*fake.Fake) session.Connector {
	return session.Connect(source, func(account string, _ *gmail.TokenSource) (mail.Port[context.Context], error) {
		f, ok := boxes[account]
		if !ok {
			return nil, fmt.Errorf("no mailbox for account %s", account)
		}
		return f, nil
	})
}

// mailbox returns an empty provider fake for the account.
func mailbox(t *testing.T, account string, messages ...fake.Message) *fake.Fake {
	t.Helper()
	f, err := fake.New(fake.Config{Account: account}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// jobs stands in for the worker's scheduler. It holds the account jobs the kind's reload ensured and
// records every job it was asked to add or drop, so a test runs each job itself, as the scheduler
// would ask it. Each round of asks comes a sync interval and more after the one before, so every
// job is due at each.
type jobs struct {
	held             map[schedule.Key]schedule.Job
	ensured, removed []string
	rounds           int
}

func newJobs() *jobs { return &jobs{held: map[schedule.Key]schedule.Job{}} }

func (j *jobs) Ensure(key schedule.Key, job schedule.Job) error {
	if _, ok := j.held[key]; ok {
		return nil
	}
	j.held[key] = job
	j.ensured = append(j.ensured, key.ID)
	return nil
}

func (j *jobs) Remove(key schedule.Key) {
	delete(j.held, key)
	j.removed = append(j.removed, key.ID)
}

// tick asks each account job it holds once, in the order of the accounts, and returns the runs'
// failures joined, each naming its account.
func (j *jobs) tick(t *testing.T) error {
	t.Helper()
	j.rounds++
	at := time.UnixMilli(1_790_000_000_000).Add(time.Duration(j.rounds) * time.Hour)
	var failures []error
	for _, key := range slices.SortedFunc(maps.Keys(j.held), func(a, b schedule.Key) int { return strings.Compare(a.ID, b.ID) }) {
		if err := j.held[key].Run(t.Context(), schedule.Ask{At: at, Phase: at}); err != nil {
			failures = append(failures, fmt.Errorf("account %s: %w", key.ID, err))
		}
	}
	return errors.Join(failures...)
}

// kind returns delta sync's job kind over pool and ring with the worker's defaults, scanning with the
// default scanner, with its series on registry, its account sessions built through connect and its
// account jobs added to j. Each of configure then changes its configuration.
func kind(t *testing.T, pool *pgxpool.Pool, ring *open.Keyring, registry prometheus.Registerer, log *slog.Logger, connect session.Connector, j *jobs, configure ...func(*Config)) *DeltaSync {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	c := Config{
		Pool: pool, Keys: ring, Scanner: s, Registry: registry, Logger: log, Jobs: j, Concurrency: 1,
		ReloadInterval: time.Minute, SyncInterval: 5 * time.Minute, FirstWindow: 7 * 24 * time.Hour, DecisionsPerTick: 200,
	}
	for _, f := range configure {
		f(&c)
	}
	d, err := build(c, map[string]session.Connector{gmailProvider: connect})
	if err != nil {
		t.Fatal(err)
	}
	return d
}

// cycle runs the kind's reload, then asks each account job it holds, and returns their failures
// joined.
func cycle(t *testing.T, d *DeltaSync, j *jobs) error {
	t.Helper()
	err := d.reload(t.Context(), schedule.Ask{})
	return errors.Join(err, j.tick(t))
}

// record is one log record's message and the attributes the tests read.
type record struct {
	Level, Msg, Account, Provider string
}

// logBuffer collects the records a run logs.
type logBuffer struct{ bytes.Buffer }

func (b *logBuffer) logger() *slog.Logger { return slog.New(slog.NewJSONHandler(b, nil)) }

func (b *logBuffer) records(t *testing.T) []record {
	t.Helper()
	var out []record
	for line := range strings.Lines(b.String()) {
		var r record
		if err := json.Unmarshal([]byte(line), &r); err != nil {
			t.Fatalf("log line %q: %v", line, err)
		}
		out = append(out, r)
	}
	return out
}

// ticked returns the accounts that have a tick recorded, sorted.
func ticked(t *testing.T, conn *pgx.Conn) []string {
	t.Helper()
	rows, err := conn.Query(t.Context(), "SELECT DISTINCT account_id FROM job_runs WHERE workload = 'sync' AND pass = 'tick' ORDER BY 1")
	if err != nil {
		t.Fatal(err)
	}
	ids, err := pgx.CollectRows(rows, pgx.RowTo[string])
	if err != nil {
		t.Fatal(err)
	}
	return ids
}

// D4's parts of VERIFICATIONS' rows for loading the account snapshot within each run and for a
// credential supplied outside the account's state row. The kind's reload takes its accounts, the OAuth
// clients and the credentials from the database, and adds a job only for a connected account whose
// provider it has an adapter for and that connects through a client, whatever the environment and a
// mounted file hold, logging every other one, a Gmail account naming no client while the provider has
// one included. A tick rewrites no credential the provider never rotated (ADR-0080, ADR-0090,
// ADR-0106).
func TestATickTakesItsAccountsFromTheDatabase(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "no-state", gmailProvider, nil, true)
	account(t, conn, "no-credential", gmailProvider, nil, false)
	account(t, conn, "work", "fastmail", sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	account(t, conn, "unpaired", gmailProvider, sealed(t, public, "unpaired-token", seal.AccountCredential("unpaired")), false)
	client(t, conn, public)
	must(t, conn, "UPDATE accounts SET oauth_client = NULL WHERE account_id = 'unpaired'")
	t.Setenv("GMAIL_REFRESH_TOKEN", "environment-token")
	mounted := filepath.Join(t.TempDir(), "refresh_token")
	if err := os.WriteFile(mounted, []byte("mounted-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	stored := storedCredential(t, conn, "personal")
	var log logBuffer
	var built []string
	source := func(c session.Credentials) *gmail.TokenSource {
		built = append(built, string(c.Credential))
		return gmailSource(http.DefaultClient)(c)
	}
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), log.logger(), fakes(source, map[string]*fake.Fake{"personal": mailbox(t, "personal")}), j)

	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"personal"}, ticked(t, conn), compare.Options); diff != "" {
		t.Errorf("the accounts ticked (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"personal"}, j.ensured, compare.Options); diff != "" {
		t.Errorf("the account jobs added (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"personal-token"}, built, compare.Options); diff != "" {
		t.Errorf("the refresh tokens the sources were built from (-want +got):\n%s", diff)
	}
	var warned []record
	for _, r := range log.records(t) {
		if r.Level == "WARN" {
			warned = append(warned, r)
		}
	}
	want := []record{
		{Level: "WARN", Msg: "an account's provider authenticates through an OAuth client and the account has none that opened, so it is not connected", Account: "unpaired", Provider: gmailProvider},
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "no-credential"},
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "no-state"},
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "unpaired"},
		{Level: "WARN", Msg: "there is no adapter for an account's provider, so it is skipped", Account: "work", Provider: "fastmail"},
	}
	if diff := cmp.Diff(want, warned, compare.Options); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
	if got := storedCredential(t, conn, "personal"); !bytes.Equal(got, stored) {
		t.Error("the tick rewrote a credential the provider never rotated")
	}
}

// D4's part of VERIFICATIONS' row for loading the account snapshot within each run. Each reload loads
// the snapshot again, so an account connected since is ticked once a reload has added its job, and the
// job of one no longer connected is dropped. A tick takes the snapshot the last reload loaded and reads
// none of its own, so an account connected after that reload is not ticked by the ticks before the
// next, and an account disconnected since that reload is ticked until the next drops its job. A reload
// whose read of the snapshot fails returns the failure, keeps the previous snapshot, which the loader
// logs, and adds and drops no job, so the accounts served go on ticking (ADR-0090, ADR-0103,
// ADR-0119).
func TestEachReloadLoadsTheAccountSnapshot(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	boxes := map[string]*fake.Fake{"personal": mailbox(t, "personal"), "second": mailbox(t, "second")}
	var log logBuffer
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), log.logger(), fakes(gmailSource(http.DefaultClient), boxes), j)
	ticks := func() map[string]int {
		rows, err := conn.Query(t.Context(), "SELECT account_id, count(*) FROM job_runs WHERE workload = 'sync' AND pass = 'tick' GROUP BY 1")
		if err != nil {
			t.Fatal(err)
		}
		out := map[string]int{}
		var id string
		var n int
		if _, err := pgx.ForEachRow(rows, []any{&id, &n}, func() error { out[id] = n; return nil }); err != nil {
			t.Fatal(err)
		}
		return out
	}
	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}
	account(t, conn, "second", gmailProvider, sealed(t, public, "second-token", seal.AccountCredential("second")), false)

	if err := j.tick(t); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(map[string]int{"personal": 2}, ticks(), compare.Options); diff != "" {
		t.Errorf("the ticks by account after an account was connected and before a reload (-want +got):\n%s", diff)
	}

	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(map[string]int{"personal": 3, "second": 1}, ticks(), compare.Options); diff != "" {
		t.Errorf("the ticks by account after the reload (-want +got):\n%s", diff)
	}
	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")

	if err := j.tick(t); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(map[string]int{"personal": 4, "second": 2}, ticks(), compare.Options); diff != "" {
		t.Errorf("the ticks by account after one was disconnected and before a reload (-want +got):\n%s", diff)
	}

	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(map[string]int{"personal": 4, "second": 3}, ticks(), compare.Options); diff != "" {
		t.Errorf("the ticks by account after the reload (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"personal"}, j.removed, compare.Options); diff != "" {
		t.Errorf("the account jobs dropped (-want +got):\n%s", diff)
	}
	revoke(t, conn, "SELECT (account_id, provider) ON accounts")

	if err := cycle(t, d, j); !errors.Is(err, accountload.ErrUntrustedRead) {
		t.Fatalf("the reload and ticks returned %v, want the failed read", err)
	}

	if diff := cmp.Diff(map[string]int{"personal": 4, "second": 4}, ticks(), compare.Options); diff != "" {
		t.Errorf("the ticks by account after a failed read (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"personal", "second"}, j.ensured, compare.Options); diff != "" {
		t.Errorf("the account jobs added (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff([]string{"personal"}, j.removed, compare.Options); diff != "" {
		t.Errorf("the account jobs dropped after a failed read (-want +got):\n%s", diff)
	}
	if !strings.Contains(log.String(), "the account snapshot was not reloaded, so the previous one stays") {
		t.Error("the failed read was not logged")
	}
}

// keyIdentifier returns the identifier of the key a stored sealed value names.
func keyIdentifier(t *testing.T, stored []byte) string {
	t.Helper()
	parts, err := seal.Split(stored)
	if err != nil {
		t.Fatal(err)
	}
	return parts.KeyID.String()
}

// keyScan returns the scan series the registry holds, keyed on the series and its label.
func keyScan(t *testing.T, reg *prometheus.Registry) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, f := range families {
		if !strings.HasSuffix(f.GetName(), "_on_old_key") {
			continue
		}
		for _, m := range f.GetMetric() {
			out[f.GetName()+" "+m.GetLabel()[0].GetValue()] = m.GetGauge().GetValue()
		}
	}
	return out
}

// D4's parts of VERIFICATIONS' row for re-sealing to the current key with its scan. With a keyring
// holding an old and a current key, the kind's reload seals an account's credential and the household
// client's secret stored under the old key again to the current one, and their scan series read 1
// while a re-seal cannot land and 0 once it has. An account whose credential cannot be opened reads 1,
// and an account with no state row reads 0 (ADR-0092, ADR-0103).
func TestAReloadReSealsWhatItOpensWithAnOldKey(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	oldPrivate, oldPublic := keyPair(t)
	newPrivate, newPublic := keyPair(t)
	_, otherPublic := keyPair(t)
	ring, err := open.NewKeyring(newPublic, oldPrivate, newPrivate)
	if err != nil {
		t.Fatal(err)
	}
	account(t, conn, "personal", gmailProvider, sealed(t, oldPublic, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "broken", gmailProvider, sealed(t, otherPublic, "broken-token", seal.AccountCredential("broken")), false)
	account(t, conn, "stateless", gmailProvider, nil, true)
	client(t, conn, oldPublic)
	reg := prometheus.NewRegistry()
	d := kind(t, syncPool(t), ring, reg, slog.New(slog.DiscardHandler), fakes(gmailSource(http.DefaultClient), map[string]*fake.Fake{"personal": mailbox(t, "personal")}), newJobs())
	revoke(t, conn, "UPDATE (credential) ON account_state")
	revoke(t, conn, "UPDATE (client_secret) ON oauth_clients")

	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}

	before := map[string]float64{
		"mediated_mailbox_credential_on_old_key personal": 1, "mediated_mailbox_credential_on_old_key broken": 1,
		"mediated_mailbox_credential_on_old_key stateless": 0, "mediated_mailbox_client_secret_on_old_key household": 1,
	}
	if diff := cmp.Diff(before, keyScan(t, reg), compare.Options); diff != "" {
		t.Errorf("the series while the re-seal cannot land (-want +got):\n%s", diff)
	}
	must(t, conn, "GRANT UPDATE (credential) ON account_state TO "+role)
	must(t, conn, "GRANT UPDATE (client_secret) ON oauth_clients TO "+role)

	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}

	after := map[string]float64{
		"mediated_mailbox_credential_on_old_key personal": 0, "mediated_mailbox_credential_on_old_key broken": 1,
		"mediated_mailbox_credential_on_old_key stateless": 0, "mediated_mailbox_client_secret_on_old_key household": 0,
	}
	if diff := cmp.Diff(after, keyScan(t, reg), compare.Options); diff != "" {
		t.Errorf("the series once re-sealed (-want +got):\n%s", diff)
	}
	current := keyIdentifier(t, sealed(t, newPublic, "x", seal.AccountCredential("x")))
	if got := keyIdentifier(t, storedCredential(t, conn, "personal")); got != current {
		t.Errorf("the account's credential is sealed to %s, want the current key %s", got, current)
	}
	if got := keyIdentifier(t, storedSecret(t, conn)); got != current {
		t.Errorf("the client's secret is sealed to %s, want the current key %s", got, current)
	}
	restarted := accountload.New(syncPool(t), ring, slog.New(slog.DiscardHandler), []string{gmailProvider})
	if err := restarted.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	a, _ := restarted.Snapshot().Account("personal")
	if c, ok := a.Client(); !ok || string(c.Secret()) != "client-secret" {
		t.Errorf("after a restart the client's secret opens to %v, want client-secret", ok)
	}
}

// unreachable returns an HTTP client whose every request goes through a proxy nothing listens on, so a
// token refresh or a provider request fails without reaching Google.
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

// D4's parts of VERIFICATIONS' rows for the rotation hand-over and for failing a unit of work after an
// authentication attempt. Each account's tick is a unit of work. When it ends, a failed one included,
// the account's source's current refresh token is handed over, so a rotated one reaches the account's
// state row and a process started afterwards reads it, and the source's latest authentication attempt
// is recorded. Google's rotation and its token endpoint are out of reach without a stand-in for them
// (ADR-0043), so each source is built holding the token a rotation would leave, and its attempt is a
// refresh through an unreachable proxy (ADR-0082, ADR-0097).
func TestEachTickHandsItsAccountOverAndRecordsItsAttempt(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	var attempt mail.AuthAttempt
	source := func(c session.Credentials) *gmail.TokenSource {
		c.Credential = append(slices.Clone(c.Credential), "-rotated"...)
		s := gmailSource(unreachable(t))(c)
		if _, err := s.AccessToken(t.Context()); err == nil {
			t.Fatal("a refresh through an unreachable proxy returned an access token")
		}
		attempt = s.LastAttempt()
		return s
	}
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler), fakes(source, map[string]*fake.Fake{"personal": mailbox(t, "personal")}), j)
	revoke(t, conn, "INSERT (account_id, run_id, workload, pass, state, resumed_from, started_at, heartbeat_at, checkpoint, counters) ON job_runs")

	if err := cycle(t, d, j); err == nil {
		t.Fatal("the tick succeeded though it could not record its run")
	}

	restarted := accountload.New(syncPool(t), ring, slog.New(slog.DiscardHandler), []string{gmailProvider})
	if err := restarted.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	if a, _ := restarted.Snapshot().Account("personal"); string(a.Credential()) != "personal-token-rotated" {
		t.Errorf("after a restart the account holds %q, want the rotated token", a.Credential())
	}
	var at time.Time
	var outcome string
	if err := conn.QueryRow(t.Context(), "SELECT last_auth_at, last_auth_outcome FROM account_state WHERE account_id = 'personal'").Scan(&at, &outcome); err != nil {
		t.Fatal(err)
	}
	if outcome != "failed" || !at.Equal(time.UnixMilli(int64(attempt.At))) {
		t.Errorf("the account's last authentication reads %s at %v, want failed at %d", outcome, at, attempt.At)
	}
}

// gauges returns the value of every series of the family name the registry holds, keyed on its
// account label.
func gauges(t *testing.T, reg *prometheus.Registry, name string) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			v := m.GetGauge().GetValue()
			if m.GetCounter() != nil {
				v = m.GetCounter().GetValue()
			}
			out[m.GetLabel()[0].GetValue()] = v
		}
	}
	return out
}

// D4's parts of VERIFICATIONS' rows for the unclassified-sender volume and for scanning once
// backfill's second pass has ended. Each tick adds what it counted of senders it could not classify to
// the account's series, and once the account's second pass has ended sets its scan backlog series to
// what still waits after its scanning. An account whose second pass has not ended has no backlog
// series from delta sync, since backfill emits its own until then (O2, ADR-0104).
func TestEachTickFeedsItsUnclassifiedAndBacklogSeries(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "ended", gmailProvider, sealed(t, public, "ended-token", seal.AccountCredential("ended")), false)
	account(t, conn, "early", gmailProvider, sealed(t, public, "early-token", seal.AccountCredential("early")), false)
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'ended'")
	client(t, conn, public)
	recent := mail.UnixMilli(time.Now().Add(-time.Hour).UnixMilli())
	message := func(id, from string) fake.Message {
		return fake.Message{
			Metadata: mail.MessageMetadata{ID: id, ThreadID: id, From: mail.Address{Email: from}, Subject: "Your order", Date: recent},
			Body:     mail.MessageBody{Text: "Thanks for your order."},
		}
	}
	boxes := map[string]*fake.Fake{}
	for _, a := range []string{"ended", "early"} {
		boxes[a] = mailbox(t, a, message("m1", "no-address-at-all"), message("m2", "orders@shop.example"),
			message("m3", "news@paper.example"), message("m4", "alerts@bank.example"))
	}
	reg := prometheus.NewRegistry()
	j := newJobs()
	d := kind(t, syncPool(t), ring, reg, slog.New(slog.DiscardHandler), fakes(gmailSource(http.DefaultClient), boxes), j,
		func(c *Config) { c.DecisionsPerTick = 2 })

	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff(map[string]float64{"ended": 1, "early": 1}, gauges(t, reg, series.UnclassifiedName), compare.Options); diff != "" {
		t.Errorf("the unclassified-sender series (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(map[string]float64{"ended": 2}, gauges(t, reg, series.BacklogName), compare.Options); diff != "" {
		t.Errorf("the scan backlog series (-want +got):\n%s", diff)
	}
}

// D4's part of VERIFICATIONS' rows for the credentials a token source is built from and for an
// account's own client. A tick builds each served account's token source from the Gmail client the
// account connects through, its identifier and its secret each in its own place, and the account's
// own stored refresh token. The two accounts connect through two clients of the one provider, so
// each source carries its own account's client and never the other's (ADR-0085, ADR-0106).
func TestEachTickBuildsItsSourcesFromTheClientAndTheAccountsToken(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	client(t, conn, public)
	must(t, conn, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('employer', $1, 'employer-id', $2)",
		gmailProvider, sealed(t, public, "employer-secret", seal.ClientSecret("employer")))
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	must(t, conn, "UPDATE accounts SET oauth_client = 'employer' WHERE account_id = 'work'")
	var built []gmail.Credentials
	source := func(c session.Credentials) *gmail.TokenSource {
		built = append(built, gmail.NewCredentials(c.ClientID, c.ClientSecret, c.Credential))
		return gmailSource(unreachable(t))(c)
	}
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler),
		fakes(source, map[string]*fake.Fake{"personal": mailbox(t, "personal"), "work": mailbox(t, "work")}), j)

	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}

	want := []gmail.Credentials{
		{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "personal-token"},
		{ClientID: "employer-id", ClientSecret: "employer-secret", RefreshToken: "work-token"},
	}
	if diff := cmp.Diff(want, built, compare.Options); diff != "" {
		t.Errorf("the credentials each tick's sources were built from (-want +got):\n%s", diff)
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

// D4's part of VERIFICATIONS' row for scanning fixtures and then searching for fixture body text,
// delta sync's logs. Ticks over fixtures whose every body carries a body marker add them, a gap is
// recovered from, and once backfill's second pass has ended a tick scans what waits, through a
// restricted sender's skip, a code flagged in a body, a clean HTML-only body and a body the conversion
// refuses, with a logger that records everything delta sync logs. A search of the account's rows in
// every table with an account column, each row read whole, and of the logs finds no body marker. The
// same search, handed a row and a log line each planted with one, finds both, so its silence is
// evidence (ADR-0009, ADR-0044).
func TestNoBodyTextReachesTheIndexOrTheLogs(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	must(t, conn, "INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by) VALUES (NULL, 'rule.bank', 'restricted', '{bank.example}', 'operator', 'test')")
	t.Cleanup(func() {
		if _, err := conn.Exec(context.Background(), "DELETE FROM policy_rules WHERE rule_id = 'rule.bank'"); err != nil {
			t.Error(err)
		}
	})
	refused := "<p>" + marker.Body("refused") + strings.Repeat(" padding", 512*1024/8) + "</p>"
	bodies := []struct{ from, text, html string }{
		{"alerts@bank.example", marker.Body("bank") + " Your verification code is 419283.", ""},
		{"orders@shop.example", marker.Body("shoptext") + " Your verification code is 419283.", "<p>" + marker.Body("shophtml") + "</p>"},
		{"hello@shop.example", "", "<p>" + marker.Body("welcome") + " Welcome aboard.</p>"},
		{"big@news.example", "", refused},
	}
	f := mailbox(t, "personal")
	var log logBuffer
	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), log.logger(), fakes(gmailSource(http.DefaultClient), map[string]*fake.Fake{"personal": f}), j)
	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}
	for i, b := range bodies {
		if err := f.Deliver(fake.Message{
			Metadata: mail.MessageMetadata{
				ID: fmt.Sprintf("m%d", i), ThreadID: fmt.Sprintf("t%d", i), From: mail.Address{Email: b.from},
				Subject: marker.Field("subject"), Date: mail.UnixMilli(time.Now().UnixMilli()),
			},
			Body: mail.MessageBody{Text: b.text, HTML: b.html},
		}); err != nil {
			t.Fatal(err)
		}
	}
	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}
	f.ExpireCursors()
	must(t, conn, "UPDATE account_state SET sync_cursor_at = now() - interval '1 hour', backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'personal'")

	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}

	var scanned, skipped, failed int
	err := conn.QueryRow(t.Context(), `SELECT count(*) FILTER (WHERE scan_state = 'scanned' AND content_flags <> '{}'),
		count(*) FILTER (WHERE scan_state = 'skipped_restricted'),
		(SELECT count(*) FROM job_run_failures WHERE account_id = 'personal' AND item_kind = 'message')
		FROM messages WHERE account_id = 'personal'`).Scan(&scanned, &skipped, &failed)
	if err != nil || scanned != 1 || skipped != 1 || failed != 1 {
		t.Fatalf("the ticks flagged %d bodies, skipped %d and failed %d (%v), want the shop's code flagged, the bank skipped and the large body refused", scanned, skipped, failed, err)
	}
	if n := len(slices.DeleteFunc(log.records(t), func(r record) bool { return r.Msg != "the account's tick ended" })); n != 3 {
		t.Fatalf("the log holds %d ended ticks, want 3", n)
	}
	if found := bodyMarkers(t, conn, "personal", log.String()); len(found) != 0 {
		t.Errorf("fixture body text reached the index or the logs:\n%s", strings.Join(found, "\n"))
	}
	must(t, conn, "INSERT INTO job_run_events (account_id, run_id, kind, detail) VALUES ('personal', 'run1', 'retry', $1)", marker.Body("planted"))
	planted := log.String() + `{"msg":"` + marker.Body("plantedlog") + `"}` + "\n"
	if found := bodyMarkers(t, conn, "personal", planted); len(found) != 2 {
		t.Errorf("the search found %d of the two planted body markers:\n%s", len(found), strings.Join(found, "\n"))
	}
}

// accountsIn returns the accounts the series name carries on reg, by the account label.
func accountsIn(t *testing.T, reg *prometheus.Registry, name string) []string {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var out []string
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "account" {
					out = append(out, l.GetValue())
				}
			}
		}
	}
	return out
}

// An account's tick series are removed once its job is dropped, so the worker, which outlives the
// account's ticks, serves no series for an account it no longer ticks (ADR-0076, ADR-0119).
func TestADroppedAccountsTickSeriesGoWithItsJob(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	reg := prometheus.NewRegistry()
	j := newJobs()
	d := kind(t, syncPool(t), ring, reg, slog.New(slog.DiscardHandler),
		fakes(gmailSource(http.DefaultClient), map[string]*fake.Fake{"personal": mailbox(t, "personal")}), j)
	if err := cycle(t, d, j); err != nil {
		t.Fatal(err)
	}
	names := []string{"mediated_mailbox_sync_cursor_gaps_total", "mediated_mailbox_unclassified_senders_total"}
	for _, name := range names {
		if got := accountsIn(t, reg, name); !slices.Equal(got, []string{"personal"}) {
			t.Fatalf("%s names %v after the account's tick, want personal", name, got)
		}
	}

	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	for _, name := range names {
		if got := accountsIn(t, reg, name); len(got) != 0 {
			t.Errorf("%s names %v once the account's job was dropped, want none", name, got)
		}
	}
}

// VERIFICATIONS' row for a job's latest success read from recorded state, delta sync's part. Each
// account's job is ensured with its latest success as its ticks record it, the end of its latest tick
// that succeeded, so a worker that restarts without ticking leaves it ageing. An account whose ticks
// all failed counts from its earliest tick's start, and one with no tick from its ensure (ADR-0119).
func TestEachTickJobStartsFromItsRecordedLatestSuccess(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	client(t, conn, public)
	for _, id := range []string{"ticking", "failing", "new"} {
		account(t, conn, id, gmailProvider, sealed(t, public, id+"-token", seal.AccountCredential(id)), false)
	}
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	run := func(account, id, state string, started, finished time.Time) {
		must(t, conn, "INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at) VALUES ($1, $2, 'sync', 'tick', $3, $4, $5)",
			account, id, state, started, finished)
	}
	run("ticking", "t1", "succeeded", t0, t0.Add(time.Minute))
	run("ticking", "t2", "succeeded", t0.Add(5*time.Minute), t0.Add(6*time.Minute))
	run("ticking", "t3", "failed", t0.Add(10*time.Minute), t0.Add(11*time.Minute))
	run("failing", "f1", "failed", t0.Add(time.Hour), t0.Add(time.Hour+time.Minute))
	run("failing", "f2", "failed", t0.Add(2*time.Hour), t0.Add(2*time.Hour+time.Minute))

	j := newJobs()
	d := kind(t, syncPool(t), ring, prometheus.NewRegistry(), slog.New(slog.DiscardHandler),
		fakes(gmailSource(http.DefaultClient), map[string]*fake.Fake{}), j)
	if err := d.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	got := map[string]time.Time{}
	for key, job := range j.held {
		got[key.ID] = job.LastSuccess.UTC()
	}
	want := map[string]time.Time{"ticking": t0.Add(6 * time.Minute), "failing": t0.Add(time.Hour), "new": {}}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the latest success each job starts from (-want +got):\n%s", diff)
	}
}
