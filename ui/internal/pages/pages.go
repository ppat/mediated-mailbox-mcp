// Package pages serves the UI's pages, the entry document and the browser bundle ui/browser embeds,
// under the UI's root, and the redirect that answers / with the root (ADR-0137). Its two endpoints,
// which the UI's spec lists at /ui/ and /{$}, each sit behind the checks every endpoint of the UI sits
// behind (ADR-0061, ADR-0062, ADR-0142).
package pages

import (
	"context"
	"net/http"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
	"github.com/ppat/mediated-mailbox-mcp/ui/browser"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/paths"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// New builds the pages endpoint, which serves, under the UI's root, a file of the embedded bundle when
// the path names one, and otherwise the entry document carrying the session's request token and the
// configuration the browser reads.
func New(ctx context.Context, p process.Capabilities, c config.UI) (http.Handler, error) {
	bundle, err := browser.Bundle()
	if err != nil {
		return nil, err
	}
	checks, err := server.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	a, err := newApp(bundle, browserConfig{
		DefaultTheme: c.DefaultTheme, StreamReconnectMax: time.Duration(c.StreamReconnectMax),
		StreamPollInterval: time.Duration(c.StreamPollInterval), ConsentRedirect: c.ConsentRedirect,
	}, checks)
	if err != nil {
		return nil, err
	}
	return checks.Wrap(a), nil
}

// Redirect builds the endpoint that answers /, the one path the UI answers outside its root, with a
// redirect to the root (ADR-0137).
func Redirect(ctx context.Context, p process.Capabilities, c config.UI) (http.Handler, error) {
	checks, err := server.Get(ctx, p, c)
	if err != nil {
		return nil, err
	}
	return checks.Wrap(http.HandlerFunc(redirect)), nil
}

// redirect answers a GET or a HEAD with a redirect to the UI's root, and any other method with 405,
// which only a request carrying its session's token reaches, since the checks refuse one without it
// first.
func redirect(w http.ResponseWriter, r *http.Request) {
	server.RequestLogOf(r.Context()).Route = "root"
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	http.Redirect(w, r, paths.Root+"/", http.StatusFound)
}
