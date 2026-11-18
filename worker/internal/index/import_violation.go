//go:build banproof

package index

// This file imports another deployable's package, each job kind's code, and data-access subsections
// its list does not name, on purpose. The index writes both job kinds share reach neither job kind's
// code, so neither job kind reaches the other's through them, and they write only through the
// subsections both job kinds' lists name: the run records are worker/internal/runs's, and the change
// cursor, a change's labels and removals and the completion flags are a job kind's own (ADR-0117,
// ADR-0143). A job kind's code it imports is a pure core that imports nothing of the index, since a
// package that does would make a cycle with this file.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/api/importtarget"               // want depguard "list 'worker-index'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion"     // want depguard "list 'worker-index'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/cursor"         // want depguard "list 'worker-index'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/jobruns/record"              // want depguard "list 'worker-index'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/messages/change"             // want depguard "list 'worker-index'"
	_ "github.com/ppat/mediated-mailbox-mcp/worker/internal/backfill/core"  // want depguard "list 'worker-index'"
	_ "github.com/ppat/mediated-mailbox-mcp/worker/internal/deltasync/core" // want depguard "list 'worker-index'"
)
