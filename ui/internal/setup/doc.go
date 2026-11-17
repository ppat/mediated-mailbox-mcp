// Package setup holds the read API's setup routes and their handlers: the installation's read, the
// OAuth client setup, connecting an account and re-authorizing or moving it, and account settings, its
// read and its lowered target (docs/UI.md sections 8.10 to 8.13, ADR-0084). It writes the account's
// rows. Each route carries its handler, and the read API endpoint, ui/internal/api, mounts them behind
// the checks.
//
// It builds the public key credentials and client secrets are sealed to and each provider's consent,
// and neither leaves it. Its import list refuses the opening half of the credential code. It reaches a
// stored client's secret only through its one operation, a client's secret by its name, which
// ui/internal/setup/clientsecret, a package of its own and the one part of the UI that opens a stored
// value, provides (ADR-0081, ADR-0143).
package setup
