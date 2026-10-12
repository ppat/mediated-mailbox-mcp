package main

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// workingTreeFiles lists the working tree as git sees it, relative to root. That is the tracked
// files and the untracked files git does not ignore, so an author's uncommitted control and patch
// are included and ignored build output, such as node_modules, is not. --deduplicate lists a path
// with a merge conflict once rather than once per stage.
func workingTreeFiles(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z", "--cached", "--others", "--exclude-standard", "--deduplicate")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w\n%s", err, stderr.String())
	}
	var files []string
	for path := range strings.SplitSeq(string(out), "\x00") {
		if path != "" {
			files = append(files, path)
		}
	}
	return files, nil
}

// builtBundle is the directory, relative to the repository root, the browser bundle is built into
// (ui/browser/scripts/build.ts) and ui/main.go embeds. git ignores what the build writes there, and
// the UI's tests read it, so a copy without it holds only the checked-in placeholder, against which
// those tests skip, check less or fail. The copies carry the checkout's bundle as it stands, built
// or not, fresh or stale, as go test run in the checkout reads it.
const builtBundle = "ui/browser/dist"

// withBuiltBundle adds to files, relative to root, every file under the checkout's built bundle that
// files does not already list. Each is added as a file, so copyTree refuses anything but a regular
// file there as it does elsewhere, which Go's embed of the bundle refuses too.
func withBuiltBundle(root string, files []string) ([]string, error) {
	listed := make(map[string]bool, len(files))
	for _, f := range files {
		listed[f] = true
	}
	all := slices.Clone(files)
	err := filepath.WalkDir(filepath.Join(root, filepath.FromSlash(builtBundle)), func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		if rel = filepath.ToSlash(rel); !listed[rel] {
			all = append(all, rel)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return files, nil
	}
	return all, err
}

// copyTree copies the files, relative to root, into a new temporary directory and returns it. The
// caller removes it. A tracked file deleted from the working tree is left out, as it is from the
// working tree. Anything but a regular file is refused, since a copy that followed a symbolic link
// would differ from the tree it stands for.
func copyTree(root string, files []string) (_ string, err error) {
	dir, err := os.MkdirTemp("", "mutproof-")
	if err != nil {
		return "", err
	}
	defer func() {
		if err != nil {
			err = errors.Join(err, os.RemoveAll(dir))
		}
	}()
	for _, rel := range files {
		src := filepath.Join(root, filepath.FromSlash(rel))
		info, err := os.Lstat(src)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			continue
		case err != nil:
			return "", err
		case !info.Mode().IsRegular():
			return "", fmt.Errorf("%s is not a regular file, and the copy the runner works in holds regular files only", rel)
		}
		dst := filepath.Join(dir, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(dst), 0o750); err != nil {
			return "", err
		}
		if err := copyFile(src, dst, info.Mode().Perm()); err != nil {
			return "", err
		}
	}
	return dir, nil
}

// copyFile streams src into a new file dst with the given permission bits.
func copyFile(src, dst string, perm fs.FileMode) (err error) {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { err = errors.Join(err, in.Close()) }()
	//nolint:gosec // A path git ls-files names under root, written into the copy.
	out, err := os.OpenFile(dst, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		return errors.Join(err, out.Close())
	}
	return out.Close()
}

// touchedPaths lists every path the patch at patchPath creates, changes or deletes, relative to root.
// git apply --numstat names each file by its path after the patch, so the source of a rename, which
// the patch deletes, is read from the patch's own rename from lines.
func touchedPaths(root, patchPath string, src []byte) ([]string, error) {
	out, err := gitApply(root, "--numstat", "-z", patchPath)
	if err != nil {
		return nil, err
	}
	// Each record is "added<TAB>deleted<TAB>path<NUL>".
	var paths []string
	for record := range strings.SplitSeq(string(out), "\x00") {
		if record == "" {
			continue
		}
		parts := strings.SplitN(record, "\t", 3)
		if len(parts) != 3 || parts[2] == "" {
			return nil, fmt.Errorf("unexpected git apply --numstat record %q", record)
		}
		paths = append(paths, parts[2])
	}
	sources, err := renameSources(src)
	if err != nil {
		return nil, err
	}
	paths = append(paths, sources...)
	if len(paths) == 0 {
		return nil, errors.New("the patch touches no file")
	}
	return paths, nil
}

// renameSources returns the rename from paths in the extended headers of a patch, which run from a
// diff --git line to the file's first hunk. Hunk lines always start with a space, + or -, so a line
// of changed code can never be read as a header.
func renameSources(src []byte) ([]string, error) {
	var sources []string
	inHeader := false
	for line := range strings.Lines(string(src)) {
		line = strings.TrimSuffix(line, "\n")
		switch {
		case strings.HasPrefix(line, "diff --git "):
			inHeader = true
		case strings.HasPrefix(line, "@@"):
			inHeader = false
		case inHeader && strings.HasPrefix(line, "rename from "):
			path := strings.TrimPrefix(line, "rename from ")
			if strings.HasPrefix(path, `"`) {
				unquoted, err := strconv.Unquote(path)
				if err != nil {
					return nil, fmt.Errorf("cannot read the quoted path in %q: %w", line, err)
				}
				path = unquoted
			}
			sources = append(sources, path)
		}
	}
	return sources, nil
}

// testCodePaths returns the paths among paths that are test code, which a demonstration must not
// touch, since a red from a changed test says nothing about the mechanism. Test code is a _test.go
// file, anything under a testdata directory, a browser test file, which ends in .test.ts or .test.tsx,
// and anything under a directory named test, where a browser package keeps its tests, their preload
// and their fixture modules.
func testCodePaths(paths []string) []string {
	var found []string
	for _, path := range paths {
		segments := strings.Split(path, "/")
		browserTest := strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".test.tsx") || slices.Contains(segments, "test")
		if strings.HasSuffix(path, "_test.go") || slices.Contains(segments, "testdata") || browserTest {
			found = append(found, path)
		}
	}
	return found
}

// gitApply runs git apply in dir, a copy that is not a repository. The ceiling stops git from
// finding a repository above the copy and applying relative to it.
func gitApply(dir string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"apply"}, args...)...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "GIT_CEILING_DIRECTORIES="+filepath.Dir(dir))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git apply %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return out, nil
}
