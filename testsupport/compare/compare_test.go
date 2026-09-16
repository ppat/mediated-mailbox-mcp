package compare

import (
	"fmt"
	"go/types"
	"maps"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"golang.org/x/tools/go/packages"
)

const module = "github.com/ppat/mediated-mailbox-mcp"

// unresolvedNames returns the entries of a readable type list that name no type, which is what a
// rename or a typo leaves behind. The compiler cannot catch it because the entries are strings.
func unresolvedNames(t *testing.T, names map[string]struct{}) []string {
	t.Helper()
	if len(names) == 0 {
		return nil
	}
	byPackage := map[string][]string{}
	var out []string
	for name := range names {
		i := strings.LastIndex(name, ".")
		if i < 0 {
			out = append(out, fmt.Sprintf("entry %q has no package path", name))
			continue
		}
		byPackage[name[:i]] = append(byPackage[name[:i]], name[i+1:])
	}
	// Tests is set because a listed type may be defined in a test file.
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedTypes | packages.NeedSyntax, Tests: true}
	loaded, err := packages.Load(cfg, slices.Sorted(maps.Keys(byPackage))...)
	if err != nil {
		t.Fatalf("loading listed packages: %v", err)
	}
	found := map[string]bool{}
	for _, pkg := range loaded {
		for _, e := range pkg.Errors {
			t.Errorf("loading %s: %v", pkg.ID, e)
		}
		if pkg.Types == nil {
			continue
		}
		for _, name := range byPackage[pkg.PkgPath] {
			if _, ok := pkg.Types.Scope().Lookup(name).(*types.TypeName); ok {
				found[pkg.PkgPath+"."+name] = true
			}
		}
	}
	for name := range names {
		if !found[name] && strings.Contains(name, ".") {
			out = append(out, fmt.Sprintf("readable type entry %q names no type", name))
		}
	}
	slices.Sort(out)
	return out
}

func TestEveryListedTypeExists(t *testing.T) {
	for _, problem := range unresolvedNames(t, readableTypes) {
		t.Error(problem)
	}
}

// listed is defined in this test file and granted by the lists below. A type in any package is
// granted the same way, by name, so this package never imports the package defining it.
type listed struct{ hidden int }

func TestUnresolvedNameReported(t *testing.T) {
	names := map[string]struct{}{
		module + "/testsupport/compare.listed":  {},
		module + "/testsupport/compare.renamed": {},
	}
	got := unresolvedNames(t, names)
	want := []string{`readable type entry "` + module + `/testsupport/compare.renamed" names no type`}
	if !slices.Equal(got, want) {
		t.Errorf("unresolved names = %v, want %v", got, want)
	}
}

// TestListedTypeReadable shows a listed type compared through its unexported fields. The report
// names the field, which only happens when the options granted access to it.
func TestListedTypeReadable(t *testing.T) {
	listing := options(map[string]struct{}{module + "/testsupport/compare.listed": {}})
	if diff := cmp.Diff(listed{hidden: 1}, listed{hidden: 2}, listing); !strings.Contains(diff, "hidden") {
		t.Fatalf("diff does not report the unexported field:\n%s", diff)
	}
}

type unlisted struct{ hidden int }

// TestUnlistedTypePanicsNamingIt holds the behaviour the shared value relies on. Comparing a type
// that is not listed panics, and the message names the type, so a missing entry is loud and local
// rather than a silent pass.
func TestUnlistedTypePanicsNamingIt(t *testing.T) {
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("cmp.Diff compared an unlisted type with unexported fields without panicking")
		}
		if msg := fmt.Sprint(r); !strings.Contains(msg, "compare.unlisted") {
			t.Fatalf("panic does not name the unlisted type: %s", msg)
		}
	}()
	_ = cmp.Diff(unlisted{hidden: 1}, unlisted{hidden: 2}, Options)
}

// TestImportsNoProjectPackage holds what lets an in-package test of any package, one under a
// deployable's internal directory included, import this package. An import of a project package here
// would form an import cycle with that package's tests.
func TestImportsNoProjectPackage(t *testing.T) {
	cfg := &packages.Config{Mode: packages.NeedName | packages.NeedImports | packages.NeedDeps}
	loaded, err := packages.Load(cfg, module+"/testsupport/compare")
	if err != nil {
		t.Fatal(err)
	}
	packages.Visit(loaded, nil, func(pkg *packages.Package) {
		for _, e := range pkg.Errors {
			t.Errorf("loading %s: %v", pkg.ID, e)
		}
		if pkg.PkgPath != module+"/testsupport/compare" && strings.HasPrefix(pkg.PkgPath, module+"/") {
			t.Errorf("imports project package %s", pkg.PkgPath)
		}
	})
}
