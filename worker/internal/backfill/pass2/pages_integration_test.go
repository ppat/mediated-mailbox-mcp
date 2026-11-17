//go:build integration

package pass2_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail/fake"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/marker"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2"
)

// pages returns the account's events and failed items as their rows hold them, each with its page
// or null, the runs of the second pass alone, in the order they were recorded.
func pages(t *testing.T, account string) (events, items []string) {
	t.Helper()
	read := func(query string) []string {
		rows, err := superuser(t).Query(context.Background(), query, account)
		if err != nil {
			t.Fatal(err)
		}
		out, err := pgx.CollectRows(rows, pgx.RowTo[string])
		if err != nil {
			t.Fatal(err)
		}
		return out
	}
	events = read(`SELECT e.kind || ' ' || coalesce(e.page::text, 'null') FROM job_run_events AS e
		JOIN job_runs AS r ON r.account_id = e.account_id AND r.run_id = e.run_id
		WHERE e.account_id = $1 AND r.pass = 'pass2' ORDER BY e.seq`)
	items = read(`SELECT f.item_id || ' ' || f.disposition || ' ' || coalesce(f.page::text, 'null') FROM job_run_failures AS f
		JOIN job_runs AS r ON r.account_id = f.account_id AND r.run_id = f.run_id
		WHERE f.account_id = $1 AND r.pass = 'pass2' ORDER BY f.seq`)
	return events, items
}

// A run of backfill's second pass records a page on each event of its timeline beside its finish and
// failure, and on each failed item, zero on one that concerns no page, such as the delisting
// transition's retry. The run screen reads a failed item's page, and the read of a run's earliest
// failed page counts a null as none, so a page recorded as null would read differently from zero
// (ADR-0016). A delta sync tick records none, which its own test holds.
func TestASecondPassRecordsAPageOnItsEventsAndFailedItems(t *testing.T) {
	messages := []fake.Message{
		message("m01", "alerts@bank.example", false, marker.Body("bank")+" Your statement is ready.", ""),
		message("m02", "a@shop.example", false, marker.Body("throttled"), ""),
		message("m03", "b@shop.example", false, marker.Body("gone"), ""),
		message("m04", "c@shop.example", false, marker.Body("nested"), strings.Repeat("<div>", 600)+marker.Body("deep")+strings.Repeat("</div>", 600)),
		message("m05", "d@shop.example", false, marker.Body("failing"), ""),
	}
	w := realWorldOf(t, 8, messages)
	inner := w.deps.Body
	throttled, failed := false, false
	w.deps.Body = func(ctx context.Context, id string) (mail.MessageBody, error) {
		switch {
		case id == "m02" && !throttled:
			throttled = true
			return mail.MessageBody{}, mail.ThrottleError{Signal: mail.ThrottleSignal{RetryAfterMillis: 1, HasRetryAfter: true}}
		case id == "m03":
			return mail.MessageBody{}, fmt.Errorf("fake: %w", mail.ErrNotFound)
		case id == "m05" && !failed:
			failed = true
			return mail.MessageBody{}, fmt.Errorf("fake: %w", mail.ErrProvider)
		}
		return inner(ctx, id)
	}
	w.open(t)
	if _, err := w.pass.Next(context.Background()); err != nil {
		t.Fatal(err)
	}
	// The next run lifts the bank's rule, so its transition returns the bank's message to pending and
	// starts the pass over, with a retry event that concerns no page.
	w.delisted = true
	w.open(t)
	// A change of scanner then has the run-start step return the shop's verdict to pending and mark the
	// pass to start over, so the next run resumes the stopped one with a retry event that concerns no
	// page. A gate skip whose subject is masked returns to pending at that run's start, with another.
	w.rescan(t)
	if _, err := w.reopen(w.ctx, w.deps); err != nil {
		t.Fatal(err)
	}
	if _, err := superuser(t).Exec(t.Context(), `UPDATE messages SET scan_state = 'skipped_gate', subject_masked = true
		WHERE account_id = $1 AND message_id = 'm03'`, w.account); err != nil {
		t.Fatal(err)
	}
	if _, err := pass2.Open(t.Context(), w.deps, w.account); err != nil {
		t.Fatal(err)
	}

	events, items := pages(t, w.account)
	wantEvents := []string{"start 0", "backoff 1", "retry 1", "progress 1", "resume 1", "progress 0", "retry 0", "resume 0", "retry 0", "retry 0"}
	if diff := cmp.Diff(wantEvents, events, compare.Options); diff != "" {
		t.Errorf("the second pass's events and their pages (-want +got):\n%s", diff)
	}
	wantItems := []string{"m02 recovered 1", "m03 gone 1", "m04 refused 1", "m05 recovered 1"}
	if diff := cmp.Diff(wantItems, items, compare.Options); diff != "" {
		t.Errorf("the second pass's failed items and their pages (-want +got):\n%s", diff)
	}
}
