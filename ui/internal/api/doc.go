// Package api is the UI's HTTP server. It serves the entry document, the embedded browser bundle and
// the read API under /api, every response under the content security policy (ADR-0062). The read API
// is the dataset endpoint, served from the registry, and the bespoke handlers of Bespoke. Those two
// are the only sources of an API route, and the contract document is generated from them (ADR-0057,
// ADR-0065), so no API route exists that the contract does not describe.
package api
