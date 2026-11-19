package process

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"sync"

	"github.com/prometheus/client_golang/prometheus"
)

// The label names every endpoint and job kind carries, so two of them may register one series name
// (ADR-0125, ADR-0142).
const (
	componentLabel = "component"
	jobKindLabel   = "job_kind"
)

// proc is one running program: its registry, its contexts, its readiness reports, its slots and the
// goroutines started through Go.
type proc struct {
	name     string
	logger   *slog.Logger
	registry *prometheus.Registry

	// run is the context the builds, the jobs and the work run under, which ends once the application
	// server has drained, or when a start is stopped before it serves. draining ends when the process
	// starts to stop.
	run      context.Context
	stopRun  context.CancelFunc
	draining context.Context
	drain    context.CancelFunc

	reportsMu sync.Mutex
	reports   []report

	slotsMu sync.Mutex
	slots   map[any]*slot
	closers []func()

	// revisions are the configuration library's revision of each top-level key (ADR-0078).
	revisions map[string]string
	// upkeepMu guards the upkeep owners asked for while the process was built, which the scheduler
	// runs once the process serves. built is set once the builds have returned, and task when the
	// process runs a task, which runs no upkeep.
	upkeepMu sync.Mutex
	upkeep   []upkeepJob
	built    bool
	task     bool

	// goMu guards the goroutines Go starts. joining is set once the process waits for them, after
	// which nothing starts. failures are their panics, and failed closes on the first.
	goMu     sync.Mutex
	joining  bool
	started  sync.WaitGroup
	failures []error
	failed   chan struct{}
}

// report is the last state one reporter reported.
type report struct {
	name   string
	state  State
	reason string
}

func newProcess(name string, logger *slog.Logger, revisions map[string]string) (*proc, error) {
	registry := prometheus.NewRegistry()
	// The runtime series are the process's, registered on the registry itself, so they carry no
	// component and no job kind (ADR-0125).
	if err := registerRuntimeMetrics(registry); err != nil {
		return nil, err
	}
	p := &proc{
		name: name, logger: logger, registry: registry, revisions: revisions,
		slots: map[any]*slot{}, failed: make(chan struct{}),
	}
	p.run, p.stopRun = context.WithCancel(context.Background())
	p.draining, p.drain = context.WithCancel(p.run)
	return p, nil
}

// capabilities are what the process hands one part, or one shared thing, being built.
type capabilities struct {
	proc    *proc
	name    string
	jobKind string
	logger  *slog.Logger
	metrics prometheus.Registerer
}

// part returns the capabilities for the part named name, labelled with the program's name and, for a
// job kind, with the job kind.
func (p *proc) part(name, jobKind string) *capabilities {
	labels := prometheus.Labels{componentLabel: p.name}
	logger := p.logger.With(componentLabel, p.name)
	if jobKind != "" {
		labels[jobKindLabel] = jobKind
		logger = logger.With(jobKindLabel, jobKind)
	}
	return &capabilities{
		proc: p, name: name, jobKind: jobKind, logger: logger,
		metrics: prometheus.WrapRegistererWith(labels, p.registry),
	}
}

func (c *capabilities) Logger() *slog.Logger           { return c.logger }
func (c *capabilities) Metrics() prometheus.Registerer { return c.metrics }
func (c *capabilities) Draining() context.Context      { return c.proc.draining }

func (c *capabilities) Report(state State, reason string) { c.proc.report(c.name, state, reason) }

func (c *capabilities) Go(name string, run func(context.Context)) bool {
	return c.proc.goroutine(c.name, name, run)
}

// report keeps the state name last reported, in the order each reporter first reported.
func (p *proc) report(name string, state State, reason string) {
	p.reportsMu.Lock()
	defer p.reportsMu.Unlock()
	for i := range p.reports {
		if p.reports[i].name == name {
			p.reports[i].state, p.reports[i].reason = state, reason
			return
		}
	}
	p.reports = append(p.reports, report{name: name, state: state, reason: reason})
}

// notReady returns the line naming the first reporter that is not ready, and false when every one
// is. It reads the reports and runs nothing.
func (p *proc) notReady() (string, bool) {
	p.reportsMu.Lock()
	defer p.reportsMu.Unlock()
	for _, r := range p.reports {
		switch r.state {
		case Ready:
			continue
		case Failed:
			return "not ready: " + r.name + " failed: " + r.reason, true
		case NotReady:
			return "not ready: " + r.name + ": " + r.reason, true
		}
	}
	return "", false
}

// goroutine starts run on a goroutine the process waits for, under the run context. A panic on it
// is recorded as the failure of the part named owner.
func (p *proc) goroutine(owner, name string, run func(context.Context)) bool {
	p.goMu.Lock()
	defer p.goMu.Unlock()
	if p.joining {
		return false
	}
	p.started.Add(1)
	go func() {
		defer p.started.Done()
		if err := recovered(func() error { run(p.run); return nil }); err != nil {
			p.goroutineFailed(fmt.Errorf("%s goroutine %s: %w", owner, name, err))
		}
	}()
	return true
}

func (p *proc) goroutineFailed(err error) {
	p.goMu.Lock()
	defer p.goMu.Unlock()
	if len(p.failures) == 0 {
		close(p.failed)
	}
	p.failures = append(p.failures, err)
}

// joinGoroutines stops Go from starting anything and waits for every goroutine it started, and
// returns their failures.
func (p *proc) joinGoroutines() error {
	p.goMu.Lock()
	p.joining = true
	p.goMu.Unlock()
	p.started.Wait()
	p.goMu.Lock()
	defer p.goMu.Unlock()
	return errors.Join(p.failures...)
}

// closeSlots closes what the slots hold, last built first.
func (p *proc) closeSlots() {
	p.slotsMu.Lock()
	closers := p.closers
	p.closers = nil
	p.slotsMu.Unlock()
	for _, closeSlot := range slices.Backward(closers) {
		closeSlot()
	}
}

// stop ends the run, waits for every goroutine and closes the slots, for a start that fails before
// it serves and for a task that is done.
func (p *proc) stop() error {
	p.drain()
	p.stopRun()
	err := p.joinGoroutines()
	p.closeSlots()
	return err
}
