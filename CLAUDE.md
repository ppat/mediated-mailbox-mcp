# CLAUDE.md

Orientation for agents working in this repository, and the home of the code's layout and
conventions. It points at the documents and never repeats them, so every fact has one home.

## The documents

| Read | For |
| --- | --- |
| [USE_CASES.md](./USE_CASES.md) | What the system is for: the outcomes on five axes, each with a falsifiable acceptance criterion |
| [DESIGN.md](./DESIGN.md) | The pillars and invariants — and the **Glossary**, the single home for vocabulary. If a term needs defining, it gets defined there, never locally |
| [docs/UI.md](./docs/UI.md) | The UI's design: the lens model, the zoom ladder, the screens, the palettes, the framework requirements, and the build guidance. Same split test as DESIGN.md, one component |
| [ROADMAP.md](./ROADMAP.md) | All the work in one place: delivery posture, value path, units with their finish lines, production points, dependencies, open decisions. The only top-level document that tracks build state |
| [docs/adr/README.md](./docs/adr/README.md) | The decision records: index, record format, statuses, and the granularity rule (one decision per record, cut by the re-argue test) |
| [TESTING.md](./TESTING.md) | What tests a piece of work must have and what proves it done, linking the records that decide it |
| [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) | Every control's proving injection, past and pending. A new control lands with its injection row |
| [docs/MUTATIONS.md](./docs/MUTATIONS.md) | The mutation ledger. Per-control proof that tests go red when the mechanism is removed, written only from implementation time |
| [.github/ISSUE_TEMPLATE/ticket.md](./.github/ISSUE_TEMPLATE/ticket.md) | The format every ticket is cut from. The rules for tickets are under [Repository process](#repository-process) |

Design questions resolve there, in that order: outcome → pillar/glossary → decision record.

## Keep the documents current — a standing duty

When something new comes up during ANY work — a decision made in conversation, a fact learned
while implementing, a plan change, a new term, a discovered risk — updating the relevant document
is part of that work, not a follow-up. Nothing about the system lives only in chat, code, or
commit messages. Use the **`update-docs` skill** to do it: it routes content to the right
document, carries the authoring procedures, and ends with a whole-set coherence check.

The binding conventions per document live in `.claude/rules/` and load automatically when the
matching file is read; the format authorities are the documents' own preambles and the decision-
record index.

## The target form for prose

These rules bind all documentation, code comments, any other artifact containing prose, and
commentary output to the user.

- Crystal clear and understandable to a cold reader, human or LLM.
- Plain English, or technical English from industry-standard or open-source vernacular. No
  invented terminology. The project's own established terminology, explicitly defined in
  DESIGN.md's Glossary, is exempt.
- Referencing common or widely understood technical or OSS concepts and constructs is fine when
  it aids understanding.
- No stating the obvious, and no restating non-novel concepts, such as explaining how a
  well-known application, platform, or tool works.
- No shorthand. Claude in particular tends to compress an idea into an invented term to save
  tokens. That compression is banned.
- Brevity is valued, but never at the expense of fidelity:
  - Fidelity loss is unacceptable.
  - Compressing into shorthand is not the way to brevity, as stated above.
  - Reach brevity by reducing filler (dropping anything not needed to convey the idea), by using
    widely understood concepts and idioms from general English or from industry-standard or
    OSS-community technical English where they genuinely add value, by avoiding rambling, walls
    of text, and stream-of-consciousness output (anything that interrupts the document's flow or
    sits outside its narrative), and by using structure to your advantage.
  - Beyond that, do not overshoot toward brevity. Overshooting ends in compression that loses
    fidelity.
- Never try to sound smart or convey the writer's ingenuity to the reader. That is a HARD NO, an
  anti-pattern to avoid always.
- Do not write in the standard corporate drone register Claude defaults to. Just as LLM
  attention wanes over a long context window, human attention wanes too, and for a human it
  wanes even in a short context the moment the drone register appears. Drone prose goes in one
  ear and out the other without any information registering.
- No adjectives unless the adjective has merit within its sentence (i.e. losing it would inhibit
  fidelity). The same holds for adverbs, a little less strictly, though still stricter than
  Claude's default.
- No em dashes, colons, or semicolons within a sentence or a phrase. The ban there is total. The
  marks are allowed where they separate a bullet header from bullet content, and within headings
  as long as the headings stay short.

## Code layout and conventions

How the code is laid out and the conventions every component follows. The decisions behind them that
had alternatives live in the records, cited by number. What tests a piece of work needs is
[TESTING.md](./TESTING.md)'s, which controls exist and how each is proven is
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md)'s, and build state is [ROADMAP.md](./ROADMAP.md)'s.
The UI's own layout is [docs/UI.md section 18](./docs/UI.md#18-repository-and-build-layout). The
data-access library and the three narrow shared libraries each describe themselves in a README in
their own directory.

### The Go module

- **One Go module at the repository root.** Every Go component is a directory of that module,
  because the commands that cover the whole repository need one module. `./...` does not reach
  across the modules of a workspace, `go mod tidy` in a workspace member ignores the workspace, and
  an image built from a deployable's directory plus the shared libraries
  ([ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md)) cannot resolve the other
  modules a workspace lists. One module is also linted from one configuration
  ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md)).
- **The module ignores `ui/browser/node_modules` and `ui/browser/codegen/node_modules`**, so Go
  files that a browser package happens to ship are never built, tested or linted as part of this
  module.
- **The top level is flat**
  ([ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md)). Nothing
  adds a grouping directory at the top level.

### Components

Each row is one top-level directory. Published names follow ADR-0054's convention, and a directory
carries the bare word.

| Directory | Kind | Published as | Holds |
| --- | --- | --- | --- |
| `core/` | Library | `mediated-mailbox-core` | The shared pure library ([ADR-0050](./docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), with one subsection per pure concern needed by more than one deployable. They are `sensitivity`, `classify`, `redact`, `authorize`, `scan`, `scangate`, `plan`, `policy`, `mail` (the canonical model, the Provider Port interface and the rate profile types), and `marker` (the marker text of [ADR-0044](./docs/adr/engineering/0044-synthetic-fixtures-marker-text.md)) |
| `db/` | Library | `mediated-mailbox-db` | The data-access library ([ADR-0047](./docs/adr/data/0047-schema-first-data-access.md)), laid out in [db/README.md](./db/README.md) |
| `provider/` | Library | `mediated-mailbox-provider` | The provider adapters and their rate profiles, the provider fake, and the contract suite, argued in [provider/README.md](./provider/README.md) |
| `ratelimit/` | Library | `mediated-mailbox-ratelimit` | The rate limiter, argued in [ratelimit/README.md](./ratelimit/README.md) |
| `testsupport/` | Library | `mediated-mailbox-testsupport` | The shared test tooling, argued in [testsupport/README.md](./testsupport/README.md) |
| `mediate/` | Deployable | `mediated-mailbox-mediate` | The mediator |
| `backfill/` | Deployable | `mediated-mailbox-backfill` | The Backfill Job |
| `sync/` | Deployable | `mediated-mailbox-sync` | Delta Sync |
| `organize/` | Deployable | `mediated-mailbox-organize` | The Reorg Engine's apply and rollback workload |
| `propose/` | Deployable | `mediated-mailbox-propose` | The Heuristics Job |
| `ui/` | Deployable | `mediated-mailbox-ui` | The UI, both halves ([docs/UI.md](./docs/UI.md#18-repository-and-build-layout)) |
| `migrate/` | Image | `mediated-mailbox-migrate` | The migration step's image, which is not a deployable and holds none of this project's Go code ([ADR-0048](./docs/adr/data/0048-forward-only-migrations.md), [ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md)) |
| `packaging/chart/` | Packaging | `mediated-mailbox` | The Helm chart ([ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)) |
| `tests/chainsaw/` | System tests | | The chainsaw suite ([ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)) |

A deployable's job word is a verb for what it does, following `organize`. `ui` keeps the directory
the Glossary gives it. Shared code is the shared pure library, or a narrow, named library that
argues its own case as [ADR-0050](./docs/adr/engineering/0050-shared-code-pure-or-narrow.md)
requires. The data-access library's case is ADR-0047's, and each of the other three argues its case
in its README.

### Inside a component

| Convention | Rule |
| --- | --- |
| Composition root | A deployable's `main.go` at its directory root is its one hand-written composition root ([ADR-0040](./docs/adr/engineering/0040-pure-core-decisions-as-values.md)). Nothing else in the deployable is `package main` |
| Private code | Everything else a deployable holds sits under `<deployable>/internal/`, so the compiler refuses an import from another component as well as the import rules of [ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md). The one exception is an `importtarget` package holding a violation file, described under [Tests](#tests) |
| Pure core | Pure-core code sits under a directory named `core`. That is the top-level `core/`, `<deployable>/internal/core/<concern>/`, and `<library>/core/` inside a library that holds pure rules of its own. The word means the Glossary's pure core wherever it appears, so the core import check matches every one of them by path, anchored at the repository root |
| Shell packages | Named for what they do, for example `api`, `mcp`, `service`, `lease`, `gmail`. No package is named `util`, `common` or `helpers` |
| Library subsections | A library is organized in subsections by concern, each a package, granular enough that an import list can name exactly the subsections a component may use, as every component's list does for the data-access library |
| Mediator layering | `mediate/internal/api` and `mediate/internal/mcp` are the two protocol roots, `mediate/internal/service` is the one service layer beneath both, and enforcement sits below it ([ADR-0030](./docs/adr/operability/0030-api-core-mcp-thin-adapter.md)). Import rules per root refuse a root reaching below the service layer |
| Generated code | Stays inside the component that generates it. Data access sits under `db/<subsection>/`, the UI's contract document at `ui/contract/`, and the browser types under `ui/browser/src/generated/` |
| Designed directories | A directory the design names as a Go package states its role in its package comment, which sits in a `doc.go` while the package holds no other Go file. Any other designed directory that holds no file yet holds a `.gitkeep` |

### Tests

| Kind | Where and how |
| --- | --- |
| Unit tests | `_test.go` files beside the code, in the external `<package>_test` package unless a test needs unexported access and is not a generator. A shell test against the provider fake, which [TESTING.md](./TESTING.md) counts as an integration test, needs no database, so it is an ordinary test file without the `integration` build tag |
| Property tests | Files named `*_property_test.go`, in external test packages ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)). The property-testing library is admitted only in these files, crash-sequence files, and the `testsupport` packages that need it |
| Crash sequences | Files named `*_crash_test.go` in the component whose machinery they target ([ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)). A crash-sequence file whose reduced sequence replays against PostgreSQL also carries the `integration` build tag |
| Integration tests against PostgreSQL | Files named `*_integration_test.go` carrying the `integration` build tag, and run only under `go tool pgrun` with `-tags integration`. `pgrun` starts one PostgreSQL container for the run and prepares a template database, and each integration test package creates its own database from it through `testsupport/postgres` ([ADR-0068](./docs/adr/engineering/0068-test-substrate-containers-directly.md)). One test in `testsupport/postgres` runs `pgrun` a second time, on the next port, to prove a run without the tag fails, so with a remote docker daemon both ports need a route. The test image's reference sits in `testsupport/cmd/pgrun/image.go`, pinned by digest and tracked by renovate |
| Violation files | Placed where the ban or import rule they prove applies, and named for it. A Go violation file ends in `_violation.go` for a rule over non-test files, or in `_violation_test.go`, `_violation_property_test.go` or another test-file suffix for a rule over test files. It carries the `banproof` build tag, so the gating lint never loads it, and holds `// want` annotations naming the finding it must produce ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md)). A browser violation file ends in `_violation.ts` or `_violation.tsx`. The gating browser lint and type check skip files with that name, and nothing imports one, so the bundler never reaches it. An SQL violation file ends in `_violation.sql` under `db/check/testdata/violations/`. A few violation files need a target of their own. A deployable's `importtarget` package sits outside `internal/` only so the other deployables' lists have something to refuse, and `residual_violation.go` at the repository root proves the list over Go files outside every component |
| Must-not-compile fixtures | Under `testdata/mustnotcompile/<case>/`, loaded by `testsupport/mustnotcompile` ([ADR-0042](./docs/adr/engineering/0042-implementation-stack.md)) |
| Golden files | Under `testdata/golden/` in the package that owns them, written and compared by the golden-file helper in `testsupport/compare` ([ADR-0070](./docs/adr/engineering/0070-unit-comparison-through-one-options-value.md)) |
| Mutation demonstrations | A patch under `testdata/mutations/` in the package whose mechanism it removes, one patch per way the mechanism is broken, each describing itself in the text before its first diff header, and run by `go tool mutproof` from the repository root ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)). The runner's package comment states the keys that text carries |
| Browser tests | Under `ui/browser/test/`, run by `bun test` with the DOM shim preloaded ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)) |

`testdata/` is always per package. A test runs from its own package's directory and reads
`testdata/` by a relative path.

### Static analysis and formatting

#### Go

- **One `.golangci.yaml` at the root** runs every Go analyser over the whole module
  ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md)), with gofumpt and
  goimports as its formatters.
- **Linters standing in for controls** are `depguard`, `errcheck`, `exhaustive` and `forbidigo`, the
  ones ADR-0071 configures. Nothing may switch their findings off.
- **Ordinary linters** are `bodyclose`, `errorlint`, `gosec`, `govet`, `ineffassign`, `misspell`,
  `revive` with a named rule set, `staticcheck` and `unused`. They form one closed list. A single
  false positive is suppressed at its line as `//nolint:<linter> // <reason>`, naming only linters
  on that list, and a whole category of them by an exclusion rule naming only linters on that list.
  gosec's own suppression comment is switched off and revive's requires a reason, so an ordinary
  suppression has one form.
- **`govulncheck`** runs in CI over the module and on a schedule, because its answer changes when
  its vulnerability database does.
- **The `go vet` analyser** of
  [ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md) lives in
  `testsupport/analysis` with a `unitchecker` program at `testsupport/cmd/vetcheck`, and runs beside
  golangci-lint as `go vet -vettool="$(go tool -n vetcheck)"` once it carries its rules. A job
  running an analyser with no rules would pass while checking nothing, so no job runs it before then
  ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)).
- **`banproof`** is the ban-proof script of
  [ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md), written as a Go
  program at `testsupport/cmd/banproof` and run as `go tool banproof`. It runs the analysers with
  the `banproof` tag and requires every violation file's expected findings and no other finding. It
  also carries the suppression check ADR-0071 requires. It refuses any line directive, in-code
  ignore comment or configuration setting that can reach a linter standing in for a control, refuses
  a directive naming a linter off the ordinary list or giving no reason, and checks the ordinary
  list against the configuration and the violation files. A refused directive is proven by a
  violation file, and a refused configuration setting by a case in the program's own tests.
- **The must-not-compile check** loads each case under a package's `testdata/mustnotcompile/`
  through `testsupport/mustnotcompile` and asserts the exact type error
  ([ADR-0042](./docs/adr/engineering/0042-implementation-stack.md)).

#### Browser

- **oxlint and ast-grep carry the browser's bans**
  ([ADR-0072](./docs/adr/engineering/0072-browser-bans-under-oxlint-and-ast-grep.md)) from
  `ui/browser/.oxlintrc.json`, which also turns on type-aware rules and fails on warnings, and
  `ui/browser/sgconfig.yml` with its rules under `ui/browser/rules/`. oxfmt formats the TypeScript
  from `ui/browser/.oxfmtrc.json`, skipping generated files and violation files.
- **`go tool banproof -browser`** proves each ban against its violation files, holds the closed list
  of ban rules, allows an oxlint directive that names only ordinary rules and gives a reason, allows
  no ast-grep suppression except the one
  [ADR-0063](./docs/adr/engineering/0063-browser-app-is-preact-with-signals.md) sanctions, and
  refuses any configuration that can switch a ban off.

#### Everything else

| Files | Tool |
| --- | --- |
| Statement and migration files | sqlfluff, from the root `.sqlfluff`, with the generator's parameters read as placeholders. The statement constraints are the parse-tree pass's in `db/check` ([db/README.md](./db/README.md)) |
| Dockerfiles | hadolint |
| Commits | gitleaks, over the commits a pull request adds |
| Markdown, YAML, shell, workflows, links, commit messages | The repository's existing hygiene workflow and its reusable jobs |

#### Pre-commit

`.pre-commit-config.yaml` carries the fast, local checks. They are the existing hygiene hooks, and
local hooks for golangci-lint's formatters, oxfmt, sqlfluff, hadolint and gitleaks. Its exclusions
keep the text fixers and formatters off files that must stay byte-for-byte as generated or recorded,
which are generated data-access code, the contract document and browser types, bun lock files,
golden files, mutation patches, recorded browser fixtures, the synthetic mail fixtures and Helm
templates. The oxfmt and sqlfluff hooks also skip violation files. Every tool a local hook runs also
runs in a CI workflow, because the hygiene workflow runs only its own named hooks.

### Images

- **One Dockerfile per image, in its component's directory**, built with the repository root as the
  context. A library holds none, which is how it is told apart
  ([ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md)). A Go
  deployable's image builds a static binary and copies only that binary onto a minimal base
  ([ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md)).
- **`ui/Dockerfile` builds the browser bundle in its first stage.** Its Go stage refuses to build
  when the bundle holds only the checked-in placeholder, because an image embedding the placeholder
  would build cleanly and serve no UI.
- **`migrate/Dockerfile` builds goose from source with the tags that exclude other databases** and
  copies it with the migration chain ([ADR-0067](./docs/adr/data/0067-migration-runner-goose.md)).

### CI workflows

One workflow per concern under `.github/workflows/`. A workflow that runs only for some changes
filters its pull-request trigger by a path allow list of the directories it watches, extended by one
entry per new component
([ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md)). The Go
workflows also watch every Go file, so a Go file in a new directory is linted and tested. Jobs that
need the repository's tools install them from `mise.toml` through
`ppat/homelab-ops-actions/actions/setup-repository-tools`.

| Workflow | Runs on | Does |
| --- | --- | --- |
| `go-lint` | Go code, the lint configuration, tool pins | `golangci-lint config verify`, `golangci-lint run ./...` over the whole module whenever its paths match, and `go tool banproof` |
| `go-test` | Go code, tool pins | Unit tests. A gating property run sets `RAPID_NOFAILFILE=true`, passes no `-short`, and reads `RAPID_CHECKS` and a non-zero `RAPID_SEED` from repository variables, failing once a property or crash test exists without them ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)) |
| `go-integration` | Go code, tool pins | The integration tests under `go tool pgrun` with `-tags integration`, with the same property-run settings |
| `go-vulncheck` | Go code, and a schedule | `govulncheck` |
| `data` | `db/`, its configuration, tool pins | sqlfluff, the generator's diffs, and the `db/check` tests |
| `ui` | `ui/`, the migration chain, the ban-proof program, tool pins | One job, which runs in order the contract, types and descriptor drift checks of [ADR-0065](./docs/adr/engineering/0065-contract-built-from-registry-consumed-as-generated-types.md), `bun build`, the UI's Go build and tests, the type check, the formatting check, browser lint, `go tool banproof -browser`, and `bun test`. The Go tests and the browser tests share the job because the recorded fixtures are regenerated in the job that runs the Go tests they come from ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)) |
| `images` | Any image's directory, or a shared library an image copies | Builds every image through `ppat/homelab-ops-actions/actions/build-docker-image`, without pushing, and requires the UI's image to fail at its guard when the bundle directory holds only the placeholder |
| `dockerfiles` | Any Dockerfile | hadolint, through the hygiene workflows' reusable job |
| `secrets` | Every pull request | gitleaks |
| `chart` | `packaging/`, tool pins | `helm lint` |
| `chainsaw` | `packaging/`, `tests/chainsaw/`, and by hand with a version | The chainsaw suite as ADR-0052 states it, against the images published for the version |
| `deep-tests` | A schedule, by hand, and a change to the workflow itself | Deep property search and crash-sequence exploration, reading its case count from a repository variable, skipped with a stated reason while no property or crash test exists ([ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)) |
| `release` | A release | Builds, pushes and signs every image by digest with keyless signing, sets the chart's `version` and `appVersion` to the release version as it packages the chart, and pushes and signs the chart ([ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md), [ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)). Every release builds every image, including a release cut by documentation alone, because the chart it publishes points at images of that version |
| `lint` | Every pull request | The repository's existing hygiene checks |
| `renovate` | A schedule | Dependency updates |

A job skipped by its own condition counts as passed under a required check, which matters if checks
are ever made required.

### Tools and versions

- **Every tool and dependency is at its latest version** unless a document explicitly states
  otherwise. The exceptions stated today are minimums that latest already meets, and the TypeScript
  copy the browser's type generation needs
  ([ADR-0065](./docs/adr/engineering/0065-contract-built-from-registry-consumed-as-generated-types.md)).
- **The command-line tools this project's own jobs and hooks run are pinned in the root `mise.toml`,
  and `mise.lock` is committed.** The lock stays in the lock-file format the mise version inside
  `setup-repository-tools` reads, because a lock written by a newer mise in a newer format fails
  every CI job. Go library dependencies are pinned in `go.mod`, browser dependencies in
  `ui/browser/package.json` and `ui/browser/codegen/package.json`, and base images in the
  Dockerfiles. The existing hygiene workflow, its reusable jobs and pre-commit's hygiene hooks carry
  their own pins. Renovate tracks every pin, and groups the Go version and the bun version across
  the places each is pinned so they move together.
- **The project's own tooling programs run through `go tool`**, from `tool` directives naming
  packages of this module, which adds no outside dependency. Outside tools with a Go module of their
  own are never `tool` directives in this module, because their requirements would take part in
  version selection and move shared dependencies inside the project's binaries.
- **The editor runs the same tools as CI.** `.vscode/settings.json` and `.vscode/extensions.json`
  make golangci-lint the Go formatter and linter with this repository's configuration, have gopls
  read the `integration` and `banproof` build tags so every Go file is analysed, add the oxc
  extension for the browser, open generated files read-only, and give the workflow, chart and
  chainsaw files their schemas.

## Repository process

- `docs` is a visible release type here — a documentation PR proposes a release when merged.
  Expected, not accidental.
- **Tickets.** A ticket ([Glossary](./DESIGN.md#glossary)) is cut from the body of the template at
  [.github/ISSUE_TEMPLATE/ticket.md](./.github/ISSUE_TEMPLATE/ticket.md), whose header line and
  sections are its format and whose placeholders say what fills each slot. Units are cut for finish
  lines, tickets for parallel work inside a unit, so a ticket takes a slice of the unit's body and
  says what stays with the unit's other tickets. It names the one unit it serves in its header line,
  and it is a sub-issue of [#118](https://github.com/ppat/mediated-mailbox-mcp/issues/118), which
  lists the tickets by unit and is not itself a ticket. A unit recut in ROADMAP.md recuts its open
  tickets. Its title says what lands, in the words the pull request title
  will use without the commit type, followed by the word unit and the unit's identifier in
  parentheses. Its links are full URLs to files on `main`. A discovery is a ticket under the unit
  whose mechanism it concerns, whether or not the unit is delivered, and work no unit covers gets
  its unit in ROADMAP.md first, since build state has no other home and documents change before
  code.
- **A ticket is done** when its pull request has merged with CI green, the unit's acceptance in
  [ROADMAP.md](./ROADMAP.md) is met for the part keyed to what the ticket lands, and every document
  its work touched is updated in that pull request, with its GitHub labels matching its diff. The
  pull request closes the ticket, and the one at which the unit meets its acceptance in ROADMAP.md
  at its finish line also carries the unit's move to the roadmap's delivered register. A closed
  ticket means its work has merged to `main`. Released and deployed are later states, tracked apart
  in ROADMAP.md.
- **GitHub labels.** A ticket carries `unit:<ID>` for the unit it serves and one
  `component:<directory>` per component of the [component table](#components) it touches, with the
  directory spelled as that table's first column spells it without the trailing slash, so
  `packaging/chart` and `tests/chainsaw` keep both segments. The pull request that closes it carries
  the unit label and one component label per component its diff touches, and the ticket's component
  labels are corrected to match when they differ. A change touching no component carries the unit
  label alone. The author applies every GitHub label at creation and brings a pull request's
  component labels back in line with its diff before it merges, correcting the ticket's in the same
  pass. A missing GitHub label is created at first use in its key's color, `unit` in blue `1D76DB`
  and `component` in purple `5319E7`. Whether a workflow takes over producing a pull request's
  component labels from its diff is an [open decision](./ROADMAP.md#open-decisions), settled when
  the commit vocabulary, the closed set of types and scopes a commit header may carry, is derived,
  since that derivation decides whether a scope carries the component. Tickets carry no other GitHub
  label, and no milestone or project.
