package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
)

// baseConfig is the smallest configuration configProblems accepts.
const baseConfig = `
version: "2"
issues:
  max-issues-per-linter: 0
  max-same-issues: 0
  uniq-by-line: false
linters:
  default: none
  enable: [errcheck, exhaustive, gosec]
  exclusions:
    generated: disable
    presets: []
    rules:
    - path: _violation\.go$
      linters: [gosec]
  settings:
    exhaustive:
      default-signifies-exhaustive: false
`

// TestConfigProblems changes one setting of baseConfig per case. Each change can switch off a finding of a
// linter standing in for a control, so each must be refused, with the problem naming the setting.
func TestConfigProblems(t *testing.T) {
	ordinary := []string{"gosec"}
	if problems := configProblems(mustParse(t, baseConfig), ordinary); len(problems) > 0 {
		t.Fatalf("the base configuration is refused:\n%s", strings.Join(problems, "\n"))
	}
	cases := []struct {
		name, old, new, problem string
	}{
		{"rule naming a control linter", "linters: [gosec]", "linters: [gosec, errcheck]", `rules\[0\] names "errcheck"`},
		{"rule naming a control linter in another letter case", "linters: [gosec]", "linters: [Errcheck]", `rules\[0\] names "Errcheck"`},
		{"rule naming an ordinary linter in another letter case", "linters: [gosec]", "linters: [Gosec]", `rules\[0\] names "Gosec"`},
		{"rule with no linters key", "      linters: [gosec]\n", "      text: \".\"\n", `rules\[0\] names no linter`},
		{"rule with an empty linters list", "linters: [gosec]", "linters: []", `rules\[0\] names no linter`},
		{"exclusion paths", "    presets: []\n", "    presets: []\n    paths: [core/]\n", `exclusions.paths`},
		{"exclusion paths-except", "    presets: []\n", "    presets: []\n    paths-except: [core/]\n", `paths-except`},
		{"preset", "presets: []", "presets: [std-error-handling]", `presets \[std-error-handling\]`},
		{"generated files skipped leniently", "generated: disable", "generated: lax", `generated must be disable`},
		{"generated files skipped strictly", "generated: disable", "generated: strict", `generated must be disable`},
		{"generated files at the default", "    generated: disable\n", "", `generated must be disable`},
		{"test files dropped", "issues:\n", "run:\n  tests: false\nissues:\n", `run.tests is false`},
		{"exit status changed", "issues:\n", "run:\n  issues-exit-code: 0\nissues:\n", `issues-exit-code is 0`},
		{"findings capped per linter", "max-issues-per-linter: 0", "max-issues-per-linter: 50", `max-issues-per-linter must be 0`},
		{"findings capped per linter at the default", "  max-issues-per-linter: 0\n", "", `max-issues-per-linter must be 0`},
		{"same findings capped", "max-same-issues: 0", "max-same-issues: 3", `max-same-issues must be 0`},
		{"one finding per line", "uniq-by-line: false", "uniq-by-line: true", `uniq-by-line must be false`},
		{"only new findings", "  uniq-by-line: false\n", "  uniq-by-line: false\n  new: true\n", `issues.new`},
		{"only findings since a revision", "  uniq-by-line: false\n", "  uniq-by-line: false\n  new-from-rev: HEAD\n", `issues.new`},
		{"only findings since the merge base", "  uniq-by-line: false\n", "  uniq-by-line: false\n  new-from-merge-base: main\n", `issues.new`},
		{"only findings in a patch", "  uniq-by-line: false\n", "  uniq-by-line: false\n  new-from-patch: p.diff\n", `issues.new`},
		{"default linter set", "default: none", "default: standard", `default must be none`},
		{"errcheck exclusions", "  settings:\n", "  settings:\n    errcheck:\n      exclude-functions: [os.Remove]\n", `exclude-functions`},
		{"default branch proves exhaustive", "default-signifies-exhaustive: false", "default-signifies-exhaustive: true", `default-signifies-exhaustive`},
		{"only marked switches", "default-signifies-exhaustive: false", "explicit-exhaustive-switch: true", `explicit-exhaustive-switch`},
		{"ignored enum members", "default-signifies-exhaustive: false", "ignore-enum-members: Deny", `ignore-enum-members`},
		{"ignored enum types", "default-signifies-exhaustive: false", "ignore-enum-types: verdict", `ignore-enum-types`},
		{"package-scoped enums only", "default-signifies-exhaustive: false", "package-scope-only: true", `package-scope-only`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if !strings.Contains(baseConfig, c.old) {
				t.Fatalf("the base configuration does not contain %q", c.old)
			}
			problems := configProblems(mustParse(t, strings.Replace(baseConfig, c.old, c.new, 1)), ordinary)
			if len(problems) != 1 || !matches(t, c.problem, problems[0]) {
				t.Fatalf("want exactly one problem matching %q, got:\n%s", c.problem, strings.Join(problems, "\n"))
			}
		})
	}
}

func TestRoleProblems(t *testing.T) {
	enabled := []string{"depguard", "forbidigo", "gosec", "revive"}
	wanted := map[string]bool{"depguard": true, "forbidigo": true, "suppression": true}
	if problems := roleProblems(enabled, []string{"gosec", "revive"}, wanted); len(problems) > 0 {
		t.Fatalf("consistent roles are refused:\n%s", strings.Join(problems, "\n"))
	}
	got := roleProblems(enabled, []string{"forbidigo", "gosec", "misspell"}, wanted)
	want := []string{
		"forbidigo is on banproof's list of ordinary linters but a violation file wants its findings, so it stands in for a control",
		"misspell is on banproof's list of ordinary linters but is not enabled in .golangci.yaml",
		"revive is enabled in .golangci.yaml, but it is not on banproof's list of ordinary linters and no violation file wants its findings",
	}
	if diff := cmp.Diff(want, got); diff != "" {
		t.Fatalf("problems (-want +got):\n%s", diff)
	}
}

func mustParse(t *testing.T, src string) lintConfig {
	t.Helper()
	c, err := parseConfig([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func matches(t *testing.T, pattern, s string) bool {
	t.Helper()
	ok, err := regexp.MatchString(pattern, s)
	if err != nil {
		t.Fatal(err)
	}
	return ok
}
