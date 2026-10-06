package api_test

import (
	"io"
	"log/slog"
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
// the entry document as meta tags, durations in whole milliseconds (docs/UI.md section 18.1).
func TestTheEntryDocumentCarriesTheBrowsersConfiguration(t *testing.T) {
	s, err := api.New(api.Options{
		Bundle: fstest.MapFS{}, Datasets: registry.Datasets(lookups), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
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
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the entry document lacks %s:\n%s", want, body)
		}
	}
}
