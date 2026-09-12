# 0069. Property tests and generated crash sequences both come from rapid

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[A3](../../../USE_CASES.md#a3--bulk-change-is-reversible),
[O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

[ADR-0055](./0055-property-based-safety-invariants.md) decides when a property-based test exists
and fixes two requirements on whatever library runs them. It leaves the library unchosen.
[ADR-0045](./0045-crash-injection-testing.md) decides that a crash-injection harness is built on
the pattern published in Amazon's S3 ShardStore work, with a synthetic crash operation the
generator can place anywhere in a sequence. It speaks of a generator throughout without saying
where the generator comes from.

Those two gaps are one choice. A library that generates sequences of named operations and reduces
a failure to a small one serves both, and taking different tools for them would mean two
generators and two shrinkers for one project.

### What this choice is not

It reads as a choice of property-testing library, which suggests grading generator expressiveness
and shrinking quality. The requirement that actually orders the field is cruder than that. It is
whether the library keeps a failing input at all. Two of the five candidates keep nothing. A found
failure is a line of output, and reproducing it later depends on a person copying a number out of a
log and pasting it back into the test. That is the step
[ADR-0055](./0055-property-based-safety-invariants.md) rules out when it requires that a failing
input, once found, is stored and replayed on every later run.

### The requirements

| Requirement | What it demands | Source |
| --- | --- | --- |
| Bounded deterministic gating runs | A fixed number of generated cases, and a seed that produces the same run again on another machine | [ADR-0055](./0055-property-based-safety-invariants.md) |
| A failing input is kept and replayed | The library writes the failing case to a file the repository can hold, and replays it on every later run with no human step | [ADR-0055](./0055-property-based-safety-invariants.md) |
| Deep search outside the gate | The same test definitions run longer on a schedule, without a second set of definitions | [ADR-0055](./0055-property-based-safety-invariants.md) |
| An operation alphabet carrying a crash step | Generated sequences of named operations, where a synthetic crash can fall anywhere in the sequence | [ADR-0045](./0045-crash-injection-testing.md) |
| A failing sequence reduces to a small one | A failure of forty operations is reported as the few that cause it | This record's requirement, because [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) requires a red that can be produced again, and an unreduced sequence cannot be acted on |
| Values built through their constructors | Generated values are built by calling exported constructors, never by writing unexported fields through reflection | [ADR-0042](./0042-implementation-stack.md) |
| Test-path footprint | What the choice adds to the dependency tree the tests carry | [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s posture applied to the test path, as [ADR-0068](./0068-test-substrate-containers-directly.md) already applies it |
| Runs under `go test` | No second runner, no separate binary in the gate, and the ordinary test flags behave normally | [ADR-0042](./0042-implementation-stack.md), [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md) |

**How these were weighted.** The first two ordered the field, because
[ADR-0055](./0055-property-based-safety-invariants.md) already fixed them and a candidate failing
either is deciding against an accepted record rather than scoring badly. The crash alphabet and
reduction came next, because [ADR-0045](./0045-crash-injection-testing.md) puts the harness on the
value path and an unreduced failure is not evidence anyone can use. Footprint and construction
through constructors came last among the graded requirements. Maintenance and documentation
quality were looked at and broke no ties, so they are not graded.

## Decision

- **`pgregory.net/rapid` runs the property-based tests and supplies the generated operation
  sequences and their reduction for the crash harness.** One library, because it serves both and
  taking a second would mean owning a second generator and a second shrinker.
- **The harness around those sequences stays this project's own code**, as
  [ADR-0045](./0045-crash-injection-testing.md) decides. The library supplies which operation runs
  when. The crash operation's behaviour, the call into the real recovery path, and the two
  invariant families are written here.
- **Go's own fuzzing is available and is not adopted.** It ships with the toolchain, costs
  nothing, and keeps failing inputs the same way. It generates byte strings rather than values, so
  an operation alphabet needs a decoder written here, and the kept failure is a byte string whose
  meaning lives in that decoder. It is the better instrument when the thing under test takes bytes
  from outside, and nothing in the project needs that today.

### What the library's ordinary path does that other records forbid

| The ordinary path | What goes wrong | What catches it |
| --- | --- | --- |
| The seed comes from `hash/maphash` when neither `-rapid.seed` nor `RAPID_SEED` is set, so it differs per process | A gating run that is not the bounded deterministic run [ADR-0055](./0055-property-based-safety-invariants.md) requires, and a failure that cannot be reproduced from the same command | The gating invocation sets `RAPID_SEED` and `-rapid.checks`, and a test fails when `RAPID_SEED` is unset, so the omission stops the build instead of quietly changing what the gate means |
| A `.fail` file holds a generated bitstream, read back against whatever the generator now produces | Changing a generator silently changes what a kept case means. It may replay the case, or replay a different case that does not fail and is discarded without a line of output. The regression test stops testing what it recorded and still looks green | The kept file is treated as the way a failure is reproduced while it is being fixed. What keeps it is an ordinary example-based test carrying the reduced case written out, which states its own expectation and cannot drift with the generator |
| Reduction replays a sequence many times against whatever it is driving | Against a database this multiplies every reset. Reusing one database identity between replays forces waiting for the previous run's asynchronous work to finish before each one, which is where the cost grows | Reduction runs against an in-memory model of the machinery, and only the reduced sequence is replayed against the real database |

### Anything deliberately left open

**Whether one in-memory model serves both crash-harness targets.** The model that reduction runs
against is either written once and shared by [ADR-0045](./0045-crash-injection-testing.md)'s two
targets, or written once per target. Sharing costs a model general enough for two machineries that
checkpoint differently. Not sharing costs writing two. Nothing else depends on which.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| Bounded deterministic gating runs | `-rapid.checks` fixes the number of generated cases, and `RAPID_SEED` fixes the seed rather than leaving it to the default |
| A failing input is kept and replayed | A `.fail` file written under `testdata/rapid/<test name>/`, found again on later runs without being named on the command line |
| Deep search outside the gate | The same tests, run by the scheduled workflow at a higher case count with no seed fixed |
| An operation alphabet carrying a crash step | `t.Repeat` takes a map of operation names to functions, and the crash operation is one more entry in it. The next name is drawn afresh at every step, so a crash can fall anywhere, twice over, or not at all |
| A failing sequence reduces to a small one | Reduction is automatic and needs no arrangement in the test |
| Values built through their constructors | `rapid.Custom` wraps a function that builds the value, so it calls the smart constructor, and `rapid.OneOf` chooses between them. The library offers no reflection path that writes unexported fields, so there is no way to misuse it |
| Test-path footprint | The library declares no dependencies of its own |
| Runs under `go test` | It is a library called from an ordinary test |

### What the implementer would otherwise pay to discover

- **A `.fail` file is read back against the current generator, silently.** This is the
  behaviour behind the rule above. Three kinds of change were tried, widening a range, adding a
  field, and changing a type. None produced an error. Some replayed the recorded case and some
  replayed a different case that did not fail and was discarded without a line of output.
- **Reduction against a real database is affordable, and the reset strategy is the whole cost.**
  Giving each replay its own schema name needs no wait for the previous replay's asynchronous work.
  Reusing one name does, and that wait dominates everything else. Measured against one machinery,
  the first did seven times as many replays in under half the time.
- **Several faults in one sequence make the machinery under test unpredictable**, and the library
  then reports the failure as not reproducible rather than reducing it to a misleading case. That
  is the right direction, and a harness whose operations can leave unpredictable residue will meet
  it.

## Alternatives considered

Five candidates were graded on a four-level scale. **4** means the candidate carries the
requirement natively. **3** means it is carried with bounded discipline or a named gotcha. **2**
means it is carried only against the tool's own grain. **1** means it cannot honestly satisfy it.
The chosen candidate is first.

Every requirement was exercised by building the same two tests on every candidate, a safety
property over a type with unexported fields and a smart constructor, and a crash sequence over a
checkpointed store. So every cell rests on running the candidate rather than on reading about it,
except the one footprint cell noted under the table.

| Requirement | rapid | Go's own fuzzing | A generator written here | gopter | testing/quick |
| --- | --- | --- | --- | --- | --- |
| Bounded deterministic gating runs | 4 | 2 | 4 | 3 | 3 |
| A failing input is kept and replayed | 4 | 4 | 2 | 1 | 1 |
| Deep search outside the gate | 4 | 4 | 3 | 3 | 3 |
| An operation alphabet carrying a crash step | 4 | 2 | 2 | 3 | 1 |
| A failing sequence reduces to a small one | 4 | 3 | 2 | 3 | 1 |
| Values built through their constructors | 4 | 4 | 4 | 2 | 1 |
| Test-path footprint | 4 | 4 | 4 | 3 | 4 |
| Runs under `go test` | 4 | 4 | 4 | 4 | 4 |

### What the grid shows

**Two requirements separated nobody.** Every candidate runs under `go test`, and every candidate's
footprint is small enough not to matter, so neither ordered anything. `gopter`'s footprint cell is
the one exception worth naming, and it is generous rather than harsh. Its declared dependency graph
lists over two thousand entries, and only four of its packages compile into a test binary. The
alarming number is an artefact of how it declares its module and not a cost anyone pays.

**One requirement split the field and decided it.** Keeping a failing input is where `gopter` and
`testing/quick` score 1. Neither writes anything to disk when a property fails. `gopter` prints a
number that a person must paste back into the test, and its own replay helper takes a hand-supplied
sequence as its argument. Three fresh runs against the same planted fault found three different
failures rather than replaying one. The generator written here scores 2 on the same requirement,
because building it included writing a file holding the seed and the reduced sequence and reloading
it on later runs, which worked. What it has none of for free is that mechanism.

**One requirement looks like it separated nobody and did not.** Building values through their
constructors reads as a formality, because every candidate can be made to do it. `gopter`'s
ergonomic path produces the zero value for a type with unexported fields and raises no error, which
records a 2 for that reason. Under [ADR-0042](./0042-implementation-stack.md) the
zero value of a verdict type is its most restrictive state, so a generator collapsing to it
produces a run where every generated case is already denied and a safety property passes without
having tested anything. `testing/quick` is graded 1 because its reflection path writes unexported
fields directly and stops the test with a runtime failure when handed such a type.

**Where the chosen candidate stands alone.** It is the only column with no cell below the top
grade.

### What the grid cannot show

The two candidates that keep a failing input are level on that requirement and differ in what the
kept thing is.

| Candidate | What a kept failure holds | How an operation sequence is written | What the search is guided by | What a generator change does to a kept failure |
| --- | --- | --- | --- | --- |
| rapid | The generated case, readable a year later as the values the generator produced | Named operations, one map entry each | Nothing. It searches at random within its bounds | Reinterprets it silently |
| Go's own fuzzing | A corpus file under `testdata/fuzz/`, holding a byte string whose meaning is whatever the decoder in this repository says it is | Bytes, turned into operations by a decoder written here | Code coverage, which reaches inputs random search does not | Reinterprets it silently |

### The reading

**Whose worst cell is the best.** `rapid` has no cell below the top grade, so the question of what
its weakest requirement costs does not arise. Among the rest, Go's own fuzzing has the best worst
cell, and its two weak cells are on the two requirements this project actually loads hardest, the
bounded random gating run and the operation alphabet. That matters more than a higher ceiling
elsewhere would, because a candidate that is merely adequate on the requirement an accepted record
already fixed is a candidate that has to be worked around on every test written against it.

**Which strengths are guarded elsewhere and which guard something nothing else does.** Most of what
these candidates offer is a second layer over ground already covered. A seeded loop that runs a
fixed number of cases is a few lines anyone can write, and the generator written here scored the
top grade on it. Running under `go test` is free for all of them. The two properties nothing else
in this project backs up are keeping a failing input and reducing a failing sequence, because if
the library does not do them then this project does, and building them was measured at five
hundred and thirty-nine lines. Those are the two the chosen candidate wins on.

That reading has one qualification the decision carries openly. Keeping a failing input turns out
to be a weaker guard than it first appears, because a kept case is silently reinterpreted when its
generator changes. The rule above answers that by writing the reduced case out as an ordinary test,
which means the guarantee this candidate is chosen for needs one piece of discipline to be durable.

**What regretting each candidate would cost.** Leaving `rapid` means rewriting the calls that build
generated values. It appears only in test files, it brings nothing else with it, and the properties
themselves survive untouched, because [ADR-0055](./0055-property-based-safety-invariants.md) ties
each one to a rule the documents already state and a rule outlives the library that runs it. The
harness's crash behaviour, its call into recovery, and its invariant checks are this project's own
either way. Leaving the generator written here costs more the longer it is kept, because every
property written against it inherits its gaps, and its failure shows up as a test that passes while
checking nothing. Leaving Go's own fuzzing costs the decoder and every kept byte string becoming
unreadable. `gopter` and `testing/quick` are not reachable positions to regret, the first because
it keeps nothing and the second because it stops the test when handed this project's types.

**Which strengths could be had without choosing the candidate, and which costs could be confined.**
`rapid`'s strengths cannot be had without it, and what having them otherwise costs was measured by
building it. Its one real weakness is that one person has written almost all of it, and that
weakness is confined to a dependency that appears in test files and compiles into nothing this
project ships, so it can be held at a fixed version indefinitely while a replacement is written.
Go's own fuzzing has the opposite shape in one respect worth keeping. Its strengths do not have to
be given up by choosing something else, because it is the toolchain and is available whenever a
thing under test takes bytes from outside.

### The candidates

**`rapid`.** The case for it is that it satisfies the requirement an accepted record already fixed,
and satisfies it in the strong form. A fault was planted in a checkpointed store, the library wrote
the failing case to a file, the fault was fixed, the case replayed, the fault was planted again,
and a fresh run given no seed caught it immediately. It costs nothing in dependencies, it offers no
way to build a value except through its constructor, and it turns the crash harness's generator
from a thing to write into one entry in a map. The case against it is that one person has written
almost every commit. Seven other people have contributed, which in practice means one maintainer
and seven single fixes. It is alive, it released in 2026, and its own documentation
promises that tests written against it keep working across releases within a major version. That
promise and the exit cost above are what make the concentration bearable rather than disqualifying,
and it is the kind of risk this project can afford to carry because the dependency reaches nothing
it ships.

**Go's own fuzzing.** The case for it is strong and was taken seriously. It is the toolchain, so
there is nothing to install, nothing to keep patched and nobody to depend on. It keeps failing
inputs the same way and adds a kind of search nothing else here offers, guided by which code the
input reaches. The case against it is what the kept artefact is. It generates byte strings, so an
operation sequence exists only through a decoder written here, and the kept failure is bytes whose
meaning lives in that decoder rather than in the file. The reduced artefact happened to be readable
when this was tried, and that was luck from an alphabet of three operations.

**A generator written for this project.** The case for it is real, which is why it was built rather
than argued about. No dependency, no maintainer to rely on, complete control, and it worked. What
decided against it is what building it taught. Its reduction step asked whether the sequence still
failed rather than whether it still failed the same way, and that produced a wrong diagnosis inside
the exercise itself, on the day it was written. A library's reduction step has had that class of
mistake found and fixed over years. The five hundred and thirty-nine lines are not the cost. The
cost is that every property written afterwards inherits whatever is wrong with them.

**`gopter`.** The case for it is a real stateful-testing package, ten years of existence, and
contributors still fixing its reduction step. It fails on one sentence, which is that nothing is
written to disk when a property fails. Its own replay helper takes a hand-supplied sequence, so its
answer to a found failure is the person with the log. Its last tagged release is from 2024, so
the fixes on its default branch are not in anything an ordinary install would fetch.

**`testing/quick`.** The case for it is that it is in the standard library, with nothing to install
and nothing to version. Its own documentation says it is frozen and not accepting new features. It
cannot express a sequence of operations, it does not reduce a failure, it keeps nothing, and its
path for building a value writes unexported fields, so handed one of this project's types it stops
the test. It disqualifies itself in its own words.

## Consequences

- **What leaving this choice would cost.** Generator calls in test files, and nothing else. The
  properties survive because they state rules the documents hold. The harness's own code survives
  because it was always this project's. Kept failing cases would be lost, which is why the reduced
  case is written out as an ordinary test in any event.
- **What would re-argue this decision.** The assumption it rests on is that keeping and replaying a
  failing input is worth a dependency. If a future release removes that behaviour, or if the rule
  requiring a reduced case to be written out as an ordinary test makes the kept file redundant in
  practice, then the field reduces to candidates this record already grades and the generator
  written here becomes the honest answer at a known price.
- **No crash-injection harness exists to import, and a generator of operation sequences does.**
  [ADR-0045](./0045-crash-injection-testing.md) states the same split, and taking the generator
  leaves the harness itself exactly as that record describes it.
- **What resolving the open question would move.** Deciding whether one in-memory model serves both
  crash-harness targets or each target gets its own changes only how much is written inside the
  harness. Nothing outside it depends on the answer.
- **Files kept under `testdata/rapid/` are test inputs**, so
  [ADR-0044](./0044-synthetic-fixtures-marker-text.md)'s rule that no real mail content enters the
  repository reaches them.
- **Assumptions about other components.** The checkpoint and operation-log machinery expose their
  state as values that can be inspected, which [ADR-0045](./0045-crash-injection-testing.md) already
  commits to, so the harness can check its invariants without reaching inside them. The scheduled
  workflow that [ADR-0055](./0055-property-based-safety-invariants.md) relies on for deep search can
  run the same tests with different flags.
