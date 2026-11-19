//go:build banproof

package policymanagement

// This file imports, from policy management's package, the setups and the pages on purpose. Only the
// read API's package mounts the setups, so the UI's list over non-test code outside the setups and the
// read API refuses them, and policy management's own list, which names no other handler package,
// refuses both. So no policy handler reaches a setup or a page. The reads are refused here by the same
// two lists, and no violation file proves it, since the reads' violation file imports this package
// and an import back would make a cycle.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/pages" // want depguard "list 'ui-policymanagement'"
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/setup" // want depguard "list 'ui-outside-setup'" depguard "list 'ui-policymanagement'"
)
