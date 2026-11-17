//go:build banproof

package release

// This file imports the conversion subsection on purpose. The release step is a pure core and takes
// the Markdown the shell converted. The pure-core list reports it, and so does the API server's list,
// which admits it to no package of the API server but the registry's build.
import (
	_ "github.com/ppat/mediated-mailbox-mcp/content/markdown" // want depguard "import 'github.com/ppat/mediated-mailbox-mcp/content/markdown' is not allowed from list 'pure-core'" depguard "list 'api'"
)
