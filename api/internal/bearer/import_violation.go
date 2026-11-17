//go:build banproof

package bearer

// This file reaches an operation from the bearer check on purpose. The check wraps a root's handler
// and reaches no operation, so the service layer, the registry's build, the body operation's provider
// functions and a data-access subsection are each refused (ADR-0053, ADR-0071).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/api/internal/bodies"   // want depguard "list 'api-bearer'"
	_ "github.com/ppat/mediated-mailbox-mcp/api/internal/registry" // want depguard "list 'api-bearer'"
	_ "github.com/ppat/mediated-mailbox-mcp/api/internal/service"  // want depguard "list 'api-bearer'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/messages"           // want depguard "list 'api-bearer'"
)
