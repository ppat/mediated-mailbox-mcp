//go:build banproof

package backfill

// This file imports packages its job kind's list does not name, on purpose. The checks in db/check
// are named by no component's list. db/oauthclients/secret is the other job kind's. The OAuth client
// setup and the policy's writes are the UI's, and the sealed credential's statements are the
// connections'. Every job kind connects as the one application user, whose grants cannot tell one job
// kind's code from another's or from the UI's, so each job kind's list is what holds its code to its own
// statements (ADR-0117, ADR-0143).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/accountstate/credential" // want depguard "list 'worker-backfill'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/check"                   // want depguard "list 'worker-backfill'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/oauthclients/secret"     // want depguard "list 'worker-backfill'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/oauthclients/setup"      // want depguard "list 'worker-backfill'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/policychanges"           // want depguard "list 'worker-backfill'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/policyrules/base"        // want depguard "list 'worker-backfill'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage"      // want depguard "list 'worker-backfill'"
)
