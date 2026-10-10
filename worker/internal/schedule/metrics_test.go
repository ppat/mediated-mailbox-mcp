package schedule_test

import (
	"context"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// gathered returns every sample registry holds, keyed by the series' name and its job_kind, account and
// outcome labels.
func gathered(t *testing.T, registry prometheus.Gatherer) map[string]float64 {
	t.Helper()
	families, err := registry.Gather()
	if err != nil {
		t.Fatal(err)
	}
	out := map[string]float64{}
	for _, f := range families {
		for _, m := range f.GetMetric() {
			labels := map[string]string{}
			for _, l := range m.GetLabel() {
				labels[l.GetName()] = l.GetValue()
			}
			key := f.GetName()
			for _, name := range []string{"job_kind", "account", "outcome"} {
				if v, ok := labels[name]; ok {
					key += " " + name + "=" + v
				}
			}
			switch {
			case m.GetGauge() != nil:
				out[key] = m.GetGauge().GetValue()
			case m.GetCounter() != nil:
				out[key] = m.GetCounter().GetValue()
			case m.GetHistogram() != nil:
				out[key] = float64(m.GetHistogram().GetSampleCount())
			}
		}
	}
	return out
}

// A job's series start at its ensure, its latest success moves at a run that succeeds and at a unit a
// run made durable and at nothing else, its backoff reads the delay while it waits one, every run is
// counted by how it ended, and a removed job's own series go with it (ADR-0119, ADR-0077).
func TestTheJobSeriesFollowEveryWayARunEnds(t *testing.T) {
	// One of the runs panics, so the test runs in a process of its own, as the panic cases do.
	inChild(t, "every way a run ends", func(t *testing.T) { synctest.Test(t, everyWayARunEnds) })
}

func everyWayARunEnds(t *testing.T) {
	registry := prometheus.NewRegistry()
	m, err := schedule.NewMetrics(registry)
	if err != nil {
		t.Fatal(err)
	}
	t0 := time.Now()
	at := func(d time.Duration) float64 { return float64(t0.Add(d).UnixMilli()) / 1000 }
	s := schedule.New(t.Context(), m)
	// A test that fails stops every job before the bubble ends, so the failure is reported.
	defer s.Stop()
	key := schedule.Key{Kind: "sync", ID: "a"}
	last := "mediated_mailbox_job_last_success_timestamp_seconds job_kind=sync account=a"
	backoff := "mediated_mailbox_job_backoff_seconds job_kind=sync account=a"
	steps := make(chan func(context.Context, schedule.Ask) error)
	ensure(t, s, key, schedule.Job{
		Interval: time.Hour, Late: 15 * time.Minute, Backoff: schedule.Backoff{Base: time.Minute, Max: time.Hour},
		Run: func(ctx context.Context, ask schedule.Ask) error {
			select {
			case step := <-steps:
				return step(ctx, ask)
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	})
	synctest.Wait()
	got := gathered(t, registry)
	if got[last] != at(0) || got["mediated_mailbox_job_late_after_seconds job_kind=sync account=a"] != 900 {
		t.Fatalf("at the ensure: latest success %v, want %v, and the bound %v, want 900", got[last], at(0), got["mediated_mailbox_job_late_after_seconds job_kind=sync account=a"])
	}
	if got["mediated_mailbox_job_runs_running job_kind=sync"] != 1 {
		t.Fatalf("%v runs running while one runs, want 1", got["mediated_mailbox_job_runs_running job_kind=sync"])
	}

	// A run that fails sets no success, and its backoff reads one minute while it waits.
	time.Sleep(time.Minute)
	steps <- func(context.Context, schedule.Ask) error { return errFailed }
	synctest.Wait()
	got = gathered(t, registry)
	if got[last] != at(0) || got[backoff] != 60 {
		t.Fatalf("after a failed run: latest success %v, want %v, and backoff %v, want 60", got[last], at(0), got[backoff])
	}

	// After the backoff, a run that answers it was not due sets no success either.
	time.Sleep(time.Minute)
	synctest.Wait()
	got = gathered(t, registry)
	if got[backoff] != 0 {
		t.Fatalf("the backoff reads %v once it ended, want 0", got[backoff])
	}
	steps <- func(context.Context, schedule.Ask) error { return schedule.ErrNotDue }
	synctest.Wait()
	if got = gathered(t, registry); got[last] != at(0) {
		t.Fatalf("after a run that was not due: latest success %v, want %v", got[last], at(0))
	}

	// A unit made durable is a success when it is made, and the run's end one more.
	s.Wake(key)
	synctest.Wait()
	time.Sleep(time.Minute)
	steps <- func(_ context.Context, ask schedule.Ask) error {
		ask.Progressed()
		if got := gathered(t, registry); got[last] != at(3*time.Minute) {
			t.Errorf("after a unit made durable: latest success %v, want %v", got[last], at(3*time.Minute))
		}
		time.Sleep(time.Minute)
		return nil
	}
	time.Sleep(time.Minute)
	synctest.Wait()
	if got = gathered(t, registry); got[last] != at(4*time.Minute) {
		t.Fatalf("after a run that succeeded: latest success %v, want %v", got[last], at(4*time.Minute))
	}

	// A run that panics sets no success.
	s.Wake(key)
	synctest.Wait()
	time.Sleep(time.Minute)
	steps <- func(context.Context, schedule.Ask) error { panic("a bug in the run") }
	synctest.Wait()
	got = gathered(t, registry)
	if got[last] != at(4*time.Minute) {
		t.Fatalf("after a run that panicked: latest success %v, want %v", got[last], at(4*time.Minute))
	}
	for outcome, want := range map[string]float64{"succeeded": 1, "failed": 1, "not_due": 1, "panicked": 1} {
		if n := got["mediated_mailbox_job_runs_finished_total job_kind=sync outcome="+outcome]; n != want {
			t.Errorf("%v runs finished %s, want %v", n, outcome, want)
		}
	}
	if n := got["mediated_mailbox_job_runs_started_total job_kind=sync"]; n != 4 {
		t.Errorf("%v runs started, want 4", n)
	}

	// A run cancelled by the job's removal sets no success, and the removal takes the job's series.
	time.Sleep(2 * time.Minute)
	synctest.Wait()
	go s.Remove(key)
	steps <- func(ctx context.Context, _ schedule.Ask) error { <-ctx.Done(); return ctx.Err() }
	synctest.Wait()
	got = gathered(t, registry)
	for _, name := range []string{last, backoff, "mediated_mailbox_job_late_after_seconds job_kind=sync account=a"} {
		if _, ok := got[name]; ok {
			t.Errorf("%s is still emitted after the job was removed", name)
		}
	}
	if n := got["mediated_mailbox_job_runs_finished_total job_kind=sync outcome=cancelled"]; n != 1 {
		t.Errorf("%v runs finished cancelled, want 1", n)
	}
	if n := got["mediated_mailbox_job_runs_running job_kind=sync"]; n != 0 {
		t.Errorf("%v runs running after the removal, want 0", n)
	}
	s.Stop()
}

// The time a job waits for a slot of its kind's limit is reported, so the late rule does not count it:
// the wait in progress by when it began, and once the wait ends, its length joined to the time waited
// since the latest success, which the job's next success clears. A job that takes a free slot reports
// no wait (ADR-0119, ADR-0077).
func TestAJobsWaitForASlotIsReported(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()
		m, err := schedule.NewMetrics(registry)
		if err != nil {
			t.Fatal(err)
		}
		t0 := time.Now()
		at := func(d time.Duration) float64 { return float64(t0.Add(d).UnixMilli()) / 1000 }
		s := schedule.New(t.Context(), m)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		limit := schedule.NewLimit(1)
		release := make(chan struct{})
		holding := schedule.Key{Kind: "backfill", ID: "a"}
		queued := schedule.Key{Kind: "backfill", ID: "c"}
		ensure(t, s, holding, schedule.Job{Interval: 24 * time.Hour, Limit: limit, Backoff: backoff, Run: func(ctx context.Context, _ schedule.Ask) error {
			select {
			case <-release:
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		}})
		synctest.Wait()
		time.Sleep(time.Minute)
		var runs atomic.Int32
		ensure(t, s, queued, schedule.Job{Interval: 24 * time.Hour, Limit: limit, Backoff: backoff, Run: func(context.Context, schedule.Ask) error {
			if runs.Add(1) == 1 {
				return errFailed
			}
			return nil
		}})
		time.Sleep(90 * time.Minute)
		synctest.Wait()
		waiting := "mediated_mailbox_job_slot_waiting_since_timestamp_seconds job_kind=backfill account="
		waited := "mediated_mailbox_job_slot_waited_seconds job_kind=backfill account="
		got := gathered(t, registry)
		if got[waiting+"c"] != at(time.Minute) || got[waited+"c"] != 0 {
			t.Errorf("while c waits: waiting since %v, want %v, and waited %v, want 0", got[waiting+"c"], at(time.Minute), got[waited+"c"])
		}
		if got[waiting+"a"] != 0 || got[waited+"a"] != 0 {
			t.Errorf("a, which took a free slot, reports waiting since %v and waited %v, want none", got[waiting+"a"], got[waited+"a"])
		}

		// c's run takes the slot a releases and fails, so the wait that ended stays counted, until the
		// run after its backoff succeeds and clears it.
		close(release)
		synctest.Wait()
		got = gathered(t, registry)
		if got[waiting+"c"] != 0 || got[waited+"c"] != (90*time.Minute).Seconds() {
			t.Errorf("after c's wait ended: waiting since %v, want 0, and waited %v, want %v", got[waiting+"c"], got[waited+"c"], (90 * time.Minute).Seconds())
		}
		time.Sleep(2 * time.Minute)
		synctest.Wait()
		got = gathered(t, registry)
		if runs.Load() != 2 || got[waited+"c"] != 0 {
			t.Errorf("after c's run %d: waited %v, want its success to clear it", runs.Load(), got[waited+"c"])
		}
		s.Stop()
	})
}

// A job's latest success starts from the one its job kind's recorded state holds, which the job carries
// as its LastSuccess, so a worker that restarts without the job succeeding leaves it ageing, and a job
// with none starts from its ensure (ADR-0119, ADR-0077).
func TestAJobsLatestSuccessStartsFromItsRecordedOne(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()
		m, err := schedule.NewMetrics(registry)
		if err != nil {
			t.Fatal(err)
		}
		s := schedule.New(t.Context(), m)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		recorded := time.Now().Add(-3 * time.Hour)
		block := func(ctx context.Context, _ schedule.Ask) error { <-ctx.Done(); return ctx.Err() }
		ensure(t, s, schedule.Key{Kind: "backfill", ID: "a"}, schedule.Job{Interval: time.Hour, Backoff: backoff, LastSuccess: recorded, Run: block})
		ensure(t, s, schedule.Key{Kind: "backfill", ID: "b"}, schedule.Job{Interval: time.Hour, Backoff: backoff, Run: block})
		synctest.Wait()
		got := gathered(t, registry)
		last := "mediated_mailbox_job_last_success_timestamp_seconds job_kind=backfill account="
		if want := float64(recorded.UnixMilli()) / 1000; got[last+"a"] != want {
			t.Errorf("the job with a recorded success starts at %v, want %v", got[last+"a"], want)
		}
		if want := float64(time.Now().UnixMilli()) / 1000; got[last+"b"] != want {
			t.Errorf("the job with none starts at %v, want its ensure %v", got[last+"b"], want)
		}
		s.Stop()
	})
}

// A job ensured again while its removal waits for its run reports only what its own loop does. The
// removed run, which makes a unit durable as it stops, reports nothing on the new job's series, so the
// new job's latest success stays the one its ensure set (ADR-0119).
func TestARemovedLoopReportsNothingOnTheJobEnsuredInItsPlace(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		registry := prometheus.NewRegistry()
		m, err := schedule.NewMetrics(registry)
		if err != nil {
			t.Fatal(err)
		}
		s := schedule.New(t.Context(), m)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		key := schedule.Key{Kind: "backfill", ID: "a"}
		recorded := time.Now().Add(-3 * time.Hour)
		var runs atomic.Int32
		ensure(t, s, key, schedule.Job{Interval: time.Hour, Backoff: backoff, LastSuccess: recorded, Run: func(ctx context.Context, ask schedule.Ask) error {
			if runs.Add(1) == 1 {
				// The first loop's run ends its unit thirty seconds after it is cancelled, as a page in
				// progress does.
				<-ctx.Done()
				time.Sleep(30 * time.Second)
				ask.Progressed()
				return ctx.Err()
			}
			<-ctx.Done()
			return ctx.Err()
		}})
		synctest.Wait()
		go s.Remove(key)
		time.Sleep(time.Second)
		ensure(t, s, key, schedule.Job{Interval: time.Hour, Backoff: backoff, LastSuccess: recorded, Run: func(ctx context.Context, _ schedule.Ask) error {
			<-ctx.Done()
			return ctx.Err()
		}})
		time.Sleep(time.Minute)
		synctest.Wait()
		last := "mediated_mailbox_job_last_success_timestamp_seconds job_kind=backfill account=a"
		if got, want := gathered(t, registry)[last], float64(recorded.UnixMilli())/1000; got != want {
			t.Errorf("the job ensured again has its latest success at %v, want the %v its ensure set", got, want)
		}
		s.Stop()
	})
}
