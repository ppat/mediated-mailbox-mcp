//go:build banproof

package accountload

// This file imports a data-access subsection the account snapshot library's list does not name, on
// purpose. The rate state is the rate limiter's.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/ratestate" // want depguard "list 'accountload'"
)
