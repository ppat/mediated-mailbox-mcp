package process

import (
	"errors"
	"fmt"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

// upkeepJob is one shared thing's upkeep, the job the scheduler runs for it under its name.
type upkeepJob struct {
	name string
	job  schedule.Job
}

// Upkeep has the process's scheduler run job as the upkeep of the shared thing p was handed to, such
// as the Session's reload, keyed by the thing's name, from when the process serves until it stops
// (ADR-0119, ADR-0142). The owner asks for it while the process is being built, from the build of
// its slot. An upkeep job belongs to no job kind, so none of the scheduler's series observe it, and a
// process whose scheduler runs only upkeep registers none of them (ADR-0125). A program that does
// one piece of work and exits runs no upkeep, so it is refused there, as are a second upkeep for one
// name and an upkeep asked for once the process has been built.
func Upkeep(p Capabilities, job schedule.Job) error {
	c, ok := p.(*capabilities)
	if !ok {
		return errors.New("the capabilities were not handed out by a process")
	}
	pr := c.proc
	pr.upkeepMu.Lock()
	defer pr.upkeepMu.Unlock()
	switch {
	case pr.task:
		return fmt.Errorf("%s asked for upkeep in a program that does one piece of work and exits", c.name)
	case pr.built:
		return fmt.Errorf("%s asked for upkeep once the process was built, and upkeep is asked for while it is built", c.name)
	}
	for _, u := range pr.upkeep {
		if u.name == c.name {
			return fmt.Errorf("%s asked for upkeep a second time", c.name)
		}
	}
	pr.upkeep = append(pr.upkeep, upkeepJob{name: c.name, job: job})
	return nil
}

// builtUpkeep closes the asking for upkeep, once the builds have returned, and returns what was asked
// for.
func (p *proc) builtUpkeep() []upkeepJob {
	p.upkeepMu.Lock()
	defer p.upkeepMu.Unlock()
	p.built = true
	return p.upkeep
}

// jobKindsOnly passes to the scheduler's series only what the jobs of the job kinds' plans do, so an
// upkeep job, keyed by its owner's name alone, is never observed. A nil series observes nothing.
type jobKindsOnly struct {
	series *schedule.Metrics
	upkeep map[string]bool
}

var _ schedule.Observer = jobKindsOnly{}

func (o jobKindsOnly) observed(key schedule.Key) bool {
	return o.series != nil && (key.ID != "" || !o.upkeep[key.Kind])
}

func (o jobKindsOnly) Ensured(key schedule.Key, late time.Duration, at time.Time) {
	if o.observed(key) {
		o.series.Ensured(key, late, at)
	}
}

func (o jobKindsOnly) Waiting(key schedule.Key, since time.Time) {
	if o.observed(key) {
		o.series.Waiting(key, since)
	}
}

func (o jobKindsOnly) Waited(key schedule.Key, since, until time.Time) {
	if o.observed(key) {
		o.series.Waited(key, since, until)
	}
}

func (o jobKindsOnly) Started(key schedule.Key) {
	if o.observed(key) {
		o.series.Started(key)
	}
}

func (o jobKindsOnly) Finished(key schedule.Key, outcome schedule.Outcome, took time.Duration) {
	if o.observed(key) {
		o.series.Finished(key, outcome, took)
	}
}

func (o jobKindsOnly) Succeeded(key schedule.Key, at time.Time) {
	if o.observed(key) {
		o.series.Succeeded(key, at)
	}
}

func (o jobKindsOnly) BackingOff(key schedule.Key, delay time.Duration) {
	if o.observed(key) {
		o.series.BackingOff(key, delay)
	}
}

func (o jobKindsOnly) Removed(key schedule.Key) {
	if o.observed(key) {
		o.series.Removed(key)
	}
}

// newScheduler returns the process's scheduler with every upkeep job ensured, for a process whose
// spec lists job kinds or whose owners asked for upkeep. Its series are registered only when job
// kinds are listed, labelled with the program's name and no job kind, each job's series carrying its
// own job kind (ADR-0119, ADR-0125, ADR-0142). A job kind sharing an upkeep's name is refused, since
// the two would be one key.
func (p *proc) newScheduler(b built) (*schedule.Scheduler, error) {
	observer := jobKindsOnly{upkeep: map[string]bool{}}
	for _, u := range b.upkeep {
		observer.upkeep[u.name] = true
	}
	for _, l := range b.plans {
		if observer.upkeep[l.kind] {
			return nil, fmt.Errorf("the job kind %s shares its name with the upkeep of a shared thing", l.kind)
		}
	}
	if len(b.plans) > 0 {
		series, err := schedule.NewMetrics(p.part("scheduler", "").Metrics())
		if err != nil {
			return nil, err
		}
		observer.series = series
	}
	scheduler := schedule.New(p.run, observer)
	for _, u := range b.upkeep {
		if err := scheduler.Ensure(schedule.Key{Kind: u.name}, u.job); err != nil {
			scheduler.Stop()
			return nil, err
		}
	}
	return scheduler, nil
}
