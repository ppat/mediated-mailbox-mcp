//go:build integration

package api_test

import (
	"net/url"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// A sender domain the operator types in any case, a dotted capital I among its letters, finds the
// failures and the senders stored under the form the domain normalizer gives, in the failures
// dataset's sender filter and grouping and in the senders dataset's search, because the server gives
// each statement the domain in that form and none of the statements it reads lowercases a domain
// (ADR-0016).
func TestADomainTypedInAnyCaseFindsItsStoredForm(t *testing.T) {
	r := newRig(t)
	const stored = "bi\u0307nk.example"
	r.exec(`INSERT INTO messages (account_id, message_id, thread_id, from_email, from_domain, sent_at, has_attachments, sender_class, scan_state)
		VALUES ($1, 'm-dotted', 't-dotted', $2, $3, $4, false, 'normal', 'scanned')`, personal, "alerts@B\u0130NK.example", stored, now)
	r.exec(`INSERT INTO senders (account_id, domain, message_count, sender_class) VALUES ($1, $2, 1, 'normal')`, personal, stored)
	r.exec(`INSERT INTO job_run_failures (account_id, run_id, seq, item_kind, item_id, page, error_class, error_summary, attempts, first_at, last_at, disposition, recovered_by)
		VALUES ($1, 'r-0901', 3, 'message', 'm-dotted', 7, 'provider_error', NULL, 1, $2, $2, 'pending', NULL)`, personal, now)
	b := r.browser()
	lens := func(values url.Values) string { return "/api/personal/lens?" + values.Encode() }

	type item struct {
		ItemID string `json:"item_id"`
	}
	typed := "B\u0130NK.EXAMPLE"
	for _, filter := range []string{typed, stored} {
		got := decode[rowsPage[item]](t, b.get(lens(url.Values{"dataset": {"failures"}, "run": {"r-0901"}, "level": {"3"}, "sender": {filter}})))
		if diff := cmp.Diff([]item{{ItemID: "m-dotted"}}, got.Rows, compare.Options); diff != "" {
			t.Errorf("sender=%+q (-want +got):\n%s", filter, diff)
		}
	}
	dropped := decode[rowsPage[item]](t, b.get(lens(url.Values{"dataset": {"failures"}, "run": {"r-0901"}, "level": {"3"}, "sender": {"!" + typed}})))
	for _, i := range dropped.Rows {
		if i.ItemID == "m-dotted" {
			t.Errorf("sender=!%+q keeps the message stored under %+q", typed, stored)
		}
	}
	if dropped.Total.Count != 2 {
		t.Errorf("sender=!%+q leaves %d items, want r-0901's other 2", typed, dropped.Total.Count)
	}
	var g groups
	read(t, b.h, "/api/{account}/lens", lens(url.Values{"dataset": {"failures"}, "run": {"r-0901"}, "level": {"1"}, "group": {"sender"}}), &g)
	keys := map[string]int64{}
	for _, row := range g.Rows {
		if k, ok := row.Key["sender"].(string); ok {
			keys[k] = row.Count
		}
	}
	if diff := cmp.Diff(map[string]int64{"": 1, stored: 1}, keys, compare.Options); diff != "" {
		t.Errorf("r-0901's groups by sender (-want +got):\n%s", diff)
	}

	type sender struct {
		Domain string `json:"domain"`
	}
	for _, search := range []string{"B\u0130NK", "\u0130N"} {
		got := decode[rowsPage[sender]](t, b.get(lens(url.Values{"dataset": {"senders"}, "level": {"3"}, "search": {search}})))
		if diff := cmp.Diff([]sender{{Domain: stored}}, got.Rows, compare.Options); diff != "" {
			t.Errorf("the senders search for %+q (-want +got):\n%s", search, diff)
		}
	}
}
