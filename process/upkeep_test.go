package process_test

import (
	"context"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

// upkept is a slot whose thing asks for upkeep that counts its runs.
type upkept struct{ runs atomic.Int64 }

// upkeep returns the owner's function of a slot whose build asks for upkeep running every interval.
func upkeep(slot *process.OnePerProcess[*upkept], interval time.Duration) func(ctx context.Context, p process.Capabilities) (*upkept, error) {
	return func(ctx context.Context, p process.Capabilities) (*upkept, error) {
		return slot.Get(ctx, p, func(_ context.Context, sp process.Capabilities) (*upkept, func(), error) {
			u := &upkept{}
			err := process.Upkeep(sp, schedule.Job{Interval: interval, Run: func(context.Context, schedule.Ask) error {
				u.runs.Add(1)
				return nil
			}})
			return u, nil, err
		})
	}
}

// A shared thing's upkeep runs on the process's scheduler from when the process serves, in a process
// of endpoints with no job kind as in one of job kinds, at once and then on its interval. It belongs to
// no job kind, so none of the scheduler's series observe it, and a process whose scheduler runs only
// upkeep registers none of them (ADR-0119, ADR-0125, ADR-0142).
func TestAnOwnersUpkeepRunsOnTheSchedulerUnobserved(t *testing.T) {
	t.Run("endpoints alone", func(t *testing.T) {
		slot := process.NewOnePerProcess[*upkept]("kept")
		get := upkeep(slot, 20*time.Millisecond)
		var u *upkept
		r := serveSpec(t, process.Spec[serving]{Name: "test", Endpoints: []process.Endpoint[serving]{{Pattern: "/", Build: func(ctx context.Context, p process.Capabilities, _ serving) (http.Handler, error) {
			var err error
			if u, err = get(ctx, p); err != nil {
				return nil, err
			}
			// A series of the scheduler's own name registered here leaves the process serving only
			// while the scheduler registers none of its series.
			running := prometheus.NewGauge(prometheus.GaugeOpts{Name: "mediated_mailbox_job_runs_running", Help: "A series under the scheduler's name."})
			return http.NotFoundHandler(), p.Metrics().Register(running)
		}}}})
		for deadline := time.Now().Add(5 * time.Second); u.runs.Load() < 3; time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("the upkeep ran %d times in five seconds at an interval of 20ms", u.runs.Load())
			}
		}
		_, metrics, _, err := get2(t, "http://"+r.probes+"/metrics")
		if err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"mediated_mailbox_job_runs_started_total", "mediated_mailbox_job_runs_finished_total", "mediated_mailbox_job_last_success_timestamp_seconds"} {
			if strings.Contains(metrics, name) {
				t.Errorf("a process whose scheduler runs only upkeep serves the scheduler's series %s", name)
			}
		}
	})
	t.Run("beside a job kind", func(t *testing.T) {
		slot := process.NewOnePerProcess[*upkept]("kept")
		get := upkeep(slot, 20*time.Millisecond)
		var u *upkept
		r := serveSpec(t, process.Spec[jobsOnly]{Name: "test", Jobs: []process.Job[jobsOnly]{{Name: "kind", Build: func(ctx context.Context, p process.Capabilities, _ jobsOnly) (schedule.Plan, error) {
			var err error
			u, err = get(ctx, p)
			return idlePlan(), err
		}}}})
		for deadline := time.Now().Add(5 * time.Second); u.runs.Load() < 3; time.Sleep(10 * time.Millisecond) {
			if time.Now().After(deadline) {
				t.Fatalf("the upkeep ran %d times in five seconds at an interval of 20ms", u.runs.Load())
			}
		}
		_, metrics, _, err := get2(t, "http://"+r.probes+"/metrics")
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(metrics, `job_kind="kind"`) || strings.Contains(metrics, `job_kind="kept"`) {
			t.Errorf("the scheduler's series do not observe the job kind alone:\n%s", metrics)
		}
	})
}

// get2 asks url with a client of its own.
func get2(t *testing.T, url string) (int, string, http.Header, error) {
	t.Helper()
	return get(t, &http.Client{}, url)
}

// Upkeep is asked for while the process is being built, once per shared thing, and not by a program
// that does one piece of work and exits, which runs no scheduler. Each other ask is refused (ADR-0142).
func TestUpkeepIsRefusedOutsideABuildOfAProcessThatServes(t *testing.T) {
	job := schedule.Job{Interval: time.Hour, Run: func(context.Context, schedule.Ask) error { return nil }}
	err := process.RunContext(t.Context(), nil, nil, io.Discard, io.Discard, process.Spec[once]{Name: "test", Task: task(func(ctx context.Context, p process.Capabilities) error {
		_, err := process.NewOnePerProcess[int]("kept").Get(ctx, p, func(_ context.Context, sp process.Capabilities) (int, func(), error) {
			return 0, nil, process.Upkeep(sp, job)
		})
		return err
	})})
	if err == nil || !strings.Contains(err.Error(), "one piece of work") {
		t.Errorf("upkeep asked for in a task ended the run with %v, want it refused", err)
	}

	var second, late error
	var held process.Capabilities
	r := serveSpec(t, process.Spec[jobsOnly]{Name: "test", Jobs: []process.Job[jobsOnly]{{Name: "kind", Build: func(ctx context.Context, p process.Capabilities, _ jobsOnly) (schedule.Plan, error) {
		_, err := process.NewOnePerProcess[int]("kept").Get(ctx, p, func(_ context.Context, sp process.Capabilities) (int, func(), error) {
			if err := process.Upkeep(sp, job); err != nil {
				return 0, nil, err
			}
			second = process.Upkeep(sp, job)
			held = sp
			return 0, nil, nil
		})
		return idlePlan(), err
	}}}})
	if second == nil {
		t.Error("a second upkeep for one shared thing was accepted")
	}
	late = process.Upkeep(held, job)
	if late == nil || !strings.Contains(late.Error(), "once the process was built") {
		t.Errorf("upkeep asked for once the process was built returned %v, want it refused", late)
	}
	r.stop()
}

// optioned is what the test's slot holds.
type optioned struct{ name string }

// The option an owner exports for a test fills its slot before anything asks, so every part that asks
// gets what the test made and nothing is built, while a run without it builds as the owner's function
// says. Run takes no option, so a main.go cannot substitute (ADR-0142).
func TestASubstituteFillsTheSlotBeforeAnythingAsks(t *testing.T) {
	slot := process.NewOnePerProcess[*optioned]("optioned")
	built := 0
	var got []*optioned
	spec := process.Spec[jobsOnly]{Name: "test", Jobs: []process.Job[jobsOnly]{{Name: "kind", Build: func(ctx context.Context, p process.Capabilities, _ jobsOnly) (schedule.Plan, error) {
		for range 2 {
			o, err := slot.Get(ctx, p, func(context.Context, process.Capabilities) (*optioned, func(), error) {
				built++
				return &optioned{name: "built"}, nil, nil
			})
			if err != nil {
				return schedule.Plan{}, err
			}
			got = append(got, o)
		}
		return idlePlan(), nil
	}}}}
	made := &optioned{name: "made by the test"}
	run := func(options ...process.Option) {
		t.Helper()
		ctx, cancel := context.WithCancel(t.Context())
		done := make(chan error, 1)
		go func() {
			done <- process.RunContext(ctx, []string{"--probe_listen=127.0.0.1:0"}, nil, io.Discard, io.Discard, spec, options...)
		}()
		for deadline := time.Now().Add(5 * time.Second); len(got) < 2 && time.Now().Before(deadline); time.Sleep(10 * time.Millisecond) {
		}
		cancel()
		if err := <-done; err != nil {
			t.Fatal(err)
		}
	}

	run(slot.Substitute(made))
	if built != 0 || len(got) != 2 || got[0] != made || got[1] != made {
		t.Errorf("with the substitute the slot was built %d times and gave %v, want the test's thing and no build", built, got)
	}
	got = nil
	run()
	if built != 1 || len(got) != 2 || got[0].name != "built" || got[0] != got[1] {
		t.Errorf("without the substitute the slot was built %d times and gave %v, want one build", built, got)
	}
}

// A shared thing reads the revision of its settings from the process, which holds the revision the
// configuration library took of each top-level key, so a change to that key's value changes it and
// a change to another key does not (ADR-0078, ADR-0142).
func TestARevisionIsReadFromTheProcess(t *testing.T) {
	revision := func(args ...string) string {
		t.Helper()
		var got string
		err := process.RunContext(t.Context(), args, nil, io.Discard, io.Discard, process.Spec[once]{Name: "test", Task: task(func(_ context.Context, p process.Capabilities) error {
			got = process.Revision(p, "own")
			return nil
		})})
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	base := revision()
	if base == "" || revision("--own.value=1") != base || revision("--log_level=warn") != base || revision("--own.value=2") == base {
		t.Errorf("the revision of own was %q, and did not follow own's value alone", base)
	}
	if got := process.Revision(nil, "own"); got != "" {
		t.Errorf("capabilities no process handed out read revision %q, want none", got)
	}
}
