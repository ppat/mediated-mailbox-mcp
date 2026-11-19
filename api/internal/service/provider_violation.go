//go:build banproof

package service

// This file reaches a provider from the service layer on purpose. The service layer reaches a provider
// only through the functions api/internal/bodies hands the body operation, so the connections and the
// provider library's calling side are refused it, and so are the rate limiter, which only the
// registry's build reaches, an adapter, its pure core and the rate limiter's pure core, which no
// package of the API server reaches, each provider's consent, which only the UI's setups build, and the
// opening of a stored credential, which only the connections do (ADR-0030, ADR-0081, ADR-0117,
// ADR-0142).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open"          // want depguard "list 'api'" depguard "list 'api-no-http'"
	_ "github.com/ppat/mediated-mailbox-mcp/process/connections"      // want depguard "list 'api'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider"                 // want depguard "list 'api'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/consent"         // want depguard "list 'api'" depguard "list 'api-no-http'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail"      // want depguard "list 'api'" depguard "list 'api-no-http'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/mail/gmail/core" // want depguard "list 'api'" depguard "list 'api-no-http'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit"       // want depguard "list 'api'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"  // want depguard "list 'api'" depguard "list 'api-no-http'"
)
