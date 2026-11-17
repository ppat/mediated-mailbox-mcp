//go:build banproof

package provider

// This file breaks the calling side's import list on purpose. The first import names a third-party
// package no list over shipped code admits, and the second the Gmail adapter's pure core, which only
// the adapter may import (ADR-0071).
import (
	_ "golang.org/x/tools/go/packages" // want depguard "list 'provider-calling'" depguard "list 'non-test-code'"

	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'provider-calling'"
)
