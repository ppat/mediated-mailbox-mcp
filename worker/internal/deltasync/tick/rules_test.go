package tick_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/alertingrules"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick"
)

// rulesFile is the chart's rules file, relative to this package.
const rulesFile = "../../../../packaging/chart/rules/alerting-rules.yaml"

// VERIFICATIONS' row for invalidating the sync cursor, D4's part, the alert. promtool's unit tests of
// the cursor gap rule. It fires from a gap's sample for about an hour, also when the series' first
// sample already counts a gap, and stays silent for an account with no gap and across a restart that
// starts the count at zero (ADR-0077, ADR-0105). A missing promtool fails the test rather than
// skipping it, because a skipped test passes in the job that was meant to run it.
func TestAlertingRules(t *testing.T) {
	alertingrules.Test(t, "testdata/alerting-rules-test.yaml")
}

// Every delta sync series the alerting rules read is one the tick's metrics register under that exact
// name, so the gap rule never watches a name nothing emits (ADR-0076).
func TestTheRulesReadOnlyEmittedSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := tick.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	m.Count("acct", tick.Result{Gap: true, Scanning: true})
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	emitted := map[string]bool{}
	for _, f := range families {
		emitted[f.GetName()] = true
	}
	alertingrules.RequireReadOnlyEmitted(t, rulesFile, `mediated_mailbox_sync_[a-z_]+`, 1, emitted)
}
