package check_test

import (
	"bytes"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// sqlcDiff copies the real migration chain and the test library into a temporary directory, keeping
// the relative layout the test library's sqlc.yaml reads them through, lets change edit the copy of
// the test library, and runs sqlc diff in it as the data workflow does. The checked-in files are never
// touched.
//
// A missing sqlc fails the test rather than skipping it, because a skipped test passes in the job that
// was meant to run it.
func sqlcDiff(t *testing.T, change func(lib string)) (string, error) {
	t.Helper()
	sqlc, err := exec.LookPath("sqlc")
	if err != nil {
		t.Fatalf("sqlc is not on PATH. Run the tests through mise: %v", err)
	}
	root := t.TempDir()
	lib := filepath.Join(root, "db", "check", "testdata")
	if err := os.CopyFS(filepath.Join(root, "db", "migrations"), os.DirFS(filepath.Join("..", "migrations"))); err != nil {
		t.Fatal(err)
	}
	if err := os.CopyFS(lib, os.DirFS("testdata")); err != nil {
		t.Fatal(err)
	}
	change(lib)
	cmd := exec.CommandContext(t.Context(), sqlc, "diff")
	cmd.Dir = lib
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	var exit *exec.ExitError
	if err != nil && !errors.As(err, &exit) {
		t.Fatalf("running sqlc diff: %v", err)
	}
	return out.String(), err
}

// requireDrift requires sqlc diff to have failed with a diff of each named generated file.
func requireDrift(t *testing.T, out string, err error, files ...string) {
	t.Helper()
	if err == nil {
		t.Fatalf("sqlc diff passed on a copy whose generated code is out of date\n%s", out)
	}
	for _, file := range files {
		if !strings.Contains(out, "--- a/"+file+"\n") {
			t.Errorf("sqlc diff did not report %s\n%s", file, out)
		}
	}
}

// TestGeneratorDiffRefusesDrift proves the regenerate-and-diff check of ADR-0066 against the test
// library, because the real library holds no statement file yet. The untouched copy must pass, so each
// failure below comes from the change made to its copy and not from the copy itself.
func TestGeneratorDiffRefusesDrift(t *testing.T) {
	t.Run("untouched", func(t *testing.T) {
		out, err := sqlcDiff(t, func(string) {})
		if err != nil {
			t.Fatalf("sqlc diff failed on an untouched copy of the test library: %v\n%s", err, out)
		}
	})

	t.Run("generated file edited by hand", func(t *testing.T) {
		out, err := sqlcDiff(t, func(lib string) {
			root, err := os.OpenRoot(lib)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := root.Close(); err != nil {
					t.Error(err)
				}
			}()
			const path = "counting/senders.sql.go"
			src, err := root.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			// Failing when the field is not there keeps a regenerated file from turning the edit into no edit.
			edited := bytes.Replace(src, []byte("Added     int64"), []byte("Added     int32"), 1)
			if bytes.Equal(edited, src) {
				t.Fatalf("%s no longer declares Added as int64, so the edit changes nothing", path)
			}
			if err := root.WriteFile(path, edited, 0o600); err != nil {
				t.Fatal(err)
			}
		})
		requireDrift(t, out, err, "counting/senders.sql.go")
	})

	// counting's statement adds to fixture_senders.message_count, listing's selects it, and reuse's
	// models.go holds the table's struct, so all three change once it is an integer.
	t.Run("column type changed without regenerating", func(t *testing.T) {
		out, err := sqlcDiff(t, func(lib string) {
			migration := "-- +goose Up\nALTER TABLE fixture_senders ALTER COLUMN message_count TYPE integer;\n"
			if err := os.WriteFile(filepath.Join(lib, "migrations", "00002_fixture_count_type.sql"), []byte(migration), 0o600); err != nil {
				t.Fatal(err)
			}
		})
		requireDrift(t, out, err, "counting/senders.sql.go", "listing/senders.sql.go", "reuse/models.go")
	})
}
