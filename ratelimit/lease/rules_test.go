package lease_test

import (
	"bytes"
	"errors"
	"io"
	"os"
	"os/exec"
	"testing"

	"github.com/google/go-cmp/cmp"
	"go.yaml.in/yaml/v3"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// The chart and the rules file it embeds, relative to this package.
const (
	chartDir  = "../../packaging/chart"
	rulesFile = chartDir + "/alerting-rules.yaml"
)

// run runs a pinned tool and returns its combined output. A missing tool fails the test rather than
// skipping it, because a skipped test passes in the job that was meant to run it.
func run(t *testing.T, tool string, args ...string) (string, error) {
	t.Helper()
	path, err := exec.LookPath(tool)
	if err != nil {
		t.Fatalf("%s is not on PATH. Run the tests through mise: %v", tool, err)
	}
	var out bytes.Buffer
	cmd := exec.CommandContext(t.Context(), path, args...)
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running %s: %v", tool, err)
	}
	return out.String(), err
}

// TestAlertingRules runs promtool's unit tests of the chart's alerting rules (ADR-0077). Runaway fires
// on two minutes of provider request cost above the hard cap times 120 seconds, from one process or
// summed over two, scraped once a minute, and stays silent below it, with the highest hard cap the
// processes emitted for the account over those two minutes, whatever the provider and after the
// process has gone. The missing-threshold rule fires on cost counted with no hard cap. Collapse
// fires on five minutes at the floor, the stall rule on five minutes with nothing granted, neither
// while no class has asked in the last minute, and the absence rule when the mediator's series are
// missing.
func TestAlertingRules(t *testing.T) {
	if out, err := run(t, "promtool", "test", "rules", "testdata/alerting-rules-test.yaml"); err != nil {
		t.Errorf("promtool test rules failed: %v\n%s", err, out)
	}
}

// rendered renders the chart with the given arguments and returns each document's kind and, for
// a PrometheusRule, its spec.
func rendered(t *testing.T, args ...string) (kinds []string, ruleSpecs []any) {
	t.Helper()
	out, err := run(t, "helm", append([]string{"template", "release", chartDir}, args...)...)
	if err != nil {
		t.Fatalf("helm template failed: %v\n%s", err, out)
	}
	dec := yaml.NewDecoder(bytes.NewBufferString(out))
	for {
		var doc struct {
			Kind string `yaml:"kind"`
			Spec any    `yaml:"spec"`
		}
		err := dec.Decode(&doc)
		if errors.Is(err, io.EOF) {
			return kinds, ruleSpecs
		}
		if err != nil {
			t.Fatalf("reading the rendered chart: %v\n%s", err, out)
		}
		kinds = append(kinds, doc.Kind)
		if doc.Kind == "PrometheusRule" {
			ruleSpecs = append(ruleSpecs, doc.Spec)
		}
	}
}

// TestTheChartShipsTheRulesOnlyWhenSwitchedOn renders the chart with no values set, which must
// hold no PrometheusRule, and with the switch on, which must hold one carrying exactly the rules file
// promtool tests (ADR-0077, ADR-0052).
func TestTheChartShipsTheRulesOnlyWhenSwitchedOn(t *testing.T) {
	if _, specs := rendered(t); len(specs) != 0 {
		t.Errorf("the chart with no values set renders %d PrometheusRule resources, want none", len(specs))
	}
	_, specs := rendered(t, "--set", "alertingRules.enabled=true")
	if len(specs) != 1 {
		t.Fatalf("the chart with the switch on renders %d PrometheusRule resources, want one", len(specs))
	}
	src, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatal(err)
	}
	var want any
	if err := yaml.Unmarshal(src, &want); err != nil {
		t.Fatalf("%s: %v", rulesFile, err)
	}
	if diff := cmp.Diff(want, specs[0], compare.Options); diff != "" {
		t.Errorf("the PrometheusRule's spec differs from %s (-file +rendered):\n%s", rulesFile, diff)
	}
}
