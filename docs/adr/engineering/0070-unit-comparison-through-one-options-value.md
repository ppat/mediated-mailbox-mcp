# 0070. Unit tests compare returned values with go-cmp, through one shared options value

**Status:** Accepted

## Context

[ADR-0040](./0040-pure-core-decisions-as-values.md) makes every pure core return its decision as a
value that a thin shell then enacts. That shape decides what the unit suite spends its time doing.
The commonest operation in it, by a wide margin, is calling a pure function and comparing the
struct it returned against the struct the test says it should have returned.

[ADR-0042](./0042-implementation-stack.md) fixes Go, so the test runner is `go test` and no
candidate displaces it. [ADR-0043](./0043-no-mocking.md) bans mocking in every form, so a
candidate's mocking capability is worth nothing and a candidate that ships one carries a cost.
[ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) imposes no coverage target, so coverage
integration is worth nothing either. What was left to choose on is how a difference between two
returned values is reported.

### What this choice is not

It reads as a choice of assertion library, and the Go field offers several with fluent vocabularies
for writing many small checks. That is the wrong shape for this suite. A test here has one
assertion, which is that the whole returned value equals the whole expected value, and what it
needs from a library is a readable account of how the two differ. The standard library answers that
question with a boolean.

### The requirements

| Requirement | What it demands | Source |
| --- | --- | --- |
| A difference is localised | When two structs differ in one field of a dozen, the report names that field rather than printing both structs | This record's requirement, from [ADR-0040](./0040-pure-core-decisions-as-values.md)'s shape, because this is the operation the suite spends its time on |
| Values with unexported fields can be compared | [ADR-0042](./0042-implementation-stack.md)'s types keep their fields unexported and are built by smart constructors, and tests must still compare them | [ADR-0042](./0042-implementation-stack.md) |
| The expected value stays literal in the test | Nothing about the way assertions are written pulls an author toward deriving the expected value from the code under test | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| Recorded responses are diffed as files | The fixture responses the browser tests use are written to files, regenerated, and compared, with a way to update them deliberately | [ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md) |
| No mock, spy or stub surface | Nothing importable from the choice substitutes behaviour | [ADR-0043](./0043-no-mocking.md) |
| Test-path footprint | What the choice adds to the dependency tree the tests carry | [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s posture applied to the test path, as [ADR-0068](./0068-test-substrate-containers-directly.md) already applies it |
| Long-term fit | Whether the project is still being maintained, and who by | The project outlives any library it picks |

**How these were weighted.** Localising a difference ordered the field, because it is the operation
the suite performs most and the only one the standard library cannot do. Everything else broke ties.
Running under `go test` was checked for every candidate and graded nowhere, because all of them do,
including the one that ships its own command.

## Decision

- **`github.com/google/go-cmp` reports differences, called from ordinary `go test` functions.** No
  assertion vocabulary and no suite runner is taken.
- **Every comparison calls `cmp.Diff` with the shared comparison options**, one `cmp.Options`
  value held in test support, built from a `cmp.AllowUnexported` entry per type whose unexported
  fields may be read.
- **Golden files are about thirty lines written here**, using `cmp.Diff` for the mismatch report
  and a `-update` flag that rewrites the file deliberately. No library is taken for them.

### What the library's ordinary path does that other records forbid

| The ordinary path | What goes wrong | What catches it |
| --- | --- | --- |
| `cmp.Diff` panics on a type with unexported fields unless given `cmp.AllowUnexported` for it | Written at each call site, that permission spreads across the suite and drifts | One `cmp.Options` value, built once and passed everywhere. A type missing from it panics and the panic names the type, so the gap is loud and local |
| The panic message offers `cmpopts.IgnoreUnexported` among its remedies | `cmp.Diff` then returns the empty string for two values differing in every unexported field, so a test asserting equality passes on values that are not equal | `cmpopts.IgnoreUnexported` is banned by `forbidigo`, configured in [ADR-0071](./0071-static-enforcement-toolchain.md) to refuse that identifier. A checked-in file using it, and the ban-proof script demanding the linter report it, are what prove the ban fires, as [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) requires of any ban standing in for a control |
| A type carrying an `Equal(T) bool` method is compared by that method, and `cmp.Diff` stops walking its fields | The report then prints both whole values with nothing marked, which is the outcome this decision exists to avoid. The method also takes precedence over `cmp.AllowUnexported`, so adopting it forecloses the other route | No `Equal` method is added to a type in order to satisfy a test |

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| A difference is localised | `cmp.Diff` marks each differing field with `-` and `+` and collapses the identical ones into a line of the form `... // 5 identical fields` |
| Values with unexported fields can be compared | One `cmp.AllowUnexported` entry per type, inside the shared `cmp.Options` value |
| The expected value stays literal in the test | A table-driven test holds the expected struct as a literal, and passes it to `cmp.Diff` as the first argument |
| Recorded responses are diffed as files | Thirty lines written here, using `cmp.Diff` for the report |
| No mock, spy or stub surface | There is nothing of the kind in the library |
| Test-path footprint | The library declares no dependencies |
| Long-term fit | Maintained inside Google, which can reassign the work, with commits in 2026 |

### What the implementer would otherwise pay to discover

- **Adding an `Equal(T) bool` method to a type silently changes how it is reported.** `cmp.Diff`
  stops walking the fields and prints both values whole. On a struct of two fields this is
  invisible, because printing both values whole is what localisation looks like at that size. On a
  struct of a dozen it is the difference between a report that says which field moved and one that
  does not.
- **A type absent from the shared `cmp.Options` value panics rather than comparing wrongly.**
  That is the intended behaviour and not a defect to route around.

## Alternatives considered

Seven candidates were graded on a four-level scale. **4** means the candidate carries the
requirement natively. **3** means it is carried with bounded discipline or a named gotcha. **2**
means it is carried only against the tool's own grain. **1** means it cannot honestly satisfy it.
The chosen candidate is the first column.

Every cell on the first two rows was produced by running the same deliberately wrong comparison of
a struct of a dozen fields, including a nested slice, against each candidate and reading what it
printed. The remaining rows are read from each project's own module declaration and release
history.

| Requirement | go-cmp | gotest.tools | The standard library alone | testify | alecthomas/assert | ginkgo with gomega | matryer/is |
| --- | --- | --- | --- | --- | --- | --- | --- |
| A difference is localised | 4 | 4 | 2 | 3 | 3 | 3 | 1 |
| Values with unexported fields can be compared | 3 | 3 | 4 | 4 | 4 | 3 | 4 |
| The expected value stays literal | 4 | 4 | 4 | 3 | 4 | 3 | 4 |
| Recorded responses are diffed as files | 1 | 4 | 1 | 1 | 1 | 1 | 1 |
| No mock, spy or stub surface | 4 | 4 | 4 | 2 | 4 | 4 | 4 |
| Test-path footprint | 4 | 3 | 4 | 2 | 4 | 1 | 4 |
| Long-term fit | 4 | 2 | 4 | 4 | 2 | 4 | 1 |

### What the grid shows

**One requirement sorts nobody in the way it looks like it does.** Recorded responses are diffed as
files separates one candidate from six, and the six are all at 1 because none of them offers the
capability at all. That is a requirement no candidate carries rather than one the field disagrees
on, and the answer is thirty lines written here, measured by writing them. It is graded because
leaving it out of the table would make one candidate's only advantage invisible.

**Comparing values with unexported fields inverts the expected ranking.** The two candidates that
localise a difference best are the two that must be told which types they may read, and the
candidates that need no telling are the ones that report a difference worst. That is not a
coincidence. Reading a struct field by field is what produces both the good report and the need for
permission.

**Where the chosen candidate stands alone.** Nowhere by itself. It shares the ceiling on localising
a difference with one other candidate, and the two are separated by the second table rather than by
the grid.

### What the grid cannot show

| Candidate | What it adds beyond the comparison | Last release | What leaving it costs |
| --- | --- | --- | --- |
| go-cmp | Nothing | 2025, with commits in 2026 | The comparison calls, which appear only in test files |
| gotest.tools | Golden files and an assertion vocabulary, built over go-cmp | 2024, with no commit since | The same calls, plus its golden helper, plus its assertions |

### The reading

**Whose worst cell is the best.** The chosen candidate is the only column with nothing below 3
apart from the requirement no candidate carries. The candidate level with it on the requirement
that orders the field last released in 2024 and has not been committed to since, and it reaches the
comparison this project wants by wrapping the chosen candidate, so taking it means depending on a
dormant project to reach a maintained one, in exchange for thirty lines.

**Which strengths are guarded elsewhere and which guard something nothing else does.** Nothing else
in this project reports which field of a returned value differs. The compiler does not, the type
system does not, and the standard library's comparison answers only whether two values are equal.
That is the one property the choice buys and it is the reason a dependency is taken at all.
Everything else these candidates offer is either forbidden by another record or available without
them.

**What regretting each candidate would cost.** Leaving the chosen one means replacing calls that
appear only in test files, with the standard library's comparison plus a hand-written report as the
fallback, which is exactly where the candidate that takes no dependency already sits. Leaving the
behaviour-driven suite would cost more, because its structure reaches into how every test file is
organised rather than into what one line of it calls. The rest sit between those two.

**Which strengths could be had without choosing the candidate, and which costs could be confined.**
The golden-file capability that separates the runner-up is detachable, and detaching it is what
this decision does. Its assertion vocabulary is detachable too, in the sense that nothing here
wants it. The chosen candidate's strength is not detachable, because reporting which field differs
is the whole of what it does. Its cost, the permission its options value carries, is confined to
one file.

### The candidates

**go-cmp.** The case for it is that it adds one library with no dependencies of its own to buy the
one thing the standard library cannot do, and buys nothing else. The case against it is the
permission ceremony. Every type whose fields are unexported needs a `cmp.AllowUnexported` entry
before it can be compared, and this project's types are like that by design, so the shared
`cmp.Options` value is a piece of test infrastructure every comparison depends on. A test that
bypasses it does not fail quietly. It panics, and the panic names the type it could not read.

**gotest.tools.** The strongest rival. It matches the chosen candidate on the requirement that
orders the field, and it also supplies golden files, which would remove the thirty lines written
here. Its last release and its last commit are both from 2024. It reaches its comparison by wrapping
the chosen candidate, so the trade is a dormant dependency in exchange for thirty lines of code
whose behaviour this project controls.

**The standard library alone.** Genuinely viable, and correctly the starting position. Its case is
no dependency and no decision. It fails on the requirement that orders the field rather than on
verbosity, which is worth stating because verbosity is the usual complaint. Its comparison returns
a boolean, so a report of which field differs has to be written by hand for every struct compared,
and this suite compares constantly.

**testify.** The most widely used option in the language, with the largest body of examples. It is
rejected on three counts and the mock package is the least of them. Its assertion package pulls in
a document-parsing library unconditionally, which is unavoidable rather than opt-out. Its style
pulls an author toward many small assertions where this suite wants one comparison against a
literal. And its familiarity pulls toward two capabilities other records forbid, its mocks and
coverage-shaped habits.

**alecthomas/assert.** A small dependency that compares values with unexported fields without being
told and without stopping the test. That is a different trade rather than a better one, because
what it removes is the signal that a type has not been declared comparable. Its last tagged release
is from 2023, so a consumer does not receive its recent work.

**ginkgo with gomega.** The candidate with a different shape, and its case was taken seriously. A
suite organised around named behaviours is a reasonable fit for a project whose tests are organised
around controls. It runs under `go test` without a separate command, which was checked rather than
assumed. It is rejected on footprint, which is the largest in the field by a wide margin, and
because the structure it offers is structure [ADR-0040](./0040-pure-core-decisions-as-values.md)
already supplies, and because its own parallelism is a second mechanism sitting beside the one `go
test` provides, which complicates sharing one database container.

**matryer/is.** Minimal by design and honest about it. It prints both values whole with no
indication of what differs, which fails the requirement that orders the field. Its last commit is
from 2023.

### One candidate rejected on a rule rather than a grade

`autogold` was considered for the golden-file requirement and is excluded on what it does rather
than on how well it does it. Its convenience is that it rewrites the test's expected value from the
output of the code under test. That is the construction
[ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) names and refuses, a test whose expected
value is read from the code under test so that breaking the code breaks the expectation with it. A
tool whose central feature is that construction cannot be used here whatever else it offers.

## Consequences

- **What leaving this choice would cost.** Calls in test files. The comparison appears nowhere
  else, and the expected values the tests carry are unaffected.
- **What would re-argue this decision.** It rests on pure cores returning decisions as values, which
  is [ADR-0040](./0040-pure-core-decisions-as-values.md)'s. If that stopped holding, comparing whole
  returned structs would stop being the operation the suite spends its time on, and the case for
  taking a dependency at all would weaken with it.
- **A piece of test infrastructure enters the repository**, one `cmp.Options` value carrying a
  `cmp.AllowUnexported` entry for every type whose unexported fields may be read. Nothing enters
  production code, which is the property the rejected equality-method route does not have.
- **A ban enters the repository**, on `cmpopts.IgnoreUnexported`, which reports no difference
  between values that differ. [ADR-0071](./0071-static-enforcement-toolchain.md) configures the
  analyser that carries it.
- **The ban names one route and the hazard has several.** A test can be made to pass on unequal
  values by a path filter that ignores a field, by a comparer that returns true, and by the equality
  method above, none of which is a call to the banned name. The ban covers the route the failure
  message recommends, which is the one an author reaches for. The rest are review discipline, and
  the enumeration is revisited when a new route is found. That the enumeration is incomplete is
  dispositioned as a revisit in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- **Assumptions about other components.** The fixture responses of
  [ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md) are written by a Go test, so
  the golden-file helper written here is what writes and compares them.
