// Package deltasync is delta sync's job kind in the worker (ADR-0117, ADR-0119). It runs its own
// statements on delta sync's own connection pool under its own runtime role (ADR-0118), and works
// from the worker's one execution context, which the worker builds over delta sync's pool, through
// account sessions of its own that call Gmail through delta sync's own adapter series. Its reload adds
// and drops one job per account the context's latest reload lists. Each job ticks its account on the
// sync interval's phase (ADR-0103). The reload also re-seals what the context's account load opened
// with a key that is not the current one, the OAuth clients' secrets included, and sets the key-scan
// series, whether or not any account is listed (ADR-0092).
//
// The worker's composition root builds it with New and ensures its reload job on the scheduler. A
// tick is tick.Run, and its due decision is worker/internal/core/deltasync/due.
package deltasync

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net/http"
	"slices"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/assembly"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/policyload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/session"
	"github.com/ppat/mediated-mailbox-mcp/provider/gmail"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/lease"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/core/deltasync/due"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/reseal"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/tick"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/schedule"
	"github.com/ppat/mediated-mailbox-mcp/worker/internal/series"
)

// Kind is delta sync's job kind as job_runs.workload spells it, which the worker's series and logs
// carry as the job kind (ADR-0016, ADR-0117).
const Kind = tick.Workload

// gmailProvider is the provider an account served through the Gmail adapter names, in accounts and
// in oauth_clients.
const gmailProvider = "gmail"

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

// lateReloads is how many reload intervals after its latest success the kind's reload counts as late.
const lateReloads = 5

// Jobs is where the kind's reload adds and drops its account jobs, the worker's scheduler.
type Jobs interface {
	Ensure(key schedule.Key, job schedule.Job) error
	Remove(key schedule.Key)
}

// Config is what delta sync's job kind is built from, and nothing more.
type Config struct {
	// Pool is delta sync's own pool, which connects as delta sync's runtime role, and which its own
	// statements, its leases, its recording of authentication attempts and its re-seal of the OAuth
	// clients' secrets run under (ADR-0118, ADR-0092).
	Pool *pgxpool.Pool
	// Context is the worker's execution context, whose loader delta sync re-seals through and whose
	// scanner masks subjects and scans bodies (ADR-0090, ADR-0092, ADR-0120).
	Context *assembly.Context
	// Client is the HTTP client the Gmail adapter and its token source call through.
	Client *http.Client
	// Registry is the worker's registry, labelled with delta sync's job kind.
	Registry prometheus.Registerer
	// Logger carries delta sync's job kind.
	Logger *slog.Logger
	// Jobs is the scheduler the reload adds and drops account jobs on.
	Jobs Jobs
	// Concurrency bounds the account jobs ticking at once, and so the bodies held at once.
	Concurrency int
	// ReloadInterval is how often the reload runs.
	ReloadInterval time.Duration
	// SyncInterval is how often an account ticks (ADR-0018, ADR-0103).
	SyncInterval time.Duration
	// FirstWindow is how far back an account with no cursor is reconciled at its first tick
	// (ADR-0105).
	FirstWindow time.Duration
	// DecisionsPerTick bounds how many waiting messages one tick decides for an account (ADR-0104).
	DecisionsPerTick int
}

// DeltaSync is delta sync's job kind.
type DeltaSync struct {
	config Config
	limit  *schedule.Limit
	// sessions hold each account's token source across ticks while the stored credential is the one
	// it was built from or the one it rotated to, one source per account shared with backfill through
	// the execution context, so an access token serves the units after the one it was issued in.
	sessions *session.Holder
	ticks    *tick.Metrics
	keyScan  *reseal.Metrics
	leases   *lease.Metrics
	// processing times each body's conversion and scan in a tick (ADR-0125).
	processing *series.Processing
	// port builds an account's Gmail adapter in a kind New built, over the series New registered, so
	// a test reaches the adapter series the kind serves.
	port func(account string, tokens gmail.Tokens) (mail.Port[context.Context], error)
	// served are the accounts the reload ensured a job for.
	served []string
}

// New returns delta sync's job kind with its series registered on c.Registry. Its account sessions
// build each Gmail adapter over c.Client, and a Gmail source over it when the execution context holds
// none for the account.
func New(c Config) (*DeltaSync, error) {
	metrics, err := gmail.NewMetrics(c.Registry)
	if err != nil {
		return nil, err
	}
	port := gmailPort(metrics, c.Client)
	connect := session.Connect(func(cr session.Credentials) *gmail.TokenSource {
		return gmail.NewTokenSource(c.Client, gmail.NewCredentials(cr.ClientID, cr.ClientSecret, cr.Credential))
	}, func(account string, source *gmail.TokenSource) (mail.Port[context.Context], error) {
		return port(account, source)
	})
	d, err := build(c, map[string]session.Connector{gmailProvider: connect})
	if err != nil {
		return nil, err
	}
	d.port = port
	return d, nil
}

// gmailPort returns the builder of an account's Gmail adapter over its tokens, calling through client
// and counting its requests on metrics, which every adapter it builds shares (ADR-0077).
func gmailPort(metrics *gmail.Metrics, client *http.Client) func(account string, tokens gmail.Tokens) (mail.Port[context.Context], error) {
	return func(account string, tokens gmail.Tokens) (mail.Port[context.Context], error) {
		return gmail.New(gmail.Config{Account: account, Client: client, Tokens: tokens, Metrics: metrics})
	}
}

// build returns the job kind with the connectors its account sessions build sources and ports with.
func build(c Config, connectors map[string]session.Connector) (*DeltaSync, error) {
	ticks, err := tick.NewMetrics(c.Registry)
	if err != nil {
		return nil, err
	}
	keyScan, err := reseal.NewMetrics(c.Registry)
	if err != nil {
		return nil, err
	}
	leases, err := lease.NewMetrics(c.Registry)
	if err != nil {
		return nil, err
	}
	processing, err := series.NewProcessing(c.Registry)
	if err != nil {
		return nil, err
	}
	return &DeltaSync{
		config: c, limit: schedule.NewLimit(c.Concurrency),
		sessions: c.Context.Sessions(connectors, c.Pool),
		ticks:    ticks, keyScan: keyScan, leases: leases, processing: processing,
	}, nil
}

// Reload returns the job kind's reload, the job keyed by the kind alone that re-seals and adds and
// drops the account jobs from what the execution context's latest reload left in force, asked on the
// reload interval and woken by each reload of the context (ADR-0090, ADR-0092, ADR-0119).
func (d *DeltaSync) Reload() (schedule.Key, schedule.Job) {
	return schedule.Key{Kind: Kind}, schedule.Job{
		Run: d.reload, Interval: d.config.ReloadInterval, Backoff: backoff, Late: lateReloads * d.config.ReloadInterval, Logger: d.config.Logger,
	}
}

// reload re-seals what the execution context's account load opened with a key that is not the
// current one and sets the key-scan series from what it found, then ensures a job for each account the
// context's latest reload lists that the kind can serve and removes the job of each it no longer
// serves, with that account's series. While no load of the accounts has succeeded it changes nothing
// and answers not due, neither a success nor a failure, so a start logs no error before the context's
// first load wakes it. When the latest read of the accounts failed it changes nothing and fails. When the latest reload's policy load failed it adds
// no job, because an account whose rules were never read would be decided as if every sender were
// restricted, still drops the jobs of the accounts no longer listed, and fails (ADR-0090, ADR-0092,
// ADR-0041).
func (d *DeltaSync) reload(ctx context.Context, _ schedule.Ask) error {
	latest := d.config.Context.Latest()
	if !latest.Loaded() {
		return schedule.ErrNotDue
	}
	policyErr := latest.Err()
	if policyErr != nil && !errors.Is(policyErr, assembly.ErrPolicyNotLoaded) {
		return policyErr
	}
	loader := d.config.Context.Loader()
	loader.Reseal(ctx)
	d.keyScan.Set(loader.ResealClients(ctx, reseal.Writer(d.config.Pool)))
	snapshot := latest.Snapshot()
	listed := d.sessions.Accounts(snapshot)
	if policyErr != nil {
		d.config.Logger.Error("the policy was not reloaded, so no account newly listed is synced until it is", "error", policyErr)
	}
	var kept []string
	for _, account := range d.served {
		if slices.Contains(listed, account) {
			kept = append(kept, account)
			continue
		}
		d.config.Logger.Info("an account is no longer listed, so its delta sync stops", "account", account)
		d.config.Jobs.Remove(schedule.Key{Kind: Kind, ID: account})
		d.ticks.Forget(account)
	}
	d.served = kept
	if policyErr != nil {
		return policyErr
	}
	for _, account := range listed {
		if slices.Contains(d.served, account) {
			continue
		}
		job := d.job(account)
		job.LastSuccess = d.seed(ctx, account)
		if err := d.config.Jobs.Ensure(schedule.Key{Kind: Kind, ID: account}, job); err != nil {
			return err
		}
		d.served = append(d.served, account)
	}
	return nil
}

// job returns the account's job, asked on the sync interval's phase and holding the kind's limit. Its
// due decision reads when its last tick was asked for, which only its own runs write.
func (d *DeltaSync) job(account string) schedule.Job {
	logger := d.config.Logger.With("account", account)
	ticked, last := false, mail.UnixMilli(0)
	return schedule.Job{
		Interval: d.config.SyncInterval, Limit: d.limit, Backoff: backoff, Late: lateIntervals * d.config.SyncInterval, Logger: logger,
		Run: func(ctx context.Context, ask schedule.Ask) error {
			at := mail.UnixMilli(ask.At.UnixMilli())
			if !due.Decide(ticked, last, at, d.config.SyncInterval.Milliseconds()) {
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
	err := tx.Run(ctx, d.config.Pool, account, func(t pgx.Tx) error {
		var rErr error
		recorded, rErr = runrecord.New(t).RecordedSuccess(ctx, Kind)
		return rErr
	})
	if err != nil {
		d.config.Logger.Warn("reading the account's latest recorded success failed, so its job counts from now", "account", account, "error", err)
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

// tick runs one tick over the account with the execution context's account snapshot and policy
// snapshot in force, which the tick takes at its entry and keeps (ADR-0090, ADR-0041), under a rate
// limiter at the target the account's state row set when the context last reloaded (ADR-0024).
func (d *DeltaSync) tick(ctx context.Context, account string, logger *slog.Logger) error {
	snapshot, policy := d.config.Context.Snapshot(), d.config.Context.PolicySnapshot()
	limiter, err := d.config.Context.Limiter(d.config.Pool, account, gmail.Profile{}.BudgetPerSecond(), d.leases)
	if err != nil {
		return err
	}
	return d.tickAccount(ctx, snapshot, account, limiter, policy, logger)
}

// tickAccount runs one tick over the account in a session opened with the credentials snapshot holds
// for it, over a token source built for the tick, then ends the session's unit of work, which hands
// its token over with the adoption stamp of the credential it holds and records its latest
// authentication attempt, a tick that failed included (ADR-0089, ADR-0097). It logs counts and
// identifiers, never a body (ADR-0009).
func (d *DeltaSync) tickAccount(ctx context.Context, snapshot *accountload.Snapshot, account string, limiter *lease.Limiter, policy policyload.Snapshot, logger *slog.Logger) error {
	sess, err := d.sessions.Open(snapshot, account)
	if err != nil {
		return err
	}
	provider := &reauthorizing{session: sess, limiter: limiter, account: account}
	result, err := tick.Run(ctx, d.tickDeps(account, provider, policy), account)
	d.ticks.Count(account, result)
	c := result.Progress.Counters
	if err != nil {
		logger.Error("the account's tick failed", "run", result.Run, "recovery", result.Recovery, "error", err)
	} else {
		logger.Info("the account's tick ended", "run", result.Run, "recovery", result.Recovery, "gap", result.Gap,
			"added", c.Added, "modified", c.Modified, "removed", c.Removed, "decided", c.Decided, "scanned", c.Scanned, "skipped", c.Skipped)
	}
	handedOver, recorded := sess.End(ctx)
	return errors.Join(err, handedOver, recorded)
}

// tickDeps returns what the account's tick runs with over provider. The gate decides under the
// thresholds backfill decides under, which are one value in the worker, and the scanner is the one
// backfill scans with, built from the worker's one scanner section, so neither job kind reopens the
// other's work (ADR-0120, ADR-0121, ADR-0104).
func (d *DeltaSync) tickDeps(account string, provider tick.Provider, policy policyload.Snapshot) tick.Deps {
	return tick.Deps{
		Store:       tick.NewPostgres(d.config.Pool),
		Provider:    provider,
		Policy:      policy.For(account),
		Gate:        scangate.DefaultConfig(),
		Scanner:     d.config.Context.Scanner(),
		Processing:  d.processing,
		FirstWindow: d.config.FirstWindow,
		Decisions:   d.config.DecisionsPerTick,
		PageSize:    pageSize,
		RunID:       runID,
		Now:         time.Now,
	}
}

// reauthorizing is the Provider a tick calls for one account. Each call goes through the leased
// port over the account's session, which reads a credential the provider refuses again from the
// account's row before the refusal is returned. A row holding another credential, one the operator
// stored since the tick took its snapshot, has the source and the port built again over it with the
// client the account now connects through (ADR-0106), kept for the tick's later calls and its
// hand-over, and the call made once more, so a re-authorization reaches the tick at the call it
// refused (ADR-0090). A row holding the refused credential, or none, returns the refusal.
type reauthorizing struct {
	session *session.Session
	limiter *lease.Limiter
	account string
}

var _ tick.Provider = (*reauthorizing)(nil)

// leased makes call over the account's leased port through the session. The call made again goes
// through call whole, a new lease included.
func leased[T any](ctx context.Context, r *reauthorizing, call func(context.Context, tick.Leased) (T, error)) (T, error) {
	return session.Call(ctx, r.session, func(ctx context.Context, port mail.Port[context.Context]) (T, error) {
		return call(ctx, tick.Leased{Limiter: r.limiter, Port: port, Account: r.account})
	})
}

// CurrentCursor implements tick.Provider.
func (r *reauthorizing) CurrentCursor(ctx context.Context) (mail.Cursor, error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) (mail.Cursor, error) { return l.CurrentCursor(ctx) })
}

// ChangesSince implements tick.Provider.
func (r *reauthorizing) ChangesSince(ctx context.Context, c mail.Cursor) (mail.ChangeSet, error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) (mail.ChangeSet, error) { return l.ChangesSince(ctx, c) })
}

// GetMessageMetadata implements tick.Provider.
func (r *reauthorizing) GetMessageMetadata(ctx context.Context, ids []string) ([]mail.MessageMetadata, error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) ([]mail.MessageMetadata, error) {
		return l.GetMessageMetadata(ctx, ids)
	})
}

// ListThreads implements tick.Provider.
func (r *reauthorizing) ListThreads(ctx context.Context, q mail.Query, page mail.PageToken) (mail.Page[mail.ThreadMetadata], error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) (mail.Page[mail.ThreadMetadata], error) {
		return l.ListThreads(ctx, q, page)
	})
}

// GetMessageBody implements tick.Provider.
func (r *reauthorizing) GetMessageBody(ctx context.Context, id string) (mail.MessageBody, error) {
	return leased(ctx, r, func(ctx context.Context, l tick.Leased) (mail.MessageBody, error) { return l.GetMessageBody(ctx, id) })
}

// runID returns a new run's identifier, sixteen random hexadecimal digits.
func runID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error.
	return hex.EncodeToString(b[:])
}
