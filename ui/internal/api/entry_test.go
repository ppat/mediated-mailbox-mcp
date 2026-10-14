package api_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/prometheus/client_golang/prometheus"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// lookups are the sender classifier's domain functions, as the composition root passes them.
var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

// TestTheEntryDocumentCarriesTheBrowsersConfiguration requires the four keys the browser reads in
// the entry document as meta tags, durations in whole milliseconds, and the bundle's stylesheet and
// entry module linked under the UI's root (docs/UI.md sections 18 and 18.1).
func TestTheEntryDocumentCarriesTheBrowsersConfiguration(t *testing.T) {
	s, err := api.New(api.Options{
		Bundle: fstest.MapFS{"main.js": {}, "main.css": {}, "fonts/a.woff2": {}}, Datasets: registry.Datasets(lookups), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics: prometheus.NewRegistry(), Clock: time.Now, StreamInterval: time.Second, TokenKey: make([]byte, api.MinTokenKey),
		Browser: api.Browser{
			DefaultTheme: "dark", StreamReconnectMax: 45 * time.Second, StreamPollInterval: 2500 * time.Millisecond,
			ConsentRedirect: "http://[::1]:5000/",
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/ui/accounts/personal", nil))
	body := w.Body.String()
	for _, want := range []string{
		`<meta name="mediated-mailbox.default_theme" content="dark">`,
		`<meta name="mediated-mailbox.stream_reconnect_max" content="45000">`,
		`<meta name="mediated-mailbox.stream_poll_interval" content="2500">`,
		`<meta name="mediated-mailbox.consent_redirect" content="http://[::1]:5000/">`,
		`<link rel="stylesheet" href="/ui/main.css">`,
		`<script type="module" src="/ui/main.js"></script>`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the entry document lacks %s:\n%s", want, body)
		}
	}
}

// root is the UI's root, written out here rather than read from the code under test, under which every
// route the server registers sits, apart from the redirect at / (ADR-0137, docs/UI.md section 5).
const root = "/ui/"

// Every route the server registers sits under the UI's root, apart from the redirect at /, so the
// product's top-level words are its surfaces, and an account's or a rule's identifier follows the word
// naming it and never sits where a route word does (ADR-0137, docs/UI.md section 5).
func TestEveryRouteSitsUnderTheUIsRoot(t *testing.T) {
	s, err := api.New(api.Options{
		Bundle: fstest.MapFS{"main.js": {}, "main.css": {}, "fonts/a.woff2": {}}, Datasets: registry.Datasets(lookups), Logger: slog.New(slog.DiscardHandler),
		Metrics: prometheus.NewRegistry(), Clock: time.Now, StreamInterval: time.Second, TokenKey: make([]byte, api.MinTokenKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	redirects := 0
	for _, pattern := range s.Served() {
		path := pattern
		if _, after, ok := strings.Cut(pattern, " "); ok {
			path = after
		}
		switch {
		case path == "/{$}":
			redirects++
		case !strings.HasPrefix(path, root):
			t.Errorf("the server registers %s, outside the UI's root %s", pattern, root)
		}
	}
	if redirects != 1 {
		t.Errorf("the server registers / %d times, want once, as the redirect to %s", redirects, root)
	}
}

// / answers with a redirect to the UI's root and issues no session, since the session's cookie is
// scoped to the root and a new one issued at / would replace the session the browser's open pages
// hold. A path outside the root that is not / answers 404, and the root issues the session scoped to
// itself (ADR-0137, docs/UI.md sections 5 and 15).
func TestTheRootRedirectsToTheUIAndNothingElseOutsideItAnswers(t *testing.T) {
	s, err := api.New(api.Options{
		Bundle: fstest.MapFS{"main.js": {}, "main.css": {}, "fonts/a.woff2": {}}, Datasets: registry.Datasets(lookups), Logger: slog.New(slog.DiscardHandler),
		Metrics: prometheus.NewRegistry(), Clock: time.Now, StreamInterval: time.Second, TokenKey: make([]byte, api.MinTokenKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/", nil))
	if w.Code != http.StatusFound || w.Header().Get("Location") != root {
		t.Errorf("/ answers %d with Location %q, want %d with %q", w.Code, w.Header().Get("Location"), http.StatusFound, root)
	}
	if c := w.Header().Values("Set-Cookie"); len(c) != 0 {
		t.Errorf("/ issues cookies %v, want none", c)
	}
	for _, p := range []string{"/personal", "/setup", "/main.js", "/api/accounts", "/mcp"} {
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		if w.Code != http.StatusNotFound {
			t.Errorf("%s answers %d, want %d, since the UI serves nothing outside its root but /", p, w.Code, http.StatusNotFound)
		}
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", root, nil))
	cookies := w.Result().Cookies()
	if w.Code != http.StatusOK || len(cookies) != 1 || cookies[0].Path != root {
		t.Errorf("%s answers %d with cookies %v, want %d with one session cookie scoped to %s", root, w.Code, cookies, http.StatusOK, root)
	}
	w = httptest.NewRecorder()
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", root+"main.js", nil))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "<!doctype html>") {
		t.Errorf("%smain.js answers %d with %q, want the bundle's file", root, w.Code, w.Body.String())
	}
}
