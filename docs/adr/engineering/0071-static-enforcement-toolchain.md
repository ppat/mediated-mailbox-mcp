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
[ADR-0070](./0070-unit-comparison-through-one-options-value.md) and
[ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) each ban calls to named functions,
which is not a kind [ADR-0042](./0042-implementation-stack.md) lists and still needs an analyser to
refuse, and ADR-0069 confines a test library to named packages, so this record hosts those too.

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
  `deny` list anywhere.** `depguard` makes a file that two lists match satisfy both, and does not
  check a file that no list matches. The lists are these, and
  [CLAUDE.md](../../../CLAUDE.md#components) names the directories they match.
  - **Non-test files of every pure-core package, in any component.** A pure core may import other
    pure-core packages and a named set of outside packages, and nothing else. An outside package
    joins the set only when its code, and the code of every package it imports, meets all four of
    these conditions. Outside the test are the Go runtime, `unsafe`, the standard library's
    `internal/` packages, and `sync` and `io`, whose package state no caller can see. A choice
    between implementations made from the processor's features is the runtime's too, so `math`
    passes.
    1. No code reads or writes a file, the network, the environment or the operating system.
    2. No code makes a system call.
    3. No package-level state holds what an earlier call computed from its input. A cache fails
       this, because it lets a later call skip the computation. A table or data set whose content
       is fixed passes, whether it is built at initialization or on first use, and so does a pool
       that recycles working memory and holds no result. `regexp` passes on both counts, and a
       pattern compiled once into a package-level value is such a table.
    4. No code has an effect beyond its return values and the values its caller passes in.

    `golang.org/x/net/idna` fails condition 1, because it imports `fmt`, which imports `os`.
    `golang.org/x/net/publicsuffix` fails it too, because it imports `net/http/cookiejar`.

    The list covers the pure-core packages inside deployables and libraries as well as the shared
    pure library, matched by path. Admitting a package is review discipline, recorded as the
    closure row in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), because no tool checks it.
  - **Each deployable.** A deployable may import its own code, the shared pure library, the
    data-access subsections its list names, and the other named libraries of
    [ADR-0050](./0050-shared-code-pure-or-narrow.md) it uses, beside the standard library and the
    outside modules its own code needs, and nothing belonging to another deployable
    ([ADR-0054](./0054-one-repository-flat-layout-naming-convention.md)).
  - **Each shared library.** The data-access library, the provider library, the rate limiter and the
    shared test support each have a list. A component's list names each data-access subsection it
    may use, so the grant check of [ADR-0066](../data/0066-data-access-generated-from-sql.md) can
    test the list against the role's grants, while the lists over code that never ships, and over
    every component's files at once, admit the whole data-access library.
  - **The mediator's two protocol roots**, each admitting the service layer and nothing below it
    ([ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md)).
  - **Non-test code everywhere.** It refuses the shared test support, the provider fake, the
    contract suite, the property-testing library and the comparison library, so none of them reaches
    code that ships.
  - **Test files.** They are matched by lists of their own, because a test imports what its
    package's non-test files may not. A test file may import what its package's non-test files may,
    the standard library's `testing` package, the comparison support of
    [ADR-0070](./0070-unit-comparison-through-one-options-value.md), the provider fake and the
    contract suite of [ADR-0043](./0043-no-mocking.md), the test support an integration test needs
    to reach its database ([ADR-0068](./0068-test-substrate-containers-directly.md)), and the
    outside modules its tests need. The property-testing library
    [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) chooses, ADR-0069's test support
    and the crash harness are admitted only in property-test files, crash-sequence files, and the
    test support that needs them, and `golang.org/x/tools` only in the shared test support, which
    holds the helper that loads a package which must not compile and the `go vet` analyser.
  - **A list over every Go file outside all components**, admitting only the standard library, so a
    directory added outside every other list is still checked.

  No list other than those named above names the property-testing library, so it is refused
  everywhere else with no `deny` key written. That holds only while every Go file in the repository
  is matched by some list, which the list over files outside every component keeps true for such
  files and review keeps true inside the components.
- **A finding of a linter standing in for a control cannot be switched off anywhere.** Those linters
  are `depguard`, `errcheck`, `exhaustive` and `forbidigo`. A violation file proves a ban fires in
  that file, and says nothing about another line where the finding was switched off, so every proof
  stays green while the control stops being one. Under the aggregator a finding can be switched off
  at its line by a directive, by the linter's own ignore comment, or for whole paths by a
  configuration setting. So the ban-proof script refuses every directive the aggregator honours that
  can reach one of those linters, including a bare `//nolint`, one naming every linter, and one
  naming such a linter in any letter case. It refuses `exhaustive`'s own ignore comment and
  `forbidigo`'s permit comment, and every configuration setting that can exclude their findings,
  among them an exclusion rule naming no linter or naming such a linter, path exclusions, presets, a
  generated-file mode other than `disable`, not linting tests, and limits or new-issue modes that
  hide findings. Every other linter the repository runs is an ordinary linter, and the ordinary
  linters form one closed list, which [CLAUDE.md](../../../CLAUDE.md#static-analysis-and-formatting)
  names. A false positive of an ordinary linter may be suppressed by a directive naming only
  ordinary linters and giving its reason, or by an exclusion rule naming only ordinary linters. The
  script checks the list against the configuration and the violation files, so a linter newly
  enabled cannot be suppressed until it is listed. **No analyser carries this rule.** `nolintlint`
  is the tool an implementer would reach for and it does not do this. Its own description is that it
  reports ill-formed or insufficient directives, so a well-formed directive naming a linter and
  carrying an explanation silences a ban while `nolintlint` at its strictest settings reports
  nothing. The aggregator offers no option to stop honouring the directives.
- **Three checks are written here, because no tool offers them.** The suppression check above,
  written to follow the aggregator's own reading of a directive so it refuses exactly what the
  aggregator would honour, and refusing the spellings the aggregator ignores today as well, so a
  later release honouring them cannot admit one silently. A package that must not compile, loaded
  with `golang.org/x/tools/go/packages` through one helper in the shared test support, which
  requires the exact type error rather than the presence of one. And the ban-proof script, which
  runs the analysers and the suppression check against the checked-in files violating each ban and
  each list, requiring each expected finding and no other. The script also runs the `go vet`
  analyser [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) writes for its own
  placement rules, against that analyser's violation files, once the analyser carries those rules.

### What each tool's ordinary path does that other records forbid

| The ordinary path | What goes wrong | What catches it |
| --- | --- | --- |
| `default-signifies-exhaustive: true`, which treats a default branch as proof the switch is complete | [ADR-0042](./0042-implementation-stack.md) requires a deny-defaulting default branch on every verdict switch, so under that setting the check goes silent on precisely the code this project is required to write, while continuing to report on code that does not matter | The setting is written out as `false`. The fixture behind the catalogue's exhaustiveness row carries the mandated default branch, so the row cannot pass with the setting wrong |
| `check-blank` and `check-type-assertions`, which both default to `false` | Discarding an error explicitly is the ordinary way an unchecked error enters code, and it is the case the check skips unless asked | Both settings written out as on, with a checked-in file discarding an error that the ban-proof script requires to be reported |
| A `deny` list of packages a pure core may not import | The list is a guess about the future. Something added to the standard library later is admitted, and nothing says so | `list-mode: strict`, `allow` lists only, and no `deny` key written anywhere in the configuration |
| `//nolint`, a linter's own ignore comment, or a configuration exclusion silencing a finding | A rule standing in for a control stops being a control wherever somebody found it inconvenient | The suppression check, written here because no analyser carries it. Each refused directive and ignore comment is proven by a checked-in violation file the ban-proof script requires the check to report, and each refused configuration setting by a case in the ban-proof script's own tests |

### Anything deliberately left open

Nothing in the import boundary. The boundary between database roles is a list per component naming
its data-access subsections, which [ADR-0066](../data/0066-data-access-generated-from-sql.md)'s
grant check tests against the grants.

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| A standard-library package can be refused | `depguard` treats standard-library packages as addressable, so they can be named in `allow` and refused when absent from it |
| A rule is expressed as what is permitted | `list-mode: strict`, which refuses anything not named in `allow`. Verified by adding `os/exec`, which appears nowhere in the configuration, to a package governed by the core rule and watching it refused |
| A violation fails the build and says where | A non-zero exit, and a message naming the file, the line, the import and the rule it broke |
| A rule cannot be switched off quietly | The suppression check refuses every directive, ignore comment and configuration setting that can reach a linter standing in for a control, anywhere in the repository, and allows an ordinary linter's suppression only in a form naming it. Each refused directive is proven by a violation file, and each refused configuration setting by a case in the ban-proof script's own tests |
| Configuration is checked in and readable | One file holding every rule and every setting |
| Works on the Go version the project builds with | The aggregator is built with the current toolchain and rebuilds the analysers against it |
| Footprint | One command. The analysers are inside it rather than beside it, apart from the `go vet` analyser [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) writes for its own placement rules, which runs beside it |

One analyser here serves no kind [ADR-0042](./0042-implementation-stack.md) names. `forbidigo`
refuses a call to an identifier named in its configuration, and
[ADR-0070](./0070-unit-comparison-through-one-options-value.md) and
[ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) place such bans. A ban proven by a
checked-in violation file needs a message naming the file and the line, which it gives. Configured
with a pattern for `cmpopts.IgnoreUnexported` it reported the call's file and line, stayed silent on
the `cmp.AllowUnexported` call beside it, and with type analysis enabled reported an aliased import
of the same function as well.

### What the implementer would otherwise pay to discover

- **`check-blank` and `check-type-assertions` are both `false` by default.** A configuration that
  enables `errcheck` and stops there does not report `_ = f()`, which is the ordinary way an
  unchecked error enters code.
- **A violation file proving a ban is an ordinary Go file in a package the analysers load, where the
  ban applies.** It carries a `banproof` build tag that only the ban-proof script's run enables, so
  the gating lint stays clean while the script sees each ban fire, and it is named for what it
  proves. A test file proves a list over test files and a non-test file proves a list over non-test
  files, because a file one list matches says nothing about another. A file under `testdata` would
  not do, because the toolchain excludes that directory and the analysers then load nothing, which
  the ban-proof script reports as a ban that did not fire.
- **`depguard`'s allow entries are string prefixes, compared with one sorted neighbour.** An entry
  `os` also admits `os/signal`, so standard-library entries in the pure-core list end in `$`, which
  makes them exact. Each import is compared only with the entry sorted just before it, so an exact
  entry under a shorter entry for the same path stops that shorter entry admitting its other
  subpackages. `$gostd` admits the standard library through one entry per top-level name, and an
  allowed module whose path starts with `go.` sorts between `go` and `go/ast` and hides every `go/`
  package, so a list holding one also lists `go/`. The violation file for each list imports the
  package its sorted neighbour would wrongly admit.
- **`depguard`'s file patterns match absolute paths.** A pattern such as `**/core/**` also matches
  every file once the checkout sits under a parent directory with that name, so every project
  pattern starts with `${base-path}`. The aggregator's cache does not key on the resolved base path,
  so comparing a configuration across two checkouts needs the cache cleared first. `dir/**.go`
  matches files directly in `dir`, and `dir/**/*.go` does not.
- **The aggregator's defaults hide findings.** Files carrying a generated-code header are excluded
  unless `generated` is `disable`, and a second finding on the same line is dropped unless
  `uniq-by-line` is `false`. `golangci-lint config verify` refuses a misspelled key, which the
  aggregator would otherwise ignore.
- **Linting only what a change touched misses findings a change causes elsewhere.** An enum value
  added in one package leaves a switch incomplete in another, and a run limited to new issues, to
  changed files, or to the changed directory reported nothing where a run over the whole module
  reported the switch. So a path filter decides whether the lint job runs, and the job lints the
  whole module.
- **`nolintlint` does not ban a suppression and no setting makes it.** Configured to require an
  explanation, require a specific linter, and report unused directives, it reported nothing against
  a directive that silenced a ban, because that directive was well formed. An implementer reaching
  for it would ship a control that controls nothing with the build staying green.
- **`forbidigo` runs with `analyze-types: true` and `exclude-godoc-examples: false`.** The second
  setting's default skips the bans inside `Example` functions in test files. With it off the ban
  matches nothing at all, not even the direct call. With it on the ban also survives the two obvious
  evasions, an aliased import and the function held in a variable, both of which were reported. The
  checked-in violation file therefore uses the aliased form, because a ban proven against the harder
  case is proven against the easier one.
- **`default-signifies-exhaustive` is the difference between a live check and a silent one** on
  this project's code specifically, because of the deny-defaulting `default` branch
  [ADR-0042](./0042-implementation-stack.md) requires on every verdict switch.
- **`allow`-list checking is transitively sound here only because the list is closed.** `depguard`
  inspects the imports written in each file and does not follow them. That is enough for the
  pure-core rule as written, because a core package may import only outside packages that meet the
  four conditions above together with everything they import, which reach nothing of this
  project's, and other core packages that the same rule governs. Every path therefore runs through
  something checked. It stops being enough the moment the allow list admits a package that fails
  the conditions or a project package the rule does not itself govern, and nothing detects that.

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
| depguard | Nothing. It is inside the aggregator already being taken | A file glob and an allow list per rule | The aggregator's suppression comment, restricted separately |
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
components once and declaring a dependency graph between them rather than repeating file globs. With
a list per component and per library the boundary is a graph of many components, so that advantage
is real, and it is worth reopening for the rules about this project's own packages. It will never be
worth reopening for the pure-core rule, because the standard-library gap is a property of how that
tool classifies imports rather than a setting.

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
since March 2025. Under the aggregator its findings are suppressible at the line, which is the whole
reason this record refuses the suppression comment wherever it could reach a control and writes a
check to enforce that. And its rules are file globs repeated per rule rather than named components,
so a boundary that grows into a graph of many components repeats itself in configuration where a
purpose-built tool would not.

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
accumulation. This change already writes the suppression check, a compile-failure assertion and the
ban-proof script, and each owned mechanism is cheap alone while the set of them is a standing
maintenance surface with no upstream. Spending that budget on the one rule an existing tool already
expresses correctly is the wrong trade.

**`gomodguard`.** Named because a reader who finds it would reasonably wonder. It governs which
external modules a project may depend on at all, which is a real question this project may want
answered later. It has no notion of one package in this repository importing another, so it cannot
express any of the import rules here.

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
  bans other records place on a named construction, which is a different job. Those records are
  [ADR-0070](./0070-unit-comparison-through-one-options-value.md) and
  [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md).
- **The allow list carries a property no tool checks.** Adding a package to it that the rule does
  not itself govern makes the pure-core rule unsound without any check failing. That is review
  discipline on the configuration and is dispositioned in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- **The lists now form a graph of many components**, the condition under which the architecture
  linter is worth reopening for the rules about this project's own packages. It is never worth
  reopening for the pure-core rule, for the reason the alternatives give.
- **Assumptions about other components.** Continuous integration can install a pinned command and
  run it over the whole module whenever a change reaches Go code or the configuration. The packages
  a pure core is permitted to import reach nothing of this project's, which is what makes checking
  written imports enough.
