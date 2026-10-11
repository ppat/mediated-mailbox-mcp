//go:build integration

package backfill

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"maps"
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
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// The role backfill connects as, so row-level security and its grants apply as in production.
const role = "mediated_mailbox_backfill"

// backfillPool returns a pool connecting as role.
func backfillPool(t *testing.T) *pgxpool.Pool {
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

// revoke revokes a grant of backfill's role for the rest of the test, and grants it again when the
// test ends, so the tests after it see the role as the migration chain made it.
func revoke(t *testing.T, conn *pgx.Conn, privilege string) {
	t.Helper()
	must(t, conn, "REVOKE "+privilege+" FROM "+role)
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
		if _, err := c.Exec(ctx, "GRANT "+privilege+" TO "+role); err != nil {
			t.Error(err)
		}
	})
}

// reset empties the tables backfill reads and writes. Every test of the package shares one database.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	for _, table := range []string{
		"masking_events", "scan_gate_decisions", "job_run_events", "job_run_failures", "job_runs", "messages", "senders",
		"rate_grants", "rate_state",
	} {
		must(t, conn, "DELETE FROM "+table)
	}
	must(t, conn, "DELETE FROM account_state")
	must(t, conn, "DELETE FROM accounts")
	must(t, conn, "DELETE FROM oauth_clients")
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
	slices.SortFunc(out, func(a, b record) int { return strings.Compare(a.Msg+a.Account, b.Msg+b.Account) })
	return out
}

// jobs stands in for the worker's scheduler. It holds the account jobs the kind's reload ensured and
// records every job it was asked to add or drop, so a test runs each job itself, as the scheduler
// would ask it.
type jobs struct {
	held             map[schedule.Key]schedule.Job
	ensured, removed []string
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

// kind returns backfill's job kind over pool and ring, scanning with the default scanner, with its
// account sessions built through connect and its account jobs added to j.
func kind(t *testing.T, pool *pgxpool.Pool, ring *open.Keyring, logger *slog.Logger, connect session.Connector, j *jobs) *Backfill {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	b, err := build(Config{
		Pool: pool, Keys: ring, Scanner: s, Registry: prometheus.NewRegistry(), Logger: logger, Jobs: j,
		Concurrency: 1, ReloadInterval: time.Minute,
	}, map[string]session.Connector{gmailProvider: connect})
	if err != nil {
		t.Fatal(err)
	}
	return b
}

// runAll reloads the kind, failing the test when the reload fails, then runs each account job it holds
// once, in the order of the accounts, as the scheduler asks a job at once when it is ensured, and
// returns the runs' failures joined, each naming its account.
func runAll(t *testing.T, b *Backfill, j *jobs) error {
	t.Helper()
	if err := b.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	var failures []error
	for _, key := range slices.SortedFunc(maps.Keys(j.held), func(a, b schedule.Key) int { return strings.Compare(a.ID, b.ID) }) {
		if err := j.held[key].Run(t.Context(), schedule.Ask{At: time.Now()}); err != nil {
			failures = append(failures, fmt.Errorf("account %s: %w", key.ID, err))
		}
	}
	return errors.Join(failures...)
}

// D1's parts of VERIFICATIONS' rows for loading the account snapshot and for a credential supplied
// outside the account's state row. The kind's reload takes its accounts, the OAuth clients and the
// credentials from the database. It loads the policy of every listed account, and adds a job only
// for a connected account whose provider it has an adapter and a client for, whatever the environment
// and a mounted file hold, logging every other one (ADR-0080, ADR-0090).
func TestARunTakesItsAccountsFromTheDatabase(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "no-state", gmailProvider, nil, true)
	account(t, conn, "no-credential", gmailProvider, nil, false)
	account(t, conn, "work", "fastmail", sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	client(t, conn, public)
	t.Setenv("GMAIL_REFRESH_TOKEN", "environment-token")
	mounted := filepath.Join(t.TempDir(), "refresh_token")
	if err := os.WriteFile(mounted, []byte("mounted-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	stored := storedCredential(t, conn, "personal")
	var log logBuffer
	j := newJobs()
	b := kind(t, backfillPool(t), ring, log.logger(), quiet, j)

	if err := runAll(t, b, j); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"personal"}, j.ensured, compare.Options); diff != "" {
		t.Errorf("the account jobs added (-want +got):\n%s", diff)
	}
	var warned []record
	for _, r := range log.records(t) {
		if r.Level == "WARN" {
			warned = append(warned, r)
		}
	}
	want := []record{
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "no-credential"},
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "no-state"},
		{Level: "WARN", Msg: "there is no adapter for an account's provider, so it is skipped", Account: "work", Provider: "fastmail"},
	}
	if diff := cmp.Diff(want, warned, compare.Options); diff != "" {
		t.Errorf("warnings (-want +got):\n%s", diff)
	}
	for _, id := range []string{"no-credential", "no-state", "personal", "work"} {
		if b.policies == nil || b.policies.Snapshot().For(id).RestrictsAll() {
			t.Errorf("the reload read no rules of listed account %s, so it would restrict every sender once the account is served", id)
		}
	}
	if got := storedCredential(t, conn, "personal"); !bytes.Equal(got, stored) {
		t.Error("the run rewrote a credential the provider never rotated")
	}
}

// An account that connects through no OAuth client is not connected, because backfill tells the loader
// Gmail authenticates through one, so it is logged and gets no job, though its provider has a client,
// and nothing looks for a client the account does not name (ADR-0080, ADR-0106).
func TestAnAccountWithoutAClientIsSkipped(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	must(t, conn, "UPDATE accounts SET oauth_client = NULL WHERE account_id = 'personal'")
	var log logBuffer
	j := newJobs()

	if err := kind(t, backfillPool(t), ring, log.logger(), quiet, j).reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}

	want := []record{
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "personal"},
		{Level: "WARN", Msg: "an account's provider authenticates through an OAuth client and the account has none that opened, so it is not connected", Account: "personal", Provider: gmailProvider},
	}
	if diff := cmp.Diff(want, log.records(t), compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
	if len(j.ensured) != 0 {
		t.Errorf("the reload added the jobs of %v, want none", j.ensured)
	}
}

// A reload whose read of the account snapshot fails returns the failure, keeps the snapshot it had, and
// adds and drops no job, so the accounts already served go on being served (ADR-0090, ADR-0119).
func TestAFailedSnapshotReadKeepsThePreviousSnapshot(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	j := newJobs()
	b := kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), quiet, j)
	if err := b.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	must(t, conn, "UPDATE account_state SET credential = NULL WHERE account_id = 'personal'")
	revoke(t, conn, "SELECT ON accounts")

	err := b.reload(t.Context(), schedule.Ask{})

	if !errors.Is(err, accountload.ErrUntrustedRead) {
		t.Fatalf("the reload returned %v, want a failed snapshot read", err)
	}
	if diff := cmp.Diff([]string{"personal"}, j.ensured, compare.Options); diff != "" {
		t.Errorf("the account jobs added (-want +got):\n%s", diff)
	}
	if len(j.removed) != 0 {
		t.Errorf("the failed reload dropped the jobs of %v, want none", j.removed)
	}
	if a, ok := b.loader.Snapshot().Account("personal"); !ok || string(a.Credential()) != "personal-token" {
		t.Errorf("after the failed read the snapshot holds personal %v, want the previous snapshot's credential", ok)
	}
}

// connector returns the connector a test run builds each account's source with source and each port
// as a provider fake holding nothing.
func connector[S session.Source](source func(session.Credentials) S) session.Connector {
	return session.Connect(source, func(account string, _ S) (mail.Port[context.Context], error) {
		return fake.New(fake.Config{Account: account})
	})
}

// refusing returns the connector whose ports fail every call with failed, over sources built with
// source.
func refusing[S session.Source](source func(session.Credentials) S, failed error) session.Connector {
	return session.Connect(source, func(account string, _ S) (mail.Port[context.Context], error) {
		f, err := fake.New(fake.Config{Account: account})
		if err != nil {
			return nil, err
		}
		return fake.Throttle(f, func(fake.Call) error { return failed }, time.Now), nil
	})
}

// quiet is the connector of a run whose ports hold nothing and whose sources make no attempt.
var quiet = connector(gmailSource(http.DefaultClient))

// rotating returns a source builder that builds the Gmail token source the run builds, holding the
// token rotations maps the account's credential to when it maps it, as a source Google rotated would
// hold it, and records each credential it was built from in built when built is not nil. Google's
// rotation itself is out of reach without a stand-in for its endpoint (ADR-0043). A source built this
// way still names the stored credential it was built from, so the adoption stamp it hands over is the
// one that credential came with.
func rotating(rotations map[string]string, built *[]gmail.Credentials) func(session.Credentials) *gmail.TokenSource {
	return func(c session.Credentials) *gmail.TokenSource {
		if built != nil {
			*built = append(*built, gmail.NewCredentials(c.ClientID, c.ClientSecret, c.Credential))
		}
		if to, ok := rotations[string(c.Credential)]; ok {
			c.Credential = []byte(to)
		}
		return gmailSource(http.DefaultClient)(c)
	}
}

// reporting is a source holding the credential it was built from and reporting attempt as its latest
// authentication attempt, for the tests of what a unit of work records.
type reporting struct {
	credential []byte
	attempt    mail.AuthAttempt
}

func (r reporting) Credential() []byte            { return r.credential }
func (r reporting) LastAttempt() mail.AuthAttempt { return r.attempt }

// reporter returns a source builder whose sources report attempt.
func reporter(attempt mail.AuthAttempt) func(session.Credentials) reporting {
	return func(c session.Credentials) reporting { return reporting{credential: c.Credential, attempt: attempt} }
}

// counting is a source holding the credential it was built from that counts the units of work ended
// over it, since the end of each reads its latest attempt once. It reports first at the first end and
// no attempt at every later one.
type counting struct {
	credential []byte
	ends       *int
	first      mail.AuthAttempt
}

func (c counting) Credential() []byte { return c.credential }

func (c counting) LastAttempt() mail.AuthAttempt {
	*c.ends++
	if *c.ends == 1 {
		return c.first
	}
	return mail.AuthAttempt{}
}

// counter returns a source builder whose sources count into ends and report first at the first end.
func counter(ends *int, first mail.AuthAttempt) func(session.Credentials) counting {
	return func(c session.Credentials) counting {
		return counting{credential: c.Credential, ends: ends, first: first}
	}
}

// opened opens the account's session over loader's snapshot, building its source with source, and
// recording through backfill's role.
func opened[S session.Source](t *testing.T, loader *accountload.Loader, account string, source func(session.Credentials) S) *session.Session {
	t.Helper()
	h := session.New(session.Config{
		Loader: loader, DB: backfillPool(t), Logger: slog.New(slog.DiscardHandler),
		Connectors: map[string]session.Connector{gmailProvider: connector(source)},
	})
	s, err := h.Open(loader.Snapshot(), account)
	if err != nil {
		t.Fatal(err)
	}
	return s
}

// unitOver returns the unit of work of an account the kind serves, over loader's snapshot, whose
// sessions build their sources with source and are held across units as the kind holds them.
func unitOver[S session.Source](t *testing.T, loader *accountload.Loader, account string, source func(session.Credentials) S) *unit {
	t.Helper()
	return &unit{
		sessions: session.New(session.Config{
			Loader: loader, DB: backfillPool(t), Logger: slog.New(slog.DiscardHandler),
			Connectors: map[string]session.Connector{gmailProvider: connector(source)}, Hold: true,
		}),
		loader: loader, account: account,
	}
}

// connectedUnit connects the account, whose row the test wrote, with its own token through the
// household client, which it writes or seals again, and returns the account's unit of work over a
// loader that has loaded it, its sessions' sources built with source.
func connectedUnit[S session.Source](t *testing.T, conn *pgx.Conn, account string, source func(session.Credentials) S) *unit {
	t.Helper()
	ring, public := keys(t)
	must(t, conn, "UPDATE account_state SET credential = $1 WHERE account_id = $2", sealed(t, public, account+"-token", seal.AccountCredential(account)), account)
	must(t, conn, `INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ($1, $2, 'client-id', $3)
		ON CONFLICT (name) DO UPDATE SET client_secret = EXCLUDED.client_secret`,
		household, gmailProvider, sealed(t, public, "client-secret", seal.ClientSecret(household)))
	must(t, conn, "UPDATE accounts SET oauth_client = $1 WHERE account_id = $2", household, account)
	return unitOver(t, loaded(t, ring, slog.New(slog.DiscardHandler)), account, source)
}

// idleUnit is connectedUnit with sources that make no attempt and never rotate, so ending a unit of
// work writes nothing.
func idleUnit(t *testing.T, conn *pgx.Conn, account string) *unit {
	t.Helper()
	return connectedUnit(t, conn, account, gmailSource(http.DefaultClient))
}

// loaded returns a loader that has loaded the snapshot.
func loaded(t *testing.T, ring *open.Keyring, log *slog.Logger) *accountload.Loader {
	t.Helper()
	l := accountload.New(backfillPool(t), ring, log, []string{gmailProvider})
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	return l
}

// The credentials each served account's token source is built from are the Gmail client the account
// connects through, its identifier and its secret each in its own place, and the account's own stored
// refresh token. The two accounts connect through two clients of the one provider, so each source
// carries its own account's client and never the other's (ADR-0085, ADR-0106).
func TestTheCredentialsAreTheClientAndTheAccountsToken(t *testing.T) {
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
	j := newJobs()

	if err := runAll(t, kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), connector(rotating(nil, &built)), j), j); err != nil {
		t.Fatal(err)
	}

	want := []gmail.Credentials{
		{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "personal-token"},
		{ClientID: "employer-id", ClientSecret: "employer-secret", RefreshToken: "work-token"},
	}
	if diff := cmp.Diff(want, built, compare.Options); diff != "" {
		t.Errorf("the credentials of the accounts' sources, in the order of the accounts (-want +got):\n%s", diff)
	}
}

// D1's part of VERIFICATIONS' row for forcing a rotation and restarting, driven through each account's
// job. The job builds the account's token source holding the account's stored refresh token, and the
// end of a unit of work over the account's session, one whose page failed included, hands the source's
// current token over, so a process started afterwards reads each rotated one (ADR-0082). Google's
// rotation itself is out of reach without a stand-in for its endpoint (ADR-0043), so each source is
// built holding the token it would hold after one.
func TestARunHandsEachAccountOverWhenItsUnitEnds(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	client(t, conn, public)
	failed := errors.New("the unit of work failed")
	var built []gmail.Credentials
	rotations := map[string]string{"personal-token": "personal-rotated", "work-token": "work-rotated"}
	j := newJobs()

	err := runAll(t, kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), refusing(rotating(rotations, &built), failed), j), j)

	if !errors.Is(err, failed) {
		t.Errorf("the runs returned %v, want the unit of work's failure", err)
	}
	held := map[string]bool{}
	for _, c := range built {
		held[c.RefreshToken] = true
	}
	if diff := cmp.Diff(map[string]bool{"personal-token": true, "work-token": true}, held, compare.Options); diff != "" {
		t.Errorf("the tokens the sources were built from (-want +got):\n%s", diff)
	}
	restarted := map[string]string{}
	for _, a := range loaded(t, ring, slog.New(slog.DiscardHandler)).Snapshot().Accounts() {
		restarted[a.ID()] = string(a.Credential())
	}
	if diff := cmp.Diff(map[string]string{"personal": "personal-rotated", "work": "work-rotated"}, restarted, compare.Options); diff != "" {
		t.Errorf("the credentials after a restart (-want +got):\n%s", diff)
	}
}

// At the end of a unit of work the account's session hands its token source's current refresh token
// to the loader. A rotated one is written back, so a process started afterwards reads it, and an
// unchanged one writes nothing (ADR-0082). Google's rotation itself is out of reach without a
// stand-in for its endpoint (ADR-0043), so each source is built holding the token it would hold after
// one.
func TestTheSourcesRefreshTokenIsHandedOver(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "rotated", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("rotated")), false)
	account(t, conn, "unchanged", gmailProvider, sealed(t, public, "same-token", seal.AccountCredential("unchanged")), false)
	client(t, conn, public)
	unchanged := storedCredential(t, conn, "unchanged")
	loader := loaded(t, ring, slog.New(slog.DiscardHandler))
	sessions := map[string]*session.Session{
		"rotated":   opened(t, loader, "rotated", rotating(map[string]string{"first-token": "second-token"}, nil)),
		"unchanged": opened(t, loader, "unchanged", rotating(nil, nil)),
	}

	for _, s := range sessions {
		if err := errors.Join(s.End(t.Context())); err != nil {
			t.Fatal(err)
		}
	}

	restarted := loaded(t, ring, slog.New(slog.DiscardHandler)).Snapshot()
	a, _ := restarted.Account("rotated")
	if got := string(a.Credential()); got != "second-token" {
		t.Errorf("after a restart the rotated account holds %q, want second-token", got)
	}
	if got := storedCredential(t, conn, "unchanged"); !bytes.Equal(got, unchanged) {
		t.Error("an unchanged refresh token was written back")
	}
}

// D1's part of VERIFICATIONS' row for failing the write-back of a rotated credential. A write-back
// that fails is logged by the loader and the end of the unit of work returns an error naming the
// account, and an unchanged one still hands over, so the failure is loud (ADR-0082).
func TestAFailedWriteBackEndsTheRunInError(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	client(t, conn, public)
	var log logBuffer
	loader := loaded(t, ring, log.logger())
	revoke(t, conn, "UPDATE (credential) ON account_state")
	personal := unitOver(t, loader, "personal", rotating(map[string]string{"first-token": "second-token"}, nil))
	work := unitOver(t, loader, "work", rotating(nil, nil))
	for _, u := range []*unit{personal, work} {
		if err := u.open(); err != nil {
			t.Fatal(err)
		}
	}

	err := personal.end(t.Context())

	if err == nil || !strings.Contains(err.Error(), "account personal: writing back the rotated credential") {
		t.Fatalf("the end of the unit of work returned %v, want the failed write-back", err)
	}
	if err := work.end(t.Context()); err != nil {
		t.Errorf("handing over an unchanged token returned %v", err)
	}
	var failures []record
	for _, r := range log.records(t) {
		if r.Level == "ERROR" {
			failures = append(failures, r)
		}
	}
	want := []record{{Level: "ERROR", Msg: "a rotated credential was not written back, so a restart before a later write-back lands loses the account's access", Account: "personal"}}
	if diff := cmp.Diff(want, failures, compare.Options); diff != "" {
		t.Errorf("error records (-want +got):\n%s", diff)
	}
}

// firstPassDeps returns the dependencies of a pass over the account through pool, enumerating a
// provider fake holding messages in pages of two, one of them from a sender without a domain.
func firstPassDeps(t *testing.T, pool *pgxpool.Pool, account string) pass1.Deps {
	t.Helper()
	var messages []fake.Message
	for i, from := range []string{"a@news.example", "b@news.example", "no-domain", "c@shop.example", "d@news.example"} {
		messages = append(messages, fake.Message{Metadata: mail.MessageMetadata{
			ID: fmt.Sprintf("m%d", i), ThreadID: fmt.Sprintf("t%d", i), From: mail.Address{Email: from},
			Subject: marker.Field("subject"), Date: mail.UnixMilli(1_700_000_000_000 + i),
		}})
	}
	f, err := fake.New(fake.Config{Account: account, PageSize: 2}, messages...)
	if err != nil {
		t.Fatal(err)
	}
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	none, err := policy.Load(nil)
	if err != nil {
		t.Fatal(err)
	}
	n := 0
	return pass1.Deps{
		Store: pass1.NewPostgres(pool), Fetch: f.EnumerateAll, Metadata: f.GetMessageMetadata, PerCall: 3,
		Policy: fixed(none.For(account)), Scanner: s,
		Lookups: classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
		RunID:   func() string { n++; return fmt.Sprintf("run%d", n) },
		Now:     time.Now,
	}
}

// Every page of the first pass is a unit of work. The account's session is opened before each page
// and its unit ended after it, handing the token over, so a rotation reaches the database a page after
// it happens rather than at the end of the run (ADR-0082). The messages whose sender the classifier
// could not classify are counted on the account's series as each page is made durable (O2), and so are
// the messages each page added to the index, each once (ADR-0125).
func TestEachPageIsAUnitOfWork(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	registry := prometheus.NewRegistry()
	messages, err := series.NewMessages(registry)
	if err != nil {
		t.Fatal(err)
	}
	metrics, err := pass1.NewMetrics(registry, messages)
	if err != nil {
		t.Fatal(err)
	}
	var ends int
	u := connectedUnit(t, conn, "personal", counter(&ends, mail.AuthAttempt{}))

	if err := passAccount(t.Context(), firstPassDeps(t, pool, "personal"), "personal", metrics, u, schedule.Ask{}, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}

	if ends != 3 {
		t.Errorf("the units of work ended %d times, want once after each of the three pages", ends)
	}
	if got := counterValue(t, registry, series.UnclassifiedName, "personal"); got != 1 {
		t.Errorf("the unclassified senders series reads %v, want 1", got)
	}
	var stored float64
	if err := conn.QueryRow(t.Context(), "SELECT count(*) FROM messages WHERE account_id = 'personal'").Scan(&stored); err != nil {
		t.Fatal(err)
	}
	if got := stageValue(t, registry, "personal", series.StageIndexed); got != stored || stored == 0 {
		t.Errorf("the messages series counts %v indexed, want the %v messages the index holds", got, stored)
	}
}

// stageValue returns the account's count of messages made durable at stage on registry, or zero when
// none was counted.
func stageValue(t *testing.T, registry prometheus.Gatherer, account, stage string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != series.MessagesName {
			continue
		}
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			if labels["account"] == account && labels["stage"] == stage {
				return m.GetCounter().GetValue()
			}
		}
	}
	return 0
}

// stepCount returns how many times the body processing series on registry observed step.
func stepCount(t *testing.T, registry prometheus.Gatherer, step string) uint64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != series.ProcessingName {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "step" && l.GetValue() == step {
					return m.GetHistogram().GetSampleCount()
				}
			}
		}
	}
	return 0
}

// A hand-over that fails is returned once the pass ends, and the pass still ends, since the loader
// keeps the rotated token in memory and a later hand-over may land it (ADR-0082). The first unit's end
// fails at recording the attempt its source reports, and the units after it report none.
func TestAFailedHandOverFailsThePassAtItsEnd(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	metrics, err := pass1.NewMetrics(prometheus.NewRegistry(), nil)
	if err != nil {
		t.Fatal(err)
	}
	var ends int
	u := connectedUnit(t, conn, "personal", counter(&ends, mail.AuthAttempt{At: 1_790_000_000_001, Outcome: mail.AuthSucceeded}))
	revoke(t, conn, "UPDATE (last_auth_at, last_auth_outcome) ON account_state")

	err = passAccount(t.Context(), firstPassDeps(t, backfillPool(t), "personal"), "personal", metrics, u, schedule.Ask{}, slog.New(slog.DiscardHandler))

	if err == nil || !strings.Contains(err.Error(), "account personal: recording the last authentication attempt") {
		t.Errorf("passAccount returned %v, want the failed hand-over", err)
	}
	var ended bool
	if err := conn.QueryRow(t.Context(), "SELECT backfill_pass1_complete FROM account_state WHERE account_id = 'personal'").Scan(&ended); err != nil || !ended {
		t.Errorf("the pass did not end after a failed hand-over (%v)", err)
	}
	if ends != 3 {
		t.Errorf("the units of work ended %d times, want three, the pass going on after the failed one", ends)
	}
}

// counterValue returns the value of the series name carries for account, failing when it has none.
func counterValue(t *testing.T, registry prometheus.Gatherer, name, account string) float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() != name {
			continue
		}
		for _, m := range f.GetMetric() {
			for _, l := range m.GetLabel() {
				if l.GetName() == "account" && l.GetValue() == account {
					if m.GetCounter() != nil {
						return m.GetCounter().GetValue()
					}
					return m.GetGauge().GetValue()
				}
			}
		}
	}
	t.Fatalf("no series %s for account %s", name, account)
	return 0
}

// The limiter a run builds spends each account under the lower target its state row sets, and every
// other account under the default target (ADR-0024).
func TestTheLimiterSpendsUnderTheStoredTarget(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "lowered", gmailProvider, nil, false)
	account(t, conn, "default", gmailProvider, nil, false)
	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.2 WHERE account_id = 'lowered'")
	pool := backfillPool(t)
	limiter, err := newLimiter(t.Context(), pool, []string{"default", "lowered"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, a := range []string{"default", "lowered"} {
		if _, err := limiter.Acquire(t.Context(), a, ratecore.Batch, 1); err != nil {
			t.Fatal(err)
		}
	}
	got := map[string]float64{}
	for _, a := range []string{"default", "lowered"} {
		var target float64
		if err := conn.QueryRow(t.Context(), "SELECT target_rate FROM rate_state WHERE account_id = $1", a).Scan(&target); err != nil {
			t.Fatal(err)
		}
		got[a] = target
	}
	if diff := cmp.Diff(map[string]float64{"default": 50, "lowered": 20}, got, compare.Options); diff != "" {
		t.Errorf("the targets the limiter spends under, in Gmail's units a second (-want +got):\n%s", diff)
	}
}

// A stored target above half the ceiling refuses the limiter, naming the account, so no run spends
// above the target an operator may only lower (ADR-0024).
func TestAStoredTargetAboveHalfTheCeilingStopsTheRun(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "raised", gmailProvider, nil, false)
	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.6 WHERE account_id = 'raised'")
	_, err := newLimiter(t.Context(), backfillPool(t), []string{"raised"}, nil)
	if !errors.Is(err, ratecore.ErrTargetOutOfRange) || !strings.Contains(err.Error(), `"raised"`) {
		t.Errorf("newLimiter returned %v, want the target refused for the account", err)
	}
}

// A page that fails is still a unit of work, so the account's unit is ended after it too, and a
// rotation made before the failure reaches the database (ADR-0082).
func TestAFailedPageIsStillHandedOver(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	metrics, err := pass1.NewMetrics(prometheus.NewRegistry(), nil)
	if err != nil {
		t.Fatal(err)
	}
	deps := firstPassDeps(t, backfillPool(t), "personal")
	deps.Fetch = func(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return mail.Page[mail.MessageMetadata]{}, mail.ErrAuthentication
	}
	var ends int
	u := connectedUnit(t, conn, "personal", counter(&ends, mail.AuthAttempt{}))

	err = passAccount(t.Context(), deps, "personal", metrics, u, schedule.Ask{}, slog.New(slog.DiscardHandler))

	if !errors.Is(err, mail.ErrAuthentication) {
		t.Errorf("passAccount returned %v, want the refused credential", err)
	}
	if ends != 1 {
		t.Errorf("the units of work ended %d times after the failed page, want once", ends)
	}
}

// unreachable returns a client whose requests fail before they leave the machine, sent through a
// proxy on a port nothing listens on, so a token source refreshing through it makes a real attempt
// that gets no answer, with no stand-in for Google's endpoint (ADR-0043).
func unreachable() *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "127.0.0.1:9"})}}
}

// attempting returns a source builder whose sources are the Gmail token sources the run builds, each
// holding a failed authentication attempt, made through unreachable.
func attempting(t *testing.T) func(session.Credentials) *gmail.TokenSource {
	return func(c session.Credentials) *gmail.TokenSource {
		source := gmail.NewTokenSource(unreachable(), gmail.NewCredentials(c.ClientID, c.ClientSecret, c.Credential))
		if _, err := source.AccessToken(t.Context()); err == nil {
			t.Fatal("a refresh through an unreachable proxy returned an access token")
		}
		if source.LastAttempt().Outcome != mail.AuthFailed {
			t.Fatalf("the source holds %+v, want a failed attempt", source.LastAttempt())
		}
		return source
	}
}

// lastAuthentication reads the account's last authentication through the statement the system
// status operation reads it with, as backfill's role (ADR-0034).
func lastAuthentication(t *testing.T, pool *pgxpool.Pool, account string) (*time.Time, *string) {
	t.Helper()
	var at *time.Time
	var outcome *string
	err := tx.Run(t.Context(), pool, account, func(q pgx.Tx) error {
		progress, err := accountstate.New(q).AccountProgress(t.Context(), account)
		if err != nil {
			return err
		}
		if progress.LastAuthAt.Valid {
			at = &progress.LastAuthAt.Time
		}
		if progress.LastAuthOutcome.Valid {
			outcome = &progress.LastAuthOutcome.String
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return at, outcome
}

// D3's part of VERIFICATIONS' row for failing a unit of work after an authentication attempt. The end
// of a unit of work over an account's session records the source's latest attempt, a unit whose page
// failed included, and the statement the system status reads returns it. An account whose source made
// no attempt records nothing (ADR-0097, ADR-0034).
func TestARunRecordsEachAccountsAttemptWhenItsUnitEnds(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	client(t, conn, public)
	failed := errors.New("the unit of work failed")
	var attempt mail.AuthAttempt
	attempted := attempting(t)
	source := func(c session.Credentials) *gmail.TokenSource {
		if string(c.Credential) != "personal-token" {
			return gmailSource(http.DefaultClient)(c)
		}
		s := attempted(c)
		attempt = s.LastAttempt()
		return s
	}
	pool := backfillPool(t)
	j := newJobs()

	err := runAll(t, kind(t, pool, ring, slog.New(slog.DiscardHandler), refusing(source, failed), j), j)

	if !errors.Is(err, failed) {
		t.Errorf("the runs returned %v, want the unit of work's failure", err)
	}
	at, outcome := lastAuthentication(t, pool, "personal")
	if at == nil || outcome == nil || !at.Equal(time.UnixMilli(int64(attempt.At))) || *outcome != "failed" {
		t.Errorf("the personal account's last authentication reads %v %v, want failed at %d", at, outcome, attempt.At)
	}
	if at, outcome := lastAuthentication(t, pool, "work"); at != nil || outcome != nil {
		t.Errorf("the work account, whose source made no attempt, reads %v %v, want nothing recorded", at, outcome)
	}
}

// connectedAccounts writes each account connected with its own token through the household client,
// and returns a loader that has loaded them.
func connectedAccounts(t *testing.T, ids ...string) *accountload.Loader {
	t.Helper()
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	for _, id := range ids {
		account(t, conn, id, gmailProvider, sealed(t, public, id+"-token", seal.AccountCredential(id)), false)
	}
	client(t, conn, public)
	return loaded(t, ring, slog.New(slog.DiscardHandler))
}

// The recorded attempt is the latest one. An attempt older than the one the row holds changes
// nothing, and one later than it, or than a row that holds none, replaces it, so a deployable whose
// unit of work ended late never puts an older outcome back (ADR-0097).
func TestOnlyALaterAttemptIsRecorded(t *testing.T) {
	ids := []string{"later-stored", "earlier-stored", "none-stored"}
	loader := connectedAccounts(t, ids...)
	conn := superuser(t)
	attempt := attempting(t)(session.Credentials{ClientID: "client-id", ClientSecret: []byte("client-secret"), Credential: []byte("refresh-token")}).LastAttempt()
	made := time.UnixMilli(int64(attempt.At)).UTC()
	must(t, conn, "UPDATE account_state SET last_auth_at = $1, last_auth_outcome = 'succeeded' WHERE account_id = 'later-stored'", made.Add(time.Millisecond))
	must(t, conn, "UPDATE account_state SET last_auth_at = $1, last_auth_outcome = 'refused' WHERE account_id = 'earlier-stored'", made.Add(-time.Millisecond))
	pool := backfillPool(t)

	for _, id := range ids {
		if err := errors.Join(opened(t, loader, id, reporter(attempt)).End(t.Context())); err != nil {
			t.Fatalf("recording for %s: %v", id, err)
		}
	}

	got := map[string]string{}
	for _, id := range ids {
		at, outcome := lastAuthentication(t, pool, id)
		if at == nil || outcome == nil {
			t.Fatalf("%s holds no attempt", id)
		}
		got[id] = fmt.Sprintf("%s at %+dms", *outcome, at.Sub(made).Milliseconds())
	}
	want := map[string]string{"later-stored": "succeeded at +1ms", "earlier-stored": "failed at +0ms", "none-stored": "failed at +0ms"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the recorded attempts (-want +got):\n%s", diff)
	}
}

// Each outcome is recorded as the adapter reported it, with the instant the attempt started, and
// the statement the system status reads returns both, so an account Google refused never reads as
// one whose attempt failed (ADR-0097, ADR-0034).
func TestEachOutcomeIsRecordedAsReported(t *testing.T) {
	reported := map[string]mail.AuthAttempt{
		"succeeded": {At: 1_790_000_000_001, Outcome: mail.AuthSucceeded},
		"refused":   {At: 1_790_000_000_002, Outcome: mail.AuthRefused},
		"failed":    {At: 1_790_000_000_003, Outcome: mail.AuthFailed},
	}
	loader := connectedAccounts(t, slices.Sorted(maps.Keys(reported))...)
	pool := backfillPool(t)

	for id, attempt := range reported {
		if err := errors.Join(opened(t, loader, id, reporter(attempt)).End(t.Context())); err != nil {
			t.Fatalf("recording for %s: %v", id, err)
		}
	}

	got := map[string]string{}
	for id := range reported {
		at, outcome := lastAuthentication(t, pool, id)
		if at == nil || outcome == nil {
			t.Fatalf("%s holds no attempt", id)
		}
		got[id] = fmt.Sprintf("%s at %d", *outcome, at.UnixMilli())
	}
	want := map[string]string{
		"succeeded": "succeeded at 1790000000001",
		"refused":   "refused at 1790000000002",
		"failed":    "failed at 1790000000003",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the recorded attempts (-want +got):\n%s", diff)
	}
}

// A recording that fails is returned naming the account, so the pass ends in an error, and the next
// unit of work records the attempt again (ADR-0097).
func TestAFailedRecordingIsReturned(t *testing.T) {
	loader := connectedAccounts(t, "personal")
	revoke(t, superuser(t), "UPDATE (last_auth_at, last_auth_outcome) ON account_state")

	err := errors.Join(opened(t, loader, "personal", attempting(t)).End(t.Context()))

	if err == nil || !strings.Contains(err.Error(), "account personal: recording the last authentication attempt") {
		t.Errorf("the recording returned %v, want the failed recording", err)
	}
}

// The end of a unit of work the account's job gives its passes returns a recording that fails, so the
// pass, which joins the hand-overs that fail into the error it ends in, ends the job's run in it too
// (ADR-0097).
func TestTheRunsHandOverReturnsAFailedRecording(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	revoke(t, conn, "UPDATE (last_auth_at, last_auth_outcome) ON account_state")
	j := newJobs()

	err := runAll(t, kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), connector(attempting(t)), j), j)

	if err == nil || !strings.Contains(err.Error(), "account personal: recording the last authentication attempt") {
		t.Errorf("the run returned %v, want the failed recording", err)
	}
}

// fixed returns a policy function that returns c for every page, the active snapshot of a policy
// nothing edits.
func fixed(c policy.Composed) func() policy.Composed { return func() policy.Composed { return c } }

// VERIFICATIONS' row for a job's latest success read from recorded state, backfill's part. Each
// account's job is ensured with its latest success as its runs record it, so a worker that restarts
// without the job succeeding leaves it ageing. An account with a pass not ended counts from its
// latest page made durable or run that succeeded, a first pass in a crash loop, with runs and no
// success, from its earliest run's start, and an account whose backfill has ended, or that has no
// run, from its ensure (ADR-0119).
func TestEachJobStartsFromItsRecordedLatestSuccess(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	client(t, conn, public)
	for _, id := range []string{"progressed", "succeeded", "looping", "ended", "new"} {
		account(t, conn, id, gmailProvider, sealed(t, public, id+"-token", seal.AccountCredential(id)), false)
	}
	t0 := time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)
	run := func(account, id, pass, state string, started time.Time, finished *time.Time) {
		must(t, conn, "INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at) VALUES ($1, $2, 'backfill', $3, $4, $5, $6)",
			account, id, pass, state, started, finished)
	}
	event := func(account, id, kind string, at time.Time) {
		must(t, conn, "INSERT INTO job_run_events (account_id, run_id, kind, at) VALUES ($1, $2, $3, $4)", account, id, kind, at)
	}
	later := func(d time.Duration) *time.Time { v := t0.Add(d); return &v }

	// A first pass that made pages durable and then stopped, and was resumed by a run that has made
	// none, so its latest success is its last page.
	run("progressed", "p1", "pass1", "failed", t0, later(time.Hour))
	event("progressed", "p1", "progress", t0.Add(30*time.Minute))
	event("progressed", "p1", "failure", t0.Add(time.Hour))
	run("progressed", "p2", "pass1", "running", t0.Add(2*time.Hour), nil)
	// A first pass that succeeded, with the second not yet ended, so its latest success is the end of
	// the run that succeeded, later than its last page.
	run("succeeded", "s1", "pass1", "succeeded", t0, later(3*time.Hour))
	event("succeeded", "s1", "progress", t0.Add(2*time.Hour))
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true WHERE account_id = 'succeeded'")
	// A first pass in a crash loop, each run stopped before a page was made durable.
	run("looping", "l1", "pass1", "failed", t0.Add(time.Hour), later(time.Hour+time.Minute))
	run("looping", "l2", "pass1", "running", t0.Add(2*time.Hour), nil)
	// An account whose backfill ended long ago.
	run("ended", "e1", "pass2", "succeeded", t0, later(time.Hour))
	must(t, conn, "UPDATE account_state SET backfill_pass1_complete = true, backfill_pass2_complete = true WHERE account_id = 'ended'")

	j := newJobs()
	b := kind(t, backfillPool(t), ring, slog.New(slog.DiscardHandler), quiet, j)
	if err := b.reload(t.Context(), schedule.Ask{}); err != nil {
		t.Fatal(err)
	}
	got := map[string]time.Time{}
	for key, job := range j.held {
		got[key.ID] = job.LastSuccess.UTC()
	}
	want := map[string]time.Time{
		"progressed": t0.Add(30 * time.Minute),
		"succeeded":  t0.Add(3 * time.Hour),
		"looping":    t0.Add(time.Hour),
		"ended":      {},
		"new":        {},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the latest success each job starts from (-want +got):\n%s", diff)
	}
}
