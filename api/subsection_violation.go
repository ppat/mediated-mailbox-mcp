//go:build banproof

package main

// This file imports data-access packages the component's list does not name, on purpose. The checks in
// db/check are named by no component's list. The OAuth client setup and the policy's writes are the
// UI's, and the API server connects as the same application user, whose grants cannot tell its code
// from the UI's, so its list is what keeps them from its code (ADR-0084, ADR-0143).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/check"              // want depguard "list 'api'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/oauthclients/setup" // want depguard "list 'api'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/policychanges"      // want depguard "list 'api'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/policyrules/base"   // want depguard "list 'api'"
	_ "github.com/ppat/mediated-mailbox-mcp/db/policyrules/manage" // want depguard "list 'api'"
)
