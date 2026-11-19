// Package process runs one program as a process from its spec (ADR-0142). A spec lists what a program
// is made of, its name, its start check, and its endpoints, its job kinds or its one task, each with
// the function that builds it. The process takes one spec, starts as every process starts, builds
// every part the spec lists, keeps what those parts share, serves the endpoints, runs the job kinds
// or the task, answers its probes from what was reported, and stops in order. It knows nothing of
// accounts, providers, policies or any one program. process/README.md argues why this is a package of
// the process family.
package process

import (
	"context"
	"log/slog"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

// Spec is what one program is made of, typed on its configuration type C. It lists, and builds
// nothing.
type Spec[C any] struct {
	// Name is the program's directory, such as "api", which the component label on its series and the
	// attribute on its log records carry. The start banner names the published name,
	// mediated-mailbox-<Name> (ADR-0054).
	Name string
	// Init is the start check. It is given what Run was given and the loaded configuration, may refuse
	// to start, and builds nothing. Nil checks nothing.
	Init func(args, environ []string, c C) error
	// Endpoints are what the program serves over HTTP, each at its pattern.
	Endpoints []Endpoint[C]
	// Jobs are the program's job kinds, each run on the process's scheduler.
	Jobs []Job[C]
	// Task is set instead, for a program that does one piece of work and exits.
	Task *Task[C]
}

// Endpoint is one handler the application server serves at Pattern.
type Endpoint[C any] struct {
	Pattern string
	Build   func(ctx context.Context, p Capabilities, c C) (http.Handler, error)
}

// Job is one job kind, named by Name, the job_kind label on its series. Its plan tells the scheduler
// which jobs to keep (ADR-0119).
type Job[C any] struct {
	Name  string
	Build func(ctx context.Context, p Capabilities, c C) (schedule.Plan, error)
}

// Task is the one piece of work a program that exits does. Build returns the work, which the process
// runs once.
type Task[C any] struct {
	Build func(ctx context.Context, p Capabilities, c C) (func(context.Context) error, error)
}

// Capabilities are what the process offers the one endpoint, job kind, task or shared thing being
// built. Each carries, unexported, the process it belongs to, which is how an owner's slot finds what
// the process keeps.
type Capabilities interface {
	// Logger is the process's logger, with the labels of what is being built.
	Logger() *slog.Logger
	// Metrics is a registerer of the process's one registry, labelling every series with component,
	// and with job_kind for a job kind.
	Metrics() prometheus.Registerer
	// Report reports what is being built as ready, not ready, or failed, with a reason when it is not
	// ready. The readiness probe answers from the last reports, and the health probe fails while one
	// reports failed. A report is made from work the part does anyway, never from a call made to feed
	// it.
	Report(state State, reason string)
	// Go runs run on a goroutine the process waits for before it closes its slots. A panic on it is
	// the failure of what started it, named by name, which stops the process in order. Once the
	// process waits for its goroutines, Go starts nothing and returns false.
	Go(name string, run func(context.Context)) bool
	// Draining ends when the process starts to stop.
	Draining() context.Context
}

// State is what a report says.
type State int

// The states a report may name. NotReady is a fault that stops the part's work, reported only where
// work that runs whether or not the process takes traffic can show it cleared. Failed is a fault only a restart clears, which fails the health probe as well
// as the readiness probe, with its reason (ADR-0142).
const (
	NotReady State = iota
	Ready
	Failed
)

// JobKind returns the job kind p was handed to, or "" for an endpoint, a task or a shared thing. It is
// how process/database gives each job kind a pool of its own (ADR-0119).
func JobKind(p Capabilities) string {
	if c, ok := p.(*capabilities); ok {
		return c.jobKind
	}
	return ""
}
