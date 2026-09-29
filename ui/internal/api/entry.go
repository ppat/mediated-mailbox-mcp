package api

import (
	"bytes"
	"html/template"
	"io/fs"
	"net/http"
	"path"
	"strings"
)

// BundleEntry is the bundle's entry module, the file bun build writes for src/app/main.ts.
const BundleEntry = "main.js"

// entryTemplate is the entry document. It is the one page a Go handler renders, on every page load, so
// it can carry the request token when the decisions arrive (ADR-0061). It holds no inline script and
// names no other origin, and the policy's tests read it (ADR-0062, ADR-0064).
const entryTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Mediated Mailbox</title>
<script type="module" src="/{{.Entry}}"></script>
</head>
<body></body>
</html>
`

// EntryTemplate returns the entry document's template text, which the policy's tests scan.
func EntryTemplate() string { return entryTemplate }

type entryData struct {
	Entry string
}

// app serves a bundle file when the path names one, and otherwise the entry document, so every
// screen route the browser's router owns loads the app.
type app struct {
	bundle   fs.FS
	files    http.Handler
	template *template.Template
}

func newApp(bundle fs.FS) (*app, error) {
	t, err := template.New("entry").Parse(entryTemplate)
	if err != nil {
		return nil, err
	}
	return &app{bundle: bundle, files: http.FileServerFS(bundle), template: t}, nil
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
	if err := a.template.Execute(&body, entryData{Entry: BundleEntry}); err != nil {
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
