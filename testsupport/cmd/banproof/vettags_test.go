package main

import (
	"strings"
	"testing"
)

// workflowWithVet is a lint workflow whose one go vet step runs the given command.
func workflowWithVet(run string) []byte {
	return []byte(`
jobs:
  lint:
    steps:
    - name: Lint
      run: golangci-lint run ./...
    - name: Vet
      run: ` + run + `
`)
}

// The go vet step's tags must equal the configuration's as a set, in either spelling of the flag,
// with the last -tags read as the go command reads it, and exactly one step may run the analysers. The configuration's tags here are not the repository's, so a
// check comparing with a list of its own fails the cases that pass.
func TestVetTagProblems(t *testing.T) {
	config := []string{"alpha", "beta"}
	cases := []struct {
		name     string
		workflow []byte
		refused  bool
	}{
		{"same tags", workflowWithVet(`go vet -tags alpha,beta -vettool="$(go tool -n vetcheck)" ./...`), false},
		{"same tags in another order, joined by =", workflowWithVet(`go vet -tags=beta,alpha -vettool=x ./...`), false},
		{"a tag the configuration does not set", workflowWithVet(`go vet -tags alpha,beta,gamma -vettool=x ./...`), true},
		{"a tag the configuration sets left out", workflowWithVet(`go vet -tags alpha -vettool=x ./...`), true},
		{"no tags", workflowWithVet(`go vet -vettool=x ./...`), true},
		{"tags given twice, the last matching", workflowWithVet(`go vet -tags gamma -tags=alpha,beta -vettool=x ./...`), false},
		{"tags given twice, the first matching", workflowWithVet(`go vet -tags alpha,beta -tags gamma -vettool=x ./...`), true},
		{"no go vet step", workflowWithVet(`go test ./...`), true},
		{"two go vet steps", []byte(`
jobs:
  a:
    steps:
    - run: go vet -tags alpha,beta -vettool=x ./...
  b:
    steps:
    - run: go vet -tags alpha,beta -vettool=x ./...
`), true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems, err := vetTagProblems(c.workflow, config)
			if err != nil {
				t.Fatal(err)
			}
			if got := len(problems) > 0; got != c.refused {
				t.Errorf("refused %v, want %v: %v", got, c.refused, problems)
			}
			for _, p := range problems {
				if !strings.HasPrefix(p, lintWorkflow+": ") {
					t.Errorf("problem %q does not name %s", p, lintWorkflow)
				}
			}
		})
	}
}

// A refusal names both lists of tags.
func TestVetTagProblemsNamesBothLists(t *testing.T) {
	problems, err := vetTagProblems(workflowWithVet(`go vet -tags alpha,gamma -vettool=x ./...`), []string{"alpha", "beta"})
	if err != nil {
		t.Fatal(err)
	}
	if len(problems) != 1 || !strings.Contains(problems[0], `"alpha,gamma"`) || !strings.Contains(problems[0], `"alpha,beta"`) {
		t.Errorf("problems %v, want one naming both lists", problems)
	}
}
