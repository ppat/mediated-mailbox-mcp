// Package paths holds where the UI serves everything, under its one root, beside the client surface's
// /api and /mcp (ADR-0137, docs/UI.md section 5). It is the only place the server writes the root, so
// the read API's routes, the bundle's files, the cookies' scope and the page addresses the server
// writes into its answers are all built from it.
package paths

import "net/url"

// Root is the UI's root. Every route the UI's server registers sits under Root + "/", apart from /,
// which it answers with a redirect to the root.
const Root = "/ui"

// API is the read API's root (docs/UI.md section 17).
const API = Root + "/api"

// Accounts is the read API's accounts list, and the prefix of every account-scoped read API path,
// which continues with the account as one path segment.
const Accounts = API + "/accounts"

// Setup is the prefix of the installation's read API paths, which belong to no account.
const Setup = API + "/setup"

// Account is the address of an account's home page, under which every page of the account sits, the
// account escaped as one path segment (docs/UI.md section 5).
func Account(account string) string {
	return Root + "/accounts/" + url.PathEscape(account)
}

// Page is the page address of screen under account, with its query. screen is the path below the
// account, any segment the caller took from data already escaped.
func Page(account, screen string, query url.Values) string {
	u := url.URL{Path: Root + "/accounts/" + account + "/" + screen, RawQuery: query.Encode()}
	return u.String()
}
