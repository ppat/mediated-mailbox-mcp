// Package host runs one process that hosts the components a composition root hands it
// (ADR-0138). It does what every process does, whatever it runs. It refuses the environment the
// process cannot run under, loads one configuration tree, sets the log level, builds the one
// metrics registry and the pools, serves the application server and the operations endpoints,
// starts and joins every background task and every goroutine a component asks its starter for, and
// stops in order. A process that runs once and exits gets the same start, registry and pools, and no
// servers or tasks. A component knows none of it. Its assembly takes what it needs as values, starts
// nothing of its own, and returns what it provides, a handler, readiness checks and tasks, so a
// component is assembled the same way whatever else shares its process. process/README.md argues why
// this is a package of the process family.
//
// Run is the whole process for a composition root whose process runs until it is told to stop, and
// RunOnce for one whose process does one piece of work and exits, as the migrator does. Both start the
// process the same way. New, Add and Serve are Run's lifecycle without the configuration, which a test
// or a harness that composes a component itself uses, so what it runs is what the composition root
// runs.
package host

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/process/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/dbmetrics"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/probes"
	"github.com/ppat/mediated-mailbox-mcp/process/runtimemetrics"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// Process is the section of the configuration every process holds, read at the top level of the
// tree, where a root embeds it inline (ADR-0078).
type Process struct {
	// LogLevel is the lowest level the process logs at, debug, info, warn or error (ADR-0122).
	LogLevel string `yaml:"log_level"`
}

// Operations is the section of the configuration a process holds when it runs until it is told to
// stop, so the host serves its operations endpoints, read at the top level of the tree beside Process.
// A process that runs once serves none, since nothing probes or scrapes a process that exits on its
// own.
type Operations struct {
	// ProbeListen is the address the operations endpoints listen on, over plain HTTP (ADR-0051).
	ProbeListen string `yaml:"probe_listen"`
}

// Serving is the section of the configuration a process holds when a component it hosts serves
// requests, read at the top level of the tree beside Process.
type Serving struct {
	// Listen is the address the application server listens on.
	Listen string `yaml:"listen"`
	// TLSCert and TLSKey name the mounted files holding the certificate chain and the private key.
	// Both set, the application server serves TLS, and neither set, plain HTTP (ADR-0127).
	TLSCert string `yaml:"tls_cert"`
	TLSKey  string `yaml:"tls_key"`
}

// DefaultProcess is the process section's defaults.
func DefaultProcess() Process {
	return Process{LogLevel: logging.DefaultLevel}
}

// DefaultOperations is the operations section's defaults.
func DefaultOperations() Operations {
	return Operations{ProbeListen: ":8080"}
}

// DefaultServing is the serving section's defaults. The key pair has no default, since a default path
// assumes the environment.
func DefaultServing() Serving {
	return Serving{Listen: ":8443"}
}

// ServesTLS reports whether s names a key pair. ValidateServing has refused one that names only one of
// its two files, so the certificate alone decides.
func (s Serving) ServesTLS() bool {
	return strings.TrimSpace(s.TLSCert) != ""
}

// ValidateProcess refuses a log_level that names no level.
func ValidateProcess(p Process) error {
	_, err := logging.ParseLevel(p.LogLevel)
	return err
}

// ValidateOperations refuses an empty probe_listen.
func ValidateOperations(o Operations) error {
	if strings.TrimSpace(o.ProbeListen) == "" {
		return errors.New("probe_listen is empty")
	}
	return nil
}

// ValidateServing refuses an empty listen, and one of the two TLS files set without the other, a blank
// path counting as unset (ADR-0127).
func ValidateServing(s Serving) error {
	if strings.TrimSpace(s.Listen) == "" {
		return errors.New("listen is empty")
	}
	if cert, key := strings.TrimSpace(s.TLSCert) != "", strings.TrimSpace(s.TLSKey) != ""; cert != key {
		set, missing := "tls_cert", "tls_key"
		if key {
			set, missing = "tls_key", "tls_cert"
		}
		return fmt.Errorf("%s is set and %s is not, and the process serves TLS with both or plain HTTP with neither", set, missing)
	}
	return nil
}

// Parts are what a component's assembly returns. Nothing in them has started.
type Parts struct {
	// Handler serves the component's requests, nil for a component that serves none.
	Handler http.Handler
	// Ready are the component's readiness checks. The process is ready only while every check of
	// every component passes.
	Ready []Check
	// Tasks are the component's background tasks, which the host starts once the process serves.
	Tasks []Task
}

// Check is one named readiness check. It returns nil while what it checks is ready.
type Check struct {
	Name  string
	Check func(context.Context) error
}

// Task is one named background task. It runs until its context ends, and an error it returns before
// then, or a panic it raises, is its failure, which stops the process.
type Task struct {
	Name string
	Run  func(context.Context) error
}

// Component is one component a composition root hands Run.
type Component[T any] struct {
	// Name is the component's directory, which the component label on its series and the attribute on
	// its log records carry.
	Name string
	// Mount is the application server's patterns the component's handler serves.
	Mount []string
	// RefuseEnvironment refuses an environment the component cannot run under, before the
	// configuration is read. Nil refuses nothing.
	RefuseEnvironment func(environ []string) error
	// Assemble builds the component from the loaded configuration and what the host provides it.
	Assemble func(ctx context.Context, provided Provided[T]) (Parts, error)
}

// Provided is what the host provides one component's assembly.
type Provided[T any] struct {
	Loaded settings.Loaded[T]
	// Logger and Registerer carry the component's name, as an attribute and as the component label.
	Logger     *slog.Logger
	Registerer prometheus.Registerer
	// Draining ends when the process begins to stop serving requests, so a request that would not end
	// by itself, such as an event stream, ends then and the server can drain the rest.
	Draining context.Context
	// OpenPool opens a pool measured on reg, which the host closes once every task has returned.
	OpenPool func(ctx context.Context, c dbconnectcore.Config, reg prometheus.Registerer) (*pgxpool.Pool, error)
	// Start starts a goroutine the component needs while it runs, the host's as much as a task.
	Start Starter
}

// Starter starts run on a goroutine of the host's, under the context the tasks run under, and
// reports whether it did. A panic run raises is the failure of the component that started it, named
// by name, which stops the process in order, and the host waits for run to return before it closes
// the pools. Once the host waits for its tasks, a Starter starts nothing and returns false, so its
// caller learns that run will not run (ADR-0138).
type Starter func(name string, run func(context.Context)) bool

// Spec is what a composition root hands Run, its configuration's root type T and its components.
type Spec[T any] struct {
	// Name is the published name the start banner names.
	Name     string
	Defaults T
	// Process returns T's process section, Operations its operations section, and Serving its serving
	// section, nil when no component serves requests.
	Process    func(T) Process
	Operations func(T) Operations
	Serving    func(T) Serving
	// Validate refuses a configuration the root's own sections cannot run with. The host validates
	// the process's sections first.
	Validate   func(T) error
	Components []Component[T]
}

// Run runs the process the spec describes until ctx ends. It starts the process as every process
// starts, then assembles each component in order and serves (ADR-0051, ADR-0078, ADR-0122).
func Run[T any](ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar, spec Spec[T]) error {
	refusals := make([]func([]string) error, 0, len(spec.Components))
	for _, c := range spec.Components {
		refusals = append(refusals, c.RefuseEnvironment)
	}
	loaded, started, err := start(args, environ, stdout, stderr, logger, level, starting[T]{
		name: spec.Name, defaults: spec.Defaults, refusals: refusals, process: spec.Process,
		validate: func(c T) error { return validate(spec, c) },
		banner: func(c T) []string {
			var lines []string
			if spec.Serving != nil {
				s := spec.Serving(c)
				lines = append(lines, fmt.Sprintf("serves  %s on %s", scheme(s.ServesTLS()), s.Listen))
			}
			return append(lines, "probes  http on "+spec.Operations(c).ProbeListen)
		},
	})
	if !started {
		return err
	}
	var serving Serving
	if spec.Serving != nil {
		serving = spec.Serving(loaded.Config)
	}

	h, err := New(logger)
	if err != nil {
		return err
	}
	// The assembly runs under the host's context, which outlives ctx until the process has drained.
	// While it runs, ctx ending ends it too, so a start told to stop stops.
	stopAssembly := context.AfterFunc(ctx, h.stopRun)
	err = assemble(h, spec, loaded)
	stopAssembly()
	if err != nil {
		h.Close()
		return errors.Join(err, h.startedFailures())
	}
	var lc net.ListenConfig
	listeners := Listeners{}
	if spec.Serving != nil {
		if listeners.App, err = lc.Listen(ctx, "tcp", serving.Listen); err != nil {
			h.Close()
			return err
		}
	}
	if listeners.Probes, err = lc.Listen(ctx, "tcp", spec.Operations(loaded.Config).ProbeListen); err != nil {
		h.Close()
		return errors.Join(err, listeners.close())
	}
	return h.Serve(ctx, listeners, serving)
}

// Once is what a composition root hands RunOnce, its configuration's root type T and the one piece of
// work its process does.
type Once[T any] struct {
	// Name is the published name the start banner names.
	Name     string
	Defaults T
	// Process returns T's process section.
	Process func(T) Process
	// Validate refuses a configuration the root's own sections cannot run with. The host validates
	// the process's section first.
	Validate func(T) error
	// Component is the directory of the component whose work the process does, which its logger and
	// its registerer carry, as a component's do in Run.
	Component string
	// RefuseEnvironment refuses an environment the work cannot run under, before the configuration is
	// read. Nil refuses nothing.
	RefuseEnvironment func(environ []string) error
	// Work does the process's one piece of work with what the host provides it, and returns once it is
	// done.
	Work func(ctx context.Context, provided Provided[T]) error
}

// RunOnce runs a process that does one piece of work and exits. It starts the process as every
// process starts, then does the work under ctx with what the host provides a component, and returns
// the work's error once every goroutine the work started has returned and the pools are closed. It
// serves no operations endpoints and no application server and runs no task, since nothing probes,
// scrapes or calls a process that exits on its own (ADR-0138, ADR-0139).
func RunOnce[T any](ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar, spec Once[T]) error {
	loaded, started, err := start(args, environ, stdout, stderr, logger, level, starting[T]{
		name: spec.Name, defaults: spec.Defaults, refusals: []func([]string) error{spec.RefuseEnvironment},
		process: spec.Process, validate: spec.Validate,
		banner: func(T) []string { return nil },
	})
	if !started {
		return err
	}
	h, err := New(logger)
	if err != nil {
		return err
	}
	err = spec.Work(ctx, Provided[T]{
		Loaded: loaded, Logger: h.Logger(spec.Component), Registerer: h.Registerer(spec.Component),
		Draining: ctx, OpenPool: h.OpenPool, Start: h.Starter(spec.Component),
	})
	h.Close()
	return errors.Join(err, h.startedFailures())
}

// starting is what start needs of a root, whichever way its process runs.
type starting[T any] struct {
	name     string
	defaults T
	refusals []func(environ []string) error
	process  func(T) Process
	// validate refuses a configuration the root's other sections cannot run with, after the process's
	// section is validated.
	validate func(T) error
	// banner returns the banner's lines below the one naming the process and its release version.
	banner func(T) []string
}

// start is what every process does before its work, however it runs. It refuses the environment the
// database driver and the root refuse before the configuration is read, answers --help and --version
// on stdout, logs the effective configuration, each value with the layer that set it, then sets level
// from log_level, validates the process's section and the root's, and writes the start banner to
// stderr. started reports whether the process goes on to its work: it is false with a nil error once
// --help or --version is answered.
func start[T any](args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar, root starting[T]) (loaded settings.Loaded[T], started bool, err error) {
	if err := dbconnect.RefusePasswordVariables(environ); err != nil {
		return loaded, false, err
	}
	for _, refuse := range root.refusals {
		if refuse == nil {
			continue
		}
		if err := refuse(environ); err != nil {
			return loaded, false, err
		}
	}
	loaded, err = settings.Load(root.defaults, args, environ)
	var help *settings.HelpRequested
	if errors.As(err, &help) {
		_, err := io.WriteString(stdout, help.Text)
		return loaded, false, err
	}
	if errors.As(err, new(*settings.VersionRequested)) {
		_, err := fmt.Fprintln(stdout, version.Number)
		return loaded, false, err
	}
	if err != nil {
		return loaded, false, fmt.Errorf("loading the configuration: %w", err)
	}
	// The effective configuration is written before the level applies, so a refused value's source,
	// log_level's included, reaches the log whatever level the configuration sets (ADR-0078, ADR-0122).
	for _, v := range loaded.Values {
		logger.Info("configuration", "path", v.Path, "source", v.Source.String(), "value", v.Value)
	}
	lv, err := logging.ParseLevel(root.process(loaded.Config).LogLevel)
	if err != nil {
		return loaded, false, fmt.Errorf("validating the configuration: %w", err)
	}
	level.Set(lv)
	if root.validate != nil {
		if err := root.validate(loaded.Config); err != nil {
			return loaded, false, fmt.Errorf("validating the configuration: %w", err)
		}
	}
	writeBanner(stderr, banner(root.name, root.banner(loaded.Config)), logger)
	return loaded, true, nil
}

// validate refuses a configuration Run's process cannot run with, its operations section, its serving
// section when a component serves requests, then the root's own sections.
func validate[T any](spec Spec[T], c T) error {
	if err := ValidateOperations(spec.Operations(c)); err != nil {
		return err
	}
	if spec.Serving != nil {
		if err := ValidateServing(spec.Serving(c)); err != nil {
			return err
		}
	}
	if spec.Validate == nil {
		return nil
	}
	return spec.Validate(c)
}

// assemble builds each component in order, with the logger and registerer carrying its name, and adds
// it to h.
func assemble[T any](h *Host, spec Spec[T], loaded settings.Loaded[T]) error {
	for _, c := range spec.Components {
		parts, err := c.Assemble(h.Context(), Provided[T]{
			Loaded: loaded, Logger: h.Logger(c.Name), Registerer: h.Registerer(c.Name),
			Draining: h.Draining(), OpenPool: h.OpenPool, Start: h.Starter(c.Name),
		})
		if err != nil {
			return err
		}
		h.Add(c.Name, c.Mount, parts)
	}
	return nil
}

// banner is the start banner, a few lines for a person reading the process's output that name the
// process and its release version, and below them its lines, such as where it serves.
func banner(name string, lines []string) string {
	text := fmt.Sprintf("%s %s\n", name, version.Number)
	for _, l := range lines {
		text += "  " + l + "\n"
	}
	return text
}

func scheme(tls bool) string {
	if tls {
		return "https"
	}
	return "http"
}

// writeBanner writes the start banner. It is no log record, and a failed write stops nothing
// (ADR-0122).
func writeBanner(stderr io.Writer, text string, logger *slog.Logger) {
	if _, err := io.WriteString(stderr, text); err != nil {
		logger.Warn("writing the start banner failed", "error", err)
	}
}

// Host is one process's lifecycle, the registry, the pools, the components added to it and the
// contexts their work runs under.
type Host struct {
	logger   *slog.Logger
	registry *prometheus.Registry
	// run is the context the assemblies and the tasks run under, which ends once the application
	// server has drained, and draining the one that ends when the process begins to stop serving.
	run      context.Context
	stopRun  context.CancelFunc
	draining context.Context
	drain    context.CancelFunc

	mu         sync.Mutex
	pools      []*pgxpool.Pool
	components []component

	// startedMu guards the goroutines Starter starts. joining is set once the host waits for them,
	// after which nothing starts. failures are their panics, and failed closes on the first.
	startedMu sync.Mutex
	joining   bool
	started   sync.WaitGroup
	failures  []error
	failed    chan struct{}
}

type component struct {
	name  string
	mount []string
	parts Parts
}

// New returns a host with the process's one registry, the Go runtime's and the process's series
// registered on it once (ADR-0125).
func New(logger *slog.Logger) (*Host, error) {
	registry := prometheus.NewRegistry()
	if err := runtimemetrics.Register(registry); err != nil {
		return nil, err
	}
	h := &Host{logger: logger, registry: registry, failed: make(chan struct{})}
	h.run, h.stopRun = context.WithCancel(context.Background())
	h.draining, h.drain = context.WithCancel(h.run)
	return h, nil
}

// Context is the context a component's assembly and its tasks run under. It ends once the process
// has drained its requests, or when a start is stopped before it serves.
func (h *Host) Context() context.Context { return h.run }

// Draining ends when the process begins to stop serving requests.
func (h *Host) Draining() context.Context { return h.draining }

// Gatherer is the process's registry, which the metrics endpoint serves.
func (h *Host) Gatherer() prometheus.Gatherer { return h.registry }

// Registerer returns the registry with every series registered through it labelled with the component
// name, so the series one library registers for several components in one process stay apart, and a
// series has the same labels however the components are packaged (ADR-0076, ADR-0125).
func (h *Host) Registerer(name string) prometheus.Registerer {
	return prometheus.WrapRegistererWith(prometheus.Labels{componentLabel: name}, h.registry)
}

// componentLabel names the component on every series a component registers.
const componentLabel = "component"

// Logger returns the process's logger with the component's name on every record.
func (h *Host) Logger(name string) *slog.Logger {
	return h.logger.With(componentLabel, name)
}

// OpenPool opens a pool from c, connecting as c's role, measured on reg, which the host closes once
// every task has returned. A pool opens no connection until its first use (ADR-0118, ADR-0125).
func (h *Host) OpenPool(ctx context.Context, c dbconnectcore.Config, reg prometheus.Registerer) (*pgxpool.Pool, error) {
	config, err := dbconnect.PoolConfig(c)
	if err != nil {
		return nil, fmt.Errorf("configuring the database connection of %s: %w", c.User, err)
	}
	pool, err := dbmetrics.OpenPool(ctx, config, reg)
	if err != nil {
		return nil, fmt.Errorf("configuring the database connection of %s: %w", c.User, err)
	}
	h.mu.Lock()
	h.pools = append(h.pools, pool)
	h.mu.Unlock()
	return pool, nil
}

// Starter returns the starter for the component named component, whose goroutines' panics name it.
func (h *Host) Starter(component string) Starter {
	return func(name string, run func(context.Context)) bool {
		h.startedMu.Lock()
		defer h.startedMu.Unlock()
		if h.joining {
			return false
		}
		h.started.Add(1)
		go func() {
			defer h.started.Done()
			err := recovered(h.run, Task{Name: name, Run: func(ctx context.Context) error {
				run(ctx)
				return nil
			}})
			if err != nil {
				h.startedFailed(fmt.Errorf("%s goroutine %s: %w", component, name, err))
			}
		}()
		return true
	}
}

// startedFailed records the failure of a started goroutine, and closes failed on the first.
func (h *Host) startedFailed(err error) {
	h.startedMu.Lock()
	defer h.startedMu.Unlock()
	if len(h.failures) == 0 {
		close(h.failed)
	}
	h.failures = append(h.failures, err)
}

// joinStarted stops Starter from starting anything and waits for every goroutine it started.
func (h *Host) joinStarted() {
	h.startedMu.Lock()
	h.joining = true
	h.startedMu.Unlock()
	h.started.Wait()
}

// startedFailures returns the failures of the goroutines Starter started. It is read once they have
// returned.
func (h *Host) startedFailures() error {
	h.startedMu.Lock()
	defer h.startedMu.Unlock()
	return errors.Join(h.failures...)
}

// Add adds a component's parts, its handler served at mount.
func (h *Host) Add(name string, mount []string, parts Parts) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.components = append(h.components, component{name: name, mount: mount, parts: parts})
}

// Close ends the host's contexts, waits for every goroutine its starters started and closes its pools,
// for a start that fails before it serves. Serve does the same on its way out.
func (h *Host) Close() {
	h.stopRun()
	h.joinStarted()
	h.mu.Lock()
	pools := h.pools
	h.pools = nil
	h.mu.Unlock()
	for _, p := range pools {
		p.Close()
	}
}

// Listeners are the listeners a process serves on. App is nil when no component serves requests.
type Listeners struct {
	App, Probes net.Listener
}

func (l Listeners) close() error {
	var errs []error
	for _, ln := range []net.Listener{l.App, l.Probes} {
		if ln != nil {
			errs = append(errs, ln.Close())
		}
	}
	return errors.Join(errs...)
}

// connectionLimits bound what one connection to either server can hold. A request's headers have 10
// seconds to arrive and the whole request 30, and an idle keep-alive connection is closed after two
// minutes. Nothing bounds writing the answer, since a request may wait for its leases and its provider
// calls, and an event stream is one answer that does not end (ADR-0130).
var connectionLimits = limits{Header: 10 * time.Second, Read: 30 * time.Second, Idle: 2 * time.Minute}

// limits are the timeouts a server runs with.
type limits struct{ Header, Read, Idle time.Duration }

// drainTimeout is how long the application server's requests have to finish once the process stops.
const drainTimeout = 10 * time.Second

// Serve serves the components added to h until ctx ends, a server fails, a task fails or a goroutine a
// starter started panics, then stops in order: the readiness probe answers not ready, Draining ends,
// the application server drains its requests for up to ten seconds, the tasks' context ends and every
// task and started goroutine is waited for, the operations server stops and the pools close. When s names a key pair, it is loaded before anything is served,
// so a start whose mounts cannot serve fails, and it is read again on each handshake (ADR-0127). Serve
// closes both listeners and the pools on every path.
func (h *Host) Serve(ctx context.Context, l Listeners, s Serving) error {
	defer h.Close()
	h.mu.Lock()
	components := h.components
	h.mu.Unlock()
	var tlsConf *tls.Config
	if l.App != nil && s.ServesTLS() {
		if _, err := tls.LoadX509KeyPair(s.TLSCert, s.TLSKey); err != nil {
			return errors.Join(fmt.Errorf("loading the TLS key pair: %w", err), l.close())
		}
		tlsConf = tlsConfig(s.TLSCert, s.TLSKey)
	}
	var serving atomic.Bool
	ready := func(r *http.Request) (bool, string) {
		if !serving.Load() {
			return false, "not ready"
		}
		for _, c := range components {
			for _, check := range c.parts.Ready {
				checking, cancel := context.WithTimeout(r.Context(), checkTimeout)
				err := check.Check(checking)
				cancel()
				if err != nil {
					h.logger.DebugContext(r.Context(), "a readiness check failed", componentLabel, c.name, "check", check.Name, "error", err)
					return false, "not ready: " + c.name + " " + check.Name
				}
			}
		}
		return true, ""
	}
	probeServer := newServer(probes.Handler(h.registry, ready, h.logger), h.logger)
	var app *http.Server
	if l.App != nil {
		mux := http.NewServeMux()
		for _, c := range components {
			if c.parts.Handler == nil {
				continue
			}
			for _, pattern := range c.mount {
				mux.Handle(pattern, c.parts.Handler)
			}
		}
		app = newServer(mux, h.logger)
		app.TLSConfig = tlsConf
		// A request's context ends once the tasks are told to stop, after the drain, so a request still
		// running then is told to stop rather than outliving the pools.
		app.BaseContext = func(net.Listener) context.Context { return h.run }
	}

	// One slot for each server and each task, so no send blocks.
	failed := make(chan error, 2+tasks(components))
	go func() { failed <- stopped(probeServer.Serve(l.Probes)) }()
	if app != nil {
		go func() {
			if tlsConf == nil {
				failed <- stopped(app.Serve(l.App))
				return
			}
			failed <- stopped(app.ServeTLS(l.App, "", ""))
		}()
	}
	var running sync.WaitGroup
	for _, c := range components {
		for _, t := range c.parts.Tasks {
			running.Add(1)
			go func() {
				defer running.Done()
				if err := recovered(h.run, t); err != nil {
					failed <- fmt.Errorf("%s task %s: %w", c.name, t.Name, err)
				}
			}()
		}
	}
	serving.Store(true)
	h.logger.Info("serving", "listen", address(l.App), "probe_listen", l.Probes.Addr().String(), "tls", tlsConf != nil)

	var err error
	select {
	case err = <-failed:
	case <-h.failed:
	case <-ctx.Done():
	}
	serving.Store(false)
	h.drain()
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), drainTimeout)
	defer cancel()
	if app != nil {
		err = errors.Join(err, shutdownServer(shutdown, app))
	}
	h.stopRun()
	running.Wait()
	h.joinStarted()
	return errors.Join(err, drained(failed), h.startedFailures(), shutdownServer(shutdown, probeServer))
}

// checkTimeout bounds one readiness check, so a probe is answered within the platform's own timeout.
const checkTimeout = 2 * time.Second

func tasks(components []component) int {
	n := 0
	for _, c := range components {
		n += len(c.parts.Tasks)
	}
	return n
}

// recovered runs t, and returns a panic it raises as its error, with the stack of its goroutine, so
// one task's panic is its failure and the process stops in order rather than at once.
func recovered(ctx context.Context, t Task) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v\n%s", ErrPanicked, r, debug.Stack())
		}
	}()
	return t.Run(ctx)
}

// ErrPanicked wraps a panic a task raised.
var ErrPanicked = errors.New("panicked")

// drained returns every failure waiting in failed once nothing more can send, the tasks' failures
// raised while the process stopped among them.
func drained(failed chan error) error {
	var errs []error
	for {
		select {
		case err := <-failed:
			errs = append(errs, err)
		default:
			return errors.Join(errs...)
		}
	}
}

// shutdownServer shuts srv down within ctx, and closes what is still open once ctx ends.
func shutdownServer(ctx context.Context, srv *http.Server) error {
	err := srv.Shutdown(ctx)
	if errors.Is(err, context.DeadlineExceeded) {
		return errors.Join(err, srv.Close())
	}
	return err
}

func address(ln net.Listener) string {
	if ln == nil {
		return ""
	}
	return ln.Addr().String()
}

// newServer returns a server for h bounded by the connection limits, logging through logger.
func newServer(h http.Handler, logger *slog.Logger) *http.Server {
	return &http.Server{
		Handler:           h,
		ReadHeaderTimeout: connectionLimits.Header,
		ReadTimeout:       connectionLimits.Read,
		IdleTimeout:       connectionLimits.Idle,
		ErrorLog:          logging.ServerErrorLog(logger),
	}
}

// stopped returns nil for a server that stopped because it was shut down.
func stopped(err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return err
}

// tlsConfig reads the key pair from its mounted files on each handshake, so a renewed certificate
// serves without a restart (ADR-0127).
func tlsConfig(certFile, keyFile string) *tls.Config {
	return &tls.Config{
		MinVersion: tls.VersionTLS12,
		GetCertificate: func(*tls.ClientHelloInfo) (*tls.Certificate, error) {
			pair, err := tls.LoadX509KeyPair(certFile, keyFile)
			if err != nil {
				return nil, fmt.Errorf("loading the TLS key pair: %w", err)
			}
			return &pair, nil
		},
	}
}
