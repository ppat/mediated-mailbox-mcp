//go:build banproof

package app_test

// This file opens a stored credential from the API server's tests on purpose. The tests seal the
// credentials they store and never open one, so the opening half of the credential family is refused
// in every file of the API server (ADR-0081).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open" // want depguard "list 'api-tests'"
)
