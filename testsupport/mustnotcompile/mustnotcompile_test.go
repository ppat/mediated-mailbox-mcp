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

// Of http.RoundTripper's one method, RoundTrip returns a Response, which carries a Request in a
// field, and hash.Hash's Sum and Size return neither.
func TestCheckReturning(t *testing.T) {
	mustnotcompile.RequireReturning(t, "net/http", "RoundTripper", "Response", "RoundTrip")
	mustnotcompile.RequireReturning(t, "net/http", "RoundTripper", "Request", "RoundTrip")
	mustnotcompile.RequireReturning(t, "net/http", "Handler", "Response")
	cases := []struct {
		name, pkg, iface, typ string
		methods               []string
		reason                string
	}{
		{"a method left out", "net/http", "RoundTripper", "Response", nil, `are ["RoundTrip"]`},
		{"a method too many", "net/http", "Handler", "Response", []string{"ServeHTTP"}, "are []"},
		{"not an interface", "net/http", "Request", "Response", nil, "is not an interface"},
		{"missing interface", "net/http", "Absent", "Response", nil, "declares no type Absent"},
		{"missing type", "net/http", "RoundTripper", "Absent", nil, "declares no type Absent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mustnotcompile.CheckReturning(c.pkg, c.iface, c.typ, c.methods...)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %v, want an error containing %q", err, c.reason)
			}
		})
	}
}

// http.RoundTripper's RoundTrip returns a Response pointer and an error.
func TestCheckResults(t *testing.T) {
	mustnotcompile.RequireResults(t, "net/http", "RoundTripper", "RoundTrip", "*Response", "error")
	cases := []struct {
		name, pkg, iface, method string
		results                  []string
		reason                   string
	}{
		{"a result left out", "net/http", "RoundTripper", "RoundTrip", []string{"*Response"}, `returns ["*Response" "error"]`},
		{"another type", "net/http", "RoundTripper", "RoundTrip", []string{"Response", "error"}, "returns"},
		{"missing method", "net/http", "RoundTripper", "Absent", nil, "has no method Absent"},
		{"not an interface", "net/http", "Request", "Write", nil, "is not an interface"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mustnotcompile.CheckResults(c.pkg, c.iface, c.method, c.results...)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %v, want an error containing %q", err, c.reason)
			}
		})
	}
}

// io.ReadCloser has exactly Close and Read, with their signatures.
func TestCheckMethods(t *testing.T) {
	mustnotcompile.RequireMethods(t, "io", "ReadCloser", "Close func() error", "Read func(p []byte) (n int, err error)")
	cases := []struct {
		name, pkg, iface string
		methods          []string
		reason           string
	}{
		{"a method left out", "io", "ReadCloser", []string{"Read func(p []byte) (n int, err error)"}, `has the methods ["Close func() error" "Read func(p []byte) (n int, err error)"]`},
		{"a method too many", "io", "Reader", []string{"Close func() error", "Read func(p []byte) (n int, err error)"}, "has the methods"},
		{"another signature", "io", "Reader", []string{"Read func(p []byte) error"}, "has the methods"},
		{"not an interface", "strings", "Builder", nil, "is not an interface"},
		{"missing interface", "io", "Absent", nil, "declares no type Absent"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			err := mustnotcompile.CheckMethods(c.pkg, c.iface, c.methods...)
			if err == nil || !strings.Contains(err.Error(), c.reason) {
				t.Fatalf("got %v, want an error containing %q", err, c.reason)
			}
		})
	}
}
