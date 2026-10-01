//go:build integration

package api_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

// policy is the content security policy ADR-0062 decides, written out here rather than read from the
// code under test, so a change to the constant turns this red (ADR-0046).
const policy = "default-src 'self'; script-src 'self'; style-src 'self'; img-src 'self' data:; " +
	"font-src 'self'; connect-src 'self'; frame-ancestors 'none'"

type response struct {
	status int
	header http.Header
	body   []byte
}

func get(t *testing.T, h http.Handler, path string) response {
	t.Helper()
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequestWithContext(t.Context(), http.MethodGet, path, nil))
	return response{status: rec.Code, header: rec.Header(), body: rec.Body.Bytes()}
}

// errorOf decodes an error body, failing the test when the body is not the error contract's shape.
func errorOf(t *testing.T, r response) (origin, code string) {
	t.Helper()
	var body struct {
		Error struct {
			Origin    string `json:"origin"`
			Code      string `json:"code"`
			RequestID string `json:"request_id"`
		} `json:"error"`
	}
	if err := json.Unmarshal(r.body, &body); err != nil {
		t.Fatalf("the body is not the error contract's shape: %v\n%s", err, r.body)
	}
	if body.Error.RequestID == "" || body.Error.RequestID != r.header.Get("X-Request-Id") {
		t.Fatalf("the error carries request id %q and the response %q", body.Error.RequestID, r.header.Get("X-Request-Id"))
	}
	return body.Error.Origin, body.Error.Code
}

// TestEveryResponseCarriesThePolicy requires the policy header, exact in name and value, on every
// kind of response the UI serves, the entry document on every screen route, the bundle, the read
// API's answers and its errors, a path no route answers, and the probes (VERIFICATIONS, the policy
// row, ADR-0064's first Go assertion).
func TestEveryResponseCarriesThePolicy(t *testing.T) {
	s, _ := server(t)
	paths := []string{
		"/", "/personal", "/personal/jobs", "/personal/plans/" + draftPlan,
		"/api/accounts", "/api/personal/system", "/api/personal/lens?dataset=plans",
		"/api/personal/lens?dataset=nothing", "/api/all/system", "/api/lens", "/api/personal/nothing",
	}
	if built(t) {
		paths = append(paths, "/main.js", "/main.css", "/fonts/IBMPlexSans-Regular-Latin1.woff2")
	}
	for _, p := range paths {
		r := get(t, s.Handler(), p)
		if got := r.header.Values("Content-Security-Policy"); !slices.Equal(got, []string{policy}) {
			t.Errorf("%s answered %d with the policy %q, want exactly %q", p, r.status, got, policy)
		}
	}
	for _, p := range []string{"/healthz", "/readyz", "/metrics"} {
		r := get(t, s.Probes(), p)
		if got := r.header.Values("Content-Security-Policy"); !slices.Equal(got, []string{policy}) {
			t.Errorf("the probe %s answered %d with the policy %q, want exactly %q", p, r.status, got, policy)
		}
	}
}

// built reports whether the bundle directory holds a bundle, which the ui workflow builds before the
// Go tests. A fresh clone holds only the placeholder.
func built(t *testing.T) bool {
	t.Helper()
	_, err := os.Stat(filepath.Join("..", "..", "browser", "dist", "main.js"))
	return err == nil
}

// TestTheEntryDocumentIsRenderedForEveryScreenRoute requires every path the browser's router owns to
// load the entry document, and it to load the bundle's entry module and its stylesheet from the UI's own
// origin.
func TestTheEntryDocumentIsRenderedForEveryScreenRoute(t *testing.T) {
	s, _ := server(t)
	for _, p := range []string{"/", "/personal", "/personal/candidates?status=pending", "/personal/jobs/r-0913"} {
		r := get(t, s.Handler(), p)
		if r.status != http.StatusOK || !strings.Contains(string(r.body), `<script type="module" src="/main.js"></script>`) ||
			!strings.Contains(string(r.body), `<link rel="stylesheet" href="/main.css">`) {
			t.Errorf("%s answered %d without the entry document:\n%s", p, r.status, r.body)
		}
		if r.header.Get("Content-Type") != "text/html; charset=utf-8" {
			t.Errorf("%s answered as %q", p, r.header.Get("Content-Type"))
		}
	}
	if r := get(t, s.Handler(), "/.gitkeep"); !strings.Contains(string(r.body), "<!doctype html>") {
		t.Errorf("the bundle's placeholder was served as a file: %d %q", r.status, r.body)
	}
}

// TestTheRegistryRefusesWhatItDoesNotDeclare requests a dataset, a dimension filter, a sort column and
// a group the registry does not declare, a level outside the ladder, a filter value no statement could
// compare, and a row detail with a malformed row, without its parent or with another parameter, and
// requires each refused as the client's fault with no statement sent to the database, the account
// check's included (VERIFICATIONS, the registry row).
func TestTheRegistryRefusesWhatItDoesNotDeclare(t *testing.T) {
	seed(t)
	pool, sent := countingPool(t, "mediated_mailbox_ui")
	s, _ := serverAs(t, pool)
	if r := get(t, s.Handler(), "/api/personal/lens?dataset=plans&level=0"); r.status != http.StatusOK || sent.sent.Load() == 0 {
		t.Fatalf("a declared request answered %d after %d statements, so the count proves nothing", r.status, sent.sent.Load())
	}
	cases := []struct {
		query, code string
	}{
		{"dataset=messages", "unknown_dataset"},
		{"dataset=plans&level=3&sender=bank.example", "unknown_dimension"},
		{"dataset=plans&level=3&proposer=agent", "unknown_dimension"},
		{"dataset=plans&level=3&sort=description,asc", "unknown_sort"},
		{"dataset=plans&level=3&sort=status,asc", "unknown_sort"},
		{"dataset=candidates&level=1&group=status", "unknown_group"},
		{"dataset=plans&level=4", "invalid_level"},
		{"dataset=plans&dataset=candidates", "repeated_parameter"},
		{"level=0", "missing_dataset"},
		{"dataset=plans&level=0&group=nothing", "unknown_group"},
		{"dataset=runs&level=3&day=yesterday", "invalid_filter"},
		{"dataset=runs&level=3&sender=none", "unknown_dimension"},
		{"dataset=runs&level=3&workload=none", "invalid_filter"},
		{"dataset=failures&run=r-0912&level=3&page_number=twelve", "invalid_filter"},
		{"dataset=failures&level=1&group=page_number", "missing_parent"},
	}
	for i := range cases {
		cases[i].query = "lens?" + cases[i].query
	}
	// The row-detail route refuses what the registry does not declare the same way.
	cases = append(cases, []struct{ query, code string }{
		{"failures/one?run=r-0912", "invalid_row"},
		{"failures/1", "missing_parent"},
		{"failures/1?run=r-0912&level=3", "unknown_parameter"},
		{"failures/1?run=r-0912&run=r-0913", "repeated_parameter"},
	}...)
	for _, c := range cases {
		before := sent.sent.Load()
		r := get(t, s.Handler(), "/api/personal/"+c.query)
		if n := sent.sent.Load() - before; n != 0 {
			t.Errorf("%s sent %d statements before its refusal", c.query, n)
		}
		origin, code := errorOf(t, r)
		if r.status != http.StatusBadRequest || origin != "client" || code != c.code {
			t.Errorf("%s answered %d %s %s, want 400 client %s", c.query, r.status, origin, code, c.code)
		}
	}
}

// TestEveryReadIsPerAccount requests account-scoped routes with no account segment, with the value
// meaning every account and with an unknown account, and requires each refused, while the account
// list stays the one unscoped read and no read for one account returns another's rows (VERIFICATIONS,
// the per-account row).
func TestEveryReadIsPerAccount(t *testing.T) {
	s, _ := server(t)
	for _, p := range []string{"/api/lens?dataset=plans", "/api/system", "/api/jobs", "/api/events", "/api/jobs/r-0912", "/api/failures/1?run=r-0912"} {
		r := get(t, s.Handler(), p)
		if origin, code := errorOf(t, r); r.status != http.StatusNotFound || origin != "client" || code != "not_found" {
			t.Errorf("%s answered %d %s %s, want 404", p, r.status, origin, code)
		}
	}
	for account, want := range map[string]string{"all": "all_accounts", "nobody": "unknown_account"} {
		for _, route := range []string{"lens?dataset=plans", "system", "jobs", "events", "jobs/r-0912", "failures/1?run=r-0912", "lens?dataset=runs", "lens?dataset=failures&run=r-0912"} {
			r := get(t, s.Handler(), "/api/"+account+"/"+route)
			if origin, code := errorOf(t, r); r.status != http.StatusBadRequest || origin != "client" || code != want {
				t.Errorf("/api/%s/%s answered %d %s %s, want 400 %s", account, route, r.status, origin, code, want)
			}
		}
	}
	accounts := get(t, s.Handler(), "/api/accounts")
	if accounts.status != http.StatusOK || !strings.Contains(string(accounts.body), `"personal"`) || !strings.Contains(string(accounts.body), `"other"`) {
		t.Fatalf("the account list is not every account: %d %s", accounts.status, accounts.body)
	}
	for _, p := range []string{
		"/api/personal/lens?dataset=plans&level=3", "/api/personal/lens?dataset=candidates&level=3",
		"/api/personal/lens?dataset=plans&level=0", "/api/personal/system", "/api/personal/jobs",
		"/api/personal/lens?dataset=runs&level=3&range=all", "/api/personal/lens?dataset=runs&level=1&group=workload&range=all",
		"/api/personal/lens?dataset=failures&run=r-0912&level=3", "/api/personal/lens?dataset=failures&run=r-0912&level=1&group=sender",
		"/api/personal/failures/1?run=r-0912", "/api/personal/jobs/r-0912",
		"/api/personal/lens?dataset=failures&run=r-other&level=3", "/api/personal/lens?dataset=failures&run=r-other&level=1&group=sender",
	} {
		r := get(t, s.Handler(), p)
		if r.status != http.StatusOK {
			t.Fatalf("%s answered %d: %s", p, r.status, r.body)
		}
		if strings.Contains(p, "run=r-other") && !strings.Contains(string(r.body), `"count":0`) {
			t.Errorf("%s answered rows of another account's run: %s", p, r.body)
		}
		leaks := []string{marker.Field("other"), "other.example", otherPlan}
		if !strings.Contains(p, "run=r-other") {
			leaks = append(leaks, "r-other")
		}
		for _, leaked := range leaks {
			if strings.Contains(string(r.body), leaked) {
				t.Errorf("%s returned the other account's %s", p, leaked)
			}
		}
	}
}

// TestTheLensAnswersItsLevels checks the dataset endpoint's two levels against the seeded rows, the
// figures at level 0 and a page at level 3, the filter grammar and the sort.
func TestTheLensAnswersItsLevels(t *testing.T) {
	s, _ := server(t)
	var page struct {
		Pages int `json:"pages"`
		Total struct {
			Count int `json:"count"`
		} `json:"total"`
		Rows []struct {
			PlanID   string `json:"plan_id"`
			Messages int    `json:"messages"`
			ApplyRun *struct {
				RunID string `json:"run_id"`
			} `json:"apply_run"`
		} `json:"rows"`
	}
	decode := func(p string) {
		t.Helper()
		r := get(t, s.Handler(), p)
		if r.status != http.StatusOK {
			t.Fatalf("%s answered %d: %s", p, r.status, r.body)
		}
		page.Rows = nil
		if err := json.Unmarshal(r.body, &page); err != nil {
			t.Fatal(err)
		}
	}
	ids := func() []string {
		var out []string
		for _, r := range page.Rows {
			out = append(out, r.PlanID)
		}
		return out
	}
	decode("/api/personal/lens?dataset=plans&level=3")
	if want := []string{applyingPlan, draftPlan, appliedPlan}; !slices.Equal(ids(), want) || page.Total.Count != 3 || page.Pages != 1 {
		t.Fatalf("the default page is %v of %d, want %v newest first", ids(), page.Total.Count, want)
	}
	if page.Rows[0].Messages != 2 || page.Rows[0].ApplyRun == nil || page.Rows[0].ApplyRun.RunID != "r-0915" {
		t.Fatalf("the applying plan's row is %+v", page.Rows[0])
	}
	decode("/api/personal/lens?dataset=plans&level=3&sort=created_at,asc&status=!DRAFT")
	if want := []string{appliedPlan, applyingPlan}; !slices.Equal(ids(), want) || page.Total.Count != 2 {
		t.Fatalf("the exclusion sorted oldest first is %v of %d, want %v", ids(), page.Total.Count, want)
	}
	decode("/api/personal/lens?dataset=plans&level=3&status=DRAFT,APPLIED&range=7d")
	if want := []string{draftPlan}; !slices.Equal(ids(), want) {
		t.Fatalf("the any-of filter within 7 days is %v, want %v", ids(), want)
	}
	decode("/api/personal/lens?dataset=plans&level=3&page=2")
	if len(page.Rows) != 0 || page.Total.Count != 3 {
		t.Fatalf("the page past the end holds %v", ids())
	}
}

// TestADatabaseFailureReportsTheDatabaseOrigin reads as a role holding no grant on the plans, whose
// refusal names a table and nothing the operator could act on, and requires it reported with the
// database origin (docs/UI.md section 17.3, ADR-0084).
func TestADatabaseFailureReportsTheDatabaseOrigin(t *testing.T) {
	seed(t)
	s, _ := serverAs(t, uiPool(t, "mediated_mailbox_backfill"))
	r := get(t, s.Handler(), "/api/personal/lens?dataset=plans&level=3")
	if origin, _ := errorOf(t, r); r.status != http.StatusServiceUnavailable || origin != "database" {
		t.Fatalf("a refused read answered %d %s, want 503 database: %s", r.status, origin, r.body)
	}
}

// TestReadinessFollowsTheDatabase requires /readyz to answer 200 while the database answers and 503
// once it does not, and /healthz 200 throughout.
func TestReadinessFollowsTheDatabase(t *testing.T) {
	seed(t)
	pool := uiPool(t, "mediated_mailbox_ui")
	s, _ := serverAs(t, pool)
	if r := get(t, s.Probes(), "/readyz"); r.status != http.StatusOK {
		t.Fatalf("ready answered %d with the database up", r.status)
	}
	pool.Close()
	if r := get(t, s.Probes(), "/readyz"); r.status != http.StatusServiceUnavailable {
		t.Fatalf("ready answered %d with the pool closed, want 503", r.status)
	}
	if r := get(t, s.Probes(), "/healthz"); r.status != http.StatusOK {
		t.Fatalf("health answered %d", r.status)
	}
}

// TestTheMetricsCarryReadsAndSubscribers requires the read latency by dataset and level after a read.
func TestTheMetricsCarryReadsAndSubscribers(t *testing.T) {
	s, _ := server(t)
	get(t, s.Handler(), "/api/personal/lens?dataset=candidates&level=0")
	r := get(t, s.Probes(), "/metrics")
	for _, want := range []string{
		`mediated_mailbox_ui_read_duration_seconds_count{dataset="candidates",level="0"} 1`,
		"mediated_mailbox_ui_stream_subscribers 0",
	} {
		if !strings.Contains(string(r.body), want) {
			t.Errorf("the metrics lack %s:\n%s", want, r.body)
		}
	}
}

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

// advance moves the recorded state on as the workloads would. A new delta-sync tick finishes, pass 2's
// checkpoint moves, and a heuristics run starts, which takes the runs table's first place.
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
			`INSERT INTO job_runs (account_id, run_id, workload, pass, state, started_at, finished_at, heartbeat_at, counters) VALUES
			($1, 'r-0916', 'sync', 'tick', 'succeeded', $2, $3, $3, '{"added": 5, "modified": 0, "removed": 2}'),
			($1, 'r-0917', 'heuristics', NULL, 'running', $4, NULL, $4, '{}')`,
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

// TestTheStreamSendsEachChangedObject subscribes to the stream and requires the current state of every
// followed object first, then one event for an object that changed and none for one that did not,
// with keep-alive comments while nothing changes (ADR-0058, docs/UI.md section 17.5). Every event's
// data must match the contract's schema for its name, and a run's event must be the same whole state
// the jobs endpoint sends for that run.
func TestTheStreamSendsEachChangedObject(t *testing.T) {
	s, _ := server(t)
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "contract", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	refs, ok := doc.Paths.Value("/api/{account}/events").Get.Extensions["x-events"].(map[string]any)
	if !ok {
		t.Fatal("the contract's stream route declares no x-events")
	}
	schemaOf := func(name string) *openapi3.Schema {
		t.Helper()
		ref, ok := refs[name].(string)
		if !ok {
			t.Fatalf("the contract declares no event %q", name)
		}
		return doc.Components.Schemas[strings.TrimPrefix(ref, "#/components/schemas/")].Value
	}
	ts := httptest.NewServer(s.Handler())
	t.Cleanup(ts.Close)
	ctx, cancel := context.WithCancel(t.Context())
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ts.URL+"/api/personal/events", nil)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := resp.Body.Close(); err != nil {
			t.Error(err)
		}
	}()
	if resp.StatusCode != http.StatusOK || resp.Header.Get("Content-Type") != "text/event-stream" || resp.Header.Get("Content-Security-Policy") != policy {
		t.Fatalf("the stream answered %d as %q", resp.StatusCode, resp.Header.Get("Content-Type"))
	}
	events := make(chan [2]string, 64)
	var keepAlives atomic.Int64
	go func() {
		defer close(events)
		scanner := bufio.NewScanner(resp.Body)
		var name string
		for scanner.Scan() {
			line := scanner.Text()
			if line == ": keep-alive" {
				keepAlives.Add(1)
			} else if n, ok := strings.CutPrefix(line, "event: "); ok {
				name = n
			} else if d, ok := strings.CutPrefix(line, "data: "); ok {
				events <- [2]string{name, d}
			}
		}
	}()
	next := func() (name string, data map[string]any) {
		t.Helper()
		select {
		case e, ok := <-events:
			if !ok {
				t.Fatal("the stream ended")
			}
			var value any
			if err := json.Unmarshal([]byte(e[1]), &value); err != nil {
				t.Fatal(err)
			}
			if err := schemaOf(e[0]).VisitJSON(value); err != nil {
				t.Fatalf("the %s event does not match its schema in the contract: %v\n%s", e[0], err, e[1])
			}
			if err := json.Unmarshal([]byte(e[1]), &data); err != nil {
				t.Fatal(err)
			}
			return e[0], data
		case <-time.After(5 * time.Second):
			t.Fatal("no event within five seconds")
		}
		return "", nil
	}

	first := map[string]bool{}
	var applying map[string]any
	for range 4 {
		name, data := next()
		if name == "run" && data["run_id"] == "r-0915" {
			applying = data
		}
		key := name
		if id, ok := data[name+"_id"].(string); ok {
			key += ":" + id
		}
		first[key] = true
		if data["account"] != personal {
			t.Fatalf("an event for %v", data["account"])
		}
	}
	for _, want := range []string{"run:r-0913", "run:r-0915", "rate", "plan:" + applyingPlan} {
		if !first[want] {
			t.Fatalf("the first poll sent %v, missing %s", first, want)
		}
	}
	var jobs struct {
		Apply struct {
			Running map[string]any `json:"running"`
		} `json:"apply"`
	}
	if err := json.Unmarshal(get(t, s.Handler(), "/api/personal/jobs").body, &jobs); err != nil {
		t.Fatal(err)
	}
	delete(applying, "account")
	if d := cmp.Diff(jobs.Apply.Running, applying); d != "" {
		t.Fatalf("the stream's run is not the whole state the jobs endpoint sends (-jobs +stream):\n%s", d)
	}
	gauge := get(t, s.Probes(), "/metrics")
	if !strings.Contains(string(gauge.body), "mediated_mailbox_ui_stream_subscribers 1") {
		t.Errorf("the subscriber gauge does not read 1 while one stream is open")
	}

	admin, err := pgx.Connect(t.Context(), postgres.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := admin.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	if _, err := admin.Exec(t.Context(), `UPDATE job_runs SET checkpoint = '{"page": 3100, "of": 3368}' WHERE run_id = 'r-0913'`); err != nil {
		t.Fatal(err)
	}
	name, data := next()
	checkpoint, ok := data["checkpoint"].(map[string]any)
	if !ok || name != "run" || data["run_id"] != "r-0913" || checkpoint["page"] != float64(3100) {
		t.Fatalf("after the checkpoint moved, the stream sent %s %v", name, data)
	}
	if _, err := admin.Exec(t.Context(), `UPDATE reorg_plans SET status = 'APPLIED' WHERE plan_id = $1`, applyingPlan); err != nil {
		t.Fatal(err)
	}
	// The plan's new status changes the plan and the apply run that carries it, so both are sent, each
	// whole, the run first.
	name, data = next()
	if name != "run" || data["run_id"] != "r-0915" || data["plan_status"] != "APPLIED" {
		t.Fatalf("after the plan applied, the stream sent %s %v, want the apply run carrying the new status", name, data)
	}
	name, data = next()
	if name != "plan" || data["status"] != "APPLIED" || data["applied"] != float64(1) || data["of"] != float64(2) {
		t.Fatalf("after the plan applied, the stream sent %s %v", name, data)
	}
	before := keepAlives.Load()
	select {
	case e := <-events:
		t.Fatalf("the stream sent %v with nothing changed", e)
	case <-time.After(300 * time.Millisecond):
	}
	if keepAlives.Load() == before {
		t.Fatal("the stream wrote no keep-alive while nothing changed")
	}
	cancel()
}
