package schedule_test

import (
	"context"
	"errors"
	"slices"
	"strconv"
	"sync"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

// listing is a plan's keys, which the test changes between listings, and what the plan saw.
type listing struct {
	mu      sync.Mutex
	keys    []string
	err     error
	running map[string]bool
	dropped []string
	built   []string
}

func (l *listing) set(keys []string, err error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.keys, l.err = keys, err
}

func (l *listing) plan(recheck time.Duration) schedule.Plan {
	return schedule.Plan{
		Keys: func(context.Context) ([]string, error) {
			l.mu.Lock()
			defer l.mu.Unlock()
			return slices.Clone(l.keys), l.err
		},
		Recheck: recheck,
		Backoff: schedule.Backoff{Base: time.Second, Max: time.Second},
		Job: func(key string) (schedule.Job, error) {
			l.mu.Lock()
			l.built = append(l.built, key)
			l.mu.Unlock()
			return schedule.Job{Run: func(ctx context.Context, _ schedule.Ask) error {
				l.mu.Lock()
				l.running[key] = true
				l.mu.Unlock()
				<-ctx.Done()
				l.mu.Lock()
				l.running[key] = false
				l.mu.Unlock()
				return nil
			}}, nil
		},
		Dropped: func(key string) {
			l.mu.Lock()
			defer l.mu.Unlock()
			// A dropped key is told only once its job has returned.
			if l.running[key] {
				l.dropped = append(l.dropped, key+" while running")
				return
			}
			l.dropped = append(l.dropped, key)
		},
	}
}

func (l *listing) state() (running []string, dropped, built []string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for key, on := range l.running {
		if on {
			running = append(running, key)
		}
	}
	slices.Sort(running)
	return running, slices.Clone(l.dropped), slices.Clone(l.built)
}

// A plan's listing keeps one job per key it lists, adding the job of a key it finds and removing the
// job of a key it no longer finds, each once and told to the plan only after its job has returned.
// The listing runs at once, again every recheck and whenever woken, and the first listing that
// succeeds is told once, a failed listing before it backing off and changing no job (ADR-0119,
// ADR-0142).
func TestAPlansListingAddsAndDropsJobsByKey(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := &listing{running: map[string]bool{}}
		l.set(nil, errors.New("the accounts cannot be read"))
		metrics := newRecorder()
		s := schedule.New(t.Context(), metrics)
		defer s.Stop()
		var listed int
		var wake func()
		plan := l.plan(time.Minute)
		plan.Woken = func(w func()) { wake = w }
		if err := s.EnsurePlan("kind", plan, nil, func() { listed++ }); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		running, _, _ := l.state()
		equal(t, "after a failed listing, the first listing told", listed, 0)
		equal(t, "after a failed listing, the jobs running", running, []string(nil))

		l.set([]string{"a", "b"}, nil)
		time.Sleep(time.Second)
		synctest.Wait()
		running, _, _ = l.state()
		equal(t, "once the backoff ended, the first listing told", listed, 1)
		equal(t, "once the backoff ended, the jobs running", running, []string{"a", "b"})

		l.set([]string{"b", "c"}, nil)
		wake()
		synctest.Wait()
		running, dropped, built := l.state()
		equal(t, "after a wake, the jobs running", running, []string{"b", "c"})
		equal(t, "after a wake, the keys dropped", dropped, []string{"a"})
		equal(t, "after a wake, the jobs built", built, []string{"a", "b", "c"})

		l.set([]string{"c"}, nil)
		time.Sleep(time.Minute)
		synctest.Wait()
		running, dropped, _ = l.state()
		equal(t, "after the recheck, the jobs running", running, []string{"c"})
		equal(t, "after the recheck, the keys dropped", dropped, []string{"a", "b"})
		equal(t, "after every listing, the first listing told", listed, 1)
	})
}

// A listing that names an empty key is refused as the listing's failure, since the listing's own job
// is keyed by the job kind alone and an empty key would name it, so no job is kept for it.
func TestAListingThatNamesAnEmptyKeyFails(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		l := &listing{running: map[string]bool{}}
		l.set([]string{"a", ""}, nil)
		s := schedule.New(t.Context(), newRecorder())
		defer s.Stop()
		listed := false
		if err := s.EnsurePlan("kind", l.plan(0), nil, func() { listed = true }); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		running, _, built := l.state()
		equal(t, "the listing told as succeeded", listed, false)
		equal(t, "the jobs built", built, []string(nil))
		equal(t, "the jobs running", running, []string(nil))
	})
}

// A plan with no listing or no job is refused when it is ensured, since its listing could never run,
// and no job is kept for it.
func TestAPlanItCouldNeverRunIsRefused(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := schedule.New(t.Context(), newRecorder())
		defer s.Stop()
		l := &listing{running: map[string]bool{}}
		full := l.plan(0)
		for name, plan := range map[string]schedule.Plan{
			"no listing": {Job: full.Job},
			"no job":     {Keys: full.Keys},
		} {
			if err := s.EnsurePlan("kind", plan, nil, nil); err == nil {
				t.Errorf("a plan with %s was ensured", name)
			}
		}
	})
}

// A plan's listing exports the lateness bound the plan gives it, so the alerting rule on a job's
// latest success reads the listing late only once that bound has passed, as it reads each job kind's
// reload today (ADR-0077, ADR-0119).
func TestAPlansListingExportsItsLatenessBound(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()
		metrics, err := schedule.NewMetrics(registry)
		if err != nil {
			t.Fatal(err)
		}
		s := schedule.New(t.Context(), metrics)
		defer s.Stop()
		l := &listing{running: map[string]bool{}}
		plan := l.plan(time.Minute)
		plan.Late = 5 * time.Minute
		if err := s.EnsurePlan("kind", plan, nil, nil); err != nil {
			t.Fatal(err)
		}
		synctest.Wait()
		families, err := registry.Gather()
		if err != nil {
			t.Fatal(err)
		}
		var got []string
		for _, f := range families {
			if f.GetName() != "mediated_mailbox_job_late_after_seconds" {
				continue
			}
			for _, m := range f.GetMetric() {
				labels := map[string]string{}
				for _, l := range m.GetLabel() {
					labels[l.GetName()] = l.GetValue()
				}
				got = append(got, labels["job_kind"]+"/"+labels["account"]+" "+strconv.FormatFloat(m.GetGauge().GetValue(), 'f', -1, 64))
			}
		}
		equal(t, "the listing's lateness bound", got, []string{"kind/ 300"})
	})
}
