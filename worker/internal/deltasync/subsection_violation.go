//go:build banproof

package deltasync

// This file imports data-access packages its job kind's list does not name, on purpose. The checks in
// db/check are named by no component's list, and db/accountstate/completion is the other job kind's,
// whose statements this job kind's role is not granted, so each job kind's code is held to its own
// role's grants inside the one process (ADR-0118).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/completion" // want depguard "list 'worker-sync'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/check"                   // want depguard "list 'worker-sync'"
)
