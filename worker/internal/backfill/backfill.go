// Package backfill is backfill's job kind in the worker (ADR-0117, ADR-0119). It holds backfill's own
// account loader, policy loader and account sessions, on backfill's own connection pool under its own
// runtime role (ADR-0118), and adds and drops one job per account its reload lists. Each job runs the
// run-start step once in the process, then whichever pass has not ended, a page at a time, and is
// otherwise a reconciler that finds nothing to do (ADR-0017, ADR-0120, ADR-0121).
//
// The worker's composition root builds it with New and ensures its reload job on the scheduler. Its
// two passes are pass1 and pass2, and its due decision is worker/internal/core/backfill/due.
package backfill

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/policy"
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	ratecore "github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass1"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/pass2"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

// Kind is backfill's job kind as job_runs.workload spells it, which the worker's series and logs carry
// as the job kind (ADR-0016, ADR-0117).
const Kind = pass1.Workload

// gmailProvider is the provider an account served through the Gmail adapter names, in accounts and
// in oauth_clients.
const gmailProvider = "gmail"

// pageSize is how many messages waiting for a scan one page of the second pass reads. Bodies are
// fetched and scanned one at a time, so it bounds a page's rework and not its memory.
const pageSize = 100

// mostPerCall is the most identifiers one call of the first pass fetching stale subjects again names,
// whatever the provider's profile would fit, so a call's rework after a crash stays small. Gmail's
// profile fits three (ADR-0023, ADR-0120).
const mostPerCall = 100

// backoff is the delay before a failed job is asked again, doubling from its base to its cap
// (ADR-0119).
var backoff = schedule.Backoff{Base: 5 * time.Second, Max: 5 * time.Minute}

// late is how long after an account job's latest success it counts as late. A success is a page made
// durable or a run with nothing to do, so the bound is a page held up by the provider's throttling,
// not a pass's length (ADR-0077, ADR-0119).
const late = time.Hour

// lateReloads is how many reload intervals after its latest success the kind's reload counts as late.
const lateReloads = 5

// Jobs is where the kind's reload adds and drops its account jobs, the worker's scheduler.
type Jobs interface {
	Ensure(key schedule.Key, job schedule.Job) error
	Remove(key schedule.Key)
}

// Config is what backfill's job kind is built from, and nothing more.
type Config struct {
	// Pool is backfill's own pool, which connects as backfill's runtime role (ADR-0118).
	Pool *pgxpool.Pool
	// Keys open the stored credentials and client secrets.
	Keys *open.Keyring
	// Scanner masks subjects and scans bodies, built once from the worker's scanner section.
	Scanner scan.Scanner
	// Client is the HTTP client the Gmail adapter and its token source call through.
	Client *http.Client
	// Registry is the worker's registry, labelled with backfill's job kind.
	Registry prometheus.Registerer
	// Logger carries backfill's job kind.
	Logger *slog.Logger
	// Jobs is the scheduler the reload adds and drops account jobs on.
	Jobs Jobs
	// Concurrency bounds the account jobs running at once, and so the bodies held at once.
	Concurrency int
	// ReloadInterval is how often the reload runs and how often each account job is asked.
	ReloadInterval time.Duration
}

// Backfill is backfill's job kind.
type Backfill struct {
	pool     *pgxpool.Pool
	scanner  scan.Scanner
	registry prometheus.Registerer
	logger   *slog.Logger
	jobs     Jobs
	limit    *schedule.Limit
	interval time.Duration
	loader   *accountload.Loader
	sessions *session.Holder
	// policies is built at the first reload that lists an account, since a policy loader reads at
	// least one, and never replaced. Every account job is ensured after it is built.
	policies *policyload.Loader
	store    pass1.Store
	second   pass2.Store
	first    *pass1.Metrics
	backlog  *pass2.Metrics
	leases   *lease.Metrics
	// processing times each body's conversion and scan in the second pass (ADR-0125).
	processing *series.Processing
	lookups    classify.Lookups
	// port builds an account's Gmail adapter in a kind New built, over the series New registered, so
	// a test reaches the adapter series the kind serves.
	port func(account string, tokens gmail.Tokens) (mail.Port[context.Context], error)
	// served are the accounts the reload ensured a job for.
	served []string
}

// New returns backfill's job kind with its series registered on c.Registry. Its account sessions
// build each Gmail source and adapter over c.Client.
func New(c Config) (*Backfill, error) {
	connect, port, err := gmailConnector(c.Registry, c.Client)
	if err != nil {
		return nil, err
	}
	b, err := build(c, map[string]session.Connector{gmailProvider: connect})
	if err != nil {
		return nil, err
	}
	b.port = port
	return b, nil
}

// build returns the job kind with the connectors its account sessions build sources and ports with.
func build(c Config, connectors map[string]session.Connector) (*Backfill, error) {
	leases, err := lease.NewMetrics(c.Registry)
	if err != nil {
		return nil, err
	}
	messages, err := series.NewMessages(c.Registry)
	if err != nil {
		return nil, err
	}
	processing, err := series.NewProcessing(c.Registry)
	if err != nil {
		return nil, err
	}
	first, err := pass1.NewMetrics(c.Registry, messages)
	if err != nil {
		return nil, err
	}
	backlog, err := pass2.NewMetrics(c.Registry, messages)
	if err != nil {
		return nil, err
	}
	loader := accountload.New(c.Pool, c.Keys, c.Logger, []string{gmailProvider})
	return &Backfill{
		pool: c.Pool, scanner: c.Scanner, registry: c.Registry, logger: c.Logger, jobs: c.Jobs,
		limit: schedule.NewLimit(c.Concurrency), interval: c.ReloadInterval, loader: loader,
		sessions: session.New(session.Config{Loader: loader, DB: c.Pool, Logger: c.Logger, Connectors: connectors, Hold: true}),
		store:    pass1.NewPostgres(c.Pool), second: pass2.NewPostgres(c.Pool),
		first: first, backlog: backlog, leases: leases, processing: processing,
		lookups: classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
	}, nil
}

// Reload returns the job kind's reload, the job keyed by the kind alone that reloads the account
// snapshot and the policy and adds and drops the account jobs (ADR-0090, ADR-0119).
func (b *Backfill) Reload() (schedule.Key, schedule.Job) {
	return schedule.Key{Kind: Kind}, schedule.Job{
		Run: b.reload, Interval: b.interval, Backoff: backoff, Late: lateReloads * b.interval, Logger: b.logger,
	}
}

// reload reloads the account snapshot and the policy of every account it lists, then ensures a job
// for each account it lists that the kind can serve and removes the job of each it no longer serves. A
// snapshot whose read fails keeps the previous one and changes no job. A policy whose reload fails
// adds no job, because an account whose rules were never read would be decided as if every sender were
// restricted, and still drops the jobs of the accounts no longer listed (ADR-0090, ADR-0041).
func (b *Backfill) reload(ctx context.Context, _ schedule.Ask) error {
	if err := b.loader.Load(ctx); err != nil {
		return fmt.Errorf("loading the accounts: %w", err)
	}
	snapshot := b.loader.Snapshot()
	listed := b.sessions.Accounts(snapshot)
	policyErr := b.loadPolicy(ctx, ids(snapshot))
	if policyErr != nil {
		b.logger.Error("the policy was not reloaded, so no account newly listed is backfilled until it is", "error", policyErr)
	}
	var kept []string
	for _, account := range b.served {
		if slices.Contains(listed, account) {
			kept = append(kept, account)
			continue
		}
		b.logger.Info("an account is no longer listed, so its backfill stops", "account", account)
		b.jobs.Remove(schedule.Key{Kind: Kind, ID: account})
		// The job's run has returned, so nothing sets the account's series again.
		b.first.Forget(account)
		b.backlog.Forget(account)
	}
	b.served = kept
	if policyErr != nil {
		return policyErr
	}
	for _, account := range listed {
		if slices.Contains(b.served, account) {
			continue
		}
		job := b.job(account)
		job.LastSuccess = b.seed(ctx, account)
		if err := b.jobs.Ensure(schedule.Key{Kind: Kind, ID: account}, job); err != nil {
			return err
		}
		b.served = append(b.served, account)
	}
	return nil
}

// loadPolicy reloads the policy of accounts through the kind's own policy loader, building it at the
// first reload that lists an account (ADR-0041, ADR-0119). A reload that fails keeps the snapshot it
// had.
func (b *Backfill) loadPolicy(ctx context.Context, accounts []string) error {
	if len(accounts) == 0 {
		return nil
	}
	if b.policies == nil {
		built, err := policyload.New(b.pool, accounts, b.registry)
		if err != nil {
			return err
		}
		b.policies = built
	} else if err := b.policies.SetAccounts(accounts); err != nil {
		return err
	}
	return b.policies.Reload(ctx)
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

// policyFor returns the account's policy in the active policy snapshot, which each page takes at its
// entry (ADR-0041, ADR-0119).
func (b *Backfill) policyFor(account string) func() policy.Composed {
	return func() policy.Composed { return b.policies.Snapshot().For(account) }
}

// run asks the account's due decision and does the work it names. The run-start step returns the
// verdicts another scanner made and the gate skips the gate no longer decides as the same skip to
// pending, and masks again from the store the subjects stored unmasked, then the first pass runs, and
// the second once the first has ended (ADR-0120, ADR-0121, ADR-0017). Each page of either pass, and
// each call of the first fetching stale subjects again, is a unit of work, which opens the account's
// session over the active account snapshot, takes the active policy and ends the session's unit after
// it, handing its token over and recording its latest attempt (ADR-0082, ADR-0090, ADR-0097). A page
// made durable is the job's success.
func (b *Backfill) run(ctx context.Context, ask schedule.Ask, account string, started *bool, logger *slog.Logger) error {
	state, err := b.state(ctx, account)
	if err != nil {
		return err
	}
	work := due.Decide(state, *started)
	if !work.Any() {
		return nil
	}
	limiter, err := newLimiter(ctx, b.pool, []string{account}, b.leases)
	if err != nil {
		return err
	}
	u := &unit{sessions: b.sessions, loader: b.loader, account: account}
	first := pass1.Deps{
		Store: b.store, Fetch: u.fetch(limiter), Metadata: u.metadata(limiter), PerCall: perCall(gmail.Profile{}),
		Policy: b.policyFor(account), Scanner: b.scanner, Lookups: b.lookups, RunID: runID, Now: time.Now,
	}
	second := secondDeps(b.second, u.body(limiter), b.policyFor(account), b.lookups, b.scanner)
	second.Processing = b.processing
	return backfillAccount(ctx, first, second, account, b.first, b.backlog, u, ask, logger, work.Start, started)
}

// backfillAccount does an account's work with each pass's dependencies. When start is set, it first
// makes the run-start step, so the verdicts another scanner made and the gate skips the gate no longer
// decides as the same skip are denied whatever the first pass does, and the subjects stored unmasked
// are masked again from the store, and records that the step was made in started. Then the first pass
// runs, and the second once the first has ended, each page a unit of work over u (ADR-0120, ADR-0121,
// ADR-0017).
func backfillAccount(ctx context.Context, first pass1.Deps, second pass2.Deps, account string, m1 *pass1.Metrics, m2 *pass2.Metrics, u *unit,
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
		progress, err := accountstate.New(t).AccountProgress(ctx, account)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
		restart, err := completion.New(t).SecondRestart(ctx, account)
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
			recorded, rErr = runrecord.New(t).RecordedSuccess(ctx, runrecord.RecordedSuccessParams{AccountID: account, Workload: Kind})
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
// counts the run as failed, so it is logged and the job backs off, until the reload that dropped the
// account removes the job.
var errNotListed = errors.New("the account is no longer served, so its run stops")

// unit is the account's session for the unit of work in progress, opened at each unit's entry.
type unit struct {
	sessions *session.Holder
	loader   *accountload.Loader
	account  string
	current  *session.Session
}

// open opens the account's session for the next unit of work over the active account snapshot.
func (u *unit) open() error {
	s, err := u.sessions.Open(u.loader.Snapshot(), u.account)
	if errors.Is(err, session.ErrNotConnected) {
		return errNotListed
	}
	if err != nil {
		return err
	}
	u.current = s
	return nil
}

// end ends the unit of work, handing the account's token over and recording its latest attempt, and
// returns their failures joined.
func (u *unit) end(ctx context.Context) error {
	if u.current == nil {
		return nil
	}
	handOver, record := u.current.End(ctx)
	u.current = nil
	return errors.Join(handOver, record)
}

// fetch returns the first pass's enumeration over the unit's session, under a lease (ADR-0025). A
// refused credential is read again and the call made once more as session.Call says, a new lease
// included.
func (u *unit) fetch(limiter *lease.Limiter) pass1.Fetch {
	return func(ctx context.Context, token mail.PageToken) (mail.Page[mail.MessageMetadata], error) {
		return session.Call(ctx, u.current, func(ctx context.Context, port mail.Port[context.Context]) (mail.Page[mail.MessageMetadata], error) {
			return pass1.Leased(limiter, port, u.account)(ctx, token)
		})
	}
}

// metadata returns the first pass's fetch of stale subjects again by identifier over the unit's
// session, under a lease (ADR-0025, ADR-0120), reading a refused credential again as fetch does.
func (u *unit) metadata(limiter *lease.Limiter) pass1.Metadata {
	return func(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
		return session.Call(ctx, u.current, func(ctx context.Context, port mail.Port[context.Context]) ([]mail.MessageMetadata, error) {
			return pass1.LeasedMetadata(limiter, port, u.account)(ctx, ids)
		})
	}
}

// body returns the second pass's body fetch over the unit's session, under a lease (ADR-0025),
// reading a refused credential again as fetch does.
func (u *unit) body(limiter *lease.Limiter) pass2.Body {
	return func(ctx context.Context, id string) (mail.MessageBody, error) {
		return session.Call(ctx, u.current, func(ctx context.Context, port mail.Port[context.Context]) (mail.MessageBody, error) {
			return leasedBody(limiter, port, u.account)(ctx, id)
		})
	}
}

// gmailSource builds the Gmail token source of an account from its credentials, the OAuth client the
// account connects through and its refresh token (ADR-0106).
func gmailSource(client *http.Client) func(session.Credentials) *gmail.TokenSource {
	return func(c session.Credentials) *gmail.TokenSource {
		return gmail.NewTokenSource(client, gmail.NewCredentials(c.ClientID, c.ClientSecret, c.Credential))
	}
}

// gmailConnector returns the connector that builds a Gmail token source for an account and a Gmail
// adapter over it, both calling through client, counting its requests on series registry serves, which
// every adapter it builds shares. It also returns the builder of the adapter over an account's tokens.
func gmailConnector(registry prometheus.Registerer, client *http.Client) (session.Connector, func(string, gmail.Tokens) (mail.Port[context.Context], error), error) {
	metrics, err := gmail.NewMetrics(registry)
	if err != nil {
		return session.Connector{}, nil, err
	}
	port := func(account string, tokens gmail.Tokens) (mail.Port[context.Context], error) {
		return gmail.New(gmail.Config{Account: account, Client: client, Tokens: tokens, Metrics: metrics})
	}
	return session.Connect(gmailSource(client), func(account string, source *gmail.TokenSource) (mail.Port[context.Context], error) {
		return port(account, source)
	}), port, nil
}

// perCall is how many identifiers one call fetching stale subjects again names under profile, the most
// whose cost fits one second's worth at the hard cap, and never more than mostPerCall (ADR-0023).
func perCall(profile mail.RateLimitProfile[context.Context]) int {
	return mail.CallSize(func(n int) float64 {
		return profile.Cost(mail.ProviderOp{Operation: mail.OpGetMessageMetadata, Messages: n}).Weight
	}, profile.BudgetPerSecond(), mostPerCall)
}

// secondDeps returns what an account's second pass runs with. The gate decides under the thresholds
// delta sync decides under, which are one value in the worker, and the scanner is the one delta sync
// scans with, built from the worker's one scanner section, so neither job kind reopens the other's
// work (ADR-0120, ADR-0121, ADR-0104).
func secondDeps(store pass2.Store, body pass2.Body, policy func() policy.Composed, lookups classify.Lookups, scanner scan.Scanner) pass2.Deps {
	return pass2.Deps{
		Store: store, Body: body, Policy: policy, Lookups: lookups, Gate: scangate.DefaultConfig(), Scanner: scanner,
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

// passAccount runs the first pass over one account a page at a time. Each page opens the account's
// session first and ends its unit after, then counts the page's unclassified senders, and a page made
// durable is the job's success. It stops at a page boundary once ctx ends.
func passAccount(ctx context.Context, deps pass1.Deps, account string, metrics *pass1.Metrics, u *unit, ask schedule.Ask, logger *slog.Logger) error {
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

// leasedBody returns the body fetch the second pass uses, a call of port's body read under a lease in
// the batch class (ADR-0025).
func leasedBody(limiter *lease.Limiter, port mail.Port[context.Context], account string) pass2.Body {
	return func(ctx context.Context, id string) (mail.MessageBody, error) {
		return lease.Call(ctx, limiter, port, account, ratecore.Batch, mail.ProviderOp{Operation: mail.OpGetMessageBody}, func(ctx context.Context) (mail.MessageBody, error) {
			return port.GetMessageBody(ctx, id)
		})
	}
}

// secondPassAccount runs the second pass over one account a page at a time. Each page opens the
// account's session first and ends its unit after, then sets the account's scan backlog, which is
// removed once the pass ends, and a page made durable is the job's success. It logs counts and identifiers, never a body (ADR-0009). It
// stops at a page boundary once ctx ends.
func secondPassAccount(ctx context.Context, deps pass2.Deps, account string, metrics *pass2.Metrics, u *unit, ask schedule.Ask, logger *slog.Logger) error {
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

// newLimiter returns the rate limiter for the Gmail accounts, each at the target its state row sets
// (ADR-0024).
func newLimiter(ctx context.Context, pool *pgxpool.Pool, accounts []string, metrics *lease.Metrics) (*lease.Limiter, error) {
	targets, err := loweredTargets(ctx, pool, accounts)
	if err != nil {
		return nil, err
	}
	limiter, err := lease.NewWithTargets(pool, gmail.Profile{}.BudgetPerSecond(), targets, metrics)
	if err != nil {
		return nil, fmt.Errorf("building the rate limiter: %w", err)
	}
	return limiter, nil
}

// loweredTargets returns the lower target each account's state row sets, a fraction of its
// provider's declared ceiling, leaving out the accounts that set none (ADR-0024).
func loweredTargets(ctx context.Context, db tx.Beginner, accounts []string) (map[string]float64, error) {
	targets := map[string]float64{}
	for _, account := range accounts {
		err := tx.Run(ctx, db, account, func(t pgx.Tx) error {
			target, err := accountstate.New(t).LoweredTarget(ctx, account)
			if err != nil {
				return err
			}
			if target.Valid {
				targets[account] = float64(target.Float32)
			}
			return nil
		})
		if err != nil {
			return nil, fmt.Errorf("reading the lowered target of account %s: %w", account, err)
		}
	}
	return targets, nil
}

// ids returns the identifiers of every account snapshot lists, the ones the kind serves and the ones
// it skips alike, whose policy the reload loads, so an account that becomes servable has its rules
// read already (ADR-0090, ADR-0041).
func ids(snapshot *accountload.Snapshot) []string {
	var out []string
	for _, a := range snapshot.Accounts() {
		out = append(out, a.ID())
	}
	return out
}

// runID returns a new run's identifier, sixteen random hexadecimal digits.
func runID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
	return hex.EncodeToString(b[:])
}
