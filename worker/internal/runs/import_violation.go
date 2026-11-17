//go:build banproof

package runs

// This file imports another deployable's package, each job kind's code, and data-access subsections
// its list does not name, on purpose. The run records both job kinds share reach neither job kind's
// code, so neither job kind reaches the other's through them, and they write only the runs' records,
// the index being worker/internal/index's and the completion flags a job kind's own (ADR-0117,
// ADR-0143). A job kind's code it imports is a pure core that imports nothing of the runs' records,
// since a package that does would make a cycle with this file.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/api/importtarget"               // want depguard "list 'worker-runs'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"     // want depguard "list 'worker-runs'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/messages/scan"               // want depguard "list 'worker-runs'"
	_ "github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/core"  // want depguard "list 'worker-runs'"
	_ "github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/core" // want depguard "list 'worker-runs'"
	_ "github.com/ppat/mediated-mailbox-mcp/worker/internal/index"          // want depguard "list 'worker-runs'"
)
