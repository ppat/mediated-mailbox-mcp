package schedule_test

import (
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/alertingrules"
)

// rulesFile is the chart's rules file, relative to this package.
const rulesFile = "../../packaging/chart/rules/alerting-rules.yaml"

// promtool's unit tests of the worker's rules (ADR-0077, ADR-0119). The absence rule fires once no
// worker has been scraped for five minutes, and the rule on a job's latest success fires for each job
// whose latest success has aged past the bound it exports, and for no other. A missing promtool fails
// the test rather than skipping it, because a skipped test passes in the job that was meant to run it.
func TestAlertingRules(t *testing.T) {
	alertingrules.Test(t, "testdata/alerting-rules-test.yaml")
}

// Every job series the alerting rules read is one the scheduler's metrics register under that exact
// name, so no rule watches a name nothing emits (ADR-0076).
func TestTheRulesReadOnlyEmittedSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := schedule.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	key := schedule.Key{Kind: "sync", ID: "acct"}
	m.Ensured(key, time.Minute, time.Now())
	m.Started(key)
	m.Finished(key, schedule.Succeeded, time.Second)
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	emitted := map[string]bool{}
	for _, f := range families {
		emitted[f.GetName()] = true
	}
	alertingrules.RequireReadOnlyEmitted(t, rulesFile, `mediated_mailbox_job_[a-z_]+`, 1, emitted)
}
