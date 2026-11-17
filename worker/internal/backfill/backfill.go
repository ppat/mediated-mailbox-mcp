// Package backfill is backfill's job kind in the worker (ADR-0117, ADR-0119). Build, which the
// worker's spec lists, builds it from the process's capabilities and the worker's configuration. It
// runs its own statements on backfill's own connection pool, as the application user (ADR-0119,
// ADR-0143), takes the accounts it works on, their policy and their connections from the process's
// Session, and calls the provider through a caller of its own, so its adapter, limiter and provider
// series carry its job kind. Its plan lists the accounts the Session holds as workable, and the
// scheduler keeps one job per account. Each job runs the run-start step once in the process, then
// whichever pass has not ended, a page at a time, each page a unit of work, and is otherwise a
// reconciler that finds nothing to do (ADR-0133, ADR-0120, ADR-0121).
//
// Its two passes are pass1 and pass2, and its due decision is worker/internal/core/backfill/due.
package backfill

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/connections"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/scanner"
	"github.com/ppat/mediated-mailbox-mcp/process/schedule"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/provider"
	providermail "github.com/ppat/mediated-mailbox-mcp/provider/mail"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

// Kind is backfill's job kind as job_runs.workload spells it, which the worker's spec names it by, so
// the worker's series and logs carry it as the job kind (ADR-0016, ADR-0117).
const Kind = pass1.Workload

// callTimeout is how long each provider call backfill makes has to answer once its lease is issued, a
// fixed timeout of the job kind's caller (ADR-0101, ADR-0119).
const callTimeout = 30 * time.Second

// pageSize is how many messages waiting for a scan one page of the second pass reads. Bodies are
// fetched and scanned one at a time, so it bounds a page's rework and not its memory.
const pageSize = 100

// mostPerCall is the most identifiers one call of the first pass fetching stale subjects again names,
// whatever the account's mailbox would fit, so a call's rework after a crash stays small. Gmail's
// profile fits three (ADR-0023, ADR-0120).
const mostPerCall = 100

// backoff is the delay before a failed job is asked again, doubling from its base to its cap
// (ADR-0119).
var backoff = schedule.Backoff{Base: 5 * time.Second, Max: 5 * time.Minute}

// late is how long after an account job's latest success it counts as late. A success is a page made
// durable or a run with nothing to do, so the bound is a page held up by the provider's throttling,
// not a pass's length (ADR-0077, ADR-0119).
const late = time.Hour

// lateReloads is how many reload intervals after its latest success the kind's listing counts as
// late.
const lateReloads = 5

// Backfill is backfill's job kind, what its plan's jobs run with.
type Backfill struct {
	pool     *pgxpool.Pool
	scanner  scan.Scanner
	logger   *slog.Logger
	limit    *schedule.Limit
	interval time.Duration
	session  *sessions.Session
	conns    *connections.Connections
	caller   *connections.Caller
	store    pass1.Store
	second   pass2.Store
	first    *pass1.Metrics
	backlog  *pass2.Metrics
	// processing times each body's conversion and scan in the second pass (ADR-0125).
	processing *scanner.Processing
	// seeding is the context a job's latest recorded success is read under when the listing finds
	// its account, which ends when the process starts to stop.
	seeding context.Context
}

// Build builds backfill's job kind from the process's capabilities and the worker's configuration,
// and returns its plan. It takes the process's scanner, its own pool, which reports whether it
// answers a ping, and the process's Session and connections, and a caller of its own in the batch class, and
// registers its series on the registerer p gives it, which labels them with its job kind (ADR-0117,
// ADR-0119, ADR-0142).
func Build(ctx context.Context, p process.Capabilities, c config.Worker) (schedule.Plan, error) {
	sc, err := scanner.Get(ctx, p, c)
	if err != nil {
		return schedule.Plan{}, err
	}
	pool, err := database.Get(ctx, p, c)
	if err != nil {
		return schedule.Plan{}, err
	}
	if err := database.ReportReadiness(ctx, p, c); err != nil {
		return schedule.Plan{}, err
	}
	s, err := sessions.Get(ctx, p, c)
	if err != nil {
		return schedule.Plan{}, err
	}
	conns, err := connections.Of(ctx, p, c, s)
	if err != nil {
		return schedule.Plan{}, err
	}
	caller, err := conns.Caller(p, pool, provider.Options{Class: provider.Batch, Timeout: callTimeout})
	if err != nil {
		return schedule.Plan{}, err
	}
	b, err := build(p, pool, s, conns, caller, sc, c)
	if err != nil {
		return schedule.Plan{}, err
	}
	return b.plan(), nil
}

// build returns the job kind with its series registered on p's registerer.
func build(p process.Capabilities, pool *pgxpool.Pool, s *sessions.Session, conns *connections.Connections, caller *connections.Caller,
	sc scan.Scanner, c config.Worker,
) (*Backfill, error) {
	messages, err := series.NewMessages(p.Metrics())
	if err != nil {
		return nil, err
	}
	processing, err := scanner.NewProcessing(p.Metrics())
	if err != nil {
		return nil, err
	}
	first, err := pass1.NewMetrics(p.Metrics(), messages)
	if err != nil {
		return nil, err
	}
	backlog, err := pass2.NewMetrics(p.Metrics(), messages)
	if err != nil {
		return nil, err
	}
	return &Backfill{
		pool: pool, scanner: sc, logger: p.Logger(),
		limit: schedule.NewLimit(c.Backfill.Concurrency), interval: time.Duration(c.ReloadInterval),
		session: s, conns: conns, caller: caller,
		store: pass1.NewPostgres(pool), second: pass2.NewPostgres(pool),
		first: first, backlog: backlog, processing: processing, seeding: p.Draining(),
	}, nil
}

// plan returns the job kind's plan. Its listing is the accounts the Session holds as workable, asked
// on the reload interval and after each of the Session's reloads whose accounts loaded, so it is not
// due before the Session's first load and fails while the Session's latest read of the accounts
// failed, and an account whose policy has not loaded gets no job. Each account it lists gets a job,
// whose latest success starts from what its runs record, and the job of each it no longer lists is
// dropped with the account's series (ADR-0090, ADR-0119).
func (b *Backfill) plan() schedule.Plan {
	return schedule.Plan{
		Keys:    func(context.Context) ([]string, error) { return b.session.Workable() },
		Recheck: b.interval,
		Backoff: backoff,
		Late:    lateReloads * b.interval,
		Job: func(account string) (schedule.Job, error) {
			job := b.job(account)
			job.LastSuccess = b.seed(b.seeding, account)
			return job, nil
		},
		Dropped: b.dropped,
		Woken:   b.session.AfterReload,
	}
}

// dropped forgets the series of an account the listing no longer finds, whose job's run has returned,
// so nothing sets them again.
func (b *Backfill) dropped(account string) {
	b.logger.Info("an account is no longer listed, so its backfill stops", "account", account)
	b.first.Forget(account)
	b.backlog.Forget(account)
}

// job returns the account's job. Its runs are asked on the reload interval and hold the kind's limit.
// It makes the run-start step at its first run that reaches it, once for the job's life, so an account
// listed again gets it again (ADR-0121).
func (b *Backfill) job(account string) schedule.Job {
	logger := b.logger.With("account", account)
	started := false
	return schedule.Job{
		Interval: b.interval, Limit: b.limit, Backoff: backoff, Late: late, Logger: logger,
		Run: func(ctx context.Context, ask schedule.Ask) error {
			return b.run(ctx, ask, account, &started, logger)
		},
	}
}

// policyFor returns the account's policy in the policy the Session has in force, which each page
// takes at its entry (ADR-0041, ADR-0119).
func (b *Backfill) policyFor(account string) func() classify.Policy {
	return func() classify.Policy { return b.session.Policy(account) }
}

// run asks the account's due decision and does the work it names. The run-start step returns the
// verdicts another scanner made and the gate skips the gate no longer decides as the same skip to
// pending, and masks again from the store the subjects stored unmasked, then the first pass runs, and
// the second once the first has ended (ADR-0120, ADR-0121, ADR-0133). Each page of either pass, and
// each call of the first fetching stale subjects again, is a unit of work, which begins on the
// account's connection through the kind's caller, takes the policy in force, and ends after it,
// writing a rotated credential back and recording the latest attempt (ADR-0082, ADR-0090,
// ADR-0097). A page made durable is the job's success.
func (b *Backfill) run(ctx context.Context, ask schedule.Ask, account string, started *bool, logger *slog.Logger) error {
	state, err := b.state(ctx, account)
	if err != nil {
		return err
	}
	work := due.Decide(state, *started)
	if !work.Any() {
		return nil
	}
	u := &unit{conns: b.conns, caller: b.caller, account: account}
	first := pass1.Deps{
		Store: b.store, Fetch: u.fetch, Metadata: u.metadata, PerCall: u.perCall,
		Policy: b.policyFor(account), Scanner: b.scanner, RunID: runID, Now: time.Now,
	}
	second := secondDeps(b.second, u.body, b.policyFor(account), b.scanner)
	second.Processing = b.processing
	return backfillAccount(ctx, first, second, account, b.first, b.backlog, u, ask, logger, work.Start, started)
}

// backfillAccount does an account's work with each pass's dependencies. When start is set, it first
// makes the run-start step, so the verdicts another scanner made and the gate skips the gate no longer
// decides as the same skip are denied whatever the first pass does, and the subjects stored unmasked
// are masked again from the store, and records that the step was made in started. Then the first pass
// runs, and the second once the first has ended, each page a unit of work over u (ADR-0120, ADR-0121,
// ADR-0133).
func backfillAccount(ctx context.Context, first pass1.Deps, second pass2.Deps, account string, m1 *pass1.Metrics, m2 *pass2.Metrics, u units,
	ask schedule.Ask, logger *slog.Logger, start bool, started *bool,
) error {
	if start {
		if err := reopen(ctx, second, account, logger); err != nil {
			return err
		}
		*started = true
	}
	if err := passAccount(ctx, first, account, m1, u, ask, logger); err != nil {
		return err
	}
	return secondPassAccount(ctx, second, account, m2, u, ask, logger)
}

// state reads what recorded state says of the account's backfill, its completion flags and the mark
// that starts its second pass over, the due decision's two one-row reads (ADR-0119).
func (b *Backfill) state(ctx context.Context, account string) (due.State, error) {
	var s due.State
	err := tx.Run(ctx, b.pool, account, func(t pgx.Tx) error {
		progress, err := accountstate.New(t).AccountProgress(ctx)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		restart, err := completion.New(t).SecondRestart(ctx)
		if err != nil {
			return err
		}
		s = due.State{FirstEnded: progress.BackfillPass1Complete, SecondEnded: progress.BackfillPass2Complete, Restart: restart}
		return nil
	})
	if err != nil {
		return due.State{}, fmt.Errorf("reading the account's backfill state: %w", err)
	}
	return s, nil
}

// seed returns when the account's job's latest success is counted from at its ensure, from what the
// account's state and its runs record, and zero, its ensure, when the read fails, which is logged
// (ADR-0119).
func (b *Backfill) seed(ctx context.Context, account string) time.Time {
	state, err := b.state(ctx, account)
	var recorded runrecord.RecordedSuccessRow
	if err == nil {
		err = tx.Run(ctx, b.pool, account, func(t pgx.Tx) error {
			var rErr error
			recorded, rErr = runrecord.New(t).RecordedSuccess(ctx, Kind)
			return rErr
		})
	}
	if err != nil {
		b.logger.Warn("reading the account's latest recorded success failed, so its job counts from now", "account", account, "error", err)
		return time.Time{}
	}
	return at(due.Seed(state, millis(recorded.LatestSuccess), millis(recorded.EarliestStart)))
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

// errNotListed stops a run at a unit boundary when the account is no longer served. The scheduler
// counts the run as failed, so it is logged and the job backs off, until the listing that dropped the
// account removes the job.
var errNotListed = errors.New("the account is no longer served, so its run stops")

// units begins and ends the account's units of work around each page of a pass.
type units interface {
	open() error
	end(ctx context.Context) error
}

// unit is the account's unit of work in progress, begun at each unit's entry through the kind's
// caller.
type unit struct {
	conns   *connections.Connections
	caller  *connections.Caller
	account string
	current *connections.Work
}

// open begins the account's next unit of work.
func (u *unit) open() error {
	w, err := u.conns.Begin(u.account, u.caller, time.Time{})
	if errors.Is(err, connections.ErrNotConnected) {
		return errNotListed
	}
	if err != nil {
		return err
	}
	u.current = w
	return nil
}

// end ends the unit of work, writing a rotated credential back and recording the latest attempt, and
// returns their failures joined.
func (u *unit) end(ctx context.Context) error {
	if u.current == nil {
		return nil
	}
	writeBack, record := u.current.End(ctx)
	u.current = nil
	return errors.Join(writeBack, record)
}

// fetch is the first pass's enumeration through the unit's mailbox, leased in the batch class
// (ADR-0025).
func (u *unit) fetch(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
	return u.current.Mailbox().EnumerateAll(ctx, token)
}

// metadata is the first pass's fetch of stale subjects again by identifier through the unit's mailbox
// (ADR-0025, ADR-0120).
func (u *unit) metadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	return u.current.Mailbox().GetMessageMetadata(ctx, ids)
}

// perCall is how many identifiers one call fetching stale subjects again names through the unit's
// mailbox, the most whose cost fits one second's worth at the hard cap, and never more than
// mostPerCall (ADR-0023).
func (u *unit) perCall() int {
	return u.current.Mailbox().MetadataCallSize(mostPerCall)
}

// body is the second pass's body fetch through the unit's mailbox, a second call naming a part held
// apart (ADR-0025, ADR-0131).
func (u *unit) body(ctx context.Context, id string) (mail.MessageBody, error) {
	return providermail.Body(ctx, u.current.Mailbox(), id)
}

// secondDeps returns what an account's second pass runs with. The gate decides under the thresholds
// delta sync decides under, which are one value in the worker, and the scanner is the one delta sync
// scans with, built from the worker's one scanner section, so neither job kind reopens the other's
// work (ADR-0120, ADR-0121, ADR-0104).
func secondDeps(store pass2.Store, body pass2.Body, policy func() classify.Policy, scanner scan.Scanner) pass2.Deps {
	return pass2.Deps{
		Store: store, Body: body, Policy: policy, Gate: scangate.DefaultConfig(), Scanner: scanner,
		PageSize: pageSize, RunID: runID, Now: time.Now,
	}
}

// reopen is the run-start step. It returns to pending every verdict of the account made under another
// scanner than the one the run scans with, masks again from the store every subject stored unmasked
// under another scanner, and returns to pending every stored gate skip the gate under the run's
// thresholds no longer decides as the same skip, before the first pass, so each verdict and skip is
// denied from the start of the run that sees it whatever the first pass does, and reopens the second
// pass when it did any of these or the first pass is due again (ADR-0120, ADR-0121).
func reopen(ctx context.Context, deps pass2.Deps, account string, logger *slog.Logger) error {
	r, err := pass2.Reopen(ctx, deps, account)
	if err != nil {
		return err
	}
	if r.Refusals > 0 {
		logger.Info("bodies refused under another size bound returned to pending", "messages", r.Refusals)
	}
	if r.Verdicts > 0 {
		logger.Info("verdicts made under another scanner returned to pending", "messages", r.Verdicts)
	}
	if r.Subjects > 0 {
		logger.Info("subjects stored unmasked under another scanner masked again from the store", "messages", r.Subjects)
	}
	if r.Skips > 0 {
		logger.Info("gate skips the gate no longer decides as the same skip returned to pending", "messages", r.Skips)
	}
	return nil
}

// passAccount runs the first pass over one account a page at a time. Each page begins the account's
// unit of work first and ends it after, then counts the page's unclassified senders, and a page made
// durable is the job's success. It stops at a page boundary once ctx ends.
func passAccount(ctx context.Context, deps pass1.Deps, account string, metrics *pass1.Metrics, u units, ask schedule.Ask, logger *slog.Logger) error {
	p, err := pass1.Open(ctx, deps, account)
	if err != nil {
		return err
	}
	if p.Run() == "" {
		logger.Debug("the first pass has ended for the account, so it is skipped")
		return nil
	}
	logger.Info("the first pass runs", "run", p.Run(), "page", p.Progress().Checkpoint.Page)
	var handOvers []error
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(handOvers, err)...)
		}
		if err := u.open(); err != nil {
			return errors.Join(append(handOvers, err)...)
		}
		step, err := p.Next(ctx)
		if hoErr := u.end(ctx); hoErr != nil {
			handOvers = append(handOvers, hoErr)
		}
		metrics.Count(account, step)
		if err != nil {
			logger.Error("the first pass failed", "run", p.Run(), "error", err)
			return errors.Join(append(handOvers, err)...)
		}
		ask.Progressed()
		at := p.Progress()
		logger.Info("step made durable", "run", p.Run(), "page", at.Checkpoint.Page, "messages", at.Counters.Messages,
			"refetched", at.Counters.Refetched, "stale", at.Checkpoint.Stale)
		if step.Done {
			logger.Info("the first pass ended", "run", p.Run(), "pages", at.Counters.Pages, "messages", at.Counters.Messages,
				"refetched", at.Counters.Refetched)
			return errors.Join(handOvers...)
		}
	}
}

// secondPassAccount runs the second pass over one account a page at a time. Each page opens the
// account's unit of work first and ends it after, then sets the account's scan backlog, which is
// removed once the pass ends, and a page made durable is the job's success. It logs counts and identifiers, never a body (ADR-0009). It
// stops at a page boundary once ctx ends.
func secondPassAccount(ctx context.Context, deps pass2.Deps, account string, metrics *pass2.Metrics, u units, ask schedule.Ask, logger *slog.Logger) error {
	p, err := pass2.Open(ctx, deps, account)
	if err != nil {
		return err
	}
	if p.Run() == "" {
		logger.Debug("the second pass has ended for the account, or waits for the first, so it is skipped")
		return nil
	}
	logger.Info("the second pass runs", "run", p.Run(), "page", p.Progress().Checkpoint.Page)
	var handOvers []error
	for {
		if err := ctx.Err(); err != nil {
			return errors.Join(append(handOvers, err)...)
		}
		if err := u.open(); err != nil {
			return errors.Join(append(handOvers, err)...)
		}
		step, err := p.Next(ctx)
		if hoErr := u.end(ctx); hoErr != nil {
			handOvers = append(handOvers, hoErr)
		}
		metrics.Count(account, step)
		if pending, bErr := deps.Store.Backlog(ctx, account); bErr == nil {
			metrics.Backlog(account, pending)
		} else {
			logger.Warn("reading the scan backlog failed", "error", bErr)
		}
		if err != nil {
			logger.Error("the second pass failed", "run", p.Run(), "error", err)
			return errors.Join(append(handOvers, err)...)
		}
		ask.Progressed()
		at := p.Progress()
		if step.Done {
			metrics.Forget(account)
			logger.Info("the second pass ended", "run", p.Run(), "pages", at.Counters.Pages,
				"decided", at.Counters.Decided, "scanned", at.Counters.Scanned, "skipped", at.Counters.Skipped, "pending", at.Counters.Pending)
			return errors.Join(handOvers...)
		}
		logger.Info("page made durable", "run", p.Run(), "pass", 2, "page", at.Checkpoint.Page, "scanned", at.Counters.Scanned)
	}
}

// runID returns a new run's identifier, sixteen random hexadecimal digits.
func runID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
	return hex.EncodeToString(b[:])
}
