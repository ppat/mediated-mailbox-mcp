//go:build banproof

package api

// This file reaches below the service layer from a protocol root on purpose. The service layer
// import must not be reported.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/mediate/internal/readiness" // want depguard "list 'mediate-api-root'"
	_ "github.com/ppat/mediated-mailbox-mcp/mediate/internal/service"
)
