package api_test

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/ui/internal/api"
)

// The policy's static half (ADR-0062, with the proof split ADR-0064 records). The entry template and
// the built bundle hold no inline script and no reference to another origin, and every stylesheet
// takes its fonts from the UI's own origin, so nothing the UI ships needs a looser policy. The scan is
// api.NeedsLooserPolicy. That a browser enforces the header is the drill of the policy row, performed
// by a person.

// TestTheEntryTemplateNeedsNoLooserPolicy scans the entry document's template.
func TestTheEntryTemplateNeedsNoLooserPolicy(t *testing.T) {
	if findings := api.NeedsLooserPolicy("entry.html", api.EntryTemplate()); len(findings) > 0 {
		t.Fatalf("the entry template needs a looser policy: %v", findings)
	}
}

// TestTheBundleNeedsNoLooserPolicy scans every file of the built bundle. On a fresh clone the bundle
// directory holds only its placeholder, and the ui workflow builds the bundle before this test runs,
// so the test skips, naming the step, when there is nothing to scan.
func TestTheBundleNeedsNoLooserPolicy(t *testing.T) {
	dist := filepath.Join("..", "..", "browser", "dist")
	var scanned int
	err := fs.WalkDir(os.DirFS(dist), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || strings.HasPrefix(d.Name(), ".") {
			return err
		}
		b, err := os.ReadFile(filepath.Join(dist, p))
		if err != nil {
			return err
		}
		scanned++
		if findings := api.NeedsLooserPolicy(p, string(b)); len(findings) > 0 {
			t.Errorf("%s needs a looser policy: %v", p, findings)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if scanned == 0 {
		t.Skip("ui/browser/dist holds no bundle. Build it with bun run build in ui/browser, as the ui workflow does first")
	}
}

// TestThePolicyScanReportsEachPlantedViolation runs the scan over checked-in files that each need a
// looser policy and requires exactly their findings (VERIFICATIONS, the policy row). They are an inline
// script, an inline event handler and a script URL in an entry template, a script fetching another
// origin by a scheme and one loading it scheme-relative, a stylesheet taking a font from another
// origin, and a stylesheet inlining a font as a data: URI beside an inlined image the policy admits.
//
// drill.html and drill.js are also the page of the policy row's drill, which a person performs
// (ADR-0064). They copy both files into ui/browser/dist after a build, run the UI from that checkout
// over plain HTTP, with neither tls_cert nor tls_key named, open /drill.html in a browser with its
// developer console open, and record the console's refusal of the inline script and of the fetch
// verbatim, with the date and the browser. The start validates the whole configuration of docs/UI.md section 18.1,
// so the database section is given too, database.host, database.name and database.password_file,
// though the drill's requests never reach the database. The page's title stays "Policy drill" whether
// or not the browser blocks the fetch, since drill.example never answers, so the console's refusal is
// the evidence.
func TestThePolicyScanReportsEachPlantedViolation(t *testing.T) {
	want := map[string][]string{
		"inline_handler.html": {"inline event handler"},
		"script_url.html":     {"script URL"},
		"scheme_relative.js":  {"another origin: //modules.example"},
		"inline_script.html":  {"inline script"},
		"external_origin.js":  {"another origin: https://collector.example/beacon"},
		"external_font.css": {
			"another origin: https://fonts.example/plex-sans.woff2",
			"stylesheet resource off the UI's origin: https://fonts.example/plex-sans.woff2",
		},
		"inline_font.css": {"font inlined as a data: URI"},
		"drill.html":      {"inline script"},
		"drill.js":        {"another origin: https://drill.example/policy"},
	}
	entries, err := os.ReadDir(filepath.Join("testdata", "policy"))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	if !slices.Equal(names, []string{"drill.html", "drill.js", "external_font.css", "external_origin.js", "inline_font.css", "inline_handler.html", "inline_script.html", "scheme_relative.js", "script_url.html"}) {
		t.Fatalf("testdata/policy holds %v, and each file needs its expected findings here", names)
	}
	for name, findings := range want {
		b, err := os.ReadFile(filepath.Join("testdata", "policy", name))
		if err != nil {
			t.Fatal(err)
		}
		if d := cmp.Diff(findings, api.NeedsLooserPolicy(name, string(b))); d != "" {
			t.Errorf("%s (-want +got):\n%s", name, d)
		}
	}
}
