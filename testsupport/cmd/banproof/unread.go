package main

import (
	"go/parser"
	"go/token"
	"maps"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

const unreadMessage = "a build of ./... in a configuration code ships in compiles this file, and its build constraints keep it out of the gating lint and vet runs, so no lint or analyser reads it (ADR-0071)"

// The check that the build of ./... reaches only packages ./... lists judges packages, and a
// package ./... lists can still hold a file no lint reads. golangci-lint and the go vet analysers
// read the files go list selects with the build tags .golangci.yaml sets, for the operating system
// and architecture of the machine running them, with cgo as that machine has it. A file whose build
// constraint excludes one of those tags, such as //go:build !integration, one built only for
// another operating system or architecture, or one built only without cgo, is compiled into an
// image or a release binary and read by nothing. So every non-test Go file a configuration that
// ships compiles, in a package ./... lists, must be a file go list selects with .golangci.yaml's
// tags in the environment banproof runs in, which in continuous integration is the lint job's own.
// Any other is refused at its package clause. The go vet step in .github/workflows/go-lint.yaml
// sets the same tags, which vettags.go checks. Assembly and other files that are not Go files are
// outside the rule.

// listedFiles returns the non-test files one configuration compiles in the packages ./... lists.
// Every configuration sets CGO_ENABLED=0, so it compiles no cgo file, and a file it compiles is never
// a cgo file for the lint either.
func listedFiles(listed, pkgs []listedPackage) []string {
	lists := map[string]bool{}
	for _, p := range listed {
		lists[p.ImportPath] = true
	}
	var out []string
	for _, p := range pkgs {
		if !lists[p.ImportPath] {
			continue
		}
		for _, name := range p.GoFiles {
			out = append(out, filepath.Join(p.Dir, name))
		}
	}
	return out
}

// unread compares the files each run compiled, keyed by the run's tag, with the files the gating lint
// reads. The untagged runs are compared with lintTags, and a tagged run with lintTags and its tag, as
// banproof's own lint run reads them. The lint's listing inherits the environment unchanged, so its
// operating system, architecture and cgo setting are the machine's.
func unread(root string, lintTags []string, shipped map[string][]string) ([]finding, error) {
	var found []finding
	for _, tag := range slices.Sorted(maps.Keys(shipped)) {
		tags := slices.Clone(lintTags)
		if tag != "" {
			tags = append(tags, tag)
		}
		args := []string{"list", "-e", "-json=Dir,GoFiles"}
		if len(tags) > 0 {
			args = append(args, "-tags="+strings.Join(tags, ","))
		}
		pkgs, err := goList(root, os.Environ(), append(args, "./...")...)
		if err != nil {
			return nil, err
		}
		read := map[string]bool{}
		for _, p := range pkgs {
			for _, name := range p.GoFiles {
				read[filepath.Join(p.Dir, name)] = true
			}
		}
		fs, err := unreadFindings(shipped[tag], read)
		if err != nil {
			return nil, err
		}
		for _, f := range fs {
			if !slices.Contains(found, f) {
				found = append(found, f)
			}
		}
	}
	return found, nil
}

// unreadFindings refuses each file in shipped that read does not hold, at its package clause.
func unreadFindings(shipped []string, read map[string]bool) ([]finding, error) {
	var found []finding
	for _, path := range shipped {
		if read[path] {
			continue
		}
		fset := token.NewFileSet()
		file, err := parser.ParseFile(fset, path, nil, parser.PackageClauseOnly)
		if err != nil {
			return nil, err
		}
		f := finding{file: path, line: fset.PositionFor(file.Package, false).Line, tool: toolUnlinted, text: unreadMessage}
		if !slices.Contains(found, f) {
			found = append(found, f)
		}
	}
	return found, nil
}
