// Package app is the API server's entry package, which holds its spec and nothing else (ADR-0142).
// The spec lists the API server's start check, mcp.Init, which refuses MCPGODEBUG (ADR-0086), and its
// two endpoints, the API root under /api/ and the MCP root at /mcp, each built by its own root's Build
// from the process's one registry of operations and behind the bearer check (ADR-0030, ADR-0087).
// The process starts the API server as it starts every program, serves the two endpoints over TLS
// when the configuration names a key pair and over plain HTTP when it names neither (ADR-0127), and
// answers any other path as not found. The API server is ready once its Session has loaded the
// accounts, their connections and their policy, and while its bearer token file holds a token.
package app

import (
	"github.com/ppat/mediated-mailbox-mcp/api/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/api/internal/mcp"
	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

// Spec is the API server's spec.
func Spec() process.Spec[config.API] {
	return process.Spec[config.API]{
		Name: "api",
		Init: mcp.Init,
		Endpoints: []process.Endpoint[config.API]{
			{Pattern: "/api/", Build: api.Build},
			{Pattern: "/mcp", Build: mcp.Build},
		},
	}
}
