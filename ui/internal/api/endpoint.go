package api

import (
	"context"
	"net/http"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
	"github.com/ppat/mediated-mailbox-mcp/process/domains"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/policymanagement"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/reads"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/setup"
)

// New builds the read API endpoint, which the UI's spec lists at /ui/api/ (ADR-0142). It builds the
// setup routes first, which load the public key the UI seals to and log its identifier before the
// database is needed, so the key the UI would seal to is in the log whatever the database answers
// (ADR-0092), and are handed policy management's count of the base rules. It then gets the process's
// pool, whose schema check refuses a schema the binary cannot run on (ADR-0140) and whose report on
// its database the UI is ready by (ADR-0142), gets the checks every endpoint sits behind, builds the
// read routes, handed the read API's account check, and the policy routes, and mounts the three
// behind the checks. Its event streams end once the process starts to stop.
func New(ctx context.Context, p process.Capabilities, c config.UI) (http.Handler, error) {
	setupRoutes, err := setup.Build(ctx, p, c, policymanagement.BaseRuleCount)
	if err != nil {
		return nil, err
	}
	pool, err := database.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	checks, err := server.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	// The sender classifier's domain functions, which policy management matches senders against
	// rules with, from their one constructor, which every program that classifies takes them from
	// (ADR-0004, ADR-0136).
	lookups := domains.Lookups()
	datasets := registry.Datasets(lookups)
	readRoutes, err := reads.Routes(reads.Options{
		Database:       pool,
		Datasets:       datasets,
		Admit:          admission(pool),
		Metrics:        p.Metrics(),
		Draining:       p.Draining(),
		Clock:          time.Now,
		Cadences:       reads.Cadences{Sync: time.Duration(c.SyncInterval), Heuristics: time.Duration(c.HeuristicsInterval)},
		StreamInterval: time.Duration(c.StreamInterval),
		Attention: attention.Thresholds{
			BacklogShare: c.AttentionBacklogShare, MaskCount: c.AttentionMaskCount,
			ServeFactor: c.AttentionServeFactor, GapDays: c.AttentionGapDays,
		},
		Lookups: lookups,
	})
	if err != nil {
		return nil, err
	}
	policyRoutes := policymanagement.Routes(policymanagement.Options{
		Database: pool,
		Clock:    time.Now,
		Lookups:  lookups,
		Identity: policymanagement.Identity{Header: c.IdentityHeader, Operator: c.OperatorName},
	})
	e, err := mount(pool, checks, datasets, readRoutes, setupRoutes, policyRoutes)
	if err != nil {
		return nil, err
	}
	return e.handler, nil
}
