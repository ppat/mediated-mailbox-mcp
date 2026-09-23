package property

import (
	"io/fs"
	"path/filepath"
	"strings"
)

// rapidFailFiles returns every file under a directory named testdata/rapid below root. rapid replays
// every such file before generating, whether or not writing is switched off, and a file that no
// longer reproduces its failure passes silently, so none may be kept (ADR-0069).
func rapidFailFiles(root string) ([]string, error) {
	var found []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if !d.IsDir() && strings.Contains(filepath.ToSlash(path), "/testdata/rapid/") {
			found = append(found, path)
		}
		return nil
	})
	return found, err
}
