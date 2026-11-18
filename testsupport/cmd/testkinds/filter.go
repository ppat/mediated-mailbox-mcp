package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// filtered names the runs whose workflow's pull-request path list the check holds to every file the
// run's tests link, so a change to code those tests link starts the workflow. It holds the
// recorded-fixtures run's workflow, and another run whose workflow narrows its list joins by an
// entry here.
var filtered = map[pair]string{
	{kindFixtures, levelIntegration}: "ui.yaml",
}

// linkedPackage is the part of go list's output the filter check reads.
type linkedPackage struct {
	Dir        string
	Standard   bool
	Module     *struct{ Main bool }
	GoFiles    []string
	CgoFiles   []string
	EmbedFiles []string
}

// linkedFiles returns every file of this module, relative to root and slash-separated, that the
// tests of pkgs compile or embed at level, as go list -deps -test reads it: the files of every
// package they link, the test files of pkgs among them.
func linkedFiles(root, level string, pkgs []string) ([]string, error) {
	root, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	args := []string{"list", "-deps", "-test", "-json=Dir,Standard,Module,GoFiles,CgoFiles,EmbedFiles"}
	if level == levelIntegration {
		args = append(args, "-tags=integration")
	}
	for _, p := range pkgs {
		args = append(args, "./"+p)
	}
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("go %s: %w\n%s", strings.Join(args, " "), err, stderr.String())
	}
	var files []string
	dec := json.NewDecoder(bytes.NewReader(out))
	for {
		var lp linkedPackage
		if err := dec.Decode(&lp); errors.Is(err, io.EOF) {
			break
		} else if err != nil {
			return nil, err
		}
		if lp.Standard || lp.Dir == "" || lp.Module == nil || !lp.Module.Main {
			continue
		}
		rel, err := filepath.Rel(root, lp.Dir)
		if err != nil {
			return nil, err
		}
		rel = filepath.ToSlash(rel)
		// A test's own variant of a package, and its external test package, list the test files
		// among their Go files and embedded files.
		names := slices.Concat(lp.GoFiles, lp.CgoFiles, lp.EmbedFiles)
		for _, n := range names {
			if filepath.IsAbs(n) {
				// A file go generates into its build cache for a test, which no change in the
				// repository edits.
				continue
			}
			files = append(files, pathJoin(rel, n))
		}
	}
	slices.Sort(files)
	return slices.Compact(files), nil
}

func pathJoin(dir, name string) string {
	if dir == "." {
		return name
	}
	return dir + "/" + name
}

// pullRequestPaths reads the path list of a workflow's pull_request trigger from its text, the list
// items under the paths key that follows the pull_request key. It reads the one form the workflows
// write, a block list of plain or quoted patterns, and parses no YAML, so a list written another way
// reads as empty and every linked file is reported missing.
func pullRequestPaths(text string) []string {
	var out []string
	inTrigger, inPaths := false, false
	for _, line := range strings.Split(text, "\n") {
		trimmed := strings.TrimSpace(line)
		switch {
		case trimmed == "pull_request:":
			inTrigger = true
		case inTrigger && trimmed == "paths:":
			inPaths = true
		case inPaths && strings.HasPrefix(trimmed, "- "):
			out = append(out, strings.Trim(strings.TrimSpace(strings.TrimPrefix(trimmed, "- ")), `'"`))
		case inPaths:
			return out
		}
	}
	return out
}

// globPattern turns a workflow's path pattern into a regular expression. It reads the subset of
// GitHub's filter syntax the workflows use, a * matching any characters but a slash and a ** any
// characters, every other character standing for itself. It refuses a pattern holding ?, +, [ or ],
// whose meanings in GitHub's syntax it does not read, so the check never judges a list by another
// reading than GitHub's.
func globPattern(pattern string) (*regexp.Regexp, error) {
	if strings.ContainsAny(pattern, "?+[]") {
		return nil, fmt.Errorf("the path pattern %q holds ?, +, [ or ], which the check does not read", pattern)
	}
	var b strings.Builder
	b.WriteString("^")
	for i := 0; i < len(pattern); i++ {
		switch c := pattern[i]; {
		case c == '*' && i+1 < len(pattern) && pattern[i+1] == '*':
			b.WriteString(".*")
			i++
		case c == '*':
			b.WriteString("[^/]*")
		default:
			b.WriteString(regexp.QuoteMeta(string(c)))
		}
	}
	b.WriteString("$")
	return regexp.Compile(b.String())
}

// unwatched returns each of files the patterns leave unwatched, as GitHub reads a path list, a
// pattern starting with ! excluding what it matches and the last pattern that matches a file
// deciding it.
func unwatched(patterns, files []string) ([]string, error) {
	type rule struct {
		re      *regexp.Regexp
		exclude bool
	}
	rules := make([]rule, 0, len(patterns))
	for _, p := range patterns {
		exclude := strings.HasPrefix(p, "!")
		re, err := globPattern(strings.TrimPrefix(p, "!"))
		if err != nil {
			return nil, err
		}
		rules = append(rules, rule{re: re, exclude: exclude})
	}
	var out []string
	for _, f := range files {
		watched := false
		for _, r := range rules {
			if r.re.MatchString(f) {
				watched = !r.exclude
			}
		}
		if !watched {
			out = append(out, f)
		}
	}
	return out, nil
}

// checkFilters requires the path list of each filtered run's workflow to watch every file its tests
// link, reported once per directory, so a change that can change what those tests see starts the
// workflow that runs them.
func checkFilters(root string, t *tree, files map[string]string) ([]string, error) {
	var problems []string
	for _, p := range sortedPairs() {
		workflow, ok := filtered[p]
		if !ok {
			continue
		}
		pkgs := t.selection(p).pkgs
		if len(pkgs) == 0 {
			continue
		}
		linked, err := linkedFiles(root, p.level, pkgs)
		if err != nil {
			return nil, err
		}
		missing, err := unwatched(pullRequestPaths(files[workflow]), linked)
		if err != nil {
			return nil, fmt.Errorf(".github/workflows/%s: %w", workflow, err)
		}
		var dirs []string
		for _, f := range missing {
			dirs = append(dirs, filepath.ToSlash(filepath.Dir(f)))
		}
		slices.Sort(dirs)
		for _, d := range slices.Compact(dirs) {
			problems = append(problems, fmt.Sprintf(".github/workflows/%s: its pull_request paths leave %s unwatched, which the %s %s tests link, so a change there would not run them", workflow, d, p.kind, p.level))
		}
	}
	return problems, nil
}
