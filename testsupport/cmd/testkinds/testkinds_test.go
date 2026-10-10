package main

import (
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/livecontract"
)

// The module under testdata/module holds a file of every kind at its level, a TestMain, a function
// whose name only looks like a test's, a file compiled only without the tag, a live test and a
// violation file their build tags hold out, and an untagged file whose name holds _violation, which
// is a unit test like any other.
func TestLoadReadsEachFilesKindLevelAndTests(t *testing.T) {
	tr, err := load("testdata/module")
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, f := range tr.files {
		got = append(got, f.pkg+"/"+f.name+" "+f.kind+" "+f.level+" "+strings.Join(f.tests, ","))
	}
	want := []string{
		"a/a_fixtures_integration_test.go fixtures integration TestRecording",
		"a/a_integration_test.go integration integration TestIntegration",
		"a/a_property_test.go property unit TestProperty",
		"a/a_test.go unit unit TestUnit,Example,FuzzUnit",
		"a/scan_violations_test.go unit unit TestPlantedViolationsAreReported",
		"b/b_crash_test.go crash integration TestCrash",
		"b/b_live_test.go   TestLive",
		"b/b_test.go unit unit TestOnlyWithoutTheTag",
		"b/b_violation_test.go   TestViolation",
	}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("files (-want +got):\n%s", diff)
	}
	if problems := tr.check(); len(problems) > 0 {
		t.Errorf("a module with a valid run for every file was refused: %v", problems)
	}
}

// testFile builds a file of a tree, its kind read from its name and level as load reads it.
func testFile(pkg, name, level string, tests ...string) file {
	f := file{pkg: pkg, name: name, kind: kindOf(name), level: level, tests: tests}
	if heldOut(name) && level == "" {
		f.kind = ""
	}
	return f
}

// newTree builds a tree whose packages compile each file at its own level, and a unit-level file at
// the integration level too, as go test compiles a file with no build constraint.
func newTree(files ...file) *tree {
	tr := &tree{pkgs: map[string]*pkg{}}
	for _, f := range files {
		p := tr.pkgs[f.pkg]
		if p == nil {
			p = &pkg{dir: f.pkg, compiled: map[string][]string{}}
			tr.pkgs[f.pkg] = p
		}
		p.all = append(p.all, f.name)
		switch f.level {
		case levelUnit:
			p.compiled[levelUnit] = append(p.compiled[levelUnit], f.name)
			p.compiled[levelIntegration] = append(p.compiled[levelIntegration], f.name)
		case levelIntegration:
			p.compiled[levelIntegration] = append(p.compiled[levelIntegration], f.name)
		}
		tr.files = append(tr.files, f)
	}
	return tr
}

func TestKindOf(t *testing.T) {
	cases := map[string]string{
		"a_test.go":                       kindUnit,
		"crash_test.go":                   kindUnit,
		"property_test.go":                kindUnit,
		"integration_test.go":             kindUnit,
		"a_property_test.go":              kindProperty,
		"a_crash_test.go":                 kindCrash,
		"a_integration_test.go":           kindIntegration,
		"a_fixtures_integration_test.go":  kindFixtures,
		"a_live_test.go":                  kindUnit,
		"a_violation_test.go":             kindUnit,
		"a_violation_property_test.go":    kindProperty,
		"a_violation_integration_test.go": kindIntegration,
	}
	for name, want := range cases {
		if got := kindOf(name); got != want {
			t.Errorf("kindOf(%q) = %q, want %q", name, got, want)
		}
	}
}

// Each file without a run is refused: a test named for a unit run compiled only with the tag, a test
// named for an integration run compiled without it, a test compiled at neither level, and a contract
// run against a real provider compiled at either level, which only its own workflow runs.
func TestCheckRefusesAFileNoWorkflowRuns(t *testing.T) {
	cases := []struct {
		name string
		file file
	}{
		{"a unit-named file needing the tag", testFile("a", "a_test.go", levelIntegration, "TestA")},
		{"an integration file compiled without the tag", testFile("a", "a_integration_test.go", levelUnit, "TestA")},
		{"a fixtures file compiled without the tag", testFile("a", "a_fixtures_integration_test.go", levelUnit, "TestA")},
		{"a file compiled at neither level", testFile("a", "a_test.go", "", "TestA")},
		{"a live test compiled with no build tag", testFile("a", "a_live_test.go", levelUnit, "TestLive")},
		{"a live test compiled with the integration tag", testFile("a", "a_live_test.go", levelIntegration, "TestLive")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			problems := newTree(c.file).check()
			if len(problems) != 1 || !strings.HasPrefix(problems[0], "a/"+c.file.name+": ") {
				t.Errorf("want one problem naming a/%s, got %v", c.file.name, problems)
			}
		})
	}
	if problems := newTree(testFile("a", "a_live_test.go", "", "TestLive"), testFile("a", "a_violation_test.go", "", "TestBan")).check(); len(problems) > 0 {
		t.Errorf("a live test and a violation file were refused: %v", problems)
	}
}

// A name the integration run's pattern holds selects a unit test of the same name in a package the
// run lists, so the check refuses it. The same name in a package the run does not list is no
// collision, since go test never compiles that package for the run.
func TestCheckRefusesANameTwoRunsSelect(t *testing.T) {
	collides := newTree(
		testFile("a", "a_test.go", levelUnit, "TestShared"),
		testFile("a", "a_integration_test.go", levelIntegration, "TestOther"),
		testFile("b", "b_integration_test.go", levelIntegration, "TestShared"),
	)
	want := []string{"a: TestShared, in a file of another kind or level, is also selected by the run of integration tests at integration level"}
	if got := collides.check(); len(got) != 1 || !strings.HasPrefix(got[0], want[0]) {
		t.Errorf("got %v, want one problem starting %q", got, want[0])
	}
	apart := newTree(
		testFile("a", "a_test.go", levelUnit, "TestShared"),
		testFile("b", "b_integration_test.go", levelIntegration, "TestShared"),
	)
	if got := apart.check(); len(got) > 0 {
		t.Errorf("a shared name in a package the run does not list was refused: %v", got)
	}
}

func TestArgs(t *testing.T) {
	tr := newTree(
		testFile("a", "a_test.go", levelUnit, "TestA", "ExampleA"),
		testFile("a", "a_property_test.go", levelUnit, "TestRule"),
		testFile("a/b", "b_test.go", levelUnit, "TestB"),
		testFile("c", "c_property_test.go", levelIntegration, "TestStored"),
		testFile("c", "c_integration_test.go", levelIntegration, "TestC"),
	)
	cases := []struct {
		name     string
		run      pair
		patterns []string
		want     []string
	}{
		{"unit", pair{kindUnit, levelUnit}, nil, []string{"-run=^(ExampleA|TestA|TestB)$", "./a", "./a/b"}},
		{"unit narrowed to a tree", pair{kindUnit, levelUnit}, []string{"./a/..."}, []string{"-run=^(ExampleA|TestA|TestB)$", "./a", "./a/b"}},
		{"unit narrowed to one package", pair{kindUnit, levelUnit}, []string{"./a/b"}, []string{"-run=^(ExampleA|TestA|TestB)$", "./a/b"}},
		{"property without the tag", pair{kindProperty, levelUnit}, nil, []string{"-run=^(TestRule)$", "./a"}},
		{"property with the tag", pair{kindProperty, levelIntegration}, nil, []string{"-tags=integration", "-run=^(TestStored)$", "./c"}},
		{"integration", pair{kindIntegration, levelIntegration}, nil, []string{"-tags=integration", "-run=^(TestC)$", "./c"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := tr.args(c.run, c.patterns)
			if err != nil {
				t.Fatal(err)
			}
			if diff := cmp.Diff(c.want, got, compare.Options); diff != "" {
				t.Errorf("args (-want +got):\n%s", diff)
			}
		})
	}
}

// A run with no test is an error, even when its kind has tests at the other level, and so are
// patterns matching none of a run's packages, because the step would select nothing and pass.
func TestArgsSelectingNothing(t *testing.T) {
	tr := newTree(
		testFile("a", "a_test.go", levelUnit, "TestA"),
		testFile("c", "c_crash_test.go", levelIntegration, "TestCrash"),
	)
	if got, err := tr.args(pair{kindCrash, levelUnit}, nil); err == nil {
		t.Errorf("a level with no test of a kind with tests gave arguments %v", got)
	}
	if _, err := tr.args(pair{kindProperty, levelUnit}, nil); err == nil {
		t.Error("a kind with no test gave arguments")
	}
	if _, err := tr.args(pair{kindUnit, levelUnit}, []string{"./c/..."}); err == nil {
		t.Error("patterns matching none of the run's packages gave arguments")
	}
	if _, err := tr.args(pair{kindUnit, levelUnit}, []string{"a"}); err == nil {
		t.Error("a pattern that is not ./dir gave arguments")
	}
}

// A contract run against a real provider that compiles at a level in the environment args runs in,
// as go's flags from GOFLAGS or a GOENV file would make it, stops every selection, so the live test
// cannot reach a test workflow's run.
func TestArgsRefusesWhileALiveTestCompiles(t *testing.T) {
	tr := newTree(
		testFile("a", "a_test.go", levelUnit, "TestA"),
		testFile("g", "g_live_test.go", levelUnit, "TestLive"),
	)
	if got, err := tr.args(pair{kindUnit, levelUnit}, nil); err == nil {
		t.Errorf("a live test compiled at the unit level, and args gave %v", got)
	}
	held := newTree(
		testFile("a", "a_test.go", levelUnit, "TestA"),
		testFile("g", "g_live_test.go", "", "TestLive"),
	)
	if _, err := held.args(pair{kindUnit, levelUnit}, nil); err != nil {
		t.Errorf("a live test its tag holds out stopped the selection: %v", err)
	}
}

// workflowSet is a workflow file for every workflow a run with tests is in, asking for that run,
// each file with the given extra lines after.
func workflowSet(tr *tree, extra map[string]string) map[string]string {
	files := map[string]string{}
	for _, p := range sortedPairs() {
		if len(tr.tests(p)) == 0 {
			continue
		}
		for _, name := range workflows[p] {
			files[name] += "      run: |\n        selection=\"$(go tool testkinds args " + p.kind + " " + p.level + ")\"\n        go test \"${args[@]}\"\n"
		}
	}
	for name, lines := range extra {
		files[name] += lines
	}
	return files
}

func TestCheckWorkflows(t *testing.T) {
	tr := newTree(
		testFile("a", "a_test.go", levelUnit, "TestA"),
		testFile("a", "a_property_test.go", levelUnit, "TestRule"),
		testFile("c", "c_property_test.go", levelIntegration, "TestStored"),
		testFile("c", "c_integration_test.go", levelIntegration, "TestC"),
		testFile("c", "c_crash_test.go", levelIntegration, "TestCrash"),
		testFile("c", "c_fixtures_integration_test.go", levelIntegration, "TestRecord"),
	)
	if got := checkWorkflows(tr, workflowSet(tr, nil)); len(got) > 0 {
		t.Fatalf("workflows asking for every run were refused: %v", got)
	}
	cases := []struct {
		name  string
		files map[string]string
		want  string
	}{
		{"go test without the selection", workflowSet(tr, map[string]string{"data.yaml": "      run: mise exec -- go test\n"}), ".github/workflows/data.yaml:1: runs go test without"},
		{"go test of a package", workflowSet(tr, map[string]string{"data.yaml": "      run: mise exec -- go test ./db/check\n"}), ".github/workflows/data.yaml:1: adds ./db/check"},
		{"go test with the live contract's tag", workflowSet(tr, map[string]string{"go-unit.yaml": "        go test -tags=gmail_live \"${args[@]}\"\n"}), ".github/workflows/go-unit.yaml:4: adds -tags=gmail_live"},
		{"go test under pgrun with a tag of its own", workflowSet(tr, map[string]string{"go-integration.yaml": "        go tool pgrun -- go test -tags integration ./...\n"}), ".github/workflows/go-integration.yaml:4: adds -tags"},
		{"a selection added to the selection", workflowSet(tr, map[string]string{"go-unit.yaml": "        go test \"${args[@]}\" -run=. ./...\n"}), ".github/workflows/go-unit.yaml:4: adds -run=."},
		{"a skip added to the selection", workflowSet(tr, map[string]string{"go-unit.yaml": "        go test -skip TestA \"${args[@]}\"\n"}), ".github/workflows/go-unit.yaml:4: adds -skip"},
		{"a failure swallowed", workflowSet(tr, map[string]string{"go-unit.yaml": "        go test \"${args[@]}\" || true\n"}), ".github/workflows/go-unit.yaml:4: adds ||"},
		{"a line continued", workflowSet(tr, map[string]string{"go-unit.yaml": "        go test \"${args[@]}\" \\\n"}), ".github/workflows/go-unit.yaml:4: adds \\"},
		{"the selection twice", workflowSet(tr, map[string]string{"go-unit.yaml": "        go test \"${args[@]}\" \"${args[@]}\"\n"}), ".github/workflows/go-unit.yaml:4: passes the selection twice"},
		{"a value for a flag that takes none", workflowSet(tr, map[string]string{"go-unit.yaml": "        go test -v=false \"${args[@]}\"\n"}), ".github/workflows/go-unit.yaml:4: gives -v a value"},
		{"a step condition", workflowSet(tr, map[string]string{"go-unit.yaml": "    - if: false\n"}), ".github/workflows/go-unit.yaml:4: a workflow that runs go test holds"},
		{"a job condition", workflowSet(tr, map[string]string{"go-unit.yaml": "    if: false\n"}), ".github/workflows/go-unit.yaml:4: a workflow that runs go test holds"},
		{"a step carrying on after a failure", workflowSet(tr, map[string]string{"go-unit.yaml": "      continue-on-error: true\n"}), ".github/workflows/go-unit.yaml:4: a workflow that runs go test holds"},
		{"go's flags from the environment", workflowSet(tr, map[string]string{"go-unit.yaml": "        GOFLAGS: -run=TestA\n"}), ".github/workflows/go-unit.yaml:4: a workflow that runs go test holds"},
		{"a run asked for in another workflow", workflowSet(tr, map[string]string{"go-integration.yaml": "        x=\"$(go tool testkinds args unit unit)\"\n"}), ".github/workflows/go-integration.yaml:4: the unit unit run belongs to go-unit.yaml"},
		{"the live contract's command in another workflow", workflowSet(tr, map[string]string{"go-unit.yaml": "      run: mise exec -- go tool livecontract gmail\n"}), ".github/workflows/go-unit.yaml:4: names the livecontract command or its marker"},
		{"the live contract's command run from its directory in another workflow", workflowSet(tr, map[string]string{"go-unit.yaml": "      run: mise exec -- go run ./testsupport/cmd/livecontract gmail\n"}), ".github/workflows/go-unit.yaml:4: names the livecontract command or its marker"},
		{"the live contract's marker in another workflow", workflowSet(tr, map[string]string{"data.yaml": "        " + livecontract.Marker + ": gmail\n"}), ".github/workflows/data.yaml:1: names the livecontract command or its marker"},
		{"a run with no test", workflowSet(tr, map[string]string{"go-crash.yaml": "        x=\"$(go tool testkinds args crash unit)\"\n"}), ".github/workflows/go-crash.yaml:4: no crash test at unit level exists"},
		{"a run that does not exist", workflowSet(tr, map[string]string{"go-unit.yaml": "        x=\"$(go tool testkinds args unit integration)\"\n"}), ".github/workflows/go-unit.yaml:4: unit integration is not a run"},
		{"a run asked for only narrowed", func() map[string]string {
			files := workflowSet(tr, nil)
			files["go-unit.yaml"] = "        selection=\"$(go tool testkinds args unit unit ./a/...)\"\n        go test -race \"${args[@]}\"\n"
			return files
		}(), ".github/workflows/go-unit.yaml: never runs the unit unit tests whole"},
		{"a run no workflow asks for", func() map[string]string {
			files := workflowSet(tr, nil)
			files["go-crash.yaml"] = "on: {}\n"
			return files
		}(), ".github/workflows/go-crash.yaml: never runs the crash integration tests"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := checkWorkflows(tr, c.files)
			if len(got) != 1 || !strings.HasPrefix(got[0], c.want) {
				t.Errorf("got %v, want one problem starting %q", got, c.want)
			}
		})
	}
	for name, extra := range map[string]map[string]string{
		"a comment naming go test":                       {"data.yaml": "    # go test ./db/check runs in go-unit.yaml\n"},
		"a narrowed ask beside the whole one":            {"go-unit.yaml": "        selection=\"$(go tool testkinds args unit unit ./a/...)\"\n        go test -race -count=1 \"${args[@]}\"\n"},
		"every flag that may be added, and a tee":        {"go-unit.yaml": "        go test -count 1 -timeout 110m -v \"${args[@]}\" | tee \"$log\"\n"},
		"a condition in a workflow that runs no go test": {"lint.yaml": "    if: false\n      continue-on-error: true\n"},
		"the live contract in its own workflow":          {"gmail-contract.yaml": "      run: mise exec -- go tool livecontract gmail\n"},
	} {
		if got := checkWorkflows(tr, workflowSet(tr, extra)); len(got) > 0 {
			t.Errorf("%s was refused: %v", name, got)
		}
	}
}
