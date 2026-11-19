//go:build banproof

package provider

// This file imports on purpose the packages of the fences table of ADR-0071 that the provider
// library's calling side may not reach, which are `credential/open` and `provider/consent`. The list
// 'provider-calling' names none of them, so it refuses each here, as the annotation on each import
// shows. No other list matching this file refuses them, so this file is what proves each fence here.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"  // want depguard "list 'provider-calling'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent" // want depguard "list 'provider-calling'"
)
