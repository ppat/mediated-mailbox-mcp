//go:build integration

package app_test

import (
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/api/app"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

// noMailbox is the adapters' mailbox for a test whose API server reaches no provider.
func noMailbox(account string, _ *fake.Token) (*fake.Throttled, error) {
	return nil, fmt.Errorf("the test's provider holds no mailbox for %s", account)
}

// serve starts an API server over the test database with args beyond the defaults flags gives, over
// adapters that reach no mailbox, and waits until it is ready.
func serve(t *testing.T, k keyPair, interval time.Duration, more ...string) *server {
	t.Helper()
	s := start(t, apiFlags(t, k, interval, time.Second, more...), connections.WithAdapters(fake.Adapters(noMailbox, gmail)))
	s.awaitReady(t, 15*time.Second)
	return s
}

// newsletter writes a message from the newsletter's domain to the account's index.
func newsletter(t *testing.T, conn *pgx.Conn, account string) {
	t.Helper()
	must(t, conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at,
		has_attachments, scan_state) VALUES ($1, 'm-news', 't', 'editor@newsletter.example', 'newsletter.example', now(), false, 'scanned')`, account)
}

// senderClass returns the sender class the account's newsletter message reads under the policy in
// force, restricted for an account whose policy restricts every sender because no load read its rules.
func (s *server) senderClass(t *testing.T, account string) string {
	t.Helper()
	status, out := s.api(t, "/api/accounts/"+account+"/messages/m-news")
	if status != http.StatusOK {
		t.Fatalf("get_message for %s answered %d %s", account, status, out)
	}
	var m struct {
		Sensitivity struct {
			SenderClass string `json:"sender_class"`
		} `json:"sensitivity"`
	}
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	return m.Sensitivity.SenderClass
}

// D3's part of VERIFICATIONS' row for reloading the account snapshot, for the accounts the API server
// serves, and the row for the operations the registry builds over the Session. Every reload runs on
// the API server's interval with no restart. An account connected while the API server runs is
// served from the next reload, with its policy loaded, and one removed is dropped. A reload whose read
// fails keeps the accounts served and is logged, and one that lists no account serves none (ADR-0090,
// ADR-0142).
func TestTheAPIServerServesTheAccountsEachReloadLists(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	accounts := newAccounts(t, conn)
	first, second := accounts[0], accounts[1]
	connect(t, conn, k, first)
	must(t, conn, "DELETE FROM accounts WHERE account_id = $1", second)
	s := serve(t, k, 100*time.Millisecond)
	if !s.served(t, first) || s.served(t, second) {
		t.Fatalf("after the first load, %s is served %v and %s %v", first, s.served(t, first), second, s.served(t, second))
	}

	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", second)
	connect(t, conn, k, second)
	newsletter(t, conn, second)
	s.awaitServed(t, second, true)
	if got := s.senderClass(t, second); got != "normal" {
		t.Errorf("an account served after a reload reads a newsletter as %s, so it has no policy of its own loaded", got)
	}
	status, out := s.api(t, "/api/accounts")
	if want := `{"accounts":[{"account_id":"` + first + `","provider":"gmail"},{"account_id":"` + second + `","provider":"gmail"}]}`; status != http.StatusOK || strings.TrimSpace(out) != want {
		t.Errorf("list_accounts answered %d %s, want %s", status, out, want)
	}

	must(t, conn, "DELETE FROM messages WHERE account_id = $1", first)
	must(t, conn, "DELETE FROM account_state WHERE account_id = $1", first)
	must(t, conn, "DELETE FROM accounts WHERE account_id = $1", first)
	s.awaitServed(t, first, false)
	if !s.served(t, second) {
		t.Errorf("after removing %s, %s is no longer served", first, second)
	}

	revoke(t, conn, "SELECT ON accounts")
	for deadline := time.Now().Add(10 * time.Second); s.logs.count("the Session was not reloaded in full, so what it held stays in force") == 0; time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("a failed scheduled reload was not logged:\n%s", s.logs.String())
		}
	}
	if !s.served(t, second) {
		t.Error("a reload whose read failed dropped the account served")
	}
	grantBack(t, conn, "SELECT ON accounts")

	must(t, conn, "DELETE FROM messages")
	must(t, conn, "DELETE FROM account_state")
	must(t, conn, "DELETE FROM accounts")
	s.awaitServed(t, second, false)
}

// D3's row for F3's rate-state series. The metrics endpoint carries them for each account the API
// server serves of a provider a shipped adapter serves, and only for those, following each reload
// (ADR-0077).
func TestTheMetricsEndpointCarriesEachServedAccountsRateState(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	accounts := newAccounts(t, conn)
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'fastmail')", accounts[0]+"-other")
	for _, a := range accounts {
		connect(t, conn, k, a)
	}
	s := serve(t, k, 100*time.Millisecond)
	reported := func() []string {
		var out []string
		for labels := range s.series(t, accountRateName) {
			for _, a := range append(slices.Clone(accounts), accounts[0]+"-other") {
				if strings.Contains(labels, `account="`+a+`"`) {
					out = append(out, a)
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
	s.awaitServed(t, accounts[0], false)
	if diff := cmp.Diff(accounts[1:], reported()); diff != "" {
		t.Errorf("after a reload dropped an account, the rate series report (-want +got):\n%s", diff)
	}
}

// reloadFailed returns the values of the policies' reload-failure series on the metrics endpoint.
func (s *server) reloadFailed(t *testing.T) []float64 {
	t.Helper()
	var values []float64
	for _, v := range s.series(t, reloadFailedName) {
		values = append(values, v)
	}
	return values
}

// awaitReloadFailed waits until the reload-failure series reads want, failing the test after ten
// seconds.
func (s *server) awaitReloadFailed(t *testing.T, want []float64) {
	t.Helper()
	var got []float64
	for deadline := time.Now().Add(10 * time.Second); time.Now().Before(deadline); time.Sleep(20 * time.Millisecond) {
		if got = s.reloadFailed(t); slices.Equal(got, want) {
			return
		}
	}
	t.Fatalf("the reload-failure series reads %v, want %v", got, want)
}

// A reload whose policy load fails still serves the accounts its list holds, so an account the list
// drops is dropped (ADR-0090). An account the policy was loaded for before keeps that policy, and one
// added meanwhile restricts every sender until a policy load reads its rules (ADR-0041). The next
// reload loads the policy again. The metrics endpoint carries one reload-failure series, labelled
// with the API server's component, that reads 0 after a load that succeeded and 1 after one that
// failed, which the reload-failure alarm reads (ADR-0077).
func TestAFailedPolicyLoadStillFollowsTheList(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	accounts := newAccounts(t, conn)
	kept, dropped := accounts[0], accounts[1]
	for _, a := range accounts {
		connect(t, conn, k, a)
		newsletter(t, conn, a)
	}
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
	s := serve(t, k, 100*time.Millisecond)
	s.awaitReloadFailed(t, []float64{0})
	if got := s.series(t, reloadFailedName); len(got) != 1 || !strings.Contains(slices.Collect(mapKeys(got))[0], `component="api"`) {
		t.Errorf("the reload-failure series are %v, want one labelled with the API server's component", got)
	}

	const policyRead = "SELECT ON policy_rules"
	revoke(t, conn, policyRead)
	must(t, conn, "DELETE FROM messages WHERE account_id = $1", dropped)
	must(t, conn, "DELETE FROM account_state WHERE account_id = $1", dropped)
	must(t, conn, "DELETE FROM accounts WHERE account_id = $1", dropped)
	added := kept + "-added"
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", added)
	connect(t, conn, k, added)
	newsletter(t, conn, added)
	s.awaitReloadFailed(t, []float64{1})
	s.awaitServed(t, added, true)
	if s.served(t, dropped) {
		t.Error("an account the list dropped is still served after the policy load failed")
	}
	if !s.served(t, kept) {
		t.Errorf("after the policy load failed, %s is no longer served", kept)
	}
	if got := s.senderClass(t, kept); got != "normal" {
		t.Errorf("an account whose policy was loaded before reads a newsletter as %s after a later load failed", got)
	}
	if got := s.senderClass(t, added); got != "restricted" {
		t.Errorf("an account added while the policy load failed reads a newsletter as %s, under a policy that never read its rules", got)
	}

	grantBack(t, conn, policyRead)
	s.awaitReloadFailed(t, []float64{0})
	if got := s.senderClass(t, added); got != "normal" {
		t.Errorf("the next reload did not load the policy of the account added meanwhile, which reads a newsletter as %s", got)
	}
}

// mapKeys returns the keys of m.
func mapKeys[V any](m map[string]V) func(func(string) bool) {
	return func(yield func(string) bool) {
		for k := range m {
			if !yield(k) {
				return
			}
		}
	}
}

// The read operations the registry builds read the accounts and the policy the Session holds in force
// and the real clock. The accounts listing lists the Session's accounts, and a message is decided under
// the policy loaded for its account, so a sender the policy lists reads restricted and an unlisted one
// normal. The system status reads a backoff ending in an hour as running, one that ended an hour ago
// as over, and a sync cursor written an hour ago as an hour old, to within thirty seconds.
func TestTheReadOperationsReadTheSession(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	accounts := newAccounts(t, conn)
	account := accounts[0]
	connect(t, conn, k, account)
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
	for id, from := range map[string]string{"m-bank": "alerts@bank.example", "m-news": "editor@newsletter.example"} {
		must(t, conn, `INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at,
			has_attachments, scan_state) VALUES ($1, $2, 't', $3, $4, now(), false, 'scanned')`,
			account, id, from, from[strings.IndexByte(from, '@')+1:])
	}
	// One account's backoff ends an hour from now and the other's ended an hour ago, and the first
	// account's sync cursor was written an hour ago, so a clock ahead of the real one, behind it or
	// stopped reads at least one of them wrong.
	must(t, conn, `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, backoff_until)
		VALUES ($1, 5, 8, 10, now() + interval '1 hour'), ($2, 5, 8, 10, now() - interval '1 hour')`, account, accounts[1])
	must(t, conn, "UPDATE account_state SET sync_cursor_at = now() - interval '1 hour' WHERE account_id = $1", account)
	s := serve(t, k, time.Hour)

	status, out := s.api(t, "/api/accounts")
	if want := `{"accounts":[{"account_id":"` + accounts[0] + `","provider":"gmail"},{"account_id":"` + accounts[1] + `","provider":"gmail"}]}`; status != http.StatusOK || strings.TrimSpace(out) != want {
		t.Errorf("list_accounts answered %d %s, want %s", status, out, want)
	}
	for id, want := range map[string]string{"m-bank": "restricted false", "m-news": "normal true"} {
		status, out := s.api(t, "/api/accounts/"+account+"/messages/"+id)
		if status != http.StatusOK {
			t.Fatalf("get_message %s answered %d %s", id, status, out)
		}
		var m struct {
			Sensitivity struct {
				SenderClass string `json:"sender_class"`
			} `json:"sensitivity"`
			BodyAvailable bool `json:"body_available"`
		}
		if err := json.Unmarshal([]byte(out), &m); err != nil {
			t.Fatal(err)
		}
		if got := fmt.Sprintf("%s %v", m.Sensitivity.SenderClass, m.BodyAvailable); got != want {
			t.Errorf("get_message %s reads %s, want %s", id, got, want)
		}
	}
	type systemStatus struct {
		Sync struct {
			CursorAgeSeconds float64 `json:"cursor_age_seconds"`
		} `json:"sync"`
		Rate struct {
			InBackoff bool `json:"in_backoff"`
		} `json:"rate"`
	}
	read := func(id string) (systemStatus, string) {
		status, out := s.api(t, "/api/accounts/"+id+"/status")
		if status != http.StatusOK {
			t.Fatalf("get_system_status %s answered %d %s", id, status, out)
		}
		var st systemStatus
		if err := json.Unmarshal([]byte(out), &st); err != nil {
			t.Fatal(err)
		}
		return st, out
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

// The API server's start runs the schema check on its pool before it serves, so it refuses a schema
// that has not applied its chain and one whose compatibility floor is above it, before it reports
// ready, and starts on its chain's own schema and on a newer one whose floor its chain meets
// (ADR-0140, ADR-0142).
func TestTheAPIServerRefusesASchemaItCannotRunOn(t *testing.T) {
	k := newKeyPair(t)
	postgres.SchemaChecks(t, "../../db/migrations", func(ctx context.Context, pool *pgxpool.Pool) error {
		c := pool.Config().ConnConfig
		password := filepath.Join(t.TempDir(), "password")
		if err := os.WriteFile(password, []byte(c.Password), 0o600); err != nil {
			t.Fatal(err)
		}
		args := []string{
			"--database.host=" + c.Host, fmt.Sprintf("--database.port=%d", c.Port), "--database.name=" + c.Database,
			"--database.user=" + postgres.ApplicationUser, "--database.sslmode=disable", "--database.password_file=" + password,
			"--token_file=" + tokenFile(t), "--listen=127.0.0.1:0", "--probe_listen=127.0.0.1:0",
			"--credential.public_key_file=" + k.publicFile, "--credential.private_key_files=[" + k.privateFile + "]",
		}
		logs := &logBuffer{}
		running, stop := context.WithCancel(ctx)
		defer stop()
		done := make(chan error, 1)
		go func() {
			done <- process.RunContext(running, args, nil, logs, io.Discard, app.Spec(), connections.WithAdapters(fake.Adapters(noMailbox, gmail)))
		}()
		for {
			select {
			case err := <-done:
				return err
			case <-time.After(20 * time.Millisecond):
			}
			if logs.count("serving") > 0 {
				stop()
				if err := <-done; err != nil {
					return fmt.Errorf("the API server that started stopped with %w", err)
				}
				return nil
			}
		}
	}, postgres.ApplicationUser)
}

// D3's part of VERIFICATIONS' row for a first reload that fails, for the API server. Its first reload
// fails to read the list of accounts after its start's schema check has passed, so it starts, serves,
// reports not ready naming the Session and tries again, rather than refusing to start, and is ready
// once a reload loads the list (ADR-0090, ADR-0142).
func TestAFirstReloadThatFailsLeavesTheAPIServerStartedAndNotReady(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	account := newAccounts(t, conn)[0]
	connect(t, conn, k, account)
	revoke(t, conn, "SELECT ON accounts")
	s := start(t, apiFlags(t, k, time.Hour, time.Second), connections.WithAdapters(fake.Adapters(noMailbox, gmail)))
	for deadline := time.Now().Add(10 * time.Second); s.logs.count("the process failed") == 0; time.Sleep(20 * time.Millisecond) {
		status, body := s.ready(t)
		if status == http.StatusServiceUnavailable && strings.Contains(body, "not ready: session: ") {
			break
		}
		if time.Now().After(deadline) {
			t.Fatalf("with its first reload failing the readiness probe answered %d %s", status, body)
		}
	}
	if s.logs.count("the process failed") != 0 {
		t.Fatalf("the API server stopped when its first reload failed:\n%s", s.logs.String())
	}
	grantBack(t, conn, "SELECT ON accounts")
	s.awaitReady(t, 15*time.Second)
	if !s.served(t, account) {
		t.Errorf("once a reload loaded the list, %s is not served", account)
	}
}

// D3's part of VERIFICATIONS' row for the API server's readiness. It is ready once the Session has
// read the accounts, their connections and their policy and while its bearer token file holds a
// token, and it reads no key pair while serving plain HTTP. Its token file emptied while it runs
// leaves it not ready, naming the bearer token (ADR-0051, ADR-0127, ADR-0142).
func TestReadyOnlyWhileTheSessionHasLoadedAndTheTokenLoads(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	token := tokenFile(t)
	s := serve(t, k, time.Hour, "--token_file="+token, "--tls_cert=", "--tls_key=")
	if err := os.WriteFile(token, []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	var status int
	var body string
	for deadline := time.Now().Add(12 * time.Second); time.Now().Before(deadline); time.Sleep(50 * time.Millisecond) {
		if status, body = s.ready(t); status == http.StatusServiceUnavailable {
			break
		}
	}
	if status != http.StatusServiceUnavailable || !strings.Contains(body, "not ready: bearer token: ") {
		t.Errorf("with its token file emptied the readiness probe answered %d %s", status, body)
	}
}

// VERIFICATIONS' row for probes a state the operator fixes does not fail, for the API server. With no
// account, with accounts whose stored credentials cannot be opened, one holding bytes that are no
// sealed value and one sealed to another key, with accounts of a provider that needs an OAuth client,
// one naming none and one whose client's secret, sealed to another key, does not open, and with an
// account whose policy was never imported, the API server's reloads succeed and both probes answer 200 (ADR-0142).
func TestNoAccountOrAnAccountThatCannotConnectFailsNoProbe(t *testing.T) {
	for _, c := range []struct {
		name  string
		plant func(t *testing.T, conn *pgx.Conn, k keyPair)
	}{
		{"no account", func(*testing.T, *pgx.Conn, keyPair) {}},
		{"accounts whose credentials cannot be opened", func(t *testing.T, conn *pgx.Conn, k keyPair) {
			accounts := newAccounts(t, conn)
			must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ($1, $2)", accounts[0], []byte("not a sealed value"))
			connect(t, conn, newKeyPair(t), accounts[1])
		}},
		{"accounts that need an OAuth client and name none or whose client does not open", func(t *testing.T, conn *pgx.Conn, k keyPair) {
			accounts := newAccounts(t, conn)
			connect(t, conn, k, accounts[0])
			client(t, conn, newKeyPair(t), "elsewhere", "elsewhere-id", "elsewhere-secret")
			through(t, conn, accounts[1], "elsewhere")
			connect(t, conn, k, accounts[1])
		}},
		{"an account whose policy was never imported", func(t *testing.T, conn *pgx.Conn, k keyPair) {
			account := newAccounts(t, conn)[0]
			client(t, conn, k, household, "household-id", "household-secret")
			through(t, conn, account, household)
			connect(t, conn, k, account)
		}},
	} {
		t.Run(c.name, func(t *testing.T) {
			conn := superuser(t)
			reset(t, conn)
			k := newKeyPair(t)
			c.plant(t, conn, k)
			s := serve(t, k, 200*time.Millisecond)
			time.Sleep(time.Second)
			for _, path := range []string{"/readyz", "/healthz"} {
				if status, body, _ := s.get(t, "http://"+s.probes+path, ""); status != http.StatusOK {
					t.Errorf("%s answered %d %q, want 200", path, status, body)
				}
			}
		})
	}
}

// D3's part of VERIFICATIONS' row for the driver's own variables. With PGHOST, PGDATABASE and
// PGSSLMODE naming another server, another database and another mode, the API server connects as its
// configuration says, so it loads its accounts and is ready (ADR-0078).
func TestTheDriversVariablesDoNotMoveTheConnection(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	t.Setenv("PGHOST", "db.invalid")
	t.Setenv("PGDATABASE", "another")
	t.Setenv("PGSSLMODE", "require")
	serve(t, newKeyPair(t), time.Hour)
}

// D3's part of VERIFICATIONS' row for the configured log level, for the API server. Started with
// log_level set to warn, it writes its effective configuration and then nothing at info, though its
// serving record and every reload's record of the accounts it serves are written at info, which a
// start with no log_level writes (ADR-0122). Under warn no serving record names the ports, so the test
// picks them.
func TestTheConfiguredLevelGovernsTheLog(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	free := func() string {
		ln, err := net.Listen("tcp", "127.0.0.1:0")
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := ln.Close(); err != nil {
				t.Error(err)
			}
		}()
		return ln.Addr().String()
	}
	quiet := &server{app: free(), probes: free(), logs: &logBuffer{}}
	args := apiFlags(t, k, time.Hour, time.Second, "--log_level=warn", "--listen="+quiet.app, "--probe_listen="+quiet.probes)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() {
		done <- process.RunContext(ctx, args, nil, quiet.logs, io.Discard, app.Spec(), connections.WithAdapters(fake.Adapters(noMailbox, gmail)))
	}()
	for deadline := time.Now().Add(15 * time.Second); ; time.Sleep(50 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatalf("the API server under warn was not ready:\n%s", quiet.logs.String())
		}
		if resp, err := statusOf(t, http.DefaultClient, "http://"+quiet.probes+"/readyz", ""); err == nil && resp.status == http.StatusOK {
			break
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	for line := range strings.Lines(quiet.logs.String()) {
		var record struct{ Msg, Level string }
		if err := json.Unmarshal([]byte(line), &record); err != nil {
			t.Fatalf("the log holds a line that is not a JSON record: %q", line)
		}
		if record.Level == "INFO" && record.Msg != "configuration" {
			t.Errorf("under warn the API server wrote %q at info", record.Msg)
		}
	}
	if loud := serve(t, k, time.Hour); loud.logs.count("accounts served") == 0 {
		t.Errorf("with no log_level the API server wrote nothing at info past its configuration:\n%s", loud.logs.String())
	}
}

// D3's part of VERIFICATIONS' row for a public key matching none of the private keys. The start loads
// the public key and every private key, refuses it when the public key matches none of them, and goes
// on when it matches any one, the second of two during a key replacement included (ADR-0088,
// ADR-0092).
func TestTheAPIServersPublicKeyMustMatchAPrivateKey(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	old, current, other := newKeyPair(t), newKeyPair(t), newKeyPair(t)
	for _, c := range []struct {
		name     string
		public   string
		privates []string
		refused  bool
	}{
		{"a public key matching no private key", other.publicFile, []string{old.privateFile, current.privateFile}, true},
		{"a public key matching the only private key", old.publicFile, []string{old.privateFile}, false},
		{"a public key matching the first of two", old.publicFile, []string{old.privateFile, current.privateFile}, false},
		{"a public key matching the second of two", current.publicFile, []string{old.privateFile, current.privateFile}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			args := apiFlags(t, old, time.Hour, time.Second,
				"--credential.public_key_file="+c.public, "--credential.private_key_files=["+strings.Join(c.privates, ", ")+"]")
			if !c.refused {
				start(t, args, connections.WithAdapters(fake.Adapters(noMailbox, gmail)))
				return
			}
			ctx, cancel := context.WithTimeout(t.Context(), 20*time.Second)
			defer cancel()
			err := process.RunContext(ctx, args, nil, io.Discard, io.Discard, app.Spec(), connections.WithAdapters(fake.Adapters(noMailbox, gmail)))
			if err == nil || !strings.Contains(err.Error(), "loading the keyring: ") || !strings.Contains(err.Error(), "matches none of the private keys") {
				t.Errorf("the run returned %v, want the keyring's refusal", err)
			}
		})
	}
}

// The MCP root reports the release version as its server's version, the one --version prints, so the
// binary names one version wherever it reports one (ADR-0078).
func TestTheMCPRootReportsTheReleaseVersion(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	s := serve(t, newKeyPair(t), time.Hour)
	initialize := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	status, text, _ := s.send(t, http.MethodPost, "http://"+s.app+"/mcp", "Bearer "+bearer, initialize)
	if status != http.StatusOK {
		t.Fatalf("initialize answered %d: %s", status, text)
	}
	if i := strings.Index(text, "{"); i > 0 {
		text = strings.TrimSpace(text[i:])
	}
	var answer struct {
		Result struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		t.Fatalf("decoding %q: %v", text, err)
	}
	if got := answer.Result.ServerInfo; got.Name != "mediated-mailbox-api" || got.Version != version.Number {
		t.Errorf("the server reports %+v, want mediated-mailbox-api at %s", got, version.Number)
	}
}

// writeKeyPair writes a new self-signed certificate for 127.0.0.1 and its key to certFile and keyFile,
// and returns the certificate.
func writeKeyPair(t *testing.T, certFile, keyFile string, serial int64) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	template := &x509.Certificate{
		SerialNumber: big.NewInt(serial),
		Subject:      pkix.Name{CommonName: "api test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
		IPAddresses:  []net.IP{net.ParseIP("127.0.0.1")},
		KeyUsage:     x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		// Self-signed, so the test's client trusts it as its own authority.
		IsCA:                  true,
		BasicConstraintsValid: true,
	}
	der, err := x509.CreateCertificate(rand.Reader, template, template, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(certFile, pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(keyFile, pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}), 0o600); err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

// servingRecord returns the serving record the start wrote.
func (s *server) servingRecord(t *testing.T) string {
	t.Helper()
	for line := range strings.Lines(s.logs.String()) {
		if strings.Contains(line, `"msg":"serving"`) {
			return line
		}
	}
	t.Fatalf("the start wrote no serving record:\n%s", s.logs.String())
	return ""
}

// answered is what one request was answered with, its status and the certificate the server
// presented, nil over plain HTTP.
type answered struct {
	status int
	peer   *x509.Certificate
}

// statusOf sends a GET to url through client, with authorization when it is not empty, and returns
// what it was answered with, or the error.
func statusOf(t *testing.T, client *http.Client, url, authorization string) (answered, error) {
	t.Helper()
	req, err := http.NewRequestWithContext(t.Context(), http.MethodGet, url, http.NoBody)
	if err != nil {
		t.Fatal(err)
	}
	if authorization != "" {
		req.Header.Set("Authorization", authorization)
	}
	resp, err := client.Do(req)
	if err != nil {
		return answered{}, err
	}
	defer func() {
		_, err := io.Copy(io.Discard, resp.Body)
		if err := errors.Join(err, resp.Body.Close()); err != nil {
			t.Error(err)
		}
	}()
	a := answered{status: resp.StatusCode}
	if resp.TLS != nil && len(resp.TLS.PeerCertificates) > 0 {
		a.peer = resp.TLS.PeerCertificates[0]
	}
	return a, nil
}

// Given a key pair, the API server serves both roots over TLS and logs that it does (ADR-0127). A plain
// HTTP request never reaches a root, a TLS request without the token is refused by the bearer check, a
// TLS request with the token reaches its root, and a key pair rotated in its mounted files serves the
// next handshake.
func TestTheSurfaceIsServedOverTLSWhenGivenAKeyPair(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	account := newAccounts(t, conn)[0]
	connect(t, conn, k, account)
	dir := t.TempDir()
	certFile, keyFile := filepath.Join(dir, "tls.crt"), filepath.Join(dir, "tls.key")
	first := writeKeyPair(t, certFile, keyFile, 1)
	s := serve(t, k, time.Hour, "--tls_cert="+certFile, "--tls_key="+keyFile)
	path := "/api/accounts/" + account + "/status"
	trusting := func(cert *x509.Certificate) *http.Client {
		pool := x509.NewCertPool()
		pool.AddCert(cert)
		return &http.Client{Transport: &http.Transport{TLSClientConfig: &tls.Config{RootCAs: pool, MinVersion: tls.VersionTLS12}, DisableKeepAlives: true}}
	}

	if resp, err := statusOf(t, &http.Client{}, "http://"+s.app+path, "Bearer "+bearer); err == nil && resp.status == http.StatusOK {
		t.Error("a plain HTTP request was served")
	}
	unauthenticated, err := statusOf(t, trusting(first), "https://"+s.app+path, "")
	if err != nil {
		t.Fatalf("a TLS request without the token: %v", err)
	}
	if unauthenticated.status != http.StatusUnauthorized {
		t.Fatalf("a TLS request without the token answered %d", unauthenticated.status)
	}
	for i := range 2 {
		cert := first
		if i == 1 {
			cert = writeKeyPair(t, certFile, keyFile, 2)
		}
		resp, err := statusOf(t, trusting(cert), "https://"+s.app+path, "Bearer "+bearer)
		if err != nil {
			t.Fatalf("handshake %d with the key pair then in the files: %v", i+1, err)
		}
		if resp.status != http.StatusOK || resp.peer == nil || resp.peer.SerialNumber.Int64() != cert.SerialNumber.Int64() {
			t.Errorf("handshake %d: status %d, want 200 with the certificate of serial %v", i+1, resp.status, cert.SerialNumber)
		}
	}
	if got := s.servingRecord(t); !strings.Contains(got, `"tls":true`) {
		t.Errorf("the start's serving record does not say it serves TLS: %s", got)
	}
}

// Given no key pair, or two TLS paths of spaces, the API server serves both roots over plain HTTP,
// behind the bearer check, and logs that it does not serve TLS (ADR-0127, ADR-0087).
func TestTheSurfaceGivenNoKeyPairIsPlain(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	account := newAccounts(t, conn)[0]
	connect(t, conn, k, account)
	for name, args := range map[string][]string{
		"neither file": nil,
		"both blank":   {"--tls_cert= ", "--tls_key=\t"},
	} {
		t.Run(name, func(t *testing.T) {
			s := serve(t, k, time.Hour, args...)
			statuses := map[string]int{}
			for name, token := range map[string]string{"with the token": "Bearer " + bearer, "without": ""} {
				resp, err := statusOf(t, http.DefaultClient, "http://"+s.app+"/api/accounts/"+account+"/status", token)
				if err != nil {
					t.Fatal(err)
				}
				statuses[name] = resp.status
			}
			if diff := cmp.Diff(map[string]int{"with the token": 200, "without": 401}, statuses, compare.Options); diff != "" {
				t.Errorf("statuses (-want +got):\n%s", diff)
			}
			if got := s.servingRecord(t); !strings.Contains(got, `"tls":false`) {
				t.Errorf("the start's serving record does not say it serves plain HTTP: %s", got)
			}
		})
	}
}

// In the API server's process a path outside the API root and the MCP root is not found, answered by
// the process's application server before any bearer check, since the spec lists the two roots at
// those two patterns alone. A path under either root, the MCP root's own among them, still meets the
// bearer check (ADR-0142, ADR-0030).
func TestAPathOutsideBothRootsIsNotFound(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	k := newKeyPair(t)
	account := newAccounts(t, conn)[0]
	connect(t, conn, k, account)
	s := serve(t, k, time.Hour)
	statuses := map[string]int{}
	for _, path := range []string{"/healthz", "/readyz", "/metrics", "/", "/apix", "/mcp/x", "/api/accounts/" + account + "/status", "/mcp"} {
		resp, err := statusOf(t, http.DefaultClient, "http://"+s.app+path, "")
		if err != nil {
			t.Fatal(err)
		}
		statuses[path] = resp.status
	}
	want := map[string]int{
		"/healthz": 404, "/readyz": 404, "/metrics": 404, "/": 404, "/apix": 404, "/mcp/x": 404,
		"/api/accounts/" + account + "/status": 401, "/mcp": 401,
	}
	if diff := cmp.Diff(want, statuses, compare.Options); diff != "" {
		t.Errorf("statuses without the token (-want +got):\n%s", diff)
	}
}
