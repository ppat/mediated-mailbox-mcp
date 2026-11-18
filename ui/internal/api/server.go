package api

import (
	"errors"
	"fmt"
	"net/http"
	"slices"

	"github.com/ppat/mediated-mailbox-mcp/db/accounts"
	"github.com/ppat/mediated-mailbox-mcp/db/tx"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/refusal"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/paths"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// Database is the UI's connection pool, connected as the application user (ADR-0143). The read API
// endpoint admits a scoped request's account through the accounts listing, which it reads on the pool
// directly because the listing reads every account (ADR-0091), and hands the pool to reads, setup and
// policymanagement, each of which states what it uses of it.
type Database interface {
	tx.Beginner
	tx.BeginnerWithOptions
	accounts.DBTX
}

// endpoint is the read API, its recording mux behind the checks.
type endpoint struct {
	// uiMux is kept, not its patterns, so Served reads every pattern registered through Handle, whenever
	// it was registered.
	uiMux   *recordingMux
	handler http.Handler
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

// mount mounts routes, those of reads, setup and policymanagement in the order the read API serves
// them, on the read API's recording mux behind checks. It refuses a route without a handler and a
// bespoke route claiming a path the registry claims or another bespoke route's, so the mounted API is
// exactly the registry's paths and the bespoke routes. A scoped bespoke route admits its account
// through db before its handler sees the request.
func mount(db accounts.DBTX, checks *server.Checks, datasets []registry.Dataset, routes ...[]server.Route) (*endpoint, error) {
	var all, bespoke []server.Route
	for _, r := range routes {
		all = append(all, r...)
	}
	for _, route := range all {
		if !route.Registry {
			bespoke = append(bespoke, route)
		}
	}
	if err := CheckPaths(registry.Paths(datasets), bespoke); err != nil {
		return nil, err
	}
	if problems := registry.Check(datasets); len(problems) > 0 {
		return nil, fmt.Errorf("the registry's statements do not match its declarations: %v", problems)
	}
	if checks == nil {
		return nil, errors.New("the read API was given no checks to sit behind")
	}
	mux := &recordingMux{mux: http.NewServeMux()}
	for _, route := range all {
		h := route.Handler
		if h == nil {
			return nil, fmt.Errorf("the route %s has no handler for %s", route.Pattern, route.Operation)
		}
		pattern := route.Verb() + " " + route.Pattern
		switch {
		case route.Registry:
			// The dataset endpoint and the row-detail routes admit their account themselves, after the
			// pure core has read the request, so a request the registry does not declare is refused
			// before any statement runs.
			mux.Handle(pattern, named(route.Pattern, h))
		case route.Scoped:
			mux.Handle(pattern, scoped(db, route.Pattern, h))
		default:
			mux.Handle(pattern, named(route.Pattern, h))
		}
	}
	mux.Handle(paths.API+"/", named("unrouted", func(w http.ResponseWriter, r *http.Request) {
		server.WriteFailure(w, r, server.ClientFault(refusal.NotFound, "no read API route answers this path"))
	}))
	// Every request is given its session, and every request whose method is not GET or HEAD is refused
	// before it is routed unless it carries the session's request token (ADR-0061).
	return &endpoint{uiMux: mux, handler: checks.Wrap(mux)}, nil
}

// Served is every pattern registered through the read API's recording mux, in registration order.
// The test of the read API's routes requires it to be exactly the contract document's operations and
// the read API's catch-all, /ui/api/, so any other pattern registered through Handle fails the test.
func (e *endpoint) Served() []string {
	return slices.Clone(e.uiMux.patterns)
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
func scoped(db accounts.DBTX, route string, h func(http.ResponseWriter, *http.Request)) http.Handler {
	return named(route, func(w http.ResponseWriter, r *http.Request) {
		if admit(db, w, r) {
			h(w, r)
		}
	})
}

// admit reports whether the request's account may be read, and answers the request when it may not.
// The read API hands it to reads, whose dataset endpoint and row details run it after the pure core has
// read the request.
func admit(db accounts.DBTX, w http.ResponseWriter, r *http.Request) bool {
	account := r.PathValue("account")
	server.RequestLogOf(r.Context()).Account = account
	if account == "all" {
		server.WriteFailure(w, r, server.ClientFault(refusal.AllAccounts, "every view is one account's, and all names every account"))
		return false
	}
	listed, err := accounts.New(db).Accounts(r.Context())
	if err != nil {
		server.DatabaseFailure(w, r, err)
		return false
	}
	if !slices.ContainsFunc(listed, func(a accounts.AccountsRow) bool { return a.AccountID == account }) {
		server.WriteFailure(w, r, server.ClientFault(refusal.UnknownAccount, "no account has this identifier"))
		return false
	}
	return true
}

// admission is the read API's account check over db, as the read routes are handed it.
func admission(db accounts.DBTX) func(w http.ResponseWriter, r *http.Request) bool {
	return func(w http.ResponseWriter, r *http.Request) bool { return admit(db, w, r) }
}
