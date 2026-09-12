# 0071. Static enforcement runs under golangci-lint, with import boundaries as a closed allow list

**Status:** Accepted · **Pillar:** [Unsafe states are unconstructable, not merely
untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) · **Serves:**
[C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

[ADR-0042](./0042-implementation-stack.md) commits the project to Go and says plainly that Go
carries the unconstructability pillar only with help. It names the conventions that do the carrying,
and then names three kinds of linter that run in continuous integration as, in its words, deputized
enforcement for what the compiler does not check natively. It names no tool for any of them.
[ADR-0040](./0040-pure-core-decisions-as-values.md) requires that a pure core's import boundary is
checked rather than trusted, and [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md)
requires that one deployable cannot import another's code. Neither names a tool either. Separately,
[ADR-0070](./0070-unit-comparison-through-one-options-value.md) bans a call to one named function,
which is not a kind [ADR-0042](./0042-implementation-stack.md) lists and still needs an analyser to
refuse it, so this record hosts that too.

These are not style checks. Several of them are controls with rows in
[docs/VERIFICATIONS.md](../../VERIFICATIONS.md), so they are judged by the standard
[ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) sets for a ban, which is that it is
proven by a checked-in file violating it, because a ban can read correctly and match nothing.

### What this choice is not

It reads as a question of which linters to enable, which suggests comparing rule catalogues. Two
things make it a different question.

The first is that a linter's shipped default can be less safe than this project's own coding
conventions, and when it is, the check goes quiet on exactly the code the conventions produce. That
is not a matter of catalogue size and it is invisible from a feature list.

The second is that expressing a rule as a list of what is forbidden and expressing it as a list of
what is permitted are not two spellings of one rule. A core that must reach nothing performing
input or output is a claim about everything, so a forbidden list is incomplete the day the language
adds something, and nothing announces that.

### The requirements

| Requirement | What it demands | Source |
| --- | --- | --- |
| A rule is expressed as what is permitted | The configuration states the allowed set, so something nobody enumerated is refused rather than admitted | This record's requirement, from [ADR-0040](./0040-pure-core-decisions-as-values.md)'s core rule being a claim about everything a package reaches |
| A standard-library package can be refused | The rule reaches `os`, `net/http`, `os/exec` and their kind, because those are where reading a file, opening a socket and running a command live | This record's requirement, because [ADR-0040](./0040-pure-core-decisions-as-values.md)'s core rule is almost entirely about them |
| A violation fails the build and says where | A non-zero exit and a message naming file, line and rule, stable enough for a script to assert on | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| A rule cannot be switched off quietly | Either no way to suppress a finding at the place it fires, or suppression that a reviewer can see | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| Configuration is checked in and readable | One file a reviewer reads, rather than flags spread through a pipeline | [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md) |
| Works on the Go version the project builds with | The tool builds and runs against the current toolchain rather than a version behind it | [ADR-0042](./0042-implementation-stack.md) |
| Footprint | What the choice adds to the repository's tool surface | [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s posture applied outside the runtime |

**How these were weighted.** Two requirements ordered the import-boundary question. Refusing a
standard-library package came first, because the pure-core rule is almost entirely about such
packages and a candidate that cannot reach them cannot carry the rule at all. Expressing a rule as
what is permitted came second, because the rule makes a claim about everything and a forbidden list
is a guess about the future. Working on the current Go version ordered the separate question of
whether to run the analysers individually or under one command, and did so unexpectedly. Everything
else broke ties.

## Decision

- **`golangci-lint` runs the analysers, configured by one checked-in file.** Not for convenience.
  The analysers this project needs are small, single-purpose projects that are released rarely, and
  the aggregator rebuilds them against a current toolchain. The exhaustiveness analyser's only
  tagged release pins a dependency that no longer builds on current Go releases, and the same
  analyser works correctly through the aggregator, which pins a newer one.
- **Exhaustiveness over enumerated types is checked by `exhaustive`**, configured with
  `default-signifies-exhaustive: false`. That is its own default, and it is written out anyway,
  because the opposite value silences the check on every switch this project is required to write.
- **Unchecked errors are checked by `errcheck`**, configured with `check-blank: true` and
  `check-type-assertions: true`. Both default to `false`, so an error discarded as `_ = f()` is not
  reported by an otherwise ordinary configuration.
- **Import boundaries are checked by `depguard` with `list-mode: strict`, as `allow` lists with no
  `deny` list anywhere.** A pure core may import a named set of standard-library packages and
  other core packages, and nothing else. What may join that set is the membership test in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), which is review discipline because no tool
  checks it. A deployable may import its own code, the shared pure
  library, and nothing belonging to another deployable.
- **A construction forbidden by name is refused by `forbidigo`**, which takes a pattern per banned
  identifier and reports the file and line of a call to it. It is configured here rather than in
  the record placing a ban, so that a second such ban has somewhere to go.
- **`//nolint` is banned wherever a rule standing in for a control applies**, because under the
  aggregator a finding can otherwise be silenced at the line where it fires. That is the packages
  carrying controls, and also every package a ban configured here reaches, which for
  [ADR-0070](./0070-unit-comparison-through-one-options-value.md)'s ban is any test that compares
  values. **No analyser carries this ban.** `nolintlint` is the tool an implementer would reach
  for and it does not do this. Its own description is that it reports ill-formed or insufficient
  directives, so a well-formed directive naming a linter and carrying an explanation silences a
  ban while `nolintlint` at its strictest settings reports nothing. The aggregator offers no
  option to stop honouring the directives.
- **Three checks are written here, because no tool offers them.** A search for the `//nolint`
  directive over the paths the bullet above names. A package that must not compile, loaded with
  `golang.org/x/tools/go/packages` from inside an ordinary test, which requires type errors to be
  present. And a script that runs the analysers and the `//nolint` search against the checked-in
  files violating each ban, requiring each one to be reported.

### What each tool's ordinary path does that other records forbid

| The ordinary path | What goes wrong | What catches it |
| --- | --- | --- |
| `default-signifies-exhaustive: true`, which treats a default branch as proof the switch is complete | [ADR-0042](./0042-implementation-stack.md) requires a deny-defaulting default branch on every verdict switch, so under that setting the check goes silent on precisely the code this project is required to write, while continuing to report on code that does not matter | The setting is written out as `false`. The fixture behind the catalogue's exhaustiveness row carries the mandated default branch, so the row cannot pass with the setting wrong |
| `check-blank` and `check-type-assertions`, which both default to `false` | Discarding an error explicitly is the ordinary way an unchecked error enters code, and it is the case the check skips unless asked | Both settings written out as on, with a checked-in file discarding an error that the ban-proof script requires to be reported |
| A `deny` list of packages a pure core may not import | The list is a guess about the future. Something added to the standard library later is admitted, and nothing says so | `list-mode: strict`, `allow` lists only, and no `deny` key written anywhere in the configuration |
| `//nolint` silencing one finding at the line where it fires | A rule standing in for a control stops being a control at whichever line somebody found inconvenient | A search for the directive over those paths, written here because no analyser carries it, with a checked-in file using the directive that the ban-proof script requires the search to report |

### Anything deliberately left open

**The third import rule is not configured yet.**
[ADR-0066](../data/0066-data-access-generated-from-sql.md) leaves open where the generated
data-access packages live, and the rule that one component cannot name another role's package cannot
be written until that resolves. Under one option the rule is another allow list of the kind this
record already uses. Under the other the compiler refuses the import and no rule is needed. Nothing
else in this record depends on which.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| A standard-library package can be refused | `depguard` treats standard-library packages as addressable, so they can be named in `allow` and refused when absent from it |
| A rule is expressed as what is permitted | `list-mode: strict`, which refuses anything not named in `allow`. Verified by adding `os/exec`, which appears nowhere in the configuration, to a package governed by the core rule and watching it refused |
| A violation fails the build and says where | A non-zero exit, and a message naming the file, the line, the import and the rule it broke |
| A rule cannot be switched off quietly | A search refuses `//nolint` wherever a rule standing in for a control applies, which is wider than the packages carrying controls, and the search is itself proven by a file that uses the directive |
| Configuration is checked in and readable | One file holding every rule and every setting |
| Works on the Go version the project builds with | The aggregator is built with the current toolchain and rebuilds the analysers against it |
| Footprint | One command. The analysers are inside it rather than beside it |

One analyser here serves no kind [ADR-0042](./0042-implementation-stack.md) names. `forbidigo`
refuses a call to an identifier named in its configuration, and
[ADR-0070](./0070-unit-comparison-through-one-options-value.md) is the first record to place such a
ban. A ban proven by a checked-in violation file needs a message naming the file and the line,
which it gives. Configured with a pattern for `cmpopts.IgnoreUnexported` it reported the call's
file and line, stayed silent on the `cmp.AllowUnexported` call beside it, and with type analysis
enabled reported an aliased import of the same function as well.

### What the implementer would otherwise pay to discover

- **`check-blank` and `check-type-assertions` are both `false` by default.** A configuration that
  enables `errcheck` and stops there does not report `_ = f()`, which is the ordinary way an
  unchecked error enters code.
- **A violation file proving a ban is an ordinary test file in a package the analysers load.** Under
  the configuration above the ban reports in a `_test.go` file as readily as in any other, so the
  file proving it is written where the ban actually applies. A file under `testdata` would not do,
  because the toolchain excludes that directory and the analysers then load nothing, which the
  ban-proof script reports as a ban that did not fire.
- **`nolintlint` does not ban a suppression and no setting makes it.** Configured to require an
  explanation, require a specific linter, and report unused directives, it reported nothing against
  a directive that silenced a ban, because that directive was well formed. An implementer reaching
  for it would ship a control that controls nothing with the build staying green.
- **`forbidigo` runs with `analyze-types: true`.** With it off the ban matches nothing at all, not
  even the direct call. With it on the ban also survives the two obvious evasions, an aliased
  import and the function held in a variable, both of which were reported. The checked-in violation
  file therefore uses the aliased form, because a ban proven against the harder case is proven
  against the easier one.
- **`default-signifies-exhaustive` is the difference between a live check and a silent one** on
  this project's code specifically, because of the deny-defaulting `default` branch
  [ADR-0042](./0042-implementation-stack.md) requires on every verdict switch.
- **`allow`-list checking is transitively sound here only because the list is closed.** `depguard`
  inspects the imports written in each file and does not follow them. That is enough for
  the pure-core rule as written, because a core package may import only standard-library packages
  that reach nothing of this project's and other core packages that the same rule governs. Every
  path therefore runs through something checked. It stops being enough the moment the allow list
  admits a package the rule does not itself govern, and nothing detects that.

## Alternatives considered

### The import boundary, where there was a field

Four candidates, graded on the four-level scale used above. **4** means the candidate carries the
requirement natively. **3** means it is carried with bounded discipline or a named gotcha. **2**
means it is carried only against the tool's own grain. **1** means it cannot honestly satisfy it.

The first two rows were produced by writing both rules against a fixture laid out the way this
repository is laid out, with a violation file and a legitimate file for each rule, and reading what
each candidate reported. The suppression row was produced by attempting to suppress a confirmed
finding five different ways. The row on what a violation reports was read from the output of those
same runs. The remaining three rows, on configuration, on the Go version, and on footprint, were
read from each candidate's own documentation and from installing it.

One column is not a tool. The check written here does not exist, so its cells state what writing it
would have to achieve rather than what was observed, and that is why it carries no cell above the
ceiling anywhere. Its two lower grades are the two things a program this project maintains would
have to earn rather than inherit, which are a message a script can match and remaining correct as
the language moves.

| Requirement | depguard | go-arch-lint | A check written here | gomodguard |
| --- | --- | --- | --- | --- |
| Refuses a standard-library package the rule forbids | 4 | 1 | 4 | 1 |
| Expressed as what is permitted | 4 | 4 | 4 | 3 |
| A rule cannot be switched off quietly | 3 | 4 | 4 | 3 |
| A violation fails the build and says where | 4 | 4 | 3 | 4 |
| Configuration is checked in and readable | 4 | 4 | 3 | 4 |
| Works on the Go version the project builds with | 4 | 4 | 4 | 4 |
| Footprint | 4 | 3 | 4 | 4 |

**One requirement decided it and the rest are commentary.** A pure core's rule is almost entirely
about standard-library packages, because those are where reading a file, opening a socket and
running a command live. One candidate allows every standard-library import unconditionally, before
any permission check runs, which was confirmed by reading the code that does it and by watching a
core package import a networking package without being reported. That is disqualifying rather than
inconvenient, and it is disqualifying for the rule this record cares most about while the same
candidate handles the deployable rule correctly.

**Two candidates were the wrong category and are named so nobody proposes them again.** `importas`
enforces consistent import naming and `grouper` enforces how declarations are grouped. Neither has
any notion of permission. `gomodguard`, graded above, operates on whole external modules rather
than on this repository's own packages, so it answers which outside dependencies the project may
take, which is a real question and not this one.

**Where the chosen candidate stands alone.** Nowhere. It shares the ceiling on the deciding
requirement with the check written here, and loses to both remaining candidates on suppression.

### What the grid cannot show

| Candidate | What it would add to the repository | How a rule is written | What suppression exists |
| --- | --- | --- | --- |
| depguard | Nothing. It is inside the aggregator already being taken | A file glob and an allow list per rule | The aggregator's suppression comment, banned separately |
| go-arch-lint | A second command in the pipeline | Named components with a dependency graph declared once | None at a line. Whole files and directories can be excluded in the shared configuration |
| A check written here | A program this project maintains | Whatever this project writes, over the list of packages a build reaches | None, because none would be written |

### The reading

**Whose worst cell is the best.** The check written here has the best worst cell and is not chosen,
which is the one place in this record where the grid and the decision point different ways. The
reason is in the next paragraph.

**Which strengths are guarded elsewhere and which guard something nothing else does.** The chosen
candidate's advantage is that it is already present, since the aggregator is taken for the
analysers that cannot run standalone on this Go version. Against that, the check written here would
have offered two things nothing else does, which are a walk through the whole set of packages a
build reaches rather than the imports written in one file, and no suppression mechanism at all
because none would exist. The first of those turns out not to be needed, because the allow list is
closed and every path runs through something checked. The second is real and is what this decision
gives up.

**What regretting each candidate would cost.** Leaving the chosen one costs a configuration block.
Leaving the check written here would cost a program and its tests. Leaving the architecture linter
would cost a configuration file and a pipeline step.

**Which strengths could be had without choosing the candidate, and which costs could be confined.**
The architecture linter keeps one advantage that is not detachable from it, which is naming
components once and declaring a dependency graph between them rather than repeating file globs. If
the open third rule lands and turns the boundary into a graph of many components, that advantage
becomes real, and it is worth reopening then for the rules about this project's own packages. It
will never be worth reopening for the pure-core rule, because the standard-library gap is a
property of how that tool classifies imports rather than a setting.

### The candidates

**`depguard`, the pick.** The case for it is that it is the only candidate that can express the rule
this record cares most about. Standard-library packages are addressable to it, so they can be named
in an `allow` list and refused when absent from one, which was verified by adding a package that
appears nowhere in the configuration and watching it refused. It expresses the rule with no `deny`
key written anywhere, so the unbounded enumeration a forbidden list invites is not merely avoided
but unavailable. And it is already present, because the aggregator is taken for analysers that
cannot run standalone on current Go releases, so the marginal cost is a configuration block.

The case against it, which is the part worth not softening. Its allow-list behaviour lives in
`list-mode: strict`, which is not its default, so the denylist shape stays one configuration key
away and a later editor can reach it without the rule changing visibly. Its upstream has been quiet
since March 2025. Under the aggregator its findings are suppressible at the line, which is the
whole reason this record bans the suppression comment and writes a check to enforce that ban. And
its rules are file globs repeated per rule rather than named components, so a boundary that grows
into a graph of many components repeats itself in configuration where a purpose-built tool would
not.

**`go-arch-lint`.** The strongest rival on shape. Allow-list expression is its only mode, so the
denylist verb does not exist to reach for, and it names components once and declares a dependency
graph between them rather than repeating globs. It was released days before this decision. It is
rejected on a single fact that no setting changes: every standard-library import is allowed
unconditionally, before any permission check runs, which was confirmed by reading the code that
does it and by watching a core package import a networking package without being reported. Since
the pure-core rule is almost entirely about standard-library packages, that is disqualifying here
while leaving the tool perfectly good at the rule it does carry.

**A check written here.** Its case is real and it is the reason this was close. It would walk
whatever set the project decided rather than the imports written in one file, and it would have no
suppression mechanism because none would be written. What decides against it is not capability but
accumulation. This change already writes a search for the suppression comment, a compile-failure
assertion and the ban-proof script, and each owned mechanism is cheap alone while the set of them
is a standing maintenance surface with no upstream. Spending that budget on the one rule an
existing tool already expresses correctly is the wrong trade.

**`gomodguard`.** Named because a reader who finds it would reasonably wonder. It governs which
external modules a project may depend on at all, which is a real question this project may want
answered later. It has no notion of one package in this repository importing another, so it cannot
express any of the three rules here.

### The remaining questions, where the field reduced to one fact each

**Whether to run the analysers separately or under one command.** The expectation was that one
command buys consolidated configuration and costs frequent upgrades. What decided it is neither. The
exhaustiveness analyser's only tagged release is from 2023, and the dependency it pins no longer
builds against current Go releases, so installing it directly produces no binary. The same analyser
reports correctly when run through the aggregator, which pins a current version of that dependency
and is itself built with the current toolchain. The import-boundary analyser has the same shape of
problem. So the aggregator is not buying convenience, it is absorbing the staleness of several small
projects, which for a project whose enforcement rests on several of them is the whole product. Its
cost is a frequent release cadence that an update bot will surface, and a configuration format that
has changed across its own major versions.

**Which exhaustiveness analyser.** Two exist and they check different shapes. `exhaustive` checks
switches over enumerated constants, and `go-check-sumtype` checks switches over a closed set of
types declared by an annotation. They are therefore a choice about how a type is declared as much as
which analyser runs. The one that checks enumerated constants treats a default branch as not proving
completeness, and the one that checks closed type sets treats it as proving completeness. Measured
on the same code, the second reported nothing at all on a switch carrying a default branch with a
variant unhandled. Since [ADR-0042](./0042-implementation-stack.md) requires that default branch on
every verdict switch, that setting would silence the check on every switch this project is required
to write. The setting can be changed, and the remedy is one line, but the safer default belongs to
the analyser whose shape this project's types already have.

`go-check-sumtype` checks switches over a closed set of types, and this project declares none, so it
has no target here, which is why [ADR-0042](./0042-implementation-stack.md) names three linter kinds
rather than a fourth covering closed sets of types. It also carries a second hazard worth recording.
The `//sumtype:decl` annotation is what gives it anything to check, and removing that annotation
leaves it reporting nothing rather than complaining, so the analyser would sit idle without saying
so. No record in the set declares a closed set of types or sketches one. The one candidate structure
found while checking, an operation in a reorganization plan where one field belongs to a single kind
of operation, is not a sensitivity-carrying or verdict type, so that record's rules do not reach it,
and its representation is undecided.

**How to assert that a package does not compile.** No tool does this. Two mechanisms were built and
each was taken from failing to passing and back. One runs `go build` against a directory named
`testdata`, which the toolchain excludes from ordinary builds, and requires a non-zero exit. The
other loads the package with `golang.org/x/tools/go/packages` from inside an ordinary test and
requires type errors to be present. Both are about fifteen lines. The second is
chosen because it makes the assertion a test like any other rather than a separate step.

**How to prove a ban fires.** `analysistest`, from `golang.org/x/tools/go/analysis`, asserts
expected diagnostics from comments in a fixture file. It works on any analyser exposed as a library
value, including a third-party one, which is more than expected, but not on a compiled command or on
an analyser embedded inside `golangci-lint`. So the mechanism is a script running the aggregator
over the checked-in violation files and requiring each to be reported. It was built and taken from
failing to passing in both directions.

## Consequences

- **What leaving these choices would cost.** A configuration file, in every case. The rules
  themselves are stated in the records that require them and survive any change of tool. The three
  checks written here are each small enough to rewrite in an afternoon.
- **What would re-argue this decision.** The aggregator's central argument is that it rebuilds
  analysers their own maintainers have not released against a current toolchain. If those projects
  resume releasing, that argument weakens to a consolidated configuration file, which is a much
  smaller claim.
- **[ADR-0042](./0042-implementation-stack.md) names three linter kinds**, and each has a tool
  here. A fourth analyser, `forbidigo`, is configured here too, and it is not one of those kinds.
  Those kinds are the enforcement that record's type conventions need, and `forbidigo` carries
  bans other records place on a named construction, which is a different job.
- **The allow list carries a property no tool checks.** Adding a package to it that the rule does
  not itself govern makes the pure-core rule unsound without any check failing. That is review
  discipline on the configuration and is dispositioned in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- **What resolving the open question would move.** Whether the third import rule is another allow
  list here or is enforced by the compiler, and if the boundary becomes a graph of many components,
  whether the architecture linter is worth reopening for this project's own packages.
- **Assumptions about other components.** Continuous integration can install a pinned command and
  run it over changed areas of the repository. The packages a pure core is permitted to import
  reach nothing of this project's, which is what makes checking written imports enough.
