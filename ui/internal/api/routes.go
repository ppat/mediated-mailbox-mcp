package api

import (
	"fmt"
	"net/http"
	"slices"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// Route is one bespoke handler as the contract describes it.
type Route struct {
	// Method is the route's method, GET for a read when it is empty. A route of any other method
	// changes state, and the server refuses it without the session's request token before routing it
	// (ADR-0061).
	Method    string
	Pattern   string
	Operation string
	Summary   string
	// Scoped says the path carries the account as its {account} segment.
	Scoped bool
	// Registry marks a route the registry claims, which admits its account after the pure core reads
	// the request. No bespoke route sets it.
	Registry bool
	// Request is the JSON body a state-changing route reads, with no other field admitted. A route
	// that reads no body has none.
	Request *schema.Type
	// Response is the JSON body of a 200 response. A stream route has none.
	Response schema.Type
	// Events are the event stream's data objects, one per event name, for a stream route.
	Events map[string]schema.Type
}

// Stream reports whether the route answers with an event stream.
func (r Route) Stream() bool { return len(r.Events) > 0 }

// Verb is the route's method, GET when Method is empty.
func (r Route) Verb() string {
	if r.Method == "" {
		return http.MethodGet
	}
	return r.Method
}

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
			Pattern: "/api/{account}/attention", Operation: "getAttention", Scoped: true,
			Summary:  "Home's worth-a-look cards, each worded, in their order",
			Response: attentionType(),
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
		{
			Pattern: "/api/setup", Operation: "getInstallation",
			Summary:  "Each OAuth client's identity and accounts, and every account's identifier, provider and client, reading no account's state",
			Response: installationType(),
		},
		{
			Method: http.MethodPost, Pattern: "/api/setup/{provider}/clients", Operation: "addClient",
			Summary: "Checks a new OAuth client with its provider and stores it under its name, its secret sealed",
			Request: ptr(addClientType()), Response: clientAnswerType(),
		},
		{
			Method: http.MethodPost, Pattern: "/api/setup/{provider}/clients/{client}", Operation: "replaceClient",
			Summary: "Checks and stores the named client's identifier and secret, or its secret alone",
			Request: ptr(replaceClientType()), Response: clientAnswerType(),
		},
		{
			Method: http.MethodPost, Pattern: "/api/setup/{provider}/clients/{client}/remove", Operation: "removeClient",
			Summary:  "Removes a client no account connects through",
			Response: removedType(),
		},
		{
			Method: http.MethodPost, Pattern: "/api/setup/connect", Operation: "startConnect",
			Summary: "Starts the session's consent attempt to connect an account, replacing any it held",
			Request: ptr(connectType()), Response: attemptAnswerType(),
		},
		{
			Pattern: "/api/setup/connect", Operation: "getConnect",
			Summary:  "The session's attempt to connect an account, or none",
			Response: attemptAnswerType(),
		},
		{
			Method: http.MethodPost, Pattern: "/api/setup/connect/finish", Operation: "finishConnect",
			Summary: "Finishes the session's attempt from the pasted address and writes the account's two rows",
			Request: ptr(finishType()), Response: finishAnswerType(),
		},
		{
			Pattern: "/api/{account}/account", Operation: "getAccount", Scoped: true,
			Summary:  "What the UI reads of the account's two rows and its rate state, never the credential",
			Response: accountType(),
		},
		{
			Method: http.MethodPost, Pattern: "/api/{account}/account/target", Operation: "setTarget", Scoped: true,
			Summary: "Stores the account's lowered target, or clears it",
			Request: ptr(targetType()), Response: targetAnswerType(),
		},
		{
			Method: http.MethodPost, Pattern: "/api/{account}/account/reauthorize", Operation: "startReauthorize", Scoped: true,
			Summary: "Starts the session's consent attempt to re-authorize the account, or move it to another client",
			Request: ptr(reauthorizeType()), Response: attemptAnswerType(),
		},
		{
			Pattern: "/api/{account}/account/reauthorize", Operation: "getReauthorize", Scoped: true,
			Summary:  "The session's attempt to re-authorize the account, or none",
			Response: attemptAnswerType(),
		},
		{
			Method: http.MethodPost, Pattern: "/api/{account}/account/reauthorize/finish", Operation: "finishReauthorize", Scoped: true,
			Summary: "Finishes the session's attempt from the pasted address and replaces the account's credential",
			Request: ptr(finishType()), Response: finishAnswerType(),
		},
	}
}

func ptr(t schema.Type) *schema.Type { return &t }

// CheckPaths refuses a bespoke route claiming a path the registry claims, and two bespoke routes
// claiming one path under one method, so no route is served by two definitions (docs/UI.md section
// 17.1). The contract generator and the server both run it.
func CheckPaths(registryPaths []string, bespoke []Route) error {
	seen := map[string]string{}
	for _, route := range bespoke {
		if slices.Contains(registryPaths, route.Pattern) {
			return fmt.Errorf("the path %s is claimed by both the registry and the bespoke handler %s", route.Pattern, route.Operation)
		}
		key := route.Verb() + " " + route.Pattern
		if prior, ok := seen[key]; ok {
			return fmt.Errorf("the path %s is claimed by two bespoke handlers, %s and %s", key, prior, route.Operation)
		}
		seen[key] = route.Operation
	}
	return nil
}

// RowOperation is the contract's operation for a dataset's row-detail route.
func RowOperation(dataset string) string {
	return "get" + pascal(dataset) + "Row"
}
