//go:build banproof

package probes

// This file breaks the probe and metrics listener's import list on purpose. The listener serves a
// registry and touches no database. The list for non-test code admits the data-access library, so
// only the listener's own list reports it.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/tx" // want depguard "list 'probes'"
)
