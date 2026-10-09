package api

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"maps"
	"net/http"
	"slices"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/executioncontext/credential/seal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/attention"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/lens"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// Database is the UI's connection pool, connected as the UI's own role (ADR-0084). Account-scoped
// reads run through the transaction helper, which sets and verifies the account (ADR-0047). The
// accounts listing runs on it directly, because the listing reads every account (ADR-0091).
type Database interface {
	tx.Beginner
	accounts.DBTX
	Ping(ctx context.Context) error
}

// Cadences are the workload intervals the jobs cards display. They are configuration shown, not
// recorded state (docs/UI.md section 8.3).
type Cadences struct {
	Sync       time.Duration
	Heuristics time.Duration
}

// Options is everything the server reads. Nothing it uses is ambient.
type Options struct {
	Bundle   fs.FS
	Database Database
	Datasets []registry.Dataset
	Logger   *slog.Logger
	Metrics  *prometheus.Registry
	// Clock is the server's time, which as_of and every relative range read.
	Clock    func() time.Time
	Cadences Cadences
	// Attention is the worth-a-look rules' thresholds (docs/UI.md sections 8.1 and 18.1).
	Attention attention.Thresholds
	// StreamInterval is how often the event stream polls the recorded state (ADR-0058).
	StreamInterval time.Duration
	// Browser is the configuration the entry document hands the browser.
	Browser Browser
	// TokenKey is the key behind the request token and the consent attempt's seal, at least
	// MinTokenKey bytes, generated at the process's start or read from token_key_file (ADR-0061,
	// ADR-0111).
	TokenKey []byte
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

// Server is the UI's server.
type Server struct {
	opts Options
	keys keys
	// reserved are the words the UI's own top-level paths use, which no account identifier may be
	// (docs/UI.md section 8.12).
	reserved []string
	// between is the fault a test injects between a setup's writes, nil otherwise.
	between     func(at string) error
	descriptors []lens.Descriptor
	metrics     *metrics
	handler     http.Handler
	probes      http.Handler
	// uiMux and probesMux are kept, not their patterns, so Served and ProbesServed read every pattern
	// registered through Handle, whenever it was registered.
	uiMux     *recordingMux
	probesMux *recordingMux
}

// recordingMux is the mux of both the UI's listener and the probes listener. A route registered
// through its Handle is recorded, and the contract's test compares the UI's routes with the contract
// and the probes' with their three. The go vet routes analyser refuses, under ui/ outside this type's
// Handle, a use of a mux's Handle or HandleFunc, of http.Handle or http.HandleFunc, and of an
// interface's or type parameter's method of the same name and parameters, so a direct call on the
// underlying mux is refused, and Handle keeps its name and its receiver. Left to review are a handler
// wrapped in front of the mux in api.New or the composition root, a path answered inside a
// catch-all's handler, the / app handler or the /api/ unrouted handler, which has no pattern to
// compare, a registration through reflection, a route an imported package such as net/http/pprof
// registers on the default mux when it is initialised, served only by a server given no handler, and
// a registration through an interface method one of whose parameter types is a type parameter. The
// mux sits in an unexported field and is embedded nowhere, so no promoted method registers a route
// unrecorded.
type recordingMux struct {
	// noCopy makes go vet's copylocks check refuse a copy of a recording mux, since a copy shares the
	// underlying mux and records its patterns where Served and ProbesServed never read them. A copy
	// whose line carries a //nolint:govet directive with a reason passes, because govet is an ordinary
	// linter, and is left to review.
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

// New builds the server. It refuses a bespoke route without a handler and a handler without a route,
// so the mounted API is exactly the registry's path and Bespoke.
func New(opts Options) (*Server, error) {
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
	k, err := newKeys(opts.TokenKey)
	if err != nil {
		return nil, err
	}
	reserved, err := Reserved(opts.Bundle)
	if err != nil {
		return nil, err
	}
	if opts.ClientSecrets == nil {
		opts.ClientSecrets = noSecrets{}
	}
	s := &Server{opts: opts, keys: k, reserved: reserved, descriptors: registry.Descriptors(opts.Datasets), metrics: m}
	a, err := newApp(opts.Bundle, opts.Browser, k)
	if err != nil {
		return nil, err
	}
	handlers := map[string]func(http.ResponseWriter, *http.Request){
		"getLens":           s.lens,
		"listAccounts":      s.listAccounts,
		"getSystem":         s.getSystem,
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
	mux.Handle("/api/", named("unrouted", func(w http.ResponseWriter, r *http.Request) {
		writeFailure(w, r, clientFault(http.StatusNotFound, "not_found", "no read API route answers this path"))
	}))
	mux.Handle("/", a)
	s.uiMux = mux
	// Every request is given its session, and every request whose method is not GET or HEAD is refused
	// before it is routed unless it carries the session's request token (ADR-0061).
	s.handler = observe(opts.Logger, withPolicy(withSession(withToken(k, mux))))

	probes := &recordingMux{mux: http.NewServeMux()}
	probes.Handle("GET /healthz", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		answer(w, r, opts.Logger, http.StatusOK, "ok")
	}))
	probes.Handle("GET /readyz", http.HandlerFunc(s.ready))
	probes.Handle("GET /metrics", promhttp.HandlerFor(opts.Metrics, promhttp.HandlerOpts{}))
	s.probesMux = probes
	s.probes = withPolicy(probes)
	return s, nil
}

// apiRoutes is the list New mounts the read API from, the registry's paths and the bespoke handlers.
// What the server actually mounts is checked against the contract through Served, so a route added
// outside this list is caught as well.
func apiRoutes(datasets []registry.Dataset) []Route {
	routes := []Route{{Pattern: registry.LensPath, Operation: "getLens", Scoped: true, Registry: true}}
	for _, d := range datasets {
		if d.Detail != nil {
			routes = append(routes, Route{Pattern: registry.RowPath(d.Name), Operation: RowOperation(d.Name), Scoped: true, Registry: true})
		}
	}
	return append(routes, Bespoke()...)
}

// Served is every pattern registered through the UI listener's recording mux, in registration order.
// The contract's test requires it to be exactly the document's operations and the two catch-alls it
// names, /api/ and /, so any other pattern registered through Handle fails the test.
func (s *Server) Served() []string {
	return slices.Clone(s.uiMux.patterns)
}

// ProbesServed is every pattern registered through the probes listener's recording mux. The contract's
// test requires it to be exactly the three probe routes, so any other pattern registered there fails
// the test.
func (s *Server) ProbesServed() []string {
	return slices.Clone(s.probesMux.patterns)
}

// Handler serves the UI, the entry document, the bundle and the read API.
func (s *Server) Handler() http.Handler { return s.handler }

// Probes serves /healthz, /readyz and /metrics, on the plain-HTTP listener apart from the UI's own.
func (s *Server) Probes() http.Handler { return s.probes }

// ready answers 200 only while the database answers as the UI's role, since every screen reads it.
func (s *Server) ready(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := s.opts.Database.Ping(ctx); err != nil {
		answer(w, r, s.opts.Logger, http.StatusServiceUnavailable, "the database does not answer")
		return
	}
	answer(w, r, s.opts.Logger, http.StatusOK, "ready")
}

// answer writes a plain-text probe answer. A failed write means the prober went away, which leaves the
// operator nothing to act on, so it is detail.
func answer(w http.ResponseWriter, r *http.Request, logger *slog.Logger, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(text + "\n")); err != nil {
		logger.DebugContext(r.Context(), "writing a probe answer failed", "error", err)
	}
}

// named records the route pattern for the log line.
func named(route string, h func(http.ResponseWriter, *http.Request)) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requestInfo(r.Context()).route = route
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
	requestInfo(r.Context()).account = account
	if account == "all" {
		writeFailure(w, r, clientFault(http.StatusBadRequest, "all_accounts", "every view is one account's, and all names every account"))
		return false
	}
	listed, err := accounts.New(s.opts.Database).Accounts(r.Context())
	if err != nil {
		s.databaseFailure(w, r, err)
		return false
	}
	if !slices.ContainsFunc(listed, func(a accounts.AccountsRow) bool { return a.AccountID == account }) {
		writeFailure(w, r, clientFault(http.StatusBadRequest, "unknown_account", "no account has this identifier"))
		return false
	}
	return true
}

// databaseFailure reports a failed read. A cancelled request is the client going away, and its
// failure is logged, never reported as the database's.
func (s *Server) databaseFailure(w http.ResponseWriter, r *http.Request, err error) {
	requestInfo(r.Context()).err = err
	if errors.Is(err, context.Canceled) {
		return
	}
	writeFailure(w, r, databaseFault())
}
