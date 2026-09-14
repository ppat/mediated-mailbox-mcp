# 0072. The browser's bans run under oxlint, with the signal-position rule on ast-grep

**Status:** Accepted ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

[ADR-0063](./0063-browser-app-is-preact-with-signals.md) and
[ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md) between them place five bans on
the browser layer and name no tool to carry them. The raw-markup escape hatch and the document
interfaces beside it are forbidden everywhere. A type assertion and `any` are forbidden inside a
fixture module. The test runner's own substitution helpers are forbidden everywhere, since the
runner ships them and only a linter keeps them out. And a signal's value must not be read inside a
rendering position, nor a signal reach the properties of a route component. Neither record names a
tool for any of them. Both describe the mechanism by what it must do, which is to forbid a named
construction by its shape.

Four of those five are controls with rows in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), so
they are judged by the standard [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) sets for
a ban, which is that it is proven by a checked-in file violating it.

The layer is a strict-mode TypeScript application bundled by bun, with no JavaScript runtime in
production and no second runtime in the test path ([ADR-0042](./0042-implementation-stack.md),
[ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md)). Its type checker already
refuses type errors, so whatever runs here earns its place on what the type checker does not catch.

### What this choice is not

It reads as a choice between linters, to be settled by comparing which expresses the five bans. It
is not, because the five bans are not the whole job. A strict-mode TypeScript application also
wants the ordinary checks a type checker does not perform, and a tool that expresses bans
beautifully while performing none of those is not a linter for this layer, it is a search tool
being used as one. The real question is which tool does which part, and whether one tool should do
both.

### The requirements

| Requirement | What it demands | Source |
| --- | --- | --- |
| Every ban is expressed without writing a program | A ban is configuration or a declarative rule, not code with control flow that can be wrong in its own right | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md), because a ban is proven by a violation file and a ban that is a program has a second way to be wrong |
| A ban is provable | Running it against a checked-in violation file gives a non-zero exit and a message a script can match, and running it against a legitimate file gives neither | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| A ban cannot go quiet without saying so | A mistake in how a ban is written is reported, rather than leaving the ban configured and silent | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md), which exists because a ban can read correctly and match nothing |
| Suppression is visible | Either no way to silence a finding where it fires, or a way a reviewer can see | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| No second runtime in the test path | Nothing requires a JavaScript runtime beside the one already present | [ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md) |
| Reads TypeScript and JSX | Parses the syntax the layer is written in | [ADR-0063](./0063-browser-app-is-preact-with-signals.md) |
| Catches what the type checker does not | Unused bindings, unreachable code, promises nobody waits for, hooks called conditionally, and the rest of what a linter is for | This record's requirement, because the layer needs an ordinary linter whatever else it needs |
| Footprint | How many packages the choice adds, since every direct dependency is listed with its reason | [ADR-0063](./0063-browser-app-is-preact-with-signals.md)'s roster |
| Configuration is checked in and readable | A reviewer can see every ban and confirm it is live | [ADR-0054](./0054-one-repository-flat-layout-naming-convention.md) |

**How these were weighted.** Catching what the type checker does not, and expressing every ban
without writing a program, ordered the field together, and no single candidate leads on both. That
is the finding that shapes the decision. The rest broke ties.

## Decision

- **`oxlint` is the browser layer's linter.** It performs the ordinary checking the type checker
  does not, and it carries four of the five bans as configuration against its own named rules.
  Covering each ban's spellings takes seven rules rather than four. `no-restricted-properties`
  misses the raw-markup property written as a JSX attribute, which `react/no-danger` reports, `bun
  test` offers its substitution helpers as globals needing no import, which `no-restricted-globals`
  reports, and a `require` of the runner escapes `no-restricted-imports`, which
  `typescript/no-require-imports` reports.
- **The fifth ban is a declarative rule in `ast-grep`.** It forbids a signal's value inside a
  rendering position. The ban's other half, a signal reaching a route component's properties, cannot
  be told apart from an ordinary value by syntax, so the render-counter test of
  [ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md) carries it alone. Two further
  `ast-grep` rules close spellings oxlint cannot see, a spread object carrying the raw-markup
  property and a dynamic import of the test runner. A rule written for TypeScript matches nothing in
  a `.tsx` file, so each of those two is written once for each.
- **A sanctioned read of a contract field named `value` carries the field name on its own line above
  the suppression comment.** [ADR-0063](./0063-browser-app-is-preact-with-signals.md) requires the
  field to be named, because the rule cannot tell a signal from an ordinary field and a later reader
  needs to know which was meant. `ast-grep`'s suppression comment takes a list of rule identifiers
  after its colon, so the field name goes on the line above rather than inside it, where no
  directive parser reads it. That keeps the tool's requirement and the record's requirement from
  having to share one line.
- **The type-aware checks are enabled by installing `oxlint-tsgolint`**, without which
  `no-floating-promises` and `no-misused-promises` sit in the configuration, report nothing, and
  say nothing about why. Those two rules are ordinary linting rather than bans standing in for
  controls, so what keeps the package present is the dependency roster of
  [ADR-0063](./0063-browser-app-is-preact-with-signals.md), whose check fails when the roster and
  the manifest disagree.

### Why the fifth ban moved to a second tool

Three facts, in the order that settles it.

**The ban cannot be expressed in oxlint without writing a program.** It has no
`no-restricted-syntax` rule, and its nearest equivalent, `no-restricted-properties` with the object
omitted, reports a `.value` read outside a rendering position as well as inside.
[ADR-0063](./0063-browser-app-is-preact-with-signals.md) accepts that the rule cannot tell a signal
from an ordinary field named the same way inside a rendering position, which is over-reporting
within the area the rule covers. It does not license a rule covering the whole file.

**So the ban needs an artifact of its own either way**, which is about fifty lines of JavaScript
loaded through oxlint's `jsPlugins` key, or about twenty lines of `ast-grep` rule using its `has`
and `inside` relational fields. Both cost an artifact, so this is a question of which artifact
rather than of whether to add one.

**One of those two artifacts can stop working silently.** Everything about loading a program
through `jsPlugins` is reported loudly, which was checked by breaking it six ways, including a
syntax error, a renamed export, a missing file and a removed configuration key. The exception is
the visitor object that program returns, whose keys nothing validates. Writing
`JSXExpresionContainer` for `JSXExpressionContainer` leaves the rule configured, reports nothing,
and exits zero on a file carrying an obvious violation. The same class of mistake against
`ast-grep`, writing `knd` for `kind`, is refused, and the refusal lists the field names that would
have been valid. That is a difference between an open object and a closed schema rather than a
difference in documentation quality.

**What this does not claim.** The ban-proof script this project already requires would catch the
silent case in continuous integration, because it runs each ban against its violation file and
requires a report. So this is not an unguarded hole being closed. What the second tool buys is that
the failure cannot arise rather than being caught after the fact, a report at the moment the rule
is edited rather than at the gate, and one fewer dependency on `jsPlugins`, which oxlint's own
documentation describes as alpha.

### What each tool's ordinary path does that other records forbid

| The ordinary path | What goes wrong | What catches it |
| --- | --- | --- |
| `no-floating-promises` and `no-misused-promises` are inert unless `oxlint-tsgolint` is installed | Both sit in the configuration, report nothing, and give no indication why. The configuration reads correctly and matches nothing | `oxlint-tsgolint` is a line on the dependency roster, and the roster check fails when the roster and the manifest disagree ([ADR-0063](./0063-browser-app-is-preact-with-signals.md)) |
| A suppression comment or a configuration change silences a ban | A ban standing in for a control stops being one wherever somebody found it inconvenient | The ban-proof script refuses any suppression comment naming a ban rule or no rule, and any configuration that can switch a ban off, including a lowered severity, an override excluding files, ignore patterns, a second configuration file and a changed lint command. It allows an oxlint directive naming only ordinary rules and giving its reason, and no `ast-grep` suppression except the sanctioned `value` read of [ADR-0063](./0063-browser-app-is-preact-with-signals.md). Each refused comment is proven by a violation file, and each refused configuration by a case in the ban-proof script's own tests |
| The list of forbidden document interfaces is a list somebody wrote | A browser shipping a new way to turn a string into markup is not on it, and nothing says so | Nothing catches it. The list is reviewed when a browser the project targets ships such an interface, which is a standing review trigger rather than a check |

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| Every ban is expressed without writing a program | Four through `no-restricted-properties`, `react/no-danger`, `no-restricted-imports`, `no-restricted-globals`, `typescript/no-require-imports`, `no-explicit-any` and `consistent-type-assertions`, and the fifth as an `ast-grep` rule file, with two more `ast-grep` rules for spellings oxlint cannot see |
| A ban is provable | Every ban was run against a violation file and a legitimate file, and each gave a non-zero exit on the first and nothing on the second |
| A ban cannot go quiet without saying so | The fifth ban's schema refuses a mistake in the rule, and the other four are configuration against named rules with nothing to misspell |
| Suppression is visible | A ban cannot be suppressed at all, apart from the sanctioned `value` read, and an ordinary rule only by a directive naming it with its reason, both checked by the ban-proof script |
| No second runtime in the test path | Both tools run without a JavaScript runtime beside the one already present |
| Reads TypeScript and JSX | Both parse the syntax the layer is written in |
| Catches what the type checker does not | Measured on a seeded application, where the type checker reported nothing and the linter reported every seeded defect |
| Footprint | Two entries on the dependency roster, each a small package with a compiled binary |
| Configuration is checked in and readable | One configuration file for the linter and the rule files for `ast-grep` in one directory |

### What the implementer would otherwise pay to discover

- **A misspelled visitor key in a `jsPlugins` program disables the rule in silence.** This is why
  the fifth ban does not live there, and it is worth knowing before writing any other such program.
- **`no-floating-promises` and `no-misused-promises` need `oxlint-tsgolint`.** Without it they are
  inert and nothing says so.
- **None of the five bans needs type information.** Every one is a question about the shape of the
  syntax, which was confirmed by expressing each without giving any candidate a project reference.

## Alternatives considered

Five candidates were graded on the four-level scale used elsewhere in this repository. **4** means
the candidate carries the requirement natively. **3** means it is carried with bounded discipline or
a named gotcha. **2** means it is carried only against the tool's own grain. **1** means it cannot
honestly satisfy it.

Every cell in the first four rows was produced by configuring each candidate and running it against
the same violation files and the same legitimate files. The last three rows are read from each
project's own registry entry and documentation.

| Requirement | oxlint | ast-grep | ESLint | biome | deno lint |
| --- | --- | --- | --- | --- | --- |
| Every ban expressed without writing a program | 3 | 4 | 4 | 3 | 1 |
| A ban is provable | 4 | 4 | 4 | 2 | 4 |
| A ban cannot go quiet without saying so | 3 | 4 | 3 | 3 | 3 |
| Suppression is visible | 3 | 4 | 3 | 4 | 3 |
| No second runtime in the test path | 4 | 4 | 3 | 4 | 4 |
| Reads TypeScript and JSX | 4 | 3 | 4 | 4 | 4 |
| Catches what the type checker does not | 4 | 1 | 4 | 3 | 3 |
| Footprint | 4 | 3 | 1 | 3 | 4 |
| Configuration is checked in and readable | 4 | 3 | 4 | 2 | 3 |

### What the grid shows

**No candidate leads on both requirements that order the field**, and that is why the decision is a
pair rather than one name. The two candidates at the ceiling on expressing every ban without
writing a program are the structural search tool, which performs no ordinary checking at all, and
the widely used linter, which is rejected on footprint below. The candidate that performs ordinary
checking best cannot express one of the five bans as configuration.

**One requirement was expected to separate the field and did not.** Every candidate produced a
stable non-zero exit and a matchable message on a violation file and nothing on a legitimate one,
with a single exception. One candidate gave no signal at all, neither a diagnostic nor an error,
for roughly a dozen attempts at a rule that was wrong, which is what its 2 records.

**Two rows rest on something other than a straight comparison.** The structural search tool's grade
for reading TypeScript reflects that it works on the shape of the syntax and has no notion of types
at all, which costs nothing today because no ban needs one. Its configuration grade reflects a
directory of small rule files rather than one file, which a reviewer reads as easily but which is
not one file.

### What the grid cannot show

The two candidates that could carry the fifth ban are separated by what a mistake in the ban does.

| Candidate | How the ban is written | What a mistake in it does | What adding it costs |
| --- | --- | --- | --- |
| oxlint | About fifty lines of JavaScript loaded through `jsPlugins`, which its own documentation calls alpha | A misspelled visitor key leaves the rule configured and silent, exit zero, nothing printed | Nothing. The tool is already present |
| ast-grep | About twenty lines of rule file using `has` and `inside`, on a closed schema | A misspelled field is refused, and the refusal lists the valid names | One roster entry, one rule file, one more invocation, one more version to track |

### The reading

**Whose worst cell is the best.** Two candidates have nothing below 3, the chosen linter and the
structural search tool, and they have opposite weaknesses. Each is weak exactly where the other is
strong, which is why the answer is a pair.

**Which strengths are guarded elsewhere and which guard something nothing else does.** Ordinary
linting guards ground nothing else covers, since the type checker reported none of the seeded
defects. The fifth ban is different, because
[ADR-0063](./0063-browser-app-is-preact-with-signals.md) already pairs it with a render-counter test
that is exact where the ban is approximate. So that ban is the second of two layers rather than the
only one, which is why an early-stage mechanism carrying it was considered acceptable before this
record settled on the other tool, and why moving it costs little either way.

**What regretting each candidate would cost.** Leaving the chosen linter means rewriting a
configuration file and finding another home for four bans. Leaving the structural search tool means
moving one rule. Leaving the widely used alternative, had it been chosen, would have meant the same
as the first plus removing the largest dependency tree in the field.

**Which strengths could be had without choosing the candidate, and which costs could be confined.**
Every strength the chosen linter has survives adding the second tool beside it. Its one real
weakness, a ban that can stop working without saying so, is confined entirely to the single rule
that caused it, and moving that rule removes the weakness without touching anything else. The
structural search tool used for one rule is its ordinary shape rather than a fragment of it.

### The candidates

**oxlint.** The case for it is that it does the larger half of the job outright. On an application
seeded with fourteen defects a linter exists to catch, where the type checker reported none, it
reported all fourteen, including hooks called conditionally and problems of accessibility in markup,
with no framework-specific extension installed. It is a compiled binary with three packages. The
case against it is `jsPlugins`, which its own documentation describes as alpha, and which does not
validate the visitor object a plugin returns. That is why it carries four bans here rather than
five.

**ast-grep.** The case for it is that its rule file is itself the pattern, so there is no program to
be wrong and no extension mechanism to depend on, and a mistake in a rule is refused with the valid
alternatives listed. The case against it is that it is a structural search tool and performs no
ordinary checking, so it cannot be the layer's only tool. Used for one rule it is doing exactly
what it is for.

**ESLint.** The case for it is the strongest on capability. It expressed all five bans as
configuration with no extension at all, its documentation is the best in the field, and the
description in [ADR-0063](./0063-browser-app-is-preact-with-signals.md) is written in its
vocabulary. It is rejected on what it brings. Roughly ninety packages beyond what the layer already
installs, in a language ecosystem [ADR-0042](./0042-implementation-stack.md) already declined on the
server for its record of supply-chain incidents, entering the part of the repository whose job is
enforcement. A disk-size argument was considered and is not made, because the larger figure includes
the type checker the project installs regardless, and measured separately its additional disk cost
is smaller than the chosen linter's. The package count is the argument and it stands alone.

**biome.** Its case is a compiled binary, its own rule catalogue, and a declarative plugin
mechanism for what the catalogue does not cover. It is rejected on provability. For roughly a dozen
attempts at a rule that was wrong it produced no diagnostic and no error, so a wrong rule and an
empty result were indistinguishable, and reaching a working rule meant reading the tool's own
source to find the names its plugin grammar expects. Its own recommended aid for testing a pattern
reported matches for patterns that were not valid. It reaches a correct rule in the end, which is
why it is not at the floor.

**deno lint.** Its case is a single binary with no package installation at all. Its rule catalogue
has no `no-restricted-*` family at all, confirmed against the full list of rules `deno lint --rules`
reports, so every ban except one needed a hand-written program. That is the outcome this decision is
trying to avoid, arriving on four bans instead of one.

## Consequences

- **What leaving these choices would cost.** A configuration file and three rule files. No browser
  code changes, because a ban constrains what may be written rather than how anything is written.
- **What would re-argue this decision.** The pair exists because no one candidate leads on both
  ordering requirements. If the chosen linter gains a rule that expresses the fifth ban as
  configuration, or if its extension interface leaves early-stage and validates what an extension
  returns, the second tool is no longer earning its place and should be removed.
- **[ADR-0063](./0063-browser-app-is-preact-with-signals.md) describes the bans as rules forbidding
  a named construction by its shape**, which is what both tools here express and is deliberately not
  the vocabulary of any one of them.
- **A second tool joins the browser layer's roster**, with its reason, as
  [ADR-0063](./0063-browser-app-is-preact-with-signals.md) requires of every direct dependency.
- **One ban has no check behind its completeness.** The forbidden document interfaces are a list
  somebody wrote, and no candidate tests whether the list is complete. That is a standing review
  trigger and is dispositioned in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- **Assumptions about other components.** The render-counter test of
  [ADR-0064](./0064-browser-tests-run-under-bun-against-a-dom-shim.md) exists and is exact where the
  fifth ban is approximate, which is what makes that ban the second of two layers, and it is the
  only layer for a signal reaching a route's properties. The ban-proof script of
  [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) can run two tools and match the output
  of each.
