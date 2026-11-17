// Package schedule is the worker's scheduler (ADR-0119). It decides when each job is asked whether
// there is work, and the job's own due decision, inside its run, decides whether there is. It stores
// nothing outside the process: which jobs exist, when each was last asked, which wakes are pending
// and each job's backoff live in memory and are rebuilt from recorded state after a restart.
//
// A job is one key, a job kind and the account it serves, and the run its job kind built for it in
// the composition root, a plain typed function. Each job runs on a goroutine of its own, never on a
// goroutine another job shares, so a blocked run blocks only its own job. The scheduler keeps these
// rules.
//
//   - At most one run of a job at a time. A wake for a job that is running, or already waiting to
//     run, coalesces into one further run.
//   - A job with an interval is asked at once when it is ensured and then on the interval's phase, as
//     a time.Ticker ticks. A run that outlasts the interval is followed at once by one further run,
//     and the interval's phase then resumes.
//   - A run takes a slot of its job's limit after its wake and gives it back when it ends, so a limit
//     bounds the runs of one job kind running at once.
//   - A run that fails is asked again after a capped exponential backoff, which a success resets. A
//     wake during a backoff waits for the backoff to end.
//   - A panic in a run is recovered in the run that raised it, and is that run's failure. Job code
//     starts a goroutine only through Go, which recovers a panic in it the same way.
//   - Remove and Stop cancel the runs' contexts and wait for them to return.
//
// The package reads the clock only through the time package, so its tests drive it under
// testing/synctest. It holds no package-level state and installs no signal handler.
package schedule

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"
)

// Key names a job, a job kind and what it runs for, an account or nothing for a job of the whole
// kind.
type Key struct {
	Kind string
	ID   string
}

// Ask is what the scheduler hands one run of a job.
type Ask struct {
	// At is when the job was asked, the instant its timer was due, or when its wake or its ensure
	// came. A due decision reads the time from it.
	At time.Time
	// Phase is the latest time on the ticker's phase at or before At, which is At itself for an ask
	// on the phase and the ticker's tick before it for a wake or the end of a backoff. It is zero for
	// a job with no interval. A due decision that measures intervals from the last run it made reads its
	// last run's Phase, so one asked off the phase does not move the phase its later asks are due on.
	Phase time.Time
	// progressed records a unit of work made durable as the job's latest success.
	progressed func()
}

// Progressed records that the run made a unit of work durable, which is the job's latest success,
// as the end of a run that succeeded is.
func (a Ask) Progressed() {
	if a.progressed != nil {
		a.progressed()
	}
}

// Run is one run of a job. It returns nil when the run succeeded, including when its due decision
// found nothing to do and that answer is the job's success, ErrNotDue when the job was not due and
// nothing it did counts as a success or a failure, and any other error when the run failed. It
// returns when ctx ends, at its next unit boundary.
type Run func(ctx context.Context, ask Ask) error

// ErrNotDue is a run's answer when its job's due decision found it not yet due, such as a job asked
// at the end of its backoff before its interval has passed. The job's failures and its latest
// success stay as they were.
var ErrNotDue = errors.New("the job is not due")

// ErrPanicked wraps a panic recovered from a run, or from a goroutine started through Go.
var ErrPanicked = errors.New("the run panicked")

// Panicked returns the error a recovered value r stands for, with the stack of the goroutine that
// recovered it, which is the goroutine that panicked. A run shell that records its own failure
// calls it from the deferred function that recovers.
func Panicked(r any) error {
	return fmt.Errorf("%w: %v\n%s", ErrPanicked, r, debug.Stack())
}

// Go runs f on a goroutine of its own and returns the function that waits for it and returns f's
// error, or the panic f raised as an error wrapping ErrPanicked. A panic in a goroutine is
// recoverable only on that goroutine, so job code starts a goroutine only through Go.
func Go(f func() error) (wait func() error) {
	done := make(chan error, 1)
	go func() {
		var err error
		defer func() {
			if r := recover(); r != nil {
				err = Panicked(r)
			}
			done <- err
		}()
		err = f()
	}()
	return func() error { return <-done }
}

// Limit bounds the runs that hold it running at once.
type Limit struct {
	slots chan struct{}
}

// NewLimit returns a limit of n runs at once. n is at least one.
func NewLimit(n int) *Limit {
	return &Limit{slots: make(chan struct{}, max(n, 1))}
}

// Backoff is the delay after consecutive failures, Base, then twice Base, then four times, up to Max.
type Backoff struct {
	Base, Max time.Duration
}

// after returns the delay after the given number of consecutive failures, at least one.
func (b Backoff) after(failures int) time.Duration {
	d := b.Base
	for i := 1; i < failures && d < b.Max; i++ {
		d *= 2
	}
	return min(d, b.Max)
}

// Job is what the scheduler runs for one key.
type Job struct {
	Run Run
	// Interval is the phase of the job's timer. Zero means no timer, so after its ask at ensure the
	// job is asked only when woken or when its backoff ends.
	Interval time.Duration
	// Limit is the limit the job's runs hold, its job kind's. Nil holds none.
	Limit   *Limit
	Backoff Backoff
	// Late is how long after its latest success the job counts as late, which the alerting rule on a
	// job's latest success reads (ADR-0077).
	Late time.Duration
	// LastSuccess is the job's latest success as its job kind's recorded state holds it, which its
	// series start from, so a process that restarts without the job succeeding leaves it ageing. Zero
	// starts them at the ensure (ADR-0119).
	LastSuccess time.Time
	// Logger logs the job's failed runs. It carries the job's kind and account.
	Logger *slog.Logger
}

// Observer is told what the scheduler's jobs do, for the worker's series. Its methods are called
// from each job's goroutine, so they are safe for concurrent use.
type Observer interface {
	// Ensured is called when a job is ensured, with the instant its latest success is first counted
	// from.
	Ensured(key Key, late time.Duration, at time.Time)
	// Waiting is called when a job starts waiting for a slot of its kind's limit, and Waited when the
	// wait ends, so the time it waited does not count toward its lateness.
	Waiting(key Key, since time.Time)
	Waited(key Key, since, until time.Time)
	// Started is called when a run starts, and Finished when it returns.
	Started(key Key)
	Finished(key Key, outcome Outcome, took time.Duration)
	// Succeeded is called at a job's success, a run that succeeded or a unit it made durable.
	Succeeded(key Key, at time.Time)
	// BackingOff is called with the delay a job waits after a failure, and with zero once the delay
	// ends.
	BackingOff(key Key, delay time.Duration)
	// Removed is called when a job is removed.
	Removed(key Key)
}

// Outcome is how a run ended.
type Outcome string

// The outcomes of a run.
const (
	Succeeded Outcome = "succeeded"
	Failed    Outcome = "failed"
	Panic     Outcome = "panicked"
	NotDue    Outcome = "not_due"
	Cancelled Outcome = "cancelled"
)

// Scheduler runs jobs. Build it with New and stop it with Stop.
type Scheduler struct {
	ctx      context.Context
	cancel   context.CancelFunc
	observer Observer

	mu   sync.Mutex
	jobs map[Key]*state
	wg   sync.WaitGroup
}

// state is one job's loop as the scheduler holds it.
type state struct {
	cancel context.CancelFunc
	wake   chan struct{}
	done   chan struct{}
	// removing is set by Remove under the scheduler's lock. The state keeps its key until its loop has
	// returned, so a job ensured again meanwhile starts its loop only after this one's done closes, and
	// never two runs of one key run at once.
	removing bool
}

// New returns a scheduler whose runs end when parent ends or Stop is called, reporting to observer.
func New(parent context.Context, observer Observer) *Scheduler {
	ctx, cancel := context.WithCancel(parent)
	return &Scheduler{ctx: ctx, cancel: cancel, observer: observer, jobs: map[Key]*state{}}
}

// Ensure starts key's job with job unless it exists. Its first ask comes at once. It returns the
// scheduler's error once it has stopped.
func (s *Scheduler) Ensure(key Key, job Job) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := s.ctx.Err(); err != nil {
		return err
	}
	var previous chan struct{}
	if st, ok := s.jobs[key]; ok {
		if !st.removing {
			return nil
		}
		previous = st.done
	}
	if job.Logger == nil {
		job.Logger = slog.New(slog.DiscardHandler)
	}
	ctx, cancel := context.WithCancel(s.ctx)
	st := &state{cancel: cancel, wake: make(chan struct{}, 1), done: make(chan struct{})}
	s.jobs[key] = st
	seed := job.LastSuccess
	if seed.IsZero() {
		seed = time.Now()
	}
	s.observer.Ensured(key, job.Late, seed)
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		defer close(st.done)
		// The key's removed loop returns first, whether or not this one is removed meanwhile, so a
		// chain of removals and ensures still runs one loop of the key at a time.
		if previous != nil {
			<-previous
		}
		s.loop(ctx, key, job, st)
	}()
	return nil
}

// Remove cancels key's job, its run in flight included, and waits for it to return. The key stays
// taken until then, so a job ensured again for it meanwhile starts once this one has returned.
func (s *Scheduler) Remove(key Key) {
	s.mu.Lock()
	st, ok := s.jobs[key]
	if ok {
		st.removing = true
	}
	s.mu.Unlock()
	if !ok {
		return
	}
	st.cancel()
	<-st.done
	// A job ensured again for the key while this one stopped holds the key and keeps the series its
	// ensure set.
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobs[key] == st {
		delete(s.jobs, key)
		s.observer.Removed(key)
	}
}

// Wake asks key's job for one further run. Any number of wakes before that run starts are one.
func (s *Scheduler) Wake(key Key) {
	s.mu.Lock()
	st, ok := s.jobs[key]
	ok = ok && !st.removing
	s.mu.Unlock()
	if !ok {
		return
	}
	select {
	case st.wake <- struct{}{}:
	default:
	}
}

// Stop cancels every run and waits until every job's loop has returned.
func (s *Scheduler) Stop() {
	s.cancel()
	s.wg.Wait()
}

// loop asks key's job for its runs until ctx ends.
func (s *Scheduler) loop(ctx context.Context, key Key, job Job, st *state) {
	anchor := time.Now()
	// point is the latest time on the phase the timer has answered, and due when the timer next fires.
	point := anchor
	due, timed := anchor, true
	failures := 0
	backingOff := false
	timer := time.NewTimer(0)
	defer timer.Stop()
	for {
		wake := st.wake
		if backingOff {
			wake = nil
		}
		var fired <-chan time.Time
		if timed {
			timer.Reset(time.Until(due))
			fired = timer.C
		}
		var at time.Time
		phase := false
		select {
		case <-ctx.Done():
			return
		case <-fired:
			at, phase = due, !backingOff
		case <-wake:
			at = time.Now()
		}
		timer.Stop()
		if backingOff {
			backingOff = false
			s.owned(key, st, func() { s.observer.BackingOff(key, 0) })
		}
		if phase && job.Interval > 0 {
			// The time on the phase answered is the latest one passed, so ticks missed while a run
			// outlasted the interval give one run between them, as a ticker's one buffered tick does.
			point = anchor.Add(time.Since(anchor) / job.Interval * job.Interval)
		}
		// A wake that came before this point is answered by the run about to start.
		select {
		case <-st.wake:
		default:
		}
		if job.Limit != nil && !s.take(ctx, key, st, job.Limit) {
			return
		}
		var onPhase time.Time
		if job.Interval > 0 {
			onPhase = anchor.Add(at.Sub(anchor) / job.Interval * job.Interval)
		}
		outcome, err := s.ask(ctx, key, st, job, at, onPhase)
		if job.Limit != nil {
			<-job.Limit.slots
		}
		switch outcome {
		case Cancelled:
			return
		case Succeeded:
			failures = 0
		case Failed, Panic:
			failures++
			delay := job.Backoff.after(failures)
			job.Logger.Error("the run failed", "error", err, "failures", failures, "backoff", delay.String())
			backingOff, due, timed = true, time.Now().Add(delay), true
			s.owned(key, st, func() { s.observer.BackingOff(key, delay) })
			continue
		case NotDue:
		}
		due, timed = point.Add(job.Interval), job.Interval > 0
	}
}

// owned calls f, which reports to the observer on key's own series, only while st is key's state. A
// job ensured again while its removal waits for its loop gets a state of its own, and the series its
// ensure set, so nothing the removed loop still does, a wait that ends or a run that makes a unit
// durable as it stops, reaches the new job's series (ADR-0119).
func (s *Scheduler) owned(key Key, st *state, f func()) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.jobs[key] == st {
		f()
	}
}

// take takes a slot of limit for key's job, waiting for one while every slot is held, and reports
// false when ctx ends first. A wait is reported to the observer, so the time the job spends waiting
// for its kind's other jobs does not count toward its lateness (ADR-0119).
func (s *Scheduler) take(ctx context.Context, key Key, st *state, limit *Limit) bool {
	select {
	case limit.slots <- struct{}{}:
		return true
	default:
	}
	since := time.Now()
	s.owned(key, st, func() { s.observer.Waiting(key, since) })
	defer func() {
		until := time.Now()
		s.owned(key, st, func() { s.observer.Waited(key, since, until) })
	}()
	select {
	case limit.slots <- struct{}{}:
		return true
	case <-ctx.Done():
		return false
	}
}

// ask makes one run of key's job, asked at at with phase the latest time on its phase, and returns
// how it ended.
func (s *Scheduler) ask(ctx context.Context, key Key, st *state, job Job, at, phase time.Time) (Outcome, error) {
	s.observer.Started(key)
	started := time.Now()
	panicked, err := call(ctx, job.Run, Ask{At: at, Phase: phase, progressed: func() {
		now := time.Now()
		s.owned(key, st, func() { s.observer.Succeeded(key, now) })
	}})
	outcome := Succeeded
	switch {
	case ctx.Err() != nil:
		outcome = Cancelled
	case panicked || errors.Is(err, ErrPanicked):
		outcome = Panic
	case errors.Is(err, ErrNotDue):
		outcome = NotDue
	case err != nil:
		outcome = Failed
	}
	s.observer.Finished(key, outcome, time.Since(started))
	if outcome == Succeeded {
		now := time.Now()
		s.owned(key, st, func() { s.observer.Succeeded(key, now) })
	}
	return outcome, err
}

// call runs run, recovering a panic in it as an error wrapping ErrPanicked.
func call(ctx context.Context, run Run, ask Ask) (panicked bool, err error) {
	defer func() {
		if r := recover(); r != nil {
			panicked, err = true, Panicked(r)
		}
	}()
	return false, run(ctx, ask)
}
