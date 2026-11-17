package pages

import (
	"io/fs"
	"net/http"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// BrowserConfig is the configuration the entry document hands the browser.
type BrowserConfig = browserConfig

// NewApp builds the pages handler over bundle, as New builds it over the embedded bundle and the
// configuration, unwrapped by the checks. It exists only in this package's test binary.
func NewApp(bundle fs.FS, b BrowserConfig, checks *server.Checks) (http.Handler, error) {
	return newApp(bundle, b, checks)
}

// RedirectHandler is the handler Redirect puts behind the checks. It exists only in this package's
// test binary.
var RedirectHandler = http.HandlerFunc(redirect)
