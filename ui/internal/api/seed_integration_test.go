//go:build integration

package api_test

import (
	"context"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/fixture"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

func TestMain(m *testing.M) {
	postgres.Main(m)
}

// now is the server's clock in every test, so as_of and every relative range are fixed and the
// recorded fixtures do not move between runs.
var now = time.Date(2026, 9, 10, 10, 16, 4, 0, time.UTC)

// The two accounts. Every read below is for personal, and other holds rows of its own, each carrying
// the marker other, which no personal read may return.
const (
	personal = "personal"
	other    = "other"
)

// The seeded plans' identifiers.
const (
	draftPlan    = "7f3a9c00-0000-4000-8000-000000000001"
	applyingPlan = "7f3a9c00-0000-4000-8000-000000000002"
	appliedPlan  = "7f3a9c00-0000-4000-8000-000000000003"
	otherPlan    = "7f3a9c00-0000-4000-8000-000000000009"
)

// seed writes the synthetic fixture database once per test, as the superuser, which the policies do
// not confine. It restarts every serial column, so identities such as an audit row's are the same
// whichever tests ran before it and the recorded fixtures do not move. Message-derived text carries field markers, and the markup markers among them are the
// ones every surface must show inert (ADR-0044, ADR-0064). The plan description and operation reason
// are the client's text, and the run errors and the authentication outcome provider text, so they
// carry markers too.
func seed(t *testing.T) {
	t.Helper()
	ctx := t.Context()
	conn, err := pgx.Connect(ctx, postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	at := func(ago time.Duration) time.Time { return now.Add(-ago) }
	bank, newsletter := fixture.Bank(), fixture.Newsletter()
	statements := []struct {
		sql  string
		args []any
	}{
		{"TRUNCATE accounts, account_state, rate_state, senders, messages, scan_gate_decisions, policy_candidates, masking_events, reorg_plans, reorg_plan_ops, reorg_op_log, job_runs, job_run_events, job_run_failures, audit_log RESTART IDENTITY CASCADE", nil},
		{"INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail'), ($2, 'gmail')", []any{personal, other}},
		{`INSERT INTO account_state (account_id, credential, backfill_pass1_complete, backfill_pass2_complete, sync_cursor, sync_cursor_at, last_auth_at, last_auth_outcome)
			VALUES ($1, '\x00', true, false, 'cursor-1', $2, $3, $4)`, []any{personal, at(4 * time.Minute), at(6 * time.Hour), marker.MarkupField("authoutcome")}},
		{
			`INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, last_throttle_at, backoff_until, classes)
			VALUES ($1, 3.1, 5.0, 8.0, $2, NULL, '{"interactive": {"reserved": 1.5, "used": 0.4}, "sync": {"reserved": 1.0, "used": 0.8}, "batch": {"reserved": 2.5, "used": 1.9}}')`,
			[]any{personal, at(2 * time.Hour)},
		},
		{
			`INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, from_name, subject, sent_at, labels, has_attachments, sender_class, content_flags, scan_state) VALUES
			($1, 'm-bank', 't-bank', $2, 'bank.example', $3, $4, $5, '{}', true, 'restricted', '{}', 'skipped_restricted'),
			($1, 'm-news', 't-news', $6, 'newsletter.example', $7, $8, $9, '{INBOX}', false, 'normal', '{mfa_code}', 'scanned'),
			($1, 'm-news2', 't-news', $6, 'Newsletter.Example', $7, $8, $9, '{INBOX}', false, 'normal', '{mfa_code}', 'pending'),
			($1, 'm-empty', 't-empty', 'nobody@', '', $7, $8, $9, '{}', false, 'normal', '{}', 'scanned'),
			($10, 'm-other', 't-other', $11::text, 'other.example', $11::text, $11::text, $9, '{}', false, 'normal', '{}', 'pending')`,
			[]any{personal, bank.FromAddress, bank.FromName, bank.Subject, at(48 * time.Hour), newsletter.FromAddress, newsletter.FromName, newsletter.Subject, at(30 * time.Hour), other, marker.Field("other")},
		},
		{
			// The newsletter's first message was scanned, so its row carries the scan's time and version
			// and the two content rules that set its flag, and no rule set its normal class. The bank's rule
			// set its restricted class. A failure's row detail shows both apart (docs/UI.md 7.1).
			`UPDATE messages SET rule_ids = '{content.mfa.subject_numeric_6,content.mfa.trigger_window}', scanned_at = $2, scanner_version = 3
			WHERE account_id = $1 AND message_id = 'm-news'`,
			[]any{personal, at(29 * time.Hour)},
		},
		{"UPDATE messages SET class_rule_id = 'rule.bank' WHERE account_id = $1 AND message_id = 'm-bank'", []any{personal}},
		{
			`INSERT INTO senders (account_id, domain, message_count, first_seen, last_seen, sender_class) VALUES
			($1, 'bank.example', 1, $2, $2, 'restricted'), ($1, 'lender.example', 4, $3, $2, 'normal')`,
			[]any{personal, at(48 * time.Hour), at(90 * 24 * time.Hour)},
		},
		{
			`INSERT INTO reorg_plans (plan_id, account_id, status, description, proposer, plan, validation, created_at, approved_at, approved_by, refusal_reason) VALUES
			($1, $4, 'DRAFT', $5, 'agent', '{}', '{"result": "passed", "findings": []}', $6, NULL, NULL, NULL),
			($2, $4, 'APPLYING', $7, 'agent', '{}', '{"result": "passed", "findings": []}', $8, $8, 'operator', NULL),
			($3, $4, 'APPLIED', $13, 'agent', '{}', '{"result": "passed", "findings": []}', $9, $9, 'operator', NULL),
			($10, $11, 'DRAFT', $12, 'agent', '{}', NULL, $6, NULL, NULL, NULL)`,
			[]any{
				draftPlan, applyingPlan, appliedPlan, personal, marker.MarkupField("plandescription") + "\nSecond line", at(28 * time.Hour),
				marker.MarkupField("applyingplan"), at(3 * time.Hour), at(10 * 24 * time.Hour), otherPlan, other, marker.Field("other"),
				marker.MarkupField("appliedplan"),
			},
		},
		{
			`INSERT INTO reorg_plan_ops (account_id, plan_id, message_id, add_labels, remove_labels, flows, reason) VALUES
			($1, $2, 'm-news', '{Newsletters}', '{INBOX}', '{INBOX>Newsletters}', $4),
			($1, $2, 'm-news2', '{Newsletters}', '{INBOX}', '{INBOX>Newsletters}', $4),
			($1, $3, 'm-news', '{Archive}', '{}', '{none>Archive}', $4),
			($1, $3, 'm-news2', '{Archive}', '{}', '{none>Archive}', $4)`,
			[]any{personal, applyingPlan, appliedPlan, marker.MarkupField("reason")},
		},
		{
			`INSERT INTO reorg_op_log (plan_id, message_id, labels_before, labels_after) VALUES
			($1, 'm-news', '{INBOX}', '{Newsletters}'), ($2, 'm-news', '{INBOX}', '{INBOX,Archive}'), ($2, 'm-news2', '{INBOX}', '{INBOX,Archive}')`,
			[]any{applyingPlan, appliedPlan},
		},
		{
			`INSERT INTO job_runs (account_id, run_id, workload, pass, state, plan_id, resumed_from, started_at, finished_at, heartbeat_at, checkpoint, counters, last_error) VALUES
			($1, 'r-0901', 'backfill', 'pass1', 'succeeded', NULL, NULL, $2, $3, $3, '{"page": 3368, "of": 3368}', '{"pages": 3368, "messages": 84212}', NULL),
			($1, 'r-0912', 'backfill', 'pass2', 'failed', NULL, NULL, $16, $17, $17, '{"page": 14, "of": 3368}', '{"pages": 14, "decided": 350, "pending": 83862, "scanned": 330, "skipped": 20}', $18),
			($1, 'r-0913', 'backfill', 'pass2', 'running', NULL, 'r-0912', $4, NULL, $5, '{"page": 3065, "of": 3368}', '{"pages": 3065, "decided": 76610, "pending": 7602, "scanned": 70100, "skipped": 6510}', NULL),
			($1, 'r-0914', 'sync', 'tick', 'succeeded', NULL, NULL, $6, $5, $5, NULL, '{"added": 3, "modified": 1, "removed": 0}', NULL),
			($1, 'r-0911', 'sync', 'gap_recovery', 'succeeded', NULL, NULL, $7, $7, $7, NULL, '{"window_start": "2026-09-08T00:00:00Z", "window_end": "2026-09-08T06:00:00Z", "reconciled": 41}', NULL),
			($1, 'r-0910', 'apply', 'apply', 'succeeded', $8, NULL, $9, $9, $9, '{"seq": 2, "of": 2}', '{"ops_done": 2, "ops_total": 2, "failures": 0}', NULL),
			($1, 'r-0915', 'apply', 'apply', 'running', $10, NULL, $11, NULL, $5, '{"seq": 1, "of": 2}', '{"ops_done": 1, "ops_total": 2, "failures": 0}', $12),
			($1, 'r-0909', 'heuristics', NULL, 'succeeded', NULL, NULL, $13, $13, $13, NULL, '{"candidates": 2}', NULL),
			($14, 'r-other', 'backfill', 'pass1', 'running', NULL, NULL, $4, NULL, $5, NULL, '{}', $15)`,
			[]any{
				personal, at(9 * 24 * time.Hour), at(8 * 24 * time.Hour), at(40 * time.Minute), at(time.Minute), at(5 * time.Minute),
				at(2 * 24 * time.Hour), appliedPlan, at(10 * 24 * time.Hour), applyingPlan, at(3 * time.Hour), marker.MarkupField("runerror"),
				at(20 * time.Hour), other, marker.Field("other"), at(50 * time.Hour), at(49 * time.Hour), marker.MarkupField("runfailure"),
			},
		},
		{
			// r-0912's failures, one per disposition and item kind, among them a message the index no
			// longer holds and an item with no page, and r-0901's, one on a message stored with an empty
			// domain beside a page item with none. seq is written so the rows and their identities do not
			// move between runs.
			`INSERT INTO job_run_failures (account_id, run_id, seq, item_kind, item_id, page, error_class, error_summary, attempts, first_at, last_at, disposition, recovered_by) VALUES
			($1, 'r-0912', 1, 'message', 'm-bank', 12, 'scanner_timeout', $2, 2, $3, $4, 'recovered', 'r-0913'),
			($1, 'r-0912', 2, 'message', 'm-news2', 12, 'provider_error', $5, 1, $3, $3, 'pending', NULL),
			($1, 'r-0912', 3, 'page', '13', 13, 'throttled', NULL, 5, $3, $6, 'abandoned', NULL),
			($1, 'r-0912', 4, 'message', 'm-gone', 14, 'gone', NULL, 1, $6, $6, 'gone', NULL),
			($1, 'r-0912', 5, 'message', 'm-news', NULL, 'provider_error', NULL, 1, $6, $6, 'pending', NULL),
			($1, 'r-0901', 1, 'message', 'm-empty', 7, 'provider_error', NULL, 1, $6, $6, 'pending', NULL),
			($1, 'r-0901', 2, 'page', '8', 8, 'throttled', NULL, 1, $6, $6, 'abandoned', NULL),
			($7, 'r-other', 1, 'message', 'm-other', 1, 'gone', $8, 1, $3, $3, 'gone', NULL)`,
			[]any{
				personal, marker.MarkupField("errorsummary"), at(50 * time.Hour), at(45 * time.Hour), marker.Field("providererror"),
				at(49*time.Hour + 30*time.Minute), other, marker.Field("other"),
			},
		},
		{
			`INSERT INTO job_run_events (account_id, run_id, kind, at, page, detail) VALUES
			($1, 'r-0913', 'start', $2, NULL, NULL), ($1, 'r-0913', 'progress', $3, 3005, NULL), ($1, 'r-0913', 'progress', $4, 3065, NULL),
			($1, 'r-0912', 'start', $5, NULL, NULL), ($1, 'r-0912', 'progress', $6, 12, NULL), ($1, 'r-0912', 'failure', $7, 12, $8),
			($1, 'r-0912', 'backoff', $7, 12, NULL), ($1, 'r-0912', 'retry', $9, 12, NULL), ($1, 'r-0912', 'finish', $10, 14, NULL),
			($11, 'r-other', 'start', $2, NULL, $12)`,
			[]any{
				personal, at(40 * time.Minute), at(9 * time.Minute), at(time.Minute), at(50 * time.Hour), at(49*time.Hour + 50*time.Minute),
				at(49*time.Hour + 40*time.Minute), marker.MarkupField("eventdetail"), at(49*time.Hour + 35*time.Minute), at(49 * time.Hour),
				other, marker.Field("other"),
			},
		},
		{
			`INSERT INTO policy_candidates (account_id, domain, signals, score, status, created_at, reviewed_at, reviewed_by) VALUES
			($1, 'lender.example', $2, 0.9, 'pending', $3, NULL, NULL),
			($1, 'bank.example', '[{"heuristic": "institution_keyword", "keyword": "bank"}]', 0.7, 'confirmed', $4, $5, 'operator'),
			($6, 'other.example', '[]', 0.5, 'pending', $3, NULL, NULL)`,
			[]any{
				personal, `[{"heuristic": "display_name", "name": "` + marker.MarkupField("displayname") + `", "domain": "bank.example"}]`,
				at(26 * time.Hour), at(5 * 24 * time.Hour), at(4 * 24 * time.Hour), other,
			},
		},
		{
			`INSERT INTO audit_log (ts, account_id, actor, action, message_id) VALUES
			($2, $1, 'agent', 'READ_BODY', 'm-news'), ($2, $1, 'agent', 'DENY_BODY', 'm-bank'), ($3, $1, 'agent', 'READ_BODY', 'm-news'), ($2, $4, 'agent', 'READ_BODY', 'm-other')`,
			[]any{personal, at(2 * time.Hour), at(3 * 24 * time.Hour), other},
		},
		{
			`INSERT INTO masking_events (account_id, message_id, field, rule_id, tier, masked_at) VALUES ($1, 'm-news', 'subject', 'content.mfa.subject_numeric_6', 1, $2)`,
			[]any{personal, at(24 * time.Hour)},
		},
		{
			`INSERT INTO scan_gate_decisions (account_id, message_id, decision, reason, decided_at) VALUES
			($1, 'm-bank', 'SKIP', 'restricted', $2), ($1, 'm-news', 'SCAN', 'normal', $2)`,
			[]any{personal, at(24 * time.Hour)},
		},
	}
	for _, s := range statements {
		if _, err := conn.Exec(ctx, s.sql, s.args...); err != nil {
			t.Fatalf("seeding: %v\n%s", err, s.sql)
		}
	}
}

// uiPool connects as the UI's own role, so every read below is held to the UI's grants and the
// row-level security policies. The test database's superuser owns the connection and sets the role
// on each new session.
func uiPool(t *testing.T, role string) *pgxpool.Pool {
	t.Helper()
	pool, _ := countingPool(t, role)
	return pool
}

// statements counts the statements a pool sends, apart from the role each new session sets, so a test
// can require that a request ran none.
type statements struct{ sent atomic.Int64 }

func (c *statements) TraceQueryStart(ctx context.Context, _ *pgx.Conn, data pgx.TraceQueryStartData) context.Context {
	if !strings.HasPrefix(data.SQL, "SET ROLE ") {
		c.sent.Add(1)
	}
	return ctx
}

func (c *statements) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// countingPool is uiPool with a count of the statements it sends.
func countingPool(t *testing.T, role string) (*pgxpool.Pool, *statements) {
	t.Helper()
	cfg, err := pgxpool.ParseConfig(postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	counter := &statements{}
	cfg.ConnConfig.Tracer = counter
	cfg.AfterConnect = func(ctx context.Context, conn *pgx.Conn) error {
		_, err := conn.Exec(ctx, "SET ROLE "+pgx.Identifier{role}.Sanitize())
		return err
	}
	pool, err := pgxpool.NewWithConfig(t.Context(), cfg)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool, counter
}

// server is the UI's server over the seeded database, as the UI's role, with the fixed clock and the
// bundle directory as the binary would embed it.
func server(t *testing.T) (*api.Server, *prometheus.Registry) {
	t.Helper()
	seed(t)
	return serverAs(t, uiPool(t, "mediated_mailbox_ui"))
}

func serverAs(t *testing.T, pool *pgxpool.Pool) (*api.Server, *prometheus.Registry) {
	t.Helper()
	reg := prometheus.NewRegistry()
	s, err := api.New(api.Options{
		Bundle:         os.DirFS(filepath.Join("..", "..", "browser", "dist")),
		Database:       pool,
		Datasets:       registry.Datasets(),
		Logger:         slog.New(slog.NewJSONHandler(io.Discard, nil)),
		Metrics:        reg,
		Clock:          func() time.Time { return now },
		Cadences:       api.Cadences{Sync: 5 * time.Minute, Heuristics: 24 * time.Hour},
		StreamInterval: 50 * time.Millisecond,
	})
	if err != nil {
		t.Fatal(err)
	}
	return s, reg
}
