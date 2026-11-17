package process

import (
	"context"
	"crypto/tls"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"sync/atomic"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/probes"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
)

// runTask builds the spec's task and runs its work under ctx, and returns the work's error once every
// goroutine the work started has returned and the slots are closed. It serves nothing and runs no
// job, since nothing probes, scrapes or calls a program that exits on its own (ADR-0139, ADR-0142).
func runTask[C any](ctx context.Context, p *proc, spec Spec[C], c C) error {
	stopBuild := context.AfterFunc(ctx, p.stopRun)
	work, err := spec.Task.Build(p.run, p.part(spec.Name, ""), c)
	stopBuild()
	if err != nil {
		return errors.Join(err, p.stop())
	}
	err = work(ctx)
	return errors.Join(err, p.stop())
}

// built is what the builds of a spec that runs until it is told to stop returned.
type built struct {
	handlers []mounted
	plans    []listed
}

type mounted struct {
	pattern string
	handler http.Handler
}

type listed struct {
	kind string
	plan schedule.Plan
	p    *capabilities
}

// build calls each endpoint's and each job kind's Build, in the spec's order, each with capabilities
// of its own.
func build[C any](p *proc, spec Spec[C], c C) (built, error) {
	var b built
	for _, e := range spec.Endpoints {
		h, err := e.Build(p.run, p.part(e.Pattern, ""), c)
		if err != nil {
			return b, fmt.Errorf("building the endpoint %s: %w", e.Pattern, err)
		}
		b.handlers = append(b.handlers, mounted{pattern: e.Pattern, handler: h})
	}
	for _, j := range spec.Jobs {
		caps := p.part(j.Name, j.Name)
		plan, err := j.Build(p.run, caps, c)
		if err != nil {
			return b, fmt.Errorf("building the job kind %s: %w", j.Name, err)
		}
		b.plans = append(b.plans, listed{kind: j.Name, plan: plan, p: caps})
	}
	return b, nil
}

// serve builds every part the spec lists, then serves the endpoints and runs the job kinds until ctx
// ends, a server fails or a goroutine started through Go panics, and stops in order.
func serve[C any](ctx context.Context, p *proc, spec Spec[C], c C, served parts) error {
	// The builds run under the run context, which outlives ctx until the process has drained. While
	// they run, ctx ending ends it too, so a start told to stop stops.
	stopBuild := context.AfterFunc(ctx, p.stopRun)
	b, err := build(p, spec, c)
	stopBuild()
	if err != nil {
		return errors.Join(err, p.stop())
	}
	var lc net.ListenConfig
	var app, probeListener net.Listener
	if len(b.handlers) > 0 {
		if app, err = lc.Listen(ctx, "tcp", served.serving.Listen); err != nil {
			return errors.Join(err, p.stop())
		}
	}
	if probeListener, err = lc.Listen(ctx, "tcp", served.operations.ProbeListen); err != nil {
		return errors.Join(err, closeListeners(app), p.stop())
	}
	return p.serveOn(ctx, app, probeListener, served.serving, b)
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

// serveOn serves on app, nil when the spec lists no endpoint, and on probes, and runs every job kind's
// plan on the scheduler, until ctx ends, a server fails or a goroutine started through Go panics.
// Then it stops in order: the readiness probe answers not ready, Draining ends, the application
// server drains its requests for up to ten seconds, the jobs' context ends and every job and every
// goroutine started through Go is waited for, the operations server stops and the slots are closed.
// When s names a key pair, it is loaded before anything is served, so a start whose key pair cannot
// serve fails, and it is read again on each handshake (ADR-0127).
func (p *proc) serveOn(ctx context.Context, app, probeListener net.Listener, s config.Serving, b built) error {
	var tlsConf *tls.Config
	if app != nil && s.ServesTLS() {
		if _, err := tls.LoadX509KeyPair(s.TLSCert, s.TLSKey); err != nil {
			return errors.Join(fmt.Errorf("loading the TLS key pair: %w", err), closeListeners(app, probeListener), p.stop())
		}
		tlsConf = tlsConfig(s.TLSCert, s.TLSKey)
	}
	var scheduler *schedule.Scheduler
	if len(b.plans) > 0 {
		// The scheduler is the process's, its series labelled with the program's name and no job kind,
		// each job's series carrying its own job kind (ADR-0119, ADR-0142).
		metrics, err := schedule.NewMetrics(p.part("scheduler", "").Metrics())
		if err != nil {
			return errors.Join(err, closeListeners(app, probeListener), p.stop())
		}
		scheduler = schedule.New(p.run, metrics)
		for _, l := range b.plans {
			name := l.kind + " listing"
			p.report(name, NotReady, "no listing has succeeded yet")
			err := scheduler.EnsurePlan(l.kind, l.plan, l.p.Logger(), func() { p.report(name, Ready, "") })
			if err != nil {
				err = fmt.Errorf("the scheduler refused the plan of the job kind %s: %w", l.kind, err)
				scheduler.Stop()
				return errors.Join(err, closeListeners(app, probeListener), p.stop())
			}
		}
	}

	var serving atomic.Bool
	ready := func(*http.Request) (bool, string) {
		if !serving.Load() {
			return false, "not ready"
		}
		line, notReady := p.notReady()
		return !notReady, line
	}
	probeServer := newServer(probes.Handler(p.registry, ready, p.logger), p.logger)
	var appServer *http.Server
	if app != nil {
		mux := http.NewServeMux()
		for _, m := range b.handlers {
			mux.Handle(m.pattern, m.handler)
		}
		appServer = newServer(mux, p.logger)
		appServer.TLSConfig = tlsConf
		// A request's context ends once the jobs are told to stop, after the drain, so a request still
		// running then is told to stop rather than outliving the slots.
		appServer.BaseContext = func(net.Listener) context.Context { return p.run }
	}

	// One place for each server, so no send blocks.
	failed := make(chan error, 2)
	go func() { failed <- serverFailed("the operations server", probeServer.Serve(probeListener)) }()
	if appServer != nil {
		go func() {
			if tlsConf == nil {
				failed <- serverFailed("the application server", appServer.Serve(app))
				return
			}
			failed <- serverFailed("the application server", appServer.ServeTLS(app, "", ""))
		}()
	}
	serving.Store(true)
	p.logger.Info("serving", "listen", address(app), "probe_listen", probeListener.Addr().String(), "tls", tlsConf != nil)

	var err error
	select {
	case err = <-failed:
	case <-p.failed:
	case <-ctx.Done():
	}
	serving.Store(false)
	p.drain()
	shutdown, cancel := context.WithTimeout(context.WithoutCancel(ctx), drainTimeout)
	defer cancel()
	if appServer != nil {
		err = errors.Join(err, shutdownServer(shutdown, appServer))
	}
	p.stopRun()
	if scheduler != nil {
		scheduler.Stop()
	}
	goroutines := p.joinGoroutines()
	err = errors.Join(err, drained(failed), goroutines, shutdownServer(shutdown, probeServer))
	p.closeSlots()
	return err
}

// recovered runs f, and returns a panic it raises as its error, with the stack of its goroutine.
func recovered(f func() error) (err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("%w: %v\n%s", ErrPanicked, r, debug.Stack())
		}
	}()
	return f()
}

// ErrPanicked wraps a panic a goroutine started through Go raised.
var ErrPanicked = errors.New("panicked")

// drained returns every failure waiting in failed once nothing more can send.
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

func closeListeners(listeners ...net.Listener) error {
	var errs []error
	for _, ln := range listeners {
		if ln != nil {
			errs = append(errs, ln.Close())
		}
	}
	return errors.Join(errs...)
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

// serverFailed returns nil for a server that stopped because it was shut down, and otherwise its
// error, naming the server.
func serverFailed(server string, err error) error {
	if errors.Is(err, http.ErrServerClosed) {
		return nil
	}
	return fmt.Errorf("%s failed: %w", server, err)
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
