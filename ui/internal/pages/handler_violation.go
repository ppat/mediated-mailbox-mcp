//go:build banproof

package pages

// This file imports the setups from the pages' package on purpose. Only the read API's package mounts
// them, so the UI's list over non-test code outside the setups and the read API refuses them, and no
// page reaches a setup's code. The reads and policy management are refused here by the same list,
// proven by the UI entry's violation file, since their own violation files import the pages and an
// import back from here would make a cycle.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/ui/internal/setup" // want depguard "list 'ui-outside-setup'"
)
