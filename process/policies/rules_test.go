package policies_test

import (
	"context"
	"errors"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/policies"
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

// Every policy series the alerting rules read is one the policies register under that exact name at
// their first load, so the reload rule never watches a name nothing emits (ADR-0076).
func TestTheRulesReadOnlyEmittedSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	if err := policies.New(unreachable{}, reg, nil).Load(t.Context(), []string{"acct"}); err == nil {
		t.Fatal("a load over a database that cannot be reached succeeded")
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

// unreachable is a database every transaction of which fails to begin.
type unreachable struct{}

func (unreachable) Begin(context.Context) (pgx.Tx, error) {
	return nil, errors.New("the database cannot be reached")
}
