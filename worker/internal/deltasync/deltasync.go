// Package deltasync is delta sync's job kind in the worker (ADR-0117, ADR-0119). Build, which the
// worker's spec lists, builds it from the process's capabilities and the worker's configuration. It
// runs its own statements on delta sync's own connection pool, as the application user (ADR-0119,
// ADR-0143), takes the accounts it works on, their policy and their connections from the process's
// Session, and calls the provider through a caller of its own, so its adapter, limiter and provider
// series carry its job kind. Its plan lists the accounts the Session holds as workable, and the
// scheduler keeps one job per account, which ticks its account on the sync interval's phase
// (ADR-0103), each tick a unit of work. Each listing also re-seals what the connections opened with
// a key that is not the current one, the OAuth clients' secrets included, and sets the key-scan
// series, whether or not any account is listed (ADR-0092).
//
// A tick is tick.Run, and its due decision is worker/internal/core/deltasync/due.
package deltasync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/scanner"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/provider"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/due"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick"
)

// Kind is delta sync's job kind as job_runs.workload spells it, which the worker's spec names it by,
// so the worker's series and logs carry it as the job kind (ADR-0016, ADR-0117).
const Kind = tick.Workload

// callTimeout is how long each provider call delta sync makes has to answer once its lease is
// issued, a fixed timeout of the job kind's caller (ADR-0101, ADR-0119).
const callTimeout = 30 * time.Second

// pageSize is how many waiting messages one read of a tick's scanning returns. Bodies are fetched and
// scanned one at a time, so it bounds a read's rework and not its memory.
const pageSize = 100

// backoff is the delay before a failed job is asked again, doubling from its base to its cap. A tick
// asked at the end of a backoff before its interval has passed is not due, so a failed tick is tried
// again at the ticker's next tick (ADR-0103, ADR-0119).
var backoff = schedule.Backoff{Base: 5 * time.Second, Max: 5 * time.Minute}

// lateIntervals is how many sync intervals after its latest success an account's job counts as late,
// the bound of G4's "a chronically stuck sync job looks healthy" (ADR-0077, ADR-0119).
const lateIntervals = 3

// lateReloads is how many reload intervals after its latest success the kind's listing counts as
// late.
const lateReloads = 5

// DeltaSync is delta sync's job kind, what its plan's jobs run with.
type DeltaSync struct {
	pool    *pgxpool.Pool
	scanner scan.Scanner
	logger  *slog.Logger
	limit   *schedule.Limit
	session *sessions.Session
	conns   *connections.Connections
	caller  *connections.Caller
	ticks   *tick.Metrics
	keyScan *keyScan
	// processing times each body's conversion and scan in a tick (ADR-0125).
	processing *scanner.Processing
	// reloadInterval is how often the listing is asked, syncInterval how often an account ticks
	// (ADR-0018, ADR-0103), firstWindow how far back an account with no cursor is reconciled at its
	// first tick (ADR-0105), and decisions how many waiting messages one tick decides for an account
	// (ADR-0104).
	reloadInterval, syncInterval, firstWindow time.Duration
	decisions                                 int
	// seeding is the context a job's latest recorded success is read under when the listing finds
	// its account, which ends when the process starts to stop.
	seeding context.Context
}

// Build builds delta sync's job kind from the process's capabilities and the worker's configuration,
// and returns its plan. It takes the process's scanner, its own pool, which reports whether it
// answers a ping, and the process's Session and connections, and a caller of its own in the sync class, and
// registers its series on the registerer p gives it, which labels them with its job kind (ADR-0117,
// ADR-0119, ADR-0142).
func Build(ctx context.Context, p process.Capabilities, c config.Worker) (schedule.Plan, error) {
	d, err := assemble(ctx, p, c)
	if err != nil {
		return schedule.Plan{}, err
	}
	return d.plan(), nil
}

// assemble gets what the job kind is built from and returns the job kind, as Build states.
func assemble(ctx context.Context, p process.Capabilities, c config.Worker) (*DeltaSync, error) {
	sc, err := scanner.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	pool, err := database.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	if err := database.ReportReadiness(ctx, p, c); err != nil {
		return nil, err
	}
	s, err := sessions.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	conns, err := connections.Of(ctx, p, c, s)
	if err != nil {
		return nil, err
	}
	caller, err := conns.Caller(p, pool, provider.Options{Class: provider.Sync, Timeout: callTimeout})
	if err != nil {
		return nil, err
	}
	return build(p, pool, s, conns, caller, sc, c)
}

// build returns the job kind with its series registered on p's registerer.
func build(p process.Capabilities, pool *pgxpool.Pool, s *sessions.Session, conns *connections.Connections, caller *connections.Caller,
	sc scan.Scanner, c config.Worker,
) (*DeltaSync, error) {
	ticks, err := tick.NewMetrics(p.Metrics())
	if err != nil {
		return nil, err
	}
	keys, err := newKeyScan(p.Metrics())
	if err != nil {
		return nil, err
	}
	processing, err := scanner.NewProcessing(p.Metrics())
	if err != nil {
		return nil, err
	}
	return &DeltaSync{
		pool: pool, scanner: sc, logger: p.Logger(), limit: schedule.NewLimit(c.Sync.Concurrency),
		session: s, conns: conns, caller: caller, ticks: ticks, keyScan: keys, processing: processing,
		reloadInterval: time.Duration(c.ReloadInterval), syncInterval: time.Duration(c.Sync.SyncInterval),
		firstWindow: time.Duration(c.Sync.FirstWindow), decisions: c.Sync.DecisionsPerTick,
		seeding: p.Draining(),
	}, nil
}

// plan returns the job kind's plan. Its listing is the accounts the Session holds as workable, asked
// on the reload interval and after each of the Session's reloads whose accounts loaded, so it is not
// due before the Session's first load and fails while the Session's latest read of the accounts
// failed, and an account whose policy has not loaded gets no job. Each account it lists gets a job,
// whose latest success starts from what its ticks record, and the job of each it no longer lists is
// dropped with the account's series (ADR-0090, ADR-0119).
func (d *DeltaSync) plan() schedule.Plan {
	return schedule.Plan{
		Keys:    d.list,
		Recheck: d.reloadInterval,
		Backoff: backoff,
		Late:    lateReloads * d.reloadInterval,
		Job: func(account string) (schedule.Job, error) {
			job := d.job(account)
			job.LastSuccess = d.seed(d.seeding, account)
			return job, nil
		},
		Dropped: d.dropped,
		Woken:   d.session.AfterReload,
	}
}

// list returns the accounts the Session holds as workable, and once the Session answers them it
// re-seals what the connections opened with a key that is not the current one and sets the key-scan
// series from what it found, whether or not any account is listed (ADR-0092).
func (d *DeltaSync) list(ctx context.Context) ([]string, error) {
	listed, err := d.session.Workable()
	if err != nil {
		return nil, err
	}
	d.keyScan.Set(d.conns.Reseal(ctx))
	return listed, nil
}

// dropped forgets the tick series of an account the listing no longer finds, whose job's run has
// returned, so nothing sets them again.
func (d *DeltaSync) dropped(account string) {
	d.logger.Info("an account is no longer listed, so its delta sync stops", "account", account)
	d.ticks.Forget(account)
}

// job returns the account's job, asked on the sync interval's phase and holding the kind's limit. Its
// due decision reads when its last tick was asked for, which only its own runs write.
func (d *DeltaSync) job(account string) schedule.Job {
	logger := d.logger.With("account", account)
	ticked, last := false, mail.UnixMilli(0)
	return schedule.Job{
		Interval: d.syncInterval, Limit: d.limit, Backoff: backoff, Late: lateIntervals * d.syncInterval, Logger: logger,
		Run: func(ctx context.Context, ask schedule.Ask) error {
			at := mail.UnixMilli(ask.At.UnixMilli())
			if !due.Decide(ticked, last, at, d.syncInterval.Milliseconds()) {
				return schedule.ErrNotDue
			}
			// The tick is recorded at its ask's time on the ticker's phase, so a tick asked at the end
			// of a backoff leaves the ticker's next tick due.
			ticked, last = true, mail.UnixMilli(ask.Phase.UnixMilli())
			return d.tick(ctx, account, logger)
		},
	}
}

// seed returns when the account's job's latest success is counted from at its ensure, from what its
// ticks record, and zero, its ensure, when the read fails, which is logged (ADR-0119).
func (d *DeltaSync) seed(ctx context.Context, account string) time.Time {
	var recorded runrecord.RecordedSuccessRow
	err := tx.Run(ctx, d.pool, account, func(t pgx.Tx) error {
		var rErr error
		recorded, rErr = runrecord.New(t).RecordedSuccess(ctx, Kind)
		return rErr
	})
	if err != nil {
		d.logger.Warn("reading the account's latest recorded success failed, so its job counts from now", "account", account, "error", err)
		return time.Time{}
	}
	return at(due.Seed(millis(recorded.LatestSuccess), millis(recorded.EarliestStart)))
}

// millis returns t in milliseconds since the epoch, and zero when it is null.
func millis(t pgtype.Timestamptz) mail.UnixMilli {
	if !t.Valid {
		return 0
	}
	return mail.UnixMilli(t.Time.UnixMilli())
}

// at returns the time ms names, and the zero time for zero.
func at(ms mail.UnixMilli) time.Time {
	if ms == 0 {
		return time.Time{}
	}
	return time.UnixMilli(int64(ms))
}

// tick runs one tick over the account as one unit of work, which begins on the account's connection
// through the kind's caller and takes the account's policy in force at its entry and keeps it
// (ADR-0041, ADR-0090), then ends, writing a rotated credential back and recording the latest
// authentication attempt, a tick that failed included (ADR-0089, ADR-0097). It logs counts and
// identifiers, never a body (ADR-0009).
func (d *DeltaSync) tick(ctx context.Context, account string, logger *slog.Logger) error {
	work, err := d.conns.Begin(account, d.caller, time.Time{})
	if err != nil {
		return err
	}
	result, err := tick.Run(ctx, d.tickDeps(tick.Through(work.Mailbox()), d.session.Policy(account)), account)
	d.ticks.Count(account, result)
	c := result.Progress.Counters
	if err != nil {
		logger.Error("the account's tick failed", "run", result.Run, "recovery", result.Recovery, "error", err)
	} else {
		logger.Info("the account's tick ended", "run", result.Run, "recovery", result.Recovery, "gap", result.Gap,
			"added", c.Added, "modified", c.Modified, "removed", c.Removed, "decided", c.Decided, "scanned", c.Scanned, "skipped", c.Skipped)
	}
	writtenBack, recorded := work.End(ctx)
	return errors.Join(err, writtenBack, recorded)
}

// tickDeps returns what a tick runs with over provider under policy. The gate decides under the
// thresholds backfill decides under, which are one value in the worker, and the scanner is the one
// backfill scans with, the process's one scanner, so neither job kind reopens the other's work
// (ADR-0120, ADR-0121, ADR-0104).
func (d *DeltaSync) tickDeps(provider tick.Provider, policy classify.Policy) tick.Deps {
	return tick.Deps{
		Store:       tick.NewPostgres(d.pool),
		Provider:    provider,
		Policy:      policy,
		Gate:        scangate.DefaultConfig(),
		Scanner:     d.scanner,
		Processing:  d.processing,
		FirstWindow: d.firstWindow,
		Decisions:   d.decisions,
		PageSize:    pageSize,
		RunID:       runID,
		Now:         time.Now,
	}
}

// runID returns a new run's identifier, sixteen random hexadecimal digits.
func runID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
	return hex.EncodeToString(b[:])
}
