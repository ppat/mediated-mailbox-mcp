package api_test

import (
	"io"
	"log/slog"
	"net/http/httptest"
	"slices"
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
// the entry document as meta tags, durations in whole milliseconds, and the words account setup
// refuses as identifiers, which the connect page's proposal avoids (docs/UI.md section 18.1).
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
	s.Handler().ServeHTTP(w, httptest.NewRequest("GET", "/personal", nil))
	body := w.Body.String()
	for _, want := range []string{
		`<meta name="mediated-mailbox.default_theme" content="dark">`,
		`<meta name="mediated-mailbox.stream_reconnect_max" content="45000">`,
		`<meta name="mediated-mailbox.stream_poll_interval" content="2500">`,
		`<meta name="mediated-mailbox.consent_redirect" content="http://[::1]:5000/">`,
		`<meta name="mediated-mailbox.reserved_identifiers" content="setup api fonts main.css main.js">`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the entry document lacks %s:\n%s", want, body)
		}
	}
}

// topLevelWords is every word the UI's own top-level paths use, which account setup refuses as an
// account identifier (docs/UI.md section 5). They are the browser router's setup, the server's api,
// and the built bundle's main.js, main.css and fonts. The list is written out, so a word the UI starts
// using is a change to it, which review checks against the accounts already stored. The browser's
// tests pin the router's word in test/routes.test.ts and the bundle's top level in test/build.test.ts.
var topLevelWords = []string{"api", "fonts", "main.css", "main.js", "setup"}

// Given a bundle whose top level is the built bundle's, the words account setup refuses are the pinned
// top-level words, and every route the server registers starts with one of them or is the catch-all
// that serves the entry document, so a route outside the list fails here even when its contract
// operation is added with it (docs/UI.md section 5). TestTheBuiltBundleReservesThePinnedWords reads
// the built bundle itself.
func TestTheTopLevelWordsAreAClosedList(t *testing.T) {
	bundle := fstest.MapFS{"main.js": {}, "main.css": {}, "fonts/a.woff2": {}, ".gitkeep": {}}
	reserved, err := api.Reserved(bundle)
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(reserved)
	if !slices.Equal(reserved, topLevelWords) {
		t.Errorf("account setup refuses %v, and the pinned top-level words are %v", reserved, topLevelWords)
	}
	s, err := api.New(api.Options{
		Bundle: bundle, Datasets: registry.Datasets(lookups), Logger: slog.New(slog.DiscardHandler),
		Metrics: prometheus.NewRegistry(), Clock: time.Now, StreamInterval: time.Second, TokenKey: make([]byte, api.MinTokenKey),
	})
	if err != nil {
		t.Fatal(err)
	}
	for _, pattern := range s.Served() {
		path := pattern
		if _, after, ok := strings.Cut(pattern, " "); ok {
			path = after
		}
		word, _, _ := strings.Cut(strings.TrimPrefix(path, "/"), "/")
		if path != "/" && !slices.Contains(topLevelWords, word) {
			t.Errorf("the server registers %s, whose first word %q is not a pinned top-level word", pattern, word)
		}
	}
}
