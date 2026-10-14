//go:build banproof

package probes

// This file breaks the operations endpoints' import list on purpose. The endpoints serve a registry
// and touch no database. The list for non-test code admits the data-access library, so only the
// endpoints' own list reports it.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/tx" // want depguard "list 'probes'"
)
