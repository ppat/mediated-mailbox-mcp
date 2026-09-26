// Package oauthclients is the data-access subsection for the oauth_clients table, which holds an
// installation's OAuth client for each provider that authenticates through one, keyed on the
// provider, its secret sealed (ADR-0016, ADR-0080, ADR-0083). The table belongs to no account, and a
// provider that authenticates otherwise has no row.
package oauthclients
