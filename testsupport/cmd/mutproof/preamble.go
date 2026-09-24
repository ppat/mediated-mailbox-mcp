package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// preamble is what a patch states about itself in the free text before its first diff header.
type preamble struct {
	control  string
	removes  string
	packages []string
	tests    []string
	// scheduled is true when the demonstration goes red only at the scheduled case count.
	scheduled bool
	// integration is true when the tests run with the integration tag under pgrun.
	integration bool
}

// preambleKeys are the keys a preamble must carry, each exactly once. optionalKeys may be given once,
// and no other key may appear.
var (
	preambleKeys = []string{"control", "removes", "packages", "tests", "scheduled-count"}
	optionalKeys = []string{"integration"}
)

// parsePatch reads the preamble of the patch at path, which must be a .patch file directly in a
// testdata/mutations directory, the only place the pre-commit fixers leave byte for byte. It refuses a
// line that is not one of the keys, a key given twice or missing, a package that is not a relative
// package path, and a patch with no diff after the preamble.
func parsePatch(path string, src []byte) (preamble, error) {
	var p preamble
	if filepath.Ext(path) != ".patch" || filepath.Base(filepath.Dir(path)) != "mutations" || filepath.Base(filepath.Dir(filepath.Dir(path))) != "testdata" {
		return p, fmt.Errorf("%s: a patch is a .patch file directly in a testdata/mutations directory", path)
	}
	values := map[string]string{}
	hasDiff := false
	scanner := bufio.NewScanner(bytes.NewReader(src))
	scanner.Buffer(make([]byte, 0, 64*1024), len(src)+1)
	line := 0
	for scanner.Scan() {
		line++
		text := scanner.Text()
		if strings.HasPrefix(text, "diff --git ") {
			hasDiff = true
			break
		}
		if strings.TrimSpace(text) == "" {
			continue
		}
		key, value, ok := strings.Cut(text, ":")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !ok || !slices.Contains(slices.Concat(preambleKeys, optionalKeys), key) {
			return p, fmt.Errorf("%s:%d: the preamble holds only the keys %s, one per line", path, line, strings.Join(slices.Concat(preambleKeys, optionalKeys), ", "))
		}
		if _, dup := values[key]; dup {
			return p, fmt.Errorf("%s:%d: %s is given twice", path, line, key)
		}
		if value == "" {
			return p, fmt.Errorf("%s:%d: %s has no value", path, line, key)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return p, fmt.Errorf("%s: %w", path, err)
	}
	if !hasDiff {
		return p, fmt.Errorf("%s: no diff --git header follows the preamble", path)
	}
	var missing []string
	for _, key := range preambleKeys {
		if _, ok := values[key]; !ok {
			missing = append(missing, key)
		}
	}
	if len(missing) > 0 {
		return p, fmt.Errorf("%s: the preamble lacks %s", path, strings.Join(missing, ", "))
	}
	p.control = values["control"]
	p.removes = values["removes"]
	p.packages = strings.Fields(values["packages"])
	for _, pkg := range p.packages {
		// Anything else reaches go test as a flag, such as -run or -short, and changes what the runs prove.
		if pkg != "." && !strings.HasPrefix(pkg, "./") {
			return p, fmt.Errorf("%s: packages holds package paths relative to the repository root, such as ./core/redact or ./core/..., and %q is not one", path, pkg)
		}
	}
	p.tests = strings.Fields(values["tests"])
	switch values["scheduled-count"] {
	case "yes":
		p.scheduled = true
	case "no":
	default:
		return p, errors.New(path + ": scheduled-count is yes or no")
	}
	switch values["integration"] {
	case "yes":
		p.integration = true
	case "no", "":
	default:
		return p, errors.New(path + ": integration is yes or no")
	}
	return p, nil
}
