//go:build integration

package api_test

import (
	"context"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

// lifter lifts the bank's rule from a connection of its own, committed, as the request reaches its
// second read of the account's policy, so a read that does not see one state would classify the
// bank's sender under two policies in one response.
type lifter struct {
	t     *testing.T
	conn  *pgx.Conn
	reads atomic.Int64
}

func (l *lifter) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if strings.Contains(data.SQL, "-- name: ComposedRules ") && l.reads.Add(1) == 2 {
		if _, err := l.conn.Exec(context.Background(), "DELETE FROM policy_rules WHERE rule_id = 'rule.bank'"); err != nil {
			l.t.Error(err)
		}
	}
	return ctx
}

func (l *lifter) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// M8's row for the UI's figures and rows classified under the policy in force, within one response.
// The bank's rule is lifted and committed between the failures dataset's two reads of the policy, its
// figures' and its rows'. The response's total and its rows still classify the bank's sender the same
// way, under the one snapshot the response reads, so the total's restricted count is the number of its
// rows that read restricted (ADR-0136).
func TestOneResponseClassifiesUnderOnePolicy(t *testing.T) {
	seed(t)
	ctx := t.Context()
	admin, err := pgx.Connect(ctx, postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := admin.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	l := &lifter{t: t, conn: admin}
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	cfg.ConnConfig.Tracer = l
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE mediated_mailbox_ui")
		return err
	}
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	s, _ := serverAs(t, pool)

	var page struct {
		Total struct {
			Restricted int64 `json:"restricted"`
		} `json:"total"`
		Rows []struct {
			FromEmail   *string `json:"from_email"`
			SenderClass *string `json:"sender_class"`
		} `json:"rows"`
	}
	read(t, s.Handler(), "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=3", &page)
	if n := l.reads.Load(); n < 2 {
		t.Fatalf("the response read the policy %d times, so the rule was never lifted inside it", n)
	}
	var restricted int64
	for _, row := range page.Rows {
		if row.SenderClass != nil && *row.SenderClass == "restricted" {
			restricted++
		}
	}
	if restricted != page.Total.Restricted || restricted != 1 {
		t.Errorf("the response's total counts %d restricted and its rows read %d, want the bank's one in both", page.Total.Restricted, restricted)
	}
}

// isolationProbe is the UI's database with every transaction it begins watched. It counts the
// transactions begun, those begun repeatable read apart, and each statement that reads the policy or
// the senders a response classifies, by whether its transaction was begun repeatable read. A
// response that begins one transaction, at repeatable read, reads one snapshot.
type isolationProbe struct {
	*pgxpool.Pool
	snapshot, plain     atomic.Int64
	begun, beginsRepeat atomic.Int64
}

func (p *isolationProbe) Begin(ctx context.Context) (pgx.Tx, error) {
	p.begun.Add(1)
	t, err := p.Pool.Begin(ctx)
	return &probedTx{Tx: t, probe: p}, err
}

func (p *isolationProbe) BeginTx(ctx context.Context, opts pgx.TxOptions) (pgx.Tx, error) {
	p.begun.Add(1)
	repeatable := opts.IsoLevel == pgx.RepeatableRead
	if repeatable {
		p.beginsRepeat.Add(1)
	}
	t, err := p.Pool.BeginTx(ctx, opts)
	return &probedTx{Tx: t, probe: p, repeatable: repeatable}, err
}

type probedTx struct {
	pgx.Tx
	probe      *isolationProbe
	repeatable bool
}

// classifying reports whether a statement reads what a response classifies a sender under.
func classifying(sql string) bool {
	return strings.Contains(sql, "-- name: ComposedRules ") || strings.Contains(sql, "-- name: SenderCounts ")
}

func (t *probedTx) count(sql string) {
	if !classifying(sql) {
		return
	}
	if t.repeatable {
		t.probe.snapshot.Add(1)
	} else {
		t.probe.plain.Add(1)
	}
}

func (t *probedTx) Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error) {
	t.count(sql)
	return t.Tx.Exec(ctx, sql, args...)
}

func (t *probedTx) Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error) {
	t.count(sql)
	return t.Tx.Query(ctx, sql, args...)
}

func (t *probedTx) QueryRow(ctx context.Context, sql string, args ...any) pgx.Row {
	t.count(sql)
	return t.Tx.QueryRow(ctx, sql, args...)
}

// M8's row for the UI's figures and rows classified under the policy in force, over every response
// that classifies. Each of them begins exactly one transaction, at repeatable read, and reads the
// policy and the senders it classifies in it, so each reads one snapshot, as the failures dataset's
// response is shown to above (ADR-0136). Each case must read the classifying statements at least
// once, so none passes by reading nothing.
func TestEveryClassifyingResponseBeginsOneSnapshot(t *testing.T) {
	r := newRig(t)
	r.seedPolicy()
	var probe *isolationProbe
	s := r.serverOver(tokenKey, func(pool *pgxpool.Pool) api.Database {
		probe = &isolationProbe{Pool: pool}
		return probe
	})
	b := r.browser().on(s)
	cases := []struct {
		name string
		do   func() response
	}{
		{"corpus", func() response { return b.get("/api/personal/corpus") }},
		{"failures figures", func() response { return b.get("/api/personal/lens?dataset=failures&run=r-0912&level=0") }},
		{"failures groups", func() response {
			return b.get("/api/personal/lens?dataset=failures&run=r-0912&level=1&group=sender")
		}},
		{"failures rows", func() response { return b.get("/api/personal/lens?dataset=failures&run=r-0912&level=3") }},
		{"failure detail", func() response { return b.get("/api/personal/failures/1?run=r-0912") }},
		{"rules figures", func() response { return b.get("/api/personal/lens?dataset=rules&level=0&range=all") }},
		{"rules rows", func() response { return b.get("/api/personal/lens?dataset=rules&level=3&range=all") }},
		{"rule detail", func() response { return b.get("/api/personal/rules/base%3Abase.bank") }},
		{"sender picker", func() response {
			return b.get("/api/personal/lens?dataset=senders&level=3&sort=message_count,desc&page=1")
		}},
		{"match", func() response { return b.get("/api/personal/policy/match?suffix=lender.example") }},
		{"release", func() response { return b.get("/api/personal/policy/release?scope=base&rule=base.bank") }},
		{"import preview", func() response {
			return b.post("/api/personal/policy/import/preview", map[string]any{"file": ownFile})
		}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			probe.snapshot.Store(0)
			probe.plain.Store(0)
			probe.begun.Store(0)
			probe.beginsRepeat.Store(0)
			if res := c.do(); res.status != http.StatusOK {
				t.Fatalf("answered %d: %s", res.status, res.body)
			}
			if n := probe.plain.Load(); n != 0 {
				t.Errorf("%d classifying reads ran outside a repeatable-read transaction", n)
			}
			if probe.snapshot.Load() == 0 {
				t.Errorf("no classifying read ran, so the case proves nothing")
			}
			if begun, repeat := probe.begun.Load(), probe.beginsRepeat.Load(); begun != 1 || repeat != 1 {
				t.Errorf("the response began %d transactions, %d of them repeatable read, want one, repeatable read", begun, repeat)
			}
		})
	}
}
