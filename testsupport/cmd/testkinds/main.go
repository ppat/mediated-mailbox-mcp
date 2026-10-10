// Command testkinds tells the kinds of Go test apart, so each kind runs in the CI workflow that
// gates a pull request on it, and in no other gating workflow, and no test runs in none (ADR-0124).
// Run it from the repository root.
//
//	go tool testkinds args <kind> <level> [./dir/... | ./dir]...
//	go tool testkinds check
//
// A test file's kind is read from its name and its level from its build constraint.
//
//   - The kind is fixtures for a name ending in _fixtures_integration_test.go, integration for
//     _integration_test.go, property for _property_test.go, crash for _crash_test.go, and unit for
//     every other _test.go file. A violation file, whose name holds _violation, builds only for the
//     ban-proof run, and a contract run against a real provider, named _live_test.go, runs only
//     through its own command, so neither has a kind while its build tags keep it from both levels.
//     A file named for a violation and compiled at a level takes the kind its name gives like any
//     other file. A live file compiled at a level is refused, because it runs only in gmail-contract.
//   - The level is unit when go test compiles the file with no build tag, and integration when it
//     compiles the file only with the integration tag, which the run needs pgrun for.
//
// The runs are the kind and level pairs in the workflows table. Each is in the one workflow that
// gates a pull request on it, and a property test or a crash sequence is also in the workflow of
// its kind's deep search, which gates nothing (ADR-0055, ADR-0045). A unit or an integration file
// has one level, and a fixtures file records the browser's fixtures from the real server in the ui
// workflow's job beside the browser tests (ADR-0064), so it is integration level. Any other pair is
// refused, and so is a test file at neither level that has a kind.
//
// args prints, one per line, the arguments that make go test run exactly one pair's tests. That is
// the integration tag when the level needs it, a -run pattern naming every top-level test, example
// and fuzz target the pair's files declare, and the packages holding those files, narrowed to the
// given package patterns when there are any. The caller passes them with "${args[@]}". Go selects
// by package and by name and never by file, so a name the pattern holds also runs in any other file
// of a listed package compiled at that level. check refuses that collision. args fails when the pair
// has no test or the patterns match none of its packages, since a step that selects nothing would
// pass green.
//
// check requires every test file to have a valid pair, requires each pair's arguments to select
// exactly that pair's tests and nothing else, reading which files compile at each level from go
// list, and reads the workflows under .github/workflows. Every line that runs go test there passes
// "${args[@]}" once and adds nothing but -race, -count, -v, -timeout and a pipe into tee, every args
// line sits in a workflow its pair runs in and names a pair that has tests, and every pair with a
// test is asked for whole, with no package pattern, by each of its workflows. A narrowed ask, such
// as the scheduler's race run, adds to the whole one. A workflow that runs go test holds no job or
// step condition, no continue-on-error and no GOFLAGS, and no workflow but gmail-contract has a
// line naming the livecontract command, however it is started, or the variable that marks its run.
// args itself refuses to print a selection when a contract run compiles at a level in its own
// environment, wherever go's flags come from.
//
// The check guards a workflow edit against honest mistakes and reads each workflow's text only for
// the forms named above. It parses neither YAML nor the shell, so a step whose shell, error
// handling, environment or selection is changed by any other means is left to review, as is a go
// test reached through a variable, a script or another program. Examples of that class are set +e
// before the line, a shell: without -e or pipefail, a quoted 'if': key, a GOENV file setting go's
// flags, a process substitution as the tee target, and the arguments filled from a narrowed ask.
//
// The -run pattern grows with the number of tests, and an argument longer than the operating
// system's limit, 128 KiB on Linux, fails the go command loudly rather than running fewer tests.
package main

import (
	"fmt"
	"os"
)

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, "testkinds:", err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if _, err := os.Stat("go.mod"); err != nil {
		return fmt.Errorf("run from the repository root, where go.mod is")
	}
	if len(args) == 0 {
		return usage()
	}
	switch args[0] {
	case "args":
		if len(args) < 3 {
			return usage()
		}
		p := pair{kind: args[1], level: args[2]}
		if _, ok := workflows[p]; !ok {
			return fmt.Errorf("%s %s is not a run, which are %s", p.kind, p.level, runList())
		}
		tree, err := load(".")
		if err != nil {
			return err
		}
		out, err := tree.args(p, args[3:])
		if err != nil {
			return err
		}
		for _, a := range out {
			fmt.Println(a)
		}
		return nil
	case "check":
		if len(args) != 1 {
			return usage()
		}
		tree, err := load(".")
		if err != nil {
			return err
		}
		files, err := readWorkflows(".github/workflows")
		if err != nil {
			return err
		}
		problems := append(tree.check(), checkWorkflows(tree, files)...)
		for _, p := range problems {
			fmt.Fprintln(os.Stderr, p)
		}
		if len(problems) > 0 {
			return fmt.Errorf("%d problem(s)", len(problems))
		}
		return nil
	}
	return usage()
}

func usage() error {
	return fmt.Errorf("usage: go tool testkinds args <kind> <level> [package pattern]... | go tool testkinds check")
}
