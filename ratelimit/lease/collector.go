package lease

import (
	"context"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/db/ratestate"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ratelimit/core"
)

// The account-level series the mediator emits from each account's rate state (ADR-0077).
// ratelimit/README.md lists them with the alerting rules that read them.
const (
	accountRateName       = "mediated_mailbox_ratelimit_account_rate"
	accountFloorName      = "mediated_mailbox_ratelimit_account_floor"
	accountBucketName     = "mediated_mailbox_ratelimit_account_bucket_level"
	accountSinceAskName   = "mediated_mailbox_ratelimit_account_seconds_since_ask"
	accountSinceGrantName = "mediated_mailbox_ratelimit_account_seconds_since_grant"
)

// collectTimeout bounds the reads one scrape makes.
const collectTimeout = 10 * time.Second

// Account is an account the collector reports on, with its provider's declared ceiling.
type Account struct {
	ID      string
	Ceiling float64
}

// Collector emits each account's rate, floor, bucket level, and the seconds since any class last
// asked and since the last grant, read from the account's rate state at scrape time (ADR-0077). The
// mediator registers it, because it runs continuously, and the job workloads' series are absent
// between their runs by design.
//
// An account with no rate state yet has never spent, and it is reported idle, at the target with an
// empty bucket and no ask or grant. An instant never set reads as infinitely long ago. A read that
// fails is reported as an invalid metric, so the scrape fails loudly and the series go absent, which
// the absence rule catches.
type Collector struct {
	db       tx.Beginner
	accounts func(context.Context) ([]Account, error)
	rate     *prometheus.Desc
	floor    *prometheus.Desc
	bucket   *prometheus.Desc
	sinceAsk *prometheus.Desc
	sinceGot *prometheus.Desc
}

var _ prometheus.Collector = (*Collector)(nil)

// NewCollector returns a Collector reading from db the accounts that accounts returns at each scrape.
// Each account's rows are read in a transaction of its own through db/tx, since row-level security
// shows a transaction one account.
func NewCollector(db tx.Beginner, accounts func(context.Context) ([]Account, error)) *Collector {
	labels := []string{"account"}
	return &Collector{
		db:       db,
		accounts: accounts,
		rate:     prometheus.NewDesc(accountRateName, "The account's current rate, in the provider's units per second.", labels, nil),
		floor:    prometheus.NewDesc(accountFloorName, "The lowest rate the controller goes to for the account, in the provider's units per second.", labels, nil),
		bucket:   prometheus.NewDesc(accountBucketName, "The account's token bucket level as last stored, in the provider's units.", labels, nil),
		sinceAsk: prometheus.NewDesc(accountSinceAskName, "Seconds since any priority class last asked for a lease on the account, by the database's clock.", labels, nil),
		sinceGot: prometheus.NewDesc(accountSinceGrantName, "Seconds since the account's last grant, by the database's clock.", labels, nil),
	}
}

// Describe implements prometheus.Collector.
func (c *Collector) Describe(ch chan<- *prometheus.Desc) {
	for _, d := range []*prometheus.Desc{c.rate, c.floor, c.bucket, c.sinceAsk, c.sinceGot} {
		ch <- d
	}
}

// Collect implements prometheus.Collector.
func (c *Collector) Collect(ch chan<- prometheus.Metric) {
	ctx, cancel := context.WithTimeout(context.Background(), collectTimeout)
	defer cancel()
	accounts, err := c.accounts(ctx)
	if err != nil {
		ch <- prometheus.NewInvalidMetric(c.rate, fmt.Errorf("listing the accounts: %w", err))
		return
	}
	for _, a := range accounts {
		if err := c.collect(ctx, ch, a); err != nil {
			ch <- prometheus.NewInvalidMetric(c.rate, fmt.Errorf("reading account %s: %w", a.ID, err))
		}
	}
}

func (c *Collector) collect(ctx context.Context, ch chan<- prometheus.Metric, a Account) error {
	limits := core.LimitsFor(a.Ceiling)
	row := ratestate.RateStateRow{CurrentRate: float32(limits.Target())}
	var now int64
	err := tx.Run(ctx, c.db, a.ID, func(t pgx.Tx) error {
		q := ratestate.New(t)
		var err error
		if now, err = q.ClockMillis(ctx); err != nil {
			return err
		}
		stored, err := q.RateState(ctx, a.ID)
		switch {
		case errors.Is(err, pgx.ErrNoRows):
			return nil
		case err != nil:
			return err
		}
		row = stored
		return nil
	})
	if err != nil {
		return err
	}
	asked := max(row.InteractiveAskedMs, row.SyncAskedMs, row.BatchAskedMs)
	for _, v := range []struct {
		desc  *prometheus.Desc
		value float64
	}{
		{c.rate, float64(row.CurrentRate)},
		// The floor at the precision the rate is stored at, so a rate held at the floor reads as
		// equal to it whatever the floor's value.
		{c.floor, float64(float32(limits.Floor()))},
		{c.bucket, float64(row.BucketLevel)},
		{c.sinceAsk, secondsSince(now, asked)},
		{c.sinceGot, secondsSince(now, row.LastGrantedMs)},
	} {
		ch <- prometheus.MustNewConstMetric(v.desc, prometheus.GaugeValue, v.value, a.ID)
	}
	return nil
}

// secondsSince returns the seconds from an instant to now, both Unix milliseconds. An instant of zero
// was never set and is infinitely long ago.
func secondsSince(now, at int64) float64 {
	if at == 0 {
		return math.Inf(1)
	}
	return float64(now-at) / 1000
}
