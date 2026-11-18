//go:build integration

package tick_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick"
)

// A tick records no page on any event of its timeline or on any failed item, since a tick reads no
// pages, and neither does a gap's recovery. The run screen reads a failed item's page, and the read of
// a run's earliest failed page counts a null as none, so a page recorded as zero would read
// differently from null (ADR-0016). Backfill's runs record a page on every event but their finish and
// failure events and on every failed item, zero for one that concerns no page, which its passes'
// tests hold.
func TestATickRecordsNoPageOnItsEventsOrFailedItems(t *testing.T) {
	conn := superuser(t)
	account := newAccount(t, conn, true)
	nested := waiting("m3", "deep@shop.example", false)
	nested.Body = mail.MessageBody{HTML: strings.Repeat("<div>", 600) + "Deep." + strings.Repeat("</div>", 600)}
	f := mailbox(t, account, waiting("m1", "alerts@bank.example", true), waiting("m2", "orders@shop.example", false), nested)
	p := &direct{port: f, failBody: map[string]error{"m2": mail.ThrottleError{}}}
	mustTick(t, deps(t, syncPool(t), p, listing(t, "bank.example"), account), account)
	// The next tick lifts the bank's rule, so its delisting transition records a retry event, and
	// the body the conversion refuses is recorded as a failed item.
	delete(p.failBody, "m2")
	d := deps(t, syncPool(t), p, listing(t), account)
	mustTick(t, d, account)
	// A tick stopped part way is recorded as failed, with a failure event, by the next one, which
	// meets a cursor gap and records a retry event and a recovery of its own.
	ctx, cancel := context.WithCancel(t.Context())
	stopped := d
	stopped.Provider = &cancelling{direct: p, at: mail.OpChangesSince, cancel: cancel}
	if _, err := tick.Run(ctx, stopped, account); err == nil {
		t.Fatal("the stopped tick succeeded")
	}
	deliver(t, f, message("m4", "next@shop.example", now))
	f.ExpireCursors()
	mustTick(t, d, account)

	events := texts(t, conn, `SELECT r.pass || ' ' || e.kind || ' ' || coalesce(e.page::text, 'null') FROM job_run_events AS e
		JOIN job_runs AS r ON r.account_id = e.account_id AND r.run_id = e.run_id
		WHERE e.account_id = $1 ORDER BY e.seq`, account)
	want := []string{
		"tick start null", "tick progress null", "tick progress null", "tick progress null", "tick finish null",
		"tick start null", "tick retry null", "tick progress null", "tick progress null", "tick finish null",
		"tick start null", "tick failure null",
		"tick start null", "tick retry null", "gap_recovery start null", "gap_recovery progress null", "gap_recovery finish null",
		"tick finish null",
	}
	if diff := cmp.Diff(want, events, compare.Options); diff != "" {
		t.Errorf("the ticks' events and their pages (-want +got):\n%s", diff)
	}
	items := texts(t, conn, `SELECT item_id || ' ' || disposition || ' ' || coalesce(page::text, 'null') FROM job_run_failures
		WHERE account_id = $1 ORDER BY seq`, account)
	if diff := cmp.Diff([]string{"m2 abandoned null", "m3 refused null"}, items, compare.Options); diff != "" {
		t.Errorf("the ticks' failed items and their pages (-want +got):\n%s", diff)
	}
}
