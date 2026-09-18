package main

import (
	"bytes"
	"crypto/sha256"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

type savedDir struct {
	path string
	mode fs.FileMode
}

// snapshot holds the bytes and mode of every file a patch touches, taken before the patch is
// applied, so the files can be put back exactly whatever happens during the run.
type snapshot struct {
	root  string
	files []savedFile
}

type savedFile struct {
	path   string
	exists bool
	data   []byte
	mode   fs.FileMode
	// parents holds the mode of each directory above an existing file, up to root, so a directory
	// the patch removed along with its last file can be made again.
	parents []savedDir
	// createdDirs are the directories above an absent file that did not exist either, deepest first.
	createdDirs []string
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
// file or anything under a testdata directory.
func testCodePaths(paths []string) []string {
	var found []string
	for _, path := range paths {
		if strings.HasSuffix(path, "_test.go") || slices.Contains(strings.Split(path, "/"), "testdata") {
			found = append(found, path)
		}
	}
	return found
}

func takeSnapshot(root string, paths []string) (*snapshot, error) {
	s := &snapshot{root: root}
	for _, rel := range paths {
		path := filepath.Join(root, filepath.FromSlash(rel))
		//nolint:gosec // A path the patch names under root, which git apply --check accepted.
		info, err := os.Lstat(path)
		switch {
		case errors.Is(err, fs.ErrNotExist):
			saved := savedFile{path: path}
			for dir := filepath.Dir(path); dir != root && len(dir) > len(root); dir = filepath.Dir(dir) {
				//nolint:gosec // A path the patch names under root, which git apply --check accepted.
				if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
					break
				}
				saved.createdDirs = append(saved.createdDirs, dir)
			}
			s.files = append(s.files, saved)
		case err != nil:
			return nil, err
		case !info.Mode().IsRegular():
			return nil, fmt.Errorf("%s is not a regular file", rel)
		default:
			//nolint:gosec // A path the patch names under root, which git apply --check accepted.
			data, err := os.ReadFile(path)
			if err != nil {
				return nil, err
			}
			saved := savedFile{path: path, exists: true, data: data, mode: info.Mode().Perm()}
			for dir := filepath.Dir(path); dir != root && len(dir) > len(root); dir = filepath.Dir(dir) {
				//nolint:gosec // A directory above a path the patch names under root, which git apply --check accepted.
				dirInfo, err := os.Stat(dir)
				if err != nil {
					return nil, err
				}
				saved.parents = append(saved.parents, savedDir{dir, dirInfo.Mode().Perm()})
			}
			s.files = append(s.files, saved)
		}
	}
	return s, nil
}

// restore puts every file back as it was when the snapshot was taken and then checks that it is.
// It keeps going past a failure so that as much as possible is restored, and names every file it
// could not restore.
func (s *snapshot) restore() error {
	var errs []error
	for _, f := range s.files {
		if f.exists {
			// A patch that deletes or renames the last file in a directory removes the directory too.
			for i := len(f.parents) - 1; i >= 0; i-- {
				if err := os.Mkdir(f.parents[i].path, f.parents[i].mode); err != nil && !errors.Is(err, fs.ErrExist) {
					errs = append(errs, err)
				}
			}
			if err := os.WriteFile(f.path, f.data, f.mode); err != nil {
				errs = append(errs, err)
				continue
			}
			if err := os.Chmod(f.path, f.mode); err != nil {
				errs = append(errs, err)
			}
			continue
		}
		if err := os.Remove(f.path); err != nil && !errors.Is(err, fs.ErrNotExist) {
			errs = append(errs, err)
		}
		for _, dir := range f.createdDirs {
			if err := os.Remove(dir); err != nil && !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, err)
			}
		}
	}
	if err := s.verify(); err != nil {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("restoring the patched files failed, restore them by hand: %w", errors.Join(errs...))
	}
	return nil
}

// verify checks that every file matches the snapshot byte for byte, with the same mode, and that
// every file and directory the patch created is gone.
func (s *snapshot) verify() error {
	var errs []error
	for _, f := range s.files {
		info, err := os.Lstat(f.path)
		if !f.exists {
			if !errors.Is(err, fs.ErrNotExist) {
				errs = append(errs, fmt.Errorf("%s did not exist before the patch and still does", f.path))
			}
			for _, dir := range f.createdDirs {
				if _, err := os.Lstat(dir); !errors.Is(err, fs.ErrNotExist) {
					errs = append(errs, fmt.Errorf("directory %s did not exist before the patch and still does", dir))
				}
			}
			continue
		}
		if err != nil {
			errs = append(errs, err)
			continue
		}
		data, err := os.ReadFile(f.path)
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if !bytes.Equal(data, f.data) || info.Mode().Perm() != f.mode {
			errs = append(errs, fmt.Errorf("%s differs from its content before the patch", f.path))
		}
	}
	return errors.Join(errs...)
}

func gitApply(root string, args ...string) ([]byte, error) {
	cmd := exec.Command("git", append([]string{"apply"}, args...)...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git apply %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return out, nil
}

// treeState maps every path git status reports under root, changed, staged or untracked, to its
// status and a hash of its content and mode. Two states that differ name a path the run changed,
// wherever it is in the working tree. Files git ignores are not in it.
func treeState(root string) (map[string]string, error) {
	cmd := exec.Command("git", "status", "--porcelain=v1", "-z", "--untracked-files=all")
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git status: %w\n%s", err, stderr.String())
	}
	state := map[string]string{}
	fields := strings.Split(string(out), "\x00")
	for i := 0; i < len(fields); i++ {
		entry := fields[i]
		if len(entry) < 4 {
			continue
		}
		status, paths := entry[:2], []string{entry[3:]}
		// A staged rename or copy is followed by its source path in a field of its own.
		if (status[0] == 'R' || status[0] == 'C') && i+1 < len(fields) {
			i++
			paths = append(paths, fields[i])
		}
		for _, rel := range paths {
			content, err := fileState(filepath.Join(root, filepath.FromSlash(rel)))
			if err != nil {
				return nil, err
			}
			state[rel] = status + " " + content
		}
	}
	return state, nil
}

func fileState(path string) (string, error) {
	info, err := os.Lstat(path)
	switch {
	case errors.Is(err, fs.ErrNotExist):
		return "absent", nil
	case err != nil:
		return "", err
	case info.Mode()&fs.ModeSymlink != 0:
		target, err := os.Readlink(path)
		return "symlink " + target, err
	case !info.Mode().IsRegular():
		return info.Mode().String(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s %x", info.Mode().Perm(), sha256.Sum256(data)), nil
}

// changedPaths returns, sorted, every path whose state differs between before and after.
func changedPaths(before, after map[string]string) []string {
	var changed []string
	for path, state := range before {
		if after[path] != state {
			changed = append(changed, path)
		}
	}
	for path := range after {
		if _, ok := before[path]; !ok {
			changed = append(changed, path)
		}
	}
	slices.Sort(changed)
	return changed
}
