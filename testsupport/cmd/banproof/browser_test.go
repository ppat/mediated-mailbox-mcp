package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

func TestParseBrowserWants(t *testing.T) {
	src := []byte(`// Prose that mentions wanting something is not an annotation.
element.innerHTML = html; // want oxlint "'innerHTML'"
{count.value /* want ast-grep "signal" */}
/* want suppression "eslint-disable" */ // eslint-disable-next-line no-console
`)
	ws, err := parseBrowserWants("f.tsx", src)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		Line          int
		Tool, Pattern string
	}
	var got []row
	for _, w := range ws {
		got = append(got, row{w.line, w.tool, w.re.String()})
	}
	want := []row{
		{2, "oxlint", "'innerHTML'"},
		{3, "ast-grep", "signal"},
		{4, "suppression", "eslint-disable"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Fatalf("wants (-want +got):\n%s", diff)
	}
}

func TestParseBrowserWantsRejectsMalformedAnnotations(t *testing.T) {
	for _, line := range []string{`// want oxlint`, `// want oxlint unquoted`, `/* want oxlint "(" */`} {
		if _, err := parseBrowserWants("f.ts", []byte(line+"\n")); err == nil {
			t.Errorf("parseBrowserWants accepted %q", line)
		}
	}
}

func TestSuppressionFindings(t *testing.T) {
	src := []byte(`// eslint-disable-next-line no-console
/* oxlint-disable */
// ast-grep-ignore: signal-value-in-render
const s = "Oxlint-Disable in a string still counts";
/* want suppression "eslint-disable" */
// an ordinary comment
`)
	var got []int
	for _, f := range suppressionFindings("f.ts", src) {
		got = append(got, f.line)
	}
	if diff := cmp.Diff([]int{1, 2, 3, 4}, got); diff != "" {
		t.Fatalf("suppression lines (-want +got):\n%s", diff)
	}
}

func TestParseToolOutput(t *testing.T) {
	ox, err := parseOxlint("/b", []byte(`{"diagnostics":[{"message":"m","code":"eslint(no-restricted-properties)","severity":"warning","filename":"src/a.ts","labels":[{"span":{"line":4}}]}]}`))
	if err != nil {
		t.Fatal(err)
	}
	ag, err := parseAstGrep("/b", []byte(`[{"file":"src/b.tsx","ruleId":"signal-value-in-render","severity":"error","message":"m","range":{"start":{"line":6}}}]`))
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		File, Tool, Text, Severity string
		Line                       int
	}
	var got []row
	for _, f := range append(ox, ag...) {
		got = append(got, row{f.file, f.tool, f.text, f.severity, f.line})
	}
	want := []row{
		{"/b/src/a.ts", "oxlint", "eslint(no-restricted-properties): m", "warning", 4},
		{"/b/src/b.tsx", "ast-grep", "signal-value-in-render: m", "error", 7},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Fatalf("findings (-want +got):\n%s", diff)
	}
}

func TestViolationImport(t *testing.T) {
	for src, want := range map[string]bool{
		`import { Row } from "../row/markup_violation.tsx";`: true,
		`export * from './markup_violation';`:                true,
		`const m = await import("./signal_violation.tsx");`:  true,
		`import { Row } from "../row/row.tsx";`:              false,
	} {
		if got := violationImport.MatchString(src); got != want {
			t.Errorf("violationImport on %q = %v, want %v", src, got, want)
		}
	}
}
