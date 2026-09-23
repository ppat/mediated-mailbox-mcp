// Package mustnotcompile asserts that construction around a package's constructors does not compile.
// Require asserts that a fixture package fails type checking with one expected message, and
// RequireNoExportedFields asserts that named struct types expose no field, which catches a field
// added after the fixtures were written. RequireFields asserts a struct type's exact fields and
// RequireParams a function's exact parameters, so a verdict type gains no field and a decision gains
// no input unnoticed.
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
	"go/types"
	"slices"
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

// RequireNoExportedFields fails the test unless every named type in the package at pkgPath is a
// struct with no exported field. An exported field lets code outside the package build or change a
// value around its constructors, which no must-not-compile fixture written against today's fields
// can notice.
func RequireNoExportedFields(t testing.TB, pkgPath string, typeNames ...string) {
	t.Helper()
	if err := CheckNoExportedFields(pkgPath, typeNames...); err != nil {
		t.Fatal(err)
	}
}

// CheckNoExportedFields returns an error naming every listed type that is missing, is not a struct,
// or has an exported field.
func CheckNoExportedFields(pkgPath string, typeNames ...string) error {
	if len(typeNames) == 0 {
		return errors.New("mustnotcompile: no type names given")
	}
	pkg, err := load(pkgPath)
	if err != nil {
		return err
	}
	var problems []string
	for _, name := range typeNames {
		obj := pkg.Scope().Lookup(name)
		if obj == nil {
			problems = append(problems, fmt.Sprintf("%s declares no type %s", pkgPath, name))
			continue
		}
		st, ok := obj.Type().Underlying().(*types.Struct)
		if !ok {
			problems = append(problems, fmt.Sprintf("%s.%s is not a struct", pkgPath, name))
			continue
		}
		for i := range st.NumFields() {
			if f := st.Field(i); f.Exported() {
				problems = append(problems, fmt.Sprintf("%s.%s has the exported field %s", pkgPath, name, f.Name()))
			}
		}
	}
	if len(problems) > 0 {
		return fmt.Errorf("mustnotcompile: %s", strings.Join(problems, "\n"))
	}
	return nil
}

// RequireFields fails the test unless CheckFields reports no mismatch.
func RequireFields(t testing.TB, pkgPath, typeName string, fields ...string) {
	t.Helper()
	if err := CheckFields(pkgPath, typeName, fields...); err != nil {
		t.Fatal(err)
	}
}

// CheckFields returns an error unless the named struct type in the package at pkgPath has exactly
// fields, in order, each written as its name and type, such as "reason Reason". A field added under
// any name and of any type then fails, which a must-not-compile fixture naming one field cannot
// notice.
func CheckFields(pkgPath, typeName string, fields ...string) error {
	pkg, err := load(pkgPath)
	if err != nil {
		return err
	}
	obj := pkg.Scope().Lookup(typeName)
	if obj == nil {
		return fmt.Errorf("mustnotcompile: %s declares no type %s", pkgPath, typeName)
	}
	st, ok := obj.Type().Underlying().(*types.Struct)
	if !ok {
		return fmt.Errorf("mustnotcompile: %s.%s is not a struct", pkgPath, typeName)
	}
	got := make([]string, st.NumFields())
	for i := range st.NumFields() {
		f := st.Field(i)
		got[i] = f.Name() + " " + types.TypeString(f.Type(), qualifier(pkg))
	}
	if !slices.Equal(got, fields) {
		return fmt.Errorf("mustnotcompile: %s.%s has the fields %q, want %q", pkgPath, typeName, got, fields)
	}
	return nil
}

// RequireParams fails the test unless CheckParams reports no mismatch.
func RequireParams(t testing.TB, pkgPath, funcName string, params ...string) {
	t.Helper()
	if err := CheckParams(pkgPath, funcName, params...); err != nil {
		t.Fatal(err)
	}
}

// CheckParams returns an error unless the named function in the package at pkgPath takes exactly
// params, in order, each written as its type, such as "policy.Composed". A variadic parameter is
// written with its dots. A parameter added later, such as one a provider could arrive through, then
// fails.
func CheckParams(pkgPath, funcName string, params ...string) error {
	pkg, err := load(pkgPath)
	if err != nil {
		return err
	}
	fn, ok := pkg.Scope().Lookup(funcName).(*types.Func)
	if !ok {
		return fmt.Errorf("mustnotcompile: %s declares no function %s", pkgPath, funcName)
	}
	sig, ok := fn.Type().(*types.Signature)
	if !ok {
		return fmt.Errorf("mustnotcompile: %s.%s has no signature", pkgPath, funcName)
	}
	got := make([]string, sig.Params().Len())
	for i := range sig.Params().Len() {
		t := sig.Params().At(i).Type()
		if s, ok := t.(*types.Slice); ok && sig.Variadic() && i == sig.Params().Len()-1 {
			got[i] = "..." + types.TypeString(s.Elem(), qualifier(pkg))
			continue
		}
		got[i] = types.TypeString(t, qualifier(pkg))
	}
	if !slices.Equal(got, params) {
		return fmt.Errorf("mustnotcompile: %s.%s takes %q, want %q", pkgPath, funcName, got, params)
	}
	return nil
}

// load type-checks the package at pkgPath.
func load(pkgPath string) (*types.Package, error) {
	pkgs, err := packages.Load(&packages.Config{Mode: packages.NeedName | packages.NeedTypes}, pkgPath)
	if err != nil {
		return nil, fmt.Errorf("mustnotcompile: loading %s: %w", pkgPath, err)
	}
	if len(pkgs) != 1 || len(pkgs[0].Errors) > 0 || pkgs[0].Types == nil {
		return nil, fmt.Errorf("mustnotcompile: %s did not load as exactly one package without errors", pkgPath)
	}
	return pkgs[0].Types, nil
}

// qualifier writes a type from pkg unqualified and a type from any other package with its package
// name.
func qualifier(pkg *types.Package) types.Qualifier {
	return func(p *types.Package) string {
		if p == pkg {
			return ""
		}
		return p.Name()
	}
}
