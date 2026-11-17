package provider

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/provider/mail"
	"github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"
	ratecore "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"
)

// Options are what a caller is, apart from its adapters, its series and its pool.
type Options struct {
	// Class is the priority class every lease of the caller is asked in (ADR-0025).
	Class Class
	// Timeout is how long each call has to answer once its lease is issued (ADR-0101).
	Timeout time.Duration
}

// Caller is one job kind's, or the API server's body operation's, own equipment for calling a
// provider, its adapters, its rate limiter's series and its provider series, its class and its
// timeout (ADR-0117). Every series it produces carries the labels of the registerer it was built
// with.
type Caller struct {
	adapters Adapters
	pool     tx.Beginner
	class    ratecore.Class
	timeout  time.Duration
	requests *Metrics
	leases   *ratelimit.Metrics
}

// NewCaller returns a caller over adapters, whose leases run on pool, registering the shared
// provider series and the rate limiter's series on reg.
func NewCaller(adapters Adapters, reg prometheus.Registerer, pool tx.Beginner, o Options) (*Caller, error) {
	class, ok := limiterClass(o.Class)
	switch {
	case !ok:
		return nil, errors.New("a caller names no priority class")
	case o.Timeout <= 0:
		return nil, errors.New("a caller's timeout is not positive")
	case pool == nil:
		return nil, errors.New("a caller has no pool to run its leases on")
	}
	requests, err := NewMetrics(reg)
	if err != nil {
		return nil, err
	}
	leases, err := ratelimit.NewMetrics(reg)
	if err != nil {
		return nil, err
	}
	return &Caller{adapters: adapters, pool: pool, class: class, timeout: o.Timeout, requests: requests, leases: leases}, nil
}

// Connection is one account's connection as the provider library needs it for a unit of work.
type Connection struct {
	// Provider is the provider the account names, which picks the adapter that serves it.
	Provider string
	// Account is the account the calls are for.
	Account string
	// Token is what the calls authenticate with.
	Token *Token
	// Target is the account's lowered rate target, a fraction of the provider's ceiling, read only
	// when Lowered is set. Otherwise the account spends at the default target (ADR-0024).
	Target  float64
	Lowered bool
	// WaitUntil is the latest each lease of the unit is waited for, zero for none (ADR-0130).
	WaitUntil time.Time
	// Again reads the account again when the provider refuses the token's credential, and returns
	// the token made over the credential the account holds now, or nil when it holds the refused
	// credential or none, so the refusal stands. A nil Again leaves every refusal standing
	// (ADR-0090, ADR-0106).
	Again func(ctx context.Context) (*Token, error)
}

// Mailbox returns the account's mailbox for one unit of work over conn. It is built over the
// account's own rate limits, at its lowered target where it has one.
func (c *Caller) Mailbox(conn Connection) (mail.Mailbox, error) {
	facts, ok := FactsOf(conn.Provider)
	entry, shipped := c.adapters.byName[conn.Provider]
	if !ok || !shipped {
		return nil, fmt.Errorf("there is no adapter for provider %q", conn.Provider)
	}
	if conn.Token == nil {
		return nil, errNoToken
	}
	targets := map[string]float64{}
	if conn.Lowered {
		targets[conn.Account] = conn.Target
	}
	limiter, err := ratelimit.NewWithTargets(c.pool, facts.Ceiling, targets, c.leases)
	if err != nil {
		return nil, fmt.Errorf("building the rate limiter: %w", err)
	}
	requests, err := c.requests.For(conn.Provider)
	if err != nil {
		return nil, err
	}
	build := func(t *Token) (adapterMailbox, error) {
		if t.provider != conn.Provider {
			return nil, fmt.Errorf("a token made for provider %q cannot call provider %q", t.provider, conn.Provider)
		}
		return entry.mailbox(conn.Account, t.source, requests)
	}
	port, err := build(conn.Token)
	if err != nil {
		return nil, err
	}
	return &mailbox{
		port: port, build: build, again: conn.Again, limiter: limiter,
		spend: ratelimit.Spend{Account: conn.Account, Class: c.class, WaitUntil: conn.WaitUntil, Timeout: c.timeout},
	}, nil
}
