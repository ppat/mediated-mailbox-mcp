package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/process/version"
)

// The MCP root reports the release version as its server's version, the one --version prints, so the
// binary names one version wherever it reports one (ADR-0078).
func TestTheMCPRootReportsTheReleaseVersion(t *testing.T) {
	reg, _ := counted(t)
	h := mustSurface(t, reg, tokenFile(t, "s3cret"))
	body := `{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2025-06-18","capabilities":{},"clientInfo":{"name":"test","version":"1"}}}`
	req := httptest.NewRequestWithContext(t.Context(), http.MethodPost, "/mcp", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json, text/event-stream")
	req.Header.Set("Authorization", "Bearer s3cret")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("initialize answered %d: %s", rec.Code, rec.Body.String())
	}
	text := rec.Body.String()
	if i := strings.Index(text, "{"); i > 0 {
		text = strings.TrimSpace(text[i:])
	}
	var answer struct {
		Result struct {
			ServerInfo struct {
				Name    string `json:"name"`
				Version string `json:"version"`
			} `json:"serverInfo"`
		} `json:"result"`
	}
	if err := json.Unmarshal([]byte(text), &answer); err != nil {
		t.Fatalf("decoding %q: %v", rec.Body.String(), err)
	}
	if got := answer.Result.ServerInfo; got.Name != "mediated-mailbox-mediate" || got.Version != version.Number {
		t.Errorf("the server reports %+v, want mediated-mailbox-mediate at %s", got, version.Number)
	}
}
