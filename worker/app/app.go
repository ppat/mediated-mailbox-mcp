// Package app is the worker's entry package. It exports the worker's section of the configuration
// and its assembly, which takes what the worker needs as values, starts nothing and returns what it
// provides, and its composition root, Run, which hands the host the worker and the configuration it
// reads (ADR-0040, ADR-0138). worker/main.go calls Run. It sits outside internal, so a composition
// root other than the worker's own can compose it, and every other package of this deployable sits
// under internal.
//
// It builds one scheduler, one connection pool per job kind under that kind's own runtime role, the
// worker's one execution context over delta sync's pool, and each job kind with what it needs and
// nothing more, and its one task ensures each kind's reload, which adds and drops the kind's account
// jobs from what the context's latest reload left in force, and which each reload of the context wakes
// (ADR-0090, ADR-0117, ADR-0118, ADR-0119). The context's reload is a second task. It links no
// data-access subsection itself and runs no statement on the pools it opens. Each job kind's pool
// reaches only its own job kind's code, which runs its statements under its role, and the context's
// statements run under delta sync's role. The import lists cannot see which pool reaches which code,
// so TestEachJobKindIsHandedThePoolOfItsOwnRole holds that openPools opens each pool as its kind's role,
// that kinds hands each kind its own and that the context is built over delta sync's, and review holds
// that the root passes the pools it opens on through Assemble (ADR-0118).
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
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/assembly"
	credentialcore "github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/core"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	dbconnectcore "github.com/ppat/mediated-mailbox-mcp/process/dbconnect/core"
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

// Configuration is the worker's root configuration type, loaded by the configuration library
// (ADR-0078). It composes the process's section and the worker's own section, inline, with the
// sections every job kind shares, read once, and the database section with a role per job kind. Its
// fields are pinned by the test beside this file, so a new value is a visible change. No account and
// no provider credential is configuration. Both come from the database (ADR-0080).
type Configuration struct {
	host.Process    `yaml:",inline"`
	host.Operations `yaml:",inline"`
	Section         `yaml:",inline"`
	Database        Database              `yaml:"database"`
	Credential      credentialcore.Config `yaml:"credential"`
	// Scanner is the scanner's section, one for every job kind that masks or scans (ADR-0120).
	Scanner scan.Config `yaml:"scanner"`
}

// Section is the worker's own section of the configuration, the values every job kind reads and a
// section per job kind.
type Section struct {
	// ReloadInterval is how often the worker's execution context reloads its account snapshot and its
	// policy, and so how soon an account connected, a credential replaced or a policy edited reaches
	// the worker, how often each job kind's reload is asked beside the wake each context reload gives
	// it, and how often backfill asks each account whether it has work (ADR-0090, ADR-0119).
	ReloadInterval time.Duration `yaml:"reload_interval"`
	Backfill       Backfill      `yaml:"backfill"`
	Sync           Sync          `yaml:"sync"`
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

// DefaultSection is the worker's own section's defaults. The reload interval is the API server's, the
// sync interval ADR-0018's, the first window ADR-0105's, and the bound on the waiting messages a tick
// decides the top of ADR-0018's steady-state tick.
func DefaultSection() Section {
	return Section{
		ReloadInterval: time.Minute,
		Backfill:       Backfill{Concurrency: 2},
		Sync:           Sync{SyncInterval: 5 * time.Minute, FirstWindow: 7 * 24 * time.Hour, DecisionsPerTick: 200, Concurrency: 4},
	}
}

// defaults are the worker's defaults. Each job kind's user is its own runtime role (ADR-0118), and the
// TLS mode is the one that fails closed. Each pool's size is set explicitly, since the driver's default
// follows the node's processors and not the worker's limit. The key files and the password files have
// no default, since a default path assumes the environment. The scanner's vocabulary and tuning are
// the ones the application ships.
func defaults() Configuration {
	return Configuration{
		Process:    host.DefaultProcess(),
		Operations: host.DefaultOperations(),
		Section:    DefaultSection(),
		Database: Database{
			Port: 5432, SSLMode: "verify-full",
			Backfill: Role{User: "mediated_mailbox_backfill", PoolSize: 4},
			Sync:     Role{User: "mediated_mailbox_sync", PoolSize: 4},
		},
		Scanner: scan.DefaultConfig(),
	}
}

// connection returns the job kind's connection as the database section of a deployable, the server's
// values with the role's.
func (d Database) connection(r Role) dbconnectcore.Config {
	return dbconnectcore.Config{
		Host: d.Host, Port: d.Port, Name: d.Name, User: r.User, SSLMode: d.SSLMode, SSLRootCert: d.SSLRootCert, PasswordFile: r.PasswordFile,
		PoolSize: r.PoolSize,
	}
}

// validate refuses a configuration the worker cannot run with, past the process's section, which the
// host validates. Each job kind's connection passes the database section's validation, every pool,
// interval, window and bound is positive, and the credential section names its key files.
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
		case kind.role.PoolSize <= 0:
			return fmt.Errorf("database.%s.pool_size %d is not positive", kind.name, kind.role.PoolSize)
		}
		if err := dbconnectcore.Validate(c.Database.connection(kind.role)); err != nil {
			return err
		}
	}
	if err := ValidateSection(c.Section); err != nil {
		return err
	}
	return credentialcore.Validate(c.Credential)
}

// ValidateSection refuses a worker section the worker cannot run with. Every interval, window and
// bound is positive.
func ValidateSection(c Section) error {
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
	return nil
}

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
	return host.Run(ctx, args, environ, stdout, stderr, logger, level, host.Spec[Configuration]{
		Name:       "mediated-mailbox-worker",
		Defaults:   defaults(),
		Process:    func(c Configuration) host.Process { return c.Process },
		Operations: func(c Configuration) host.Operations { return c.Operations },
		Validate:   validate,
		Components: []host.Component[Configuration]{{Name: "worker", Assemble: fromConfiguration}},
	})
}

// fromConfiguration builds what the worker needs from the configuration, the scanner from its section
// under the section's revision, the keyring, a pool per job kind connecting as that kind's role and the
// worker's execution context over delta sync's pool, which holds the credential write-back and re-seal
// grants (ADR-0092, ADR-0118), and assembles the worker from them. The context's reload is the
// process's task.
func fromConfiguration(ctx context.Context, provided host.Provided[Configuration]) (host.Parts, error) {
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
	ec, err := newContext(pools, keys, scanner, c.Section, provided.Registerer, provided.Logger)
	if err != nil {
		return host.Parts{}, err
	}
	parts, err := Assemble(ctx, Needs{
		Section: c.Section, Pools: pools, Context: ec, Client: http.DefaultClient,
		Registerer: provided.Registerer, Logger: provided.Logger,
	})
	if err != nil {
		return host.Parts{}, err
	}
	parts.Tasks = append(parts.Tasks, host.Task{Name: "execution context reload", Run: ec.Run})
	return parts, nil
}

// newContext returns the worker's execution context over delta sync's pool, and so under delta sync's
// role, reloading on the reload interval (ADR-0090, ADR-0118).
func newContext(pools Pools, keys *open.Keyring, scanner scan.Scanner, s Section, reg prometheus.Registerer, logger *slog.Logger) (*assembly.Context, error) {
	return assembly.New(assembly.Needs{
		Pool: pools.Sync, Keys: keys, ThroughClient: []string{gmailProvider}, Scanner: scanner,
		Lookups:        classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
		ReloadInterval: s.ReloadInterval, Registerer: reg, Logger: logger,
	})
}

// gmailProvider is the provider an account served through the Gmail adapter names, in accounts and
// in oauth_clients.
const gmailProvider = "gmail"

// opener opens a pool from a connection, measured on a registerer.
type opener func(ctx context.Context, c dbconnectcore.Config, reg prometheus.Registerer) (*pgxpool.Pool, error)

// openPools returns each job kind's pool through open, connecting as that kind's own role with its
// password file, opening at most its pool size of connections, measured on registry labelled with the
// job kind. A pool opens no connection until its first use (ADR-0118, ADR-0125).
func openPools(ctx context.Context, d Database, open opener, registry prometheus.Registerer) (Pools, error) {
	backfillPool, err := open(ctx, d.connection(d.Backfill), labelled(registry, backfill.Kind))
	if err != nil {
		return Pools{}, err
	}
	syncPool, err := open(ctx, d.connection(d.Sync), labelled(registry, deltasync.Kind))
	if err != nil {
		return Pools{}, err
	}
	return Pools{Backfill: backfillPool, Sync: syncPool}, nil
}

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

// Needs are what the worker is assembled from.
type Needs struct {
	Section Section
	// Pools are the job kinds' pools, each connecting as its kind's runtime role (ADR-0118).
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
// readiness checks, each job kind's pool answering as its role and each job kind's first reload done.
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
	bc, dc := kinds(n.Pools, n.Context, n.Section, n.Client, n.Logger, n.Registerer, s)
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
func kinds(pools Pools, ec *assembly.Context, c Section, client *http.Client, logger *slog.Logger,
	registry prometheus.Registerer, s *schedule.Scheduler,
) (backfill.Config, deltasync.Config) {
	return backfill.Config{
		Pool: pools.Backfill, Context: ec, Client: client,
		Registry: labelled(registry, backfill.Kind), Logger: logger.With("job_kind", backfill.Kind), Jobs: s,
		Concurrency: c.Backfill.Concurrency, ReloadInterval: c.ReloadInterval,
	}, deltasync.Config{
		Pool: pools.Sync, Context: ec, Client: client,
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
