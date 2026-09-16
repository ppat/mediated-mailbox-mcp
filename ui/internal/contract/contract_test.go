package contract_test

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	"github.com/getkin/kin-openapi/openapi3"
)

var update = flag.Bool("update", false, "write the generated document to ui/contract instead of comparing")

// documentPath is the checked-in document, relative to this package's directory.
var documentPath = filepath.Join("..", "..", "contract", "openapi.json")

// document populates kin-openapi's document types from the registry and the handler list. The registry
// declares no entries and the handler list is empty, so the document has no paths yet.
func document() *openapi3.T {
	return &openapi3.T{
		OpenAPI: "3.1.1",
		Info: &openapi3.Info{
			Title:   "mediated-mailbox-ui",
			Version: "0",
		},
		Paths: openapi3.NewPaths(),
	}
}

// render serializes the document in the form checked in, two-space indentation and one trailing newline,
// which the repository's text fixers leave unchanged.
func render(t *testing.T, doc *openapi3.T) []byte {
	t.Helper()
	out, err := json.MarshalIndent(doc, "", "  ")
	if err != nil {
		t.Fatalf("serializing the document: %v", err)
	}
	return append(out, '\n')
}

func TestDocument(t *testing.T) {
	doc := document()
	if err := doc.Validate(context.Background()); err != nil {
		t.Fatalf("the generated document is not valid OpenAPI: %v", err)
	}
	got := render(t, doc)
	if *update {
		if err := os.WriteFile(documentPath, got, 0o600); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(documentPath)
	if err != nil {
		t.Fatalf("reading the checked-in document: %v. Generate it with: go generate ./ui/internal/contract", err)
	}
	if !bytes.Equal(got, want) {
		t.Fatalf("%s differs from what the registry and handler list generate. Regenerate with: go generate ./ui/internal/contract\n--- generated\n%s", documentPath, got)
	}
}
