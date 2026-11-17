package pages_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"regexp"
	"strings"
	"testing"
	"testing/fstest"
	"time"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/pages"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
)

// root is the UI's root, written out here rather than read from the code under test (ADR-0137).
const root = "/ui/"

// bundle stands for the embedded bundle, its entry module, its stylesheet, a font and the placeholder
// a fresh clone's bundle directory keeps.
var bundle = fstest.MapFS{"main.js": {}, "main.css": {}, "fonts/a.woff2": {}, ".gitkeep": {}}

// behindChecks returns the pages handler over bundle with b, behind checks, as New returns it.
func behindChecks(t *testing.T, b pages.BrowserConfig, checks *server.Checks) http.Handler {
	t.Helper()
	a, err := pages.NewApp(bundle, b, checks)
	if err != nil {
		t.Fatal(err)
	}
	return checks.Wrap(a)
}

func newChecks(t *testing.T) *server.Checks {
	t.Helper()
	checks, err := server.New([]byte("the request token key of the tests, 32 bytes at least"), slog.New(slog.DiscardHandler))
	if err != nil {
		t.Fatal(err)
	}
	return checks
}

// TestTheEntryDocumentCarriesTheBrowsersConfiguration requires the four keys the browser reads in
// the entry document as meta tags, durations in whole milliseconds, and the bundle's stylesheet and
// entry module linked under the UI's root (docs/UI.md sections 18 and 18.1).
func TestTheEntryDocumentCarriesTheBrowsersConfiguration(t *testing.T) {
	h := behindChecks(t, pages.BrowserConfig{
		DefaultTheme: "dark", StreamReconnectMax: 45 * time.Second, StreamPollInterval: 2500 * time.Millisecond,
		ConsentRedirect: "http://[::1]:5000/",
	}, newChecks(t))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", "/ui/accounts/personal", nil))
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

var tokenMeta = regexp.MustCompile(`<meta name="mediated-mailbox.request_token" content="([^"]*)">`)

// The entry document carries the request token of the session it issues, which a state-changing
// request of that session carries back past the checks, while another session's token or none is
// refused, so the page and the token check derive the token alike (ADR-0061).
func TestTheEntryDocumentCarriesItsSessionsRequestToken(t *testing.T) {
	checks := newChecks(t)
	h := behindChecks(t, pages.BrowserConfig{}, checks)
	load := func() (*http.Cookie, string) {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", root, nil))
		cookies := w.Result().Cookies()
		m := tokenMeta.FindStringSubmatch(w.Body.String())
		if len(cookies) != 1 || m == nil || m[1] == "" {
			t.Fatalf("the entry document issued cookies %v and carried no token:\n%s", cookies, w.Body.String())
		}
		return cookies[0], m[1]
	}
	session, token := load()
	_, another := load()
	reached := checks.Wrap(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) }))
	for _, c := range []struct {
		token string
		want  int
	}{{token, http.StatusNoContent}, {another, http.StatusForbidden}, {"", http.StatusForbidden}} {
		r := httptest.NewRequest("POST", root+"api/setup/connect", nil)
		r.AddCookie(session)
		if c.token != "" {
			r.Header.Set("X-Request-Token", c.token)
		}
		w := httptest.NewRecorder()
		reached.ServeHTTP(w, r)
		if w.Code != c.want {
			t.Errorf("a POST with the token %q answered %d, want %d", c.token, w.Code, c.want)
		}
	}
}

// The root issues the session scoped to itself, a path below it naming a file of the bundle answers
// that file, and the placeholder a fresh clone's bundle directory keeps is never served as a file
// (ADR-0137, docs/UI.md sections 5 and 15).
func TestTheRootIssuesItsSessionAndServesTheBundle(t *testing.T) {
	h := behindChecks(t, pages.BrowserConfig{}, newChecks(t))
	w := httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", root, nil))
	cookies := w.Result().Cookies()
	if w.Code != http.StatusOK || len(cookies) != 1 || cookies[0].Path != root {
		t.Errorf("%s answers %d with cookies %v, want %d with one session cookie scoped to %s", root, w.Code, cookies, http.StatusOK, root)
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", root+"main.js", nil))
	if w.Code != http.StatusOK || strings.Contains(w.Body.String(), "<!doctype html>") {
		t.Errorf("%smain.js answers %d with %q, want the bundle's file", root, w.Code, w.Body.String())
	}
	w = httptest.NewRecorder()
	h.ServeHTTP(w, httptest.NewRequest("GET", root+".gitkeep", nil))
	if !strings.Contains(w.Body.String(), "<!doctype html>") {
		t.Errorf("the bundle's placeholder was served as a file: %d %q", w.Code, w.Body.String())
	}
}

// / answers a GET and a HEAD with a redirect to the UI's root and every other method with 405 naming
// the two it answers, and behind the checks it issues no session, since the session's cookie is
// scoped to the root and a new one issued at / would replace the session the browser's open pages
// hold (ADR-0137, docs/UI.md sections 5 and 15).
func TestTheRootRedirectsToTheUI(t *testing.T) {
	for _, method := range []string{http.MethodGet, http.MethodHead} {
		w := httptest.NewRecorder()
		newChecks(t).Wrap(pages.RedirectHandler).ServeHTTP(w, httptest.NewRequest(method, "/", nil))
		if w.Code != http.StatusFound || w.Header().Get("Location") != root {
			t.Errorf("%s / answers %d with Location %q, want %d with %q", method, w.Code, w.Header().Get("Location"), http.StatusFound, root)
		}
		if c := w.Header().Values("Set-Cookie"); len(c) != 0 {
			t.Errorf("%s / issues cookies %v, want none", method, c)
		}
	}
	w := httptest.NewRecorder()
	pages.RedirectHandler.ServeHTTP(w, httptest.NewRequest(http.MethodPost, "/", nil))
	if w.Code != http.StatusMethodNotAllowed || w.Header().Get("Allow") != "GET, HEAD" {
		t.Errorf("POST / answers %d allowing %q, want %d allowing GET, HEAD", w.Code, w.Header().Get("Allow"), http.StatusMethodNotAllowed)
	}
}

// TestTheEntryDocumentIsRenderedForEveryScreenRoute requires every path the browser's router owns,
// deep paths and one with a query among them, to load the entry document as HTML, and it to load the
// bundle's entry module and its stylesheet from the UI's own origin.
func TestTheEntryDocumentIsRenderedForEveryScreenRoute(t *testing.T) {
	h := behindChecks(t, pages.BrowserConfig{}, newChecks(t))
	for _, p := range []string{root, "/ui/accounts/personal", "/ui/accounts/personal/candidates?status=pending", "/ui/accounts/personal/jobs/r-0913"} {
		w := httptest.NewRecorder()
		h.ServeHTTP(w, httptest.NewRequest("GET", p, nil))
		body := w.Body.String()
		if w.Code != http.StatusOK || !strings.Contains(body, `<script type="module" src="/ui/main.js"></script>`) ||
			!strings.Contains(body, `<link rel="stylesheet" href="/ui/main.css">`) {
			t.Errorf("%s answered %d without the entry document:\n%s", p, w.Code, body)
		}
		if got := w.Header().Get("Content-Type"); got != "text/html; charset=utf-8" {
			t.Errorf("%s answered as %q", p, got)
		}
	}
}
