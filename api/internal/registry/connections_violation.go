//go:build banproof

package registry

// This file reaches the connections from the registry's build on purpose. Only api/internal/bodies
// begins a unit of work, so the connections are refused here (ADR-0142).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections" // want depguard "list 'api-registry'"
)
