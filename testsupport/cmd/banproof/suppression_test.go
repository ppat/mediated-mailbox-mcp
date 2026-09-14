package main

import (
	"testing"

	"github.com/google/go-cmp/cmp"
)

// TestDirectiveProblem runs the classifier over the spellings measured against golangci-lint, with gosec
// as the ordinary linter and forbidigo standing in for a control. Every spelling that silenced forbidigo
// is refused, and so is every spelling of a directive the tool ignores today.
func TestDirectiveProblem(t *testing.T) {
	ordinary := []string{"gosec", "staticcheck"}
	cases := map[string]bool{
		// Silences every linter.
		"//nolint":                      true,
		"// nolint":                     true,
		"//nolint reason words":         true,
		"// //nolint":                   true,
		"//nolint:all":                  true,
		"//nolint:allow-this":           true,
		"//nolint:gosec,all":            true,
		"//nolint:gosec, ALL // reason": true,
		// Names a linter off the ordinary list.
		"//nolint:gosec,forbidigo":            true,
		"//nolint:gosec, forbidigo // reason": true,
		"//nolint: forbidigo":                 true,
		"//nolint:FORBIDIGO // reason":        true,
		"////nolint:forbidigo // reason":      true,
		"//nolint:gosec reason words":         true,
		"//nolint:gosec;forbidigo // reason":  true,
		"//nolint:gosec -- reason":            true,
		"//nolint:gosec, // reason":           true,
		"//nolint: // reason":                 true,
		"//nolint:unknownlinter // reason":    true,
		// Names only ordinary linters but gives no reason.
		"//nolint:gosec":      true,
		"//nolint:gosec //":   true,
		"//nolint:gosec //  ": true,
		// Spellings golangci-lint does not honour today.
		"//NOLINT":           true,
		"//Nolint:gosec":     true,
		"/* nolint */":       true,
		"/*nolint*/":         true,
		"//\tnolint":         true,
		"/* nolint:gosec */": true,
		// The in-code directives of linters standing in for controls.
		"//exhaustive:ignore":   true,
		"// exhaustive:ignore":  true,
		"/*exhaustive:ignore*/": true,
		"//exhaustive:enforce":  true,
		"//permit:fmt.Println":  true,
		// Allowed.
		"//nolint:gosec // reason":                   false,
		"//nolint:Gosec // reason":                   false,
		"//nolint:gosec,staticcheck // reason":       false,
		"//nolint:gosec //nolint":                    false,
		"// nolintlint is a linter, not a directive": false,
		"// an ordinary comment":                     false,
		"//go:build banproof":                        false,
		"// #nosec G304":                             false,
		"/* a block comment */":                      false,
	}
	for text, want := range cases {
		why, got := directiveProblem(text, ordinary)
		if got != want {
			t.Errorf("directiveProblem(%q) = %q, %v, want refused %v", text, why, got, want)
		}
	}
}

func TestDirectiveFindings(t *testing.T) {
	src := []byte(`package p

var a = 1 //nolint:forbidigo // reason
var b = 1 //nolint:gosec // reason
var c = "//nolint"
// Package prose.
//exhaustive:ignore
var d = 1
`)
	var lines []int
	for _, f := range directiveFindings("f.go", src, []string{"gosec"}) {
		lines = append(lines, f.line)
	}
	if diff := cmp.Diff([]int{3, 7}, lines); diff != "" {
		t.Fatalf("lines reported (-want +got):\n%s", diff)
	}
}
