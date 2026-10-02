//go:build integration

package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
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
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/accountload"
	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/pass1"
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
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
	client(t, conn, public)
	t.Setenv("GMAIL_REFRESH_TOKEN", "environment-token")
	mounted := filepath.Join(t.TempDir(), "refresh_token")
	if err := os.WriteFile(mounted, []byte("mounted-token"), 0o600); err != nil {
		t.Fatal(err)
	}
	stored := storedCredential(t, conn, "personal")
	var log logBuffer

	if err := backfill(t.Context(), backfillPool(t), ring, log.logger(), prometheus.NewRegistry(), idle); err != nil {
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

// An account that connects through no OAuth client is not connected, because backfill tells the loader
// Gmail authenticates through one, so it is logged and skipped, though its provider has a client, and
// nothing looks for a client the account does not name (ADR-0080, ADR-0106).
func TestAnAccountWithoutAClientIsSkipped(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	must(t, conn, "UPDATE accounts SET oauth_client = NULL WHERE account_id = 'personal'")
	var log logBuffer

	if err := backfill(t.Context(), backfillPool(t), ring, log.logger(), prometheus.NewRegistry(), idle); err != nil {
		t.Fatal(err)
	}

	want := []record{
		{Level: "WARN", Msg: "an account is not connected, so it is skipped", Account: "personal"},
		{Level: "WARN", Msg: "an account's provider authenticates through an OAuth client and the account has none that opened, so it is not connected", Account: "personal", Provider: gmailProvider},
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

	err := backfill(t.Context(), backfillPool(t), ring, log.logger(), prometheus.NewRegistry(), idle)

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
func idle(context.Context, served) error { return nil }

// called returns h with its source replaced by after, a source holding what calls made over h's
// source would leave it holding, a rotated refresh token or an authentication attempt. Neither changes
// which stored value the source was built over, so the adoption stamp stays h's.
func called(h holding, after *gmail.TokenSource) holding {
	return holding{source: after, adoption: h.adoption}
}

// adopted returns what the run would hold for the account after building its source over the
// credential the loader holds for it: the source, holding token, with the account's adoption stamp
// in the loader's snapshot.
func adopted(t *testing.T, loader *accountload.Loader, account, token string) holding {
	t.Helper()
	a, ok := loader.Snapshot().Account(account)
	if !ok {
		t.Fatalf("the loader holds no account %s", account)
	}
	return holding{
		source:   gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{ClientID: "id", ClientSecret: "secret", RefreshToken: token}),
		adoption: a.Adoption(),
	}
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

	got, err := credentials(loaded(t, ring, slog.New(slog.DiscardHandler)).Snapshot(), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}

	want := map[string]gmail.Credentials{
		"personal": {ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "personal-token"},
		"work":     {ClientID: "employer-id", ClientSecret: "employer-secret", RefreshToken: "work-token"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("credentials (-want +got):\n%s", diff)
	}
}

// D1's part of VERIFICATIONS' row for forcing a rotation and restarting, driven through the run. The
// run builds each served account's token source holding the account's stored refresh token, and the
// hand-over it gives its unit of work hands the account's source's current token over, so a process
// started afterwards reads each rotated one (ADR-0082). Google's rotation itself is out of reach
// without a stand-in for its endpoint (ADR-0043), so the unit of work replaces each source with one
// holding the token it would hold after one before it hands the account over.
func TestARunHandsEachAccountOverWhenItsUnitEnds(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	client(t, conn, public)
	failed := errors.New("the unit of work failed")
	held := map[string]string{}
	rotate := func(ctx context.Context, s served) error {
		for account, h := range s.sources {
			held[account] = h.source.RefreshToken()
			s.sources[account] = called(h, gmail.NewTokenSource(http.DefaultClient, gmail.Credentials{
				ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: account + "-rotated",
			}))
			if err := s.handOver(ctx, account); err != nil {
				return err
			}
		}
		return failed
	}

	err := backfill(t.Context(), backfillPool(t), ring, slog.New(slog.DiscardHandler), prometheus.NewRegistry(), rotate)

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

// At the end of a unit of work the composition root hands the account's token source's current
// refresh token to the loader. A rotated one is written back, so a process started afterwards reads
// it, and an unchanged one writes nothing (ADR-0082). Google's rotation itself is out of reach without
// a stand-in for its endpoint (ADR-0043), so each source is built holding the token it would hold
// after one.
func TestTheSourcesRefreshTokenIsHandedOver(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "rotated", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("rotated")), false)
	account(t, conn, "unchanged", gmailProvider, sealed(t, public, "same-token", seal.AccountCredential("unchanged")), false)
	client(t, conn, public)
	unchanged := storedCredential(t, conn, "unchanged")
	loader := loaded(t, ring, slog.New(slog.DiscardHandler))
	sources := map[string]holding{
		"rotated":   adopted(t, loader, "rotated", "second-token"),
		"unchanged": adopted(t, loader, "unchanged", "same-token"),
	}

	for account, h := range sources {
		if err := handOver(t.Context(), loader, account, h); err != nil {
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
// that fails is logged by the loader and returns an error naming the account, and an unchanged one
// still hands over, so the failure is loud (ADR-0082).
func TestAFailedWriteBackEndsTheRunInError(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "first-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	var log logBuffer
	loader := loaded(t, ring, log.logger())
	revoke(t, conn, "UPDATE (credential) ON account_state")
	sources := map[string]holding{
		"personal": adopted(t, loader, "personal", "second-token"),
		"work":     adopted(t, loader, "work", "work-token"),
	}

	err := handOver(t.Context(), loader, "personal", sources["personal"])

	if err == nil || !strings.Contains(err.Error(), "account personal: writing back the rotated credential") {
		t.Fatalf("handOver returned %v, want the failed write-back", err)
	}
	if err := handOver(t.Context(), loader, "work", sources["work"]); err != nil {
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
		Store: pass1.NewPostgres(pool), Fetch: f.EnumerateAll,
		Policy: none.For(account), Scanner: s,
		Lookups: classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
		RunID:   func() string { n++; return fmt.Sprintf("run%d", n) },
		Now:     time.Now,
	}
}

// Every page of the first pass is a unit of work. The account's token is handed over after each page,
// so a rotation reaches the database a page after it happens rather than at the end of the run
// (ADR-0082). The messages whose sender the classifier could not classify are counted on the
// account's series as each page is made durable (O2).
func TestEachPageIsAUnitOfWork(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	pool := backfillPool(t)
	registry := prometheus.NewRegistry()
	metrics, err := pass1.NewMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	var handedOver []string
	handOver := func(_ context.Context, account string) error {
		handedOver = append(handedOver, account)
		return nil
	}

	if err := passAccount(t.Context(), firstPassDeps(t, pool, "personal"), "personal", metrics, handOver, slog.New(slog.DiscardHandler)); err != nil {
		t.Fatal(err)
	}

	if diff := cmp.Diff([]string{"personal", "personal", "personal"}, handedOver, compare.Options); diff != "" {
		t.Errorf("the hand-overs, one after each of the three pages (-want +got):\n%s", diff)
	}
	if got := counterValue(t, registry, "mediated_mailbox_backfill_unclassified_senders_total", "personal"); got != 1 {
		t.Errorf("the unclassified senders series reads %v, want 1", got)
	}
}

// A hand-over that fails is returned once the pass ends, and the pass still ends, since the loader
// keeps the rotated token in memory and a later hand-over may land it (ADR-0082).
func TestAFailedHandOverFailsThePassAtItsEnd(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	metrics, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	failed := errors.New("the write-back failed")
	calls := 0
	handOver := func(context.Context, string) error {
		calls++
		if calls == 1 {
			return failed
		}
		return nil
	}

	err = passAccount(t.Context(), firstPassDeps(t, backfillPool(t), "personal"), "personal", metrics, handOver, slog.New(slog.DiscardHandler))

	if !errors.Is(err, failed) {
		t.Errorf("passAccount returned %v, want the failed hand-over", err)
	}
	var ended bool
	if err := conn.QueryRow(t.Context(), "SELECT backfill_pass1_complete FROM account_state WHERE account_id = 'personal'").Scan(&ended); err != nil || !ended {
		t.Errorf("the pass did not end after a failed hand-over (%v)", err)
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

// D1's part of VERIFICATIONS' rows for the reload-failure alarm. The health probe answers while
// backfill runs, and its metrics endpoint serves the policy loader's reload-failure series of the run's
// policy load, so the alarm reaches the platform from a running backfill (ADR-0041, ADR-0051,
// ADR-0077).
func TestTheProbesServeTheRunsSeries(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	pool := backfillPool(t)
	registry := prometheus.NewRegistry()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	connect, err := gmailPorts(registry)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := firstPass(pool, s, registry, slog.New(slog.DiscardHandler), connect); err != nil {
		t.Fatal(err)
	}
	if err := backfill(t.Context(), pool, ring, slog.New(slog.DiscardHandler), registry, idle); err != nil {
		t.Fatal(err)
	}
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	stop := serveProbes(ln, registry, slog.New(slog.DiscardHandler))
	defer func() {
		if err := stop(); err != nil {
			t.Error(err)
		}
	}()
	get := func(path string) (int, string) {
		res, err := http.Get("http://" + ln.Addr().String() + path)
		if err != nil {
			t.Fatal(err)
		}
		body, err := io.ReadAll(res.Body)
		if err := errors.Join(err, res.Body.Close()); err != nil {
			t.Fatal(err)
		}
		return res.StatusCode, string(body)
	}
	if code, body := get("/healthz"); code != http.StatusOK || body != "ok\n" {
		t.Errorf("/healthz answered %d %q, want 200 ok", code, body)
	}
	code, body := get("/metrics")
	if code != http.StatusOK {
		t.Fatalf("/metrics answered %d", code)
	}
	if !strings.Contains(body, "\nmediated_mailbox_policyload_reload_failed 0\n") {
		t.Errorf("/metrics does not serve the reload-failure series:\n%s", body)
	}
}

// A page that fails is still a unit of work, so the account's token is handed over after it too, and
// a rotation made before the failure reaches the database (ADR-0082).
func TestAFailedPageIsStillHandedOver(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	metrics, err := pass1.NewMetrics(prometheus.NewRegistry())
	if err != nil {
		t.Fatal(err)
	}
	deps := firstPassDeps(t, backfillPool(t), "personal")
	deps.Fetch = func(context.Context, mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return mail.Page[mail.MessageMetadata]{}, mail.ErrAuthentication
	}
	var handedOver []string
	handOver := func(_ context.Context, account string) error {
		handedOver = append(handedOver, account)
		return nil
	}

	err = passAccount(t.Context(), deps, "personal", metrics, handOver, slog.New(slog.DiscardHandler))

	if !errors.Is(err, mail.ErrAuthentication) {
		t.Errorf("passAccount returned %v, want the refused credential", err)
	}
	if diff := cmp.Diff([]string{"personal"}, handedOver, compare.Options); diff != "" {
		t.Errorf("the hand-overs after the failed page (-want +got):\n%s", diff)
	}
}

// unreachable returns a client whose requests fail before they leave the machine, sent through a
// proxy on a port nothing listens on, so a token source refreshing through it makes a real attempt
// that gets no answer, with no stand-in for Google's endpoint (ADR-0043).
func unreachable() *http.Client {
	return &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(&url.URL{Scheme: "http", Host: "127.0.0.1:9"})}}
}

// attempted returns a token source holding a failed authentication attempt, made through
// unreachable.
func attempted(t *testing.T) *gmail.TokenSource {
	t.Helper()
	source := gmail.NewTokenSource(unreachable(), gmail.Credentials{ClientID: "client-id", ClientSecret: "client-secret", RefreshToken: "refresh-token"})
	if _, err := source.AccessToken(t.Context()); err == nil {
		t.Fatal("a refresh through an unreachable proxy returned an access token")
	}
	if source.LastAttempt().Outcome != mail.AuthFailed {
		t.Fatalf("the source holds %+v, want a failed attempt", source.LastAttempt())
	}
	return source
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

// D3's part of VERIFICATIONS' row for failing a unit of work after an authentication attempt. The
// hand-over the run gives its unit of work records each account's source's latest attempt, a unit
// that failed included, and the statement the system status reads returns it. An account whose
// source made no attempt records nothing (ADR-0097, ADR-0034).
func TestARunRecordsEachAccountsAttemptWhenItsUnitEnds(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	account(t, conn, "work", gmailProvider, sealed(t, public, "work-token", seal.AccountCredential("work")), false)
	client(t, conn, public)
	failed := errors.New("the unit of work failed")
	var attempt mail.AuthAttempt
	work := func(ctx context.Context, s served) error {
		s.sources["personal"] = called(s.sources["personal"], attempted(t))
		attempt = s.sources["personal"].source.LastAttempt()
		for _, account := range []string{"personal", "work"} {
			if err := s.handOver(ctx, account); err != nil {
				return err
			}
		}
		return failed
	}
	pool := backfillPool(t)

	err := backfill(t.Context(), pool, ring, slog.New(slog.DiscardHandler), prometheus.NewRegistry(), work)

	if !errors.Is(err, failed) {
		t.Errorf("backfill returned %v, want the unit of work's failure", err)
	}
	at, outcome := lastAuthentication(t, pool, "personal")
	if at == nil || outcome == nil || !at.Equal(time.UnixMilli(int64(attempt.At))) || *outcome != "failed" {
		t.Errorf("the personal account's last authentication reads %v %v, want failed at %d", at, outcome, attempt.At)
	}
	if at, outcome := lastAuthentication(t, pool, "work"); at != nil || outcome != nil {
		t.Errorf("the work account, whose source made no attempt, reads %v %v, want nothing recorded", at, outcome)
	}
}

// The recorded attempt is the latest one. An attempt older than the one the row holds changes
// nothing, and one later than it, or than a row that holds none, replaces it, so a deployable whose
// unit of work ended late never puts an older outcome back (ADR-0097).
func TestOnlyALaterAttemptIsRecorded(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	for _, id := range []string{"later-stored", "earlier-stored", "none-stored"} {
		account(t, conn, id, gmailProvider, nil, false)
	}
	source := attempted(t)
	made := time.UnixMilli(int64(source.LastAttempt().At)).UTC()
	must(t, conn, "UPDATE account_state SET last_auth_at = $1, last_auth_outcome = 'succeeded' WHERE account_id = 'later-stored'", made.Add(time.Millisecond))
	must(t, conn, "UPDATE account_state SET last_auth_at = $1, last_auth_outcome = 'refused' WHERE account_id = 'earlier-stored'", made.Add(-time.Millisecond))
	pool := backfillPool(t)

	for _, id := range []string{"later-stored", "earlier-stored", "none-stored"} {
		if err := recordAttempt(t.Context(), pool, id, source.LastAttempt()); err != nil {
			t.Fatalf("recording for %s: %v", id, err)
		}
	}

	got := map[string]string{}
	for _, id := range []string{"later-stored", "earlier-stored", "none-stored"} {
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
	conn := superuser(t)
	reset(t, conn)
	reported := map[string]mail.AuthAttempt{
		"succeeded": {At: 1_790_000_000_001, Outcome: mail.AuthSucceeded},
		"refused":   {At: 1_790_000_000_002, Outcome: mail.AuthRefused},
		"failed":    {At: 1_790_000_000_003, Outcome: mail.AuthFailed},
	}
	for id := range reported {
		account(t, conn, id, gmailProvider, nil, false)
	}
	pool := backfillPool(t)

	for id, attempt := range reported {
		if err := recordAttempt(t.Context(), pool, id, attempt); err != nil {
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
	conn := superuser(t)
	reset(t, conn)
	account(t, conn, "personal", gmailProvider, nil, false)
	revoke(t, conn, "UPDATE (last_auth_at, last_auth_outcome) ON account_state")

	err := recordAttempt(t.Context(), backfillPool(t), "personal", attempted(t).LastAttempt())

	if err == nil || !strings.Contains(err.Error(), "account personal: recording the last authentication attempt") {
		t.Errorf("recordAttempt returned %v, want the failed recording", err)
	}
}

// The hand-over the run gives its unit of work returns a recording that fails, so the unit of work,
// which joins the hand-overs that fail into the error its pass ends in, ends in it too (ADR-0097).
func TestTheRunsHandOverReturnsAFailedRecording(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keys(t)
	account(t, conn, "personal", gmailProvider, sealed(t, public, "personal-token", seal.AccountCredential("personal")), false)
	client(t, conn, public)
	revoke(t, conn, "UPDATE (last_auth_at, last_auth_outcome) ON account_state")
	work := func(ctx context.Context, s served) error {
		s.sources["personal"] = called(s.sources["personal"], attempted(t))
		return s.handOver(ctx, "personal")
	}

	err := backfill(t.Context(), backfillPool(t), ring, slog.New(slog.DiscardHandler), prometheus.NewRegistry(), work)

	if err == nil || !strings.Contains(err.Error(), "account personal: recording the last authentication attempt") {
		t.Errorf("backfill returned %v, want the failed recording", err)
	}
}
