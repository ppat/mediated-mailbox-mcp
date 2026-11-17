package server

import (
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/core/schema"
)

// Route is one route of the read API as the contract describes it, carrying the handler that serves
// it.
type Route struct {
	// Method is the route's method, GET for a read when it is empty. A route of any other method
	// changes state, and the checks refuse it without the session's request token before its handler
	// sees it (ADR-0061).
	Method    string
	Pattern   string
	Operation string
	Summary   string
	// Scoped says the path carries the account as its {account} segment.
	Scoped bool
	// Registry marks a route the registry claims, the dataset endpoint and a row detail, which admits
	// its account after the pure core reads the request. No bespoke route sets it.
	Registry bool
	// Request is the JSON body a state-changing route reads, with no other field admitted. A route
	// that reads no body has none.
	Request *schema.Type
	// Response is the JSON body of a 200 response. A stream route has none.
	Response schema.Type
	// Events are the event stream's data objects, one per event name, for a stream route.
	Events map[string]schema.Type
	// Query names the optional query parameters a read takes.
	Query []string
	// Download is the media type of a route that answers with a file to save rather than JSON.
	Download string
	// Refusal is the body a refused request's 400 carries when it names problems beyond the error
	// contract's, as a refused policy write or file does.
	Refusal *schema.Type
	// Handler serves the route. The read API endpoint mounts it at the route's method and pattern, so
	// no list apart from the routes binds an operation to the function that serves it.
	Handler http.HandlerFunc
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
