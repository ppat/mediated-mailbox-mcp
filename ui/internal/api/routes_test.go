package api_test

import (
	"log/slog"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/getkin/kin-openapi/openapi3"

	"golang.org/x/net/idna"
	"golang.org/x/net/publicsuffix"

	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/policymanagement"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/reads"
	uiserver "github.com/ppat/mediated-mailbox-mcp/ui/internal/server"
	"github.com/ppat/mediated-mailbox-mcp/ui/internal/setup"
)

// lookups are the sender classifier's domain functions, as New passes them.
var lookups = classify.Lookups{ToUnicode: idna.Lookup.ToUnicode, ToASCII: idna.Lookup.ToASCII, Registrable: publicsuffix.EffectiveTLDPlusOne}

// newChecks are the checks over key, logging each request's line through logger, as the process's
// are built over the key token_key_file names.
func newChecks(t *testing.T, key []byte, logger *slog.Logger) *uiserver.Checks {
	t.Helper()
	checks, err := uiserver.New(key, logger)
	if err != nil {
		t.Fatal(err)
	}
	return checks
}

// served builds the server and returns every pattern it registered through its recording mux. It
// touches no database while it is built, so it is given none.
func served(t *testing.T) []string {
	t.Helper()
	e := readAPI(t, nil, newChecks(t, make([]byte, uiserver.MinTokenKey), slog.New(slog.DiscardHandler)),
		reads.Options{Clock: time.Now, StreamInterval: time.Second}, setup.Options{}, policymanagement.Options{})
	return e.Served()
}

// apiRoot is the read API's root, the pattern the UI's spec mounts the read API at, written out here
// rather than read from the code under test (ADR-0137).
const apiRoot = "/ui/api/"

// TestTheServerServesExactlyTheDocumentsRoutes builds the server and requires every pattern it
// registered through its recording mux to be an operation of the checked-in contract document or the
// read API's catch-all, /ui/api/, which refuses every path under the read API that no operation
// answers, and each of those to be registered. The document is read from the file rather than from
// the lists the server is built from, so a route registered through the recording mux that the
// document lacks is caught (ADR-0057).
func TestTheServerServesExactlyTheDocumentsRoutes(t *testing.T) {
	doc, err := openapi3.NewLoader().LoadFromFile(filepath.Join("..", "..", "contract", "openapi.json"))
	if err != nil {
		t.Fatal(err)
	}
	want := []string{apiRoot}
	for path, item := range doc.Paths.Map() {
		for method := range item.Operations() {
			want = append(want, method+" "+path)
		}
	}
	slices.Sort(want)
	got := served(t)
	slices.Sort(got)
	if !slices.Equal(got, want) {
		t.Fatalf("the server registers %v, and the document's operations with the catch-all %s are %v", got, apiRoot, want)
	}
}

// Every route the read API registers sits under the read API's root, the pattern the UI's spec mounts
// it at, so the process routes every one of them to it, and every route sits under the UI's root, so
// the product's top-level words are its surfaces and an account's or a rule's identifier follows the
// word naming it and never sits where a route word does (ADR-0137, docs/UI.md section 5). The spec's
// own test holds the endpoints to the UI's root.
func TestEveryRouteSitsUnderTheReadAPIsRoot(t *testing.T) {
	for _, pattern := range served(t) {
		path := pattern
		if _, after, ok := strings.Cut(pattern, " "); ok {
			path = after
		}
		if !strings.HasPrefix(path, apiRoot) {
			t.Errorf("the read API registers %s, outside its root %s", pattern, apiRoot)
		}
	}
}
