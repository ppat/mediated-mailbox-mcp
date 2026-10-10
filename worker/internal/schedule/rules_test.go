package schedule_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"regexp"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// rulesFile is the chart's rules file, relative to this package.
const rulesFile = "../../../packaging/chart/alerting-rules.yaml"

// promtool's unit tests of the worker's rules (ADR-0077, ADR-0119). The absence rule fires once no
// worker has been scraped for five minutes, and the rule on a job's latest success fires for each job
// whose latest success has aged past the bound it exports, and for no other. A missing promtool fails
// the test rather than skipping it, because a skipped test passes in the job that was meant to run it.
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
	src, err := os.ReadFile(rulesFile)
	if err != nil {
		t.Fatal(err)
	}
	read := regexp.MustCompile(`mediated_mailbox_job_[a-z_]+`).FindAllString(string(src), -1)
	if len(read) == 0 {
		t.Fatal("no alerting rule reads a job series")
	}
	for _, name := range read {
		if !emitted[name] {
			t.Errorf("the rules read %s, which the scheduler does not emit", name)
		}
	}
}
