package main

import (
	"encoding/json"
	"fmt"
	"path/filepath"
)

// toolNolint is the tool name want annotations use for the suppression directive search. Findings
// from golangci-lint carry the reporting linter's name.
const toolNolint = "nolint"

// A finding is one report from any tool, with its file as an absolute path.
type finding struct {
	file string
	line int
	tool string
	text string
}

func (f finding) String() string {
	return fmt.Sprintf("%s:%d: %s: %s", f.file, f.line, f.tool, f.text)
}

type lintReport struct {
	Issues []struct {
		FromLinter string
		Text       string
		Pos        struct {
			Filename string
			Line     int
		}
	}
}

// parseLint reads golangci-lint's JSON output. Its file names are relative to base.
func parseLint(base string, out []byte) ([]finding, error) {
	var r lintReport
	if err := json.Unmarshal(out, &r); err != nil {
		return nil, fmt.Errorf("reading golangci-lint output: %w", err)
	}
	fs := make([]finding, 0, len(r.Issues))
	for _, i := range r.Issues {
		fs = append(fs, finding{
			file: absolute(base, i.Pos.Filename),
			line: i.Pos.Line,
			tool: i.FromLinter,
			text: i.Text,
		})
	}
	return fs, nil
}

func absolute(base, name string) string {
	if filepath.IsAbs(name) {
		return filepath.Clean(name)
	}
	return filepath.Join(base, name)
}
