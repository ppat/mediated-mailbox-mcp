// Package deltasync is delta sync's job kind in the worker (ADR-0117, ADR-0119). It holds delta sync's
// own account loader, policy loader and account sessions, on delta sync's own connection pool under
// its own runtime role (ADR-0118), and adds and drops one job per account its reload lists. Each job
// ticks its account on the sync interval's phase (ADR-0103). The reload also re-seals what the account
// load opened with a key that is not the current one, the OAuth clients' secrets included, and sets
// the key-scan series, whether or not any account is listed (ADR-0092).
//
// The worker's composition root builds it with New and ensures its reload job on the scheduler. A
// tick is tick.Run, and its due decision is worker/internal/core/deltasync/due.
package deltasync

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
	"github.com/ppat/mediated-mailbox-mcp/core/scan"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
	"github.com/ppat/mediated-mailbox-mcp/db/accountstate"
	runrecord "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/accountload"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/open"
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
	// Pool is delta sync's own pool, which connects as delta sync's runtime role (ADR-0118).
	Pool *pgxpool.Pool
	// Keys open the stored credentials and client secrets, and seal again what was sealed to a key
	// other than the current one (ADR-0092).
	Keys *open.Keyring
	// Scanner masks subjects and scans bodies, built once from the worker's scanner section.
	Scanner scan.Scanner
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
	config   Config
	limit    *schedule.Limit
	loader   *accountload.Loader
	sessions *session.Holder
	// policies is built at the first reload that lists an account, since a policy loader reads at
	// least one, and never replaced. Every account job is ensured after it is built.
	policies *policyload.Loader
	ticks    *tick.Metrics
	keyScan  *reseal.Metrics
	leases   *lease.Metrics
	// processing times each body's conversion and scan in a tick (ADR-0125).
	processing *series.Processing
	lookups    classify.Lookups
	// port builds an account's Gmail adapter in a kind New built, over the series New registered, so
	// a test reaches the adapter series the kind serves.
	port func(account string, tokens gmail.Tokens) (mail.Port[context.Context], error)
	// served are the accounts the reload ensured a job for.
	served []string
}

// New returns delta sync's job kind with its series registered on c.Registry. Each tick's account
// session builds a Gmail source and adapter over c.Client, fresh for the tick, so delta sync holds no
// source across ticks.
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
	loader := accountload.New(c.Pool, c.Keys, c.Logger, []string{gmailProvider})
	return &DeltaSync{
		config: c, limit: schedule.NewLimit(c.Concurrency), loader: loader,
		sessions: session.New(session.Config{Loader: loader, DB: c.Pool, Logger: c.Logger, Connectors: connectors}),
		ticks:    ticks, keyScan: keyScan, leases: leases, processing: processing,
		lookups: classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne},
	}, nil
}

// Reload returns the job kind's reload, the job keyed by the kind alone that reloads the account
// snapshot and the policy, re-seals, and adds and drops the account jobs (ADR-0090, ADR-0092,
// ADR-0119).
func (d *DeltaSync) Reload() (schedule.Key, schedule.Job) {
	return schedule.Key{Kind: Kind}, schedule.Job{
		Run: d.reload, Interval: d.config.ReloadInterval, Backoff: backoff, Late: lateReloads * d.config.ReloadInterval, Logger: d.config.Logger,
	}
}

// reload reloads the account snapshot, re-seals what it opened with a key that is not the current one
// and sets the key-scan series from what it found, reloads the policy of every account it lists,
// then ensures a job for each account it lists that the kind can serve and removes the job of each it
// no longer serves, with that account's series. A snapshot whose read fails keeps the previous one and
// changes no job. A policy whose reload fails adds no job, because an account whose rules were never
// read would be decided as if every sender were restricted, and still drops the jobs of the accounts no
// longer listed (ADR-0090, ADR-0092, ADR-0041).
func (d *DeltaSync) reload(ctx context.Context, _ schedule.Ask) error {
	if err := d.loader.Load(ctx); err != nil {
		return fmt.Errorf("loading the accounts: %w", err)
	}
	d.loader.Reseal(ctx)
	d.keyScan.Set(d.loader.ResealClients(ctx, reseal.Writer(d.config.Pool)))
	snapshot := d.loader.Snapshot()
	listed := d.sessions.Accounts(snapshot)
	policyErr := d.loadPolicy(ctx, ids(snapshot))
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

// loadPolicy reloads the policy of accounts through the kind's own policy loader, building it at the
// first reload that lists an account, so a tick decides against the policy as it stands, which the
// delisting comparison needs (ADR-0037, ADR-0041). A reload that fails keeps the snapshot it had.
func (d *DeltaSync) loadPolicy(ctx context.Context, accounts []string) error {
	if len(accounts) == 0 {
		return nil
	}
	if d.policies == nil {
		built, err := policyload.New(d.config.Pool, accounts, d.config.Registry)
		if err != nil {
			return err
		}
		d.policies = built
	} else if err := d.policies.SetAccounts(accounts); err != nil {
		return err
	}
	return d.policies.Reload(ctx)
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
		recorded, rErr = runrecord.New(t).RecordedSuccess(ctx, runrecord.RecordedSuccessParams{AccountID: account, Workload: Kind})
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

// tick runs one tick over the account with the kind's active account snapshot and policy snapshot,
// which the tick takes at its entry and keeps (ADR-0090, ADR-0041), under a rate limiter at the target
// the account's state row sets (ADR-0024).
func (d *DeltaSync) tick(ctx context.Context, account string, logger *slog.Logger) error {
	snapshot, policy := d.loader.Snapshot(), d.policies.Snapshot()
	limiter, err := newLimiter(ctx, d.config.Pool, []string{account}, d.leases)
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
		Lookups:     d.lookups,
		Gate:        scangate.DefaultConfig(),
		Scanner:     d.config.Scanner,
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

// newLimiter returns the rate limiter for the Gmail accounts, each at the target its state row sets
// (ADR-0024).
func newLimiter(ctx context.Context, pool *pgxpool.Pool, accounts []string, metrics *lease.Metrics) (*lease.Limiter, error) {
	targets := map[string]float64{}
	for _, account := range accounts {
		err := tx.Run(ctx, pool, account, func(t pgx.Tx) error {
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
	limiter, err := lease.NewWithTargets(pool, gmail.Profile{}.BudgetPerSecond(), targets, metrics)
	if err != nil {
		return nil, fmt.Errorf("building the rate limiter: %w", err)
	}
	return limiter, nil
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
