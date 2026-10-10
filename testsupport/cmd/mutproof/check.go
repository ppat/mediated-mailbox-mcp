package main

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/cmd/mutproof/internal/hunkmatch"
)

// fixtureDir holds mutproof's own fixtures, relative to the repository root. Their patches target
// the fixture module its tests build, not the repository, so checkTree leaves them out. The
// runner's own mutation patches, in its testdata/mutations, are checked like any other.
const fixtureDir = "testsupport/cmd/mutproof/testdata/fixture"

// checkTree checks every mutation patch under root without running a demonstration, which is what
// the mutation-patches workflow runs on every pull request. A mutation patch is a .patch file under
// a testdata/mutations directory, at any depth, outside fixtureDir. Each must apply to the tree as
// it stands, by git apply --check, and each of its hunks must match at one place only
// (internal/hunkmatch), the same two checks demonstrate runs before a demonstration. It writes one
// line per patch it refuses, as a GitHub Actions error annotation followed by the reason, and
// returns an error when any patch is refused. A walk that cannot read a directory, or that finds no patch at all, is an
// error too, since the check would otherwise pass with nothing checked.
func checkTree(w io.Writer, root string) error {
	root, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	var patches []string
	err = filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		if d.IsDir() && rel == fixtureDir {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.HasSuffix(rel, ".patch") && strings.Contains("/"+rel, "/testdata/mutations/") {
			patches = append(patches, rel)
		}
		return nil
	})
	if err != nil {
		return fmt.Errorf("listing the mutation patches: %w", err)
	}
	if len(patches) == 0 {
		return errors.New("found no mutation patch under a testdata/mutations directory, so the check would pass with nothing checked")
	}
	if _, err := fmt.Fprintf(w, "checking %d mutation patches\n", len(patches)); err != nil {
		return err
	}
	refused := 0
	for _, rel := range patches {
		reason := checkPatch(root, rel)
		if reason == nil {
			continue
		}
		refused++
		first, rest, _ := strings.Cut(reason.Error(), "\n")
		if _, err := fmt.Fprintf(w, "::error file=%s::%s\n", rel, first); err != nil {
			return err
		}
		if rest != "" {
			if _, err := fmt.Fprintln(w, rest); err != nil {
				return err
			}
		}
	}
	if refused > 0 {
		return fmt.Errorf("%d of %d mutation patches refused", refused, len(patches))
	}
	return nil
}

// checkPatch runs git apply --check and then hunkmatch.Check on the patch at rel, relative to root.
func checkPatch(root, rel string) error {
	path := filepath.Join(root, filepath.FromSlash(rel))
	src, err := os.ReadFile(path) //nolint:gosec // A patch the walk found under root.
	if err != nil {
		return err
	}
	if _, err := gitApply(root, "--check", path); err != nil {
		return fmt.Errorf("it no longer applies to the tree: %w", err)
	}
	return hunkmatch.Check(root, src)
}
