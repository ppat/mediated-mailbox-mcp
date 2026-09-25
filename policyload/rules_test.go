package policyload_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/policyload"
)

// rulesFile is the chart's rules file, relative to this package.
const rulesFile = "../packaging/chart/alerting-rules.yaml"

// TestAlertingRules runs promtool's unit tests of the policy reload rule (ADR-0077). The rule fires
// while a process's latest reload failed, from the first sample when the failure came before the
// first scrape, and stays silent once a reload succeeds and for a process whose reloads succeed. A
// missing promtool fails the test rather than skipping it, because a skipped test passes in the job
// that was meant to run it.
func TestAlertingRules(t *testing.T) {
	path, err := exec.LookPath("promtool")
	if err != nil {
		t.Fatalf("promtool is not on PATH. Run the tests through mise: %v", err)
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(t.Context(), path, "test", "rules", "testdata/alerting-rules-test.yaml")
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	var exit *exec.ExitError
	switch {
	case errors.As(err, &exit):
		t.Errorf("promtool test rules failed: %v\n%s", err, out.String())
	case err != nil:
		t.Fatalf("running promtool: %v", err)
	}
}

// Every policy loader series the alerting rules read is one a Loader registers under that exact
// name, so the reload rule never watches a name nothing emits (ADR-0076).
func TestTheRulesReadOnlyEmittedSeries(t *testing.T) {
	reg := prometheus.NewRegistry()
	if _, err := policyload.New(nil, []string{"acct"}, reg); err != nil {
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
	src, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatal(err)
	}
	read := regexp.MustCompile(`mediated_mailbox_policyload_[a-z_]+`).FindAllString(string(src), -1)
	if len(read) == 0 {
		t.Fatal("no alerting rule reads a policy loader series")
	}
	for _, name := range read {
		if !emitted[name] {
			t.Errorf("the rules read %s, which no Loader emits", name)
		}
	}
}
