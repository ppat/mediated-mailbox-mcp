package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"maps"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// browserLintScript is the gating lint command in ui/browser/package.json. Its only ignore pattern and glob
// skip violation files, which banproof lints instead. A flag added here, such as another ignore pattern or
// --allow for a rule, would switch a ban off in the gating run while banproof still proves it, so any change
// to the command must be made here as well, where a reviewer reads it beside this reason.
const browserLintScript = `oxlint --ignore-pattern '**/*_violation.*' && ast-grep scan --globs '!**/*_violation.*'`

// banPlugins are the oxlint plugins that own a ban rule. Leaving one out of plugins switches its rules off
// with no error, as measured for react/no-danger. eslint's rules need no plugin entry.
var banPlugins = []string{"react", "typescript"}

// oxlintConfigName matches every configuration file name oxlint reads. oxlint applies a nested
// configuration to the files below it, so only the one at the browser layer's root may exist.
var oxlintConfigName = regexp.MustCompile(`^(\.oxlintrc\.jsonc?|oxlint\.config\.[cm]?[jt]s)$`)

// browserConfigProblems refuses every configuration of the browser tools that can switch off a ban rule,
// and allows configuration touching only ordinary rules. Measured on oxlint: a category switched off does
// not reach a rule configured by name, and an override's plugins only add plugins, so neither is refused.
func browserConfigProblems(dir string) ([]string, error) {
	var problems []string
	refuse := func(file, format string, args ...any) {
		problems = append(problems, filepath.Join(browserDir, file)+": "+fmt.Sprintf(format, args...))
	}

	src, err := os.ReadFile(filepath.Join(dir, ".oxlintrc.json"))
	if err != nil {
		return nil, err
	}
	ps, err := oxlintConfigProblems(src, browserBanRules)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		refuse(".oxlintrc.json", "%s", p)
	}

	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() && d.Name() == "node_modules" {
			return filepath.SkipDir
		}
		if !d.IsDir() && oxlintConfigName.MatchString(d.Name()) && path != filepath.Join(dir, ".oxlintrc.json") {
			refuse(strings.TrimPrefix(path, dir+string(filepath.Separator)), "is an oxlint configuration below the browser layer's root, which oxlint applies to the files beside it")
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	var manifest struct {
		Scripts map[string]string `json:"scripts"`
	}
	src, err = os.ReadFile(filepath.Join(dir, "package.json"))
	if err != nil {
		return nil, err
	}
	if err := json.Unmarshal(src, &manifest); err != nil {
		return nil, fmt.Errorf("reading package.json: %w", err)
	}
	if got := manifest.Scripts["lint"]; got != browserLintScript {
		refuse("package.json", "the lint script is %q, not the command banproof knows, %q", got, browserLintScript)
	}

	src, err = os.ReadFile(filepath.Join(dir, "sgconfig.yml"))
	if err != nil {
		return nil, err
	}
	ruleDirs, ps, err := sgconfigProblems(src)
	if err != nil {
		return nil, err
	}
	for _, p := range ps {
		refuse("sgconfig.yml", "%s", p)
	}
	for _, ruleDir := range ruleDirs {
		files, err := filepath.Glob(filepath.Join(dir, ruleDir, "*.y*ml"))
		if err != nil {
			return nil, err
		}
		for _, file := range files {
			src, err := os.ReadFile(file)
			if err != nil {
				return nil, err
			}
			ps, err := astGrepRuleProblems(src)
			if err != nil {
				return nil, fmt.Errorf("%s: %w", file, err)
			}
			for _, p := range ps {
				refuse(strings.TrimPrefix(file, dir+string(filepath.Separator)), "%s", p)
			}
		}
	}
	return problems, nil
}

type oxlintOverride struct {
	Files        []string       `json:"files"`
	ExcludeFiles []string       `json:"excludeFiles"`
	Rules        map[string]any `json:"rules"`
}

type oxlintConfig struct {
	Extends        []string         `json:"extends"`
	IgnorePatterns []string         `json:"ignorePatterns"`
	Plugins        *[]string        `json:"plugins"`
	Rules          map[string]any   `json:"rules"`
	Overrides      []oxlintOverride `json:"overrides"`
}

// oxlintConfigProblems refuses the settings of .oxlintrc.json that reach a ban rule.
func oxlintConfigProblems(src []byte, bans []string) ([]string, error) {
	var c oxlintConfig
	if err := json.Unmarshal(src, &c); err != nil {
		return nil, fmt.Errorf("reading .oxlintrc.json: %w", err)
	}
	var problems []string
	if len(c.Extends) > 0 {
		problems = append(problems, "extends brings in configuration banproof does not read")
	}
	if len(c.IgnorePatterns) > 0 {
		problems = append(problems, fmt.Sprintf("ignorePatterns %v drops every rule's findings, the bans' included, for those files", c.IgnorePatterns))
	}
	if c.Plugins == nil {
		problems = append(problems, fmt.Sprintf("plugins is not set, and oxlint's default plugins leave out %v, which own ban rules", banPlugins))
	} else {
		for _, p := range banPlugins {
			if !slices.Contains(*c.Plugins, p) {
				problems = append(problems, fmt.Sprintf("plugins leaves out %s, which owns a ban rule, so that rule reports nothing", p))
			}
		}
	}
	problems = append(problems, banSeverityProblems("rules", c.Rules, bans)...)
	for i, o := range c.Overrides {
		where := fmt.Sprintf("overrides[%d].rules", i)
		problems = append(problems, banSeverityProblems(where, o.Rules, bans)...)
		if len(o.ExcludeFiles) > 0 && namesBan(o.Rules, bans) {
			problems = append(problems, fmt.Sprintf("overrides[%d].excludeFiles narrows where a ban rule applies", i))
		}
	}
	return problems, nil
}

// banSeverityProblems refuses a ban rule, under any plugin prefix, set to anything but an error.
func banSeverityProblems(where string, rules map[string]any, bans []string) []string {
	var problems []string
	for _, name := range slices.Sorted(maps.Keys(rules)) {
		if !slices.Contains(bans, name[strings.LastIndex(name, "/")+1:]) {
			continue
		}
		if !isErrorSeverity(rules[name]) {
			problems = append(problems, fmt.Sprintf("%s sets the ban rule %s to %v, and a ban reports as an error", where, name, rules[name]))
		}
	}
	return problems
}

func namesBan(rules map[string]any, bans []string) bool {
	for name := range rules {
		if slices.Contains(bans, name[strings.LastIndex(name, "/")+1:]) {
			return true
		}
	}
	return false
}

// isErrorSeverity reports whether an oxlint rule value is an error, written as a string, a number or the
// first element of an array carrying options.
func isErrorSeverity(v any) bool {
	if a, ok := v.([]any); ok && len(a) > 0 {
		v = a[0]
	}
	switch s := v.(type) {
	case string:
		return s == "error" || s == "deny"
	case float64:
		return s == 2
	}
	return false
}

// sgconfigProblems refuses ast-grep project settings that change which files or languages its rules read.
func sgconfigProblems(src []byte) ([]string, []string, error) {
	var c map[string]any
	if err := yaml.Unmarshal(src, &c); err != nil {
		return nil, nil, fmt.Errorf("reading sgconfig.yml: %w", err)
	}
	var problems []string
	for _, key := range slices.Sorted(maps.Keys(c)) {
		switch key {
		case "ruleDirs", "utilDirs", "testConfigs":
		default:
			problems = append(problems, fmt.Sprintf("%s can change which files or languages the ban rules read", key))
		}
	}
	var dirs []string
	if raw, ok := c["ruleDirs"].([]any); ok {
		for _, d := range raw {
			if s, ok := d.(string); ok {
				dirs = append(dirs, s)
			}
		}
	}
	if !slices.Equal(dirs, []string{"rules"}) {
		problems = append(problems, fmt.Sprintf("ruleDirs is %v, not [rules]", dirs))
		dirs = []string{"rules"}
	}
	return dirs, problems, nil
}

// astGrepRuleProblems refuses a rule file that narrows or quietens a ban. Every ast-grep rule is a ban.
func astGrepRuleProblems(src []byte) ([]string, error) {
	var problems []string
	dec := yaml.NewDecoder(bytes.NewReader(src))
	for {
		var r map[string]any
		err := dec.Decode(&r)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, err
		}
		if r == nil {
			continue
		}
		id := fmt.Sprint(r["id"])
		if r["severity"] != "error" {
			problems = append(problems, fmt.Sprintf("rule %s has severity %v, and a ban reports as an error", id, r["severity"]))
		}
		for _, key := range []string{"files", "ignores"} {
			if _, ok := r[key]; ok {
				problems = append(problems, fmt.Sprintf("rule %s sets %s, which narrows the files a ban reads", id, key))
			}
		}
	}
	return problems, nil
}
