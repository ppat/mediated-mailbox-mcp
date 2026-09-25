//go:build integration

package policyload_test

import (
	"context"
	"crypto/rand"
	"errors"
	"regexp"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/policyload"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// reloadFailedName is the series the reload-failure alarm reads.
const reloadFailedName = "mediated_mailbox_policyload_reload_failed"

// backfill returns a pool connecting as backfill's role, the first deployable that loads policy.
// Row-level security applies to it, as it does in production.
func backfill(t *testing.T) *pgxpool.Pool {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.RuntimeParams["options"] = "-c role=mediated_mailbox_backfill"
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

// superuser returns a connection that bypasses row-level security, to write the policy tables and
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

// must runs a statement and fails the test on an error.
func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

var unsafeName = regexp.MustCompile(`[^a-z0-9]+`)

// newAccounts creates n accounts named for the test and returns them in the order the loader reads
// them, so the first reads the base rules. It first deletes every policy rule, because the base rules
// are shared by every account in the package's database.
func newAccounts(t *testing.T, conn *pgx.Conn, n int) []string {
	t.Helper()
	must(t, conn, "DELETE FROM policy_rules")
	prefix := "acct-" + strings.Trim(unsafeName.ReplaceAllString(strings.ToLower(t.Name()), "-"), "-") +
		"-" + strings.ToLower(rand.Text()[:8])
	var accounts []string
	for i := range n {
		account := prefix + "-" + string(rune('a'+i))
		must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", account)
		accounts = append(accounts, account)
	}
	return accounts
}

// addRule writes one restricted rule. An empty account writes a base rule.
func addRule(t *testing.T, conn *pgx.Conn, account, id string, suffixes ...string) {
	t.Helper()
	var owner any
	if account != "" {
		owner = account
	}
	must(t, conn, `INSERT INTO policy_rules (account_id, rule_id, class, domain_suffix, source, created_by)
		VALUES ($1, $2, 'restricted', $3, 'operator', 'test')`, owner, id, suffixes)
}

// seed writes two base rules and two rules of each account's own.
func seed(t *testing.T, conn *pgx.Conn, accounts []string) {
	t.Helper()
	addRule(t, conn, "", "base.fidelity", "fidelity.com", "fmr.com")
	addRule(t, conn, "", "base.irs", "irs.gov")
	for _, account := range accounts {
		addRule(t, conn, account, account+".bank", "examplebank.com")
		addRule(t, conn, account, account+".clinic", "exampleclinic.org")
	}
}

// view is what a test can observe of one account's policy in a snapshot.
type view struct {
	RestrictsAll bool
	Rules        map[string][]string
}

// observe returns each account's view of s.
func observe(s policyload.Snapshot, accounts ...string) map[string]view {
	out := map[string]view{}
	for _, account := range accounts {
		c := s.For(account)
		v := view{RestrictsAll: c.RestrictsAll(), Rules: map[string][]string{}}
		for _, r := range c.Rules() {
			v.Rules[r.ID()] = r.DomainSuffixes()
		}
		out[account] = v
	}
	return out
}

// reloadFailed returns the value of the reload-failure series on reg.
func reloadFailed(t *testing.T, reg prometheus.Gatherer) float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range families {
		if f.GetName() == reloadFailedName {
			if n := len(f.GetMetric()); n != 1 {
				t.Fatalf("%s has %d series, want one", reloadFailedName, n)
			}
			return f.GetMetric()[0].GetGauge().GetValue()
		}
	}
	t.Fatalf("%s is not registered", reloadFailedName)
	return 0
}

// errInjected is the error a cut read returns, standing in for a connection lost mid-read.
var errInjected = errors.New("the read was cut short")

// fault cuts one account's read of the policy rules short. The read returns the first rows of the
// statement's result and then fails, as a read whose connection is lost partway does. With no rows
// left to cut, before runs instead, just ahead of the account's read, and the read is left whole.
type fault struct {
	// account names the account whose read fails.
	account string
	// rows is how many rows the read returns before it fails.
	rows int
	// before, when set, runs ahead of the account's read in place of cutting it.
	before func()
	// fired counts the reads the fault cut short or ran before, so a test can show its fault happened.
	fired atomic.Int64
}

// faulty opens transactions on a pool and cuts short the read its fault names, while fault is set.
type faulty struct {
	pool  *pgxpool.Pool
	fault atomic.Pointer[fault]
}

func (f *faulty) Begin(ctx context.Context) (pgx.Tx, error) {
	t, err := f.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &faultyTx{Tx: t, fault: f.fault.Load()}, nil
}

type faultyTx struct {
	pgx.Tx
	fault *fault
}

func (t *faultyTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	f := t.fault
	targeted := f != nil && strings.HasPrefix(sql, "-- name: PolicyRules ") && len(args) > 0 && args[0] == f.account
	if targeted && f.before != nil {
		f.before()
		f.fired.Add(1)
	}
	rows, err := t.Tx.Query(ctx, sql, args...)
	if err != nil || !targeted || f.before != nil {
		return rows, err
	}
	return &cutRows{Rows: rows, left: f.rows, fault: f}, nil
}

// cutRows returns left rows and then ends with errInjected.
type cutRows struct {
	pgx.Rows
	left  int
	cut   bool
	fault *fault
}

func (r *cutRows) Next() bool {
	if r.left == 0 {
		if !r.cut {
			r.cut = true
			r.fault.fired.Add(1)
			r.Close()
		}
		return false
	}
	r.left--
	return r.Rows.Next()
}

func (r *cutRows) Err() error {
	if r.cut {
		return errInjected
	}
	return r.Rows.Err()
}
