package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// The kinds and levels a test file has, as the package comment defines them.
const (
	kindUnit        = "unit"
	kindProperty    = "property"
	kindCrash       = "crash"
	kindIntegration = "integration"
	kindFixtures    = "fixtures"

	levelUnit        = "unit"
	levelIntegration = "integration"
)

type pair struct{ kind, level string }

// workflows names the workflows each run is in. Every run is in the one workflow that gates a pull
// request on it, and the property tests and crash sequences are also in the workflow of their deep
// search, which gates nothing (ADR-0055, ADR-0045). A pair missing here is refused.
var workflows = map[pair][]string{
	{kindUnit, levelUnit}:               {"go-unit.yaml"},
	{kindProperty, levelUnit}:           {"go-property.yaml", "go-property-deep.yaml"},
	{kindProperty, levelIntegration}:    {"go-property.yaml", "go-property-deep.yaml"},
	{kindIntegration, levelIntegration}: {"go-integration.yaml"},
	{kindCrash, levelUnit}:              {"go-crash.yaml", "go-crash-deep.yaml"},
	{kindCrash, levelIntegration}:       {"go-crash.yaml", "go-crash-deep.yaml"},
	{kindFixtures, levelIntegration}:    {"ui.yaml"},
}

func runList() string {
	var s []string
	for p := range workflows {
		s = append(s, p.kind+" "+p.level)
	}
	slices.Sort(s)
	return strings.Join(s, ", ")
}

// pkg is one package directory and the test files go test compiles in it at each level.
type pkg struct {
	dir string // relative to the root, slash-separated, "." for the root
	// compiled holds, per level, the test file names compiled at it.
	compiled map[string][]string
	// all holds every test file name in the directory, compiled at a level or not.
	all []string
}

// file is one test file and the tests it declares.
type file struct {
	pkg   string
	name  string
	kind  string // empty for a violation file or a live test its build tags keep from both levels
	level string // empty when compiled at neither level
	tests []string
}

// tree is what the checks and the selection read: every package's test files, and each file's kind,
// level and declared tests.
type tree struct {
	pkgs  map[string]*pkg
	files []file
}

// listedPackage is the part of go list's output the tree is built from.
type listedPackage struct {
	Dir            string
	TestGoFiles    []string
	XTestGoFiles   []string
	IgnoredGoFiles []string
}

// load builds the tree for the module at root from go list at each level and the files' sources.
func load(root string) (*tree, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	t := &tree{pkgs: map[string]*pkg{}}
	for _, level := range []string{levelUnit, levelIntegration} {
		listed, err := goList(abs, level)
		if err != nil {
			return nil, err
		}
		for _, lp := range listed {
			rel, err := filepath.Rel(abs, lp.Dir)
			if err != nil {
				return nil, err
			}
			rel = filepath.ToSlash(rel)
			p := t.pkgs[rel]
			if p == nil {
				p = &pkg{dir: rel, compiled: map[string][]string{}}
				t.pkgs[rel] = p
			}
			p.compiled[level] = append(append([]string{}, lp.TestGoFiles...), lp.XTestGoFiles...)
			for _, names := range [][]string{lp.TestGoFiles, lp.XTestGoFiles, lp.IgnoredGoFiles} {
				for _, n := range names {
					if strings.HasSuffix(n, "_test.go") && !slices.Contains(p.all, n) {
						p.all = append(p.all, n)
					}
				}
			}
		}
	}
	for _, dir := range sortedKeys(t.pkgs) {
		p := t.pkgs[dir]
		slices.Sort(p.all)
		for _, name := range p.all {
			f := file{pkg: dir, name: name, kind: kindOf(name)}
			switch {
			case slices.Contains(p.compiled[levelUnit], name):
				f.level = levelUnit
			case slices.Contains(p.compiled[levelIntegration], name):
				f.level = levelIntegration
			}
			if heldOut(name) && f.level == "" {
				f.kind = ""
			}
			f.tests, err = declaredTests(filepath.Join(abs, filepath.FromSlash(dir), name))
			if err != nil {
				return nil, err
			}
			t.files = append(t.files, f)
		}
	}
	return t, nil
}

// goList lists every package ./... reaches at a level. -e keeps a package whose files all sit behind
// a build constraint, and the ignore directive in go.mod is honoured as for every other go command.
func goList(root, level string) ([]listedPackage, error) {
	args := []string{"list", "-e", "-json=Dir,TestGoFiles,XTestGoFiles,IgnoredGoFiles"}
	if level == levelIntegration {
		args = append(args, "-tags=integration")
	}
	args = append(args, "./...")
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	var listed []listedPackage
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var lp listedPackage
		if err := dec.Decode(&lp); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		listed = append(listed, lp)
	}
	return listed, nil
}

// kindOf reads a test file's kind from its name. A file named for a kind ends in an underscore and
// the kind's suffix, so a library's own tests, such as crash_test.go in testsupport/crash, are unit
// tests.
func kindOf(name string) string {
	switch {
	case strings.HasSuffix(name, "_fixtures_integration_test.go"):
		return kindFixtures
	case strings.HasSuffix(name, "_integration_test.go"):
		return kindIntegration
	case strings.HasSuffix(name, "_property_test.go"):
		return kindProperty
	case strings.HasSuffix(name, "_crash_test.go"):
		return kindCrash
	}
	return kindUnit
}

// heldOut reports whether a file's name marks it as a violation file, which builds only for the
// ban-proof run, or a contract run against a real provider, which runs only through its own command.
// Such a file has no kind only while its build tags keep it from both levels, so a file named so
// that compiles at a level is run like any other.
func heldOut(name string) bool {
	return strings.Contains(name, "_violation") || strings.HasSuffix(name, "_live_test.go")
}

// declaredTests returns the names of the top-level tests, examples and fuzz targets a file declares,
// the functions go test's -run selects. TestMain is not one, since it runs whenever its package's
// tests do.
func declaredTests(path string) ([]string, error) {
	f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, d := range f.Decls {
		fn, ok := d.(*ast.FuncDecl)
		if !ok || fn.Recv != nil {
			continue
		}
		if n := fn.Name.Name; n != "TestMain" && (isTest(n, "Test") || isTest(n, "Example") || isTest(n, "Fuzz")) {
			names = append(names, n)
		}
	}
	return names, nil
}

// isTest reports whether name is the prefix followed by nothing or by a character that is not a
// lower-case letter, as go test reads it.
func isTest(name, prefix string) bool {
	if !strings.HasPrefix(name, prefix) {
		return false
	}
	if len(name) == len(prefix) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(name[len(prefix):])
	return !unicode.IsLower(r)
}

func (f file) pair() pair { return pair{f.kind, f.level} }

// tests returns every test of the pair, as package directory and name, sorted.
func (t *tree) tests(p pair) []string {
	var s []string
	for _, f := range t.files {
		if f.pair() == p {
			for _, n := range f.tests {
				s = append(s, f.pkg+" "+n)
			}
		}
	}
	slices.Sort(s)
	return slices.Compact(s)
}

// selection is what go test runs for a pair: the packages, the names the pattern holds, and what
// that selects, as package directory and name.
type selection struct {
	pkgs, names, selected []string
}

func (t *tree) selection(p pair) selection {
	var sel selection
	for _, f := range t.files {
		if f.pair() == p {
			sel.pkgs = append(sel.pkgs, f.pkg)
			sel.names = append(sel.names, f.tests...)
		}
	}
	slices.Sort(sel.pkgs)
	sel.pkgs = slices.Compact(sel.pkgs)
	slices.Sort(sel.names)
	sel.names = slices.Compact(sel.names)
	for _, f := range t.files {
		if !slices.Contains(sel.pkgs, f.pkg) || !slices.Contains(t.pkgs[f.pkg].compiled[p.level], f.name) {
			continue
		}
		for _, n := range f.tests {
			if slices.Contains(sel.names, n) {
				sel.selected = append(sel.selected, f.pkg+" "+n)
			}
		}
	}
	slices.Sort(sel.selected)
	sel.selected = slices.Compact(sel.selected)
	return sel
}

// args returns go test's arguments for a pair, narrowed to the package patterns given.
func (t *tree) args(p pair, patterns []string) ([]string, error) {
	for _, f := range t.files {
		if strings.HasSuffix(f.name, "_live_test.go") && f.level != "" {
			return nil, fmt.Errorf("%s/%s, a contract run against a real provider, compiles at the %s level in this environment, so no selection is printed", f.pkg, f.name, f.level)
		}
	}
	sel := t.selection(p)
	if len(sel.names) == 0 {
		return nil, fmt.Errorf("no %s test at %s level exists, so the step would select nothing", p.kind, p.level)
	}
	pkgs := sel.pkgs
	if len(patterns) > 0 {
		var narrowed []string
		for _, d := range pkgs {
			for _, pat := range patterns {
				m, err := matchPattern(pat, d)
				if err != nil {
					return nil, err
				}
				if m {
					narrowed = append(narrowed, d)
					break
				}
			}
		}
		if len(narrowed) == 0 {
			return nil, fmt.Errorf("the patterns %v match no package holding a %s %s test", patterns, p.kind, p.level)
		}
		pkgs = narrowed
	}
	var out []string
	if p.level == levelIntegration {
		out = append(out, "-tags=integration")
	}
	out = append(out, "-run=^("+strings.Join(sel.names, "|")+")$")
	for _, d := range pkgs {
		out = append(out, "./"+strings.TrimPrefix(d, "./"))
	}
	return out, nil
}

// matchPattern reports whether a package directory matches ./dir or ./dir/..., the only patterns
// accepted.
func matchPattern(pattern, dir string) (bool, error) {
	if !strings.HasPrefix(pattern, "./") {
		return false, fmt.Errorf("package pattern %q is neither ./dir nor ./dir/... in form", pattern)
	}
	p := strings.TrimPrefix(pattern, "./")
	if rest, ok := strings.CutSuffix(p, "/..."); ok {
		return dir == rest || strings.HasPrefix(dir, rest+"/"), nil
	}
	if strings.Contains(p, "...") {
		return false, fmt.Errorf("package pattern %q is neither ./dir nor ./dir/... in form", pattern)
	}
	return dir == p, nil
}

// check returns every test file without a valid run, and every run whose arguments would select a
// test of another run.
func (t *tree) check() []string {
	var problems []string
	for _, f := range t.files {
		path := f.pkg + "/" + f.name
		switch {
		case f.kind == "":
			continue
		case strings.HasSuffix(f.name, "_live_test.go"):
			problems = append(problems, fmt.Sprintf("%s: a contract run against a real provider compiled at the %s level, so a test workflow would run it. It runs only in gmail-contract through go tool livecontract, so it carries its provider's build tag", path, f.level))
		case f.level == "":
			problems = append(problems, fmt.Sprintf("%s: compiled neither with no build tag nor with the integration tag, so no workflow runs it", path))
		default:
			if _, ok := workflows[f.pair()]; !ok {
				problems = append(problems, fmt.Sprintf("%s: a %s test at %s level, which no workflow runs. %s", path, f.kind, f.level, fix(f)))
			}
		}
	}
	for _, p := range sortedPairs() {
		own := t.tests(p)
		for _, s := range t.selection(p).selected {
			if !slices.Contains(own, s) {
				dir, name, _ := strings.Cut(s, " ")
				problems = append(problems, fmt.Sprintf("%s: %s, in a file of another kind or level, is also selected by the run of %s tests at %s level, which holds a test of the same name. Rename one", dir, name, p.kind, p.level))
			}
		}
	}
	return problems
}

func fix(f file) string {
	if f.level == levelIntegration {
		return "A test needing the integration tag is named for its kind, _integration_test.go when it is an example test"
	}
	return "A test file compiled with no build tag is named _test.go, _property_test.go or _crash_test.go, so an _integration_test.go or _fixtures_integration_test.go file carries the integration tag"
}

func sortedPairs() []pair {
	ps := make([]pair, 0, len(workflows))
	for p := range workflows {
		ps = append(ps, p)
	}
	slices.SortFunc(ps, func(a, b pair) int { return strings.Compare(a.kind+" "+a.level, b.kind+" "+b.level) })
	return ps
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
