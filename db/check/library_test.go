package check_test

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"go.yaml.in/yaml/v3"
)

const module = "github.com/ppat/mediated-mailbox-mcp"

// library is a tree laid out as the data-access library. Every check runs over the real library and
// over the test library under testdata. The real library holds only what every check accepts, so a
// check run over it alone could pass having refused nothing. The test library holds content each check must accept
// or refuse, and the tests over it require exactly that, so a check weakened until it reads or
// refuses nothing turns them red.
type library struct {
	dir         string   // the root, relative to this package, where go test runs
	migrations  []string // the migration chain's directories, in the order they apply
	bootstrap   string   // SQL a superuser runs before the chain
	violations  string   // SQL violation files, or empty when the library holds none
	importLists string   // the golangci-lint configuration whose import lists admit components to subsections
	// libraryWide names the import lists that may admit the whole library, because none of them is a
	// component's own list.
	libraryWide []string
	// crossCutting names the import lists that span components, such as the lists over every pure
	// core or over files outside every component. None is a deployable's, so none runs a library's
	// statements under a role, and admitting a library's code through one is not a deployable
	// admitting the library.
	crossCutting []string
	// root is the directory the module's import paths are read from, the repository's root for the
	// real library, where a library list's globs name the directories its packages sit in.
	root string
}

var (
	realLibrary = library{
		dir:         "..",
		migrations:  []string{"../migrations"},
		bootstrap:   "../bootstrap",
		importLists: "../../.golangci.yaml",
		// The data-access library's own list, the test tooling lists over code that never ships, and the
		// lists over all non-test code and all ordinary tests, which leave each component to its own list.
		libraryWide:  []string{"db", "non-test-code", "ordinary-tests", "testsupport", "testsupport-without-rapid"},
		crossCutting: []string{"provider-test-code", "pure-core", "pure-core-tests", "residual"},
		root:         "../..",
	}
	testLibrary = library{
		dir:          "testdata",
		migrations:   []string{"../migrations", "testdata/migrations"},
		bootstrap:    "testdata/bootstrap",
		violations:   "testdata/violations",
		importLists:  "testdata/golangci.yaml",
		libraryWide:  []string{"db"},
		crossCutting: []string{"everything"},
		root:         "testdata",
	}
	// layoutLibrary holds only a sqlc.yaml whose blocks break the layout the checks rely on, and one
	// block that follows it and was never generated. sqlc never runs over it.
	layoutLibrary = library{
		dir:        "testdata/layout",
		migrations: []string{"../migrations", "testdata/migrations"},
	}
)

// subsection is one block of a library's sqlc.yaml, a generated package holding the accessors for one
// concern.
type subsection struct {
	name string // the package name, which is also its directory under the library's root
	dir  string
}

// subsections returns the library's subsections and fails the test on any block of its sqlc.yaml that
// breaks the layout, which layout describes.
func (lib library) subsections(t *testing.T) []subsection {
	t.Helper()
	subs, problems := lib.layout(t)
	requireNoProblems(t, problems)
	return subs
}

// layout reads the library's sqlc.yaml and checks that every block follows the one layout the checks
// rely on. Each block reads the migration chain and generates into the directory holding its statement
// files. A block breaking that layout is reported and is no subsection.
//
// sqlc refuses a configuration without a block and a block without a statement file, so a library
// holding no statement file has no sqlc.yaml. That returns no subsection. A statement file added
// without a block is then read by no check, which TestEverySQLFileIsRead refuses.
func (lib library) layout(t *testing.T) (subs []subsection, problems []string) {
	t.Helper()
	path := filepath.Join(lib.dir, "sqlc.yaml")
	src, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		t.Fatal(err)
	}
	var config struct {
		SQL []struct {
			Engine  string `yaml:"engine"`
			Schema  any    `yaml:"schema"`
			Queries any    `yaml:"queries"`
			Gen     struct {
				Go struct {
					Package string `yaml:"package"`
					Out     string `yaml:"out"`
				} `yaml:"go"`
			} `yaml:"gen"`
		} `yaml:"sql"`
	}
	if err := yaml.Unmarshal(src, &config); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	var chain []string
	for _, dir := range lib.migrations {
		rel, err := filepath.Rel(lib.dir, dir)
		if err != nil {
			t.Fatal(err)
		}
		chain = append(chain, filepath.ToSlash(rel))
	}
	for i, block := range config.SQL {
		gen := block.Gen.Go
		switch {
		case block.Engine != "postgresql" || !slices.Equal(stringList(block.Schema), chain):
			problems = append(problems, fmt.Sprintf("%s block %d must read the migration chain %v, with engine postgresql", path, i, chain))
		case block.Queries != gen.Out || gen.Out != gen.Package || strings.ContainsAny(gen.Out, `/\.`):
			problems = append(problems, fmt.Sprintf("%s block %d must generate package %q into the directory %q, which holds its statement files", path, i, gen.Package, gen.Package))
		default:
			subs = append(subs, subsection{name: gen.Package, dir: filepath.Join(lib.dir, gen.Package)})
		}
	}
	return subs, problems
}

func TestSubsectionLayoutReported(t *testing.T) {
	subs, problems := layoutLibrary.layout(t)
	chain := "[../../../migrations ../migrations]"
	requireProblems(t, problems, []string{
		"testdata/layout/sqlc.yaml block 1 must read the migration chain " + chain + ", with engine postgresql",
		"testdata/layout/sqlc.yaml block 2 must read the migration chain " + chain + ", with engine postgresql",
		`testdata/layout/sqlc.yaml block 3 must generate package "moved" into the directory "moved", which holds its statement files`,
		`testdata/layout/sqlc.yaml block 4 must generate package "other" into the directory "other", which holds its statement files`,
		`testdata/layout/sqlc.yaml block 5 must generate package "nested/dir" into the directory "nested/dir", which holds its statement files`,
	})
	if len(subs) != 1 || subs[0].name != "ungenerated" {
		t.Errorf("subsections = %v, want only the block that follows the layout, ungenerated", subs)
	}
}

// stringList reads a YAML value written either as one string or as a list of strings.
func stringList(v any) []string {
	switch v := v.(type) {
	case string:
		return []string{v}
	case []any:
		var out []string
		for _, item := range v {
			s, ok := item.(string)
			if !ok {
				return nil
			}
			out = append(out, s)
		}
		return out
	}
	return nil
}

func readYAML(t *testing.T, file string, into any) {
	t.Helper()
	src, err := os.ReadFile(file)
	if err != nil {
		t.Fatal(err)
	}
	if err := yaml.Unmarshal(src, into); err != nil {
		t.Fatalf("%s: %v", file, err)
	}
}

// requireProblems fails unless got holds exactly the problems in want, in any order.
func requireProblems(t *testing.T, got, want []string) {
	t.Helper()
	got, want = slices.Sorted(slices.Values(got)), slices.Sorted(slices.Values(want))
	if !slices.Equal(got, want) {
		t.Errorf("problems reported:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

// requireNoProblems fails with every problem in got.
func requireNoProblems(t *testing.T, got []string) {
	t.Helper()
	for _, p := range got {
		t.Error(p)
	}
}

// walkSkippingTestdata walks root, skipping any directory named testdata below it, which holds input
// for tests rather than library content.
func walkSkippingTestdata(root string, visit func(path string, d fs.DirEntry) error) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" && path != root {
			return filepath.SkipDir
		}
		return visit(path, d)
	})
}

func sameFile(t *testing.T, a, b string) bool {
	t.Helper()
	absA, errA := filepath.Abs(a)
	absB, errB := filepath.Abs(b)
	if errA != nil || errB != nil {
		t.Fatal(errA, errB)
	}
	return absA == absB
}

func TestTestLibrarySubsections(t *testing.T) {
	var got []string
	for _, s := range testLibrary.subsections(t) {
		got = append(got, s.name)
	}
	if want := []string{"counting", "listing", "reuse"}; !slices.Equal(got, want) {
		t.Errorf("test library subsections = %v, want %v", got, want)
	}
}
