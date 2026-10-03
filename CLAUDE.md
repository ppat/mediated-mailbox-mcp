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
| [docs/MUTATIONS.md](./docs/MUTATIONS.md) | The mutation ledger. Per-control proof that tests go red when the mechanism is broken, written only from implementation time |
| [.github/ISSUE_TEMPLATE/ticket.md](./.github/ISSUE_TEMPLATE/ticket.md) | The format every ticket is cut from. The rules for tickets are under [Repository process](#repository-process) |

Design questions resolve there, in that order: outcome → pillar/glossary → decision record.

## Keep the documents current — a standing duty

When something new comes up during ANY work — a decision made in conversation, a fact learned
while implementing, a plan change, a new term, a discovered risk — updating the relevant document
is part of that work, not a follow-up. Nothing about the system lives only in chat, code, or
commit messages. Use the **`update-docs` skill** to do it: it routes content to the right
document, carries the authoring procedures, and ends with a whole-set coherence check.

The whole-set coherence check is its own skill, **`coherence-check`**. It keeps the entire
repository internally consistent, documents and implementation alike, and applies to implementation
work as much as to document changes. The `update-docs` and `adversarial-review` skills both run it.

The binding conventions per document live in `.claude/rules/` and load automatically when the
matching file is read; the format authorities are the documents' own preambles and the decision-
record index.

## Code layout and conventions

How the code is laid out and the conventions every component follows. The decisions behind them that
had alternatives live in the records, cited by number. What tests a piece of work needs is
[TESTING.md](./TESTING.md)'s, which controls exist and how each is proven is
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md)'s, and build state is [ROADMAP.md](./ROADMAP.md)'s.
The UI's own layout is [docs/UI.md section 18](./docs/UI.md#18-repository-and-build-layout). The
data-access library and the nine narrow shared libraries each describe themselves in a README in
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
| `accountload/` | Library | `mediated-mailbox-accountload` | The loading of a deployable's accounts, the installation's OAuth clients and their opened credentials into one account snapshot, the compare-and-set write-back of a rotated credential, and delta sync's re-seal, argued in [accountload/README.md](./accountload/README.md) |
| `core/` | Library | `mediated-mailbox-core` | The shared pure library ([ADR-0050](./docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), with one subsection per pure concern needed by more than one deployable. They are `sensitivity`, `classify`, `redact`, `authorize`, `scan`, `scangate`, `index` (the decisions every workload that writes the index makes about a message), `plan`, `policy`, and `mail` (the canonical model, the Provider Port interface, the canonical query, the rate profile types and the hard-cap fraction) |
| `credential/` | Library | `mediated-mailbox-credential` | The sealing and opening of an account's provider credential, the loading of its keys, and the configuration section naming the key files with its validation, in `credential/core`, argued in [credential/README.md](./credential/README.md) |
| `db/` | Library | `mediated-mailbox-db` | The data-access library ([ADR-0047](./docs/adr/data/0047-schema-first-data-access.md)), laid out in [db/README.md](./db/README.md) |
| `dbconnect/` | Library | `mediated-mailbox-dbconnect` | A deployable's database section of its configuration and the connection built from it, argued in [dbconnect/README.md](./dbconnect/README.md) |
| `policyload/` | Library | `mediated-mailbox-policyload` | The loading of the policy tables into one snapshot, and the reload-failure alarm, argued in [policyload/README.md](./policyload/README.md) |
| `provider/` | Library | `mediated-mailbox-provider` | The provider adapters and their rate profiles, Gmail's consent apart from its adapter, the provider fake, the contract suite, and the one-time Gmail consent command, argued in [provider/README.md](./provider/README.md) |
| `ratelimit/` | Library | `mediated-mailbox-ratelimit` | The rate limiter, argued in [ratelimit/README.md](./ratelimit/README.md) |
| `sanitize/` | Library | `mediated-mailbox-sanitize` | The conversion of a body's HTML to clean Markdown, argued in [sanitize/README.md](./sanitize/README.md) |
| `settings/` | Library | `mediated-mailbox-settings` | The layering of defaults, one optional configuration file, environment variables and flags into each deployable's configuration, argued in [settings/README.md](./settings/README.md) |
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
requires. The data-access library's case is ADR-0047's, and each of the other nine argues its case
in its README.

### Inside a component

| Convention | Rule |
| --- | --- |
| Composition root | A deployable's `main.go` at its directory root is its one hand-written composition root ([ADR-0040](./docs/adr/engineering/0040-pure-core-decisions-as-values.md)). Nothing else in the deployable is `package main` |
| Operator commands | A command a person runs by hand, which no deployable runs, sits under its library as `cmd/<name>/` in `package main`. The Gmail consent for the test account's token is `provider/gmail/cmd/consent`, a developer's tool that ships in no image ([ADR-0107](./docs/adr/provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)). `credential/cmd/keygen` writes the key pair that credentials are sealed to, and ships as signed static binaries attached to each release ([ADR-0088](./docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)). The test tooling programs under `testsupport/cmd/` follow the same form |
| Private code | Everything else a deployable holds sits under `<deployable>/internal/`, so the compiler refuses an import from another component as well as the import rules of [ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md). The one exception is an `importtarget` package holding a violation file, described under [Tests](#tests) |
| Pure core | Pure-core code sits under a directory named `core`. That is the top-level `core/`, `<deployable>/internal/core/<concern>/`, and `<library>/core/` inside a library that holds pure rules of its own. The word means the Glossary's pure core wherever it appears, so the core import check matches every one of them by path, anchored at the repository root |
| Shell packages | Named for what they do, for example `api`, `mcp`, `service`, `lease`, `gmail`. No package is named `util`, `common` or `helpers` |
| Library subsections | A library is organized in subsections by concern, each a package, granular enough that an import list can name exactly the subsections a component may use, as every component's list does for the data-access library |
| Mediator layering | `mediate/internal/api` and `mediate/internal/mcp` are the two protocol roots, `mediate/internal/service` is the one service layer beneath both, and enforcement sits below it ([ADR-0030](./docs/adr/operability/0030-api-core-mcp-thin-adapter.md)). Import rules per root refuse a root reaching below the service layer. Each root is generated from the service layer's operation registry by one file, `api.go` and `mcp.go`. Each registry entry declares its effect, and its HTTP method, path shape and MCP annotations are derived from it ([ADR-0087](./docs/adr/operability/0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)). Only the generator file of its root is admitted the library its protocol is served with, `net/http` and the MCP SDK. The MCP SDK is admitted nowhere else in the mediator, and every other package under `mediate/internal/`, a new one included, is admitted no HTTP package, so an operation cannot be registered on the client surface from any of them. The composition root mounts the two roots on an HTTP mux, and what it mounts is left to review ([ADR-0053](./docs/adr/engineering/0053-parity-by-construction.md), [ADR-0086](./docs/adr/engineering/0086-mcp-root-on-the-official-go-sdk.md)) |
| Generated code | Stays inside the component that generates it. Data access sits under `db/<subsection>/`, the UI's contract document at `ui/contract/`, the mediator's at `mediate/contract/`, and the browser types under `ui/browser/src/generated/` |
| Designed directories | A directory the design names as a Go package states its role in its package comment, which sits in a `doc.go` while the package holds no other Go file. Any other designed directory that holds no file yet holds a `.gitkeep` |

### Tests

| Kind | Where and how |
| --- | --- |
| Unit tests | `_test.go` files beside the code, in the external `<package>_test` package unless a test needs unexported access and is not a generator. A shell test against the provider fake, which [TESTING.md](./TESTING.md) counts as an integration test, needs no database, so it is an ordinary test file without the `integration` build tag |
| Property tests | Files named `*_property_test.go`, in external test packages ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)). The property-testing library is admitted only in these files, crash-sequence files, and the `testsupport` packages that need it. A property runs through `property.Check` and its generator report through `property.Report`, both in `testsupport/property`. Each fails a run whose `RAPID_SEED` is unset or zero or whose `RAPID_NOFAILFILE` is not `true`, so a local `go test` sets the values `go-test.yaml` sets |
| Crash sequences | Files named `*_crash_test.go` in the component whose machinery they target ([ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)). A crash-sequence file whose reduced sequence replays against PostgreSQL also carries the `integration` build tag |
| Integration tests against PostgreSQL | Files named `*_integration_test.go` carrying the `integration` build tag, and run only under `go tool pgrun` with `-tags integration`. `pgrun` starts one PostgreSQL container for the run and prepares a template database, and each integration test package creates its own database from it through `testsupport/postgres` ([ADR-0068](./docs/adr/engineering/0068-test-substrate-containers-directly.md)). One test in `testsupport/postgres` runs `pgrun` a second time, on the next port, to prove a run without the tag fails, so with a remote docker daemon both ports need a route. The test image's reference sits in `testsupport/cmd/pgrun/image.go`, pinned by digest and tracked by renovate |
| Contract runs against a real provider | Files named `*_live_test.go` beside the adapter, carrying a build tag of the provider's own, `gmail_live` for Gmail, so no other test run compiles them, while lint and the `go vet` analysers read them. They add their own marked messages and check and report only those ([ADR-0043](./docs/adr/engineering/0043-no-mocking.md)). They run only through `go tool livecontract <provider>`, from `testsupport/cmd/livecontract`, which sets the marker the live test requires. Run any other way, the build tag included, a live test skips and names the command, so no accidental or incidental test invocation reaches a real provider |
| The live agent's exercise harness | `mediate/exercise_integration_test.go`, an integration test serving the mediator as its composition root builds it over the provider fake, one account holding the shared synthetic fixtures with the bank's domain restricted, written to the index as backfill's passes write it. It runs only when `MEDIATED_MAILBOX_EXERCISE` names the address it listens on, so every other run skips it, and the operator, or a session on the operator's instruction, starts it under `go tool pgrun` with `-run` naming it and `-timeout 0` for the manual exercises with a live agent that [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) keys to D3. It prints the account, the bearer token and the roots' addresses, serves plain HTTP until it is interrupted, and then prints every audit row the exercise wrote. Nothing that ships can select the fake |
| Violation files | Placed where the ban or import rule they prove applies, and named for it. A Go violation file ends in `_violation.go` for a rule over non-test files, or in `_violation_test.go`, `_violation_property_test.go` or another test-file suffix for a rule over test files. It carries the `banproof` build tag, so the gating lint never loads it, and holds `// want` annotations naming the finding it must produce ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md)). A browser violation file ends in `_violation.ts` or `_violation.tsx`. The gating browser lint and type check skip files with that name, and nothing imports one, so the bundler never reaches it. An SQL violation file ends in `_violation.sql` under `db/check/testdata/violations/`. A few violation files need a target of their own. A deployable's `importtarget` package sits outside `internal/` only so the other deployables' lists have something to refuse, and `residual_violation.go` at the repository root proves the list over Go files outside every component. `propose/internal/testdata/target`, `propose/internal/_target` and `propose/internal/.target` are what `propose/unlinted_violation.go` imports, one in each kind of directory `./...` skips by its name. They carry no `banproof` tag, because `./...` never lists them and only the violation file imports them, so nothing builds them outside the ban-proof run |
| Must-not-compile fixtures | Under `testdata/mustnotcompile/<case>/`, loaded by `testsupport/mustnotcompile` ([ADR-0042](./docs/adr/engineering/0042-implementation-stack.md)) |
| Golden files | Under `testdata/golden/` in the package that owns them, written and compared by the golden-file helper in `testsupport/compare` ([ADR-0070](./docs/adr/engineering/0070-unit-comparison-through-one-options-value.md)). A recorded file read outside the recording test's package sits where its reader expects it, outside `testdata/golden/`, and is written and compared by the helper's `GoldenAt`. They are the browser's fixtures under `ui/browser/test/fixtures/`, recorded by the UI's server tests ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)), and the contract document under `ui/contract/`. Both forms share one `-update` flag, which `testsupport/compare` registers and no other package does |
| Mutation demonstrations | A patch under `testdata/mutations/` in the package whose control it demonstrates, which is usually the package whose file it edits. A control can rest on another package's code, as the Redaction Gate's proof that nothing reaches it but its parameters rests on the fields of the classifier's types, and a patch breaking it there still sits with the control. There is one patch per way the mechanism is broken, each describing itself in the text before its first diff header, and `go tool mutproof` runs them from the repository root ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)). The runner's package comment states the keys that text carries. A patch whose tests are the browser's sits under `ui/browser/testdata/mutations/` and has the runner run them with `bun test` |
| Browser tests | Under `ui/browser/test/`, run by `bun test` with the DOM shim preloaded ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)). The app reads its clock through its dependencies' `now`, and its components schedule their delays, a region's loading phases, the live clock's tick and the keyboard map's g window, through `timers`. A test moves time by advancing `ManualTimers` and never waits on the machine's clock, which under load fails any test that races a real delay. Nothing `settle` waits on is real I/O or a real frame either. The recorded fetch in `test/fixtures/fetch.ts` reads every recording when a test gives its answers and answers each read from memory within microtasks, and `mount` in `test/render.ts` renders inside `act`, so the first render's effects run before it returns rather than on the DOM shim's next frame. `settle` then turns the task queue once per round inside `act`, which lands every read in flight, and returns after the first round that renders nothing, since only a render queues an effect that could start another read. It fails at a bound on its rounds, because a tree still rendering by then is in a loop. A read a test holds open on purpose stays open. The event stream's transport runs only in a browser and keeps the browser's own timers for its reconnects and polls |

`testdata/` is always per package. A test runs from its own package's directory and reads
`testdata/` by a relative path. The one exception is `propose/internal/testdata/target`, a violation
file's import target described under Violation files.

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
- **The `go vet` analysers** live in `testsupport/analysis` with a `unitchecker` program at
  `testsupport/cmd/vetcheck`, and run beside golangci-lint as `go vet -tags
  integration,gmail_live,devloop -vettool="$(go tool -n vetcheck)"`, with the tags golangci-lint's
  configuration sets, which banproof checks. The placement analyser's two rules refuse a call to
  `property.Report` or `property.Check` reachable from inside a property
  ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)). The globals
  analyser refuses package-level state in a pure core by the four rules
  [ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md) states. The environment
  analyser refuses every standard-library function that returns an environment variable's value by
  a name its caller gives, or the environment as a whole, which is `os.Getenv`, `os.LookupEnv`,
  `os.Environ`, `os.ExpandEnv`, `syscall.Getenv`, `syscall.Environ` and
  `(*exec.Cmd).Environ`. Functions that read fixed platform variables for their own purpose, such as
  `os.UserHomeDir`, `os.TempDir` and `http.ProxyFromEnvironment`, stay allowed. The rule covers every
  package of the module outside `testsupport`, apart from test files and a deployable's `main.go` at
  its directory root, the composition root
  ([ADR-0078](./docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)). It is
  scoped by path, which a `forbidigo` rule could carry only through an exclusion ADR-0071 refuses.
  The txhelper analyser holds every call of a generated data-access subsection's `New` to a
  function literal passed to `db/tx.Run`, built from the transaction of the innermost such literal
  around it, and refuses an assignment to that transaction or taking its address, `WithTx`, `New`
  used as a value and a declared function passed to `tx.Run`
  ([ADR-0047](./docs/adr/data/0047-schema-first-data-access.md)). It holds a literal passed to
  `db/tx.RunBase`, the base-policy transaction, to the same rules, and builds `db/policyrules/base`
  only from such a literal and no other subsection from one
  ([ADR-0112](./docs/adr/data/0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)).
  A subsection is recognised by its
  type, a package under `db/` whose `New` returns its own `Queries`. It covers every package, test
  files included, and exempts the accounts listing, the read of `oauth_clients`, delta sync's
  re-seal of a client secret and the UI's OAuth client setup, listing each client's identity and
  adding, replacing and removing a client, each written as one chained call, which are not
  account-scoped. The one gap it leaves is a query value built
  inside the literal that escapes it and is used after `tx.Run` returns, which its package comment
  states. The routes analyser refuses a route registered on a mux in the UI other than through its
  recording mux, as any use of `(*http.ServeMux).Handle`, `(*http.ServeMux).HandleFunc`,
  `http.Handle`, `http.HandleFunc`, or a method of an interface or type parameter named `Handle` or
  `HandleFunc` with the mux's parameters, in a non-test file under `ui/` outside the recording mux's
  own `Handle` method ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md)).
  Registration through reflection, by an imported package on the default mux, or through an
  interface method one of whose parameter types is a type parameter is left to review. The
  mediator is outside its scope, and what the mediator's composition root mounts is left to review.
  The raw SQL analyser refuses a statement the UI runs other than through the data-access library,
  as any use, on any type, of a method named and typed like one of the driver's methods that run a
  statement or start a batch or pipeline that does, `Query`, `QueryRow`, `Exec`, `Prepare`,
  `ExecParams`, `SendBatch`, `ExecBatch`, `CopyFrom`, `CopyTo` and `StartPipeline`, in a non-test
  file under `ui/`, so the read of a client's sealed secret stays with
  the part its import list admits it to
  ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md),
  [ADR-0081](./docs/adr/operability/0081-credentials-sealed-to-a-public-key.md)). A statement run
  through reflection, written to the wire through the raw connection the lower-level connection
  hands out, or run through a method one of whose parameter types is a type parameter is left to
  review.
  banproof requires each rule from its violation file, where a want annotation names the finding as
  `vetcheck`.
- **`banproof`** is the ban-proof script of
  [ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md), written as a Go
  program at `testsupport/cmd/banproof` and run as `go tool banproof`. It runs the analysers with
  the `banproof` tag and requires every violation file's expected findings and no other finding. It
  also carries the suppression check ADR-0071 requires. It refuses any line directive, in-code
  ignore comment or configuration setting that can reach a linter standing in for a control, refuses
  a directive naming a linter off the ordinary list or giving no reason, and checks the ordinary
  list against the configuration and the violation files. A refused directive is proven by a
  violation file, and a refused configuration setting by a case in the program's own tests. It also
  carries the unlinted check, because no lint reads a package `./...` does not list while a file
  importing it builds it in
  ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md)). Every package in the
  non-test build graph of `./...` is the standard library, a package `./...` lists, or another
  module's package the module cache holds. Anything else is refused at each import of it from a
  listed package, with the finding named as `unlinted`. The check runs in each configuration code
  ships in, `CGO_ENABLED=0` for Linux and macOS on amd64 and arm64 with no build tag, a list the
  program copies from the Dockerfiles and the release workflow by hand. It runs once more with the
  `banproof` tag to prove the violation files. Test files may import such a package. The same
  check refuses, at its package clause, every non-test Go file a run compiles in a package `./...`
  lists that `go list` does not select with the build tags `.golangci.yaml` sets in the environment
  the check runs in, which in CI is the lint job's own. A file whose constraint excludes a tag the
  gating lint and vet runs set, one built only for another operating system or architecture, and
  one built only without cgo are each refused, since no lint reads them. banproof also refuses the
  `go-lint` workflow when its `go vet` step's `-tags` differ from `.golangci.yaml`'s build tags as a
  set, and its own `go vet` run takes the configuration's tags and `banproof`.
- **The must-not-compile check** loads each case under a package's `testdata/mustnotcompile/`
  through `testsupport/mustnotcompile` and asserts the exact type error
  ([ADR-0042](./docs/adr/engineering/0042-implementation-stack.md)). The same package's
  `RequireNoExportedFields` asserts that sensitivity-carrying types expose no field, which a
  fixture written against today's fields cannot. `RequireFields` asserts a verdict type's exact
  fields and `RequireParams` a decision's exact parameters, so neither gains a field or an input
  unnoticed. `RequireMethods` asserts an interface's exact methods and signatures, and
  `RequireReturning` and `RequireResults` which of its methods return a type and exactly what they
  return, as the Provider Port's one body-returning operation needs.

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
| Statement and migration files | sqlfluff, from the root `.sqlfluff`, with the generator's parameters read as placeholders. The placeholder templater renders a parameter with no value as its bare name, and RF02 reports that name as an unqualified column when the statement reads more than one table. A table function such as `unnest` does not count as a second table. A data-access subsection where RF02 would report a parameter this way gives its parameters values in a `.sqlfluff` of its own. Those files set parameter values only, and turn no rule off. The statement constraints are the parse-tree pass's in `db/check` ([db/README.md](./db/README.md)) |
| Dockerfiles | hadolint |
| Commits | gitleaks, over the commits a pull request adds |
| Commit headers and the pull request title | commitlint, from the root `commitlint.config.js`, whose vocabulary and pairing rule are [.claude/rules/commits.md](./.claude/rules/commits.md)'s |
| Markdown, YAML, shell, workflows, links, commit messages | The repository's existing hygiene workflow and its reusable jobs |

#### Pre-commit

`.pre-commit-config.yaml` carries the fast, local checks. They are the existing hygiene hooks, the
commitlint hook at the commit-msg stage, pinned to the version the root `package.json` pins, and
local hooks for golangci-lint's formatters, oxfmt, sqlfluff, hadolint and gitleaks. Its exclusions
keep the text fixers and formatters off files that must stay byte-for-byte as generated, recorded or
vendored, which are generated data-access code, the contract document and browser types, bun lock
files, golden files, mutation patches, recorded browser fixtures, the synthetic mail fixtures, Helm
templates and the vendored font licence. The oxfmt and sqlfluff hooks also skip violation files. Every tool a local hook runs also
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
| `go-lint` | Go code, the lint configuration, tool pins | `go mod tidy -diff`, `golangci-lint config verify`, `golangci-lint run ./...` over the whole module whenever its paths match, the `go vet` analysers, and `go tool banproof` |
| `go-test` | Go code, the chart's alerting rules with the template that ships them, tool pins | Unit tests, which include `promtool test rules` over the alerting rules and a render of the chart that ships them ([ADR-0077](./docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)). The gating property run sets a fixed non-zero `RAPID_SEED`, the gating `RAPID_CHECKS` and `RAPID_NOFAILFILE=true` in the workflow and passes no `-short` ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)) |
| `go-integration` | Go code, the chart's alerting rules, tool pins | The integration tests under `go tool pgrun` with `-tags integration`, with the same property-run settings |
| `gmail-contract` | A weekly schedule on the main branch, and by hand on any branch | `go tool livecontract gmail`, the Gmail adapter's contract suite against the test account, with its credentials from GitHub Actions secrets ([ADR-0043](./docs/adr/engineering/0043-no-mocking.md)). Never automatically on a pull request |
| `go-vulncheck` | Go code, and a schedule | `govulncheck` |
| `data` | `db/`, its configuration, tool pins | sqlfluff, the generator's diffs, and the `db/check` tests |
| `ui` | `ui/`, the migration chain, the data-access subsections the UI's list names, the ban-proof program, tool pins | One job, which runs in order the contract, types and descriptor drift checks of [ADR-0065](./docs/adr/engineering/0065-contract-built-from-registry-consumed-as-generated-types.md), `bun build`, the UI's Go build and its tests under `go tool pgrun` with `-tags integration` and the same property-run settings, which record the browser's fixtures and diff them, the type check, the formatting check, browser lint, `go tool banproof -browser`, and `bun test`. The Go tests and the browser tests share the job because the recorded fixtures are regenerated in the job that runs the Go tests they come from ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)) |
| `images` | Any image's directory, or a shared library an image copies | Builds every image through `ppat/homelab-ops-actions/actions/build-docker-image`, without pushing, and requires the UI's image to fail at its guard when the bundle directory holds only the placeholder |
| `dockerfiles` | Any Dockerfile | hadolint, through the hygiene workflows' reusable job |
| `secrets` | Every pull request | gitleaks |
| `mutation-patches` | Every pull request | `git apply --check` over every mutation patch under a `testdata/mutations/` directory, excluding `testsupport/cmd/mutproof/testdata/`'s own fixtures, after a self-test proves the check refuses a patch whose context no longer matches. Unconditional rather than path-filtered, because a patch's diff context can span any file in the tree, and neither `go-test` nor `go-integration` runs it, so a patch's plain applicability is a standing guard apart from the mutation demonstration itself, which stays event-driven ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)) |
| `chart` | `packaging/`, tool pins | `helm lint` |
| `chainsaw` | `packaging/`, `tests/chainsaw/`, and by hand with a version | The chainsaw suite as ADR-0052 states it, against the images published for the version |
| `deep-tests` | A schedule, by hand, and a change to the workflow itself | Deep property search and crash-sequence exploration at the scheduled case count, which a manual run may override, with a fresh seed each run, skipped with a stated reason while no property or crash test exists ([ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)) |
| `release` | A release | Builds, pushes and signs every image by digest with keyless signing, sets the chart's `version` and `appVersion` to the release version as it packages the chart, pushes and signs the chart, and builds the `credential/cmd/keygen` binaries as `mediated-mailbox-keygen-<os>-<arch>` for linux and darwin on amd64 and arm64, signs each keylessly into a Sigstore bundle, and attaches each binary and its bundle to the release ([ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md), [ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md), [ADR-0088](./docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)). Every release builds every image, including a release cut by documentation alone, because the chart it publishes points at images of that version |
| `lint` | Every pull request | The repository's existing hygiene checks, `commit-messages`, commitlint over the branch commits, and `commit-taxonomy`, which derives every header Renovate and release-please can emit and lints it, requires each to be true of its file, and checks a pull request's headers against its diff ([ADR-0073](./docs/adr/engineering/0073-commit-header-type-sizes-release-scope-names-surface.md)). `commit-taxonomy` carries no path filter, `needs:` or `if:`, because a skipped job satisfies a required check |
| `pr-title` | Every pull request, on open, edit, synchronize and reopen | commitlint over the pull request title, the string that lands on `main` for a multi-commit pull request ([ADR-0073](./docs/adr/engineering/0073-commit-header-type-sizes-release-scope-names-surface.md)). Never gated, for the same reason |
| `pr-labels` | Every pull request, on open, edit, synchronize, reopen, label and unlabel | Sets the pull request's `component:` labels to exactly the components its diff touches, read from the [component table](#components) at the pull request's head, creating a missing label in the component color. A new component needs only its table row. A pull request from a fork cannot write labels, so its run fails |
| `renovate` | A schedule | Dependency updates |

A job skipped by its own condition counts as passed under a required check, which is why the three
checks that gate the commit vocabulary carry no condition.

### Tools and versions

- **Every tool and dependency is at its latest version** unless a document explicitly states
  otherwise. The exceptions stated today are minimums that latest already meets, and the TypeScript
  copy the browser's type generation needs
  ([ADR-0065](./docs/adr/engineering/0065-contract-built-from-registry-consumed-as-generated-types.md)).
- **The command-line tools this project's own jobs and hooks run are pinned in the root `mise.toml`,
  and `mise.lock` is committed.** The lock stays in the lock-file format the mise version inside
  `setup-repository-tools` reads, because a lock written by a newer mise in a newer format fails
  every CI job. Go library dependencies are pinned in `go.mod`, browser dependencies in
  `ui/browser/package.json` and `ui/browser/codegen/package.json`, the commitlint the gates and the
  `commit-taxonomy` check run in the root `package.json` with `bun.lock` committed, and base images
  in the Dockerfiles. The existing hygiene workflow, its reusable jobs and pre-commit's hygiene
  hooks carry their own pins. Renovate tracks every pin, and groups the Go version and the bun
  version across the places each is pinned so they move together. It runs `go mod tidy` after an
  update to a Go dependency, because the update writes the new module's sums and prunes none of the
  ones the module graph stops selecting, and an update that shifts what the graph selects of other
  modules leaves out the sums those modules now need. The `go-lint` workflow fails on that drift
  whatever produced it.
- **The project's own tooling programs run through `go tool`**, from `tool` directives naming
  packages of this module, which adds no outside dependency. Outside tools with a Go module of their
  own are never `tool` directives in this module, because their requirements would take part in
  version selection and move shared dependencies inside the project's binaries.
- **The editor runs the same tools as CI.** `.vscode/settings.json` and `.vscode/extensions.json`
  make golangci-lint the Go formatter and linter with this repository's configuration, have gopls
  read the `integration`, `banproof`, `gmail_live` and `devloop` build tags so every Go file is
  analysed, run tests with the gating run's property settings, add the oxc extension for the
  browser, open generated files read-only, and give the workflow, chart and chainsaw files their
  schemas.

## Repository process

- `docs` is a visible release type here — a documentation PR proposes a release when merged.
  Expected, not accidental. The commit vocabulary, the closed set of types and scopes a commit
  header may carry, the pairing rule between them and what each type does to a release, is
  [.claude/rules/commits.md](./.claude/rules/commits.md)'s, decided by
  [ADR-0073](./docs/adr/engineering/0073-commit-header-type-sizes-release-scope-names-surface.md).
  The `commit-messages`, `pr-title` and `commit-taxonomy` checks of the
  [workflow table](#ci-workflows) gate every pull request on it.
- **Tickets.** A ticket ([Glossary](./DESIGN.md#glossary)) is cut from the body of the template at
  [.github/ISSUE_TEMPLATE/ticket.md](./.github/ISSUE_TEMPLATE/ticket.md), whose header line and
  sections are its format and whose placeholders say what fills each slot. Units are cut for finish
  lines, tickets for parallel work inside a unit, so a ticket takes a slice of the unit's body and
  says what stays with the unit's other tickets. It names the one unit it serves in its header line,
  and it is a sub-issue of [#118](https://github.com/ppat/mediated-mailbox-mcp/issues/118), which
  lists the tickets by unit and is not itself a ticket. A unit recut in ROADMAP.md recuts its open
  tickets. Its title says what lands, in the words the pull request title will use without the
  commit type and scope, followed by the word unit and the unit's identifier in parentheses. Its
  links are full URLs to files on `main`. A discovery is a ticket under the unit whose mechanism it
  concerns, whether or not the unit is delivered, and work no unit covers gets its unit in
  ROADMAP.md first, since build state has no other home and documents change before code. #118's
  body, meaning its rows and each ticket's State, is written only by the control session that
  [.claude/loop.md](./.claude/loop.md) runs. It marks a ticket in-progress when it launches the
  ticket's session, and built only once the ticket's pull request has merged, so no dependent ticket
  starts before its blocker is on `main`. A session that cuts a ticket links it as a sub-issue and
  sends the control session its row. The control session derives every State again on each
  iteration, so a change made while none runs is caught when it next runs.
- **A ticket is done** when its pull request has merged with CI green, the unit's acceptance in
  [ROADMAP.md](./ROADMAP.md) is met for the part keyed to what the ticket lands, and every document
  its work touched is updated in that pull request, with its GitHub labels matching its diff. The
  pull request closes the ticket, and the one at which the unit meets its acceptance in ROADMAP.md
  at its finish line also carries the unit's move to the roadmap's delivered register. A closed
  ticket means its work has merged to `main`. Released and deployed are later states, tracked apart
  in ROADMAP.md.
- **A builder subagent builds a pull request through the `builder` skill,** which holds the brief
  a builder gets, the order it works in, the proof and gates it runs, and the report it hands back.
- **A pull request an agent builds leaves draft** only once the **`adversarial-review` skill**'s
  loop has found nothing that stands, its CI is green, and it merges cleanly onto current `main`.
  The skill holds who reviews, what every review checks and how findings are ruled on.
- **GitHub labels.** A ticket carries `unit:<ID>` for the unit it serves and one
  `component:<directory>` per component of the [component table](#components) it touches, with the
  directory spelled as that table's first column spells it without the trailing slash, so
  `packaging/chart` and `tests/chainsaw` keep both segments. The pull request that closes it carries
  the unit label and one component label per component its diff touches. A change touching no
  component carries the unit label alone. The author applies the ticket's labels and the pull
  request's unit label at creation. The `pr-labels` workflow of the [workflow table](#ci-workflows)
  sets the pull request's component labels from its diff and keeps them in line with it, and the
  author corrects the ticket's component labels to match the pull request's before it merges. A
  missing GitHub label is created at first use in its key's color, `unit` in blue `1D76DB` and
  `component` in purple `5319E7`. The commit scope never carries the component
  ([ADR-0073](./docs/adr/engineering/0073-commit-header-type-sizes-release-scope-names-surface.md)),
  so the component labels are the one channel that does. Tickets carry no other GitHub label, and
  no milestone or project.
