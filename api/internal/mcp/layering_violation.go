//go:build banproof

package mcp

// This file reaches below the service layer from a protocol root on purpose. The service layer
// import must not be reported.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/api/internal/readiness" // want depguard "list 'api-mcp-root'"
	_ "github.com/ppat/mediated-mailbox-mcp/api/internal/service"
)
