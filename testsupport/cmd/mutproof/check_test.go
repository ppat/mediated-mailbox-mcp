package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// scratchRepo makes a git repository holding the files, committed, with core.autocrlf set to false in
// the repository.
func scratchRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	git(t, root, "config", "core.autocrlf", "false")
	git(t, root, "config", "user.name", "test")
	git(t, root, "config", "user.email", "test@example.invalid")
	for name, content := range files {
		writeFile(t, mustMkdir(t, filepath.Dir(filepath.Join(root, filepath.FromSlash(name))), filepath.Base(name)), content, 0o600)
	}
	git(t, root, "add", "--all")
	git(t, root, "commit", "--quiet", "-m", "baseline")
	return root
}

// diffOf writes the edits over the committed files, takes git diff with the given lines of context,
// and puts the files back, returning the patch with a preamble line before it.
func diffOf(t *testing.T, root string, context int, edits map[string]string) string {
	t.Helper()
	for name, content := range edits {
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), content, 0o600)
	}
	cmd := exec.Command("git", "diff", "-U"+strconv.Itoa(context))
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git diff: %v", err)
	}
	git(t, root, "checkout", "--quiet", "--", ".")
	return "control: a control\n\n" + string(out)
}

// twoBlocks is a file in which the block at lines 11 to 13 repeats at lines 21 to 23, between
// lines that differ.
var twoBlocks = func() string {
	var b strings.Builder
	for i := 1; i <= 30; i++ {
		if i%10 >= 1 && i%10 <= 3 && i > 10 {
			fmt.Fprintf(&b, "same %d\n", i%10)
		} else {
			fmt.Fprintf(&b, "line %d\n", i)
		}
	}
	return b.String()
}()

// checkRoot is a repository holding twoBlocks in pkg/f.txt.
func checkRoot(t *testing.T) string {
	t.Helper()
	return scratchRepo(t, map[string]string{"pkg/f.txt": twoBlocks})
}

// unique, repeated and stale are patches over pkg/f.txt. The first applies at one place, the second
// has a hunk whose context matches twice, and the third no longer applies.
func unique(root string, t *testing.T) string {
	return diffOf(t, root, 3, map[string]string{"pkg/f.txt": strings.Replace(twoBlocks, "line 5\n", "mutated\n", 1)})
}

func repeated(root string, t *testing.T) string {
	return diffOf(t, root, 1, map[string]string{"pkg/f.txt": strings.Replace(twoBlocks, "same 2\n", "mutated\n", 1)})
}

func stale(root string, t *testing.T) string {
	return strings.Replace(unique(root, t), " line 4\n", " line four\n", 1)
}

// existing creates pkg/f.txt, which exists, so git apply --check refuses it though its one hunk
// has one place.
func existing(string, *testing.T) string {
	return "diff --git a/pkg/f.txt b/pkg/f.txt\nnew file mode 100644\n--- /dev/null\n+++ b/pkg/f.txt\n@@ -0,0 +1 @@\n+created\n"
}

func TestCheckTree(t *testing.T) {
	type patches = map[string]func(string, *testing.T) string
	cases := []struct {
		name    string
		patches patches
		// refused names the patches the check must refuse, and none means it must pass.
		refused []string
		wantErr string
	}{
		{"unique", patches{"pkg/testdata/mutations/a.patch": unique}, nil, ""},
		{"nested", patches{"pkg/testdata/mutations/deeper/a.patch": repeated}, []string{"pkg/testdata/mutations/deeper/a.patch"}, "1 of 1 mutation patches refused"},
		{"repeated", patches{"pkg/testdata/mutations/a.patch": unique, "pkg/testdata/mutations/b.patch": repeated}, []string{"pkg/testdata/mutations/b.patch"}, "1 of 2 mutation patches refused"},
		{"stale", patches{"pkg/testdata/mutations/a.patch": stale}, []string{"pkg/testdata/mutations/a.patch"}, "1 of 1 mutation patches refused"},
		{"creates a file that exists", patches{"pkg/testdata/mutations/a.patch": existing}, []string{"pkg/testdata/mutations/a.patch"}, "1 of 1 mutation patches refused"},
		{"fixtures left out", patches{"pkg/testdata/mutations/a.patch": unique, "testsupport/cmd/mutproof/testdata/fixture/pkg/testdata/mutations/b.patch": repeated, "testsupport/cmd/mutproof/testdata/fixture/pkg/testdata/mutations/c.patch": stale}, nil, ""},
		{"runner's own patches checked", patches{"testsupport/cmd/mutproof/testdata/mutations/a.patch": repeated}, []string{"testsupport/cmd/mutproof/testdata/mutations/a.patch"}, "1 of 1 mutation patches refused"},
		{"not a mutation patch", patches{"pkg/testdata/mutations/a.patch": unique, "pkg/testdata/other/b.patch": repeated, "pkg/testdata/mutations/c.diff": repeated}, nil, ""},
		{"no patch", patches{"pkg/testdata/other/b.patch": unique}, nil, "found no mutation patch"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := checkRoot(t)
			for rel, patch := range c.patches {
				path := filepath.Join(root, filepath.FromSlash(rel))
				if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
					t.Fatal(err)
				}
				writeFile(t, path, patch(root, t), 0o600)
			}
			var out bytes.Buffer
			err := checkTree(&out, root)
			if c.wantErr == "" && err != nil || c.wantErr != "" && (err == nil || !strings.Contains(err.Error(), c.wantErr)) {
				t.Fatalf("got error %v, want %q\n%s", err, c.wantErr, out.String())
			}
			for rel := range c.patches {
				annotated := strings.Contains(out.String(), "::error file="+rel+"::")
				want := false
				for _, r := range c.refused {
					want = want || r == rel
				}
				if annotated != want {
					t.Errorf("%s refused %v, want %v:\n%s", rel, annotated, want, out.String())
				}
			}
		})
	}
}

// TestCheckTreeRefusesAnUnreadableDirectory sets beside a patch that passes a mutation directory
// the check cannot read, so only a refusal to trust an incomplete walk can fail the check.
func TestCheckTreeRefusesAnUnreadableDirectory(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("permissions do not bind root")
	}
	root := checkRoot(t)
	writeFile(t, mustMkdir(t, filepath.Join(root, "pkg", "testdata", "mutations"), "a.patch"), unique(root, t), 0o600)
	hidden := filepath.Join(root, "other", "testdata", "mutations")
	writeFile(t, mustMkdir(t, hidden, "b.patch"), repeated(root, t), 0o600)
	if err := os.Chmod(hidden, 0); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := os.Chmod(hidden, 0o750); err != nil { //nolint:gosec // The test's own temporary directory.
			t.Error(err)
		}
	})
	var out bytes.Buffer
	if err := checkTree(&out, root); err == nil || !strings.Contains(err.Error(), "listing the mutation patches") {
		t.Fatalf("got error %v, want the unreadable directory refused\n%s", err, out.String())
	}
}

func mustMkdir(t *testing.T, dir, name string) string {
	t.Helper()
	if err := os.MkdirAll(dir, 0o750); err != nil {
		t.Fatal(err)
	}
	return filepath.Join(dir, name)
}
