//go:build banproof

package sessions

// This file breaks the Session's import list on purpose. The Session imports no credential and no
// provider code, so the opening half of the credential family and the provider library's calling
// side are refused. The connections' own package imports this one, so the compiler refuses the import
// back as a cycle (ADR-0071, ADR-0117, ADR-0142).
import (
	_ "github.com/ppat/mediated-mailbox-mcp/credential/open" // want depguard "list 'sessions'"
	_ "github.com/ppat/mediated-mailbox-mcp/provider"        // want depguard "list 'sessions'"
)
