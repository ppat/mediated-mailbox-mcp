//go:build integration

package lease_test

import (
	"context"
	"crypto/rand"
	"regexp"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// ceiling is the declared budget the tests issue under, Gmail's. The target is 50, the hard cap 80
// and the floor 5, and at the target interactive reserves 15, sync 10 and batch 25.
const ceiling = 100

// hardCap is one second's worth at the hard cap under ceiling.
const hardCap = 80

// spenders returns a pool connecting as backfill's role, one of the four whose lists admit the rate
// limiter. Row-level security applies to it, as it does in production.
func spenders(t *testing.T) *pgxpool.Pool {
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

// superuser returns a connection that bypasses row-level security, to seed and read what the tests
// set up and observe.
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

var unsafeName = regexp.MustCompile(`[^a-z0-9]+`)

// newAccount creates an account named for the test and returns it, so each test, and each run of it,
// has rate state of its own.
func newAccount(t *testing.T, conn *pgx.Conn) string {
	t.Helper()
	account := "acct-" + strings.Trim(unsafeName.ReplaceAllString(strings.ToLower(t.Name()), "-"), "-") +
		"-" + strings.ToLower(rand.Text()[:8])
	must(t, conn, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", account)
	return account
}

// must runs a statement and fails the test on an error.
func must(t *testing.T, conn *pgx.Conn, sql string, args ...any) {
	t.Helper()
	if _, err := conn.Exec(t.Context(), sql, args...); err != nil {
		t.Fatalf("%s: %v", sql, err)
	}
}

// issued is the instant a lease was issued at, by the database's clock.
func issued(l core.Lease) int64 { return l.Expires - 1000 }

// busiestSecond returns the most tokens the leases hold whose issue instants fall inside any one
// second, the window each grant is judged in, from one second before it up to it.
func busiestSecond(leases []core.Lease) float64 {
	most := 0.0
	for _, l := range leases {
		sum := 0.0
		for _, other := range leases {
			if at := issued(other); at > issued(l)-1000 && at <= issued(l) {
				sum += other.Tokens
			}
		}
		most = max(most, sum)
	}
	return most
}

// ledger collects the leases workers are granted.
type ledger struct {
	mu     sync.Mutex
	leases []core.Lease
}

func (g *ledger) add(l core.Lease) {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.leases = append(g.leases, l)
}

func (g *ledger) all() []core.Lease {
	g.mu.Lock()
	defer g.mu.Unlock()
	return slices.Clone(g.leases)
}

// tokensIssuedBetween sums the leases issued in [from, to).
func tokensIssuedBetween(leases []core.Lease, from, to int64) float64 {
	sum := 0.0
	for _, l := range leases {
		if at := issued(l); at >= from && at < to {
			sum += l.Tokens
		}
	}
	return sum
}

// clock reads the database's clock in Unix milliseconds.
func clock(t *testing.T, conn *pgx.Conn) int64 {
	t.Helper()
	var now int64
	if err := conn.QueryRow(t.Context(), "SELECT floor(extract(EPOCH FROM clock_timestamp()) * 1000)::bigint").Scan(&now); err != nil {
		t.Fatal(err)
	}
	return now
}

// until runs work in n goroutines until the deadline, and waits for them.
func until(deadline time.Duration, n int, work func(ctx context.Context)) {
	ctx, cancel := context.WithTimeout(context.Background(), deadline)
	defer cancel()
	var wg sync.WaitGroup
	for range n {
		wg.Go(func() { work(ctx) })
	}
	wg.Wait()
}
