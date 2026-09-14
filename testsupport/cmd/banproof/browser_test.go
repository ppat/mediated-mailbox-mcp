package main

import (
	"strings"
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

func TestSanctionedValueRead(t *testing.T) {
	src := []byte(`{
  // Row.value is the contract field, not a signal.
  // ast-grep-ignore: signal-value-in-render
  props.row.value
}
{
  // ast-grep-ignore: signal-value-in-render
  props.row.value
}
{
  // The contract field, not a signal.
  // ast-grep-ignore: signal-value-in-render
  props.row.value
}
{
  // Row.value is the contract field, not a signal.
  // ast-grep-ignore: signal-value-in-render, markup-prop-tsx
  props.row.value
}
{
  // Row.value is the contract field, not a signal.
  // ast-grep-ignore
  props.row.value
}
{
  // Row.value is the contract field. ast-grep-ignore: signal-value-in-render
  // ast-grep-ignore: signal-value-in-render
  props.row.value
}
{
  /* Row.value is the contract field, not a signal. */
  // ast-grep-ignore: signal-value-in-render
  props.row.value
}
{
  // Row.value is the contract field, not a signal.
  props.row.value // ast-grep-ignore: signal-value-in-render
}
{
  // Row.value is the contract field, not a signal.
  // oxlint-disable-next-line signal-value-in-render
  props.row.value
}
`)
	var got []int
	for _, f := range suppressionFindings("f.tsx", src) {
		got = append(got, f.line)
	}
	// Only the first directive is the sanctioned form. The second has no field line above it, the third's
	// comment names no field, the fourth and fifth name more than the signal rule, the sixth's field line is
	// itself a directive and is reported too, the seventh names the field in a block comment, the eighth
	// shares a line with the read, and the ninth is oxlint's.
	if diff := cmp.Diff([]int{7, 12, 17, 22, 26, 27, 32, 37, 41}, got); diff != "" {
		t.Fatalf("suppression lines (-want +got):\n%s", diff)
	}
	if fs := suppressionFindings("f.ts", src[:strings.Index(string(src), "}")+1]); len(fs) != 1 {
		t.Fatalf("the sanctioned form in a .ts file, which the signal rule does not read, is allowed: %v", fs)
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
