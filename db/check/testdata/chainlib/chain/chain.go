// Package chain stands in for a package of a second library that runs a subsection's statements of
// its own and imports the first library's package that runs that library's statements. The grant
// check reads its imports and never builds it.
package chain

import (
	_ "github.com/ppat/mediated-mailbox-mcp/db/listing"
	_ "github.com/ppat/mediated-mailbox-mcp/fixturelib/lease"
)
