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
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
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
	// StreamInterval is how often the event stream polls the recorded state (ADR-0058).
	StreamInterval time.Duration
}

// Server is the UI's server.
type Server struct {
	opts        Options
	descriptors []lens.Descriptor
	metrics     *metrics
	handler     http.Handler
	probes      http.Handler
	served      []string
}

// recordingMux is the UI's mux. A route registered through its Handle is recorded and compared with
// the contract. A direct call on the underlying mux inside package api, and a handler wrapped in
// front of the mux in api.New or the composition root, are left to review. The mux sits in an
// unexported field and is embedded nowhere, so no promoted method registers a route unrecorded.
type recordingMux struct {
	mux      *http.ServeMux
	patterns []string
}

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
	if err := CheckPaths(Bespoke()); err != nil {
		return nil, err
	}
	if problems := registry.Check(opts.Datasets); len(problems) > 0 {
		return nil, fmt.Errorf("the registry's statements do not match its declarations: %v", problems)
	}
	m, err := newMetrics(opts.Metrics)
	if err != nil {
		return nil, fmt.Errorf("registering the metrics: %w", err)
	}
	s := &Server{opts: opts, descriptors: registry.Descriptors(opts.Datasets), metrics: m}
	a, err := newApp(opts.Bundle)
	if err != nil {
		return nil, err
	}
	handlers := map[string]func(http.ResponseWriter, *http.Request){
		"getLens":      s.lens,
		"listAccounts": s.listAccounts,
		"getSystem":    s.getSystem,
		"getJobs":      s.getJobs,
		"streamEvents": s.streamEvents,
	}
	mux := &recordingMux{mux: http.NewServeMux()}
	for _, route := range apiRoutes() {
		h, ok := handlers[route.Operation]
		if !ok {
			return nil, fmt.Errorf("the route %s has no handler for %s", route.Pattern, route.Operation)
		}
		delete(handlers, route.Operation)
		switch {
		case route.Pattern == registry.LensPath:
			// The dataset endpoint admits its account itself, after the pure core has read the request,
			// so a request the registry does not declare is refused before any statement runs.
			mux.Handle("GET "+route.Pattern, named(route.Pattern, h))
		case route.Scoped:
			mux.Handle("GET "+route.Pattern, s.scoped(route.Pattern, h))
		default:
			mux.Handle("GET "+route.Pattern, named(route.Pattern, h))
		}
	}
	if len(handlers) > 0 {
		return nil, fmt.Errorf("handlers with no route: %v", slices.Sorted(maps.Keys(handlers)))
	}
	mux.Handle("/api/", named("unrouted", func(w http.ResponseWriter, r *http.Request) {
		writeFailure(w, r, clientFault(http.StatusNotFound, "not_found", "no read API route answers this path"))
	}))
	mux.Handle("/", a)
	s.served = mux.patterns
	s.handler = observe(opts.Logger, withPolicy(mux))

	probes := http.NewServeMux()
	probes.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		answer(w, r, http.StatusOK, "ok")
	})
	probes.HandleFunc("GET /readyz", s.ready)
	probes.Handle("GET /metrics", promhttp.HandlerFor(opts.Metrics, promhttp.HandlerOpts{}))
	s.probes = withPolicy(probes)
	return s, nil
}

// apiRoutes is the list New mounts the read API from, the registry's path and the bespoke handlers.
// What the server actually mounts is checked against the contract through Served, so a route added
// outside this list is caught as well.
func apiRoutes() []Route {
	return append([]Route{{Pattern: registry.LensPath, Operation: "getLens", Scoped: true}}, Bespoke()...)
}

// Served are the read API's routes the server mounted, as "METHOD /path" patterns, apart from the
// catch-all that refuses every other path under /api. The contract's test requires them to equal the
// document's operations.
func (s *Server) Served() []string {
	var out []string
	for _, p := range s.served {
		if strings.Contains(p, "/api/") && p != "/api/" {
			out = append(out, p)
		}
	}
	return out
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
		answer(w, r, http.StatusServiceUnavailable, "the database does not answer")
		return
	}
	answer(w, r, http.StatusOK, "ready")
}

// answer writes a plain-text probe answer. A failed write means the prober went away.
func answer(w http.ResponseWriter, r *http.Request, status int, text string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	if _, err := w.Write([]byte(text + "\n")); err != nil {
		slog.DebugContext(r.Context(), "writing a probe answer failed", "error", err)
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
