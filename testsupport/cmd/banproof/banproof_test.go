package main

import (
	"regexp"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

func TestParseWants(t *testing.T) {
	src := []byte(`package p

import _ "os/exec" // want depguard "list 'pure-core'" depguard ` + "`list 'non-test-code'`" + `

var s = "// want depguard \"inside a string\""
// want suppression "suppression"
var t = 1 //nolint:forbidigo // want suppression "after a directive"
// Prose that mentions // want depguard "x" is not an annotation.
var u = 1 /* want errcheck "in a block comment" suppression "x" */ // nolint
/* Prose in a block comment that mentions want depguard "x" is not an annotation. */
`)
	ws, err := parseWants("f.go", src)
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
		{3, "depguard", "list 'pure-core'"},
		{3, "depguard", "list 'non-test-code'"},
		{6, "suppression", "suppression"},
		{7, "suppression", "after a directive"},
		{9, "errcheck", "in a block comment"},
		{9, "suppression", "x"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Fatalf("wants (-want +got):\n%s", diff)
	}
}

func TestParseWantsRejectsMalformedAnnotations(t *testing.T) {
	for _, body := range []string{`want depguard`, `want depguard unquoted`, `want depguard "("`, `want `} {
		if _, err := parseWants("f.go", []byte("package p\n// "+body+"\n")); err == nil {
			t.Errorf("parseWants accepted %q", body)
		}
	}
}

func TestMatch(t *testing.T) {
	w := func(line int, tool, pattern string) want {
		return want{file: "f.go", line: line, tool: tool, re: regexp.MustCompile(pattern)}
	}
	f := func(line int, tool, text string) finding {
		return finding{file: "f.go", line: line, tool: tool, text: text}
	}
	wants := []want{
		w(1, "depguard", "list"),          // matches both findings on line 1
		w(1, "depguard", "list 'second'"), // matches only the second
		w(2, "forbidigo", "Make"),         // no finding
		w(3, "errcheck", "after"),         // the finding names another tool
	}
	findings := []finding{
		f(1, "depguard", "import 'x' is not allowed from list 'second'"),
		f(1, "depguard", "import 'x' is not allowed from list 'first'"),
		f(3, "depguard", "after"),
		f(4, "errcheck", "unchecked"),
	}
	unmet, unexpected := match(wants, findings)
	var gotUnmet, gotUnexpected []string
	for _, u := range unmet {
		gotUnmet = append(gotUnmet, u.String())
	}
	for _, u := range unexpected {
		gotUnexpected = append(gotUnexpected, u.String())
	}
	wantUnmet := []string{`f.go:2: forbidigo "Make"`, `f.go:3: errcheck "after"`}
	wantUnexpected := []string{"f.go:3: depguard: after", "f.go:4: errcheck: unchecked"}
	if diff := cmp.Diff(wantUnmet, gotUnmet); diff != "" {
		t.Errorf("unmet (-want +got):\n%s", diff)
	}
	if diff := cmp.Diff(wantUnexpected, gotUnexpected); diff != "" {
		t.Errorf("unexpected (-want +got):\n%s", diff)
	}
}

func TestNeedsTag(t *testing.T) {
	cases := map[string]bool{
		"//go:build banproof\n\npackage p\n":                true,
		"//go:build integration && banproof\n\npackage p\n": true,
		"//go:build banproof || integration\n\npackage p\n": false,
		"//go:build !banproof\n\npackage p\n":               false,
		"package p\n\n//go:build banproof\n":                false,
		"// Package p.\npackage p\n":                        false,
	}
	for src, want := range cases {
		got, err := needsTag([]byte(src), "banproof")
		if err != nil || got != want {
			t.Errorf("needsTag(%q) = %v, %v, want %v", src, got, err, want)
		}
	}
}

func TestViolationName(t *testing.T) {
	cases := map[string]bool{
		"io_violation.go":                     true,
		"io_violation_test.go":                true,
		"make_violation_property_test.go":     true,
		"rapid_violation_integration_test.go": true,
		"violation.go":                        false,
		"io_test.go":                          false,
		"violations_test.go":                  false,
	}
	for name, want := range cases {
		if got := violationName.MatchString(name); got != want {
			t.Errorf("violationName(%q) = %v, want %v", name, got, want)
		}
	}
}
