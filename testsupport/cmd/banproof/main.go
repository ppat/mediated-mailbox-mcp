// Command banproof proves that every lint ban and import list standing in for a control still
// fires.
//
// Each checked-in violation file breaks a rule on purpose and states, in want annotations on the
// offending lines, which tool must report which finding there. banproof runs golangci-lint with the
// repository's one configuration and the search for suppression directives. It fails unless
// every want is reported and nothing else is. A rule that has been weakened until it matches
// nothing therefore turns the run red, because its want goes unreported.
//
// No linter standing in for a control may be switched off, anywhere (ADR-0071). The search for
// suppression directives refuses every comment that can silence such a linter's finding. It allows a
// golangci-lint directive only when every linter the directive names is on the closed list of ordinary
// linters and a reason follows the names (suppression.go). A check of .golangci.yaml refuses every
// setting that can reach such a linter, allows exclusion rules naming only ordinary linters, and
// cross-checks the list against the enabled linters and the violation files (config.go).
//
// Violation files carry a build constraint that is false unless the banproof tag is set, so the
// gating lint, build and test runs never see them. banproof enables the tag. It also fails when the
// tag and the file name disagree, so the tag cannot hide an ordinary test. A violation file's name
// ends in _violation.go for a rule over non-test files, or _violation followed by the file kind's own
// suffix for a rule over test files, such as _violation_test.go or _violation_property_test.go.
//
// It also runs go vet with the placement analyser under testsupport/analysis, whose findings want
// annotations name as placement, the analyser's name.
//
// With -browser it proves the browser layer's bans instead, the same way, with oxlint, ast-grep and a
// search for their suppression directives (browser.go). The browser half needs bun and the browser
// layer's installed packages, which the Go half does not.
//
// Run it from the repository root.
//
//	go tool banproof
//	go tool banproof -browser
package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"go/build/constraint"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
)

// violationName matches the names violation files must carry.
var violationName = regexp.MustCompile(`_violation(_[a-z]+)?(_test)?\.go$`)

func main() {
	tag := flag.String("tag", "banproof", "build tag that enables violation files. Empty runs one untagged run whose findings are all classified")
	browser := flag.Bool("browser", false, "prove the browser layer's bans under ui/browser instead of the Go bans")
	flag.Parse()
	var err error
	if *browser {
		var root string
		if root, err = os.Getwd(); err == nil {
			err = runBrowser(root)
		}
	} else {
		err = run(*tag)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "banproof:", err)
		os.Exit(1)
	}
}

func run(tag string) error {
	root, err := os.Getwd()
	if err != nil {
		return err
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return errors.New("run from the repository root, where go.mod is")
	}

	var problems []string
	if out, err := command(root, "golangci-lint", "config", "verify"); err != nil {
		return fmt.Errorf("golangci-lint config verify failed: %w\n%s", err, out)
	}
	out, err := command(root, "golangci-lint", "config", "path")
	if err != nil {
		return fmt.Errorf("golangci-lint config path failed: %w\n%s", err, out)
	}
	if used := strings.TrimSpace(string(out)); used != configName {
		problems = append(problems, fmt.Sprintf("golangci-lint reads %s rather than %s", used, configName))
	}
	src, err := os.ReadFile(filepath.Join(root, configName))
	if err != nil {
		return err
	}
	config, err := parseConfig(src)
	if err != nil {
		return err
	}
	problems = append(problems, configProblems(config, ordinaryLinters)...)

	files, err := goFiles(root)
	if err != nil {
		return err
	}
	var found []finding
	var expected []want
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		found = append(found, directiveFindings(path, src, ordinaryLinters)...)
		isViolation := violationName.MatchString(filepath.Base(path))
		if tag != "" {
			tagged, err := needsTag(src, tag)
			if err != nil {
				return fmt.Errorf("%s: %w", path, err)
			}
			switch {
			case isViolation && !tagged:
				problems = append(problems, fmt.Sprintf("%s: violation file is not excluded unless the %s tag is set", path, tag))
			case !isViolation && tagged:
				problems = append(problems, fmt.Sprintf("%s: only violation files may depend on the %s tag", path, tag))
			}
		}
		ws, err := parseWants(path, src)
		if err != nil {
			return err
		}
		switch {
		case isViolation && len(ws) == 0:
			problems = append(problems, fmt.Sprintf("%s: violation file states no want", path))
		case !isViolation && len(ws) > 0:
			problems = append(problems, fmt.Sprintf("%s: want annotations outside a violation file are never checked", path))
		}
		expected = append(expected, ws...)
	}

	problems = append(problems, roleProblems(config.Linters.Enable, ordinaryLinters, wantedTools(expected))...)

	lintFindings, err := lint(root, tag)
	if err != nil {
		return err
	}
	found = append(found, lintFindings...)
	vetFindings, err := vet(root, tag)
	if err != nil {
		return err
	}
	found = append(found, vetFindings...)

	unmet, unexpected := match(expected, found)
	for _, w := range unmet {
		problems = append(problems, "not reported: "+w.String())
	}
	for _, f := range unexpected {
		problems = append(problems, "reported without a want: "+f.String())
	}
	if len(problems) > 0 {
		slices.Sort(problems)
		return fmt.Errorf("%d problems\n%s", len(problems), strings.Join(problems, "\n"))
	}
	fmt.Printf("banproof: %d expected findings reported across %d Go files, nothing else reported\n", len(expected), len(files))
	return nil
}

// goFiles lists the repository's Go files. It skips what the go command skips, which is testdata,
// directories starting with a dot or an underscore, and the ignored browser dependencies.
func goFiles(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != root && (name == "testdata" || name == "node_modules" || strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".go") {
			out = append(out, path)
		}
		return nil
	})
	return out, err
}

// needsTag reports whether the file's build constraint excludes it whenever tag is not set, whatever
// other tags are set.
func needsTag(src []byte, tag string) (bool, error) {
	for _, line := range strings.Split(string(src), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "package ") {
			return false, nil
		}
		if !constraint.IsGoBuild(line) {
			continue
		}
		expr, err := constraint.Parse(line)
		if err != nil {
			return false, err
		}
		return !expr.Eval(func(t string) bool { return t != tag }), nil
	}
	return false, nil
}

// lint runs golangci-lint over the whole module with the repository's configuration. The flags
// passed here only add the tag, and golangci-lint adds it to the configuration's own build tags.
func lint(root, tag string) ([]finding, error) {
	args := []string{"run", "--output.json.path=stdout", "--show-stats=false"}
	if tag != "" {
		args = append(args, "--build-tags="+tag)
	}
	args = append(args, "./...")
	cmd := exec.Command("golangci-lint", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	// golangci-lint exits 1 when it reports findings. Any other failure means it did not lint.
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return nil, fmt.Errorf("golangci-lint run failed: %w\n%s", err, stderr.String())
	}
	return parseLint(root, stdout.Bytes())
}

// vetFinding matches one diagnostic line go vet prints.
var vetFinding = regexp.MustCompile(`^(.+\.go):(\d+):\d+: (.*)$`)

// vet runs go vet with the placement analyser over the module and returns its findings, each under
// the analyser's name.
func vet(root, tag string) ([]finding, error) {
	tool, err := command(root, "go", "tool", "-n", "vetcheck")
	if err != nil {
		return nil, fmt.Errorf("go tool -n vetcheck failed: %w\n%s", err, tool)
	}
	// integration is set because golangci-lint's configuration sets it, so crash sequences and other
	// integration-tagged files are checked by both.
	tags := "integration"
	if tag != "" {
		tags += "," + tag
	}
	args := []string{"vet", "-vettool=" + strings.TrimSpace(string(tool)), "-tags=" + tags}
	args = append(args, "./...")
	cmd := exec.Command("go", args...)
	cmd.Dir = root
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err = cmd.Run()
	var exit *exec.ExitError
	// go vet exits 1 when it reports findings. Any other failure means it did not vet.
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return nil, fmt.Errorf("go vet failed: %w\n%s", err, stderr.String())
	}
	var out []finding
	for _, line := range strings.Split(stderr.String(), "\n") {
		if line == "" || strings.HasPrefix(line, "# ") {
			continue
		}
		m := vetFinding.FindStringSubmatch(line)
		if m == nil {
			return nil, fmt.Errorf("go vet printed a line banproof cannot read: %s\n%s", line, stderr.String())
		}
		n, err := strconv.Atoi(m[2])
		if err != nil {
			return nil, err
		}
		out = append(out, finding{file: absolute(root, m[1]), line: n, tool: "placement", text: m[3]})
	}
	return out, nil
}

func command(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir
	return cmd.CombinedOutput()
}
