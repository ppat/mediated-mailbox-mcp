package check_test

import (
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"testing"
)

// checkGeneration is sqlc's own check that every statement typechecks against the migration chain,
// named as a generation violation file's want annotation states it.
const checkGeneration = "generation"

// bodyColumn is the column every generation violation file reads, and the one sqlc must report
// missing. A failure on any other column would leave the check green with a body column in the schema.
const bodyColumn = "body"

// TestGenerationRefusesAColumnTheSchemaDoesNotHold places each generation violation file in a
// subsection of a copy of the test library, whose configuration reads the real migration chain, and
// requires sqlc generate to fail there, naming the file and the body column as the one that does not
// exist. A statement reading a body column then cannot produce a result type (ADR-0047, on
// ADR-0016's absence). The untouched copy must generate, so each failure is the violation file's
// doing.
func TestGenerationRefusesAColumnTheSchemaDoesNotHold(t *testing.T) {
	t.Run("untouched", func(t *testing.T) {
		if out, err := sqlcIn(t, func(string) {}, "generate"); err != nil {
			t.Fatalf("sqlc generate failed on an untouched copy of the test library: %v\n%s", err, out)
		}
	})
	paths := sqlFiles(t, filepath.Join(testLibrary.violations, checkGeneration))
	if len(paths) == 0 {
		t.Fatalf("no violation files in %s", filepath.Join(testLibrary.violations, checkGeneration))
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			src, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			if want := wants(string(src)); !slices.Equal(want, []string{checkGeneration}) {
				t.Fatalf("%s wants %v, want exactly %s", path, want, checkGeneration)
			}
			name := filepath.Base(path)
			out, err := sqlcIn(t, func(lib string) {
				//nolint:gosec // The name is a checked-in violation file's, written into the test's own copy.
				if err := os.WriteFile(filepath.Join(lib, "listing", name), src, 0o600); err != nil {
					t.Fatal(err)
				}
			}, "generate")
			if err == nil {
				t.Fatalf("sqlc generate passed with %s in the test library\n%s", name, out)
			}
			if !regexp.MustCompile(regexp.QuoteMeta(name) + `:\d+:\d+: column "` + bodyColumn + `" does not exist`).MatchString(out) {
				t.Errorf("sqlc generate failed, but not on a missing %s column in %s\n%s", bodyColumn, name, out)
			}
		})
	}
}
