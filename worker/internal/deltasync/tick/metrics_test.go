package tick_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

// A tick's messages indexed and scanned are added to the account's count by stage, and an account no
// tick serves any longer loses its count (ADR-0125).
func TestATicksMessagesAreCountedByStage(t *testing.T) {
	reg := prometheus.NewRegistry()
	m, err := tick.NewMetrics(reg)
	if err != nil {
		t.Fatal(err)
	}
	m.Count("acct", tick.Result{Indexed: 3, Scanned: 2})
	m.Count("acct", tick.Result{Indexed: 1})
	m.Count("other", tick.Result{Scanned: 4})
	m.Forget("other")
	want := map[string]float64{"acct " + series.StageIndexed: 4, "acct " + series.StageScanned: 2}
	if diff := cmp.Diff(want, counted(t, reg), compare.Options); diff != "" {
		t.Errorf("messages counted (-want +got):\n%s", diff)
	}
}

// counted returns the messages series on reg, keyed by account and stage.
func counted(t *testing.T, reg prometheus.Gatherer) map[string]float64 {
	t.Helper()
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, f := range families {
		if f.GetName() != series.MessagesName {
			continue
		}
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			out[labels["account"]+" "+labels["stage"]] = m.GetCounter().GetValue()
		}
	}
	return out
}
