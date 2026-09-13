# 0069. Property tests and generated crash sequences use rapid, and the project writes three pieces of test support alongside it

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[C3](../../../USE_CASES.md#c3--content-based-secrets-caught),
[A3](../../../USE_CASES.md#a3--bulk-change-is-reversible),
[A4](../../../USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything),
[O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

### What is already settled, and what that leaves

[ADR-0055](./0055-property-based-safety-invariants.md) decides which property-based tests exist. It
explains what a property-based test is and names the kinds of rule such a test may check. It
requires that a gating run use a fixed number of generated cases and a fixed seed, that a failing
input once found is stored and replayed on every later run, and that a longer random search runs on
a schedule outside the gating run using the same test definitions.

[ADR-0045](./0045-crash-injection-testing.md) decides the crash-injection harness. It generates
sequences of named operations with a simulated crash placeable anywhere in the sequence, runs the
real recovery path after each crash, and checks that what was reported durable survived and that
recovery finished. It fixes a deliberately coarse crash model and two targets, the reorganization
apply and rollback path first and backfill resume second. The harness itself, meaning the crash
operation's behaviour, the call into recovery and the two checks, is this project's own code.

[ADR-0042](./0042-implementation-stack.md) fixes Go on the server, so every candidate is a Go option
and the tests run under the ordinary `go test` command.
[ADR-0070](./0070-unit-comparison-through-one-options-value.md) decides how values are compared,
[ADR-0068](./0068-test-substrate-containers-directly.md) how the test database is started, and
[ADR-0043](./0043-no-mocking.md) forbids mocks.

What is left to choose is narrower than "a property-testing library". It is what produces generated
values, what produces generated operation sequences, what keeps and replays a failing case, and what
reduces a failing case to a small one. These are two jobs, one for property tests and one for crash
sequences. They are judged separately and may land on different answers.

Four words recur below.

- **Reduction**, also called shrinking. When a generated input breaks a rule, the tool searches for
  a smaller input that breaks it the same way, so a person can see the cause. Forty operations that
  fail together are not a diagnosis. The three that cause the failure are.
- **Integrated reduction.** The library records the stream of random numbers it handed the generator
  and reduces that stream, so the generator is re-run on shorter or smaller streams.
- **Type-directed reduction.** The reducer works on the value itself, for example by removing an
  item from a list, and never sees a stream of random numbers.
- **A fail file.** The file a library writes to disk holding a failing case so a later run can
  replay it.

### Why this is not the problem it looks like

It looks like a comparison of library features. Two facts about this project make it something else.

**Values here are built through validating constructors.**
[ADR-0042](./0042-implementation-stack.md) gives every sensitivity-carrying type unexported fields
and an exported constructor that refuses an invalid combination, and the zero value of such a type
is its most restrictive state. So a generator that draws each field on its own and hands the result
to the constructor, called an **independent-draw generator** below, has much of its output refused.
The obvious repair is a **conditional generator**, one that decides what region of inputs it wants
and builds only values the constructor accepts. That repair teaches the generator the very rule
under test, which is the hazard [ADR-0055](./0055-property-based-safety-invariants.md) exists to
avoid. And a conditional generator is the shape integrated reduction handles worst, because reducing
a random number that chose a region changes what every later number means.

Whether this binds depends on the size of the space being generated. The sensitivity space the
design defines is small. [DESIGN.md](../../../DESIGN.md#glossary) makes sender class `normal` or
`restricted` and content flags an MFA code or a login link, and
[ADR-0007](../redaction/0007-composite-scan-gate.md) gives four scan states. That is 32
combinations, of which exactly 2 release a body, a normal sender with no flags that is either
scanned or skipped by the scan gate. Thirteen of the 32 can occur. ADR-0007's gate sends a
restricted sender's unscanned mail to `SKIPPED_RESTRICTED`, content flags come only from a scan, and
[ADR-0037](../redaction/0037-delisting-transition.md) returns a delisted sender's messages to
pending scan, so a normal sender's mail is never left at `SKIPPED_RESTRICTED`. Those rules leave
eight combinations. [ADR-0002](../redaction/0002-fetch-time-re-evaluation.md) applies a newly listed
sender on the next call, so a restricted sender with mail already scanned or already skipped by the
gate also occurs. That adds five, and it is exactly the case ADR-0002 exists for.

A **boundary combination** below means one where exactly one of sender class, content flags and scan
state denies release, so a single change would release the body. Those are where a leak would hide.
At this size an independent-draw generator reached the releasable and boundary combinations within a
run of 200 cases, so the repair is not needed and the conflict with integrated reduction does not
arise. At 100 cases the median run had not yet reached every boundary combination.

**Several requirements look like library features and are not.** Reducing a crash sequence against
an in-memory model rather than the real database is a property of the harness. A fixed seed reaching
every package is a property of how the suite is invoked. Reducing to a small case separates nothing,
because every candidate that reduces at all meets it.

### The requirements

| Requirement | What it demands | Source |
| --- | --- | --- |
| Runs under `go test` | An ordinary import called from test functions. No second runner and no separate binary in the gating run | [ADR-0042](./0042-implementation-stack.md), [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md) |
| Values built through exported constructors | The generator never produces a value by any route that skips the constructor. A value built that way arrives at its zero value, which here is the most restrictive state, so a property over it passes while checking nothing. The catalogue rows proving that such values cannot be built depend on the same rule | This record's requirement, from [ADR-0042](./0042-implementation-stack.md)'s construction rule and the construction rows in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) |
| Test-path dependency footprint | What the choice adds to the dependency tree the test path carries | This record's requirement. It applies to the test path the same posture [ADR-0068](./0068-test-substrate-containers-directly.md), [ADR-0070](./0070-unit-comparison-through-one-options-value.md) and [ADR-0071](./0071-static-enforcement-toolchain.md) apply to theirs |
| Files the tool writes are test data | A fail file is a test input, so [ADR-0044](./0044-synthetic-fixtures-marker-text.md)'s synthetic-fixture rule reaches it | [ADR-0044](./0044-synthetic-fixtures-marker-text.md) |
| Comparison through the shared options value | A property or harness check comparing two values goes through the shared comparison options | [ADR-0070](./0070-unit-comparison-through-one-options-value.md) |
| Nothing pushes toward a mock | No mock, spy or stub surface in the ordinary path | [ADR-0043](./0043-no-mocking.md) |
| A red run is attributable | Running the suite against a deliberately broken mechanism shows which tests went red, and why | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| Fixed case count and fixed seed | A gating run generates the same cases on any machine | [ADR-0055](./0055-property-based-safety-invariants.md) |
| A failing case is kept on the day it is found | Written where the repository holds it and replayed automatically on later runs, with no person copying a seed | [ADR-0055](./0055-property-based-safety-invariants.md) |
| A kept failing case survives a generator edit | After somebody edits the generator, the kept case still reproduces its failure, or the run fails loudly saying it no longer can | This record's requirement. Keeping a case that silently stops reproducing does not meet [ADR-0055](./0055-property-based-safety-invariants.md)'s requirement that a found failure stays found |
| Longer search on the same definitions | A scheduled run raises the case count without a second set of tests | [ADR-0055](./0055-property-based-safety-invariants.md) |
| The kinds of rule ADR-0055 allows can be expressed | The kinds ADR-0055 admits, among them model-based properties, metamorphic relations, round trips, idempotence, postconditions and stateful properties | [ADR-0055](./0055-property-based-safety-invariants.md) |
| Reports what the generator produced | States a required mix of input kinds and fails the run when the generator misses it | This record's requirement. It was graded but never used to rule a candidate out. It answers the blind spot [ADR-0055](./0055-property-based-safety-invariants.md) names |
| Operation sequences with a crash step | Generated sequences of named operations, a crash placeable anywhere and more than once | [ADR-0045](./0045-crash-injection-testing.md) |
| Reduction does not replay against the real database | Reduction runs against an in-memory model of the machinery, and only the reduced sequence replays against PostgreSQL | This record's requirement. Each replay against the database costs a reset |

### How the requirements were weighted

Two requirements ordered the field. **Values built through exported constructors**, because a
property that silently builds a zero value checks nothing while looking green, and does it on the
types this system most needs checked. And **a failing case kept on the day it is found**, because
ADR-0055 requires it and a candidate lacking it is deciding against that record.

Operation sequences with a crash step, and a kept failing case that survives a generator edit, came
next.

Where a library and hand-written code both meet the requirements, **the library is preferred unless
it makes the whole worse**. So the burden of proof sits on writing it by hand.

Footprint broke ties. Reporting what the generator produced was graded and never allowed to rule a
candidate out.

Three things were deliberately not graded. Fine-grained crash enumeration, because ADR-0045 decides
against it. Integration with coverage percentages, because
[ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) sets no coverage target. And **reducing a
failing case to a small one**, because every candidate that reduces at all meets it, so it separates
nothing. The one candidate it would have failed, `testing/quick`, has no reduction at all and fails
elsewhere too.

Libraries outside Go were not candidates. They would be considered only if every Go option clearly
failed the requirements, which none did. What they offer and no Go option does is stating a required
mix of generated inputs and failing the run when it is not reached.

## Decision

### What was chosen

- **`pgregory.net/rapid` generates values and operation sequences, and reduces failing ones, at
  version 1.3.0 or later.** The minimum is there because 1.3.0 is the first release that reads its
  settings from environment variables, `RAPID_SEED`, `RAPID_CHECKS` and `RAPID_NOFAILFILE`. Its
  command-line flags exist only in packages that import it, so a repository-wide `go test ./...`
  carrying them fails in every other package. The environment variables reach every package.
- **The reason for the library over writing it all by hand.** Both rapid's reduction and a
  hand-written reducer meet the requirement, so reduction capability does not separate them. What
  separates them is who carries the risk of a defect in reduction. A reducer with a subtle bug does
  not turn a test red. It gives a confident, small and wrong explanation of a failure, and nothing
  about the output shows it. Reduction is the kind of code where the next such defect is not
  visible, and rapid's reducer is used by many other projects, which makes it more likely its
  defects have already surfaced. That last point is a reasonable expectation rather than a measured
  fact. It does not depend on the size of the sensitivity space or on the gating case count. It does
  depend on both reducers meeting the requirement.
- **Generators for sensitivity-carrying values draw each field independently rather than being
  conditional generators, and generators that build lists use rapid's collection generators, such as
  `rapid.SliceOfN`.** The rule covers how values and lists are built. Which operation a crash
  sequence runs next is drawn by `t.Repeat` and the operation sampler, below. For
  sensitivity-carrying values, at this system's size that shape reached the releasable and boundary
  combinations within a 200-case run, and rapid reduces it well. For the reorganization plan
  generator in the crash harness, a collection generator draws each plan operation without seeing
  the ones drawn before it. A conditional generator produced the kind of plan operation it targeted
  in 77.0 percent of cases, against 59.2 percent for the collection-generator shape, where the
  generator report required at least 25 percent. The collection-generator shape cleared that
  minimum, so it is used there too. Whether the same holds for the backfill target's generators is
  left open below.
- **Three pieces of test support are written and owned by this project.** The generator report
  answers a gap every Go candidate has. The failing-case store answers a gap every candidate has
  except Go fuzzing, which stores the literal input. Both builds compared in the Alternatives carry
  the two. The operation sampler answers how rapid in particular draws operations, and part of what
  it does is available in another Go library, so it is a cost of this choice.

| Piece | What it does | Why |
| --- | --- | --- |
| The generator report | Classifies each generated case, reports the mix, and fails the run when a stated minimum share of a kind of input is not reached | No Go library can fail a run on a missed mix. [ADR-0055](./0055-property-based-safety-invariants.md) names the resulting blind spot. A draw the constructor refuses is counted as its own kind of input, which is how the report shows most of a run being refused |
| The failing-case store | Stores a failing case as the arguments passed to the constructor plus a description of the value's shape, and replays it | rapid's own fail file stops reproducing after a generator edit, silently, on two separate triggers set out below. The store's cases still reproduce after a generator edit, and fail loudly with an instruction when the value's shape changes |
| The operation sampler | For the crash harness's scheduled run, draws a fresh set of operation weights for each generated sequence | rapid draws operations unevenly, by the sorted position of their names. Fixed hand-picked weights make that worse. A fresh set per sequence was the best option at the gating budget, and the only one to reach a planted failure needing four back-to-back crash and recovery cycles at all, in 8 percent of runs on the reorganization target and 12 percent on backfill, falling to none with more cycles. No generated search tried reached such a failure reliably. An example-based test written for that failure would reach it. rapid's own draw did slightly better, 98 percent against 95, on a planted failure needing four writes in a row on the backfill target |

### What rapid's ordinary path does that other records forbid, and what stops it

Each numbered row is one rule a builder follows. The last column says what enforces it.

| # | The ordinary path | What goes wrong | The rule, and what enforces it |
| --- | --- | --- | --- |
| 1 | With no seed set, or a seed of zero, the seed comes from a per-process hash | A gating run differs on every machine, so it is not the fixed run ADR-0055 requires | The gating invocation sets `RAPID_SEED` to a non-zero value and sets `RAPID_CHECKS`. **A test fails unless `RAPID_SEED` is set and non-zero** |
| 2 | `go test -short` divides rapid's case count by five and its sequence length by two | The gating run silently becomes a fifth of the run it claims to be | The gating invocation does not pass `-short`. **Review** |
| 3 | `rapid.Make` and `rapid.MakeCustom` build a struct by reflection and skip unexported fields | Every unexported field arrives at its zero value with no error. Here that is the most restrictive state, so a property over it passes while checking nothing | Neither is called. **A named-identifier ban in [ADR-0071](./0071-static-enforcement-toolchain.md)'s linter**, proven by a checked-in file that violates it |
| 4 | A generator writes a struct literal inside `rapid.Custom` | The constructor is bypassed without naming any banned identifier | Generators live in a test package whose name ends in `_test`, which cannot see the unexported fields of the package it tests. **The compiler** refuses the literal there |
| 5 | rapid is imported wherever a test wants it | The rules in this table cannot be enforced in packages nobody is watching | The import is confined to the packages holding property tests or running generated crash sequences, the crash harness, the three pieces of test support, and the violation files of the analyser in rows 8 and 9. **The linter's import rule** |
| 6 | rapid writes a fail file under `testdata/rapid/` when a property fails | After a generator edit that changes how many values are drawn, rapid reports the file invalid through the test log, which a normal run does not print, and passes. After an edit that keeps the number of draws, it reinterprets the stored numbers and reports nothing. A future release changing the file's format stamp invalidates every kept case the same way. Each time the run is green with the fault still present | The gating invocation sets `RAPID_NOFAILFILE` to `true`. **A test fails unless it is set to `true`.** The failing-case store keeps the case instead, and every reduced failing case is also written out as an ordinary example-based test, which states its own expected answer and cannot drift with the generator |
| 7 | Before generating, rapid replays every fail file it finds under `testdata/rapid/`, whether or not writing is switched off | A fail file written by a local run without the setting above, once committed, replays in every gating run with the same silent failures as row 6 | No fail file is kept in the repository. **A test fails when any file exists under `testdata/rapid/`** |
| 8 | A property records the generated mix while it can still fail | rapid re-runs a failing property many times while reducing it, and a recorder inside the property counts every re-run. On a failing run a mix whose true share was 33.3 percent was reported as 18.7 percent | The generator report runs in its own property containing nothing that can fail, the generator included. **A purpose-written `go vet` analyser** rejects a report call reachable from a property that can fail, following calls through local helpers |
| 9 | Code placed after `rapid.Check` in the same test | rapid ends a failing run through `runtime.Goexit`, so nothing after the check runs on exactly the run that found something | The failing-case store is written from a cleanup registered before the check. **The same analyser** rejects a store write from inside a property |
| 10 | A generator for a sensitivity-carrying value is written as a conditional generator | It teaches the generator the rule under test, and rapid reduces that shape worst | Such generators draw each field independently. **Review**, because no tool can recognise the shape of a generator |
| 11 | A generator builds a list by drawing a length and then looping | In the case measured rapid did not reduce a list built that way, because lowering the drawn length changes how many draws follow it and the shorter case stops failing. A plan generator written like this left all seven plan operations in place after 4,843 runs, and rewriting it with `rapid.SliceOfN` fixed it | Lists are built with rapid's collection generators. **Review** |
| 12 | The operation sampler's weights are drawn outside rapid | rapid replays the sequence many times while reducing it. Weights from outside change on every replay, and one reduction saw 13 different weight sets with no warning | The sampler draws its weights from rapid's own random stream. **Review** |
| 13 | The sampler's weights are stored with a failing case | rapid's reduction flattens the weights while keeping the sequence it reports, measured from a crash share of 0.42 to 0.33 in a reported sequence still holding four crashes. Replaying from stored weights reproduced the failure 3 times in 200 | The stored case holds the operation sequence, which replayed the failure every time, and has no field for weights. **The compiler** |
| 14 | A failure report quotes the sampler's weights | The weights shown contradict the sequence shown in the same report, for the reason in row 13 | A failure report describes the operation sequence and never the weights. **Review**, because a report can format any value |

One further piece of rapid's ordinary path is answered by a component rather than a rule. `t.Repeat`
draws the next operation by position in the sorted list of operation names, so renaming an operation
changes how often it runs. On the backfill target the crash was drawn 1.37 times as often as the
write whose unflushed state is the actual hazard. The operation sampler answers it in the scheduled
run.

Of the fourteen rules, six are enforced by the linter, the compiler or the analyser, being rows 3,
4, 5, 8, 9 and 13. Three are enforced by a test in the suite, rows 1, 6 and 7. Five rest on review,
rows 2, 10, 11, 12 and 14.

### Anything deliberately left open

**The gating case count.** ADR-0055 requires a fixed count and does not fix one. The counts measured
were 100 and 200, and nothing limits the choice to those. At 200 an independent-draw generator
reached both releasable combinations and all the boundary combinations in every one of twelve seeds
tried, with the boundary set needing a median of 103 cases and at worst 171. At 100 that median
makes coverage close to a coin flip, while a conditional generator reaches the releasable
combinations in a median of 6 cases. So 100 costs half the run time of 200 and leaves the choice of
generator shape in doubt, 200 settled it in every seed tried, and a higher count costs more run time
again. The coverage figures were measured over all 32 combinations rather than the 13 that can
occur, which makes the boundary comparison between the two generator shapes weaker than the numbers
suggest.

**Whether the coverage holds for the real generators.** The coverage above was measured against a
model built from the design's three axes, not against this project's generators. One option is to
write the real generator for one sensitivity-carrying type twice, once independent-draw and once
conditional, and compare how often each reaches a released body before writing the rest. The other
is to write the generators as decided and let the generator report show the mix on every run. The
first costs one generator written twice and settles the question before anything depends on it. The
second costs nothing up front and finds out later.

**Whether the backfill target's generators use the same shape.** The collection-generator shape was
measured for the reorganization plan generator only. For backfill resume, one option is to use the
same shape and read the generator report, and the other is to measure a conditional generator
against it first. The first costs nothing up front. The second costs one generator written twice,
and matters if the interesting states in backfill are rarer than in a reorganization plan. rapid's
collection generators draw each element without seeing the ones drawn before it, so a backfill
generator conditioning an element on earlier ones would also change row 11 for backfill.

**Whether one in-memory model serves both crash-harness targets.** Reduction runs against an
in-memory model of the machinery. One model shared by the reorganization apply path and backfill
resume must be general enough for two machineries that checkpoint differently. A model per target
costs writing two. Nothing outside the harness depends on which.

**Whether the operation sampler also runs in the gating run.** It is decided for the scheduled run.
A longer gating run does not substitute for it, and at the gating budget a planted failure the
harness caught 28 percent of the time is, with the seed fixed as ADR-0055 requires, a 72 percent
chance the gating run never catches it. Running it in the gating run as well costs gating time.
Leaving it out keeps that class of failure to the scheduled run, where the longer search already
runs.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| Runs under `go test` | rapid is an ordinary import. Settings arrive as environment variables so a repository-wide invocation works |
| Values built through exported constructors | Generators call the constructors inside `rapid.Custom`. The linter bans `rapid.Make` and `rapid.MakeCustom`. The external test package makes a literal with unexported fields a compile error. The two catch different things, a named facility and an unnamed literal |
| Test-path dependency footprint | rapid declares no dependencies, so it adds only itself to the module graph |
| Files the tool writes are test data | rapid writes no fail file and none is kept in the repository. The failing-case store writes constructor arguments, and generated mail values carry [ADR-0044](./0044-synthetic-fixtures-marker-text.md)'s marker text |
| Comparison through the shared options value | Properties and harness checks return a `cmp.Diff` report through [ADR-0070](./0070-unit-comparison-through-one-options-value.md)'s shared options |
| Nothing pushes toward a mock | rapid has no mock, spy or stub surface |
| A red run is attributable | No fail file exists under `testdata/rapid/`, so none replays ahead of the generated run, and a red comes from the run itself. The analyser keeps the report and the store out of failing properties, so neither distorts the attribution |
| Fixed case count and fixed seed | `RAPID_CHECKS` and a non-zero `RAPID_SEED` in the gating invocation, a test failing unless that seed is set, and no `-short` |
| A failing case is kept on the day it is found | The failing-case store, replayed on the next run with no person involved |
| A kept failing case survives a generator edit | The failing-case store, which stores constructor arguments rather than rapid's random numbers, and the reduced case written out as an example-based test. The store catches a generator edit. The written-out test catches everything else, because it states its own answer |
| Longer search on the same definitions | A scheduled run raising `RAPID_CHECKS`. rapid's `MakeFuzz` can also turn a property into a Go fuzzing target for coverage-guided search, though that path does not reduce and stores only an opaque byte string |
| The kinds of rule ADR-0055 allows can be expressed | `rapid.Check` for single-value properties and `t.Repeat` for operation sequences |
| Reports what the generator produced | The generator report, in its own property |
| Operation sequences with a crash step | `t.Repeat` with the crash as one named operation, lists inside operations built with collection generators, and the operation sampler in the scheduled run |
| Reduction does not replay against the real database | Reduction runs against the in-memory model. The in-memory model and PostgreSQL agreed on every check across 450 cases, and on the two builds with a planted fault at least one check fired in 70 of 150 cases on one and in at least 132 of 150 on the other, so the model can carry the search and the database only the final replay |

### What the implementer would otherwise pay to discover

- **Most of an independent-draw run is refused by the constructors.** A generator with no
  deliberately invalid draws was refused 93.8 percent of the time when it always asked for a body,
  and 46.5 percent in a milder case. That is why the generator report matters. The repair of
  generating only accepted values is the conditional generator this decision rules out for
  sensitivity-carrying values, because it teaches the generator the rule under test.
- **Some failures cannot be reduced to their true minimum by either reducer.** A failure needing two
  fields to change together left both rapid and a hand-written reducer at the same larger case. The
  written-out example-based test should say which fields matter rather than trusting the reduction.
- **A check comparing the state before a change with the state after it passes when the code does
  nothing.** Deliberately breaking the apply step so it did less left every such comparison in the
  harness green, including the one comparing the rolled-back state with the original. Only checks
  against an absolute expected value caught it. Deliberately breaking the restore was caught only by
  the comparison. The harness needs both kinds.
- **An assertion that fires rarely is reached at the gating count by luck.** One harness check fired
  on 7.8 percent of cases. A mutation demonstration relying on it should run at the scheduled count.
- **Tuning a generator to satisfy the generator report can blind a mutation demonstration.** A
  generator edited until its report passed moved the first failure on a planted defect from case 14
  to case 291. At gating counts of 100 and 200 the mutation demonstration then stayed green when it
  had to go red, and went red again only at 500. The report was right that the stated mix was
  reached. It cannot say a kind of input nobody named was lost.
- **The real database is 34 to 40 times slower than the in-memory model** on the same sequences.
- **Fixed operation weights tuned for one target can destroy another.** Weighting the crash down
  helped the reorganization target slightly and took detection of one planted failure on the
  backfill target from 55 percent to 0, because the same weights raised the checkpoint that resets
  the run.

## Alternatives considered

### The grid

Each candidate is graded on every requirement. **4** means it carries the requirement natively.
**3** means it carries it with bounded discipline or one small piece of owned code. **2** means it
carries it only by convention, or with a named trap. **1** means it cannot honestly carry it. "No
library" means hand-written generators, a seeded bounded run loop, a failing-case store, an
operation sampler and a type-directed reducer. It was built and measured, not estimated.

| Requirement | rapid | No library | Go fuzzing | gopter | testing/quick |
| --- | --- | --- | --- | --- | --- |
| Runs under `go test` | 4 | 4 | 4 | 4 | 4 |
| Values built through exported constructors | 3 | 4 | 4 | 2 | 2 |
| Test-path dependency footprint | 4 | 4 | 4 | 3 | 4 |
| Files the tool writes are test data | 3 | 4 | 3 | 4 | 4 |
| Comparison through the shared options value | 4 | 4 | 4 | 3 | 2 |
| Nothing pushes toward a mock | 4 | 4 | 4 | 4 | 4 |
| A red run is attributable | 3 | 4 | 3 | 3 | 2 |
| Fixed case count and fixed seed | 4 | 4 | 2 | 3 | 3 |
| A failing case is kept on the day it is found | 4 | 3 | 4 | 1 | 1 |
| A kept failing case survives a generator edit | 2 | 4 | 3 | 1 | 1 |
| Longer search on the same definitions | 4 | 4 | 3 | 3 | 4 |
| The kinds of rule ADR-0055 allows can be expressed | 4 | 4 | 3 | 4 | 2 |
| Reports what the generator produced | 1 | 4 | 1 | 2 | 1 |
| Operation sequences with a crash step | 3 | 4 | 2 | 4 | 1 |
| Reduction does not replay against the real database | 4 | 4 | 3 | 4 | 1 |

`gopter` is `github.com/leanovate/gopter`. Go fuzzing is the toolchain's `go test -fuzz`. Each
column is graded on the candidate alone. The decision switches rapid's own fail file off and keeps
failing cases in the failing-case store, so in the chosen configuration rapid's 4 on keeping a
failing case is not the mechanism in use.

### What the grid shows

Two rows separated nobody. Every candidate runs under `go test` and none has a mock surface, because
none of them is a mocking tool. Footprint barely separated, because rapid and the standard library
options add nothing and gopter's cost is in its module graph rather than its compiled code.

The rows that were measured by building and running are constructors, footprint, the fixed seed for
rapid, both kept-case rows, operation sequences, and every cell of the no-library column. The rest
were read from source and documentation.

Counted down each column, rapid has one cell at 1, one at 2 and four at 3. No library has one cell
at 3 and nothing lower. Go fuzzing has one at 1, two at 2 and six at 3. gopter has two at 1, two at
2 and five at 3. `testing/quick` has five at 1, four at 2 and one at 3.

rapid's two lowest cells are the two that the generator report and the failing-case store exist for.
The no-library column is the only one with no cell below 3.

### What the grid cannot show

The grid does not separate rapid from writing everything by hand. These facts do.

| | rapid with the owned pieces | No library |
| --- | --- | --- |
| Who carries a defect in reduction | A reducer used by many other projects | A reducer this project owns and must keep correct, roughly 50 more lines for every new type |
| Rules the build imposes | Fourteen, numbered in the ordinary-path table, of which six are enforced by a tool or the compiler, three by a test in the suite, and five by review | One, that the seed is read from an environment variable |
| Tooling that exists only because of the library | A linter ban, an import rule, and the `go vet` analyser | None |
| Project code, including the three owned pieces | About 717 lines | About 764 lines |

The line counts are from builds made to answer these questions rather than finished test support,
and the difference is too small to decide anything.

### The reading

**Whose worst cell is best.** Writing everything by hand, whose worst cell is a 3. rapid's worst is
a 1, on reporting what the generator produced. That weakness is not confined to rapid. It is closed
by a piece this project writes under either choice, so the higher floor of the no-library column is
bought with the same owned code.

**Which strengths guard something nothing else does.** Of the strengths of rapid's that the decision
uses, the fixed seed is also guarded by the catalogue row that fails the suite when the seed is
unset. Reduction is guarded by nothing else. Whether a reported failure is explained correctly
depends only on the reducer, and that is where rapid's wide use counts.

**What regretting each would cost.** Regretting rapid means replacing the run harness and taking on
a reducer. The properties, the generators, the three owned pieces and the harness checks survive,
because they are ordinary Go and the project's own. Regretting no library means discovering a
reduction defect by a wrong diagnosis, which nothing flags, and then fixing a reducer this project
owns. Both exits are cheap in code. The second is expensive in how late it surfaces.

**Which strengths can be had without choosing a candidate.** The no-library column's two largest
leads, reporting what the generator produced and a kept case surviving an edit, come from the owned
pieces, and those are had alongside rapid. Its smaller leads, on constructors, on files written to
disk and on attribution, come from not having rapid's facilities to misuse, and the rules in the
ordinary-path table close each of those. rapid's one strength that cannot be had without choosing it
is the reducer. That is why the column with the best floor is not the choice.

### rapid

**For it.** It declares no dependencies, so it adds only itself to the module graph. Its integrated
reduction took a forty-step failing sequence to exactly the three operations that caused it with no
arrangement by the test author. It keeps a failing case and replays it on the next run, a capability
this decision stops relying on in favour of the failing-case store for the reasons in the
ordinary-path table. It has a fixed-count, fixed-seed mode set from the environment, and raises the
count for a longer scheduled search over the same tests. Its compatibility promise for its major
version has held since 1.0.0. Its maintainer is active, with outside contributors, and version 1.3.0
was released in 2026.

**Against it.** Its fail file silently stops reproducing after a generator edit, on two separate
triggers, and the run passes. Its operation draw depends on how operation names sort. It offers a
reflection-based struct builder that silently produces zero values, listed in its own documentation
beside the safe path. It cannot report what its generator produced. Its maintainer wrote in 2021
that he was not in favour of building that in, later invited a proposed change, and that change has
been untouched since 2022. Closing all of this costs fourteen rules, a linter ban, an import rule
and a purpose-written analyser, where the hand-written build needs one rule. Its flags break a
repository-wide test run, and `-short` silently shrinks the gating run. Each of these is closable,
and closing them is a standing cost the project carries for as long as it uses the library.

### No library

**For it.** It grades 3 or better on every requirement in the grid. Its failing-case store survives
a generator edit, because it stores the constructor's arguments rather than a stream of random
numbers. It answers what the generator produced. Its sequence reducer reached the smallest
reproducing case on every seed tried. It needs one rule and no tooling beyond what exists.

**Against it.** The project owns reduction for as long as it has property tests. A reducer that asks
whether a shortened case still fails, rather than whether it still fails the same way, gives wrong
diagnoses. Measured over 500 failing sequences, the first question gave 50 wrong diagnoses and the
second gave none, so that defect is understood and closed, but the next defect in reduction code
will not announce itself. Type-directed reduction also costs roughly 50 lines for every new type,
and the order in which it tries simpler values is knowledge that decays, because somebody can
reorder it and reduction gets worse with nothing going red.

### Go fuzzing

**For it.** It is part of the toolchain and has no dependencies. Its stored cases begin with a
format version line, and its source treats a version it does not recognise as an error rather than
silently skipping the case. It stores the literal input, so when a stored case changes meaning the
change is in project code a reviewer can read. It is under active development.

**Against it.** It cannot generate a struct. Its own source accepts a fixed list of fifteen distinct
types, all numbers, booleans, strings or byte slices, so every value comes from a decoder the
project writes, and operation sequences come from decoding bytes. Under a plain `go test` run it
generates nothing, and under `-fuzz` it takes no seed, so a bounded repeatable run and a generated
run cannot be the same run. It reduces bytes rather than the sequence they decode to, and it does
not reduce a failing case supplied by hand. Its search is reachable from rapid through `MakeFuzz`
without adopting it.

### gopter

**For it.** Its package for operation sequences generates operations conditioned on the current
state, can weight them, and draws them evenly. It reduced a forty-step sequence to the true
four-step minimum in 15 to 32 replays.

**Against it.** It keeps nothing. A failure prints a seed and a case count for a person to paste
back into the test, and its maintainer's answer to a request for exactly that capability was to
expose a function taking the sequence by hand, so it will not change. Its ordinary way of composing
generators drops reduction entirely whenever a mapping changes the value's type, with no warning.
Its module declares Go 1.12, which turns off module-graph pruning and pulls 160 modules into the
graph even though only a handful of its own packages compile into a test binary. Its latest tag is
from 2024 and its most recent reduction fix is in no release.

### testing/quick

**For it.** It is in the standard library with no dependencies. On construction it fails loudly, by
panicking on a type with unexported fields, rather than producing an empty value.

**Against it.** Its own documentation says "The testing/quick package is frozen and is not accepting
new features." It does no reduction at all, keeps nothing, and has no concept of an operation
sequence. Getting past its panic means supplying every argument through `Config.Values`, which is
writing every generator by hand with fewer conveniences.

## Consequences

- **Leaving rapid** means replacing the run harness, taking on a reducer, and rewriting each
  generator against another interface. The properties' rules, the three owned pieces and the crash
  harness's checks survive, because they are this project's own code. What each generator
  constructs, and why, survives with them.
- **What would re-argue this.** A measurement against the real generators showing that an
  independent-draw generator misses the releasable or boundary combinations at the chosen case
  count. Conditional generators would then be needed, rapid's reduction of them degrades, and
  writing reduction by hand becomes the stronger choice. Separately, rapid gaining a way to fail a
  run on a missed mix would retire the generator report, and gaining weighted operation sampling
  could retire the operation sampler.
- **What this adds to other components.** [ADR-0071](./0071-static-enforcement-toolchain.md)'s
  linter configuration carries the ban on `rapid.Make` and `rapid.MakeCustom` and the import rule,
  and the gating run carries a `go vet` analyser. The catalogue carries rows for the gating
  invocation's environment, a fail file kept in the repository, the import rule, the ban, the
  analyser's two rules, the generator report and the failing-case store. This record also requires
  generated mail values to carry [ADR-0044](./0044-synthetic-fixtures-marker-text.md)'s marker text,
  which that record requires of fixtures, so the leak searches built on it can recognise generated
  values too.
- **Assumptions about other components.** The checkpoint machinery and the operation log expose
  their state as plain values, as [ADR-0045](./0045-crash-injection-testing.md) already assumes, so
  the harness checks them without reaching inside. Every sensitivity-carrying type is built only
  through its constructor, as [ADR-0042](./0042-implementation-stack.md) requires, which is what
  makes the constructor the generator's only route.
- **What resolving an open question moves.** Fixing the case count at 100 puts the choice of
  generator shape back in question and makes the measurement against real generators necessary
  before generators are written. Finding that the real generators need conditional generation
  re-argues this record. Deciding that the operation sampler runs in the gating run changes how long
  that run takes. Sharing one in-memory model between the two harness targets changes only the
  harness's internals.
