package schedule

import (
	"context"
	"errors"
	"log/slog"
	"time"
)

// Plan is what a job kind's Build returns, which tells the scheduler which jobs to keep for the job
// kind (ADR-0119, ADR-0142). A key is what one job runs for, an account for backfill and delta sync.
type Plan struct {
	// Keys lists the keys to keep a job for now. An error is the listing's failure, after which it is
	// asked again once its backoff ends, and ErrNotDue an answer that changes nothing and is neither a
	// success nor a failure.
	Keys func(ctx context.Context) ([]string, error)
	// Recheck is how often Keys is asked again. Zero asks it only when it is woken or its backoff ends.
	Recheck time.Duration
	// Backoff is the listing's own backoff after a failure.
	Backoff Backoff
	// Late is how long after its latest success the listing counts as late, which the alerting rule
	// on a job's latest success reads (ADR-0077). Zero counts it late at once.
	Late time.Duration
	// Job returns the job for one key the listing finds that has none. An error is the listing's
	// failure, and the keys before it keep the jobs they were given.
	Job func(key string) (Job, error)
	// Dropped is told each key the listing no longer finds, once its job has been removed and has
	// returned. Nil is told nothing.
	Dropped func(key string)
	// Woken is handed the function that asks Keys again at once, which the plan gives to what changes
	// its keys, such as the reload of what the job kind works on. Nil wakes nothing.
	Woken func(wake func())
}

// EnsurePlan keeps one job per key plan lists for the job kind kind, adding the job of a key the
// listing finds and removing the job of one it no longer finds. The listing is itself a job of the
// kind, keyed by the kind alone, asked at once, then every plan.Recheck and whenever woken, and its
// failures logged through logger. listed is called once, when a listing first succeeds, which is
// when the job kind has read what it runs for and kept its jobs at least once. It returns the
// scheduler's error once it has stopped.
//
// A plan with no Keys or no Job is refused, since its listing could never run.
func (s *Scheduler) EnsurePlan(kind string, plan Plan, logger *slog.Logger, listed func()) error {
	if plan.Keys == nil || plan.Job == nil {
		return errors.New("the plan has no listing or no job")
	}
	key := Key{Kind: kind}
	if plan.Woken != nil {
		plan.Woken(func() { s.Wake(key) })
	}
	held := map[string]bool{}
	first := true
	// The listing's run reads and writes held alone, and the scheduler runs one run of a job at a time.
	return s.Ensure(key, Job{
		Interval: plan.Recheck,
		Backoff:  plan.Backoff,
		Late:     plan.Late,
		Logger:   logger,
		Run: func(ctx context.Context, _ Ask) error {
			keys, err := plan.Keys(ctx)
			if err != nil {
				return err
			}
			listing := make(map[string]bool, len(keys))
			for _, k := range keys {
				if k == "" {
					// The listing's own job is keyed by the kind alone, so an empty key would name it.
					return errors.New("the listing names an empty key")
				}
				listing[k] = true
			}
			for k := range held {
				if listing[k] {
					continue
				}
				s.Remove(Key{Kind: kind, ID: k})
				delete(held, k)
				if plan.Dropped != nil {
					plan.Dropped(k)
				}
			}
			for _, k := range keys {
				if held[k] {
					continue
				}
				job, err := plan.Job(k)
				if err != nil {
					return err
				}
				if err := s.Ensure(Key{Kind: kind, ID: k}, job); err != nil {
					return err
				}
				held[k] = true
			}
			if first && listed != nil {
				listed()
			}
			first = false
			return nil
		},
	})
}
