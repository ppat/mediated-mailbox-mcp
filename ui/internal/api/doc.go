// Package api is the UI's read API endpoint, which the UI's spec lists at /ui/api/ and New builds
// (ADR-0137, ADR-0142). It serves the read API behind the checks every endpoint of the UI sits
// behind, the content security policy among them (ADR-0062). The read API is the dataset endpoint,
// served from the registry, and the bespoke handlers of Bespoke. Those two are the only sources of
// an API route, and the contract document is generated from them (ADR-0057, ADR-0065), so no API
// route exists that the contract does not describe. The entry document, the bundle and the redirect
// at / are ui/internal/pages'.
package api
