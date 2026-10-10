package hunkmatch_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

// The tests write each patch with git diff in a scratch repository, so its shape is a real mutation
// patch's, move the file on afterwards as later work on the tree does, and hold the package's reading
// of the patch to what git apply does with it.

// runGit runs git in dir, which a test made, and returns its output. GIT_CEILING_DIRECTORIES keeps git
// from finding a repository above dir.
func runGit(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(dir))
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func git(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := runGit(dir, args...)
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return out
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// scratchRepo makes a git repository holding the files, committed. core.autocrlf is set to false in
// the repository, so a carriage return a test writes reaches git apply whatever the user's own
// configuration sets.
func scratchRepo(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init", "--quiet")
	git(t, root, "config", "core.autocrlf", "false")
	git(t, root, "config", "user.name", "test")
	git(t, root, "config", "user.email", "test@example.invalid")
	for name, content := range files {
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), content)
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
		writeFile(t, filepath.Join(root, filepath.FromSlash(name)), content)
	}
	out := git(t, root, "diff", "-U"+strconv.Itoa(context))
	git(t, root, "checkout", "--quiet", "--", ".")
	if out == "" {
		t.Fatal("the edits changed nothing")
	}
	return "control: a control\n\n" + out
}

// requireGitApplies requires git apply --check to accept the patch, so a refusal by the package is
// its own.
func requireGitApplies(t *testing.T, root, patch string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "p.patch")
	writeFile(t, path, patch)
	if out, err := runGit(root, "apply", "--check", path); err != nil {
		t.Fatalf("git apply --check refuses the patch, so the case does not reach the package: %v\n%s", err, out)
	}
}

// gitApplied applies the patch with git apply, returns what it left in each of the named files, and
// puts the tree back.
func gitApplied(t *testing.T, root, patch string, names []string) map[string]string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "p.patch")
	writeFile(t, path, patch)
	git(t, root, "apply", path)
	got := map[string]string{}
	for _, name := range names {
		data, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
		if err != nil {
			t.Fatal(err)
		}
		got[name] = string(data)
	}
	git(t, root, "reset", "--quiet", "--hard")
	git(t, root, "clean", "--quiet", "-fd")
	return got
}

func lines(n int, f func(i int) string) string {
	var b strings.Builder
	for i := 1; i <= n; i++ {
		b.WriteString(f(i))
		b.WriteString("\n")
	}
	return b.String()
}

// twoBlocks is a file in which the block at lines 11 to 13 repeats at lines 21 to 23, between
// lines that differ.
var twoBlocks = lines(30, func(i int) string {
	if i%10 >= 1 && i%10 <= 3 && i > 10 {
		return "same " + strconv.Itoa(i%10)
	}
	return "line " + strconv.Itoa(i)
})
