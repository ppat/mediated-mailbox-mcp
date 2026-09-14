// Package mustnotcompile asserts that a fixture package fails type checking with one expected
// message.
//
// A package whose types must refuse a construction keeps one fixture per case under
// testdata/mustnotcompile/<case>/ and calls Require from an ordinary test. The go command skips
// testdata, so the fixture never breaks a build, a vet run or a lint run.
//
// Check is strict because two looser assertions pass on broken fixtures. A fixture whose import
// path is mistyped also produces type errors, so the presence of a type error proves nothing. A
// message matching only a field name keeps passing after the field is renamed, because the error
// then reads "unknown field". So every error must be a type error, and every type error must
// contain the expected text.
package mustnotcompile

import (
	"errors"
	"fmt"
	"strings"
	"testing"

	"golang.org/x/tools/go/packages"
)

// Require fails the test unless Check reports no mismatch.
func Require(t testing.TB, dir, want string) {
	t.Helper()
	if err := Check(dir, want); err != nil {
		t.Fatal(err)
	}
}

// Check loads the package in dir, a path relative to the calling test's package directory, and
// returns an error unless loading it reports at least one error, every error is a type error, and
// every type error's message contains want.
func Check(dir, want string) error {
	if want == "" {
		return errors.New("mustnotcompile: the expected message is empty")
	}
	// NeedDeps is set because without it the loader also reports the compiler's message as a list
	// error, which would duplicate each type error under another kind.
	mode := packages.NeedName | packages.NeedImports | packages.NeedDeps | packages.NeedTypes |
		packages.NeedSyntax | packages.NeedTypesInfo
	pkgs, err := packages.Load(&packages.Config{Mode: mode}, dir)
	if err != nil {
		return fmt.Errorf("mustnotcompile: loading %s: %w", dir, err)
	}
	if len(pkgs) != 1 {
		return fmt.Errorf("mustnotcompile: %s matched %d packages, want exactly one", dir, len(pkgs))
	}
	pkg := pkgs[0]
	if len(pkg.Errors) == 0 {
		return fmt.Errorf("mustnotcompile: %s type-checks without error, want an error containing %q", dir, want)
	}
	var problems []string
	for _, e := range pkg.Errors {
		switch {
		case e.Kind != packages.TypeError:
			problems = append(problems, fmt.Sprintf("not a type error: %s", e))
		case !strings.Contains(e.Msg, want):
			problems = append(problems, fmt.Sprintf("unexpected type error: %s", e))
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("mustnotcompile: %s, want only type errors containing %q:\n%s",
			dir, want, strings.Join(problems, "\n"))
	}
	return nil
}
