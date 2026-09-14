package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
)

// The browser layer's half of the proof. It uses the same want annotations and the same matching as the
// Go half, over the browser's TypeScript files, with oxlint and ast-grep as the reporting tools and a
// search for suppression directives.
//
// A browser violation file is named *_violation.ts or *_violation.tsx. The gating commands skip that name,
// oxlint and ast-grep by an ignore flag on their command lines and the type check by an exclude in
// tsconfig.json, while bun test never discovers such a file and bun build never reaches one nobody
// imports. This half runs both tools with their configuration unchanged and without the ignore flags.

// browserDir is the browser layer's directory, relative to the repository root.
var browserDir = filepath.Join("ui", "browser")

// Tool names used in browser want annotations.
const (
	toolOxlint  = "oxlint"
	toolAstGrep = "ast-grep"
)

var browserViolationName = regexp.MustCompile(`_violation\.tsx?$`)

// browserWant finds a want annotation in a line comment or a block comment. A block comment's
// annotation ends at the comment's end, which lets a directive follow it on the same line.
var browserWant = regexp.MustCompile(`(?://|/\*)\s*want\s(.*)$`)

// suppressionDirective matches every directive oxlint or ast-grep honours to silence a finding, and
// matches it anywhere on a line, strings included, so matching more than the tools honour fails closed.
var suppressionDirective = regexp.MustCompile(`(?i)(eslint|oxlint)-disable|ast-grep-ignore`)

// sanctionedIgnore is the one suppression the browser layer allows. ADR-0063 sanctions it for a contract
// field named value read in a rendering position, which the signal rule cannot tell from a signal. The
// directive sits on its own line, names only that rule, and follows a line comment naming the field, as
// ADR-0072 states. ast-grep then skips the rule on the next line.
const sanctionedIgnore = "// ast-grep-ignore: signal-value-in-render"

// fieldNamed matches the line comment naming the field, such as // Row.value is the contract field.
var fieldNamed = regexp.MustCompile(`^//\s.*\b[A-Za-z_$][\w$]*\.value\b`)

// violationImport matches an import or re-export of a violation module, which would put it in the bundle.
var violationImport = regexp.MustCompile(`(?:from|import)\s*\(?\s*["'][^"']*_violation(?:\.tsx?)?["']`)

func runBrowser(root string) error {
	dir := filepath.Join(root, browserDir)
	files, err := browserFiles(dir)
	if err != nil {
		return err
	}
	var problems []string
	var found []finding
	var expected []want
	for _, path := range files {
		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		isViolation := browserViolationName.MatchString(filepath.Base(path))
		ws, err := parseBrowserWants(path, src)
		if err != nil {
			return err
		}
		switch {
		case isViolation && len(ws) == 0:
			problems = append(problems, fmt.Sprintf("%s: violation file states no want", path))
		case !isViolation && len(ws) > 0:
			problems = append(problems, fmt.Sprintf("%s: want annotations outside a violation file are never checked", path))
		}
		if !isViolation && violationImport.Match(src) {
			problems = append(problems, fmt.Sprintf("%s: imports a violation file, which would bundle it", path))
		}
		expected = append(expected, ws...)
		found = append(found, suppressionFindings(path, src)...)
	}

	oxlint, err := runOxlint(dir)
	if err != nil {
		return err
	}
	astGrep, err := runAstGrep(dir)
	if err != nil {
		return err
	}
	for _, f := range slices.Concat(oxlint, astGrep) {
		if f.severity != "error" {
			problems = append(problems, fmt.Sprintf("reported as %s, which the gating run does not fail on: %s", f.severity, f.finding))
			continue
		}
		found = append(found, f.finding)
	}

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
	fmt.Printf("banproof: %d expected findings reported across %d browser files, nothing else reported\n", len(expected), len(files))
	return nil
}

// browserFiles lists the browser layer's TypeScript files, skipping installed packages, the bundle, and
// directories starting with a dot.
func browserFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		name := d.Name()
		if d.IsDir() {
			if path != dir && (name == "node_modules" || name == "dist" || strings.HasPrefix(name, ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.HasSuffix(name, ".ts") || strings.HasSuffix(name, ".tsx") {
			out = append(out, path)
		}
		return nil
	})
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%s not found. Run from the repository root", browserDir)
	}
	return out, err
}

// parseBrowserWants reads the want annotations of one TypeScript file. It reads lines rather than
// tokens, so an annotation-shaped string is taken for an annotation, which only a violation file may
// hold anyway.
func parseBrowserWants(path string, src []byte) ([]want, error) {
	var out []want
	for i, line := range strings.Split(string(src), "\n") {
		body, ok := browserWantBody(line)
		if !ok {
			continue
		}
		pairs, err := parseWantPairs(body)
		if err != nil {
			return nil, fmt.Errorf("%s:%d: %w", path, i+1, err)
		}
		for _, p := range pairs {
			out = append(out, want{file: path, line: i + 1, tool: p.tool, re: p.re})
		}
	}
	return out, nil
}

// browserWantBody returns the text of a line's annotation after the word want, which holds its tool and
// pattern pairs.
func browserWantBody(line string) (body string, ok bool) {
	m := browserWant.FindStringSubmatchIndex(line)
	if m == nil {
		return "", false
	}
	body = line[m[2]:m[3]]
	if strings.HasPrefix(line[m[0]:], "/*") {
		end := strings.Index(body, "*/")
		if end < 0 {
			return "", false
		}
		body = body[:end]
	}
	return body, true
}

// suppressionFindings reports every line holding a suppression directive, ignoring the text of a want
// annotation on that line, except the sanctioned read of a field named value. Every ban the browser
// tools carry stands in for a control, so no other suppression is allowed.
func suppressionFindings(path string, src []byte) []finding {
	var out []finding
	lines := strings.Split(string(src), "\n")
	for i, line := range lines {
		if sanctioned(path, lines, i) {
			continue
		}
		text := line
		if m := browserWant.FindStringIndex(line); m != nil {
			annotation := line[m[0]:]
			if strings.HasPrefix(annotation, "/*") {
				if end := strings.Index(annotation, "*/"); end >= 0 {
					annotation = annotation[:end+2]
				}
			}
			text = strings.Replace(line, annotation, "", 1)
		}
		if suppressionDirective.MatchString(text) {
			out = append(out, finding{file: path, line: i + 1, tool: toolSuppression, text: "suppression directive " + strings.TrimSpace(text)})
		}
	}
	return out
}

// sanctioned reports whether line i of a file is the suppression ADR-0063 sanctions, in the form ADR-0072
// states. The signal rule reads .tsx files only.
func sanctioned(path string, lines []string, i int) bool {
	if !strings.HasSuffix(path, ".tsx") || i == 0 || strings.TrimSpace(lines[i]) != sanctionedIgnore {
		return false
	}
	above := strings.TrimSpace(lines[i-1])
	return fieldNamed.MatchString(above) && !suppressionDirective.MatchString(above)
}

// A toolFinding is a finding with the severity its tool gave it.
type toolFinding struct {
	finding
	severity string
}

type oxlintReport struct {
	Diagnostics []struct {
		Message  string
		Code     string
		Severity string
		Filename string
		Labels   []struct {
			Span struct {
				Line int
			}
		}
	}
}

// runOxlint runs oxlint over the browser layer with the flags the gating run passes, except the one
// ignoring violation files.
func runOxlint(dir string) ([]toolFinding, error) {
	out, err := browserTool(dir, "oxlint", "--type-aware", "--format=json")
	if err != nil {
		return nil, err
	}
	return parseOxlint(dir, out)
}

func parseOxlint(dir string, out []byte) ([]toolFinding, error) {
	var r oxlintReport
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("reading oxlint output: %w\n%s", err, out)
	}
	var fs []toolFinding
	for _, d := range r.Diagnostics {
		line := 0
		if len(d.Labels) > 0 {
			line = d.Labels[0].Span.Line
		}
		fs = append(fs, toolFinding{
			finding:  finding{file: absolute(dir, d.Filename), line: line, tool: toolOxlint, text: d.Code + ": " + d.Message},
			severity: d.Severity,
		})
	}
	return fs, nil
}

type astGrepMatch struct {
	File     string
	RuleID   string `json:"ruleId"`
	Severity string
	Message  string
	Range    struct {
		Start struct {
			Line int
		}
	}
}

// runAstGrep runs ast-grep's scan over the browser layer with the project's rule files.
func runAstGrep(dir string) ([]toolFinding, error) {
	out, err := browserTool(dir, "ast-grep", "scan", "--json=compact")
	if err != nil {
		return nil, err
	}
	return parseAstGrep(dir, out)
}

func parseAstGrep(dir string, out []byte) ([]toolFinding, error) {
	var ms []astGrepMatch
	if err := json.Unmarshal(out, &ms); err != nil {
		return nil, fmt.Errorf("reading ast-grep output: %w\n%s", err, out)
	}
	var fs []toolFinding
	for _, m := range ms {
		fs = append(fs, toolFinding{
			// ast-grep counts lines from zero.
			finding:  finding{file: absolute(dir, m.File), line: m.Range.Start.Line + 1, tool: toolAstGrep, text: m.RuleID + ": " + m.Message},
			severity: m.Severity,
		})
	}
	return fs, nil
}

// browserTool runs an installed browser tool under bun, which provides the runtime its launcher script
// asks for. It returns standard output. Exit status 1 means the tool reported findings. Anything else, or
// anything on standard error beyond ast-grep's notice about its skipped install script, means it did not
// run cleanly.
func browserTool(dir, name string, args ...string) ([]byte, error) {
	cmd := exec.Command("bun", slices.Concat([]string{"--bun", filepath.Join("node_modules", ".bin", name)}, args)...)
	cmd.Dir = dir
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exit *exec.ExitError
	if err != nil && (!errors.As(err, &exit) || exit.ExitCode() != 1) {
		return nil, fmt.Errorf("%s failed: %w\n%s", name, err, stderr.String())
	}
	if unexpected := unexpectedStderr(stderr.String()); unexpected != "" {
		return nil, fmt.Errorf("%s printed errors, so it did not check cleanly:\n%s", name, unexpected)
	}
	return stdout.Bytes(), nil
}

// expectedStderr matches what ast-grep prints on standard error during a clean run. The first two lines
// are its notice on every run when bun did not run its install script, which bun skips by default and
// which changes nothing about what it reports. The last two summarize a scan that found errors.
var expectedStderr = regexp.MustCompile(`^(\[warn\] postinstall script did not run; falling back to runtime binary resolution\.` +
	`|Enable postinstall to avoid the per-invocation overhead\.` +
	`|Error: \d+ error\(s\) found in code\.` +
	`|Help: Scan succeeded and found error level diagnostics in the codebase\.)$`)

func unexpectedStderr(stderr string) string {
	var out []string
	for _, line := range strings.Split(stderr, "\n") {
		if line = strings.TrimSpace(line); line == "" || expectedStderr.MatchString(line) {
			continue
		}
		out = append(out, line)
	}
	return strings.Join(out, "\n")
}
