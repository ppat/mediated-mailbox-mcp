package api

import (
	"fmt"
	"slices"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// Route is one bespoke handler as the contract describes it. Every read route is a GET.
type Route struct {
	Pattern   string
	Operation string
	Summary   string
	// Scoped says the path carries the account as its {account} segment.
	Scoped bool
	// Registry marks a route the registry claims, which admits its account after the pure core reads
	// the request. No bespoke route sets it.
	Registry bool
	// Response is the JSON body of a 200 response. A stream route has none.
	Response schema.Type
	// Events are the event stream's data objects, one per event name, for a stream route.
	Events map[string]schema.Type
}

// Stream reports whether the route answers with an event stream.
func (r Route) Stream() bool { return len(r.Events) > 0 }

// Bespoke is the list of bespoke handlers, the second of the two sources the contract is generated
// from (ADR-0057). A handler exists only where a screen needs a shape the ladder does not produce, and
// the server mounts exactly these and the dataset endpoint under /api. Each route's handler is bound
// by its operation in New, which refuses a route without one and a handler without a route.
func Bespoke() []Route {
	return []Route{
		{
			Pattern: "/api/accounts", Operation: "listAccounts",
			Summary:  "Every account's identifier and provider, the one unscoped read",
			Response: accountsType(),
		},
		{
			Pattern: "/api/{account}/system", Operation: "getSystem", Scoped: true,
			Summary:  "The account's operational, corpus and decisions blocks",
			Response: systemType(),
		},
		{
			Pattern: "/api/{account}/jobs", Operation: "getJobs", Scoped: true,
			Summary:  "One block per workload, the rate block and the cadences",
			Response: jobsType(),
		},
		{
			Pattern: "/api/{account}/jobs/{run}", Operation: "getRun", Scoped: true,
			Summary:  "One run, its resumer, its item failures by disposition and the runs that recovered them, and its timeline",
			Response: runSummaryType(),
		},
		{
			Pattern: "/api/{account}/events", Operation: "streamEvents", Scoped: true,
			Summary: "The live stream, one event per changed object with its whole current state",
			Events:  eventTypes(),
		},
	}
}

// CheckPaths refuses a bespoke route claiming a path the registry claims, and two bespoke routes
// claiming one path, so no route is served by two definitions (docs/UI.md section 17.1). The contract
// generator and the server both run it.
func CheckPaths(registryPaths []string, bespoke []Route) error {
	seen := map[string]string{}
	for _, route := range bespoke {
		if slices.Contains(registryPaths, route.Pattern) {
			return fmt.Errorf("the path %s is claimed by both the registry and the bespoke handler %s", route.Pattern, route.Operation)
		}
		if prior, ok := seen[route.Pattern]; ok {
			return fmt.Errorf("the path %s is claimed by two bespoke handlers, %s and %s", route.Pattern, prior, route.Operation)
		}
		seen[route.Pattern] = route.Operation
	}
	return nil
}

// RowOperation is the contract's operation for a dataset's row-detail route.
func RowOperation(dataset string) string {
	return "get" + pascal(dataset) + "Row"
}
