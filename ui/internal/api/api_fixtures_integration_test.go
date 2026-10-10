//go:build integration

package api_test

import (
	"bytes"
	"context"
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

// fixtures are the responses the browser's tests are given, recorded from this server over the
// seeded database (ADR-0064). Each is compared with the checked-in file, and -update re-records it. A
// browser test answers a path only with the recording made at that exact path, so the paths here are
// the ones the browser's URL grammar builds.
func fixtures() map[string]string {
	return map[string]string{
		"accounts.json":                               "/api/accounts",
		"system-other.json":                           "/api/other/system",
		"system.json":                                 "/api/personal/system",
		"jobs.json":                                   "/api/personal/jobs",
		"plans-summary.json":                          "/api/personal/lens?dataset=plans&level=0&range=all&sort=created_at,desc",
		"plans-rows.json":                             "/api/personal/lens?dataset=plans&level=3&range=all&sort=created_at,desc&page=1",
		"plans-summary-rejected.json":                 "/api/personal/lens?dataset=plans&level=0&range=all&sort=created_at,desc&status=REJECTED",
		"plans-rows-empty.json":                       "/api/personal/lens?dataset=plans&level=3&range=all&sort=created_at,desc&page=1&status=REJECTED",
		"candidates-summary-pending.json":             "/api/personal/lens?dataset=candidates&level=0&range=all&sort=score,desc&status=pending",
		"candidates-rows-pending.json":                "/api/personal/lens?dataset=candidates&level=3&range=all&sort=score,desc&page=1&status=pending",
		"candidates-summary-all.json":                 "/api/personal/lens?dataset=candidates&level=0&range=all&sort=score,desc",
		"candidates-rows-all.json":                    "/api/personal/lens?dataset=candidates&level=3&range=all&sort=score,desc&page=1",
		"candidates-rows-pending-oldest.json":         "/api/personal/lens?dataset=candidates&level=3&range=all&sort=created_at,asc&page=1&status=pending",
		"attention.json":                              "/api/personal/attention",
		"attention-other.json":                        "/api/other/attention",
		"candidates-rows-pending-other.json":          "/api/other/lens?dataset=candidates&level=3&range=all&sort=score,desc&page=1&status=pending",
		"candidates-rows-pending-oldest-other.json":   "/api/other/lens?dataset=candidates&level=3&range=all&sort=created_at,asc&page=1&status=pending",
		"runs-summary.json":                           "/api/personal/lens?dataset=runs&level=0&range=7d&sort=started_at,desc&pass=!tick",
		"runs-rows.json":                              "/api/personal/lens?dataset=runs&level=3&range=7d&sort=started_at,desc&page=1&pass=!tick",
		"runs-by-workload.json":                       "/api/personal/lens?dataset=runs&level=1&group=workload&range=7d&sort=started_at,desc&pass=!tick",
		"runs-backfill-by-state.json":                 "/api/personal/lens?dataset=runs&level=2&group=state&range=7d&sort=started_at,desc&pass=!tick&workload=backfill",
		"runs-summary-backfill.json":                  "/api/personal/lens?dataset=runs&level=0&range=7d&sort=started_at,desc&pass=!tick&workload=backfill",
		"run-r-0912.json":                             "/api/personal/jobs/r-0912",
		"run-r-0913.json":                             "/api/personal/jobs/r-0913",
		"failures-by-error-class.json":                "/api/personal/lens?dataset=failures&run=r-0912&level=1&group=error_class&sort=last_at,desc",
		"failures-by-disposition.json":                "/api/personal/lens?dataset=failures&run=r-0912&level=1&group=disposition&sort=last_at,desc",
		"failures-rows.json":                          "/api/personal/lens?dataset=failures&run=r-0912&level=3&sort=last_at,desc&page=1",
		"failures-provider-error-by-sender.json":      "/api/personal/lens?dataset=failures&run=r-0912&level=2&group=sender&sort=last_at,desc&error_class=provider_error",
		"failures-provider-error-by-disposition.json": "/api/personal/lens?dataset=failures&run=r-0912&level=1&group=disposition&sort=last_at,desc&error_class=provider_error",
		"failures-provider-error-rows.json":           "/api/personal/lens?dataset=failures&run=r-0912&level=3&sort=last_at,desc&page=1&error_class=provider_error",
		"failure-5.json":                              "/api/personal/failures/5?run=r-0912",
		"failure-1.json":                              "/api/personal/failures/1?run=r-0912",
		"failure-4.json":                              "/api/personal/failures/4?run=r-0912",
		"error-unknown-dataset.json":                  "/api/personal/lens?dataset=messages",
		"error-unknown-account.json":                  "/api/nobody/system",
		"jobs-other.json":                             "/api/other/jobs",
		"runs-summary-other.json":                     "/api/other/lens?dataset=runs&level=0&range=7d&sort=started_at,desc&pass=!tick",
		"runs-rows-other.json":                        "/api/other/lens?dataset=runs&level=3&range=7d&sort=started_at,desc&page=1&pass=!tick",
		"runs-summary-custom.json":                    "/api/personal/lens?dataset=runs&level=0&range=2026-09-01,2026-09-10&sort=started_at,desc&pass=!tick",
		"runs-rows-custom.json":                       "/api/personal/lens?dataset=runs&level=3&range=2026-09-01,2026-09-10&sort=started_at,desc&page=1&pass=!tick",
		"error-unknown-run-other.json":                "/api/other/jobs/r-0912",
		"error-unknown-account-run.json":              "/api/nobody/jobs/r-0912",
		"error-unknown-sort-summary.json":             "/api/personal/lens?dataset=plans&level=0&range=all&sort=bogus,desc",
		"error-unknown-sort-rows.json":                "/api/personal/lens?dataset=plans&level=3&range=all&sort=bogus,desc&page=1",
	}
}

// TestTheRecordedFixturesMatchTheServer records each fixture and requires every response to match its
// declaration in the contract document, so the types the browser is compiled against describe what
// the server sends (ADR-0065). The fixture diff is the other half of the fixture drift row.
func TestTheRecordedFixturesMatchTheServer(t *testing.T) {
	s, _ := server(t)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "contract", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	record(t, s, doc, fixtures())
	// The live screens' tests read some paths again after the recorded state has moved on, so those
	// paths are recorded a second time after it moves (docs/UI.md sections 8.3 and 9).
	advance(t)
	record(t, s, doc, laterFixtures())
	// The jobs screen's bar for pass 2 must appear on a new run's first page event, so the jobs
	// answer is recorded once more after a pass 2 run starts that records no page yet.
	startPass2(t)
	record(t, s, doc, map[string]string{"jobs-pass2-started.json": "/api/personal/jobs"})
	// The rate budget must appear on an account's first spend, so the other account's jobs answer is
	// recorded once more after it spends for the first time.
	firstSpend(t)
	record(t, s, doc, map[string]string{"jobs-other-spent.json": "/api/other/jobs"})
	// Home's states for an account with nothing indexed, nothing awaiting a decision and nothing worth a
	// look are recorded once the other account's recorded state is removed (docs/UI.md section 8.1).
	emptyOther(t)
	record(t, s, doc, map[string]string{
		"system-other-empty.json":                         "/api/other/system",
		"jobs-other-empty.json":                           "/api/other/jobs",
		"attention-other-empty.json":                      "/api/other/attention",
		"candidates-rows-pending-other-empty.json":        "/api/other/lens?dataset=candidates&level=3&range=all&sort=score,desc&page=1&status=pending",
		"candidates-rows-pending-oldest-other-empty.json": "/api/other/lens?dataset=candidates&level=3&range=all&sort=created_at,asc&page=1&status=pending",
	})
	// The banner of a pass 1 a change of scanner re-opened, and Home's backfill cell for it, are
	// recorded once the personal account's pass 1, which succeeded, runs again (docs/UI.md sections 8.1
	// and 12).
	reopenPass1(t)
	record(t, s, doc, map[string]string{"system-reopened.json": "/api/personal/system", "jobs-reopened.json": "/api/personal/jobs"})
}

// reopenPass1 moves the personal account on as a backfill run that finds the first pass due again
// would (ADR-0120). Pass 2's running run succeeds, the next run clears both passes' completion and
// starts pass 1 again from the checkpoint of the enumeration that ended, and that run has fetched 12
// of the 42 subjects stored masked under the earlier scanner again.
func reopenPass1(t *testing.T) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	for _, st := range []struct {
		sql  string
		args []any
	}{
		{
			`UPDATE job_runs SET state = 'succeeded', finished_at = $2 WHERE account_id = $1 AND run_id = 'r-0918'`,
			[]any{personal, now.Add(-8 * time.Second)},
		},
		{
			`UPDATE account_state SET backfill_pass1_complete = false, backfill_pass2_complete = false WHERE account_id = $1`,
			[]any{personal},
		},
		{
			`INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, heartbeat_at, checkpoint, counters) VALUES
			($1, 'r-0919', 'backfill', 'pass1', 'running', $2, $2,
				'{"page": 3368, "token": "", "of": 3368, "version": 1, "revision": "r2", "stale": 30}',
				'{"pages": 0, "messages": 0, "remasked": 0, "refetched": 12}')`,
			[]any{personal, now.Add(-6 * time.Second)},
		},
	} {
		if _, err := conn.Exec(t.Context(), st.sql, st.args...); err != nil {
			t.Fatalf("re-opening pass 1: %v\n%s", err, st.sql)
		}
	}
}

// emptyOther removes every row the other account holds apart from the account itself, so it reads as an
// account whose backfill has not started.
func emptyOther(t *testing.T) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	for _, table := range []string{
		"job_run_events", "job_run_failures", "job_runs", "masking_events", "audit_log", "scan_gate_decisions",
		"policy_candidates", "reorg_plans", "messages", "rate_state",
	} {
		if _, err := conn.Exec(t.Context(), "DELETE FROM "+table+" WHERE account_id = $1", other); err != nil {
			t.Fatalf("emptying %s: %v", table, err)
		}
	}
}

// firstSpend gives the other account, which has never spent, its first rate state.
func firstSpend(t *testing.T) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	const sql = `INSERT INTO rate_state (account_id, current_rate, target_rate, hard_cap, last_throttle_at, backoff_until, classes)
		VALUES ($1, 0.6, 5.0, 8.0, NULL, NULL, '{"interactive": {"reserved": 1.5, "used": 0.0}, "sync": {"reserved": 1.0, "used": 0.6}, "batch": {"reserved": 2.5, "used": 0.0}}')`
	if _, err := conn.Exec(t.Context(), sql, other); err != nil {
		t.Fatalf("the first spend: %v", err)
	}
}

// startPass2 finishes pass 2's run and starts another that has recorded no checkpoint page yet.
func startPass2(t *testing.T) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	for _, sql := range []string{
		`UPDATE job_runs SET state = 'failed', finished_at = $2 WHERE account_id = $1 AND run_id = 'r-0913'`,
		`INSERT INTO job_runs (account_id, run_id, workload, pass, state, resumed_from, started_at, heartbeat_at, checkpoint, counters) VALUES
		($1, 'r-0918', 'backfill', 'pass2', 'running', 'r-0913', $2, $2, '{}', '{}')`,
	} {
		if _, err := conn.Exec(t.Context(), sql, personal, now.Add(-10*time.Second)); err != nil {
			t.Fatalf("starting pass 2: %v\n%s", err, sql)
		}
	}
}

// laterFixtures are the responses recorded after advance moves the recorded state on, each at a path
// the browser's tests also read before the move.
func laterFixtures() map[string]string {
	return map[string]string{
		"jobs-later.json":         "/api/personal/jobs",
		"runs-summary-later.json": "/api/personal/lens?dataset=runs&level=0&range=7d&sort=started_at,desc&pass=!tick",
		"runs-rows-later.json":    "/api/personal/lens?dataset=runs&level=3&range=7d&sort=started_at,desc&page=1&pass=!tick",
	}
}

// advance moves the recorded state on as the workloads would. The running gap recovery finishes, a new
// delta-sync tick finishes, pass 2's checkpoint moves, and another gap recovery starts, which takes the
// runs table's first place.
func advance(t *testing.T) {
	t.Helper()
	conn, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	at := func(ago time.Duration) time.Time { return now.Add(-ago) }
	for _, st := range []struct {
		sql  string
		args []any
	}{
		{
			`UPDATE job_runs SET checkpoint = '{"page": 3300, "of": 3368}', heartbeat_at = $2 WHERE account_id = $1 AND run_id = 'r-0913'`,
			[]any{personal, at(30 * time.Second)},
		},
		{
			`UPDATE job_runs SET state = 'succeeded', finished_at = $2, heartbeat_at = $2, counters = '{"window_start": "2026-09-10T08:00:00Z", "window_end": "2026-09-10T09:00:00Z", "reconciled": 3}' WHERE account_id = $1 AND run_id = 'r-0908'`,
			[]any{personal, at(100 * time.Second)},
		},
		{
			`INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at, heartbeat_at, counters) VALUES
			($1, 'r-0916', 'sync', 'tick', 'succeeded', $2, $3, $3, '{"added": 5, "modified": 0, "removed": 2}'),
			($1, 'r-0917', 'sync', 'gap_recovery', 'running', $4, NULL, $4, '{}')`,
			[]any{personal, at(90 * time.Second), at(time.Minute), at(30 * time.Second)},
		},
	} {
		if _, err := conn.Exec(t.Context(), st.sql, st.args...); err != nil {
			t.Fatalf("advancing: %v\n%s", err, st.sql)
		}
	}
}

// record requests each path, requires its answer to match its declaration in the contract document,
// and compares it with its recorded file, which -update writes.
func record(t *testing.T, s *api.Server, doc *openapi3.T, paths map[string]string) {
	t.Helper()
	for name, p := range paths {
		r := get(t, s.Handler(), p)
		route := strings.SplitN(strings.TrimPrefix(p, "/api/"), "?", 2)[0]
		pattern := "/api/accounts"
		if route != "accounts" {
			pattern = "/api/{account}/" + strings.SplitN(route, "/", 2)[1]
		}
		// A run's summary and a failure's row detail carry their identity in the path.
		if parts := strings.Split(route, "/"); len(parts) == 3 {
			pattern = map[string]string{"jobs": "/api/{account}/jobs/{run}", "failures": "/api/{account}/failures/{row}"}[parts[1]]
		}
		op := doc.Paths.Value(pattern).Get
		status := r.status
		content := op.Responses.Status(status)
		if content == nil {
			t.Fatalf("%s answered %d, which the document does not declare", p, status)
		}
		var value any
		if err := json.Unmarshal(r.body, &value); err != nil {
			t.Fatalf("%s: %v", p, err)
		}
		if err := content.Value.Content.Get("application/json").Schema.Value.VisitJSON(value); err != nil {
			t.Errorf("%s does not match its declaration in the contract document: %v\n%s", p, err, r.body)
		}
		var pretty bytes.Buffer
		if err := json.Indent(&pretty, r.body, "", "  "); err != nil {
			t.Fatal(err)
		}
		if strings.HasPrefix(name, "error-") {
			// The request id is random, so the recording holds a fixed one in its place.
			var e map[string]map[string]any
			if err := json.Unmarshal(r.body, &e); err != nil {
				t.Fatal(err)
			}
			e["error"]["request_id"] = "recorded"
			b, err := json.MarshalIndent(e, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			pretty.Reset()
			pretty.Write(append(b, '\n'))
		}
		compare.GoldenAt(t, filepath.Join("..", "..", "browser", "test", "fixtures", name), pretty.Bytes())
	}
}
