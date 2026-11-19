//go:build banproof

package mcp

import (
	"net/http" // want depguard "list 'api-mcp-root'"
)

// This file hand-registers an MCP endpoint beside the MCP root's generator, on purpose. Only the
// generator, mcp.go, may serve HTTP, so an operation the registry does not carry cannot be added to
// the root (ADR-0053).
func routeRegistered(mux *http.ServeMux) {
	mux.HandleFunc("POST /mcp/approve_plan", func(http.ResponseWriter, *http.Request) {})
}
