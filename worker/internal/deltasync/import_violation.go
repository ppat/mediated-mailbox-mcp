//go:build banproof

package deltasync

// This file imports another deployable's package and another job kind's code on purpose. A job kind
// reaches its own code, the worker's shared code and the shared libraries, never another job kind's
// code, though both sit in one deployable (ADR-0117).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/mediate/importtarget"              // want depguard "list 'worker-sync'"
	_ "github.com/ppat/mediated-mailbox-mcp/worker/internal/core/backfill/due" // want depguard "list 'worker-sync'"
)
