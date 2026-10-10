package check_test

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// nameComment matches the name a statement's name comment gives it, in a statement file or in a Go
// string the library writes by hand, as db/tx's statements carry.
var nameComment = regexp.MustCompile(`-- name: ([A-Za-z0-9_]+)`)

// namedSources returns the files of the library that give statements a name, the statement files of
// every subsection and every hand-written non-test Go file outside testdata, where db/tx's own
// statements carry name comments. A generated Go file repeats its statement file's names and is left
// out.
func (lib library) namedSources(t *testing.T) []string {
	t.Helper()
	paths := lib.statementFiles(t)
	err := filepath.WalkDir(lib.dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "testdata" {
			return filepath.SkipDir
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") || strings.HasSuffix(path, "_violation.go") {
			return nil
		}
		src, err := os.ReadFile(path) //nolint:gosec // The path is a file of the library the walk found.
		if err != nil {
			return err
		}
		if !strings.HasPrefix(string(src), "// Code generated") {
			paths = append(paths, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	slices.Sort(paths)
	return paths
}

// repeatedStatementNames returns a problem for each statement name given more than once across paths,
// naming every file that gives it. The statement latency series labels a statement by its name, so a
// name given twice would merge two statements' series into one (ADR-0125).
func repeatedStatementNames(t *testing.T, paths []string) []string {
	t.Helper()
	givenIn := map[string][]string{}
	for _, path := range paths {
		src, err := os.ReadFile(path) //nolint:gosec // The path is a file of the library.
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range nameComment.FindAllStringSubmatch(string(src), -1) {
			givenIn[m[1]] = append(givenIn[m[1]], path)
		}
	}
	var problems []string
	for name, files := range givenIn {
		if len(files) > 1 {
			problems = append(problems, fmt.Sprintf("the statement name %s is given in %s", name, strings.Join(files, ", ")))
		}
	}
	slices.Sort(problems)
	return problems
}

// TestStatementNamesAreUnique holds every statement name of the library, db/tx's included, to one
// statement.
func TestStatementNamesAreUnique(t *testing.T) {
	paths := realLibrary.namedSources(t)
	if !slices.Contains(paths, filepath.Join("..", "tx", "tx.go")) {
		t.Errorf("the named sources %v leave out db/tx's statements", paths)
	}
	requireNoProblems(t, repeatedStatementNames(t, paths))
}

// TestRepeatedStatementNameReported shows the check reporting the names a violation file shares with
// the base policy's subsection, each with both files.
func TestRepeatedStatementNameReported(t *testing.T) {
	base := filepath.Join("..", "policyrules", "base", "base.sql")
	violation := filepath.Join("testdata", "violations", "base", "base_policy_violation.sql")
	got := repeatedStatementNames(t, []string{base, violation})
	var want []string
	for _, name := range []string{"AddBaseRule", "BaseRules", "EditBaseRule", "LiftBaseRule", "RecordBaseChange"} {
		want = append(want, "the statement name "+name+" is given in "+base+", "+violation)
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("repeated names (-want +got):\n%s", diff)
	}
}
