package main

import (
	"fmt"
	"slices"
	"strings"

	"go.yaml.in/yaml/v3"
)

// lintWorkflow is the workflow whose go vet step is the gating vet run.
const lintWorkflow = ".github/workflows/go-lint.yaml"

// vetTagProblems refuses a go vet step in the lint workflow whose build tags differ, as a set, from the
// build tags .golangci.yaml sets. The check that every file a build that ships compiles is read
// compares with the configuration's tags, so the gating vet run reads what it compares with only while
// the two lists hold the same tags (ADR-0071). Exactly one step may run go vet with -vettool.
func vetTagProblems(workflow []byte, buildTags []string) ([]string, error) {
	var wf struct {
		Jobs map[string]struct {
			Steps []struct {
				Run string `yaml:"run"`
			} `yaml:"steps"`
		} `yaml:"jobs"`
	}
	if err := yaml.Unmarshal(workflow, &wf); err != nil {
		return nil, fmt.Errorf("reading %s: %w", lintWorkflow, err)
	}
	var runs []string
	for _, job := range wf.Jobs {
		for _, step := range job.Steps {
			if strings.Contains(step.Run, "go vet") && strings.Contains(step.Run, "-vettool") {
				runs = append(runs, step.Run)
			}
		}
	}
	if len(runs) != 1 {
		return []string{fmt.Sprintf("%s: %d steps run go vet with -vettool, and banproof reads the gating vet run's tags from exactly one", lintWorkflow, len(runs))}, nil
	}
	vetTags, found := tagsFlag(runs[0])
	want := slices.Sorted(slices.Values(buildTags))
	got := slices.Sorted(slices.Values(vetTags))
	if !found || !slices.Equal(slices.Compact(got), slices.Compact(want)) {
		return []string{fmt.Sprintf("%s: the go vet step sets the build tags %q, and %s sets %q, so a file one run reads the other may not (ADR-0071)", lintWorkflow, strings.Join(vetTags, ","), configName, strings.Join(buildTags, ","))}, nil
	}
	return nil, nil
}

// tagsFlag returns the tags a -tags flag in a command line names, written as -tags value or
// -tags=value, and whether the line has one. The go command keeps the last -tags it is given, so
// the last one is read.
func tagsFlag(line string) ([]string, bool) {
	fields := strings.Fields(line)
	var tags []string
	found := false
	for i, f := range fields {
		var value string
		switch {
		case f == "-tags" || f == "--tags":
			if i+1 >= len(fields) {
				continue
			}
			value = fields[i+1]
		case strings.HasPrefix(f, "-tags=") || strings.HasPrefix(f, "--tags="):
			_, value, _ = strings.Cut(f, "=")
		default:
			continue
		}
		found = true
		tags = nil
		if value = strings.Trim(value, `"'`); value != "" {
			tags = strings.Split(value, ",")
		}
	}
	return tags, found
}
