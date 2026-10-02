package api

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
	"time"
)

// BundleEntry is the bundle's entry module, the file bun build writes for src/app/main.ts.
const BundleEntry = "main.js"

// BundleStylesheet is the bundle's one stylesheet, which bun build writes beside the entry module from
// the stylesheet src/app/main.ts imports.
const BundleStylesheet = "main.css"

// entryTemplate is the entry document. It is the one page a Go handler renders, on every page load, so
// it carries the session's request token (ADR-0061). It holds no inline script and
// names no other origin, and the policy's tests read it (ADR-0062, ADR-0064).
const entryTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Mediated Mailbox</title>
<meta name="mediated-mailbox.default_theme" content="{{.Browser.DefaultTheme}}">
<meta name="mediated-mailbox.stream_reconnect_max" content="{{.ReconnectMax}}">
<meta name="mediated-mailbox.stream_poll_interval" content="{{.PollInterval}}">
<meta name="mediated-mailbox.consent_redirect" content="{{.Browser.ConsentRedirect}}">
<meta name="mediated-mailbox.request_token" content="{{.Token}}">
<link rel="stylesheet" href="/{{.Stylesheet}}">
<script type="module" src="/{{.Entry}}"></script>
</head>
<body></body>
</html>
`

// EntryTemplate returns the entry document's template text, which the policy's tests scan.
func EntryTemplate() string { return entryTemplate }

// Browser is the configuration the browser reads, rendered into the entry document as meta tags, one
// per key, with durations in whole milliseconds (docs/UI.md section 18.1).
type Browser struct {
	DefaultTheme       string
	StreamReconnectMax time.Duration
	StreamPollInterval time.Duration
	// ConsentRedirect is the loopback address a consent redirects to, which the connect page pictures
	// and checks a pasted address against.
	ConsentRedirect string
}

type entryData struct {
	Entry        string
	Stylesheet   string
	Browser      Browser
	ReconnectMax int64
	PollInterval int64
	// Token is the request token of the session the page loads in, which every state-changing request
	// the page sends carries.
	Token string
}

// app serves a bundle file when the path names one, and otherwise the entry document, so every
// screen route the browser's router owns loads the app.
type app struct {
	bundle   fs.FS
	files    http.Handler
	template *template.Template
	browser  Browser
	keys     keys
}

func newApp(bundle fs.FS, browser Browser, k keys) (*app, error) {
	t, err := template.New("entry").Parse(entryTemplate)
	if err != nil {
		return nil, err
	}
	return &app{bundle: bundle, files: http.FileServerFS(bundle), template: t, browser: browser, keys: k}, nil
}

// Reserved is every word the UI's own top-level paths use, which no account identifier may be, so
// every account's screens can be reached (docs/UI.md section 8.12). It is the router's setup, the
// read API's api, and the name of every file and directory at the top of the bundle the server
// serves, read from the bundle itself rather than listed, so a file the build adds is reserved with
// it. A dot file is not served as a file, so it is no path the UI uses.
func Reserved(bundle fs.FS) ([]string, error) {
	entries, err := fs.ReadDir(bundle, ".")
	if err != nil {
		return nil, fmt.Errorf("reading the bundle's top level: %w", err)
	}
	reserved := []string{"setup", "api"}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), ".") {
			reserved = append(reserved, e.Name())
		}
	}
	return reserved, nil
}

func (a *app) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	requestInfo(r.Context()).route = "app"
	if r.Method != http.MethodGet && r.Method != http.MethodHead {
		w.Header().Set("Allow", "GET, HEAD")
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if a.isFile(r.URL.Path) {
		a.files.ServeHTTP(w, r)
		return
	}
	var body bytes.Buffer
	if err := a.template.Execute(&body, entryData{
		Entry: BundleEntry, Stylesheet: BundleStylesheet, Browser: a.browser,
		ReconnectMax: a.browser.StreamReconnectMax.Milliseconds(), PollInterval: a.browser.StreamPollInterval.Milliseconds(),
		Token: a.keys.requestToken(sessionOf(r.Context()).id),
	}); err != nil {
		requestInfo(r.Context()).err = err
		http.Error(w, "the UI server failed", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	if _, err := w.Write(body.Bytes()); err != nil {
		requestInfo(r.Context()).err = err
	}
}

// isFile reports whether p names a regular file of the bundle other than a dot file, such as the
// placeholder the bundle directory keeps on a fresh clone.
func (a *app) isFile(p string) bool {
	name := strings.TrimPrefix(path.Clean(p), "/")
	if name == "" || strings.HasPrefix(path.Base(name), ".") {
		return false
	}
	info, err := fs.Stat(a.bundle, name)
	return err == nil && info.Mode().IsRegular()
}
