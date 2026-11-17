package reads

import (
	"context"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/paths"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// Database is the UI's connection pool as the reads use it. Account-scoped reads run through the
// transaction helper, which sets and verifies the account (ADR-0047), and a read that classifies
// senders runs over its repeatable-read snapshot, so one response sees one state of the policy and
// the index (ADR-0136). The accounts listing runs on it directly, because the listing reads every
// account (ADR-0091).
type Database interface {
	tx.Beginner
	tx.BeginnerWithOptions
	accounts.DBTX
}

// Cadences are the workload intervals the jobs cards display. They are configuration shown, not
// recorded state (docs/UI.md section 8.3).
type Cadences struct {
	Sync       time.Duration
	Heuristics time.Duration
}

// Options are everything the read routes read. The read API endpoint builds them from the
// configuration and the process, and its tests build them by hand, with a fixed clock.
type Options struct {
	Database Database
	Datasets []registry.Dataset
	// Admit is the read API's account check, which the dataset endpoint and the row details run after
	// the pure core has read the request. It answers the request when the account may not be read.
	Admit func(w http.ResponseWriter, r *http.Request) bool
	// Metrics is where the reads register their series (ADR-0076).
	Metrics prometheus.Registerer
	// Draining ends when the process begins to stop serving requests, which ends every open event
	// stream so the rest of the requests can drain. Nil never ends.
	Draining context.Context
	// Clock is the reads' time, which as_of and every relative range read.
	Clock    func() time.Time
	Cadences Cadences
	// Attention is the worth-a-look rules' thresholds (docs/UI.md sections 8.1 and 18.1).
	Attention attention.Thresholds
	// StreamInterval is how often the event stream polls the recorded state (ADR-0058).
	StreamInterval time.Duration
	// Lookups are the functions the sender classifier normalizes domains with (ADR-0004).
	Lookups classify.Lookups
}

// handlers are the read routes' handlers over what they read.
type handlers struct {
	opts        Options
	descriptors []lens.Descriptor
	metrics     *metrics
}

// snapshot is the database as a source of repeatable-read, read-only transactions, over which every
// read that classifies a sender runs, so each statement of one response reads one state and its
// figures and rows agree about every sender's class (ADR-0136).
func (s *handlers) snapshot() tx.Beginner { return tx.Snapshot(s.opts.Database) }

// draining returns what ends when the process begins to stop serving requests, or nil, which never
// ends, when the reads were given nothing.
func (s *handlers) draining() <-chan struct{} {
	if s.opts.Draining == nil {
		return nil
	}
	return s.opts.Draining.Done()
}

// Routes are the read routes, each carrying its handler, in the order the read API mounts them: the
// dataset endpoint, one row detail per dataset that declares a provenance query, then the bespoke
// reads (ADR-0057). It registers the reads' series on o.Metrics.
func Routes(o Options) ([]server.Route, error) {
	m, err := newMetrics(o.Metrics)
	if err != nil {
		return nil, fmt.Errorf("registering the metrics: %w", err)
	}
	s := &handlers{opts: o, descriptors: registry.Descriptors(o.Datasets), metrics: m}
	routes := []server.Route{{Pattern: registry.LensPath, Operation: "getLens", Scoped: true, Registry: true, Handler: s.lens}}
	for _, d := range o.Datasets {
		if d.Detail != nil {
			routes = append(routes, server.Route{Pattern: registry.RowPath(d.Name), Operation: RowOperation(d.Name), Scoped: true, Registry: true, Handler: s.rowDetail(d)})
		}
	}
	return append(routes, []server.Route{
		{
			Pattern: paths.Accounts, Operation: "listAccounts",
			Summary:  "Every account's identifier and provider, the one unscoped read",
			Response: accountsType(), Handler: s.listAccounts,
		},
		{
			Pattern: paths.Accounts + "/{account}/attention", Operation: "getAttention", Scoped: true,
			Summary:  "Home's worth-a-look cards, each worded, in their order",
			Response: attentionType(), Handler: s.getAttention,
		},
		{
			Pattern: paths.Accounts + "/{account}/system", Operation: "getSystem", Scoped: true,
			Summary:  "The account's operational and decisions blocks",
			Response: systemType(), Handler: s.getSystem,
		},
		{
			Pattern: paths.Accounts + "/{account}/corpus", Operation: "getCorpus", Scoped: true,
			Summary:  "Home's corpus figures, which count every message of the account, and the time they were read",
			Response: corpusType(), Handler: s.getCorpus,
		},
		{
			Pattern: paths.Accounts + "/{account}/jobs", Operation: "getJobs", Scoped: true,
			Summary:  "One block per workload, the rate block and the cadences",
			Response: jobsType(), Handler: s.getJobs,
		},
		{
			Pattern: paths.Accounts + "/{account}/jobs/{run}", Operation: "getRun", Scoped: true,
			Summary:  "One run, its resumer, its item failures by disposition and the runs that recovered them, and its timeline",
			Response: runSummaryType(), Handler: s.getRun,
		},
		{
			Pattern: paths.Accounts + "/{account}/events", Operation: "streamEvents", Scoped: true,
			Summary: "The live stream, one event per changed object with its whole current state",
			Events:  eventTypes(), Handler: s.streamEvents,
		},
	}...), nil
}

// RowOperation is the contract's operation for a dataset's row-detail route.
func RowOperation(dataset string) string {
	return "get" + pascal(dataset) + "Row"
}
