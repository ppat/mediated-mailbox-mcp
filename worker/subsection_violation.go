//go:build banproof

package main

// This file imports data-access packages on purpose. The checks in db/check are named by no
// component's list, and the worker's own list names no subsection but the start check's, which the
// host runs on each job kind's pool, so any other subsection's statements run only in the worker's job
// kinds' code, each held to its own list (ADR-0117, ADR-0140, ADR-0143).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/check"           // want depguard "list 'worker'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/messages/ingest" // want depguard "list 'worker'"
)
