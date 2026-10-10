//go:build integration

package pass2_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/provider/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2"
	core "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/pass2"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// active is the policy a running pass takes at each page's entry, as the worker's policy loader holds
// it. Each reload builds a new snapshot whether or not a rule changed (ADR-0041), so a reload here is
// a fresh load of the rows the policy tables hold.
type active struct {
	t       *testing.T
	account string
	rows    []policy.Row
	current policy.Composed
}

// reload loads rows into a new snapshot and makes it the active one.
func (a *active) reload(rows ...policy.Row) {
	a.t.Helper()
	p, err := policy.Load(rows)
	if err != nil {
		a.t.Fatal(err)
	}
	a.rows, a.current = rows, p.For(a.account)
}

// next makes one page durable and fails the test when it fails.
func next(t *testing.T, p *pass2.Pass) {
	t.Helper()
	if _, err := p.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
}

// VERIFICATIONS' row for a policy edit during a running second pass. A rule restricting the shop is
// added while the pass runs, and a reload makes it the active policy. Before the run's next page, the
// shop's messages stored as normal are stored restricted by the same run, and its message on a later
// page is skipped as restricted, its body never fetched. Then a reload whose rules are the same by
// value follows a write setting the shop's messages back to normal directly, and they stay normal,
// since the run makes the comparisons again only when the rules differ by value (ADR-0119, ADR-0113).
func TestAPolicyEditReachesARunningSecondPassWithinAPage(t *testing.T) {
	messages := []fake.Message{
		message("m01", "a@shop.example", false, marker.Body("shopfirst"), ""),
		message("m02", "news@news.example", false, marker.Body("newsfirst"), ""),
		message("m03", "b@shop.example", false, marker.Body("shoplater"), ""),
		message("m04", "news@news.example", false, marker.Body("newslater"), ""),
	}
	w := realWorldOf(t, 1, messages)
	a := &active{t: t, account: w.account}
	a.reload()
	w.deps.Policy = func() policy.Composed { return a.current }
	p, err := pass2.Open(context.Background(), w.deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	next(t, p)
	if st := inspectPostgres(t, postgres.URL(t), w.account).Messages["m01"]; st.Class != "normal" || st.State != "scanned" {
		t.Fatalf("before the edit m01 is %+v, want a normal sender scanned", st)
	}

	shop := policy.Row{ID: "rule.shop", Class: policy.Restricted, DomainSuffixes: []string{"shop.example"}}
	a.reload(shop)
	next(t, p)
	edited := inspectPostgres(t, postgres.URL(t), w.account)
	for _, id := range []string{"m01", "m03"} {
		if st := edited.Messages[id]; st.Class != "restricted" {
			t.Errorf("after the edit's first page message %s is %+v, want its sender stored restricted", id, st)
		}
	}
	if st := edited.Messages["m02"]; st.State != "scanned" {
		t.Errorf("the page after the edit left m02 %+v, want it scanned", st)
	}
	next(t, p)
	if st := inspectPostgres(t, postgres.URL(t), w.account).Messages["m03"]; st.State != "skipped_restricted" {
		t.Errorf("the shop's later message is %+v, want skipped as restricted", st)
	}

	if _, err := superuser(t).Exec(context.Background(),
		"UPDATE messages SET sender_class = 'normal', class_rule_id = NULL WHERE account_id = $1 AND from_domain = 'shop.example'", w.account); err != nil {
		t.Fatal(err)
	}
	a.reload(shop)
	next(t, p)
	end := inspectPostgres(t, postgres.URL(t), w.account)
	for _, id := range []string{"m01", "m03"} {
		if st := end.Messages[id]; st.Class != "normal" {
			t.Errorf("after a reload with the same rules message %s is %+v, want it left normal until the next run's start", id, st)
		}
	}
	if diff := cmp.Diff([]string{"m01", "m02", "m04"}, w.fetched, compare.Options); diff != "" {
		t.Errorf("the bodies asked for, the shop's later one never (-want +got):\n%s", diff)
	}
	if len(end.Runs) != 1 {
		t.Errorf("the account has %d runs of the second pass, want the one that took the edit", len(end.Runs))
	}
}

// A rule removed during a running second pass returns its sender's messages stored as restricted to a
// normal class and pending scan before the run's next page, and the same run starts over from the first
// waiting message, as the delisting transition at a run's start does, and then scans them (ADR-0119,
// ADR-0037).
func TestARuleRemovedDuringARunningSecondPassReachesItWithinAPage(t *testing.T) {
	messages := []fake.Message{
		message("m01", "alerts@bank.example", false, marker.Body("bankfirst"), ""),
		message("m02", "news@news.example", false, marker.Body("newsfirst"), ""),
		message("m03", "codes@bank.example", false, marker.Body("banklater"), ""),
		message("m04", "news@news.example", false, marker.Body("newslater"), ""),
	}
	w := realWorldOf(t, 1, messages)
	bank := policy.Row{ID: "rule.bank", Class: policy.Restricted, DomainSuffixes: []string{listedDomain}}
	a := &active{t: t, account: w.account}
	a.reload(bank)
	w.deps.Policy = func() policy.Composed { return a.current }
	p, err := pass2.Open(context.Background(), w.deps, w.account)
	if err != nil {
		t.Fatal(err)
	}
	next(t, p)
	next(t, p)
	before := inspectPostgres(t, postgres.URL(t), w.account)
	if st := before.Messages["m01"]; st.State != "skipped_restricted" {
		t.Fatalf("before the removal m01 is %+v, want skipped as restricted", st)
	}

	a.reload()
	next(t, p)
	after := inspectPostgres(t, postgres.URL(t), w.account)
	latest, _ := after.latest()
	if latest.Progress.Checkpoint == (core.Checkpoint{}) {
		t.Errorf("the page after the removal left the run at %+v, want it past the first message it read again", latest.Progress.Checkpoint)
	}
	if st := after.Messages["m01"]; st.Class != "normal" || st.State != "scanned" {
		t.Errorf("after the removal's first page m01 is %+v, want a normal sender scanned from the first waiting message", st)
	}
	for {
		s, err := p.Next(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if s.Done {
			break
		}
	}
	end := inspectPostgres(t, postgres.URL(t), w.account)
	if st := end.Messages["m03"]; st.Class != "normal" || st.State != "scanned" {
		t.Errorf("at the pass's end m03 is %+v, want a normal sender scanned", st)
	}
	if diff := cmp.Diff([]string{"m02", "m01", "m03", "m04"}, w.fetched, compare.Options); diff != "" {
		t.Errorf("the bodies asked for, the bank's only after the removal (-want +got):\n%s", diff)
	}
	if len(end.Runs) != 1 {
		t.Errorf("the account has %d runs of the second pass, want the one that took the removal", len(end.Runs))
	}
}

// VERIFICATIONS' row for a job's run that panics, the second pass's start. A panic raised in the
// comparisons the second pass makes at its start, after the run is recorded, is recovered in the run
// and recorded as that run's failure with its stack, and Open returns it as its error (ADR-0119).
func TestAPanicAtTheSecondPassesStartIsRecordedAsItsRunsFailure(t *testing.T) {
	w := realWorldOf(t, 1, []fake.Message{message("m01", "news@news.example", false, marker.Body("news"), "")})
	w.deps.Policy = func() policy.Composed { panic("a bug in the comparisons") }
	if _, err := pass2.Open(context.Background(), w.deps, w.account); !errors.Is(err, schedule.ErrPanicked) {
		t.Fatalf("opening the pass returned %v, want the panic as its error", err)
	}
	var state, last string
	if err := superuser(t).QueryRow(context.Background(),
		"SELECT state, coalesce(last_error, '') FROM job_runs WHERE account_id = $1 AND pass = 'pass2'", w.account).Scan(&state, &last); err != nil {
		t.Fatal(err)
	}
	if state != "failed" || !strings.Contains(last, "a bug in the comparisons") || !strings.Contains(last, "goroutine") {
		t.Errorf("the second pass run is recorded %q with %q, want failed with the panic and its stack", state, last)
	}
}
