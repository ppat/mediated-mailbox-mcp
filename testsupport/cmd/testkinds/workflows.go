package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract"
)

var (
	goTest   = regexp.MustCompile(`\bgo test\b`)
	argsLine = regexp.MustCompile(`testkinds args ([a-z]+) ([a-z]+)([^)"]*)`)
	// held matches what would change whether a workflow's go test runs, what it selects, or whether
	// its failure fails the step from outside the go test line: a condition on a job or a step, a
	// step or job that carries on after a failure, and go's flags from the environment.
	held = regexp.MustCompile(`^(- )?(if|continue-on-error):|\bGOFLAGS\b`)
)

// liveWorkflow is the one workflow that runs a contract run against a real provider, and live
// matches any line naming its command, however it is started, go tool livecontract, go run of its
// directory or its binary, and any line naming the variable that marks its run, read from its one
// definition so a rename keeps the check.
const liveWorkflow = "gmail-contract.yaml"

var live = regexp.MustCompile(`(?i)livecontract|` + regexp.QuoteMeta(livecontract.Marker))

// selected is the spelling every workflow line that runs go test carries.
const selected = `"${args[@]}"`

// addable is the closed list of flags a go test line may add to the selected arguments, none of
// which changes which tests run or whether a failure fails the step, each with whether it takes a
// value.
var addable = map[string]bool{"-race": false, "-v": false, "-count": true, "-timeout": true}

// goTestProblem returns why a line that runs go test does something other than run exactly the
// selected arguments, or "". After go test it allows only the selection, once, the flags in
// addable, and a pipe into tee naming one file, so the step still fails when the tests fail under
// the shell's pipefail.
func goTestProblem(line string) string {
	tokens := strings.Fields(line[goTest.FindStringIndex(line)[1]:])
	seen := false
	for i := 0; i < len(tokens); i++ {
		tok := tokens[i]
		switch {
		case tok == selected:
			if seen {
				return "passes the selection twice"
			}
			seen = true
		case tok == "|" && i+3 == len(tokens) && tokens[i+1] == "tee":
			i = len(tokens)
		default:
			name, _, hasValue := strings.Cut(tok, "=")
			takesValue, ok := addable[name]
			switch {
			case !ok:
				return fmt.Sprintf("adds %s, which is not one of the flags a go test line may add, -race, -count, -v and -timeout, nor a pipe into tee", tok)
			case takesValue && !hasValue:
				i++
				if i == len(tokens) {
					return fmt.Sprintf("gives %s no value", tok)
				}
			case !takesValue && hasValue:
				return fmt.Sprintf("gives %s a value", name)
			}
		}
	}
	if !seen {
		return fmt.Sprintf("runs go test without %s from go tool testkinds args", selected)
	}
	return ""
}

// readWorkflows reads every workflow file in dir, keyed by its base name.
func readWorkflows(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	files := map[string]string{}
	for _, e := range entries {
		if e.IsDir() || (filepath.Ext(e.Name()) != ".yaml" && filepath.Ext(e.Name()) != ".yml") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, e.Name()))
		if err != nil {
			return nil, err
		}
		files[e.Name()] = string(b)
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("no workflow file in %s", dir)
	}
	return files, nil
}

// checkWorkflows requires every go test line in a workflow to run exactly the selected arguments,
// every args line to sit in a workflow its run is in and to name a run that has tests, and every run
// with a test to be asked for whole, with no package pattern, in each of its workflows. A workflow
// that runs go test may hold no condition, no continue-on-error and no GOFLAGS.
func checkWorkflows(t *tree, files map[string]string) []string {
	var problems []string
	asked := map[pair][]string{}
	for _, name := range sortedKeys(files) {
		lines := strings.Split(files[name], "\n")
		runsGoTest := false
		for _, line := range lines {
			if trimmed := strings.TrimSpace(line); !strings.HasPrefix(trimmed, "#") && goTest.MatchString(trimmed) {
				runsGoTest = true
			}
		}
		for i, line := range lines {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "#") {
				continue
			}
			where := fmt.Sprintf(".github/workflows/%s:%d", name, i+1)
			if goTest.MatchString(trimmed) {
				if why := goTestProblem(trimmed); why != "" {
					problems = append(problems, fmt.Sprintf("%s: %s, so it can run a test another workflow runs, or none", where, why))
				}
			}
			if name != liveWorkflow && live.MatchString(trimmed) {
				problems = append(problems, fmt.Sprintf("%s: names the livecontract command or its marker, and a contract run against a real provider runs only in %s", where, liveWorkflow))
			}
			if runsGoTest && held.MatchString(trimmed) {
				problems = append(problems, fmt.Sprintf("%s: a workflow that runs go test holds %q, which can change what its tests select or whether their failure fails it", where, trimmed))
			}
			for _, m := range argsLine.FindAllStringSubmatch(trimmed, -1) {
				p := pair{m[1], m[2]}
				want, ok := workflows[p]
				switch {
				case !ok:
					problems = append(problems, fmt.Sprintf("%s: %s %s is not a run, which are %s", where, p.kind, p.level, runList()))
				case !slices.Contains(want, name):
					problems = append(problems, fmt.Sprintf("%s: the %s %s run belongs to %s", where, p.kind, p.level, strings.Join(want, " and ")))
				case len(t.tests(p)) == 0:
					problems = append(problems, fmt.Sprintf("%s: no %s test at %s level exists, so the step would select nothing", where, p.kind, p.level))
				case !strings.Contains(m[3], "./"):
					asked[p] = append(asked[p], name)
				}
			}
		}
	}
	for _, p := range sortedPairs() {
		if len(t.tests(p)) == 0 {
			continue
		}
		for _, w := range workflows[p] {
			if !slices.Contains(asked[p], w) {
				problems = append(problems, fmt.Sprintf(".github/workflows/%s: never runs the %s %s tests whole, which it exists to run. An ask narrowed to some packages adds to the whole one and never stands for it", w, p.kind, p.level))
			}
		}
	}
	return problems
}
