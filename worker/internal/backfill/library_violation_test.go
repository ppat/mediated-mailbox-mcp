//go:build banproof

package backfill_test

// This file imports the opening code from the job kind's tests on purpose. The job kind's tests store
// the credentials they need sealed and never open one, so the job kind's list over its test files
// refuses the opening code as its list over its non-test files does, and no test of the job kind links
// it (ADR-0071, ADR-0143).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open" // want depguard "list 'worker-backfill-tests'"
)
