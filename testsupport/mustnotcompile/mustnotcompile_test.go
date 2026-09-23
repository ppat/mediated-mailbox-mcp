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

// strings.Builder holds exactly an address and a buffer.
func TestCheckFields(t *testing.T) {
	mustnotcompile.RequireFields(t, "strings", "Builder", "addr *Builder", "buf []byte")
	cases := []struct {
		name, pkg, typ string
		fields         []string
		reason         string
	}{
		{"a field left out", "strings", "Builder", []string{"addr *Builder"}, `has the fields ["addr *Builder" "buf []byte"]`},
		{"a field too many", "strings", "Builder", []string{"addr *Builder", "buf []byte", "body string"}, "has the fields"},
		{"another type", "strings", "Builder", []string{"addr *Builder", "buf string"}, "has the fields"},
		{"a function", "strings", "Lines", nil, "strings.Lines is not a struct"},
		{"missing type", "strings", "Absent", nil, "declares no type Absent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mustnotcompile.CheckFields(c.pkg, c.typ, c.fields...)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %v, want an error containing %q", err, c.reason)
			}
		})
	}
}

// strings.Repeat takes a string and an int, and fmt.Sprintf a string and a variadic list.
func TestCheckParams(t *testing.T) {
	mustnotcompile.RequireParams(t, "strings", "Repeat", "string", "int")
	mustnotcompile.RequireParams(t, "fmt", "Sprintf", "string", "...any")
	cases := []struct {
		name, pkg, fn string
		params        []string
		reason        string
	}{
		{"a parameter left out", "strings", "Repeat", []string{"string"}, `takes ["string" "int"]`},
		{"a parameter too many", "strings", "Repeat", []string{"string", "int", "string"}, "takes"},
		{"variadic written plainly", "fmt", "Sprintf", []string{"string", "[]any"}, "takes"},
		{"a type", "strings", "Builder", nil, "declares no function Builder"},
		{"missing function", "strings", "Absent", nil, "declares no function Absent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mustnotcompile.CheckParams(c.pkg, c.fn, c.params...)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %v, want an error containing %q", err, c.reason)
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
