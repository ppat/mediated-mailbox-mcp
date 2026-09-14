package check_test

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// statementFiles lists the statement files of every subsection in the library's sqlc.yaml.
func (lib library) statementFiles(t *testing.T) []string {
	t.Helper()
	var out []string
	for _, s := range lib.subsections(t) {
		out = append(out, sqlFiles(t, s.dir)...)
	}
	return out
}

func statementFindings(t *testing.T, lib library) []string {
	t.Helper()
	s := readSchema(t, lib.migrationChain(t))
	var out []string
	for _, path := range lib.statementFiles(t) {
		for _, f := range checkStatements(s, parseFile(t, path)) {
			out = append(out, f.String())
		}
	}
	return out
}

func TestStatementFiles(t *testing.T) {
	requireNoProblems(t, statementFindings(t, realLibrary))
}

// TestTestLibraryStatementFiles shows the gating run reading the statement files sqlc.yaml lists, and
// the checks accepting statements that break no constraint.
func TestTestLibraryStatementFiles(t *testing.T) {
	want := []string{"testdata/counting/senders.sql", "testdata/listing/senders.sql", "testdata/reuse/sender.sql"}
	if got := testLibrary.statementFiles(t); !slices.Equal(got, want) {
		t.Errorf("statement files read = %v, want %v", got, want)
	}
	requireNoProblems(t, statementFindings(t, testLibrary))
}

func TestMigrationAndBootstrapFiles(t *testing.T) {
	for _, lib := range []library{realLibrary, testLibrary} {
		files := lib.migrationChain(t)
		for _, path := range sqlFiles(t, lib.bootstrap) {
			files = append(files, parseFile(t, path))
		}
		for _, f := range files {
			for _, found := range checkMigration(f) {
				t.Error(found)
			}
		}
	}
}

// TestViolationFiles runs the checks over SQL files that each break checks on purpose. Each file states
// the checks it must fail in want annotations, and the test fails unless exactly those are reported. A
// check weakened until it refuses nothing therefore turns this test red.
func TestViolationFiles(t *testing.T) {
	s := readSchema(t, testLibrary.migrationChain(t))
	covered := map[string]bool{}
	run := func(dir string, check func(sqlFile) []finding) {
		paths := sqlFiles(t, filepath.Join(testLibrary.violations, dir))
		if len(paths) == 0 {
			t.Errorf("no violation files in %s", filepath.Join(testLibrary.violations, dir))
		}
		for _, path := range paths {
			f := parseFile(t, path)
			want := wants(f.src)
			if len(want) == 0 {
				t.Errorf("%s states no want", path)
			}
			var got []string
			for _, found := range check(f) {
				got = append(got, found.check)
				t.Log(found)
			}
			slices.Sort(want)
			slices.Sort(got)
			if !slices.Equal(got, want) {
				t.Errorf("%s: reported %v, want %v", path, got, want)
			}
			for _, w := range want {
				covered[w] = true
			}
		}
	}
	run("statements", func(f sqlFile) []finding { return checkStatements(s, f) })
	run("migrations", checkMigration)
	for _, check := range append(slices.Clone(statementChecks), checkDatabaseCode) {
		if !covered[check] {
			t.Errorf("no violation file wants %s, so nothing shows that check still refuses anything", check)
		}
	}
}

// unreadSQLFiles reports a SQL file in the library that no check reads, such as statements in a
// directory sqlc.yaml does not list, which would otherwise escape every check here.
func unreadSQLFiles(t *testing.T, lib library) []string {
	t.Helper()
	read := lib.statementFiles(t)
	for _, dir := range append(slices.Clone(lib.migrations), lib.bootstrap) {
		read = append(read, sqlFiles(t, dir)...)
	}
	if lib.violations != "" {
		read = append(read, sqlFiles(t, filepath.Join(lib.violations, "statements"))...)
		read = append(read, sqlFiles(t, filepath.Join(lib.violations, "migrations"))...)
	}
	var out []string
	err := walkSkippingTestdata(lib.dir, func(path string, d fs.DirEntry) error {
		if !d.IsDir() && strings.HasSuffix(path, ".sql") && !slices.ContainsFunc(read, func(r string) bool { return sameFile(t, r, path) }) {
			out = append(out, fmt.Sprintf("%s is read by no check", path))
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return out
}

func TestEverySQLFileIsRead(t *testing.T) {
	requireNoProblems(t, unreadSQLFiles(t, realLibrary))
}

func TestUnreadSQLFileReported(t *testing.T) {
	requireProblems(t, unreadSQLFiles(t, testLibrary), []string{"testdata/stray/unread.sql is read by no check"})
}
