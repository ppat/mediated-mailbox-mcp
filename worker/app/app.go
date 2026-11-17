// Package app is the worker's entry package. It exports the worker's assembly, which takes what the
// worker needs as values, starts nothing and returns what it provides, and its composition root,
// Run, which hands the host the worker and its configuration, config.Worker (ADR-0040, ADR-0078,
// ADR-0138). worker/main.go calls Run. It sits outside internal, so a composition root other than
// the worker's own can compose it, and every other package of this deployable sits under internal.
//
// It builds one scheduler, one connection pool per job kind, each connecting as the one application
// user, the worker's one execution context over delta sync's pool, and each job kind with what it
// needs and nothing more, and its one task ensures each kind's reload, which adds and drops the kind's
// account jobs from what the context's latest reload left in force, and which each reload of the
// context wakes (ADR-0090, ADR-0117, ADR-0119, ADR-0143). The context's reload is a second task. It
// links no data-access subsection itself and runs no statement on the pools it opens. Each job kind's
// pool reaches only its own job kind's code, so one job kind cannot starve another of connections. The
// import lists cannot see which pool reaches which code, so TestEachJobKindIsHandedAPoolOfItsOwn holds
// that openPools opens a pool per job kind and that kinds hands each kind its own, and review holds
// that the root passes the pools it opens on through Assemble (ADR-0119).
package app

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"sync/atomic"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/db/migrations"
	"github.com/ppat/mediated-mailbox-mcp/db/schemacompatibility"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/assembly"
	"github.com/ppat/mediated-mailbox-mcp/process/host"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// scannerSection is the scanner's section of the configuration, whose revision every verdict and
// masking decision is made under (ADR-0005, ADR-0078). Backfill and delta sync scan with the
// execution context's one scanner built from it, so both record the same revision by construction
// (ADR-0120).
const scannerSection = "scanner"

// Run is the worker's process. The host loads the configuration, and the worker is assembled from
// it, the scanner built from its section before anything else starts, so a section it refuses refuses
// the start, and the keyring loaded before any connection is made, so a public key matching none of
// the private keys refuses the start (ADR-0088). When ctx ends, every run's context is cancelled and
// Run returns once each has returned (ADR-0119, ADR-0138).
//
// logger is the logger main.go built over level, at info until the host sets level from log_level
// once the effective configuration is written, so every logger derived from logger follows it
// (ADR-0122). stdout answers --help and --version, and stderr is where the start banner goes,
// standard error in main.go, so standard output carries only the logger's JSON records otherwise.
func Run(ctx context.Context, args, environ []string, stdout, stderr io.Writer, logger *slog.Logger, level *slog.LevelVar) error {
	return host.Run(ctx, args, environ, stdout, stderr, logger, level, host.Spec[config.Worker]{
		Name:        "mediated-mailbox-worker",
		Defaults:    config.DefaultWorker(),
		Process:     func(c config.Worker) config.Process { return c.Process },
		Operations:  func(c config.Worker) config.Operations { return c.Operations },
		Validate:    config.Worker.Check,
		CheckSchema: checkSchema,
		Components:  []host.Component[config.Worker]{{Name: "worker", Assemble: fromConfiguration}},
	})
}

// checkSchema refuses a schema this binary's migration chain cannot run on, read through pool under
// the user pool connects as. The host runs it on each pool it opens, before the pool is used
// (ADR-0140).
func checkSchema(ctx context.Context, pool *pgxpool.Pool) error {
	v, err := schemacompatibility.New(pool).SchemaVersions(ctx)
	if err != nil {
		return fmt.Errorf("reading the schema's versions: %w", err)
	}
	return migrations.Refuse(v.Applied, v.OldestChainVersion)
}

// fromConfiguration builds what the worker needs from the configuration, the scanner from its section
// under the section's revision, the keyring, a pool per job kind connecting as the application user,
// and the worker's execution context over delta sync's pool (ADR-0119, ADR-0143), and assembles the
// worker from them. The context's reload is the
// process's task.
func fromConfiguration(ctx context.Context, provided host.Provided[config.Worker]) (host.Parts, error) {
	scanner, err := buildScanner(provided.Loaded)
	if err != nil {
		return host.Parts{}, fmt.Errorf("validating the configuration: %w", err)
	}
	c := provided.Loaded.Config
	keys, err := open.Load(c.Credential.PublicKeyFile, c.Credential.PrivateKeyFiles...)
	if err != nil {
		return host.Parts{}, fmt.Errorf("loading the keyring: %w", err)
	}
	pools, err := openPools(ctx, c.Database, provided.OpenPool, provided.Registerer)
	if err != nil {
		return host.Parts{}, err
	}
	ec, err := newContext(pools, keys, scanner, c.ReloadInterval, provided.Registerer, provided.Logger)
	if err != nil {
		return host.Parts{}, err
	}
	parts, err := Assemble(ctx, Needs{
		Config: c, Pools: pools, Context: ec, Client: http.DefaultClient,
		Registerer: provided.Registerer, Logger: provided.Logger,
	})
	if err != nil {
		return host.Parts{}, err
	}
	parts.Tasks = append(parts.Tasks, host.Task{Name: "execution context reload", Run: ec.Run})
	return parts, nil
}

// newContext returns the worker's execution context over delta sync's pool, reloading on the reload
// interval (ADR-0090).
func newContext(pools Pools, keys *open.Keyring, scanner scan.Scanner, reload config.Duration, reg prometheus.Registerer, logger *slog.Logger) (*assembly.Context, error) {
	return assembly.New(assembly.Needs{
		Pool: pools.Sync, Keys: keys, ThroughClient: []string{gmailProvider}, Scanner: scanner,
		Lookups:        classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
		ReloadInterval: time.Duration(reload), Registerer: reg, Logger: logger,
	})
}

// gmailProvider is the provider an account served through the Gmail adapter names, in accounts and
// in oauth_clients.
const gmailProvider = "gmail"

// opener opens a pool from a connection, measured on a registerer.
type opener func(ctx context.Context, c config.Database, reg prometheus.Registerer) (*pgxpool.Pool, error)

// openPools returns a pool of its own for each job kind through open, each from the one database
// section, so each connects as the application user and opens at most the section's pool size of
// connections, measured on registry labelled with the job kind. A pool opens no connection until its
// first use (ADR-0119, ADR-0125, ADR-0143).
func openPools(ctx context.Context, d config.Database, open opener, registry prometheus.Registerer) (Pools, error) {
	backfillPool, err := open(ctx, d, labelled(registry, backfill.Kind))
	if err != nil {
		return Pools{}, err
	}
	syncPool, err := open(ctx, d, labelled(registry, deltasync.Kind))
	if err != nil {
		return Pools{}, err
	}
	return Pools{Backfill: backfillPool, Sync: syncPool}, nil
}

// buildScanner builds the scanner from its section of the loaded configuration, under the
// configuration library's revision of that section, so a change to any of its values in any layer
// changes the revision every verdict and mask records, and a change to another section does not
// (ADR-0005, ADR-0078, ADR-0120).
func buildScanner(loaded settings.Loaded[config.Worker]) (scan.Scanner, error) {
	s, err := scan.New(loaded.Config.Scanner, loaded.Revisions[scannerSection])
	if err != nil {
		return scan.Scanner{}, fmt.Errorf("scanner: %w", err)
	}
	return s, nil
}

// Pools are the job kinds' connection pools, one per job kind, each connecting as the application user.
type Pools struct {
	Backfill, Sync *pgxpool.Pool
}

// Needs are what the worker is assembled from.
type Needs struct {
	// Config is the worker's configuration, of which the assembly reads the reload interval and each
	// job kind's section.
	Config config.Worker
	// Pools are the job kinds' pools, one per job kind (ADR-0119).
	Pools Pools
	// Context is the process's execution context, whose scanner every job kind masks and scans with
	// (ADR-0090, ADR-0120).
	Context *assembly.Context
	// Client is the HTTP client each Gmail adapter calls through.
	Client *http.Client
	// Registerer is where every series of the worker is registered, each job kind's labelled with it
	// (ADR-0076, ADR-0117).
	Registerer prometheus.Registerer
	Logger     *slog.Logger
}

// Assemble builds the worker from n and returns its parts, the task that runs the scheduler and the
// readiness checks, each job kind's pool answering and each job kind's first reload done.
// Each reload of the execution context wakes every job kind's reload. It starts nothing. The
// scheduler's runs end when ctx ends (ADR-0119).
func Assemble(ctx context.Context, n Needs) (host.Parts, error) {
	w, err := assemble(ctx, n)
	if err != nil {
		return host.Parts{}, err
	}
	parts := host.Parts{Tasks: []host.Task{{Name: "scheduler", Run: w.run}}}
	for _, r := range w.reloads {
		parts.Ready = append(parts.Ready, host.Check{Name: r.key.Kind + " reload", Check: r.reloaded})
	}
	for kind, pool := range map[string]*pgxpool.Pool{backfill.Kind: n.Pools.Backfill, deltasync.Kind: n.Pools.Sync} {
		parts.Ready = append(parts.Ready, host.Check{Name: kind + " database", Check: pool.Ping})
	}
	return parts, nil
}

// worker is the assembled worker, its scheduler and the reload of each job kind.
type worker struct {
	scheduler *schedule.Scheduler
	reloads   []*reload
}

// reload is one job kind's reload job and its key, and whether a run of it has succeeded.
type reload struct {
	key  schedule.Key
	job  schedule.Job
	done atomic.Bool
}

// reloaded returns nil once a run of the reload has succeeded, so the job kind has read its accounts
// and ensured their jobs at least once.
func (r *reload) reloaded(context.Context) error {
	if !r.done.Load() {
		return errors.New("no reload has succeeded yet")
	}
	return nil
}

// assemble builds everything the worker runs, the scheduler with its series on n's registerer, and
// each job kind on its own pool, with its series on the registerer labelled by its job kind and a
// logger carrying it. Each Gmail adapter calls through n's client. Assemble and the tests of the scrape
// both call it, so every series the worker emits is the one the metrics endpoint serves (ADR-0076,
// ADR-0077).
func assemble(ctx context.Context, n Needs) (*worker, error) {
	metrics, err := schedule.NewMetrics(n.Registerer)
	if err != nil {
		return nil, err
	}
	s := schedule.New(ctx, metrics)
	bc, dc := kinds(n.Pools, n.Context, n.Config, n.Client, n.Logger, n.Registerer, s)
	b, err := backfill.New(bc)
	if err != nil {
		return nil, err
	}
	d, err := deltasync.New(dc)
	if err != nil {
		return nil, err
	}
	w := &worker{scheduler: s}
	for _, kind := range []interface {
		Reload() (schedule.Key, schedule.Job)
	}{b, d} {
		key, job := kind.Reload()
		r := &reload{key: key}
		run := job.Run
		job.Run = func(ctx context.Context, ask schedule.Ask) error {
			err := run(ctx, ask)
			if err == nil {
				r.done.Store(true)
			}
			return err
		}
		r.job = job
		w.reloads = append(w.reloads, r)
	}
	n.Context.OnReload(func(context.Context) {
		for _, r := range w.reloads {
			s.Wake(r.key)
		}
	})
	return w, nil
}

// kinds returns what each job kind is built from. Each is handed its own pool, the one execution
// context, whose one scanner built from the worker's scanner section has both record the same scanner
// version and revision on every verdict and mask (ADR-0117, ADR-0120), the registerer labelled with its
// job kind, a logger carrying it, and the scheduler, which adds and drops its jobs.
func kinds(pools Pools, ec *assembly.Context, c config.Worker, client *http.Client, logger *slog.Logger,
	registry prometheus.Registerer, s *schedule.Scheduler,
) (backfill.Config, deltasync.Config) {
	return backfill.Config{
		Pool: pools.Backfill, Context: ec, Client: client,
		Registry: labelled(registry, backfill.Kind), Logger: logger.With("job_kind", backfill.Kind), Jobs: s,
		Concurrency: c.Backfill.Concurrency, ReloadInterval: time.Duration(c.ReloadInterval),
	}, deltasync.Config{
		Pool: pools.Sync, Context: ec, Client: client,
		Registry: labelled(registry, deltasync.Kind), Logger: logger.With("job_kind", deltasync.Kind), Jobs: s,
		Concurrency: c.Sync.Concurrency, ReloadInterval: time.Duration(c.ReloadInterval),
		SyncInterval: time.Duration(c.Sync.SyncInterval), FirstWindow: time.Duration(c.Sync.FirstWindow), DecisionsPerTick: c.Sync.DecisionsPerTick,
	}
}

// labelled returns registry with every series registered through it labelled by the job kind, so the
// series a shared library registers once per job kind stay attributable to the kind that emits them,
// and two job kinds registering one library's series do not collide (ADR-0117).
func labelled(registry prometheus.Registerer, kind string) prometheus.Registerer {
	return prometheus.WrapRegistererWith(prometheus.Labels{"job_kind": kind}, registry)
}

// run ensures each job kind's reload, whose first run comes at once and ensures the kind's account
// jobs, and returns once ctx has ended and every run has returned.
func (w *worker) run(ctx context.Context) error {
	for _, r := range w.reloads {
		if err := w.scheduler.Ensure(r.key, r.job); err != nil {
			w.scheduler.Stop()
			return err
		}
	}
	<-ctx.Done()
	w.scheduler.Stop()
	return nil
}
