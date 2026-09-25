//go:build integration

package policyload_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// grantRead is backfill's grant on the policy rules, as the migration chain gives it.
const grantRead = "GRANT SELECT (account_id, rule_id, class, domain_suffix) ON policy_rules TO mediated_mailbox_backfill"

// newLoader returns a loader for accounts over db and the registry its series is on.
func newLoader(t *testing.T, db *faulty, accounts []string) (*policyload.Loader, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	l, err := policyload.New(db, accounts, reg)
	if err != nil {
		t.Fatal(err)
	}
	return l, reg
}

// seeded is each account's view of the rules seed writes.
func seeded(accounts []string) map[string]view {
	out := map[string]view{}
	for _, account := range accounts {
		out[account] = view{Rules: map[string][]string{
			"base.fidelity":     {"fidelity.com", "fmr.com"},
			"base.irs":          {"irs.gov"},
			account + ".bank":   {"examplebank.com"},
			account + ".clinic": {"exampleclinic.org"},
		}}
	}
	return out
}

// Each account's policy is the base rules with its own, and never another account's (ADR-0026).
// Before the first reload, and for an account the loader does not read, every sender is restricted,
// because absence denies (ADR-0041). A snapshot a caller took keeps its policy after a later reload.
// The accounts are given out of order and one twice. The loader reads each once, since a second read
// would repeat its rules and fail every reload, and finds each whatever the order it was given in.
func TestTheLoaderComposesEachAccountsPolicy(t *testing.T) {
	conn := superuser(t)
	accounts := newAccounts(t, conn, 3)
	read, unread := accounts[:2], accounts[2]
	seed(t, conn, accounts)
	l, _ := newLoader(t, &faulty{pool: backfill(t)}, []string{read[1], read[0], read[1]})

	restrictsAll := map[string]view{}
	for _, account := range accounts {
		restrictsAll[account] = view{RestrictsAll: true, Rules: map[string][]string{}}
	}
	if diff := cmp.Diff(restrictsAll, observe(l.Snapshot(), accounts...), compare.Options); diff != "" {
		t.Errorf("before the first reload (-want +got):\n%s", diff)
	}

	if err := l.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	taken := l.Snapshot()
	want := seeded(read)
	want[unread] = restrictsAll[unread]
	if diff := cmp.Diff(want, observe(taken, accounts...), compare.Options); diff != "" {
		t.Errorf("after a reload (-want +got):\n%s", diff)
	}
	if !taken.Loaded() {
		t.Error("the snapshot after a reload reports no valid load")
	}

	must(t, conn, "DELETE FROM policy_rules WHERE rule_id = 'base.irs'")
	if err := l.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	if diff := cmp.Diff(want, observe(taken, accounts...), compare.Options); diff != "" {
		t.Errorf("the snapshot taken before a later reload (-want +got):\n%s", diff)
	}
	if _, kept := observe(l.Snapshot(), read[0])[read[0]].Rules["base.irs"]; kept {
		t.Error("the active snapshot still holds a base rule deleted before the last reload")
	}
}

// A read of the policy tables that fails, before any row or partway, never replaces the active
// policy, and the alarm's series reads 1 until a reload succeeds (ADR-0041, ADR-0077). Each fault
// leaves rows in hand that would validate, none at all or a part of the policy, so only telling the
// failed read apart from a whole one keeps the active policy. The second account is read after the
// first, so a fault there comes after a read that completed.
func TestAFailedReadNeverReplacesTheActivePolicy(t *testing.T) {
	cut := func(account, rows int) func(*testing.T, *pgx.Conn, *faulty, []string) func() int64 {
		return func(_ *testing.T, _ *pgx.Conn, db *faulty, accounts []string) func() int64 {
			f := &fault{account: accounts[account], rows: rows}
			db.fault.Store(f)
			return f.fired.Load
		}
	}
	cases := []struct {
		name string
		// breakRead breaks the next reads for the accounts. It returns a count of the reads it cut
		// short, or nil when the database itself fails the read.
		breakRead func(t *testing.T, conn *pgx.Conn, db *faulty, accounts []string) func() int64
		// mend undoes what breakRead did in the database, so a later reload succeeds.
		mend func(t *testing.T, conn *pgx.Conn, accounts []string)
	}{
		{name: "the first account's read fails before any row", breakRead: cut(0, 0)},
		{name: "the first account's read fails after one row", breakRead: cut(0, 1)},
		{name: "the second account's read fails before any row", breakRead: cut(1, 0)},
		{name: "the second account's read fails after one row", breakRead: cut(1, 1)},
		{
			// A base rule written after the first account's read and before the second's would
			// compose the two accounts from different base policies.
			name: "a base rule is added between two accounts' reads",
			breakRead: func(t *testing.T, conn *pgx.Conn, db *faulty, accounts []string) func() int64 {
				f := &fault{account: accounts[1], before: func() { addRule(t, conn, "", "base.late", "late.example") }}
				db.fault.Store(f)
				return f.fired.Load
			},
			mend: func(t *testing.T, conn *pgx.Conn, _ []string) {
				must(t, conn, "DELETE FROM policy_rules WHERE rule_id = 'base.late'")
			},
		},
		{
			name: "the role cannot read the table",
			breakRead: func(t *testing.T, conn *pgx.Conn, _ *faulty, _ []string) func() int64 {
				must(t, conn, "REVOKE SELECT ON policy_rules FROM mediated_mailbox_backfill")
				t.Cleanup(func() {
					if _, err := conn.Exec(context.Background(), grantRead); err != nil {
						t.Error(err)
					}
				})
				return nil
			},
			mend: func(t *testing.T, conn *pgx.Conn, _ []string) {
				must(t, conn, grantRead)
			},
		},
		{
			// The rules are read in the order of their identifiers, so this one comes after the
			// account's other rules, and the reader cannot decode a null domain suffix.
			name: "an account's rule cannot be decoded, after its other rules",
			breakRead: func(t *testing.T, conn *pgx.Conn, _ *faulty, accounts []string) func() int64 {
				must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
					VALUES ($1, $1 || '.null', 'restricted', ARRAY['examplenull.com', NULL]::text[], 'operator', 'test')`, accounts[1])
				return nil
			},
			mend: func(t *testing.T, conn *pgx.Conn, accounts []string) {
				must(t, conn, "DELETE FROM policy_rules WHERE rule_id = $1", accounts[1]+".null")
			},
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			conn := superuser(t)
			accounts := newAccounts(t, conn, 2)
			seed(t, conn, accounts)
			db := &faulty{pool: backfill(t)}
			l, reg := newLoader(t, db, accounts)
			if err := l.Reload(t.Context()); err != nil {
				t.Fatal(err)
			}
			want := seeded(accounts)

			fired := c.breakRead(t, conn, db, accounts)
			err := l.Reload(t.Context())
			t.Logf("the reload over the failed read returned %v", err)
			if !errors.Is(err, policyload.ErrUntrustedRead) {
				t.Errorf("a reload over a failed read returned %v, want an error wrapping ErrUntrustedRead", err)
			}
			if fired != nil && fired() != 1 {
				t.Fatalf("the fault cut %d reads short, want one", fired())
			}
			if diff := cmp.Diff(want, observe(l.Snapshot(), accounts...), compare.Options); diff != "" {
				t.Errorf("the active policy after a failed read (-want +got):\n%s", diff)
			}
			if got := reloadFailed(t, reg); got != 1 {
				t.Errorf("%s after a failed read = %v, want 1", reloadFailedName, got)
			}

			db.fault.Store(nil)
			if c.mend != nil {
				c.mend(t, conn, accounts)
			}
			if err := l.Reload(t.Context()); err != nil {
				t.Fatalf("a reload after the fault was undone: %v", err)
			}
			if got := reloadFailed(t, reg); got != 0 {
				t.Errorf("%s after a reload succeeded = %v, want 0", reloadFailedName, got)
			}
		})
	}
}

// A policy with no rules, read whole, is the policy, and it replaces the active one (ADR-0041). It
// is what a failed read would look like if the loader went by the rows alone, so it shows that the
// loader refuses a failed read for failing and not for being empty.
func TestAnEmptyPolicyReadWholeIsAccepted(t *testing.T) {
	conn := superuser(t)
	accounts := newAccounts(t, conn, 2)
	seed(t, conn, accounts)
	l, reg := newLoader(t, &faulty{pool: backfill(t)}, accounts)
	if err := l.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}

	must(t, conn, "DELETE FROM policy_rules")
	if err := l.Reload(t.Context()); err != nil {
		t.Fatalf("a reload of an empty policy: %v", err)
	}
	want := map[string]view{}
	for _, account := range accounts {
		want[account] = view{Rules: map[string][]string{}}
	}
	if diff := cmp.Diff(want, observe(l.Snapshot(), accounts...), compare.Options); diff != "" {
		t.Errorf("the active policy after an empty policy was read whole (-want +got):\n%s", diff)
	}
	if got := reloadFailed(t, reg); got != 0 {
		t.Errorf("%s after an empty policy was read whole = %v, want 0", reloadFailedName, got)
	}
}

// An update to the policy tables that does not validate never takes effect. The active policy
// keeps serving, and the alarm's series reads 1 until an update validates (ADR-0041, ADR-0077).
func TestAnInvalidUpdateNeverDisplacesTheActivePolicy(t *testing.T) {
	conn := superuser(t)
	accounts := newAccounts(t, conn, 2)
	seed(t, conn, accounts)
	l, reg := newLoader(t, &faulty{pool: backfill(t)}, accounts)
	if err := l.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := seeded(accounts)

	addRule(t, conn, "", "base.typo", "example com")
	err := l.Reload(t.Context())
	if !errors.Is(err, policyload.ErrInvalidUpdate) {
		t.Errorf("a reload of an invalid update returned %v, want an error wrapping ErrInvalidUpdate", err)
	}
	if diff := cmp.Diff(want, observe(l.Snapshot(), accounts...), compare.Options); diff != "" {
		t.Errorf("the active policy after an invalid update (-want +got):\n%s", diff)
	}
	if got := reloadFailed(t, reg); got != 1 {
		t.Errorf("%s after an invalid update = %v, want 1", reloadFailedName, got)
	}

	must(t, conn, "UPDATE policy_rules SET domain_suffix = ARRAY['example.com'] WHERE rule_id = 'base.typo'")
	if err := l.Reload(t.Context()); err != nil {
		t.Fatalf("a reload after the update was fixed: %v", err)
	}
	for _, account := range accounts {
		want[account].Rules["base.typo"] = []string{"example.com"}
	}
	if diff := cmp.Diff(want, observe(l.Snapshot(), accounts...), compare.Options); diff != "" {
		t.Errorf("the active policy after the update was fixed (-want +got):\n%s", diff)
	}
	if got := reloadFailed(t, reg); got != 0 {
		t.Errorf("%s after the update was fixed = %v, want 0", reloadFailedName, got)
	}
}

// A reload its caller cancelled keeps the active policy and raises no alarm, so a process shutting
// down mid-reload never pages. A reload whose deadline passed is a failed read, so reloads that keep
// timing out against a locked table or a stalled database are loud, and it keeps the active policy
// too (ADR-0041, ADR-0077).
func TestAReloadItsCallerCancelledRaisesNoAlarm(t *testing.T) {
	conn := superuser(t)
	accounts := newAccounts(t, conn, 2)
	seed(t, conn, accounts)
	l, reg := newLoader(t, &faulty{pool: backfill(t)}, accounts)
	if err := l.Reload(t.Context()); err != nil {
		t.Fatal(err)
	}
	want := seeded(accounts)

	cancelled, cancel := context.WithCancel(t.Context())
	cancel()
	err := l.Reload(cancelled)
	if !errors.Is(err, context.Canceled) || errors.Is(err, policyload.ErrUntrustedRead) {
		t.Errorf("a cancelled reload returned %v, want an error wrapping context.Canceled and not ErrUntrustedRead", err)
	}
	if diff := cmp.Diff(want, observe(l.Snapshot(), accounts...), compare.Options); diff != "" {
		t.Errorf("the active policy after a cancelled reload (-want +got):\n%s", diff)
	}
	if got := reloadFailed(t, reg); got != 0 {
		t.Errorf("%s after a cancelled reload = %v, want 0", reloadFailedName, got)
	}

	expired, stop := context.WithDeadline(t.Context(), time.Now().Add(-time.Second))
	defer stop()
	if err := l.Reload(expired); !errors.Is(err, policyload.ErrUntrustedRead) {
		t.Errorf("a reload past its deadline returned %v, want an error wrapping ErrUntrustedRead", err)
	}
	if diff := cmp.Diff(want, observe(l.Snapshot(), accounts...), compare.Options); diff != "" {
		t.Errorf("the active policy after a reload past its deadline (-want +got):\n%s", diff)
	}
	if got := reloadFailed(t, reg); got != 1 {
		t.Errorf("%s after a reload past its deadline = %v, want 1", reloadFailedName, got)
	}
}
