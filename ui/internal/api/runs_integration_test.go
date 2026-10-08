//go:build integration

package api_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
)

// conforms requires a response to match its route's declaration in the checked-in contract document,
// so a shape the browser's types describe is the shape the server sends (ADR-0065).
func conforms(t *testing.T, pattern string, r response) {
	t.Helper()
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "contract", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	item := doc.Paths.Value(pattern)
	if item == nil || item.Get == nil {
		t.Fatalf("the document declares no GET %s", pattern)
	}
	content := item.Get.Responses.Status(r.status)
	if content == nil {
		t.Fatalf("%s answered %d, which the document does not declare", pattern, r.status)
	}
	var value any
	if err := json.Unmarshal(r.body, &value); err != nil {
		t.Fatal(err)
	}
	if err := content.Value.Content.Get("application/json").Schema.Value.VisitJSON(value); err != nil {
		t.Fatalf("%s does not match its declaration: %v\n%s", pattern, err, r.body)
	}
}

// read requests a path, requires 200 and a body matching the contract, and decodes it into v, which
// it empties first.
func read(t *testing.T, h http.Handler, pattern, path string, v any) {
	t.Helper()
	r := get(t, h, path)
	if r.status != http.StatusOK {
		t.Fatalf("%s answered %d: %s", path, r.status, r.body)
	}
	conforms(t, pattern, r)
	// v is emptied first, since decoding into a filled value merges into its maps and slices.
	reflect.ValueOf(v).Elem().SetZero()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatal(err)
	}
}

type figure struct {
	Key   string  `json:"key"`
	Value *int64  `json:"value"`
	At    *string `json:"at"`
	Link  string  `json:"link"`
}

type groups struct {
	Group string `json:"group"`
	Total struct {
		Count      int64  `json:"count"`
		Restricted *int64 `json:"restricted"`
		Flagged    *int64 `json:"flagged"`
	} `json:"total"`
	Rows []struct {
		Key        map[string]any `json:"key"`
		Count      int64          `json:"count"`
		Restricted *int64         `json:"restricted"`
		Flagged    *int64         `json:"flagged"`
	} `json:"rows"`
}

// counts is each group's key and count, written key=count, in the order sent, with none for the null
// group.
func (g groups) counts() []string {
	var out []string
	for _, row := range g.Rows {
		for _, v := range row.Key {
			k := "none"
			if v != nil {
				k = fmt.Sprint(v)
			}
			out = append(out, k+"="+strconv.FormatInt(row.Count, 10))
		}
	}
	return out
}

func n(v int64) *int64 { return &v }

// TestTheRunsDatasetAnswersItsLevels checks the runs dataset against the seeded runs at every level,
// its figures, its default exclusion of delta-sync ticks, its sorts, its groups and its filters
// (docs/UI.md sections 8.3 and 17.1).
func TestTheRunsDatasetAnswersItsLevels(t *testing.T) {
	s, _ := server(t)
	h := s.Handler()

	var summary struct {
		Figures []figure `json:"figures"`
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=0&range=all&pass=!tick", &summary)
	failedAt := now.Add(-49 * 60 * 60 * 1e9).UTC().Format("2006-01-02T15:04:05Z")
	want := []figure{
		{Key: "runs", Value: n(7), Link: "/personal/jobs"},
		{Key: "running", Value: n(2), Link: "/personal/jobs?state=running"},
		{Key: "failed", Value: n(1), Link: "/personal/jobs?state=failed"},
		{Key: "last_failure", At: &failedAt, Link: "/personal/jobs/r-0912"},
	}
	if d := cmp.Diff(want, summary.Figures); d != "" {
		t.Fatalf("the figures (-want +got):\n%s", d)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=0&state=succeeded", &summary)
	if last := summary.Figures[3]; last.At != nil || last.Value != nil || last.Link != "/personal/jobs?state=failed" {
		t.Fatalf("with no failed run the last failure is %+v, want no time and the failed runs' link", last)
	}

	var page struct {
		Pages int `json:"pages"`
		Total struct {
			Count int `json:"count"`
		} `json:"total"`
		Rows []struct {
			RunID    string `json:"run_id"`
			Failures int64  `json:"failures"`
		} `json:"rows"`
	}
	ids := func() []string {
		var out []string
		for _, r := range page.Rows {
			out = append(out, r.RunID)
		}
		return out
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=3&range=all&pass=!tick", &page)
	if want := []string{"r-0913", "r-0915", "r-0909", "r-0911", "r-0912", "r-0901", "r-0910"}; !slices.Equal(ids(), want) || page.Total.Count != 7 {
		t.Fatalf("the runs newest first, ticks excluded, are %v of %d, want %v", ids(), page.Total.Count, want)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=3&range=all&sort=failures,desc&pass=!tick", &page)
	if page.Rows[0].RunID != "r-0912" || page.Rows[0].Failures != 5 || page.Rows[1].RunID != "r-0901" || page.Rows[1].Failures != 2 || page.Rows[2].Failures != 0 {
		t.Fatalf("sorted by failures the first rows are %+v, want r-0912 with 5 then r-0901 with 2", page.Rows[:3])
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=3&range=all&sort=duration,desc&workload=backfill", &page)
	if want := []string{"r-0901", "r-0912", "r-0913"}; !slices.Equal(ids(), want) {
		t.Fatalf("backfill by duration longest first is %v, want %v, the running one's to as_of", ids(), want)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=3&range=7d&pass=tick", &page)
	if want := []string{"r-0914"}; !slices.Equal(ids(), want) {
		t.Fatalf("the ticks in 7 days are %v, want %v", ids(), want)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=3&range=all&day=2026-09-01,2026-09-08", &page)
	if want := []string{"r-0911", "r-0912", "r-0901"}; !slices.Equal(ids(), want) {
		t.Fatalf("the runs started on two days are %v, want %v", ids(), want)
	}

	var g groups
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=1&group=workload&range=all", &g)
	if want := []string{"backfill=3", "apply=2", "sync=2", "heuristics=1"}; !slices.Equal(g.counts(), want) || g.Total.Count != 8 || g.Total.Restricted != nil {
		t.Fatalf("by workload the groups are %v of %d, want %v of 8 and no sensitivity", g.counts(), g.Total.Count, want)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=2&group=state&range=all&workload=backfill", &g)
	if want := []string{"failed=1", "running=1", "succeeded=1"}; !slices.Equal(g.counts(), want) {
		t.Fatalf("backfill by state is %v, want %v", g.counts(), want)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=runs&level=1&group=day&range=all&workload=!apply", &g)
	if want := []string{"2026-09-08=2", "2026-09-10=2", "2026-09-01=1", "2026-09-09=1"}; !slices.Equal(g.counts(), want) {
		t.Fatalf("by day the groups are %v, want %v", g.counts(), want)
	}
}

// TestTheFailuresDatasetAnswersItsLevels checks a run's failures at every level, the sensitivity
// totals, the null groups and their none filters, and the rows' message fields (docs/UI.md sections
// 8.4 and 17.1).
func TestTheFailuresDatasetAnswersItsLevels(t *testing.T) {
	s, _ := server(t)
	h := s.Handler()

	var summary struct {
		Figures []figure `json:"figures"`
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=0", &summary)
	link := func(d string) string { return "/personal/jobs/r-0912?disposition=" + d + "&level=3" }
	want := []figure{
		{Key: "failures", Value: n(5), Link: "/personal/jobs/r-0912?level=3"},
		{Key: "recovered", Value: n(1), Link: link("recovered")},
		{Key: "pending", Value: n(2), Link: link("pending")},
		{Key: "gone", Value: n(1), Link: link("gone")},
		{Key: "abandoned", Value: n(1), Link: link("abandoned")},
	}
	if d := cmp.Diff(want, summary.Figures); d != "" {
		t.Fatalf("the figures (-want +got):\n%s", d)
	}

	var g groups
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=1&group=error_class", &g)
	if want := []string{"provider_error=2", "gone=1", "scanner_timeout=1", "throttled=1"}; !slices.Equal(g.counts(), want) {
		t.Fatalf("by error class the groups are %v, want %v", g.counts(), want)
	}
	if g.Total.Count != 5 || *g.Total.Restricted != 1 || *g.Total.Flagged != 2 {
		t.Fatalf("the total is %+v, want 5 with the bank message restricted and both newsletters flagged", g.Total)
	}
	if first := g.Rows[0]; *first.Restricted != 0 || *first.Flagged != 2 {
		t.Fatalf("provider errors count %d restricted and %d flagged, want 0 and 2", *first.Restricted, *first.Flagged)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=1&group=sender", &g)
	if want := []string{"newsletter.example=2", "none=2", "bank.example=1"}; !slices.Equal(g.counts(), want) {
		t.Fatalf("by sender the groups are %v, want %v, the page item and the gone message in none last among equals", g.counts(), want)
	}
	// The newsletter's two messages are stored under one domain, as the domain normalizer writes it. A
	// filter naming the domain in any case returns the group's rows, and an exclusion in any case drops
	// them all, since the server gives the filter its values in the stored form.
	var items struct {
		Total struct {
			Count int64 `json:"count"`
		} `json:"total"`
	}
	for _, spelling := range []string{"newsletter.example", "Newsletter.Example", "NEWSLETTER.EXAMPLE"} {
		read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=3&sender="+spelling, &items)
		if items.Total.Count != 2 {
			t.Errorf("sender=%s returns %d items, want the group's 2", spelling, items.Total.Count)
		}
		read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=3&sender=!"+spelling, &items)
		if items.Total.Count != 3 {
			t.Errorf("sender=!%s leaves %d items, want 3 with the group's 2 dropped", spelling, items.Total.Count)
		}
	}
	// A message stored with an empty domain is a group of its own, keyed empty, apart from the null
	// group of the items with no message, and none selects the null group alone.
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0901&level=1&group=sender", &g)
	if len(g.Rows) != 2 || g.Rows[0].Key["sender"] != "" || g.Rows[0].Count != 1 || g.Rows[1].Key["sender"] != nil || g.Rows[1].Count != 1 {
		t.Fatalf("r-0901 by sender is %+v, want the empty domain's group and the null group, one each", g.Rows)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0901&level=3&sender=none", &items)
	if items.Total.Count != 1 {
		t.Fatalf("sender=none on r-0901 returns %d items, want the page item alone", items.Total.Count)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=1&group=page_number", &g)
	if want := []string{"12=2", "13=1", "14=1", "none=1"}; !slices.Equal(g.counts(), want) {
		t.Fatalf("by page the groups are %v, want %v with none last", g.counts(), want)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=2&group=disposition&sender=none", &g)
	if want := []string{"abandoned=1", "gone=1"}; !slices.Equal(g.counts(), want) {
		t.Fatalf("the items with no sender by disposition are %v, want %v", g.counts(), want)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=2&group=error_class&page_number=!none", &g)
	if g.Total.Count != 4 {
		t.Fatalf("excluding the items with no page leaves %d, want 4", g.Total.Count)
	}

	var page struct {
		Rows []struct {
			Seq         int64    `json:"seq"`
			ItemKind    string   `json:"item_kind"`
			FromEmail   *string  `json:"from_email"`
			Subject     *string  `json:"subject"`
			Labels      []string `json:"labels"`
			SenderClass *string  `json:"sender_class"`
			Page        *int32   `json:"page"`
			RecoveredBy *string  `json:"recovered_by"`
		} `json:"rows"`
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=3&sort=attempts,desc", &page)
	var seqs []int64
	for _, r := range page.Rows {
		seqs = append(seqs, r.Seq)
	}
	if want := []int64{3, 1, 2, 4, 5}; !slices.Equal(seqs, want) {
		t.Fatalf("by attempts the rows are %v, want %v, the identity breaking ties", seqs, want)
	}
	byItem := map[int64]int{}
	for i, r := range page.Rows {
		byItem[r.Seq] = i
	}
	bank, pageItem, gone := page.Rows[byItem[1]], page.Rows[byItem[3]], page.Rows[byItem[4]]
	if bank.SenderClass == nil || *bank.SenderClass != "restricted" || bank.Subject == nil || bank.RecoveredBy == nil || *bank.RecoveredBy != "r-0913" {
		t.Fatalf("the bank message's row is %+v", bank)
	}
	if pageItem.FromEmail != nil || pageItem.Labels != nil || pageItem.ItemKind != "page" || pageItem.Page == nil || *pageItem.Page != 13 {
		t.Fatalf("the page item's row is %+v, want no message fields", pageItem)
	}
	if gone.FromEmail != nil || gone.SenderClass != nil {
		t.Fatalf("the gone message's row is %+v, want no message fields", gone)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0912&level=3&sender=bank.example,none&page_number=13,14", &page)
	seqs = nil
	for _, r := range page.Rows {
		seqs = append(seqs, r.Seq)
	}
	slices.Sort(seqs)
	if want := []int64{3, 4}; !slices.Equal(seqs, want) {
		t.Fatalf("a sender or no sender on pages 13 and 14 is %v, want %v", seqs, want)
	}
}

// TestTheRowDetailAnswersItsRow requires a failure's row detail with its error summary and its
// message's audit rows, a page item's with none, and the client's 404 for a row the account's parent
// does not hold, another account's among them (docs/UI.md section 17.1).
func TestTheRowDetailAnswersItsRow(t *testing.T) {
	s, _ := server(t)
	h := s.Handler()
	var detail struct {
		Run string `json:"run"`
		Row struct {
			Seq int64 `json:"seq"`
		} `json:"row"`
		ErrorSummary   *string  `json:"error_summary"`
		ClassRuleID    *string  `json:"class_rule_id"`
		RuleIDs        []string `json:"rule_ids"`
		ScannedAt      *string  `json:"scanned_at"`
		ScannerVersion *int32   `json:"scanner_version"`
		Audit          []struct {
			Action string `json:"action"`
		} `json:"audit"`
		AuditCount int64 `json:"audit_count"`
	}
	read(t, h, "/api/{account}/failures/{row}", "/api/personal/failures/1?run=r-0912", &detail)
	if detail.Row.Seq != 1 || detail.ErrorSummary == nil || *detail.ErrorSummary != marker.MarkupField("errorsummary") ||
		len(detail.Audit) != 1 || detail.Audit[0].Action != "DENY_BODY" || detail.AuditCount != 1 {
		t.Fatalf("the bank message's failure detail is %+v", detail)
	}
	if detail.ClassRuleID == nil || *detail.ClassRuleID != "rule.bank" || detail.RuleIDs == nil || len(detail.RuleIDs) != 0 ||
		detail.ScannedAt != nil || detail.ScannerVersion != nil {
		t.Fatalf("the bank message's sensitivity block is %v %v %v %v, want its class set by rule.bank, no content rule, never scanned",
			detail.ClassRuleID, detail.RuleIDs, detail.ScannedAt, detail.ScannerVersion)
	}
	read(t, h, "/api/{account}/failures/{row}", "/api/personal/failures/5?run=r-0912", &detail)
	if len(detail.Audit) != 2 || detail.AuditCount != 2 {
		t.Fatalf("the newsletter's failure carries %d audit rows of %d, want 2 of 2", len(detail.Audit), detail.AuditCount)
	}
	scanned := now.Add(-29 * time.Hour).Format(time.RFC3339)
	if want := []string{"content.mfa.subject_numeric_6", "content.mfa.trigger_window"}; detail.ClassRuleID != nil || !slices.Equal(detail.RuleIDs, want) ||
		detail.ScannedAt == nil || *detail.ScannedAt != scanned || detail.ScannerVersion == nil || *detail.ScannerVersion != 3 {
		t.Fatalf("the newsletter's sensitivity block is %v %v %v %v, want no class rule, %v, %s and version 3",
			detail.ClassRuleID, detail.RuleIDs, detail.ScannedAt, detail.ScannerVersion, want, scanned)
	}
	read(t, h, "/api/{account}/failures/{row}", "/api/personal/failures/3?run=r-0912", &detail)
	if len(detail.Audit) != 0 || detail.AuditCount != 0 || detail.ClassRuleID != nil || detail.RuleIDs != nil {
		t.Fatalf("the page item's detail carries audit rows or a sensitivity block: %+v", detail)
	}
	for _, p := range []string{"/api/personal/failures/9?run=r-0912", "/api/personal/failures/1?run=r-0913", "/api/personal/failures/1?run=r-other"} {
		r := get(t, h, p)
		if origin, code := errorOf(t, r); r.status != http.StatusNotFound || origin != "client" || code != "unknown_row" {
			t.Errorf("%s answered %d %s %s, want 404 unknown_row", p, r.status, origin, code)
		}
		conforms(t, "/api/{account}/failures/{row}", r)
	}
}

// TestTheRunSummaryAnswersItsRun requires a run's summary, its resumer, its failures per disposition,
// the runs that recovered them and its timeline in the order recorded, and the client's 404 for a run
// the account does not hold (docs/UI.md section 17.4).
func TestTheRunSummaryAnswersItsRun(t *testing.T) {
	s, _ := server(t)
	h := s.Handler()
	var summary struct {
		Run struct {
			RunID     string  `json:"run_id"`
			State     string  `json:"state"`
			LastError *string `json:"last_error"`
		} `json:"run"`
		Resumer *struct {
			RunID       string `json:"run_id"`
			ResumedFrom string `json:"resumed_from"`
		} `json:"resumer"`
		Failures struct {
			Count        int64            `json:"count"`
			Dispositions map[string]int64 `json:"dispositions"`
		} `json:"failures"`
		RecoveredBy []struct {
			RunID     string `json:"run_id"`
			Recovered int64  `json:"recovered"`
		} `json:"recovered_by"`
		Events []struct {
			Kind   string  `json:"kind"`
			Page   *int32  `json:"page"`
			Detail *string `json:"detail"`
		} `json:"events"`
	}
	read(t, h, "/api/{account}/jobs/{run}", "/api/personal/jobs/r-0912", &summary)
	if summary.Run.RunID != "r-0912" || summary.Run.State != "failed" || summary.Resumer == nil || summary.Resumer.RunID != "r-0913" {
		t.Fatalf("the failed run and its resumer are %+v, %+v", summary.Run, summary.Resumer)
	}
	if d := cmp.Diff(map[string]int64{"recovered": 1, "pending": 2, "gone": 1, "abandoned": 1}, summary.Failures.Dispositions); d != "" || summary.Failures.Count != 5 {
		t.Fatalf("the failures are %d, dispositions (-want +got):\n%s", summary.Failures.Count, d)
	}
	if len(summary.RecoveredBy) != 1 || summary.RecoveredBy[0].RunID != "r-0913" || summary.RecoveredBy[0].Recovered != 1 {
		t.Fatalf("the recovering runs are %+v", summary.RecoveredBy)
	}
	var kinds []string
	for _, e := range summary.Events {
		kinds = append(kinds, e.Kind)
	}
	if want := []string{"start", "progress", "failure", "backoff", "retry", "finish"}; !slices.Equal(kinds, want) {
		t.Fatalf("the timeline is %v, want %v", kinds, want)
	}
	if e := summary.Events[2]; e.Detail == nil || *e.Detail != marker.MarkupField("eventdetail") || e.Page == nil || *e.Page != 12 {
		t.Fatalf("the failure event is %+v", e)
	}
	read(t, h, "/api/{account}/jobs/{run}", "/api/personal/jobs/r-0909", &summary)
	if summary.Resumer != nil || summary.Failures.Count != 0 || len(summary.Failures.Dispositions) != 0 || len(summary.Events) != 0 {
		t.Fatalf("a run with no resumer, failure or event reads %+v", summary)
	}
	for _, p := range []string{"/api/personal/jobs/r-9999", "/api/personal/jobs/r-other"} {
		r := get(t, h, p)
		if origin, code := errorOf(t, r); r.status != http.StatusNotFound || origin != "client" || code != "unknown_run" {
			t.Errorf("%s answered %d %s %s, want 404 unknown_run", p, r.status, origin, code)
		}
	}
}

// TestTheEmptyGroupsWordSelectsTheRowsItCounts checks that the group of a value stored empty, r-0901's
// message stored with an empty domain, is named by sender=empty, which selects exactly the rows the
// group counts, and that its exclusion leaves the others (docs/UI.md sections 5 and 17.1).
func TestTheEmptyGroupsWordSelectsTheRowsItCounts(t *testing.T) {
	s, _ := server(t)
	h := s.Handler()

	var g groups
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0901&level=1&group=sender", &g)
	var empty int64 = -1
	for _, row := range g.Rows {
		if row.Key["sender"] == "" {
			empty = row.Count
		}
	}
	if empty != 1 {
		t.Fatalf("r-0901 by sender is %+v, want the empty domain's group counting 1", g.Rows)
	}
	var page struct {
		Total struct {
			Count int64 `json:"count"`
		} `json:"total"`
		Rows []struct {
			ItemID string `json:"item_id"`
		} `json:"rows"`
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0901&level=3&sender=empty", &page)
	if page.Total.Count != empty || len(page.Rows) != 1 || page.Rows[0].ItemID != "m-empty" {
		t.Fatalf("sender=empty returns %d items, %+v, want the group's %d, the message stored with an empty domain", page.Total.Count, page.Rows, empty)
	}
	read(t, h, "/api/{account}/lens", "/api/personal/lens?dataset=failures&run=r-0901&level=3&sender=!empty", &page)
	if page.Total.Count != g.Total.Count-empty {
		t.Fatalf("sender=!empty leaves %d items, want %d, every item but the group's", page.Total.Count, g.Total.Count-empty)
	}
}
