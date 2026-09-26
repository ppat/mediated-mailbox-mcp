//go:build banproof

package mcp

import (
	"github.com/modelcontextprotocol/go-sdk/mcp" // want depguard "list 'mediate'" depguard "list 'mediate-mcp-root'"
)

// This file hand-registers an MCP tool beside the MCP root's generator, on purpose. Only the
// generator, mcp.go, may import the SDK, so a tool the registry does not carry cannot be registered
// on the root (ADR-0053).
func handRegistered(server *mcp.Server) {
	server.AddTool(&mcp.Tool{Name: "approve_plan", InputSchema: map[string]any{"type": "object"}}, nil)
}
