// Package registry builds the API server's one registry of operations, which both roots are generated
// from, once per process, and registers beside it F3's rate-state collector, the per-account rate
// series only the API server emits (ADR-0030, ADR-0053, ADR-0077, ADR-0142). The operations read the
// index, the recorded state, and the accounts and policy the process's Session holds in force, and
// the body operation alone is built with the functions of api/internal/bodies, the one package of the
// API server that reaches a provider. This package reaches none, and imports neither the connections
// nor an adapter: each account's ceiling for the collector comes from the facts about its provider.
package registry

import (
	"context"
	"encoding/json"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/api/internal/bodies"
	"github.com/ppat/mediated-mailbox-mcp/api/internal/service"
	"github.com/ppat/mediated-mailbox-mcp/content/markdown"
	markdowncore "github.com/ppat/mediated-mailbox-mcp/content/markdown/core"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/scanner"
	"github.com/ppat/mediated-mailbox-mcp/process/sessions"
	"github.com/ppat/mediated-mailbox-mcp/provider"
	"github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"
)

// held is the slot the process keeps the one registry in, so both roots are generated from it.
var held = process.NewOnePerProcess[service.Registry]("registry")

// Get returns the process's registry of operations, built the first time either root asks and the
// same one on every later call. Its build registers the operations' series and the rate-state
// collector, and has the Session's every reload serve the accounts that reload put in force, so the
// registry refuses every call naming an account the list in force does not hold (ADR-0090).
func Get(ctx context.Context, p process.Capabilities, c config.API) (service.Registry, error) {
	return held.Get(ctx, p, func(ctx context.Context, shared process.Capabilities) (service.Registry, func(), error) {
		r, err := build(ctx, shared, c)
		return r, nil, err
	})
}

// build builds the registry for the process's capabilities p.
func build(ctx context.Context, p process.Capabilities, c config.API) (service.Registry, error) {
	s, err := sessions.Get(ctx, p, c)
	if err != nil {
		return service.Registry{}, err
	}
	pool, err := database.Get(ctx, p, c)
	if err != nil {
		return service.Registry{}, err
	}
	fetch, err := bodies.New(ctx, p, c)
	if err != nil {
		return service.Registry{}, err
	}
	scan, err := scanner.Get(ctx, p, c)
	if err != nil {
		return service.Registry{}, err
	}
	observe, err := outcomes(p.Metrics())
	if err != nil {
		return service.Registry{}, err
	}
	processing, err := scanner.NewProcessing(p.Metrics())
	if err != nil {
		return service.Registry{}, err
	}
	if err := p.Metrics().Register(ratelimit.NewCollector(pool, rateAccounts(s))); err != nil {
		return service.Registry{}, err
	}
	ops, err := timed(service.Operations(service.Sources{
		DB:       pool,
		Accounts: accounts(s),
		Policy:   s.Policy,
		Now:      time.Now,
	}, service.Bodies{
		Policy: fetch.Policy,
		Open:   fetch.Open,
		Render: func(body mail.MessageBody) (string, error) {
			defer processing.Since(scanner.StepConvert, time.Now())
			return markdown.Render(body)
		},
		Literal: markdowncore.Literal,
		Scanner: scan,
		Observe: observe,
	}), p.Metrics())
	if err != nil {
		return service.Registry{}, err
	}
	registry, err := service.NewRegistry(nil, ops...)
	if err != nil {
		return service.Registry{}, err
	}
	logger := p.Logger()
	s.AfterReload(func() {
		ids := ids(s)
		registry.Serve(ids)
		logger.Info("accounts served", "accounts", len(ids))
	})
	return registry, nil
}

// accounts returns the accounts listing's source, the accounts the Session holds in force with their
// providers.
func accounts(s *sessions.Session) func() []service.Account {
	return func() []service.Account {
		var out []service.Account
		for _, a := range s.Accounts() {
			out = append(out, service.Account{ID: a.ID, Provider: a.Provider})
		}
		return out
	}
}

// ids returns the identifiers of the accounts the Session holds in force, in order.
func ids(s *sessions.Session) []string {
	var out []string
	for _, a := range s.Accounts() {
		out = append(out, a.ID)
	}
	return out
}

// rateAccounts returns what the rate-state collector reports on, every account in force whose
// provider a shipped adapter serves, with the ceiling that provider's facts declare (ADR-0023,
// ADR-0077).
func rateAccounts(s *sessions.Session) func(context.Context) ([]ratelimit.Account, error) {
	return func(context.Context) ([]ratelimit.Account, error) {
		var out []ratelimit.Account
		for _, a := range s.Accounts() {
			facts, ok := provider.FactsOf(a.Provider)
			if !ok {
				continue
			}
			out = append(out, ratelimit.Account{ID: a.ID, Ceiling: facts.Ceiling})
		}
		return out, nil
	}
}

// The series counting body requests, served and denied, a denial told apart by the stage that
// decided it and its reason (ADR-0036, ADR-0002).
const (
	bodiesServedName = "mediated_mailbox_api_bodies_served_total"
	bodiesDeniedName = "mediated_mailbox_api_bodies_denied_total"
)

// outcomes registers on reg the series counting each body request's outcome and returns what counts
// one.
func outcomes(reg prometheus.Registerer) (func(account string, o service.Outcome), error) {
	served := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: bodiesServedName,
		Help: "The bodies the API server served, each after its audit row was written.",
	}, []string{"account"})
	denied := prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: bodiesDeniedName,
		Help: "The body requests the API server denied, by the stage that decided it, gate or serve, and the reason.",
	}, []string{"account", "stage", "reason"})
	for _, c := range []prometheus.Collector{served, denied} {
		if err := reg.Register(c); err != nil {
			return nil, err
		}
	}
	return func(account string, o service.Outcome) {
		if o.Stage == "" {
			served.WithLabelValues(account).Inc()
			return
		}
		denied.WithLabelValues(account, o.Stage, o.Reason).Inc()
	}, nil
}

// operationDurationName is the series timing each operation of the client surface (ADR-0125).
const operationDurationName = "mediated_mailbox_api_operation_duration_seconds"

// timed returns ops with each operation's handler timed on a series metrics serves, labelled by the
// operation's name, so every call either root makes is timed from the handler's start to its return,
// after the registry's account check. The service layer is unchanged and imports no metrics library
// (ADR-0125).
func timed(ops []service.Operation, metrics prometheus.Registerer) ([]service.Operation, error) {
	duration := prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    operationDurationName,
		Help:    "How long each operation of the client surface took, from its handler's start to its return, by operation.",
		Buckets: []float64{0.005, 0.01, 0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
	}, []string{"operation"})
	if err := metrics.Register(duration); err != nil {
		return nil, err
	}
	out := make([]service.Operation, 0, len(ops))
	for _, op := range ops {
		handle, observed := op.Handle, duration.WithLabelValues(op.Name)
		op.Handle = func(ctx context.Context, account string, input json.RawMessage) (json.RawMessage, error) {
			start := time.Now()
			defer func() { observed.Observe(time.Since(start).Seconds()) }()
			return handle(ctx, account, input)
		}
		out = append(out, op)
	}
	return out, nil
}
