//go:build integration

package assembly_test

import (
	"context"
	"errors"
	"log/slog"
	"math"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/assembly"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// The role the context connects as in these tests, delta sync's, as the worker builds it (ADR-0118).
const role = "mediated_mailbox_sync"

// ceiling is a provider's declared budget per second, Gmail's.
const ceiling = 100

// pool returns a pool connecting as role, so the role's grants apply as they do in production.
func pool(t *testing.T) *pgxpool.Pool {
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

// superuser returns a connection holding every privilege, to write what the UI's setups write and to
// break what the tests break.
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

// reset empties the tables the context reads. Every test of the package shares one database.
func reset(t *testing.T, conn *pgx.Conn) {
	t.Helper()
	for _, table := range []string{"rate_grants", "rate_state", "account_state", "policy_rules", "accounts", "oauth_clients"} {
		must(t, conn, "DELETE FROM "+table)
	}
}

// revoke revokes a grant of role for the rest of the test, and grants it again when the test ends.
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

// keyring returns a keyring and the public key sealing to it.
func keyring(t *testing.T) (*open.Keyring, seal.PublicKey) {
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

// connected writes a connected account of a provider that authenticates without an OAuth client.
func connected(t *testing.T, conn *pgx.Conn, public seal.PublicKey, id string) {
	t.Helper()
	credential, err := public.Seal([]byte(id+"-token"), seal.AccountCredential(id))
	if err != nil {
		t.Fatal(err)
	}
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", id)
	must(t, conn, "INSERT INTO account_state (account_id, credential) VALUES ($1, $2)", id, credential)
}

// newContext returns an execution context over pool opening with ring, whose observers record in
// observed each reload they are told of.
func newContext(t *testing.T, ring *open.Keyring, observed *[]string) *assembly.Context {
	t.Helper()
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	ec, err := assembly.New(assembly.Needs{
		Pool: pool(t), Keys: ring, Scanner: s, Lookups: classify.Lookups{
			ToUnicode: func(s string) (string, error) { return s, nil }, ToASCII: func(s string) (string, error) { return s, nil },
			Registrable: func(s string) (string, error) { return s, nil },
		},
		ReloadInterval: time.Hour, Registerer: prometheus.NewRegistry(), Logger: slog.New(slog.DiscardHandler),
	})
	if err != nil {
		t.Fatal(err)
	}
	ec.OnReload(func(context.Context) {
		*observed = append(*observed, strings.Join(ids(ec.Snapshot()), ","))
	})
	return ec
}

func ids(s *accountload.Snapshot) []string {
	var out []string
	for _, a := range s.Accounts() {
		out = append(out, a.ID())
	}
	return out
}

// Each account spends under the lowered target its state row set when the context last reloaded, and
// every other account under the default target, so a limiter built for a unit of work spends at the
// target the operator set (ADR-0024).
func TestTheLimiterSpendsUnderTheStoredTarget(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keyring(t)
	connected(t, conn, public, "lowered")
	connected(t, conn, public, "default")
	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.2 WHERE account_id = 'lowered'")
	var observed []string
	ec := newContext(t, ring, &observed)
	if err := ec.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	got := map[string]float64{}
	for _, a := range []string{"default", "lowered"} {
		limiter, err := ec.Limiter(pool(t), a, ceiling, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := limiter.Acquire(t.Context(), a, ratecore.Batch, 1); err != nil {
			t.Fatal(err)
		}
		var target float64
		if err := conn.QueryRow(t.Context(), "SELECT target_rate FROM rate_state WHERE account_id = $1", a).Scan(&target); err != nil {
			t.Fatal(err)
		}
		got[a] = target
	}
	if diff := cmp.Diff(map[string]float64{"default": 50, "lowered": 20}, got, compare.Options); diff != "" {
		t.Errorf("the targets each account's limiter spends under, in Gmail's units a second (-want +got):\n%s", diff)
	}
}

// A stored target above half the ceiling refuses the account's limiter, naming the account, so no unit
// of work spends above the target an operator may only lower, and every other account's limiter is
// built all the same (ADR-0024).
func TestAStoredTargetAboveHalfTheCeilingStopsTheAccountsWork(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keyring(t)
	connected(t, conn, public, "raised")
	connected(t, conn, public, "other")
	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.6 WHERE account_id = 'raised'")
	var observed []string
	ec := newContext(t, ring, &observed)
	if err := ec.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	_, err := ec.Limiter(pool(t), "raised", ceiling, nil)
	if !errors.Is(err, ratecore.ErrTargetOutOfRange) || !strings.Contains(err.Error(), `"raised"`) {
		t.Errorf("the raised account's limiter returned %v, want the target refused for the account", err)
	}
	if _, err := ec.Limiter(pool(t), "other", ceiling, nil); err != nil {
		t.Errorf("the other account's limiter returned %v, want it built", err)
	}
}

// A reload whose read of the accounts fails keeps in force what the reload before it read and tells
// no observer. One whose policy load fails swaps its accounts in all the same, tells the observers and
// returns an error wrapping ErrPolicyNotLoaded, and the policy in force stays the one read before
// (ADR-0090, ADR-0041).
func TestAFailedReloadKeepsWhatItCouldNotRead(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keyring(t)
	connected(t, conn, public, "personal")
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES (NULL, 'base.bank', 'restricted', ARRAY['bank.example'], 'operator', 'test')`)
	var observed []string
	ec := newContext(t, ring, &observed)
	if !errors.Is(ec.Latest().Err(), assembly.ErrNotLoaded) || ec.Latest().Loaded() {
		t.Fatalf("before any reload the context reads %v, want not loaded", ec.Latest().Err())
	}
	if err := ec.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if !ec.PolicySnapshot().Loaded() {
		t.Fatal("the reload loaded no policy")
	}

	connected(t, conn, public, "work")
	revoke(t, conn, "SELECT ON accounts")
	err := ec.Reload(t.Context())
	if !errors.Is(err, accountload.ErrUntrustedRead) || errors.Is(err, assembly.ErrPolicyNotLoaded) {
		t.Errorf("a reload whose read failed returned %v, want the failed read", err)
	}
	if got := ids(ec.Snapshot()); !slices.Equal(got, []string{"personal"}) || !ec.Latest().Loaded() {
		t.Errorf("after the failed read the context serves %v, want the accounts read before", got)
	}

	must(t, conn, "GRANT SELECT ON accounts TO "+role)
	revoke(t, conn, "SELECT ON policy_rules")
	err = ec.Reload(t.Context())
	if !errors.Is(err, assembly.ErrPolicyNotLoaded) || !errors.Is(err, policyload.ErrUntrustedRead) {
		t.Errorf("a reload whose policy read failed returned %v, want the policy not loaded", err)
	}
	if got := ids(ec.Snapshot()); !slices.Equal(got, []string{"personal", "work"}) {
		t.Errorf("after the policy's failure the context serves %v, want the accounts the reload read", got)
	}
	if !errors.Is(ec.Latest().Err(), assembly.ErrPolicyNotLoaded) {
		t.Errorf("the latest reload reads %v, want the policy not loaded", ec.Latest().Err())
	}
	if ec.Policy("personal").Composed().RestrictsAll() {
		t.Error("the account read before lost its policy to a reload whose policy read failed")
	}
	if diff := cmp.Diff([]string{"personal", "personal,work"}, observed, compare.Options); diff != "" {
		t.Errorf("the reloads the observers were told of (-want +got):\n%s", diff)
	}
}

// Run loads at once when no load has succeeded, so a process with no reload of its own serves its
// accounts from its start, and stops when its context ends.
func TestRunLoadsAtOnce(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keyring(t)
	connected(t, conn, public, "personal")
	var observed []string
	ec := newContext(t, ring, &observed)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- ec.Run(ctx) }()
	for deadline := time.Now().Add(10 * time.Second); !ec.Latest().Loaded(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("Run did not load within ten seconds of its start, with an hour's reload interval")
		}
	}
	cancel()
	if err := <-done; err != nil {
		t.Errorf("Run returned %v once its context ended, want nil", err)
	}
}

// targetRate returns the target an account's rate state was last written at, in Gmail's units a
// second, by a limiter that spent for it.
func targetRate(t *testing.T, conn *pgx.Conn, account string) float64 {
	t.Helper()
	var target float64
	if err := conn.QueryRow(t.Context(), "SELECT target_rate FROM rate_state WHERE account_id = $1", account).Scan(&target); err != nil {
		t.Fatal(err)
	}
	return target
}

// A lowered target the operator changes while a process runs applies from the context's next reload,
// so a unit of work after that reload spends under the new target, the account's earlier one dropped
// (ADR-0024, ADR-0090).
func TestALoweredTargetChangedBetweenReloadsAppliesFromTheNext(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keyring(t)
	connected(t, conn, public, "lowered")
	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.2 WHERE account_id = 'lowered'")
	var observed []string
	ec := newContext(t, ring, &observed)
	spend := func() float64 {
		t.Helper()
		if err := ec.Reload(t.Context()); err != nil {
			t.Fatal(err)
		}
		limiter, err := ec.Limiter(pool(t), "lowered", ceiling, nil)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := limiter.Acquire(t.Context(), "lowered", ratecore.Batch, 1); err != nil {
			t.Fatal(err)
		}
		// The fraction is stored as a real, so the target is compared to a thousandth of a unit.
		return math.Round(targetRate(t, conn, "lowered")*1000) / 1000
	}
	if got := spend(); got != 20 {
		t.Fatalf("the first reload's limiter spends under %v, want the stored 20", got)
	}
	must(t, conn, "UPDATE account_state SET lowered_target_rate = 0.3 WHERE account_id = 'lowered'")
	if got := spend(); got != 30 {
		t.Errorf("after the target was raised to 0.3 the next reload's limiter spends under %v, want 30", got)
	}
	must(t, conn, "UPDATE account_state SET lowered_target_rate = NULL WHERE account_id = 'lowered'")
	if got := spend(); got != 50 {
		t.Errorf("after the target was cleared the next reload's limiter spends under %v, want the default 50", got)
	}
}

// A process whose database does not answer its first load keeps trying, every few seconds rather than
// once a reload interval, so it serves its accounts soon after the database answers (ADR-0090).
func TestRunRetriesItsFirstLoadUntilItSucceeds(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keyring(t)
	connected(t, conn, public, "personal")
	revoke(t, conn, "SELECT ON accounts")
	var observed []string
	ec := newContext(t, ring, &observed)
	ctx, cancel := context.WithCancel(t.Context())
	done := make(chan error, 1)
	go func() { done <- ec.Run(ctx) }()
	defer func() {
		cancel()
		if err := <-done; err != nil {
			t.Error(err)
		}
	}()
	for deadline := time.Now().Add(10 * time.Second); errors.Is(ec.Latest().Err(), assembly.ErrNotLoaded); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("Run did not try its first load within ten seconds")
		}
	}
	if ec.Latest().Loaded() {
		t.Fatal("the first load succeeded with the listing refused")
	}
	must(t, conn, "GRANT SELECT ON accounts TO "+role)
	for deadline := time.Now().Add(10 * time.Second); !ec.Latest().Loaded(); time.Sleep(10 * time.Millisecond) {
		if time.Now().After(deadline) {
			t.Fatal("Run did not load within ten seconds of the database answering, with an hour's reload interval")
		}
	}
}

// reloadsTimed returns how many reloads the context's reload duration series on reg has observed.
func reloadsTimed(t *testing.T, reg prometheus.Gatherer) uint64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == "mediated_mailbox_executioncontext_reload_duration_seconds" {
			return f.GetMetric()[0].GetHistogram().GetSampleCount()
		}
	}
	t.Fatal("the reload duration series is not registered")
	return 0
}

// Every reload its caller did not cancel is timed, one that succeeds and one whose read fails, and a
// reload its caller cancelled is not (ADR-0125).
func TestEachReloadItsCallerDidNotCancelIsTimed(t *testing.T) {
	conn := superuser(t)
	reset(t, conn)
	ring, public := keyring(t)
	connected(t, conn, public, "personal")
	s, err := scan.New(scan.DefaultConfig(), "a-revision")
	if err != nil {
		t.Fatal(err)
	}
	reg := prometheus.NewRegistry()
	ec, err := assembly.New(assembly.Needs{
		Pool: pool(t), Keys: ring, Scanner: s, ReloadInterval: time.Hour, Registerer: reg, Logger: slog.New(slog.DiscardHandler),
		Lookups: classify.Lookups{
			ToUnicode: func(s string) (string, error) { return s, nil }, ToASCII: func(s string) (string, error) { return s, nil },
			Registrable: func(s string) (string, error) { return s, nil },
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := ec.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	revoke(t, conn, "SELECT ON accounts")
	if err := ec.Reload(t.Context()); err == nil {
		t.Fatal("a reload with the listing refused succeeded")
	}
	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	if err := ec.Reload(cancelled); !errors.Is(err, context.Canceled) {
		t.Fatalf("a cancelled reload returned %v, want context.Canceled", err)
	}
	if got := reloadsTimed(t, reg); got != 2 {
		t.Errorf("reloads timed = %d, want 2, the one that succeeded and the one whose read failed", got)
	}
}
