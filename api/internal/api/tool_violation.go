//go:build banproof

package api

import (
	"github.com/modelcontextprotocol/go-sdk/mcp" // want depguard "list 'api'" depguard "list 'api-api-root'"
)

// This file registers an MCP tool from the API root on purpose. Only the MCP root's generator, mcp.go,
// may import the SDK, so the API root cannot serve a tool the MCP root does not (ADR-0053, ADR-0086).
func toolRegistered(server *mcp.Server) {
	server.AddTool(&mcp.Tool{Name: "approve_plan", InputSchema: map[string]any{"type": "object"}}, nil)
}
