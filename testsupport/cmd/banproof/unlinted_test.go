package main

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

// replacedBy returns a package's module, replaced by version, or by a directory when version is
// empty.
func replacedBy(version string) *struct {
	Path    string
	Replace *struct{ Version string }
} {
	return &struct {
		Path    string
		Replace *struct{ Version string }
	}{Replace: &struct{ Version string }{version}}
}

// The report below stands for every way a build of ./... reaches a package ./... does not list:
// a package under testdata, a symbolic link, a nested module, a module go.work adds, a module
// replaced by a local directory outside the repository, one replaced by a directory that sits under
// the module cache, and a nested module published under this module's path and fetched into the
// module cache. A module replaced by another version is admitted. Beside them sit the packages the rule admits, the
// standard library, packages ./... lists, and another module's package in the module cache, which
// also reaches an unlisted package of this module only packages from the cache import. An unlisted
// package importing another is reported only where a listed package reaches the first, and a line
// directive does not move a finding off the import's own line.
func TestUnlintedFindings(t *testing.T) {
	const (
		module = "example.com/m"
		cache  = "/cache/mod"
		repo   = "/repo"
	)
	dir := t.TempDir()
	write := func(name, src string) string {
		t.Helper()
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
		return name
	}
	leak := write("leak.go", `package leak

import (
	"fmt"

	_ "example.com/m/a/testdata/w"
	"example.com/m/b"
	dot "example.com/m/a/link"
	"example.com/other/x"
	_ "example.com/m/nested"
	_ "example.com/tool"
)
`)
	second := write("second.go", `package leak

//line elsewhere.go:40
import (
	_ "example.com/local"
	_ "example.com/m/pub"
	_ "example.com/evil"
	_ "example.com/fork"
)
`)
	clean := write("clean.go", `package leak

import _ "example.com/m/b"
`)
	inner := write("w.go", `package w

import _ "example.com/m/a/testdata/v"
`)
	listed := []listedPackage{{ImportPath: "example.com/m/leak"}, {ImportPath: "example.com/m/b"}}
	pkgs := []listedPackage{
		{ImportPath: "fmt", Standard: true},
		{ImportPath: "example.com/m/b", Dir: repo + "/b"},
		{ImportPath: "example.com/m/a/testdata/w", Dir: dir, GoFiles: []string{inner}, Imports: []string{"example.com/m/a/testdata/v"}},
		{ImportPath: "example.com/m/a/testdata/v", Dir: repo + "/a/testdata/v"},
		{ImportPath: "example.com/m/a/link", Dir: repo + "/a/link"},
		{ImportPath: "example.com/other/x", Dir: cache + "/example.com/other@v1.0.0/x", Imports: []string{"example.com/m/hidden"}},
		{ImportPath: "example.com/m/hidden", Dir: cache + "/example.com/m/hidden@v0.1.0"},
		{ImportPath: "example.com/m/nested", Dir: repo + "/nested"},
		{ImportPath: "example.com/tool", Dir: repo + "/tools/tool"},
		{ImportPath: "example.com/local", Dir: "/elsewhere/local"},
		{ImportPath: "example.com/m/pub", Dir: cache + "/example.com/m/pub@v0.1.0"},
		{ImportPath: "example.com/evil", Dir: cache + "/example.com/evil@v1.0.0", Module: replacedBy("")},
		{ImportPath: "example.com/fork", Dir: cache + "/example.com/fork@v1.0.1", Module: replacedBy("v1.0.1")},
		{
			ImportPath: "example.com/m/leak", Dir: dir, GoFiles: []string{leak, clean}, CgoFiles: []string{second},
			Imports: []string{"fmt", "example.com/m/a/testdata/w", "example.com/m/b", "example.com/m/a/link", "example.com/other/x", "example.com/m/nested", "example.com/tool", "example.com/local", "example.com/m/pub", "example.com/evil", "example.com/fork"},
		},
	}
	found, err := unlintedFindings("/repo/go.mod", module, cache, listed, pkgs)
	if err != nil {
		t.Fatal(err)
	}
	type row struct {
		File     string
		Line     int
		Tool     string
		Imported string
	}
	var got []row
	for _, f := range found {
		imported, _, _ := strings.Cut(strings.TrimPrefix(strings.TrimPrefix(f.text, "imports "), "the build of ./... reaches "), ",")
		imported, _, _ = strings.Cut(imported, " ")
		got = append(got, row{filepath.Base(f.file), f.line, f.tool, imported})
	}
	slices.SortFunc(got, func(a, b row) int { return strings.Compare(a.File+a.Imported, b.File+b.Imported) })
	want := []row{
		{"go.mod", 1, toolUnlinted, "example.com/m/hidden"},
		{"leak.go", 8, toolUnlinted, "example.com/m/a/link"},
		{"leak.go", 6, toolUnlinted, "example.com/m/a/testdata/w"},
		{"leak.go", 10, toolUnlinted, "example.com/m/nested"},
		{"leak.go", 11, toolUnlinted, "example.com/tool"},
		{"second.go", 7, toolUnlinted, "example.com/evil"},
		{"second.go", 5, toolUnlinted, "example.com/local"},
		{"second.go", 6, toolUnlinted, "example.com/m/pub"},
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("findings (-want +got):\n%s", diff)
	}
}

// A clean report has no finding, and a package go list could not load fails the check.
func TestUnlintedFindingsCleanAndBroken(t *testing.T) {
	listed := []listedPackage{{ImportPath: "example.com/m/a"}}
	clean := []listedPackage{
		{ImportPath: "os", Standard: true},
		{ImportPath: "example.com/m/a", Dir: "/repo/a", Imports: []string{"os", "example.com/x"}},
		{ImportPath: "example.com/x", Dir: "/cache/mod/example.com/x@v1.0.0"},
	}
	found, err := unlintedFindings("/repo/go.mod", "example.com/m", "/cache/mod", listed, clean)
	if err != nil || len(found) != 0 {
		t.Errorf("clean report: findings %v, error %v, want neither", found, err)
	}
	broken := append(slices.Clone(clean), listedPackage{ImportPath: "example.com/y", Error: &struct{ Err string }{"cannot find module"}})
	if _, err := unlintedFindings("/repo/go.mod", "example.com/m", "/cache/mod", listed, broken); err == nil {
		t.Error("a package go list could not load passed the check")
	}
}

// A fixture module whose package imports a testdata package from a file only one configuration
// builds, one per shipped operating system and architecture, one built only without cgo, one built
// only without the banproof tag and one built only with it, beside a module go.work adds and one
// replaced by a directory. Each import is refused, so dropping any configuration or run loses its
// finding.
func TestUnlintedRunsEveryShippedConfiguration(t *testing.T) {
	t.Setenv("CGO_ENABLED", "1")
	t.Setenv("GOFLAGS", "")
	t.Setenv("GOWORK", "")
	root := t.TempDir()
	write := func(name, src string) {
		t.Helper()
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o750); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(src), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	write("go.mod", "module example.com/fx\n\ngo 1.27\n\nrequire example.com/loc v0.0.0\n\nreplace example.com/loc => ./loc\n")
	write("go.work", "go 1.27\n\nuse (\n\t.\n\t./other\n)\n")
	write("other/go.mod", "module example.com/other\n\ngo 1.27\n")
	write("other/other.go", "package other\n")
	write("loc/go.mod", "module example.com/loc\n\ngo 1.27\n")
	write("loc/loc.go", "package loc\n")
	write("app/app.go", "package app\n\nimport (\n\t_ \"example.com/loc\"\n\t_ \"example.com/other\"\n)\n")
	files := map[string]string{
		"x_linux_amd64.go":  "linuxamd64",
		"x_linux_arm64.go":  "linuxarm64",
		"x_darwin_amd64.go": "darwinamd64",
		"x_darwin_arm64.go": "darwinarm64",
		"nocgo.go":          "nocgo",
		"notag.go":          "notag",
		"tagged.go":         "tagged",
	}
	constraints := map[string]string{"nocgo.go": "//go:build !cgo\n\n", "notag.go": "//go:build !banproof\n\n", "tagged.go": "//go:build banproof\n\n"}
	for file, pkg := range files {
		write("app/"+file, constraints[file]+"package app\n\nimport _ \"example.com/fx/testdata/"+pkg+"\"\n")
		write("testdata/"+pkg+"/x.go", "package "+pkg+"\n")
	}
	found, err := unlinted(root, "banproof")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range found {
		imported, _, _ := strings.Cut(strings.TrimPrefix(f.text, "imports "), ",")
		got = append(got, filepath.Base(f.file)+" "+imported)
	}
	slices.Sort(got)
	want := []string{
		"app.go example.com/loc",
		"app.go example.com/other",
		"nocgo.go example.com/fx/testdata/nocgo",
		"notag.go example.com/fx/testdata/notag",
		"tagged.go example.com/fx/testdata/tagged",
		"x_darwin_amd64.go example.com/fx/testdata/darwinamd64",
		"x_darwin_arm64.go example.com/fx/testdata/darwinarm64",
		"x_linux_amd64.go example.com/fx/testdata/linuxamd64",
		"x_linux_arm64.go example.com/fx/testdata/linuxarm64",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("findings (-want +got):\n%s", diff)
	}
}
