package main

import (
	"fmt"
	"slices"

	"go.yaml.in/yaml/v3"
)

// configName is the one golangci-lint configuration file (ADR-0071). golangci-lint would take a
// .golangci.yml, .golangci.toml or .golangci.json first if one existed, so banproof asks golangci-lint
// which file it reads and requires this one.
const configName = ".golangci.yaml"

// lintConfig holds the settings of .golangci.yaml that can switch off a finding of a linter standing in
// for a control. Key names are read exactly as written. golangci-lint reads keys in any letter case, but
// golangci-lint config verify, which banproof runs first, refuses a key its schema does not name.
type lintConfig struct {
	Run struct {
		Tests          *bool `yaml:"tests"`
		IssuesExitCode *int  `yaml:"issues-exit-code"`
	} `yaml:"run"`
	Issues struct {
		MaxIssuesPerLinter *int   `yaml:"max-issues-per-linter"`
		MaxSameIssues      *int   `yaml:"max-same-issues"`
		UniqByLine         *bool  `yaml:"uniq-by-line"`
		New                bool   `yaml:"new"`
		NewFromRev         string `yaml:"new-from-rev"`
		NewFromMergeBase   string `yaml:"new-from-merge-base"`
		NewFromPatch       string `yaml:"new-from-patch"`
	} `yaml:"issues"`
	Linters struct {
		Default    string   `yaml:"default"`
		Enable     []string `yaml:"enable"`
		Exclusions struct {
			Generated   string   `yaml:"generated"`
			Presets     []string `yaml:"presets"`
			Paths       []string `yaml:"paths"`
			PathsExcept []string `yaml:"paths-except"`
			Rules       []struct {
				Linters []string `yaml:"linters"`
			} `yaml:"rules"`
		} `yaml:"exclusions"`
		Settings struct {
			Errcheck struct {
				ExcludeFunctions []string `yaml:"exclude-functions"`
			} `yaml:"errcheck"`
			Exhaustive struct {
				DefaultSignifiesExhaustive bool   `yaml:"default-signifies-exhaustive"`
				ExplicitExhaustiveSwitch   bool   `yaml:"explicit-exhaustive-switch"`
				IgnoreEnumMembers          string `yaml:"ignore-enum-members"`
				IgnoreEnumTypes            string `yaml:"ignore-enum-types"`
				PackageScopeOnly           bool   `yaml:"package-scope-only"`
			} `yaml:"exhaustive"`
		} `yaml:"settings"`
	} `yaml:"linters"`
}

func parseConfig(src []byte) (lintConfig, error) {
	var c lintConfig
	if err := yaml.Unmarshal(src, &c); err != nil {
		return c, fmt.Errorf("reading %s: %w", configName, err)
	}
	return c, nil
}

// configProblems refuses every setting that can switch off a finding of a linter standing in for a
// control, where that finding fires or across a path or the whole tree. Exclusion rules may name only
// ordinary linters.
//
// The list is of golangci-lint's settings as they stand, so a release adding a setting that reaches every
// linter at once is caught only by review.
func configProblems(c lintConfig, ordinary []string) []string {
	var out []string
	refuse := func(format string, args ...any) {
		out = append(out, configName+": "+fmt.Sprintf(format, args...))
	}
	if c.Run.Tests != nil && !*c.Run.Tests {
		refuse("run.tests is false, which drops every finding in a test file")
	}
	if c.Run.IssuesExitCode != nil && *c.Run.IssuesExitCode != 1 {
		refuse("run.issues-exit-code is %d, so a run reporting findings does not fail with the status banproof and CI expect", *c.Run.IssuesExitCode)
	}
	if c.Issues.MaxIssuesPerLinter == nil || *c.Issues.MaxIssuesPerLinter != 0 {
		refuse("issues.max-issues-per-linter must be 0, or findings beyond the cap are not reported")
	}
	if c.Issues.MaxSameIssues == nil || *c.Issues.MaxSameIssues != 0 {
		refuse("issues.max-same-issues must be 0, or repeated findings beyond the cap are not reported")
	}
	if c.Issues.UniqByLine == nil || *c.Issues.UniqByLine {
		refuse("issues.uniq-by-line must be false, or a second finding on a line is not reported")
	}
	if c.Issues.New || c.Issues.NewFromRev != "" || c.Issues.NewFromMergeBase != "" || c.Issues.NewFromPatch != "" {
		refuse("issues.new and the issues.new-from settings report only findings in changed code")
	}
	if c.Linters.Default != "none" {
		refuse("linters.default must be none, so the enabled linters are exactly linters.enable and each has a known role")
	}
	ex := c.Linters.Exclusions
	if ex.Generated != "disable" {
		refuse("linters.exclusions.generated must be disable, or a file whose header says it is generated is not checked")
	}
	if len(ex.Presets) > 0 {
		refuse("linters.exclusions.presets %v. A preset excludes findings of linters it names inside golangci-lint, such as errcheck under std-error-handling. Write an exclusion rule naming ordinary linters instead", ex.Presets)
	}
	if len(ex.Paths) > 0 || len(ex.PathsExcept) > 0 {
		refuse("linters.exclusions.paths and paths-except drop every linter's findings for a path")
	}
	for i, r := range ex.Rules {
		if len(r.Linters) == 0 {
			refuse("linters.exclusions.rules[%d] names no linter, so it excludes every linter's findings it matches", i)
		}
		for _, name := range r.Linters {
			// golangci-lint compares these names exactly, so a name in another letter case matches nothing.
			// It is refused anyway, because it reads as naming the linter.
			if !slices.Contains(ordinary, name) {
				refuse("linters.exclusions.rules[%d] names %q, which is not on banproof's list of ordinary linters", i, name)
			}
		}
	}
	if len(c.Linters.Settings.Errcheck.ExcludeFunctions) > 0 {
		refuse("linters.settings.errcheck.exclude-functions stops errcheck reporting those calls")
	}
	x := c.Linters.Settings.Exhaustive
	if x.DefaultSignifiesExhaustive {
		refuse("linters.settings.exhaustive.default-signifies-exhaustive silences the check on every switch carrying the deny-defaulting default branch")
	}
	if x.ExplicitExhaustiveSwitch {
		refuse("linters.settings.exhaustive.explicit-exhaustive-switch checks only switches marked with a comment")
	}
	if x.IgnoreEnumMembers != "" || x.IgnoreEnumTypes != "" {
		refuse("linters.settings.exhaustive.ignore-enum-members and ignore-enum-types exclude enum members or types from the check")
	}
	if x.PackageScopeOnly {
		refuse("linters.settings.exhaustive.package-scope-only skips enums declared inside functions")
	}
	return out
}

// roleProblems cross-checks the list of ordinary linters against the enabled linters and against the
// linters violation files want. A linter standing in for a control has a violation file, so it cannot
// also be ordinary. An enabled linter that is neither ordinary nor wanted has no stated role.
func roleProblems(enabled, ordinary []string, wanted map[string]bool) []string {
	var out []string
	for _, name := range ordinary {
		if !slices.Contains(enabled, name) {
			out = append(out, fmt.Sprintf("%s is on banproof's list of ordinary linters but is not enabled in %s", name, configName))
		}
		if wanted[name] {
			out = append(out, fmt.Sprintf("%s is on banproof's list of ordinary linters but a violation file wants its findings, so it stands in for a control", name))
		}
	}
	for _, name := range enabled {
		if !slices.Contains(ordinary, name) && !wanted[name] {
			out = append(out, fmt.Sprintf("%s is enabled in %s, but it is not on banproof's list of ordinary linters and no violation file wants its findings", name, configName))
		}
	}
	return out
}

// wantedTools returns the tools named by want annotations.
func wantedTools(ws []want) map[string]bool {
	out := map[string]bool{}
	for _, w := range ws {
		out[w.tool] = true
	}
	return out
}
