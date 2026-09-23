package property

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"pgregory.net/rapid"
)

// storeDir is where each package keeps its stored failing cases, one file per test, relative to the
// package directory a test runs in.
const storeDir = "testdata/property"

// stored is the file the failing-case store keeps for one test.
type stored[A any] struct {
	// Shape describes the argument struct the cases were written with. A case written with another
	// shape would build a different value, so a mismatch fails the run rather than replaying it.
	Shape string `json:"shape"`
	Cases []A    `json:"cases"`
}

// Check runs prop over arguments from draw, after replaying every failing case the store keeps for
// the test.
//
// A stored case is replayed first, each as its own subtest. When generation finds a failure, the
// reduced case is added to the store from a cleanup registered before rapid runs, because rapid ends
// a failing run with runtime.Goexit and nothing placed after it runs on that run. The store keeps the
// arguments, so a stored case still builds the value that failed after draw is edited. Write the
// reduced case out as an example-based test as well, stating which fields matter.
func Check[A any](t *testing.T, draw func(*rapid.T) A, prop func(rapid.TB, A)) {
	t.Helper()
	requireSettings(t)
	shape, err := describe(reflect.TypeFor[A]())
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(storeDir, strings.ReplaceAll(t.Name(), "/", "_")+".json")
	kept, err := load[A](path, shape)
	if err != nil {
		t.Fatal(err)
	}
	for i, a := range kept.Cases {
		t.Run(fmt.Sprintf("stored case %d", i), func(t *testing.T) { prop(t, a) })
	}
	if t.Failed() {
		t.Fatalf("a stored failing case in %s still fails. Fix the code, then delete the case once its example-based test exists", path)
	}

	var last A
	drew := false
	t.Cleanup(func() {
		// The test can fail for a reason other than the property, such as a check after Check or
		// rapid refusing too many draws. Only a case that fails the property on its own is stored.
		if !t.Failed() || !drew || !fails(t.Name(), prop, last) {
			return
		}
		kept.Shape = shape
		if !slices.ContainsFunc(kept.Cases, func(c A) bool { return reflect.DeepEqual(c, last) }) {
			kept.Cases = append(kept.Cases, last)
		}
		if err := save(path, kept); err != nil {
			t.Errorf("the failing case could not be stored: %v", err)
			return
		}
		t.Logf("stored the failing case %+v in %s. Commit it, and write it out as an example-based test", last, path)
	})
	rapid.Check(t, func(rt *rapid.T) {
		a := draw(rt)
		// rapid replays the reduced failing case last, so this holds that case when the cleanup runs.
		last, drew = a, true
		prop(rt, a)
	})
}

func load[A any](path, shape string) (stored[A], error) {
	var kept stored[A]
	src, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return kept, nil
	}
	if err != nil {
		return kept, err
	}
	if err := json.Unmarshal(src, &kept); err != nil {
		return kept, fmt.Errorf("%s cannot be read as stored failing cases: %w", path, err)
	}
	if kept.Shape != shape {
		return kept, fmt.Errorf("the stored failing cases in %s were written for the arguments %s, and the draw function now returns %s. "+
			"Write each stored case out as an example-based test against the new arguments, then delete %s", path, kept.Shape, shape, path)
	}
	return kept, nil
}

func save[A any](path string, kept stored[A]) error {
	src, err := json.MarshalIndent(kept, "", "  ")
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		return err
	}
	return os.WriteFile(path, append(src, '\n'), 0o600)
}

// describe returns a description of an argument type that changes whenever a stored case would stop
// meaning the same arguments. It refuses a type the store cannot write whole, such as one with an
// unexported field, which would be stored without that field.
func describe(t reflect.Type) (string, error) {
	switch t.Kind() {
	case reflect.Bool, reflect.String,
		reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64,
		reflect.Float32, reflect.Float64:
		return t.Kind().String(), nil
	case reflect.Slice:
		elem, err := describe(t.Elem())
		return "[]" + elem, err
	case reflect.Struct:
		fields := make([]string, 0, t.NumField())
		for i := range t.NumField() {
			f := t.Field(i)
			if !f.IsExported() {
				return "", fmt.Errorf("the argument type %s has the unexported field %s, which the failing-case store cannot keep", t, f.Name)
			}
			d, err := describe(f.Type)
			if err != nil {
				return "", err
			}
			fields = append(fields, f.Name+" "+d)
		}
		return "struct{" + strings.Join(fields, "; ") + "}", nil
	case reflect.Invalid, reflect.Uintptr, reflect.Complex64, reflect.Complex128, reflect.Array, reflect.Chan,
		reflect.Func, reflect.Interface, reflect.Map, reflect.Pointer, reflect.UnsafePointer:
		return "", fmt.Errorf("the argument type %s cannot be kept by the failing-case store. Use booleans, numbers, strings, slices and structs of them", t)
	default:
		return "", fmt.Errorf("the argument type %s cannot be kept by the failing-case store", t)
	}
}
