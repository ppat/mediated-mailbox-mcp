//go:build banproof

package release

// This file imports the conversion subsection on purpose. The release step is a pure core and takes
// the Markdown the shell converted. The API server's list and the list for non-test code both admit the
// subsection, so the pure-core list reports it, and so does the list keeping HTTP out of the
// API server's packages, which admits only what those packages import.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/content/markdown" // want depguard "import 'github.com/ppat/mediated-mailbox-mcp/content/markdown' is not allowed from list 'pure-core'" depguard "list 'api-no-http'"
)
