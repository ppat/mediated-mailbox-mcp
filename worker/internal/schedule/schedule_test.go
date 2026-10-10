package schedule_test

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

var errFailed = errors.New("the run failed")

// equal fails the test when got differs from want.
func equal[T any](t *testing.T, what string, got, want T) {
	t.Helper()
	if d := cmp.Diff(want, got, compare.Options); d != "" {
		t.Fatalf("%s (-want +got):\n%s", what, d)
	}
}

// ensure ensures key's job on s and fails the test when the scheduler refuses it.
func ensure(t *testing.T, s *schedule.Scheduler, key schedule.Key, job schedule.Job) {
	t.Helper()
	if err := s.Ensure(key, job); err != nil {
		t.Error(err)
	}
}

// textLogger returns a logger writing text lines to w.
func textLogger(w io.Writer) *slog.Logger { return slog.New(slog.NewTextHandler(w, nil)) }

// run is one run as the recorder saw it.
type run struct {
	key      schedule.Key
	started  time.Duration
	outcome  schedule.Outcome
	finished bool
}

// recorder is an Observer keeping every run in order, and every success, under a lock. Its times are
// offsets from t0, which a test sets to the bubble's start.
type recorder struct {
	t0 time.Time

	mu        sync.Mutex
	runs      []run
	successes map[schedule.Key][]time.Duration
	backoffs  map[schedule.Key][]time.Duration
	removed   []schedule.Key
}

func newRecorder() *recorder {
	return &recorder{t0: time.Now(), successes: map[schedule.Key][]time.Duration{}, backoffs: map[schedule.Key][]time.Duration{}}
}

func (r *recorder) Ensured(key schedule.Key, _ time.Duration, _ time.Time) {}

func (r *recorder) Waiting(schedule.Key, time.Time) {}

func (r *recorder) Waited(schedule.Key, time.Time, time.Time) {}

func (r *recorder) Started(key schedule.Key) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.runs = append(r.runs, run{key: key, started: time.Since(r.t0)})
}

func (r *recorder) Finished(key schedule.Key, outcome schedule.Outcome, _ time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for i := len(r.runs) - 1; i >= 0; i-- {
		if r.runs[i].key == key && !r.runs[i].finished {
			r.runs[i].outcome, r.runs[i].finished = outcome, true
			return
		}
	}
}

func (r *recorder) Succeeded(key schedule.Key, at time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.successes[key] = append(r.successes[key], at.Sub(r.t0))
}

func (r *recorder) BackingOff(key schedule.Key, delay time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.backoffs[key] = append(r.backoffs[key], delay)
}

func (r *recorder) Removed(key schedule.Key) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.removed = append(r.removed, key)
}

// starts returns when each run of key started, as offsets from t0.
func (r *recorder) starts(key schedule.Key) []time.Duration {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []time.Duration
	for _, x := range r.runs {
		if x.key == key {
			out = append(out, x.started)
		}
	}
	return out
}

// outcomes returns how each run of key ended.
func (r *recorder) outcomes(key schedule.Key) []schedule.Outcome {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []schedule.Outcome
	for _, x := range r.runs {
		if x.key == key {
			out = append(out, x.outcome)
		}
	}
	return out
}

func minutes(m ...float64) []time.Duration {
	out := make([]time.Duration, 0, len(m))
	for _, x := range m {
		out = append(out, time.Duration(x*float64(time.Minute)))
	}
	return out
}

var (
	syncKey = schedule.Key{Kind: "sync", ID: "a"}
	backoff = schedule.Backoff{Base: time.Minute, Max: 4 * time.Minute}
)

// A job with an interval is asked at once and then on the interval's phase. A run that outlasts the
// interval is followed at once by one run, asked at the first tick of the phase it missed, and the
// phase then resumes, as a time.Ticker ticks (ADR-0103).
func TestATimedJobIsAskedOnTheIntervalsPhase(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		var n atomic.Int32
		var asked []time.Duration
		var mu sync.Mutex
		err := s.Ensure(syncKey, schedule.Job{Interval: 5 * time.Minute, Backoff: backoff, Run: func(_ context.Context, ask schedule.Ask) error {
			mu.Lock()
			asked = append(asked, ask.At.Sub(r.t0))
			mu.Unlock()
			if n.Add(1) == 2 {
				time.Sleep(17 * time.Minute) // outlasts three intervals, ending at 22
			} else {
				time.Sleep(time.Minute)
			}
			return nil
		}})
		if err != nil {
			t.Fatal(err)
		}
		time.Sleep(39 * time.Minute)
		synctest.Wait()
		s.Stop()
		equal(t, "starts", r.starts(syncKey), minutes(0, 5, 22, 25, 30, 35))
		equal(t, "asked at", asked, minutes(0, 5, 10, 25, 30, 35))
	})
}

// Any number of wakes while a job runs give exactly one further run, and a job never runs twice at
// once.
func TestWakesDuringARunCoalesceIntoOneRun(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		var running atomic.Int32
		ensure(t, s, syncKey, schedule.Job{Interval: time.Hour, Backoff: backoff, Run: func(context.Context, schedule.Ask) error {
			if running.Add(1) > 1 {
				t.Error("two runs of one job at once")
			}
			defer running.Add(-1)
			time.Sleep(10 * time.Minute)
			return nil
		}})
		time.Sleep(time.Minute)
		for range 5 {
			s.Wake(syncKey)
		}
		time.Sleep(30 * time.Minute)
		synctest.Wait()
		s.Stop()
		equal(t, "starts", r.starts(syncKey), minutes(0, 10))
	})
}

// A run of a job is the job's own, so a wake while it runs never starts a second beside it, and the
// run's end is what the scheduler waits for before it is asked again.
func TestARunIsAskedAgainOnlyAfterItReturns(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		release := make(chan struct{})
		var running, peak atomic.Int32
		ensure(t, s, syncKey, schedule.Job{Interval: time.Hour, Backoff: backoff, Run: func(context.Context, schedule.Ask) error {
			c := running.Add(1)
			if c > peak.Load() {
				peak.Store(c)
			}
			defer running.Add(-1)
			<-release
			return nil
		}})
		synctest.Wait()
		s.Wake(syncKey)
		synctest.Wait()
		if got := len(r.starts(syncKey)); got != 1 {
			t.Fatalf("%d runs started while the first held, want 1", got)
		}
		close(release)
		synctest.Wait()
		s.Stop()
		equal(t, "outcomes", r.outcomes(syncKey), []schedule.Outcome{schedule.Succeeded, schedule.Succeeded})
		if peak.Load() != 1 {
			t.Fatalf("%d runs of one job at once", peak.Load())
		}
	})
}

// A limit bounds the runs holding it at once across the jobs of a kind, and jobs holding another
// limit, or none, are not held by it.
func TestALimitBoundsTheRunsOfAJobKindAtOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := schedule.New(t.Context(), newRecorder())
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		limit := schedule.NewLimit(2)
		var now, peak, syncs atomic.Int32
		for _, id := range []string{"a", "b", "c", "d"} {
			ensure(t, s, schedule.Key{Kind: "backfill", ID: id}, schedule.Job{Interval: time.Hour, Limit: limit, Backoff: backoff, Run: func(context.Context, schedule.Ask) error {
				c := now.Add(1)
				if c > peak.Load() {
					peak.Store(c)
				}
				time.Sleep(10 * time.Minute)
				now.Add(-1)
				return nil
			}})
			ensure(t, s, schedule.Key{Kind: "sync", ID: id}, schedule.Job{Interval: time.Hour, Backoff: backoff, Run: func(context.Context, schedule.Ask) error {
				syncs.Add(1)
				return nil
			}})
		}
		synctest.Wait()
		if syncs.Load() != 4 {
			t.Fatalf("%d sync runs, want 4: the jobs holding no limit were held behind backfill's", syncs.Load())
		}
		time.Sleep(25 * time.Minute)
		synctest.Wait()
		s.Stop()
		if peak.Load() != 2 {
			t.Fatalf("at most %d backfill runs at once, want 2", peak.Load())
		}
	})
}

// childCase names the case a test binary started by a test runs in a process of its own.
const childCase = "SCHEDULE_TEST_CHILD_CASE"

// inChild runs body in a test binary of its own, started for this test alone with the case named, and
// fails the test when that process fails or stops, so a panic no code recovers, which stops the whole
// process, fails this test rather than every test after it.
func inChild(t *testing.T, name string, body func(t *testing.T)) {
	t.Helper()
	if os.Getenv(childCase) == name {
		body(t)
		return
	}
	//nolint:gosec // The test binary starts itself, with a pattern built from the test's own name.
	cmd := exec.CommandContext(t.Context(), os.Args[0], "-test.run=^"+regexp.QuoteMeta(t.Name())+"$", "-test.count=1")
	cmd.Env = append(os.Environ(), childCase+"="+name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("the process running the case stopped or failed: %v\n%s", err, out)
	}
}

// A panic in a run, on the run's own goroutine or on one it started through Go, is that run's
// failure with its stack, backs the job off, and stops no other job or its timer. Each case runs in a
// process of its own, which a panic no code recovers would stop.
func TestAPanicIsItsRunsFailureAndStopsNothingElse(t *testing.T) {
	for name, panics := range map[string]schedule.Run{
		"in the run": func(context.Context, schedule.Ask) error { panic("a bug in the run") },
		"in a goroutine it started": func(context.Context, schedule.Ask) error {
			return schedule.Go(func() error { panic("a bug in the run") })()
		},
	} {
		t.Run(name, func(t *testing.T) {
			inChild(t, name, func(t *testing.T) {
				synctest.Test(t, func(t *testing.T) {
					r := newRecorder()
					s := schedule.New(t.Context(), r)
					// A test that fails stops every job before the bubble ends, so the failure is reported.
					defer s.Stop()
					bad, good := schedule.Key{Kind: "sync", ID: "bad"}, schedule.Key{Kind: "sync", ID: "good"}
					var logged lockedBuilder
					ensure(t, s, bad, schedule.Job{Interval: 5 * time.Minute, Backoff: backoff, Logger: textLogger(&logged), Run: panics})
					ensure(t, s, good, schedule.Job{Interval: 5 * time.Minute, Backoff: backoff, Run: func(context.Context, schedule.Ask) error { return nil }})
					time.Sleep(16 * time.Minute)
					synctest.Wait()
					s.Stop()
					// The backoff is one minute, then two, then four, its cap.
					equal(t, "the panicking job's starts", r.starts(bad), minutes(0, 1, 3, 7, 11, 15))
					equal(t, "the other job's starts", r.starts(good), minutes(0, 5, 10, 15))
					for _, o := range r.outcomes(bad) {
						if o != schedule.Panic {
							t.Fatalf("a run of the panicking job ended %q, want %q", o, schedule.Panic)
						}
					}
					if text := logged.String(); !strings.Contains(text, "a bug in the run") || !strings.Contains(text, "goroutine") {
						t.Fatalf("the failure was not logged with its panic and stack: %s", text)
					}
				})
			})
		})
	}
}

// lockedBuilder is a strings.Builder safe for the writes of several goroutines.
type lockedBuilder struct {
	mu sync.Mutex
	b  strings.Builder
}

func (l *lockedBuilder) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.Write(p)
}

func (l *lockedBuilder) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.b.String()
}

// A failure backs the job off, and a success resets the backoff, so a failure after a success waits
// the backoff's base again.
func TestASuccessResetsTheBackoff(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		var n atomic.Int32
		ensure(t, s, syncKey, schedule.Job{Interval: 5 * time.Minute, Backoff: schedule.Backoff{Base: time.Minute, Max: time.Hour}, Run: func(context.Context, schedule.Ask) error {
			if c := n.Add(1); c == 2 || c == 3 || c == 5 {
				return errFailed
			}
			return nil
		}})
		time.Sleep(14 * time.Minute)
		synctest.Wait()
		s.Stop()
		// 0 succeeds, 5 fails, 6 fails, 8 succeeds, 10 fails and waits one minute, not four.
		equal(t, "starts", r.starts(syncKey), minutes(0, 5, 6, 8, 10, 11))
		equal(t, "backoffs", r.backoffs[syncKey], []time.Duration{time.Minute, 0, 2 * time.Minute, 0, time.Minute, 0})
	})
}

// Remove cancels one job's run and returns once it has returned, and Stop does so for every job.
func TestRemoveAndStopCancelTheirRunsAndWaitForThem(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		var returned atomic.Int32
		long := func(ctx context.Context, _ schedule.Ask) error {
			<-ctx.Done()
			time.Sleep(time.Second) // ends its unit after the cancellation
			returned.Add(1)
			return ctx.Err()
		}
		a, b := schedule.Key{Kind: "backfill", ID: "a"}, schedule.Key{Kind: "backfill", ID: "b"}
		ensure(t, s, a, schedule.Job{Interval: time.Hour, Backoff: backoff, Run: long})
		ensure(t, s, b, schedule.Job{Interval: time.Hour, Backoff: backoff, Run: long})
		synctest.Wait()
		s.Remove(a)
		if returned.Load() != 1 {
			t.Errorf("Remove returned with %d runs returned, want 1", returned.Load())
		}
		equal(t, "removed", r.removed, []schedule.Key{a})
		s.Stop()
		if returned.Load() != 2 {
			t.Fatalf("Stop returned with %d runs returned, want 2", returned.Load())
		}
		equal(t, "outcomes", r.outcomes(a), []schedule.Outcome{schedule.Cancelled})
		if err := s.Ensure(a, schedule.Job{Run: long}); err == nil {
			t.Fatal("a stopped scheduler ensured a job")
		}
	})
}

// A wake during a job's backoff waits for the backoff to end, so a failing job woken often is not
// run at the rate it is woken.
func TestAWakeDuringABackoffWaitsForItsEnd(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		key := schedule.Key{Kind: "backfill", ID: "a"}
		ensure(t, s, key, schedule.Job{Interval: time.Hour, Backoff: backoff, Run: func(context.Context, schedule.Ask) error { return errFailed }})
		for range 42 {
			time.Sleep(10 * time.Second)
			s.Wake(key)
		}
		synctest.Wait()
		s.Stop()
		// The backoff is one minute, then two, then four: 0, 1, 3 and 7 minutes, never every ten seconds.
		equal(t, "starts", r.starts(key), minutes(0, 1, 3, 7))
	})
}

// A job with no interval is asked at its ensure and then only when woken, once for the wakes that
// came during a run, and is never asked again on its own.
func TestAJobWithNoIntervalIsAskedOnlyWhenWoken(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		key := schedule.Key{Kind: "backfill", ID: "a"}
		var n atomic.Int32
		ensure(t, s, key, schedule.Job{Backoff: backoff, Run: func(ctx context.Context, _ schedule.Ask) error {
			if n.Add(1) > 50 {
				t.Error("a job with no interval is asked again on its own")
				<-ctx.Done()
				return nil
			}
			time.Sleep(10 * time.Minute)
			return nil
		}})
		time.Sleep(time.Minute)
		for range 5 {
			s.Wake(key)
		}
		time.Sleep(3 * time.Hour)
		s.Wake(key)
		time.Sleep(time.Hour)
		synctest.Wait()
		s.Stop()
		equal(t, "starts", r.starts(key), minutes(0, 10, 181))
	})
}

// A run that answers it was not due is neither a success nor a failure. A job asked at the end of its
// backoff that is not due keeps its failures, records no success, and is asked again on its phase.
func TestANotDueAnswerChangesNeitherFailuresNorSuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		var n atomic.Int32
		ensure(t, s, syncKey, schedule.Job{Interval: 5 * time.Minute, Backoff: backoff, Run: func(context.Context, schedule.Ask) error {
			switch n.Add(1) {
			case 1:
				return nil
			case 2:
				return errFailed
			case 3:
				return schedule.ErrNotDue
			case 4:
				return errFailed
			}
			return nil
		}})
		time.Sleep(13 * time.Minute)
		synctest.Wait()
		s.Stop()
		// 0 succeeds, 5 fails, 6 is not due, 10 fails a second time in a row and waits two minutes.
		equal(t, "starts", r.starts(syncKey), minutes(0, 5, 6, 10, 12))
		equal(t, "successes", r.successes[syncKey], minutes(0, 12))
	})
}

// A unit of work a run makes durable is the job's success when it is made, and the run's success is
// one more when it returns.
func TestAUnitMadeDurableIsASuccess(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		r := newRecorder()
		s := schedule.New(t.Context(), r)
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		key := schedule.Key{Kind: "backfill", ID: "a"}
		ensure(t, s, key, schedule.Job{Backoff: backoff, Run: func(_ context.Context, ask schedule.Ask) error {
			for range 3 {
				time.Sleep(20 * time.Minute)
				ask.Progressed()
			}
			return nil
		}})
		time.Sleep(2 * time.Hour)
		synctest.Wait()
		s.Stop()
		equal(t, "successes", r.successes[key], minutes(20, 40, 60, 60))
	})
}

// overlapping is a run that tells how many runs of its job run at once. Each run lasts a second, or
// ends its unit for thirty seconds after its context ends, as a page in progress does, so a second
// loop for the key would overlap it.
type overlapping struct {
	running, most, asked atomic.Int32
}

func (o *overlapping) run(ctx context.Context, _ schedule.Ask) error {
	o.asked.Add(1)
	n := o.running.Add(1)
	defer o.running.Add(-1)
	for {
		m := o.most.Load()
		if n <= m || o.most.CompareAndSwap(m, n) {
			break
		}
	}
	select {
	case <-ctx.Done():
		time.Sleep(30 * time.Second)
		return ctx.Err()
	case <-time.After(time.Second):
		return nil
	}
}

// A job ensured again while its removal waits for its run to end gets no second loop beside the first.
// The key stays taken until the removed loop has returned, so the new loop's first run starts only
// after the old run ended, and never two runs of one job run at once (ADR-0119).
func TestAJobEnsuredWhileItIsRemovedNeverRunsTwiceAtOnce(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := schedule.New(t.Context(), newRecorder())
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		key := schedule.Key{Kind: "backfill", ID: "a"}
		var o overlapping
		job := schedule.Job{Interval: time.Minute, Backoff: backoff, Run: o.run}
		ensure(t, s, key, job)
		time.Sleep(500 * time.Millisecond)
		go s.Remove(key)
		time.Sleep(time.Second)
		ensure(t, s, key, job)
		time.Sleep(5 * time.Minute)
		synctest.Wait()
		if most := o.most.Load(); most != 1 {
			t.Errorf("at most %d runs of one job ran at once, want 1", most)
		}
		if o.asked.Load() < 2 {
			t.Errorf("the job ensured again ran %d times in all, want its run again once the removed one ended", o.asked.Load())
		}
		s.Stop()
	})
}

// Ensuring and removing one job from many goroutines at once never runs two runs of the job at once,
// and leaves no loop running once the job is removed. A loop left running would keep asking its job on
// its interval. The race detector runs it in the go-test workflow.
func TestEnsuringAndRemovingAJobRapidlyLeavesNoLoopRunning(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := schedule.New(t.Context(), newRecorder())
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		key := schedule.Key{Kind: "backfill", ID: "a"}
		var o overlapping
		job := schedule.Job{Interval: time.Minute, Backoff: backoff, Run: o.run}
		var wg sync.WaitGroup
		// Each goroutine starts a little after the one before, so an ensure lands while another
		// goroutine's remove waits for the run it cancelled.
		for i := range 20 {
			wg.Go(func() {
				time.Sleep(time.Duration(i) * 7 * time.Millisecond)
				for range 50 {
					ensure(t, s, key, job)
					s.Wake(key)
					time.Sleep(100 * time.Millisecond)
					s.Remove(key)
				}
			})
		}
		wg.Wait()
		s.Remove(key)
		synctest.Wait()
		before := o.asked.Load()
		time.Sleep(time.Hour)
		synctest.Wait()
		if after := o.asked.Load(); after != before {
			t.Errorf("the job was asked %d times after it was removed", after-before)
		}
		if most := o.most.Load(); most != 1 {
			t.Errorf("at most %d runs of one job ran at once, want 1", most)
		}
		s.Stop()
	})
}

// Each ask carries the latest time on the ticker's phase at or before it, which is the ask's own time
// on the phase and the ticker's tick before it for the end of a backoff and for a wake, so a job
// measuring its interval from the phase keeps it through either (ADR-0119).
func TestEachAskCarriesItsTimeOnThePhase(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		s := schedule.New(t.Context(), newRecorder())
		// A test that fails stops every job before the bubble ends, so the failure is reported.
		defer s.Stop()
		key := schedule.Key{Kind: "sync", ID: "a"}
		t0 := time.Now()
		type asked struct{ At, Phase time.Duration }
		var (
			mu   sync.Mutex
			asks []asked
		)
		short := schedule.Backoff{Base: 5 * time.Second, Max: time.Minute}
		ensure(t, s, key, schedule.Job{Interval: time.Minute, Backoff: short, Run: func(_ context.Context, a schedule.Ask) error {
			mu.Lock()
			defer mu.Unlock()
			asks = append(asks, asked{At: a.At.Sub(t0), Phase: a.Phase.Sub(t0)})
			if len(asks) == 2 {
				return errFailed
			}
			return nil
		}})
		time.Sleep(150 * time.Second)
		s.Wake(key)
		time.Sleep(time.Second)
		synctest.Wait()
		s.Stop()
		mu.Lock()
		defer mu.Unlock()
		want := []asked{
			{At: 0, Phase: 0},
			{At: time.Minute, Phase: time.Minute},
			{At: time.Minute + 5*time.Second, Phase: time.Minute},
			{At: 2 * time.Minute, Phase: 2 * time.Minute},
			{At: 150 * time.Second, Phase: 2 * time.Minute},
		}
		if diff := cmp.Diff(want, asks, compare.Options); diff != "" {
			t.Errorf("the asks with their times on the phase (-want +got):\n%s", diff)
		}
	})
}
