# 0124. Each kind of test runs in a CI workflow of its own, selected by the kind its file's name gives, at triggers set by measurement

**Status:** Accepted · **Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

The tests come in kinds that the documents already tell apart. A unit test is example-based or
property-based, and an integration test, which runs against PostgreSQL under the integration build
tag, is example-based, property-based or a crash sequence
([TESTING.md](../../../TESTING.md#what-to-test-with-what)). A mutation demonstration is a different
kind again, since it runs a control's tests with the mechanism broken
([ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md)). Each kind has its own rule for when it
runs. The gating property runs are bounded and deterministic and the deep search runs out of band
([ADR-0055](./0055-property-based-safety-invariants.md),
[ADR-0069](./0069-property-and-crash-sequences-from-rapid.md)). The bounded crash runs gate every pull
request and the deep exploration runs scheduled
([ADR-0045](./0045-crash-injection-testing.md)). The recording of the browser's fixtures runs in the
job of the browser tests that read them
([ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md)).

The workflows did not follow those kinds. The unit workflow ran every test that needs no build tag,
property tests included. The integration workflow ran every test again with the tag, so each unit
test ran twice, and the UI's workflow ran the UI's tests a third time. The deep search ran every
test, example tests included. No workflow ran the mutation demonstrations at all. Nothing stopped a
test from running in two workflows, or a new test from running in none.

Go's test runner selects tests by package and by the test function's name, never by file. A build
tag decides which files compile, so it can keep integration files out of a run, but it cannot keep
the files that need no tag out of a run with the tag. The file names already carry the kind,
`_test.go`, `_property_test.go`, `_crash_test.go`, `_integration_test.go`, as
[CLAUDE.md](../../../CLAUDE.md#tests) lays them out, and the mutation ledger's patches name the
tests they turn red by those tests' names, so renaming the tests would make every patch's preamble
stale.

## Decision

- **A test file's kind is read from its name, and its level from its build constraint.** The kinds
  are unit for a plain `_test.go` file, property for `_property_test.go`, crash for `_crash_test.go`,
  integration for `_integration_test.go`, and fixtures for `_fixtures_integration_test.go`, the
  integration tests that record the browser's fixtures. A file is at the unit level when it compiles
  with no build tag, and at the integration level when it compiles only with the integration tag.
  A unit test is unit level, and an integration or fixtures test is integration level. A property
  test or a crash sequence may be either.
- **One program turns a kind and a level into go test's arguments.** `go tool testkinds args <kind>
  <level>` prints the integration tag when the level needs it, a `-run` pattern naming every
  top-level test, example and fuzz target the kind's files at that level declare, and the packages
  holding those files. Every workflow line that runs `go test` passes those arguments and no other
  selection.
- **Each run is in its workflows and no other, and a check proves it.** `go tool testkinds check`
  runs in the lint workflow. It refuses a test file whose kind and level no workflow runs, and a
  file compiled at neither level. A contract run against a real provider and a violation file are
  apart only while their build tags keep them from both levels. A file named for a violation and
  compiled at a level takes the kind its name gives like any other file, and a contract run
  compiled at a level is refused, since it runs only in its own workflow, `gmail-contract`,
  through `go tool livecontract`. The same refusal stops `args` from printing any selection when a
  contract run compiles at a level in the environment `args` itself runs in, wherever that
  environment's go flags come from, and the check refuses any line naming the livecontract
  command, however it is started, and the variable that marks its run, in any workflow but
  `gmail-contract`.
  It reads which files compile at each level from `go list`, and refuses a name a run's pattern
  holds that also names a test of another kind or level in a package the run lists, because go test
  would run it twice. It reads the workflow files and refuses a run asked for in a workflow that is
  not one of its own, a step asking for a run with no test, and a run with tests that one of its
  workflows never asks for whole, since an ask narrowed to some packages, as the scheduler's race
  run is, adds to the whole ask and never stands for it. A line that runs `go test` fails closed. It
  passes the program's arguments, may add only flags that change neither which tests run nor
  whether a failure fails the step, from a closed list of `-race`, `-count`, `-v` and `-timeout`,
  and may pipe only into `tee`. Any other flag or argument, `-run`, `-skip`, `-tags` and package
  patterns among them, any other way of joining the line to another command, `|| true` included, a
  line continued onto the next, and a workflow holding a job or step condition, `continue-on-error`
  or `GOFLAGS` are refused. The check guards a workflow edit against honest mistakes, and reads
  each workflow's text only for the forms named here. It parses neither YAML nor the shell, so a
  step whose shell, error handling, environment or selection is changed by any other means is left
  to review, as is a `go test` reached through a variable, a script or another program. Examples
  of that class are `set +e` before the line, a `shell:` without `-e` or pipefail, a quoted `'if':`
  key, a `GOENV` file setting go's flags, a process substitution as the `tee` target, and the
  arguments filled from a narrowed ask. The program fails a step whose run has no test, so a
  workflow never passes having selected nothing.
- **One workflow per role, and its triggers alone decide when it runs.** A workflow holds one role,
  and no job in it exists to decide whether another job runs or whether it gates.

  | Workflow | Role | Triggers |
  | --- | --- | --- |
  | `go-unit` | The unit tests, gating | Every pull request touching Go code or the browser bundle's sources |
  | `go-property` | The property tests at the gating seed 1 and 200 cases, gating | Every pull request touching Go code |
  | `go-property-deep` | The deep search over the same property tests, a fresh seed each run and 10,000 cases, gating nothing | Weekly, by hand with an optional case count, and a pull request changing this file |
  | `go-crash` | The bounded crash sequences at seed 1 and 200 cases, gating | Every pull request touching Go code |
  | `go-crash-deep` | The deep exploration over the same crash sequences, a fresh seed each run and 10,000 cases, gating nothing | Weekly, by hand with an optional case count, and a pull request changing this file |
  | `go-integration` | The integration tests, gating | Every pull request touching Go code or the browser bundle's sources |
  | `ui` | The recording of the browser's fixtures, beside the browser tests that read them, gating | Every pull request touching the UI or what its fixtures are recorded from |
  | `mutation-demonstrations` | Every demonstration in the ledger, gating nothing | Weekly, by hand, and a pull request changing this file |

  A property test or a crash sequence has two runs with two characters, so two workflows. The
  gating run takes the fixed seed and the gating case count and gates every pull request. The deep
  run takes a fresh seed and the scheduled count, which a manual run may override, gates nothing,
  fails when no property or sequence reported a passed run, and is never cancelled by the next run.
  Its trigger on a pull request changing its own file exercises it before the change lands.
- **A workflow builds what its tests read.** The UI's unit and integration tests read the built
  browser bundle, the scan of every bundled file for what would need a looser content security
  policy, the entry document loading the bundle's entry module, and the reserved words a bundle's
  top level holds. Against the checked-in placeholder they skip or check less. So the unit and the
  integration workflows build the bundle before their run, as the `ui` workflow builds it before the
  recording, and watch the bundle's sources.
- **The triggers follow the measured cost of each kind.** Measured on this repository's CI runners,
  in the pull request that implements this record. Each time is the wall time of the step that runs
  the tests on a fresh GitHub-hosted `ubuntu-24.04` runner, compilation and PostgreSQL's start under
  pgrun included and the job's setup left out.

  | Run | What it ran | Wall time of the step |
  | --- | --- | --- |
  | Unit | 552 test names in 62 packages | 115 s |
  | The scheduler's tests under the race detector | One package | 28 s |
  | Property, gating, no tag | 36 names in 10 packages | 25 s |
  | Property, gating, integration level | 2 names in 1 package, under pgrun | 20 s |
  | Crash, bounded, integration level | 4 names in 2 packages, under pgrun | 40 s |
  | Crash, deep, 10,000 cases | the same | 65 s |
  | Property, deep, 10,000 cases | the property tests at both levels | 189 s |
  | Integration | 300 names in 18 packages, under pgrun | 91 s |
  | Fixtures, with the UI's build | 3 names in 1 package, under pgrun | 47 s |
  | Mutation demonstrations, the whole ledger | 1,551 patches, 578 of them under pgrun and 66 under bun, across 16 parallel jobs | 964 s to 1,241 s per job, 17,772 s of runner time in all, 23 minutes from the run's start to its end |

  Every gating run fits inside the few minutes a pull request already waits, so each gates every
  pull request its paths touch. The bounded crash runs take seconds of test time, as
  [ADR-0045](./0045-crash-injection-testing.md) expected, and keep gating. A mutation demonstration
  runs a control's tests twice in fresh copies of the tree, and an integration patch starts
  PostgreSQL twice, so the whole ledger costs about five hours of runner time, its sixteen jobs
  holding sixteen runners for twenty minutes. It runs weekly and by hand, which bounds how long a
  demonstration that stopped holding goes unnoticed to a week, and the demonstration of a control
  that lands or changes still runs by hand then, as
  [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) requires.
- **The mutation workflow fails on any demonstration that did not hold.** It is one job over a
  fixed matrix of sixteen. Each job finds the patches as the applicability guard finds them, sorts
  them by the kind of test they run, and takes every sixteenth, reading its index and the number of
  jobs from the matrix itself, so every patch falls in exactly one job and each job gets an even
  share of each kind. A job fails when the runner reports a surviving mutant, a demonstration that
  could not be judged, or fewer demonstrations held than patches given, and when its share is
  empty. It sets the gating seed and case count, so a demonstration proves the tests the gating run
  runs go red.

## Alternatives considered

- **A build tag per kind.** The case for it is that go test selects by file only through build
  tags, so a tag per kind would make each workflow's selection a compile-time fact. Rejected because
  Go cannot keep an untagged file out of a tagged run, so every unit file would need a tag of its
  own, and a test helper shared by two kinds in one package would need both.
- **A name prefix per kind on every test function**, such as `TestProperty…`, with `-run` and
  `-skip` over the prefixes. The case for it is that the selection is one short pattern per workflow,
  with no program. Rejected because it renames every property, crash and integration test, and the
  mutation ledger's patches and rows name the tests they turn red, so each renamed test would make
  its demonstrations stale and, by [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md)'s rule
  that a change to a control's tests reproduces its demonstration, demand them all again.
- **Selection by package**, moving each kind's tests into packages of their own. No case was tabled
  for it. An integration test sits beside the shell it tests, as
  [CLAUDE.md](../../../CLAUDE.md#tests) lays out, and moving it out would separate it from the
  unexported code it reaches.
- **One deep workflow for both kinds.** The case for it is one scheduled workflow for every long
  search. Rejected because it holds two kinds, so neither kind's deep run is visible as its own.
- **The gating run and the deep run of a kind as two jobs of one workflow**, a job condition
  choosing between them by the event and a job reading the pull request's files deciding whether the
  deep job runs on it. The case for it is one file per kind. Rejected because it braids two roles,
  one that gates every pull request and one that searches and gates nothing, into one construct
  whose behaviour depends on conditions a reader has to evaluate, where two workflows let their
  triggers say it.
- **A planning job computing the mutation workflow's shards**, feeding the matrix. The case for it
  is that the shards are computed once and checked to cover every patch. Rejected because a fixed
  matrix whose jobs read their own index and the number of jobs covers every patch exactly once by
  construction, with no job depending on another.
- **The mutation ledger on every pull request, or the patches a pull request's diff affects.** The
  case for the first is that a demonstration would never go stale unnoticed. Rejected on the measured
  cost, hours of runner time on every pull request. The case for the second is the event-driven rule
  of [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) run automatically. Rejected because a
  demonstration is affected by a change to its control, its tests or a generator they draw from,
  which no file list in a diff names, so finding the affected patches would take a dependency
  analysis of its own, and the weekly run already bounds how long a stale demonstration goes
  unnoticed.
- **Running the fixture recording in the integration workflow.** The case for it is that it is an
  integration test like the others. Rejected because
  [ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md) places the recording in the
  job of the browser tests that read the fixtures.

## Consequences

- A new test file is named for its kind, and the check refuses one named for a kind at a level no
  workflow runs, such as a plain `_test.go` file that needs the integration tag.
- Two tests of different kinds may share a name only when no run lists both their packages. The
  check names the collision and one of them is renamed.
- A new kind of Go test, or a new level, is a change to the program's table of runs, to a workflow,
  and to this record.
- A `go test` step that needs a flag outside the closed list changes the program's list, under
  review, since a flag that selects tests or swallows a failure would let a test run twice or in
  none.
- The `-run` pattern grows with the number of tests. An argument past the operating system's limit
  fails the go command loudly rather than running fewer tests.
- Assumptions about other components: `go list` reports which test files compile under a set of
  build tags as go test compiles them, and go test's `-run` selects top-level tests, examples and
  fuzz targets by name, with every subtest of a selected test running. What a workflow's text
  changes by any means other than the forms the check reads is left to review, as the Decision
  states.
