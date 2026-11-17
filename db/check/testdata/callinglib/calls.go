// Package callinglib stands in for a library's package that names no subsection and imports another
// library's package that runs that library's statements, as the provider library's calling side
// imports the rate limiter. Its list governs this directory's files alone, so the package under it
// is no part of it. The grant check reads its imports and never builds it.
package callinglib

import _ "github.com/ppat/mediated-mailbox-mcp/fixturelib/lease"
