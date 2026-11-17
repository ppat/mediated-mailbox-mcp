// Package reads holds the read API's read routes and their handlers: the dataset endpoint served from
// the registry, the row details, and the bespoke reads of Home, System, Jobs, a run and the accounts
// listing, with the event stream (docs/UI.md sections 17 and 18). Each route carries its handler. The
// read API endpoint, ui/internal/api, mounts them behind the checks, and hands the dataset endpoint
// and the row details the account check they run after the pure core has read the request. It holds
// no key, no consent and no code that writes.
package reads
