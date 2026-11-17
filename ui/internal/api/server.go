package api

import (
	"context"
	"errors"
	"fmt"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/paths"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// Database is the UI's connection pool, connected as the application user (ADR-0143). Account-scoped
// reads run through the transaction helper, which sets and verifies the account (ADR-0047), and a
// read that classifies senders runs over its repeatable-read snapshot, so one response sees one state
// of the policy and the index (ADR-0136). The accounts listing runs on it directly, because the
// listing reads every account (ADR-0091).
type Database interface {
	tx.Beginner
	tx.BeginnerWithOptions
	accounts.DBTX
}

// snapshot is the database as a source of repeatable-read, read-only transactions, over which every
// read that classifies a sender runs, so each statement of one response reads one state and its
// figures and rows agree about every sender's class (ADR-0136).
func (s *Server) snapshot() tx.Beginner { return tx.Snapshot(s.opts.Database) }

// Cadences are the workload intervals the jobs cards display. They are configuration shown, not
// recorded state (docs/UI.md section 8.3).
type Cadences struct {
	Sync       time.Duration
	Heuristics time.Duration
}

// Options is everything the server reads. Nothing it uses is ambient. New builds them from the
// configuration and the process, and the package's tests build them by hand, with a fixed clock, the
// provider fake's consents and a fault between a setup's writes, which no configuration carries.
type Options struct {
	Database Database
	Datasets []registry.Dataset
	// Checks are what the read API sits behind, and what the consent attempt is sealed with
	// (ADR-0061, ADR-0111).
	Checks *server.Checks
	// Metrics is where the server registers its series (ADR-0076).
	Metrics prometheus.Registerer
	// Draining ends when the process begins to stop serving requests, which ends every open event
	// stream so the rest of the requests can drain. Nil never ends.
	Draining context.Context
	// Clock is the server's time, which as_of and every relative range read.
	Clock    func() time.Time
	Cadences Cadences
	// Attention is the worth-a-look rules' thresholds (docs/UI.md sections 8.1 and 18.1).
	Attention attention.Thresholds
	// StreamInterval is how often the event stream polls the recorded state (ADR-0058).
	StreamInterval time.Duration
	// Seal is the public key the UI seals each client's secret and each credential to. It holds no key
	// that opens one (ADR-0081).
	Seal seal.PublicKey
	// Consents are the consents of the providers whose accounts connect through an OAuth client, by
	// provider, which the setups check clients and connect accounts through (ADR-0106, ADR-0107).
	Consents map[string]mail.Consent[context.Context]
	// ClientSecrets hands a code exchange its client's secret.
	ClientSecrets ClientSecrets
	// Lookups are the functions the sender classifier normalizes domains with, which policy
	// management matches senders against rules with (ADR-0004).
	Lookups classify.Lookups
	// Identity is who a policy write is recorded as made by (ADR-0084).
	Identity Identity
}

// Server is the read API's server.
type Server struct {
	opts Options
	keys keys
	// between is the fault a test injects between a setup's writes, nil otherwise.
	between     func(at string) error
	descriptors []lens.Descriptor
	metrics     *metrics
	handler     http.Handler
	// uiMux is kept, not its patterns, so Served reads every pattern registered through Handle, whenever
	// it was registered.
	uiMux *recordingMux
}

// recordingMux is the read API's mux. A route registered through its Handle is recorded, and the test
// of the read API's routes compares them with the contract. The go vet routes analyser refuses, under
// ui/ outside this type's Handle, a use of a mux's Handle or HandleFunc, of http.Handle or
// http.HandleFunc, and of an interface's or type parameter's method of the same name and parameters,
// so a direct call on the underlying mux is refused, and Handle keeps its name and its receiver. The
// endpoints the process mounts are those the UI's spec lists, which its own test reads. Left to review
// are a handler wrapped in front of the mux in New or in another endpoint's Build, a path answered
// inside a catch-all's handler, the /ui/ pages handler or the /ui/api/ unrouted handler, which has no
// pattern to compare, a registration through reflection, a route an imported package such as net/http/pprof
// registers on the default mux when it is initialised, served only by a server given no handler, and
// a registration through an interface method one of whose parameter types is a type parameter. The
// mux sits in an unexported field and is embedded nowhere, so no promoted method registers a route
// unrecorded.
type recordingMux struct {
	// noCopy makes go vet's copylocks check refuse a copy of a recording mux, since a copy shares the
	// underlying mux and records its patterns where Served never reads them. A copy whose line carries
	// a //nolint:govet directive with a reason passes, because govet is an ordinary linter, and is left
	// to review.
	noCopy   noCopy
	mux      *http.ServeMux
	patterns []string
}

// noCopy holds no state. Its Lock and Unlock methods are what copylocks looks for.
type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

func (m *recordingMux) Handle(pattern string, h http.Handler) {
	m.patterns = append(m.patterns, pattern)
	m.mux.Handle(pattern, h)
}

func (m *recordingMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	m.mux.ServeHTTP(w, r)
}

// newServer builds the server. It refuses a bespoke route without a handler and a handler without a
// route, so the mounted API is exactly the registry's path and Bespoke.
func newServer(opts Options) (*Server, error) {
	if err := CheckPaths(registry.Paths(opts.Datasets), Bespoke()); err != nil {
		return nil, err
	}
	if problems := registry.Check(opts.Datasets); len(problems) > 0 {
		return nil, fmt.Errorf("the registry's statements do not match its declarations: %v", problems)
	}
	m, err := newMetrics(opts.Metrics)
	if err != nil {
		return nil, fmt.Errorf("registering the metrics: %w", err)
	}
	if opts.Checks == nil {
		return nil, errors.New("the read API was given no checks to sit behind")
	}
	if opts.ClientSecrets == nil {
		opts.ClientSecrets = noSecrets{}
	}
	s := &Server{opts: opts, keys: keys{attempt: opts.Checks.AttemptCipher()}, descriptors: registry.Descriptors(opts.Datasets), metrics: m}
	handlers := map[string]func(http.ResponseWriter, *http.Request){
		"getLens":           s.lens,
		"listAccounts":      s.listAccounts,
		"getSystem":         s.getSystem,
		"getCorpus":         s.getCorpus,
		"getAttention":      s.getAttention,
		"getJobs":           s.getJobs,
		"getRun":            s.getRun,
		"streamEvents":      s.streamEvents,
		"getInstallation":   s.getInstallation,
		"addClient":         s.addClient,
		"replaceClient":     s.replaceClient,
		"removeClient":      s.removeClient,
		"startConnect":      s.startConnect,
		"getConnect":        s.getConnect,
		"finishConnect":     s.finishConnect,
		"getAccount":        s.getAccount,
		"setTarget":         s.setTarget,
		"startReauthorize":  s.startReauthorize,
		"getReauthorize":    s.getReauthorize,
		"finishReauthorize": s.finishReauthorize,
		"getPolicyMatch":    s.getMatch,
		"getPolicyRelease":  s.getRelease,
		"getBaseMatch":      s.getBaseMatch,
		"addRule":           s.postAccountRule,
		"editRule":          s.postAccountEdit,
		"liftRule":          s.postAccountLift,
		"exportPolicy":      s.getAccountExport,
		"previewImport":     s.postAccountPreview,
		"importPolicy":      s.postAccountImport,
		"getBasePolicy":     s.getBasePolicy,
		"getBaseHistory":    s.getBaseHistory,
		"addBaseRule":       s.postBaseRule,
		"editBaseRule":      s.postBaseEdit,
		"liftBaseRule":      s.postBaseLift,
		"exportBasePolicy":  s.getBaseExport,
		"previewBaseImport": s.postBasePreview,
		"importBasePolicy":  s.postBaseImport,
	}
	for _, d := range opts.Datasets {
		if d.Detail != nil {
			handlers[RowOperation(d.Name)] = s.rowDetail(d)
		}
	}
	mux := &recordingMux{mux: http.NewServeMux()}
	for _, route := range apiRoutes(opts.Datasets) {
		h, ok := handlers[route.Operation]
		if !ok {
			return nil, fmt.Errorf("the route %s has no handler for %s", route.Pattern, route.Operation)
		}
		delete(handlers, route.Operation)
		pattern := route.Verb() + " " + route.Pattern
		switch {
		case route.Registry:
			// The dataset endpoint and the row-detail routes admit their account themselves, after the
			// pure core has read the request, so a request the registry does not declare is refused
			// before any statement runs.
			mux.Handle(pattern, named(route.Pattern, h))
		case route.Scoped:
			mux.Handle(pattern, s.scoped(route.Pattern, h))
		default:
			mux.Handle(pattern, named(route.Pattern, h))
		}
	}
	if len(handlers) > 0 {
		return nil, fmt.Errorf("handlers with no route: %v", slices.Sorted(maps.Keys(handlers)))
	}
	mux.Handle(paths.API+"/", named("unrouted", func(w http.ResponseWriter, r *http.Request) {
		server.WriteFailure(w, r, server.ClientFault(http.StatusNotFound, "not_found", "no read API route answers this path"))
	}))
	s.uiMux = mux
	// Every request is given its session, and every request whose method is not GET or HEAD is refused
	// before it is routed unless it carries the session's request token (ADR-0061).
	s.handler = opts.Checks.Wrap(mux)
	return s, nil
}

// apiRoutes is the list newServer mounts the read API from, the registry's paths and the bespoke handlers.
// What the server actually mounts is checked against the contract through Served, so a route added
// outside this list is caught as well.
func apiRoutes(datasets []registry.Dataset) []server.Route {
	routes := []server.Route{{Pattern: registry.LensPath, Operation: "getLens", Scoped: true, Registry: true}}
	for _, d := range datasets {
		if d.Detail != nil {
			routes = append(routes, server.Route{Pattern: registry.RowPath(d.Name), Operation: RowOperation(d.Name), Scoped: true, Registry: true})
		}
	}
	return append(routes, Bespoke()...)
}

// Served is every pattern registered through the read API's recording mux, in registration order.
// The test of the read API's routes requires it to be exactly the contract document's operations and
// the read API's catch-all, /ui/api/, so any other pattern registered through Handle fails the test.
func (s *Server) Served() []string {
	return slices.Clone(s.uiMux.patterns)
}

// Handler serves the read API behind the checks.
func (s *Server) Handler() http.Handler { return s.handler }

// draining returns what ends when the process begins to stop serving requests, or nil, which never
// ends, when the server was given nothing.
func (s *Server) draining() <-chan struct{} {
	if s.opts.Draining == nil {
		return nil
	}
	return s.opts.Draining.Done()
}

// named records the route pattern for the log line.
func named(route string, h func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		server.RequestLogOf(r.Context()).Route = route
		h(w, r)
	})
}

// scoped admits a request only for an account that exists. The account is the path's {account}
// segment, so a request without one does not route (404). The value meaning every account, and an
// account the accounts table does not hold, are client faults (docs/UI.md section 17). Nothing in the
// read API aggregates across accounts.
func (s *Server) scoped(route string, h func(http.ResponseWriter, *http.Request)) http.Handler {
	return named(route, func(w http.ResponseWriter, r *http.Request) {
		if s.admit(w, r) {
			h(w, r)
		}
	})
}

// admit reports whether the request's account may be read, and answers the request when it may not.
func (s *Server) admit(w http.ResponseWriter, r *http.Request) bool {
	account := r.PathValue("account")
	server.RequestLogOf(r.Context()).Account = account
	if account == "all" {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "all_accounts", "every view is one account's, and all names every account"))
		return false
	}
	listed, err := accounts.New(s.opts.Database).Accounts(r.Context())
	if err != nil {
		s.databaseFailure(w, r, err)
		return false
	}
	if !slices.ContainsFunc(listed, func(a accounts.AccountsRow) bool { return a.AccountID == account }) {
		server.WriteFailure(w, r, server.ClientFault(http.StatusBadRequest, "unknown_account", "no account has this identifier"))
		return false
	}
	return true
}

// databaseFailure reports a failed read. A cancelled request is the client going away, and its
// failure is logged, never reported as the database's.
func (s *Server) databaseFailure(w http.ResponseWriter, r *http.Request, err error) {
	server.RequestLogOf(r.Context()).Err = err
	if errors.Is(err, context.Canceled) {
		return
	}
	server.WriteFailure(w, r, server.DatabaseFault())
}
