package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// toolUnlinted is the tool name want annotations use for the check that code reached from ./... is
// code ./... lists.
const toolUnlinted = "unlinted"

// The check states ADR-0071's rule directly. golangci-lint, the go vet analysers and the grant check
// read only the packages ./... lists, and a file importing a package by path builds it in whether or
// not ./... lists it. So every package in the non-test build graph of ./..., the standard library
// apart, is either listed by ./... of this module or a dependency the module cache holds under a path
// outside this module's. Anything else, a package under testdata or in a directory ./... skips, one
// reached through a symbolic link, a nested module however the build reaches it, a module go.work
// adds, or a module replaced by a local directory, is reported at each import of it from a package
// ./... lists.
//
// The graph depends on the build configuration, so the check runs in every configuration code
// ships in, with no build tag, as the images and the release build. The images build with
// CGO_ENABLED=0 for linux, in each Dockerfile, and the release builds credential/cmd/keygen with
// CGO_ENABLED=0 for linux and darwin on amd64 and arm64, in .github/workflows/release.yaml. A
// configuration added there is added here by hand. When banproof proves the violation files it runs
// the check once more with their tag, in the first configuration, so their imports are refused too.
var unlintedConfigurations = []unlintedConfiguration{
	{"linux", "amd64"},
	{"linux", "arm64"},
	{"darwin", "amd64"},
	{"darwin", "arm64"},
}

// An unlintedConfiguration is one operating system and architecture code ships for.
type unlintedConfiguration struct{ goos, goarch string }

const unreachedMessage = "the build of ./... reaches %s through the module cache, and ./... does not list it, so no lint, analyser or grant check reads it (ADR-0071)"

const unlintedMessage = "imports %s, a package ./... does not list and the module cache does not hold for another module, so no lint, analyser or grant check reads it (ADR-0071)"

// A listedPackage is the part of go list's report on one package the check reads.
type listedPackage struct {
	ImportPath string
	Dir        string
	Standard   bool
	GoFiles    []string
	CgoFiles   []string
	Imports    []string
	Module     *struct {
		Path    string
		Replace *struct{ Version string }
	}
	Error *struct{ Err string }
}

// unlinted runs the check in every configuration with no build tag, and once more in the first
// configuration with tag when it is not empty, and returns the findings of every run.
func unlinted(root, tag string) ([]finding, error) {
	modulePath, modcache, err := moduleFacts(root)
	if err != nil {
		return nil, err
	}
	type run struct {
		config unlintedConfiguration
		tag    string
	}
	var runs []run
	for _, c := range unlintedConfigurations {
		runs = append(runs, run{c, ""})
	}
	if tag != "" {
		runs = append(runs, run{unlintedConfigurations[0], tag})
	}
	var found []finding
	for _, r := range runs {
		env := append(os.Environ(), "CGO_ENABLED=0", "GOOS="+r.config.goos, "GOARCH="+r.config.goarch)
		args := []string{"list", "-e"}
		if r.tag != "" {
			args = append(args, "-tags="+r.tag)
		}
		listed, err := goList(root, env, append(args, "./...")...)
		if err != nil {
			return nil, err
		}
		pkgs, err := goList(root, env, append(args, "-deps", "-json=ImportPath,Dir,Standard,GoFiles,CgoFiles,Imports,Module,Error", "./...")...)
		if err != nil {
			return nil, err
		}
		fs, err := unlintedFindings(filepath.Join(root, "go.mod"), modulePath, modcache, listed, pkgs)
		if err != nil {
			return nil, fmt.Errorf("%s/%s with tags %q: %w", r.config.goos, r.config.goarch, r.tag, err)
		}
		for _, f := range fs {
			if !slices.Contains(found, f) {
				found = append(found, f)
			}
		}
	}
	return found, nil
}

// moduleFacts returns the main module's path and the module cache's directory.
func moduleFacts(root string) (string, string, error) {
	out, err := goCommand(root, nil, "mod", "edit", "-json")
	if err != nil {
		return "", "", err
	}
	var mod struct{ Module struct{ Path string } }
	if err := json.Unmarshal(out, &mod); err != nil {
		return "", "", fmt.Errorf("reading go mod edit -json: %w", err)
	}
	if mod.Module.Path == "" {
		return "", "", errors.New("go.mod names no module")
	}
	cache, err := goCommand(root, nil, "env", "GOMODCACHE")
	if err != nil {
		return "", "", err
	}
	return mod.Module.Path, strings.TrimSpace(string(cache)), nil
}

// goList runs go list and decodes its report, one package per value.
func goList(root string, env []string, args ...string) ([]listedPackage, error) {
	out, err := goCommand(root, env, args...)
	if err != nil {
		return nil, err
	}
	if !slices.Contains(args, "-deps") {
		var pkgs []listedPackage
		for line := range strings.SplitSeq(strings.TrimSpace(string(out)), "\n") {
			if line != "" {
				pkgs = append(pkgs, listedPackage{ImportPath: line})
			}
		}
		return pkgs, nil
	}
	var pkgs []listedPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for dec.More() {
		var p listedPackage
		if err := dec.Decode(&p); err != nil {
			return nil, fmt.Errorf("reading go list output: %w", err)
		}
		pkgs = append(pkgs, p)
	}
	return pkgs, nil
}

func goCommand(root string, env []string, args ...string) ([]byte, error) {
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	cmd.Env = env
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("go %s failed: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	return stdout.Bytes(), nil
}

// unlintedFindings applies the rule to one configuration's report. listed is what ./... lists and
// pkgs the build graph of ./... with each package's files and imports. A package go list could not
// load fails the check, since its imports cannot be judged. A refused package that only packages
// from the module cache import, so no import of it sits in a file of this module, is reported at
// gomod, the main module's go.mod, since the build still reaches it.
func unlintedFindings(gomod, modulePath, modcache string, listed, pkgs []listedPackage) ([]finding, error) {
	lists := map[string]bool{}
	for _, p := range listed {
		lists[p.ImportPath] = true
	}
	refused := map[string]bool{}
	for _, p := range pkgs {
		if p.Error != nil {
			return nil, fmt.Errorf("go list could not load %s: %s", p.ImportPath, p.Error.Err)
		}
		if p.Standard || lists[p.ImportPath] {
			continue
		}
		ours := p.ImportPath == modulePath || strings.HasPrefix(p.ImportPath, modulePath+"/")
		replacedByDirectory := p.Module != nil && p.Module.Replace != nil && p.Module.Replace.Version == ""
		if !ours && !replacedByDirectory && strings.HasPrefix(p.Dir, modcache+string(filepath.Separator)) {
			continue
		}
		refused[p.ImportPath] = true
	}
	var found []finding
	covered := map[string]bool{}
	for _, p := range pkgs {
		if lists[p.ImportPath] || refused[p.ImportPath] {
			for _, i := range p.Imports {
				covered[i] = true
			}
		}
		if !lists[p.ImportPath] || !slices.ContainsFunc(p.Imports, func(i string) bool { return refused[i] }) {
			continue
		}
		for _, name := range slices.Concat(p.GoFiles, p.CgoFiles) {
			fs, err := importFindings(filepath.Join(p.Dir, name), refused)
			if err != nil {
				return nil, err
			}
			found = append(found, fs...)
		}
	}
	for _, p := range pkgs {
		if refused[p.ImportPath] && !covered[p.ImportPath] {
			found = append(found, finding{file: gomod, line: 1, tool: toolUnlinted, text: fmt.Sprintf(unreachedMessage, p.ImportPath)})
		}
	}
	return found, nil
}

// importFindings reports each import in the file of a package in refused.
func importFindings(path string, refused map[string]bool) ([]finding, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, nil, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	var out []finding
	for _, spec := range file.Imports {
		imported, err := strconv.Unquote(spec.Path.Value)
		if err != nil {
			return nil, err
		}
		if refused[imported] {
			out = append(out, finding{
				file: path,
				line: fset.PositionFor(spec.Path.Pos(), false).Line,
				tool: toolUnlinted,
				text: fmt.Sprintf(unlintedMessage, imported),
			})
		}
	}
	return out, nil
}
