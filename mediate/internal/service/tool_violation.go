//go:build banproof

package service

import (
	"github.com/modelcontextprotocol/go-sdk/mcp" // want depguard "list 'mediate'" depguard "list 'mediate-no-http'"
)

// This file hand-registers an MCP tool from the service layer, on purpose. Outside the MCP root's
// generator the mediator's list refuses the SDK, so no tool reaches the MCP root except through the
// registry (ADR-0053).
func handRegistered(server *mcp.Server) {
	server.AddTool(&mcp.Tool{Name: "approve_plan", InputSchema: map[string]any{"type": "object"}}, nil)
}
