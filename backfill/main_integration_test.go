//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
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

// reset empties the tables backfill reads. Every test of the package shares one database.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
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

// account writes an account, and a state row holding credential unless stateless.
func account(t *testing.T, conn *pgx.Conn, id, provider string, credential []byte, stateless bool) {
	t.Helper()
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, $2)", id, provider)
	if !stateless {
		must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ($1, $2)", id, credential)
	}
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
	Accounts                      float64
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

// D1's parts of VERIFICATIONS' rows for loading the account snapshot at the start of a run and for a
// credential supplied outside the account's state row. A run takes its accounts, the OAuth clients and
// the credentials from the database. It loads the policy of every listed account, and serves only a
// connected account whose provider it has an adapter and a client for, whatever the environment and a
// mounted file hold, logging every other one (ADR-0080, ADR-0090).
func TestARunTakesItsAccountsFromTheDatabase(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "no-state", gmailProvider, nil, true)
	account(t, conn, "no-credential", gmailProvider, nil, false)
	account(t, conn, "work", "fastmail", sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	must(t, conn, "INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ($1, $2, $3)",
		gmailProvider, "client-id", sealed(t, public, "client-secret", seal.ClientSecret(gmailProvider)))
	t.Setenv("GMAIL_REFRESH_TOKEN", "environment-token")
	mounted := filepath.Join(t.TempDir(), "refresh_token")
	if err := os.WriteFile(mounted, []byte("mounted-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	stored := storedCredential(t, conn, "personal")
	var log logBuffer

	if err := backfill(t.Context(), backfillPool(t), ring, log.logger(), idle); err != nil {
		t.Fatal(err)
	}

	want := []record{
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "no-credential"},
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "no-state"},
		{Level: "WARN", Msg: "backfill has no adapter for an account's provider, so it is skipped", Account: "work", Provider: "fastmail"},
		{Level: "INFO", Msg: "policy loaded", Accounts: 4},
	}
	if diff := cmp.Diff(want, log.records(t), compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
	if got := storedCredential(t, conn, "personal"); !bytes.Equal(got, stored) {
		t.Error("the run rewrote a credential the provider never rotated")
	}
}

// An account whose provider has no OAuth client in the snapshot is logged and skipped, and nothing
// looks for a client that is not there (ADR-0080).
func TestAnAccountWithoutAClientIsSkipped(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	var log logBuffer

	if err := backfill(t.Context(), backfillPool(t), ring, log.logger(), idle); err != nil {
		t.Fatal(err)
	}

	want := []record{
		{Level: "WARN", Msg: "an account's provider has no OAuth client, so it is skipped", Account: "personal", Provider: gmailProvider},
		{Level: "INFO", Msg: "policy loaded", Accounts: 1},
	}
	if diff := cmp.Diff(want, log.records(t), compare.Options); diff != "" {
		t.Errorf("log (-want +got):\n%s", diff)
	}
}

// A snapshot whose read fails stops the run before any policy is loaded, since a run that exits holds
// no previous snapshot to keep (ADR-0090).
func TestAFailedSnapshotReadStopsTheRun(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	revoke(t, conn, "SELECT ON accounts")
	var log logBuffer

	err := backfill(t.Context(), backfillPool(t), ring, log.logger(), idle)

	if !errors.Is(err, accountload.ErrUntrustedRead) {
		t.Fatalf("backfill returned %v, want a failed snapshot read", err)
	}
	for _, r := range log.records(t) {
		if r.Msg == "policy loaded" {
			t.Error("the run loaded the policy after the snapshot's read failed")
		}
	}
}

// idle is a unit of work that does nothing.
func idle(context.Context, map[string]*gmail.TokenSource) error { return nil }

// loaded returns a loader that has loaded the snapshot.
func loaded(t *testing.T, ring *open.Keyring, log *slog.Logger) *accountload.Loader {
	t.Helper()
	l := accountload.New(backfillPool(t), ring, log)
	if err := l.Load(t.Context()); err != nil {
		t.Fatal(err)
	}
	return l
}

// The credentials each served account's token source is built from are the installation's Gmail
// client, its identifier and its secret each in its own place, and the account's own stored refresh
// token (ADR-0083).
func TestTheCredentialsAreTheClientAndTheAccountsToken(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	must(t, conn, "INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ($1, $2, $3)",
		gmailProvider, "client-id", sealed(t, public, "client-secret", seal.ClientSecret(gmailProvider)))

	got := credentials(loaded(t, ring, slog.New(slog.DiscardHandler)).Snapshot(), slog.New(slog.DiscardHandler))

	want := map[string]gmail.Credentials{
		"personal": {ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "personal-token"},
		"work":     {ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "work-token"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("credentials (-want +got):\n%s", diff)
	}
}

// D1's part of VERIFICATIONS' row for forcing a rotation and restarting, driven through the run. The
// run builds each served account's token source holding the account's stored refresh token, and when
// its unit of work ends it hands every source's current token over, a unit of work that failed
// included, so a process started afterwards reads each rotated one (ADR-0082). Google's rotation
// itself is out of reach without a stand-in for its endpoint (ADR-0043), so the unit of work replaces
// each source with one holding the token it would hold after one.
func TestARunHandsEveryAccountOverWhenItsWorkEnds(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	must(t, conn, "INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ($1, $2, $3)",
		gmailProvider, "client-id", sealed(t, public, "client-secret", seal.ClientSecret(gmailProvider)))
	failed := errors.New("the unit of work failed")
	held := map[string]string{}
	rotate := func(_ context.Context, sources map[string]*gmail.TokenSource) error {
		for account, source := range sources {
			held[account] = source.RefreshToken()
			sources[account] = gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{
				ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: account + "-rotated",
			})
		}
		return failed
	}

	err := backfill(t.Context(), backfillPool(t), ring, slog.New(slog.DiscardHandler), rotate)

	if !errors.Is(err, failed) {
		t.Errorf("backfill returned %v, want the unit of work's failure", err)
	}
	if diff := cmp.Diff(map[string]string{"personal": "personal-token", "work": "work-token"}, held, compare.Options); diff != "" {
		t.Errorf("the tokens the sources were built holding (-want +got):\n%s", diff)
	}
	restarted := map[string]string{}
	for _, a := range loaded(t, ring, slog.New(slog.DiscardHandler)).Snapshot().Accounts() {
		restarted[a.ID()] = string(a.Credential())
	}
	if diff := cmp.Diff(map[string]string{"personal": "personal-rotated", "work": "work-rotated"}, restarted, compare.Options); diff != "" {
		t.Errorf("the credentials after a restart (-want +got):\n%s", diff)
	}
}

// At the end of a unit of work the composition root hands each token source's current refresh token to the loader. A rotated one
// is written back, so a process started afterwards reads it, and an unchanged one writes nothing
// (ADR-0082). Google's rotation itself is out of reach without a stand-in for its endpoint (ADR-0043),
// so each source is built holding the token it would hold after one.
func TestTheSourcesRefreshTokenIsHandedOverAtTheEndOfTheRun(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "rotated", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("rotated")), false)
	account(t, conn, "unchanged", gmailProvider, sealed(t, public, "same-token", seal.AccountCredential("unchanged")), false)
	unchanged := storedCredential(t, conn, "unchanged")
	loader := loaded(t, ring, slog.New(slog.DiscardHandler))
	sources := map[string]*gmail.TokenSource{
		"rotated":   gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{ClientID: "id", ClientSecret: "secret", RefreshToken: "second-token"}),
		"unchanged": gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{ClientID: "id", ClientSecret: "secret", RefreshToken: "same-token"}),
	}

	if err := handOver(t.Context(), loader, sources); err != nil {
		t.Fatal(err)
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
// that fails is logged by the loader, the other accounts are still handed over, and the run ends in
// an error naming the account, so the failure is loud (ADR-0082).
func TestAFailedWriteBackEndsTheRunInError(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	var log logBuffer
	loader := loaded(t, ring, log.logger())
	revoke(t, conn, "UPDATE (credential) ON account_state")
	sources := map[string]*gmail.TokenSource{
		"personal": gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{ClientID: "id", ClientSecret: "secret", RefreshToken: "second-token"}),
		"work":     gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{ClientID: "id", ClientSecret: "secret", RefreshToken: "work-token"}),
	}

	err := handOver(t.Context(), loader, sources)

	if err == nil || !strings.Contains(err.Error(), "account personal: writing back the rotated credential") {
		t.Fatalf("handOver returned %v, want the failed write-back", err)
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
