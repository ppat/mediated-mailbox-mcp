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

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/registry"
)

// TestTheEntryDocumentCarriesTheBrowsersConfiguration requires the three keys the browser reads in
// the entry document as meta tags, durations in whole milliseconds (docs/UI.md section 18.1).
func TestTheEntryDocumentCarriesTheBrowsersConfiguration(t *testing.T) {
	s, err := api.New(api.Options{
		Bundle: fstest.MapFS{}, Datasets: registry.Datasets(), Logger: slog.New(slog.NewTextHandler(io.Discard, nil)),
		Metrics: prometheus.NewRegistry(), Clock: time.Now, StreamInterval: time.Second,
		Browser: api.Browser{DefaultTheme: "dark", StreamReconnectMax: 45 * time.Second, StreamPollInterval: 2500 * time.Millisecond},
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
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the entry document lacks %s:\n%s", want, body)
		}
	}
}
