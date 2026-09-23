package mustnotcompile_test

import (
	"strings"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

const unexported = "cannot refer to unexported field buf in struct literal of type strings.Builder"

// The fixtures abuse a standard-library type, so these tests exercise the check itself without
// depending on any project type.
func TestRequirePassesOnTheExpectedTypeError(t *testing.T) {
	mustnotcompile.Require(t, "./testdata/mustnotcompile/unexportedfield", unexported)
}

func TestCheckRejectsFixturesThatDoNotProveTheConstructionFails(t *testing.T) {
	cases := []struct {
		name, dir, want, reason string
	}{
		{"compiles", "./testdata/mustnotcompile/compiles", unexported, "type-checks without error"},
		{"bad import", "./testdata/mustnotcompile/badimport", unexported, "unexpected type error"},
		{"syntax error", "./testdata/mustnotcompile/syntaxerror", unexported, "not a type error"},
		{"different message", "./testdata/mustnotcompile/unexportedfield", "unknown field buf", "unexpected type error"},
		{"missing directory", "./testdata/mustnotcompile/absent", unexported, "directory not found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mustnotcompile.Check(c.dir, c.want)
			if err == nil {
				t.Fatalf("Check passed a fixture that must fail it")
			}
			if !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("Check failed for another reason than %q:\n%v", c.reason, err)
			}
		})
	}
}

// strings.Builder keeps every field unexported, and url.URL exports its fields.
func TestCheckNoExportedFields(t *testing.T) {
	mustnotcompile.RequireNoExportedFields(t, "strings", "Builder")
	cases := []struct {
		name, pkg, typ, reason string
	}{
		{"exported field", "net/url", "URL", "has the exported field Scheme"},
		{"a function", "strings", "Lines", "strings.Lines is not a struct"},
		{"missing type", "strings", "Absent", "declares no type Absent"},
		{"interface", "io", "Reader", "is not a struct"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mustnotcompile.CheckNoExportedFields(c.pkg, c.typ)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %v, want an error containing %q", err, c.reason)
			}
		})
	}
}
