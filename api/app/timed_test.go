package app

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/service"
)

// Each operation's handler is timed under the operation's name for as long as it runs, and the
// handler's answer passes through unchanged (ADR-0125).
func TestEachOperationIsTimedForItsHandler(t *testing.T) {
	const took = 20 * time.Millisecond
	reg := prometheus.NewRegistry()
	ops, err := timed([]service.Operation{{
		Name: "slow",
		Handle: func(context.Context, string, json.RawMessage) (json.RawMessage, error) {
			time.Sleep(took)
			return json.RawMessage(`{"ok":true}`), nil
		},
	}}, reg)
	if err != nil {
		t.Fatal(err)
	}
	out, err := ops[0].Handle(t.Context(), "personal", json.RawMessage(`{}`))
	if err != nil || string(out) != `{"ok":true}` {
		t.Fatalf("the timed handler returned %s, %v", out, err)
	}
	families, err := reg.Gather()
	if err != nil {
		t.Fatal(err)
	}
	var count uint64
	var sum float64
	for _, f := range families {
		if f.GetName() != operationDurationName {
			continue
		}
		for _, m := range f.GetMetric() {
			if len(m.GetLabel()) == 1 && m.GetLabel()[0].GetName() == "operation" && m.GetLabel()[0].GetValue() == "slow" {
				count, sum = m.GetHistogram().GetSampleCount(), m.GetHistogram().GetSampleSum()
			}
		}
	}
	if count != 1 || sum < took.Seconds() {
		t.Errorf("the operation was observed %d times for %v seconds, want once for at least %v", count, sum, took.Seconds())
	}
}
