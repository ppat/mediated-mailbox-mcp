//go:build banproof

package release

// This file imports the conversion subsection on purpose. The release step is a pure core and takes
// the Markdown the shell converted. The mediator's list and the list for non-test code both admit the
// subsection, so the pure-core list reports it, and so does the list keeping HTTP out of the
// mediator's packages, which admits only what those packages import.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/sanitize/markdown" // want depguard "import 'github.com/ppat/mediated-mailbox-mcp/sanitize/markdown' is not allowed from list 'pure-core'" depguard "list 'mediate-no-http'"
)
