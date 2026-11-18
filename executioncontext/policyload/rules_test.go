package policyload_test

import (
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/alertingrules"
)

// rulesFile is the chart's rules file, relative to this package.
const rulesFile = "../../packaging/chart/rules/alerting-rules.yaml"

// TestAlertingRules runs promtool's unit tests of the policy reload rule (ADR-0077). The rule fires
// while a process's latest reload failed, from the first sample when the failure came before the
// first scrape, and stays silent once a reload succeeds and for a process whose reloads succeed. A
// missing promtool fails the test rather than skipping it, because a skipped test passes in the job
// that was meant to run it.
func TestAlertingRules(t *testing.T) {
	alertingrules.Test(t, "testdata/alerting-rules-test.yaml")
}

// Every policy loader series the alerting rules read is one a Loader registers under that exact
// name, so the reload rule never watches a name nothing emits (ADR-0076).
func TestTheRulesReadOnlyEmittedSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	if _, err := policyload.New(nil, []string{"acct"}, reg, lookups); err != nil {
		t.Fatal(err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	emitted := map[string]bool{}
	for _, f := range families {
		emitted[f.GetName()] = true
	}
	alertingrules.RequireReadOnlyEmitted(t, rulesFile, `mediated_mailbox_policyload_[a-z_]+`, 1, emitted)
}
