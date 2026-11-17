package process_test

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

// shared is what the test's slot holds, built once per process.
type shared struct{}

// What a slot holds is built by the first call of its owner's function and returned by every later
// one, from two endpoints and a job kind alike. It is built with the process's own capabilities,
// labelled with the program's name and no job kind, whichever part asked first, so its series and its
// records do not depend on build order. A build that fails keeps nothing (ADR-0142).
func TestASlotHoldsOneThingBuiltFromTheProcess(t *testing.T) {
	for _, order := range []string{"a job kind asks first", "an endpoint asks first"} {
		t.Run(order, func(t *testing.T) {
			slot := process.NewOnePerProcess[*shared]("shared")
			var builds atomic.Int64
			var got []*shared
			ask := func(ctx context.Context, p process.Capabilities) error {
				s, err := slot.Get(ctx, p, func(_ context.Context, sp process.Capabilities) (*shared, func(), error) {
					builds.Add(1)
					counter := prometheus.NewCounter(prometheus.CounterOpts{Name: "mediated_mailbox_test_shared_total", Help: "A shared thing's counter."})
					return &shared{}, nil, sp.Metrics().Register(counter)
				})
				got = append(got, s)
				return err
			}
			endpoints := []process.Endpoint[serving]{}
			for _, pattern := range []string{"/a/", "/b/"} {
				endpoints = append(endpoints, process.Endpoint[serving]{Pattern: pattern, Build: func(ctx context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
					return http.NotFoundHandler(), ask(ctx, p)
				}})
			}
			jobs := []process.Job[serving]{{Name: "kind", Build: func(ctx context.Context, p process.Capabilities, _ serving) (schedule.Plan, error) {
				return idlePlan(), ask(ctx, p)
			}}}
			// The endpoints are built before the job kinds, so a spec of job kinds alone has a job kind ask
			// first.
			spec := process.Spec[serving]{Name: "test", Endpoints: endpoints, Jobs: jobs}
			if order == "a job kind asks first" {
				spec = process.Spec[serving]{Name: "test", Jobs: append(jobs, process.Job[serving]{Name: "other", Build: func(ctx context.Context, p process.Capabilities, _ serving) (schedule.Plan, error) {
					return idlePlan(), ask(ctx, p)
				}})}
			}
			r := serveSpec(t, spec)
			if builds.Load() != 1 || len(got) < 2 || got[0] != got[1] || got[len(got)-1] != got[0] {
				t.Errorf("the slot was built %d times and returned %v, want one thing built once", builds.Load(), got)
			}
			_, metrics, _, err := get(t, &http.Client{}, "http://"+r.probes+"/metrics")
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(metrics, "\n"+`mediated_mailbox_test_shared_total{component="test"} 0`) {
				t.Errorf("the shared thing's series is not labelled with the program alone:\n%s", metrics)
			}
		})
	}

	slot := process.NewOnePerProcess[int]("failing")
	attempts := 0
	err := process.RunContext(t.Context(), nil, nil, io.Discard, io.Discard, process.Spec[once]{Name: "test", Task: task(func(ctx context.Context, p process.Capabilities) error {
		for range 2 {
			if _, err := slot.Get(ctx, p, func(context.Context, process.Capabilities) (int, func(), error) {
				attempts++
				return 0, nil, errors.New("not yet")
			}); err == nil {
				return errors.New("a failed build was returned")
			}
		}
		return nil
	})})
	if err != nil || attempts != 2 {
		t.Errorf("the run returned %v after %d builds, want a failed build kept nowhere and tried again", err, attempts)
	}
}
