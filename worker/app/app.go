// Package app is the worker's entry package and composition root. It constructs the object graph by
// hand in ordinary code, and worker/main.go calls it. It sits outside internal, so a composition root
// other than the worker's own can compose it, and every other package of this deployable sits under
// internal.
//
// It builds one scheduler, one connection pool per job kind under that kind's own runtime role, and
// each job kind with what it needs and nothing more, then ensures each kind's reload, which adds and
// drops the kind's account jobs (ADR-0117, ADR-0118, ADR-0119). It links no data-access subsection
// itself and runs no statement on the pools it opens. Each pool reaches only its own job kind's code,
// which runs its statements under its role. The import lists cannot see which pool reaches which
// code, so TestEachJobKindIsHandedThePoolOfItsOwnRole holds that openPools opens each pool as its
// kind's role and that kinds hands each kind its own, and review holds that Run passes the pools it
// opens on through assemble (ADR-0118).
package app

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"

	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/process/dbconnect"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
	"github.com/ppat/mediated-mailbox-mcp/process/dbmetrics"
	"github.com/ppat/mediated-mailbox-mcp/process/logging"
	"github.com/ppat/mediated-mailbox-mcp/process/probes"
	"github.com/ppat/mediated-mailbox-mcp/process/settings"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
)

// scannerSection is the scanner's section of the configuration, whose revision every verdict and
// masking decision is made under (ADR-0005, ADR-0078). Backfill and delta sync scan with the one
// scanner built from it, so both record the same revision by construction (ADR-0120).
const scannerSection = "scanner"

// Configuration is the worker's root configuration type, loaded by the configuration library
// (ADR-0078). It holds the sections every job kind shares, read once, and a section per job kind. Its
// fields are pinned by the test beside this file, so a new value is a visible change. No account and
// no provider credential is configuration. Both come from the database (ADR-0080).
type Configuration struct {
	// ProbeListen is the address the health probe and the metrics endpoint listen on, over plain
	// HTTP (ADR-0051).
	ProbeListen string `yaml:"probe_listen"`
	// ReloadInterval is how often each job kind reloads its account snapshot and its policy, and so
	// how soon an account connected, a credential replaced or a policy edited reaches the worker, and
	// how often backfill asks each account whether it has work (ADR-0090, ADR-0119).
	ReloadInterval time.Duration         `yaml:"reload_interval"`
	Database       Database              `yaml:"database"`
	Credential     credentialcore.Config `yaml:"credential"`
	// Scanner is the scanner's section, one for every job kind that masks or scans (ADR-0120).
	Scanner scan.Config `yaml:"scanner"`
	// LogLevel is the lowest level the worker logs at, debug, info, warn or error (ADR-0122).
	LogLevel string   `yaml:"log_level"`
	Backfill Backfill `yaml:"backfill"`
	Sync     Sync     `yaml:"sync"`
}

// Database is the worker's database section. The server is named once, and each job kind connects as
// a runtime role of its own, with its own password file and its own pool (ADR-0078, ADR-0118).
type Database struct {
	Host        string `yaml:"host" settings:"required"`
	Port        int    `yaml:"port"`
	Name        string `yaml:"name" settings:"required"`
	SSLMode     string `yaml:"sslmode"`
	SSLRootCert string `yaml:"sslrootcert"`
	Backfill    Role   `yaml:"backfill"`
	Sync        Role   `yaml:"sync"`
}

// Role is one job kind's connection, its runtime role, the mounted file holding its password, and how
// many connections its pool opens at most (ADR-0079).
type Role struct {
	User         string `yaml:"user"`
	PasswordFile string `yaml:"password_file" settings:"required"`
	PoolSize     int    `yaml:"pool_size"`
}

// Backfill is backfill's section.
type Backfill struct {
	// Concurrency is how many accounts' backfills run at once, which bounds the bodies backfill holds
	// at once (ADR-0119).
	Concurrency int `yaml:"concurrency"`
}

// Sync is delta sync's section.
type Sync struct {
	// SyncInterval is how often each account ticks, the sync interval (ADR-0018, ADR-0103).
	SyncInterval time.Duration `yaml:"sync_interval"`
	// FirstWindow is how far back an account with no cursor is reconciled at its first tick
	// (ADR-0105).
	FirstWindow time.Duration `yaml:"first_window"`
	// DecisionsPerTick bounds how many waiting messages one tick decides for an account (ADR-0104).
	DecisionsPerTick int `yaml:"decisions_per_tick"`
	// Concurrency is how many accounts tick at once, which bounds the bodies delta sync holds at once
	// (ADR-0119).
	Concurrency int `yaml:"concurrency"`
}

// defaults are the worker's defaults. Each job kind's user is its own runtime role (ADR-0118), and the
// TLS mode is the one that fails closed. Each pool's size is set explicitly, since the driver's default
// follows the node's processors and not the worker's limit. The reload interval is the mediator's, the
// sync interval ADR-0018's, the first window ADR-0105's, and the bound on the waiting messages a tick
// decides the top of ADR-0018's steady-state tick. The key files and the password files have no
// default, since a default path assumes the environment. The scanner's vocabulary and tuning are the
// ones the application ships.
func defaults() Configuration {
	return Configuration{
		ProbeListen:    ":8080",
		ReloadInterval: time.Minute,
		Database: Database{
			Port: 5432, SSLMode: "verify-full",
			Backfill: Role{User: "mediated_mailbox_backfill", PoolSize: 4},
			Sync:     Role{User: "mediated_mailbox_sync", PoolSize: 4},
		},
		Scanner:  scan.DefaultConfig(),
		LogLevel: logging.DefaultLevel,
		Backfill: Backfill{Concurrency: 2},
		Sync:     Sync{SyncInterval: 5 * time.Minute, FirstWindow: 7 * 24 * time.Hour, DecisionsPerTick: 200, Concurrency: 4},
	}
}

// connection returns the job kind's connection as the database section of a deployable, the server's
// values with the role's.
func (d Database) connection(r Role) dbconnectcore.Config {
	return dbconnectcore.Config{
		Host: d.Host, Port: d.Port, Name: d.Name, User: r.User, SSLMode: d.SSLMode, SSLRootCert: d.SSLRootCert, PasswordFile: r.PasswordFile,
	}
}

// validate refuses a configuration the worker cannot run with. Each job kind's connection passes the
// database section's validation, every pool, interval, window and bound is positive, and the
// credential section names its key files.
func validate(c Configuration) error {
	for _, kind := range []struct {
		name string
		role Role
	}{{backfill.Kind, c.Database.Backfill}, {deltasync.Kind, c.Database.Sync}} {
		// The role's own values are refused by their own paths, so the shared ones are all a refusal
		// of the composed section can name, and its paths are the worker's too.
		switch {
		case kind.role.User == "":
			return fmt.Errorf("database.%s.user is empty", kind.name)
		case kind.role.PasswordFile == "":
			return fmt.Errorf("database.%s.password_file is empty", kind.name)
		}
		if err := dbconnectcore.Validate(c.Database.connection(kind.role)); err != nil {
			return err
		}
		if kind.role.PoolSize <= 0 {
			return fmt.Errorf("database.%s.pool_size %d is not positive", kind.name, kind.role.PoolSize)
		}
	}
	switch {
	case c.ReloadInterval <= 0:
		return fmt.Errorf("reload_interval %s is not positive", c.ReloadInterval)
	case c.Backfill.Concurrency <= 0:
		return fmt.Errorf("backfill.concurrency %d is not positive", c.Backfill.Concurrency)
	case c.Sync.SyncInterval <= 0:
		return fmt.Errorf("sync.sync_interval %s is not positive", c.Sync.SyncInterval)
	case c.Sync.FirstWindow <= 0:
		return fmt.Errorf("sync.first_window %s is not positive", c.Sync.FirstWindow)
	case c.Sync.DecisionsPerTick <= 0:
		return fmt.Errorf("sync.decisions_per_tick %d is not positive", c.Sync.DecisionsPerTick)
	case c.Sync.Concurrency <= 0:
		return fmt.Errorf("sync.concurrency %d is not positive", c.Sync.Concurrency)
	}
	return credentialcore.Validate(c.Credential)
}

// Run loads the configuration and the keyring, connects each job kind to the database as its own
// role, and runs every job kind until ctx ends, serving the health probe and the metrics endpoint the
// whole time (ADR-0051, ADR-0117). It logs the effective configuration first, each value with the
// layer that set it (ADR-0078). The scanner is built from its section before anything else starts, so
// a section it refuses refuses the start. The keyring is loaded before any connection is made, so a
// public key matching none of the private keys refuses the start (ADR-0088). When ctx ends, every
// run's context is cancelled and Run returns once each has returned (ADR-0119).
//
// logger is the logger main.go built over level, at info until Run sets level from log_level once
// the effective configuration is written, so every logger derived from logger follows it (ADR-0122).
func Run(ctx context.Context, args, environ []string, logger *slog.Logger, level *slog.LevelVar) error {
	if err := dbconnect.RefusePasswordVariables(environ); err != nil {
		return err
	}
	loaded, err := settings.Load(defaults(), args, environ)
	var help *settings.HelpRequested
	if errors.As(err, &help) {
		fmt.Print(help.Text)
		return nil
	}
	if err != nil {
		return fmt.Errorf("loading the configuration: %w", err)
	}
	// The effective configuration is written before the level applies, so a refused value's source,
	// log_level's included, reaches the log whatever level the configuration sets (ADR-0078, ADR-0122).
	for _, v := range loaded.Values {
		logger.Info("configuration", "path", v.Path, "source", v.Source.String(), "value", v.Value)
	}
	lv, err := logging.ParseLevel(loaded.Config.LogLevel)
	if err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	level.Set(lv)
	c := loaded.Config
	if err := validate(c); err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	scanner, err := buildScanner(loaded)
	if err != nil {
		return fmt.Errorf("validating the configuration: %w", err)
	}
	keys, err := open.Load(c.Credential.PublicKeyFile, c.Credential.PrivateKeyFiles...)
	if err != nil {
		return fmt.Errorf("loading the keyring: %w", err)
	}
	registry := prometheus.NewRegistry()
	if err := registerProcess(registry); err != nil {
		return err
	}
	pools, err := openPools(ctx, c.Database, registry)
	if err != nil {
		return err
	}
	defer pools.Backfill.Close()
	defer pools.Sync.Close()
	ln, err := net.Listen("tcp", c.ProbeListen)
	if err != nil {
		return fmt.Errorf("listening for the probes: %w", err)
	}
	w, err := assemble(ctx, registry, pools, keys, scanner, c, http.DefaultClient, logger)
	if err != nil {
		return errors.Join(err, ln.Close())
	}
	stopProbes := probes.Serve(ln, w.registry, logger)
	err = w.run(ctx)
	return errors.Join(err, stopProbes())
}

// openPools returns each job kind's pool, connecting as that kind's own role, measured on registry
// labelled with the job kind. A pool opens no connection until its first use (ADR-0118, ADR-0125).
func openPools(ctx context.Context, d Database, registry prometheus.Registerer) (Pools, error) {
	backfillPool, err := connect(ctx, d, d.Backfill, labelled(registry, backfill.Kind))
	if err != nil {
		return Pools{}, err
	}
	syncPool, err := connect(ctx, d, d.Sync, labelled(registry, deltasync.Kind))
	if err != nil {
		backfillPool.Close()
		return Pools{}, err
	}
	return Pools{Backfill: backfillPool, Sync: syncPool}, nil
}

// connect returns the job kind's pool, connecting as its role with its password file, opening at most
// its pool size of connections, measured on the job kind's registerer (ADR-0118, ADR-0125).
func connect(ctx context.Context, d Database, r Role, registry prometheus.Registerer) (*pgxpool.Pool, error) {
	config, err := dbconnect.PoolConfig(d.connection(r))
	if err != nil {
		return nil, fmt.Errorf("configuring the database connection of %s: %w", r.User, err)
	}
	config.MaxConns = int32(min(r.PoolSize, 1<<31-1)) //nolint:gosec // Bounded above.
	pool, err := measured(ctx, config, registry)
	if err != nil {
		return nil, fmt.Errorf("configuring the database connection of %s: %w", r.User, err)
	}
	return pool, nil
}

// measured opens a pool from config with every statement it runs timed and its statistics read on
// registry, which carries the job kind whose pool it is (ADR-0125).
func measured(ctx context.Context, config *pgxpool.Config, registry prometheus.Registerer) (*pgxpool.Pool, error) {
	tracer, err := dbmetrics.NewTracer(registry)
	if err != nil {
		return nil, err
	}
	config.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(ctx, config)
	if err != nil {
		return nil, err
	}
	if err := dbmetrics.RegisterPool(registry, pool); err != nil {
		pool.Close()
		return nil, err
	}
	return pool, nil
}

// registerProcess registers on registry the Go runtime's series, the default set and the live heap,
// the garbage collector's and the process's CPU time and the scheduling latency, and the process's
// own series, once for the process and without a job kind (ADR-0125).
func registerProcess(registry prometheus.Registerer) error {
	for _, c := range []prometheus.Collector{
		collectors.NewGoCollector(collectors.WithGoCollectorRuntimeMetrics(collectors.GoRuntimeMetricsRule{Matcher: runtimeMetrics})),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	} {
		if err := registry.Register(c); err != nil {
			return err
		}
	}
	return nil
}

// runtimeMetrics names the runtime metrics added to the Go collector's default set, each by its whole
// name (ADR-0125).
var runtimeMetrics = regexp.MustCompile(`^(/gc/heap/live:bytes|/cpu/classes/gc/total:cpu-seconds|/cpu/classes/total:cpu-seconds|/sched/latencies:seconds)$`)

// buildScanner builds the scanner from its section of the loaded configuration, under the
// configuration library's revision of that section, so a change to any of its values in any layer
// changes the revision every verdict and mask records, and a change to another section does not
// (ADR-0005, ADR-0078, ADR-0120).
func buildScanner(loaded settings.Loaded[Configuration]) (scan.Scanner, error) {
	s, err := scan.New(loaded.Config.Scanner, loaded.Revisions[scannerSection])
	if err != nil {
		return scan.Scanner{}, fmt.Errorf("scanner: %w", err)
	}
	return s, nil
}

// Pools are the job kinds' connection pools, each connecting as its own kind's runtime role.
type Pools struct {
	Backfill, Sync *pgxpool.Pool
}

// worker is the assembled worker, its registry, its scheduler and the reload of each job kind.
type worker struct {
	registry  *prometheus.Registry
	scheduler *schedule.Scheduler
	reloads   []reload
}

// reload is one job kind's reload job and its key.
type reload struct {
	key schedule.Key
	job schedule.Job
}

// assemble builds everything the worker runs on the process's registry, the scheduler with its series
// on it, and each job kind on its own pool, with its series on the registry labelled by its job kind
// and a logger carrying it. Each Gmail adapter calls through client. Run and the tests of the scrape
// both call it, so every series the worker emits is the one the metrics endpoint serves (ADR-0076,
// ADR-0077).
func assemble(ctx context.Context, registry *prometheus.Registry, pools Pools, keys *open.Keyring, scanner scan.Scanner, c Configuration, client *http.Client,
	logger *slog.Logger,
) (*worker, error) {
	metrics, err := schedule.NewMetrics(registry)
	if err != nil {
		return nil, err
	}
	s := schedule.New(ctx, metrics)
	bc, dc := kinds(pools, keys, scanner, c, client, logger, registry, s)
	b, err := backfill.New(bc)
	if err != nil {
		return nil, err
	}
	d, err := deltasync.New(dc)
	if err != nil {
		return nil, err
	}
	w := &worker{registry: registry, scheduler: s}
	for _, kind := range []interface {
		Reload() (schedule.Key, schedule.Job)
	}{b, d} {
		key, job := kind.Reload()
		w.reloads = append(w.reloads, reload{key: key, job: job})
	}
	return w, nil
}

// kinds returns what each job kind is built from. Each is handed its own pool, the one keyring, the
// one scanner built from the worker's scanner section, so both record the same scanner version and
// revision on every verdict and mask (ADR-0117, ADR-0120), the registry labelled with its job kind, a
// logger carrying it, and the scheduler, which adds and drops its jobs.
func kinds(pools Pools, keys *open.Keyring, scanner scan.Scanner, c Configuration, client *http.Client, logger *slog.Logger,
	registry prometheus.Registerer, s *schedule.Scheduler,
) (backfill.Config, deltasync.Config) {
	return backfill.Config{
		Pool: pools.Backfill, Keys: keys, Scanner: scanner, Client: client,
		Registry: labelled(registry, backfill.Kind), Logger: logger.With("job_kind", backfill.Kind), Jobs: s,
		Concurrency: c.Backfill.Concurrency, ReloadInterval: c.ReloadInterval,
	}, deltasync.Config{
		Pool: pools.Sync, Keys: keys, Scanner: scanner, Client: client,
		Registry: labelled(registry, deltasync.Kind), Logger: logger.With("job_kind", deltasync.Kind), Jobs: s,
		Concurrency: c.Sync.Concurrency, ReloadInterval: c.ReloadInterval,
		SyncInterval: c.Sync.SyncInterval, FirstWindow: c.Sync.FirstWindow, DecisionsPerTick: c.Sync.DecisionsPerTick,
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
