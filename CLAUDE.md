# CLAUDE.md

Orientation for agents working in this repository, and the home of the code's layout and
conventions. It points at the documents and never repeats them, so every fact has one home.

## The documents

| Read | For |
| --- | --- |
| [USE_CASES.md](./USE_CASES.md) | What the system is for: the outcomes on five axes, each with a falsifiable acceptance criterion |
| [DESIGN.md](./DESIGN.md) | The pillars and invariants — and the **Glossary**, the single home for vocabulary. If a term needs defining, it gets defined there, never locally |
| [docs/UI.md](./docs/UI.md) | The UI's design: the lens model, the zoom ladder, the screens, the palettes, the framework requirements, and the build guidance. Same split test as DESIGN.md, one component |
| [ROADMAP.md](./ROADMAP.md) | All the work in one place: delivery posture, value path, units with their finish lines, production points, dependencies, open decisions. The only top-level document that tracks build state, apart from a decision record's status ([docs/adr/README.md](./docs/adr/README.md)) |
| [docs/adr/README.md](./docs/adr/README.md) | The decision records: index, record format, statuses, and the granularity rule (one decision per record, cut by the re-argue test) |
| [TESTING.md](./TESTING.md) | What tests a piece of work must have and what proves it done, linking the records that decide it |
| [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) | Every control's proving injection, past and pending. A new control lands with its injection row |
| [docs/MUTATIONS.md](./docs/MUTATIONS.md) | The mutation ledger. Per-control proof that tests go red when the mechanism is broken, written only from implementation time |
| [.github/ISSUE_TEMPLATE/ticket.md](./.github/ISSUE_TEMPLATE/ticket.md) | The format every ticket is cut from. The rules for tickets are under [Repository process](#repository-process) |

Design questions resolve there in the order outcome → pillar/glossary → decision record.

## Keep the documents current — a standing duty

When something new comes up during ANY work, such as a decision made in conversation, a fact
learned while implementing, a plan change, a new term or a discovered risk, updating the relevant
document is part of that work, not a follow-up. Nothing about the system lives only in chat, code,
or commit messages. To do it, launch the **`update-docs` skill** with the Skill tool and use it in
full, following the approach it lays out, never bits and pieces of it. It routes content to the
right document, carries the authoring procedures, and ends with a whole-set coherence check.

The whole-set coherence check is its own skill, **`coherence-check`**. It keeps the entire
repository internally consistent, documents and implementation alike, and applies to implementation
work as much as to document changes. The `update-docs` and `adversarial-review` skills both run it.

The binding conventions per document live in `.claude/rules/` and load automatically when the
matching file is read. The format authorities are the documents' own preambles and the
decision-record index.

## Code layout and conventions

How the code is laid out and the conventions every component follows. The decisions behind them that
had alternatives live in the records, cited by number. What tests a piece of work needs is
[TESTING.md](./TESTING.md)'s, which controls exist and how each is proven is
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md)'s, and build state is [ROADMAP.md](./ROADMAP.md)'s,
apart from a decision record's status. The UI's own layout is
[docs/UI.md section 18](./docs/UI.md#18-repository-and-build-layout). The data-access library and
the five narrow shared libraries, three of them families, each describe themselves in a README in
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

Each row is one component, which is one top-level directory except the packaging row, whose two
directories are one component and share its `component:` label. Published names follow ADR-0054's
convention, and a directory carries the bare word.

| Directory | Kind | Published as | Holds |
| --- | --- | --- | --- |
| `content/` | Family | `mediated-mailbox-content` | The code that reads a message's content in memory and stores none of it. Its package is `content/markdown`, the conversion of a body's HTML to clean Markdown, argued in [content/README.md](./content/README.md) |
| `core/` | Library | `mediated-mailbox-core` | The shared pure library ([ADR-0050](./docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), with one subsection per pure concern needed by more than one deployable. They are `sensitivity`, `classify`, `redact`, `authorize`, `scan`, `scangate`, `index` (the decisions every job kind that writes the index makes about a message), `plan`, `policy`, `mail`, the canonical mail model and the Provider Port's pure names, [listed below](#what-coremail-holds), and `config` (every program's configuration type with its defaults and checks, and a length of time as `config.Duration`, so it imports no clock) |
| `credential/` | Family | `mediated-mailbox-credential` | The encryption of a stored credential, argued in [credential/README.md](./credential/README.md). `credential/seal` seals a value to the public key. `credential/open` loads the private keys and opens a sealed value with the key its header names, and only `process/connections`, the UI's `ui/internal/setup/clientsecret` and the test tool `testsupport/cmd/keypaircheck` import it. The settings naming the key files are `core/config`'s |
| `db/` | Library | `mediated-mailbox-db` | The data-access library ([ADR-0047](./docs/adr/data/0047-schema-first-data-access.md)), laid out in [db/README.md](./db/README.md) |
| `process/` | Family | `mediated-mailbox-process` | What a process is and what it holds ([ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)), argued in [process/README.md](./process/README.md). Its packages are [listed below this table](#the-packages-of-the-process-family-and-the-provider-library) |
| `provider/` | Library | `mediated-mailbox-provider` | Everything about talking to a provider, argued in [provider/README.md](./provider/README.md). Its packages are [listed below this table](#the-packages-of-the-process-family-and-the-provider-library) |
| `testsupport/` | Library | `mediated-mailbox-testsupport` | The shared test tooling, argued in [testsupport/README.md](./testsupport/README.md) |
| `api/` | Deployable | `mediated-mailbox-api` | The API server |
| `worker/` | Deployable | `mediated-mailbox-worker` | The worker, which runs every background job kind ([ADR-0117](./docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)). Its spec in `worker/app` lists its job kinds, which run on the process's scheduler, `process/schedule` ([ADR-0119](./docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)). Each job kind's shell code sits under `worker/internal/<job kind>/`, with backfill as `backfill` and delta sync as `deltasync`, and each job kind has an import list of its own. Each package's pure core sits in a `core` beside it, such as `worker/internal/backfill/core` and `worker/internal/deltasync/tick/core`. The code the job kinds share is `worker/internal/index`, which writes a message to the index and runs the gate-and-scan loop, and `worker/internal/runs`, which records runs, each with its pure core beside it. The series the job kinds share are `worker/internal/series` |
| `ui/` | Deployable | `mediated-mailbox-ui` | The UI, both halves ([docs/UI.md](./docs/UI.md#18-repository-and-build-layout)) |
| `migrator/` | Image | `mediated-mailbox-migrator` | The migrator, the migration step's image, which is not a deployable. Its `main.go` and its spec in `migrator/app` are shaped like a deployable's, and the spec's one task, built by `migrate.Build` in `migrator/internal/migrate`, applies the chain `db/migrations` embeds through `db/migrate` and exits. The task's package is the migrator's own, since a task's `Build` takes the process's capabilities and the layers bar `db/` from importing `process` ([ADR-0048](./docs/adr/data/0048-forward-only-migrations.md), [ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md), [ADR-0139](./docs/adr/data/0139-the-migration-runner-is-goose-linked-into-the-migrator.md), [ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)) |
| `cli/` | Program | `mediated-mailbox-cli` | The command-line program, the one binary an operator runs by hand, with a subcommand per task. Its `main.go` reads the subcommand and `--version`, and each subcommand sits under `cli/internal/<subcommand>/`. Its one subcommand is `keygen`, which writes the key pair credentials are sealed to ([ADR-0088](./docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)). It has no image, and ships as signed static binaries attached to each release |
| `packaging/chart/`, `packaging/tests/chainsaw/` | Packaging | `mediated-mailbox` | The Helm chart ([ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)), its usage guide in [packaging/chart/README.md](./packaging/chart/README.md), and its unit tests, a Go package of test files alone that `.helmignore` keeps out of the packaged chart with their `testdata/` and the values file under `ci/`. The chainsaw suite in `packaging/tests/chainsaw/` is the system test of the chart ([ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)), apart from every deployable ([ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md)), and carries the chart's label |

#### The packages of the process family and the provider library

The last column names the only importers a package's fence admits. Their own lists admit it, and
so does every wider list that matches their files, while every list that matches only other
packages' files and does not admit the package refuses it. So a fence holds only where every
non-test file outside the importers is matched by a list that leaves the importers' files out and
does not admit the package, and where no such list exists yet the code ticket that builds the fence
writes one ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md), whose fences
table names each refusing list). A wider list, which admits this project's packages by prefix,
refuses no fenced package. A package with no entry there is admitted to each component whose import
list names it.

| Package | Holds | Imported only by |
| --- | --- | --- |
| `process` | The spec, which lists what a program is made of, and the process that runs it. That is the build and the run, the servers, the stop order, the readiness reports, the health and readiness probes and the metrics endpoint of every process that runs until it is told to stop, the registration of the Go runtime's and the process's series, and the slots the owner packages keep shared things in | |
| `process/settings` | The layering of defaults, one optional configuration file, environment variables and flags into the configuration of each program that runs as a process, and the reading of a `config.Duration` | |
| `process/database` | The opening of every measured pool, through `database.Get` and, for the migrator, `database.ForMigration`, the latency of every statement a connection runs, the statistics of each pool, the schema check, and the readiness report of a pool answering a ping, the process's one pool's under `database` and a job kind's own pool's under its job kind's name, which a program opts into through `database.ReportReadiness` | |
| `process/database/core` | The label a statement's latency is recorded under, the database package's pure decision | |
| `process/schedule` | The scheduler, which runs each job kind's plan, adds and drops its jobs by key, and reports the job kind not ready until its first listing has succeeded | |
| `process/sessions` | The Session, all the user state a process holds, with its one reload, the reload's order and its failure rules | |
| `process/accounts` | The reading of the list of accounts | |
| `process/policies` | Each account's policy, the base policy included, the load concurrent body requests share, and the reload-failure alarm | |
| `process/connections` | Each account's connection to its provider, the unit of work and the re-seal to a new key | `api/internal/bodies` and the packages of a job kind that calls a provider, and the worker's test files for the one option the connections export for a test |
| `process/scanner` | The building of the scanner, once per process, and the body-processing histogram | |
| `process/domains` | The domain lookups every program classifies senders with, the IDNA conversions and the public suffix list the sender classifier normalizes domains through, which its pure core cannot import | |
| `process/logging` | The logger every program that runs as a process writes JSON to standard output through, and the reading of its `log_level` | |
| `process/version` | The release version every binary's `--version` reports, which the release cut writes | |
| `process/host` | The host each program ran on before it was on its spec, which no program runs on now, kept beside `process` until the refactor [ROADMAP.md](./ROADMAP.md) records deletes it | |
| `process/probes`, `process/runtimemetrics` | The operations endpoints' handler, and the registration of the Go runtime's and the process's series, which `process` and `process/host` share until the host is deleted | |
| `provider` | The calling side, which is the caller, the token, the one wrapper of a mailbox's operations, the list of shipped adapters, the facts about each provider and the shared provider metrics | |
| `provider/mail` | What any mail provider can do, `mail.Mailbox`, the Provider Port's interface for mail, with its consent contract and the call sizes a caller needs, which backfill and delta sync's tick read. Each is what `core/mail`'s `CallSize` gives within the hard-cap fraction `core/mail` holds. It also holds `mail.Body`, a body's fetch over a `mail.Mailbox` with the second call for a part held apart, which every program that fetches a body uses. The port's errors, the body's part and the canonical model it speaks are `core/mail`'s | |
| `provider/mail/gmail` | The Gmail adapter and its rate profile | `provider` |
| `provider/mail/gmail/core` | The Gmail adapter's pure core of body decisions | `provider/mail/gmail` |
| `provider/internal/adapters` | The representation of the list of adapters and of a token, which `provider` exports as `provider.Adapters` and `provider.Token`, so only the provider library builds a list holding an adapter, `provider.Shipped` and, for a test, the provider fake's `fake.Adapters` | The provider library, by Go's rule for an `internal` directory |
| `provider/mail/fake` | The provider fake, and the list of adapters over it a test hands the connections through `connections.WithAdapters` | Test files |
| `provider/mail/contract` | The contract suite | |
| `provider/google` | Google's grant, its token source and token endpoint and its consent, with the one-time Gmail consent command | |
| `provider/consent` | Each provider's consent by name | `ui/internal/setup` |
| `provider/ratelimit` | The rate limiter, its shared state and its leased call | `provider`, and the API server's `api/internal/registry` for the rate-state collector |
| `provider/ratelimit/core` | The rate limiter's pure rules, its limits and its state, reached only by the limiter and the caller that builds it | `provider` and `provider/ratelimit` |
| `provider/mail/jmap`, `provider/calendar/google`, `provider/calendar/caldav` | Designed directories for the JMAP adapter and the calendar adapters | `provider` |

#### What `core/mail` holds

A name sits in `core/mail`, beside the canonical model, when code that may not import the provider
library reads it, when it is a pure decision, which sits under a directory named `core`, or when it
stays with the throttle error, as the names the rate profiles are stated in do. The Provider Port's
interface, `mail.Mailbox`, is `provider/mail`'s, with the call sizes it carries.

| Names | Why they sit in `core/mail` |
| --- | --- |
| The canonical model, with the normalization of attachment media and their mapping to the closed vocabulary of attachment types, and the canonical query | The adapters, the pure decisions of `core/` and the programs all read the model the port speaks ([ADR-0010](./docs/adr/provider/0010-one-provider-port.md)) |
| `MaxBodyBytes`, the size bound of a rendered body | The conversion in `content/markdown` reads it ([ADR-0131](./docs/adr/provider/0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md)) |
| The port's errors, among them the throttle error, with the throttle signal and its scopes, which it carries | `core/index` classifies the errors and the API server's service layer reads them, and neither may import the provider library |
| `NextBodyPart`, the decision on a second body call, with `BodyPart` and `HeldApartError`, which it reads | It is a pure decision ([ADR-0131](./docs/adr/provider/0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md)) |
| `RateLimitProfile`, the interface every adapter's rate profile implements, with the operations it prices, the cost type and the hard-cap fraction | They stay with the throttle error. The adapters and the provider fake read the profile's types and the throttle signal, the contract suite those and the hard-cap fraction, and the rate limiter's pure rules the cost type, the throttle signal and the fraction, so none of them is the limiter's own ([ADR-0023](./docs/adr/operability/0023-adapter-declares-cost.md)) |
| `CallSize`, the sizing of a call within one second's worth at the hard cap | It is a pure decision. The call sizes `mail.Mailbox` carries are what it gives ([ADR-0010](./docs/adr/provider/0010-one-provider-port.md)) |

The components sit in layers, and a layer imports only the layers above it.

```text
core                           shared pure decisions, the canonical mail model, and core/config
content                        reading a message's content
db                             the statements
credential                     the encryption of a stored credential
provider                       everything about talking to a provider
process                        what a process is and what it holds
api  ui  worker  migrator  cli the programs
testsupport  packaging         the test tooling, and the chart with its system test
```

A deployable is named for its role in one plain word, as `api` and `worker` are. `ui` keeps
the directory the Glossary gives it. Shared code is the shared pure library, or a narrow, named library
that argues its own case as [ADR-0050](./docs/adr/engineering/0050-shared-code-pure-or-narrow.md)
requires. The data-access library's case is ADR-0047's, and each of the other five argues its case
in its README, a family once for all its packages. Where shared code goes, and when a library or a
family of packages is founded, are the rules against sprawl in
[ADR-0050](./docs/adr/engineering/0050-shared-code-pure-or-narrow.md). How a component, a family or
a package is named, and who chooses the name, is the naming convention in
[ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md).

### Inside a component

| Convention | Rule |
| --- | --- |
| Spec | A deployable's or the migrator's entry package, `<deployable>/app`, holds its spec, `process.Spec`, and nothing else. What a spec lists, what a `Build` is handed and returns, what the process keeps for the parts and the order it starts and stops in are [ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)'s. The conventions that go with the spec are [listed below this table](#the-conventions-that-go-with-the-spec) |
| Operator commands | A command an operator runs by hand, which no deployable runs, is a subcommand of the command-line program, `cli/`, which ships as signed static binaries attached to each release. Its `keygen` subcommand writes the key pair that credentials are sealed to ([ADR-0088](./docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)). A developer's tool, which ships nowhere, sits under its library as `cmd/<name>/` in `package main`. The Gmail consent for the test account's token is `provider/google/cmd/consent`, a developer's tool that ships in no image ([ADR-0107](./docs/adr/provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)). The test tooling programs under `testsupport/cmd/` follow the same form |
| Private code | Everything else a deployable holds sits under `<deployable>/internal/`, so the compiler refuses an import from another component as well as the import rules of [ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md). The exceptions are the entry package, the UI's browser bundle package `ui/browser`, and an `importtarget` package holding a violation file, described under [Tests](#tests). `ui/browser` embeds the bundle beside it, since an embed pattern cannot reach a parent directory, so the package serving the UI's pages, `ui/internal/pages`, imports it |
| Pure core | Pure-core code sits under a directory named `core`. That is the top-level `core/`, `<deployable>/internal/core/<concern>/`, such as the API server's `api/internal/core/release`, `<deployable>/internal/<package>/core/` beside the shell code that uses it, such as the worker's `worker/internal/backfill/core`, `<library>/core/` inside a library that holds pure rules of its own, `<library>/<package>/core/` inside a library's package whose pure rules are that package's alone, such as the Gmail adapter's `provider/mail/gmail/core` and the rate limiter's `provider/ratelimit/core`, and `<family>/<package>/core/` inside a family's package, since each package of a family keeps its own and a family never pools them. The word means the Glossary's pure core wherever it appears, so the core import check matches a `core` path element at any depth, anchored at the repository root |
| Shell packages | Named for what they do, for example `api`, `mcp`, `service`, `bearer`, `gmail`. No package is named `util`, `common` or `helpers` |
| Library subsections | A library is organized in subsections by concern, each a package, granular enough that an import list can name exactly the subsections a component may use, as every component's list does for the data-access library |
| API server layering | The API server's packages and the fences between them, [listed below this table](#the-api-servers-layering) |
| Generated code | Stays inside the component that generates it. Data access sits under `db/<subsection>/`, the UI's contract document at `ui/contract/`, the API server's at `api/contract/`, and the browser types under `ui/browser/src/generated/` |
| Designed directories | A directory the design names as a Go package states its role in its package comment, which sits in a `doc.go` while the package holds no other Go file. Any other designed directory that holds no file yet holds a `.gitkeep` |

#### The conventions that go with the spec

- **The wiring is ordinary code.** The spec builds nothing, so a reader can follow the wiring, with
  nothing resolved by registry, reflection, annotation or naming convention
  ([ADR-0040](./docs/adr/engineering/0040-pure-core-decisions-as-values.md),
  [ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).
- **`main.go` is one call.** A deployable's or the migrator's `main.go` at the directory root is
  `process.Run(os.Args[1:], os.Environ(), app.Spec())`, since only a `main.go` may read the
  environment ([ADR-0078](./docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)).
  `process.Run` sets up the signal context the stop begins from and turns the run's result into the
  exit status. Nothing else in the deployable is `package main`.
- **The configuration type is `core/config`'s**, with its defaults and checks. A `Build` takes the
  program's whole configuration and reads what it needs.
- **The schema check.** `database.Get` runs it on each pool before returning it. The check reads the
  schema's versions through the start check's subsection and compares them with the chain's through
  `db/migrations`
  ([ADR-0140](./docs/adr/data/0140-each-migration-declares-the-oldest-chain-that-runs-on-it.md)).
- **Logs.** The process builds the logger and makes it the process default for code the project
  does not own. A `Build` takes it through the capabilities and hands it to every shell that logs. A
  `forbidigo` ban refuses, in every Go file, reading a default logger through `slog`'s or `log`'s
  package-level functions and the built-in `print` and `println`
  ([ADR-0122](./docs/adr/engineering/0122-logs-through-slog-at-a-configured-level-handed-to-shells.md)).
- **Goroutines and routes.** A `Build` starts no goroutine of its own and registers no route on a
  mux it is handed. The worker's job code starts a goroutine only through the scheduler's helper,
  `schedule.Go`, whose panic fails that run and stops nothing else
  ([ADR-0119](./docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).
  The API server's and the UI's code, and a thing the process shares, start one only through the
  process's `Go`, whose panic fails what started it and stops the process in order
  ([ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). The
  goroutines analyser refuses a `go` statement and any use of a function that starts a goroutine
  running what it is given in the non-test code under `api/`, `ui/` and `worker/`, so that code
  starts one only through a helper that recovers it. Which helper is held by review, since a job
  kind's `Build` is handed the process's `Go` too and the analyser refuses a call of neither, and so
  is a goroutine a thing the process shares starts, since the analyser does not read the process
  family's code.
- **The entry sits outside `internal/`**, so the compiler lets another program's spec join it, and
  the import lists refuse it to every other component until such a spec exists.
- **A deployable with nothing to run yet** holds its entry as a designed directory.

#### The API server's layering

| Package | Holds | Its fence |
| --- | --- | --- |
| `api/internal/api`, `api/internal/mcp` | The two protocol roots, each generated by one file, `api.go` and `mcp.go`, from the one registry of operations. `api/internal/mcp` also holds the spec's `Init`, which refuses `MCPGODEBUG` | Only the generator file of each root is admitted the library its protocol is served with, `net/http` and the MCP SDK. Import rules per root refuse a root reaching below the service layer |
| `api/internal/service` | The one service layer beneath both roots, holding every operation, the body operation among them. Enforcement sits below it ([ADR-0030](./docs/adr/operability/0030-api-core-mcp-thin-adapter.md)). Its decisions sit in the API server's pure cores, what each operation may be and the generation checks in `api/internal/core/operation`, a failure's origin and message in `api/internal/core/failure`, and the release step in `api/internal/core/release`, and it reads each operation's schemas once into the facts the checks and the roots read. It names, as aliases, the types and values of those cores the roots read, since the roots reach no lower than this layer | Admitted no connections and no provider, held by the API server's list `api` over its non-test code, which leaves out the MCP root's generator, `api/internal/bodies` and `api/internal/registry`. Each of those two packages' own lists, `api-bodies` and `api-registry`, refuses the other's fenced package |
| `api/internal/registry` | The building of the registry of operations, and the rate-state collector registered beside it. Each entry declares its effect, and its HTTP method, path shape and MCP annotations are derived from it ([ADR-0087](./docs/adr/operability/0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)) | The one package of the API server admitted `provider/ratelimit` |
| `api/internal/bodies` | The two functions the body operation needs of a provider, loading the policy fresh and fetching a body through a unit of work. It is the one package of the API server that begins a unit of work, and the body operation alone is built with its functions | The one package of the API server admitted `process/connections` |
| `api/internal/bearer` | The bearer-token check and the no-store header, and the API server's readiness report on its token file, read again every five seconds, once per process | Admitted `net/http` as a whole package, though it is no root |
| Every other package under `api/internal/`, a new one included | | Admitted no HTTP package and not the MCP SDK, so an operation cannot be registered on the client surface from any of them. The list `api-no-http`, over the non-test code under `api/internal/` that leaves out the two roots and `api/internal/bearer`, holds it |

The process mounts each endpoint the spec lists at its pattern. Which endpoints the spec lists, and
any route `api/internal/bearer`'s wrapper adds to the root handler it wraps, are left to review
([ADR-0053](./docs/adr/engineering/0053-parity-by-construction.md),
[ADR-0086](./docs/adr/engineering/0086-mcp-root-on-the-official-go-sdk.md),
[ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).

### Tests

| Kind | Where and how |
| --- | --- |
| Unit tests | `_test.go` files beside the code, in the external `<package>_test` package unless a test needs unexported access and is not a generator. A shell test against the provider fake, which [TESTING.md](./TESTING.md) counts as an integration test, needs no database, so it is an ordinary test file without the `integration` build tag |
| Property tests | Files named `*_property_test.go`, in external test packages ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)). The property-testing library is admitted only in these files, crash-sequence files, and the `testsupport` packages that need it. A property runs through `property.Check` and its generator report through `property.Report`, or `property.ReportReach` for cases drawn several ways that can each reach several kinds, as a crash test's are, all in `testsupport/property`. Each fails a run whose `RAPID_SEED` is unset or zero or whose `RAPID_NOFAILFILE` is not `true`, so a local `go test` sets the values `go-property.yaml` sets |
| Crash sequences | Files named `*_crash_test.go` in the component whose machinery they target ([ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)). A crash-sequence file whose reduced sequence replays against PostgreSQL also carries the `integration` build tag |
| Integration tests against PostgreSQL | Files named `*_integration_test.go` carrying the `integration` build tag, and run only under `go tool pgrun` with `-tags integration`. `pgrun` starts one PostgreSQL container for the run and prepares a template database, and each integration test package creates its own database from it through `testsupport/postgres` ([ADR-0068](./docs/adr/engineering/0068-test-substrate-containers-directly.md)). One test in `testsupport/postgres` runs `pgrun` a second time, on the next port, to prove a run without the tag fails, so with a remote docker daemon both ports need a route. The test image's reference sits in `testsupport/cmd/pgrun/image.go`, pinned by digest and tracked by renovate |
| Recorded browser fixtures | Integration tests that record the browser's fixtures from the real server and fail when a checked-in fixture differs are named `*_fixtures_integration_test.go`, carry the `integration` build tag, and run in the `ui` workflow beside the browser tests that read the fixtures ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)) |
| Which workflow runs a test | A test file's name gives its kind, unit, property, crash, integration or recorded fixtures, and its build constraint its level, unit when it compiles with no build tag and integration when it compiles only with the `integration` tag. `go tool testkinds args <kind> <level>`, from `testsupport/cmd/testkinds`, prints the arguments that make `go test` run exactly that kind at that level, and `go tool testkinds check` refuses a test file whose kind and level no workflow runs, a test name two runs would both select, a workflow that asks for its run only narrowed to some packages, a workflow line that runs `go test` without those arguments or adds anything beyond `-race`, `-count`, `-v`, `-timeout` and a pipe into `tee`, a job or step condition, `continue-on-error` or `GOFLAGS` in a workflow that runs `go test`, and a pull-request path list of the `ui` workflow, the recorded-fixtures run's, that leaves unwatched a file of this module the recorded-fixtures tests compile or embed, as `go list -deps -test` reads them, a pattern in that list holding `?`, `+`, `[` or `]`, which the check does not read, being refused, so each test runs in the workflow that gates on its kind, and a property test or crash sequence also in its kind's deep workflow, and in no other ([ADR-0124](./docs/adr/engineering/0124-each-kind-of-test-runs-in-a-workflow-of-its-own.md)). A unit test is at the unit level, an integration or recorded-fixtures test at the integration level, and a property test or a crash sequence at either. A contract run against a real provider runs only in its own workflow, `gmail-contract`, through `go tool livecontract`. The check refuses a `_live_test.go` file that compiles at either level, and any line naming the livecontract command, however it is started, or its marker variable in any other workflow, and `args` prints no selection while a live file compiles at a level in its own environment. The check guards a workflow edit against honest mistakes and reads each workflow's text only for the forms named here, parsing neither YAML nor the shell, so a step whose shell, error handling, environment or selection is changed by any other means is left to review, as is a `go test` reached through a variable, a script or another program, examples being `set +e`, a `shell:` without `-e` or pipefail, a quoted `'if':` key and a `GOENV` file |
| Contract runs against a real provider | Files named `*_live_test.go` beside the adapter, carrying a build tag of the provider's own, `gmail_live` for Gmail, so no other test run compiles them, while lint and the `go vet` analysers read them. They add their own marked messages and check and report only those ([ADR-0043](./docs/adr/engineering/0043-no-mocking.md)). They run only through `go tool livecontract <provider>`, from `testsupport/cmd/livecontract`, which sets the marker the live test requires. Run any other way, the build tag included, a live test skips and names the command, so no accidental or incidental test invocation reaches a real provider |
| The live agent's exercise harness | `api/app/exercise_integration_test.go`, an integration test serving the API server through its spec and `process.RunContext`, which runs it as `process.Run` does for its `main.go`, over the provider fake, one account holding the shared synthetic fixtures with the bank's domain restricted, written to the index as backfill's passes write it. It hands the process connections over the fake through the one option the connections' package exports for a test, so what it serves is what the API server serves apart from the fake ([ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). It runs only when `MEDIATED_MAILBOX_EXERCISE` names the address it listens on, so every other run skips it, and the operator, or a session on the operator's instruction, starts it under `go tool pgrun` with `-run` naming it and `-timeout 0` for the manual exercises with a live agent that [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) keys to D3. It prints the account, the bearer token and the roots' addresses, serves plain HTTP until it is interrupted, and then prints every audit row the exercise wrote. Nothing that ships can select the fake |
| Violation files | Placed where the ban or import rule they prove applies, and named for it. A Go violation file ends in `_violation.go` for a rule over non-test files, or in `_violation_test.go`, `_violation_property_test.go` or another test-file suffix for a rule over test files. It carries the `banproof` build tag, so the gating lint never loads it, and holds `// want` annotations naming the finding it must produce ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md)). A browser violation file ends in `_violation.ts` or `_violation.tsx`. The gating browser lint and type check skip files with that name, and nothing imports one, so the bundler never reaches it. An SQL violation file ends in `_violation.sql` under `db/check/testdata/violations/`. A few violation files need a target of their own. A deployable's `importtarget` package sits outside `internal/` only so the other deployables' lists have something to refuse, and `residual_violation.go` at the repository root proves the list over Go files outside every component. `worker/internal/testdata/target`, `worker/internal/_target` and `worker/internal/.target` are what `worker/unlinted_violation.go` imports, one in each kind of directory `./...` skips by its name. They carry no `banproof` tag, because `./...` never lists them and only the violation file imports them, so nothing builds them outside the ban-proof run |
| Must-not-compile fixtures | Under `testdata/mustnotcompile/<case>/`, loaded by `testsupport/mustnotcompile` ([ADR-0042](./docs/adr/engineering/0042-implementation-stack.md)) |
| Golden files | Under `testdata/golden/` in the package that owns them, written and compared by the golden-file helper in `testsupport/compare` ([ADR-0070](./docs/adr/engineering/0070-unit-comparison-through-one-options-value.md)). A recorded file read outside the recording test's package sits where its reader expects it, outside `testdata/golden/`, and is written and compared by the helper's `GoldenAt`. They are the browser's fixtures under `ui/browser/test/fixtures/`, recorded by the UI's server tests ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)), the contract document under `ui/contract/`, and the table of account identifier cases at `ui/browser/test/identifiers.json`, which the UI's server tests write from the table they test the server's check and the database against, so the browser's test of the proposal reads the same cases ([ADR-0128](./docs/adr/data/0128-account-identifiers-are-one-lowercase-dns-label.md)). Both forms share one `-update` flag, which `testsupport/compare` registers and no other package does |
| Known-answer files | Under `testdata/knownanswer/` in the package whose test reads them, holding bytes an earlier build wrote that this build must still read, such as a key pair with a value sealed to it. They are the test's expectation, so they are written once, outside the test, and never regenerated. No `-update` flag or helper writes them, since a build that rewrote them would pass against itself. The test's comment says how they were made |
| Mutation demonstrations | A patch under `testdata/mutations/` in the package whose control it demonstrates, which is usually the package whose file it edits. A control can rest on another package's code, as the Redaction Gate's proof that nothing reaches it but its parameters rests on the fields of the classifier's types, and a patch breaking it there still sits with the control. There is one patch per way the mechanism is broken, each describing itself in the text before its first diff header, and `go tool mutproof` runs them from the repository root ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)). Each hunk's context and removed lines match at one place only in the file it targets, so git apply cannot put it anywhere else, and the runner refuses a patch that breaks this before it runs a demonstration. A patch failing it is regenerated with wider context, `git diff -U<n>`, its changed lines kept as they were. The runner's package comment states the keys that text carries. A patch whose tests are the browser's sits under `ui/browser/testdata/mutations/` and has the runner run them with `bun test` |
| Browser tests | Under `ui/browser/test/`, run by `bun test` with the DOM shim preloaded ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)). The app reads its clock through its dependencies' `now`, and its components schedule their delays, a region's loading phases, the live clock's tick and the keyboard map's g window, through `timers`. A test moves time by advancing `ManualTimers` and never waits on the machine's clock, which under load fails any test that races a real delay. Nothing `settle` waits on is real I/O or a real frame either. The recorded fetch in `test/fixtures/fetch.ts` reads every recording when a test gives its answers and answers each read from memory within microtasks, and `mount` in `test/render.ts` renders inside `act`, so the first render's effects run before it returns rather than on the DOM shim's next frame. `settle` then turns the task queue once per round inside `act`, which lands every read in flight, and returns after the first round that renders nothing, since only a render queues an effect that could start another read. It fails at a bound on its rounds, because a tree still rendering by then is in a loop. A read a test holds open on purpose stays open. The event stream's transport runs only in a browser and keeps the browser's own timers for its reconnects and polls |

`testdata/` is always per package. A test runs from its own package's directory and reads
`testdata/` by a relative path. The one exception is `worker/internal/testdata/target`, a violation
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
  integration,gmail_live -vettool="$(go tool -n vetcheck)"`, with the tags golangci-lint's
  configuration sets, which banproof checks. The placement analyser's two rules refuse a call to a
  generator report, `property.Report` or `property.ReportReach`, or to `property.Check`, reachable
  from inside a property
  ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)). The globals
  analyser refuses package-level state in a pure core by the four rules
  [ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md) states. The environment
  analyser refuses every standard-library function that returns an environment variable's value by
  a name its caller gives, or the environment as a whole, which is `os.Getenv`, `os.LookupEnv`,
  `os.Environ`, `os.ExpandEnv`, `syscall.Getenv`, `syscall.Environ` and `(*exec.Cmd).Environ`.
  Functions that read fixed platform variables for their own purpose, such as `os.UserHomeDir`,
  `os.TempDir` and `http.ProxyFromEnvironment`, stay allowed. The rule covers every package of the
  module outside `testsupport`, apart from test files and a deployable's or the migrator's `main.go`
  at its directory root, which hands the environment to `process.Run`
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
  A subsection is recognised by its type, a package under `db/` whose `New` returns its own
  `Queries`. It covers every package, test files included. It exempts the calls that are not
  account-scoped, each written as one chained call. They are the accounts listing, the read of
  `oauth_clients`, the connections' re-seal of a client secret, which delta sync runs, the UI's
  OAuth client setup (listing each client's identity, and adding, replacing and removing a client),
  the UI's client-secret package's read of one client's sealed secret by its name, and the start
  check's read of the schema's versions. This paragraph is the one place the list is written, which
  [ADR-0047](./docs/adr/data/0047-schema-first-data-access.md) and the analyser's comment point to. The one gap it leaves is a query value built
  inside the literal that escapes it and is used after `tx.Run` returns, which its package comment
  states. The routes analyser refuses a route registered on a mux in the UI other than through its
  recording mux, as any use of `(*http.ServeMux).Handle`, `(*http.ServeMux).HandleFunc`,
  `http.Handle`, `http.HandleFunc`, or a method of an interface or type parameter named `Handle` or
  `HandleFunc` with the mux's parameters, in a non-test file under `ui/` outside the recording
  mux's own `Handle` method
  ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md)). Registration through
  reflection, by an imported package on the default mux, or through an interface method one of
  whose parameter types is a type parameter is left to review. The API server is outside its
  scope. Which endpoints its spec lists, and any route `api/internal/bearer`'s wrapper adds to the
  root handler it wraps, are left to review. The raw SQL analyser refuses a statement the API
  server, the UI or the worker runs other than through the data-access library, as any use, on any
  type, of a method named and typed like one of the driver's methods that run a statement or start
  a batch or pipeline that does, `Query`, `QueryRow`, `Exec`, `Prepare`, `ExecParams`,
  `SendBatch`, `ExecBatch`, `CopyFrom`, `CopyTo` and `StartPipeline`, in a non-test file under
  `api/`, `ui/` or `worker/`. So a program's code runs only the statements of the data-access
  subsections its exact import list admits. The API server, the UI and the worker connect as one
  application user, so that is what bounds what their code can do in the database
  ([ADR-0143](./docs/adr/data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)).
  In the UI it keeps the read of a client's sealed secret with the part its import list admits it
  to ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md),
  [ADR-0081](./docs/adr/operability/0081-credentials-sealed-to-a-public-key.md)). A statement run
  through reflection, written to the wire through the raw connection the lower-level connection
  hands out, or run through a method one of whose parameter types is a type parameter is left to
  review. So is a statement written by hand in code outside those three directories, since the
  analyser does not read it. The process family's code runs as the application user in the API
  server, the UI and the worker, and as the migrator's user in the migrator, and the Session, the
  connections and the provider library's rate limiter run in the API server and the worker. The
  callers analyser's two rules hold a method of the connections to the callers of one package each.
  The attach of the connections to the Session, `(*sessions.Session).Attach`, is used only in
  `process/connections`, and the re-seal to a new key, `conns.Reseal`, only in delta sync's
  package, `worker/internal/deltasync`. Each rule reports any use, a call, a method value or a
  method expression, in a non-test file of any package of the module, and a use of a method of the
  same name and signature on an interface or a type parameter. A call through reflection, or through
  an interface method whose name or signature differs, is left to review
  ([ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md),
  [ADR-0092](./docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md),
  [ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). The
  goroutines analyser refuses, in a non-test file of any package under `worker/`, a `go` statement
  and any use, a call, a method value or a method expression, of a function that starts a goroutine
  running what it is given, which are `(*sync.WaitGroup).Go`, `time.AfterFunc`, `context.AfterFunc`,
  `runtime.SetFinalizer`, `runtime.AddCleanup`, `(*net/http.Server).RegisterOnShutdown`,
  `(*errgroup.Group).Go` and `TryGo` and `(*singleflight.Group).DoChan` of `golang.org/x/sync`.
  It refuses the same in a non-test file of any package under `api/` or `ui/`. So that code starts
  a goroutine only through a helper that recovers it. The worker's job code uses `schedule.Go`, the
  helper of the process's scheduler in `process/schedule`, which recovers a panic on it and returns
  it as the run's error, so it fails that run and stops nothing else
  ([ADR-0119](./docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).
  The API server's and the UI's code use the process's `Go`, which recovers a panic on it as the
  failure of what started it, stops the process in order, and waits for it before the slots are
  closed ([ADR-0142](./docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).
  The analyser refuses a call of neither helper, and a job kind's `Build` is handed the process's
  `Go` too, so which helper code uses is held by review. A thing the process shares, which its
  owner package under `process/` builds, starts one through the process's `Go` too, held by review,
  since the analyser does not read the process family's code. Such a function reached through an
  interface value or reflection, a dependency that calls a function or value it was given on a
  goroutine of its own, and a function of any other module that starts a goroutine are left to
  review. The unsafeimport analyser refuses an import of `unsafe` in a non-test file of any
  package of the module, a `go:linkname`'s included, and any use of `reflect.NewAt`,
  `reflect.SliceAt`, `(reflect.Value).UnsafePointer` and `(reflect.Value).SetPointer`, which hand
  out or take an unsafe pointer with no import of `unsafe`, since code isolation per job kind in the
  worker rests on memory safety
  ([ADR-0117](./docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)). A use of
  those through an interface value or reflect's own method lookup, and writing the process's own
  memory through the operating system, such as through `/proc/self/mem` at an address
  `(reflect.Value).Pointer` gives, are left to review. Every list admits the standard library
  through one entry and no list has a deny entry, so the rule is an analyser. The worker's own test
  holds the packages of its build outside the standard library that import `unsafe` or hold assembly
  or a `.syso` object to the list it names, which changes only by review, and its Dockerfile to a
  build without cgo. banproof requires each rule from its violation file, where a want annotation
  names the finding as `vetcheck`.
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
  return, as the Provider Port's one body-returning operation needs. `RequireTakers` asserts exactly
  which of a package's functions, methods and package-level variables can hold a value carrying a
  type, an alias of it included, through anything but a function type, as the API server's body
  operation needs to be the one operation handed the functions that begin a unit of work.

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
files, golden files, the known-answer files under a package's `testdata/knownanswer/`, mutation
patches, recorded browser fixtures, the synthetic mail fixtures, Helm templates and the vendored
font licence. The oxfmt and sqlfluff hooks also skip violation files. Every tool a local hook runs
also runs in a CI workflow, because the hygiene workflow runs only its own named hooks.

### Images

- **One Dockerfile per image, in its component's directory**, built with the repository root as the
  context. A library holds none, which is how it is told apart
  ([ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md)). Each
  Go image, every deployable's and the migrator's, builds a static binary and copies only that
  binary onto a minimal base
  ([ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md)).
- **`ui/Dockerfile` builds the browser bundle in its first stage.** Its Go stage refuses to build
  when the bundle holds only the checked-in placeholder, because an image embedding the placeholder
  would build cleanly and serve no UI.
- **No image this repository pins is pulled from Docker Hub.** CI's anonymous pulls there hit its rate limit, and its
  logins time out. The golang stages pull from `public.ecr.aws/docker/library/golang`, whose tag
  list Renovate reads in full, so the go group's Dockerfile pins move with go.mod and mise.toml.
  `oven/bun`, which no other registry serves, and the official PostgreSQL image the tests start pull
  from `mirror.gcr.io`, at the same digests. Its tag list can trail Docker Hub's by days, which
  delays Renovate's update of those two pins.

### CI workflows

One workflow per concern under `.github/workflows/`. A workflow that runs only for some changes
filters its pull-request trigger by a path allow list of the directories it watches, extended by one
entry per new component
([ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md)). The Go
workflows also watch every Go file, so a Go file in a new directory is linted and tested. Every
workflow line that runs `go test` passes the arguments `go tool testkinds args` prints for its kind,
so each kind of test runs in its own workflows and no other, and each workflow holds one role,
its triggers alone deciding when it runs
([ADR-0124](./docs/adr/engineering/0124-each-kind-of-test-runs-in-a-workflow-of-its-own.md)). The
`images` workflow is the exception on a pull request. It watches only Dockerfiles, `go.mod` and
`go.sum` there, since the Go workflows build and test the code, and a new component's Dockerfile is
among them without an entry. Its push trigger, which publishes `latest`, names everything the images
are built from, so a new deployable, or a library an image comes to link, adds its entry there. Jobs
that need the repository's tools install them from `mise.toml` through
`ppat/homelab-ops-actions/actions/setup-repository-tools`.

| Workflow | Runs on | Does |
| --- | --- | --- |
| `go-lint` | Go code, the lint configuration, every workflow file, tool pins | `go mod tidy -diff`, `golangci-lint config verify`, `golangci-lint run ./...` over the whole module whenever its paths match, the `go vet` analysers, `go tool banproof`, and `go tool testkinds check`, which reads every workflow file ([ADR-0124](./docs/adr/engineering/0124-each-kind-of-test-runs-in-a-workflow-of-its-own.md)) |
| `go-unit` | Go code, the chart, the browser bundle's sources, tool pins | The browser bundle's build, which the UI's tests read, then the unit tests, the example tests that need no build tag, which include `promtool test rules` over the alerting rules and a render of the chart that ships them ([ADR-0077](./docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)), the chart's own tests in `packaging/chart`, which render it, and each deployable's test that loads the file the chart renders for it through its configuration library ([ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)), the `db/check` tests and the comparison of the UI's contract document with the registry. Every gating test run sets a fixed non-zero `RAPID_SEED`, the gating `RAPID_CHECKS` and `RAPID_NOFAILFILE=true` in the workflow and passes no `-short` ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)). The scheduler's tests, `process/schedule`'s, run once more under the race detector, with cgo enabled for that test build alone ([ADR-0119](./docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)) |
| `go-property` | Go code, tool pins | The gating run of every property test, at the gating seed and case count, those needing the integration tag under `go tool pgrun` ([ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md), [ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)) |
| `go-property-deep` | A schedule, by hand, and a change to the workflow itself | The deep search over the same property tests at the scheduled case count, which a manual run may override, with a fresh seed each run, never cancelled by the next run, and failing when no property reported a passed run. It gates nothing ([ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md), [ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)) |
| `go-crash` | Go code, tool pins | The bounded run of every crash sequence, at the gating seed and case count, those needing the integration tag under `go tool pgrun` ([ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)) |
| `go-crash-deep` | A schedule, by hand, and a change to the workflow itself | The deep exploration over the same crash sequences, as `go-property-deep` runs the property tests ([ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)) |
| `go-integration` | Go code, the chart's alerting rules, the browser bundle's sources, tool pins | The browser bundle's build, which the UI's tests read, then the integration tests, the example tests that need the integration tag, under `go tool pgrun` with the gating property-run settings |
| `gmail-contract` | The schedule [TESTING.md](./TESTING.md#when-tests-run) states | `go tool livecontract gmail`, the Gmail adapter's contract suite against the test account, with its credentials from GitHub Actions secrets ([ADR-0043](./docs/adr/engineering/0043-no-mocking.md)) |
| `go-vulncheck` | Go code, and a schedule | `govulncheck` |
| `data` | `db/`, its configuration, tool pins | sqlfluff and the generator's diffs. The `db/check` tests run in `go-unit` and `go-integration` by their kind |
| `ui` | `ui/`, each directory of this module the recorded-fixtures tests compile or embed, the migration chain among them, which `go tool testkinds check` holds the list to, the ban-proof program, tool pins | One job, which runs in order the types and descriptor drift checks of [ADR-0065](./docs/adr/engineering/0065-contract-built-from-registry-consumed-as-generated-types.md), `bun build`, the UI's Go build and the tests that record the browser's fixtures and diff them, under `go tool pgrun` with `-tags integration` and the gating property-run settings, the type check, the formatting check, browser lint, `go tool banproof -browser`, and `bun test`. The recording tests and the browser tests share the job because the recorded fixtures are regenerated in the job that runs the Go tests they come from ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)). The contract document's own drift check is a unit test and runs in `go-unit`, and the UI's other Go tests run in the workflows of their kinds |
| `images` | A pull request changing a Dockerfile, `go.mod`, `go.sum` or the workflow itself, a push to `main` matching the publish list, and by hand | One job per image. On a pull request it builds every image through `ppat/homelab-ops-actions/actions/build-docker-image` without pushing, and the `ui-placeholder` job requires the UI's image to fail at its guard when the bundle directory holds only the placeholder. On a push to `main`, or by hand on `main`, the same job also pushes and signs each image through the action, by the action's own inputs, and tags its digest `latest`, refusing a `latest` that names another digest ([ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)). The publish list names what the images are built from, the directories of the shipped components each image's build copies and links, `go.mod` and `go.sum`, every Dockerfile, the browser bundle's sources and the workflow itself, and leaves out tests, documents, test data, mutation patches, violation files and their import targets, sqlfluff configurations, the test and tool packages no image links, the code generator's configuration and the SQL statement files it reads, whose generated Go is what reaches a binary, and `process/version/version.go`, which only the release cut writes and whose release publishes that commit as `latest` itself. Negated patterns come last, because the last matching pattern wins. Runs on `main` queue and never cancel. It is one of the two writers of `latest`, the release's image builds being the other. The publish list is kept by hand, extended per new component, and a failed `latest` build, or a `latest` a release left behind `main`, stays until the next matching push or a run by hand, while the chainsaw suite refuses a mixed or unlabelled set |
| `dockerfiles` | Any Dockerfile | hadolint, through the hygiene workflows' reusable job |
| `secrets` | Every pull request | gitleaks |
| `mutation-patches` | Every pull request | `go tool mutproof -check`, which runs `git apply --check` over every mutation patch under a `testdata/mutations/` directory, excluding the runner's own fixtures under `testsupport/cmd/mutproof/testdata/fixture/`, and refuses a patch with a hunk whose context matches at more than one place in its file, after a self-test proves the check refuses a patch whose context no longer matches and a hunk whose context matches twice. Unconditional rather than path-filtered, because a patch's diff context can span any file in the tree, and neither `go-unit` nor `go-integration` runs it, so a patch's applicability is a standing guard apart from the mutation demonstration itself, which runs when its control lands or changes ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)) and in full in `mutation-demonstrations` |
| `mutation-demonstrations` | A schedule, by hand, and a change to the workflow itself | The browser bundle's build, which the UI's tests read and the runner carries into its copies, then `go tool mutproof` over every mutation patch the `mutation-patches` workflow checks, in one job over a fixed matrix of sixteen, each taking every sixteenth patch by the index and job count the matrix gives it, at the gating seed and case count with the scheduled count for a patch that asks for it. A job fails on a surviving mutant, a demonstration that could not be judged or did not hold, fewer demonstrations held than patches given, or an empty share. Never on every pull request, because a full run costs hours of runner time ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md), [ADR-0124](./docs/adr/engineering/0124-each-kind-of-test-runs-in-a-workflow-of-its-own.md)) |
| `chart` | `packaging/`, tool pins | `helm lint --strict` with the chart's required values from `packaging/chart/ci/required-values.yaml`, since its values schema refuses a render without them |
| `chainsaw` | By hand with a version. Its pull-request trigger, on `packaging/` and the workflow itself, is off while the refactor [ROADMAP.md](./ROADMAP.md) records is under way | The chainsaw suite as ADR-0052 states it. On a pull request it deploys `main`'s `latest` images, by the four digests it resolves at its start and logs, refusing a set built from different commits or holding an image whose revision label names none, and reporting that it did not run when no `latest` images exist. Started by hand it deploys the images published for the version it names, and fails when one is missing |
| `release` | A release | Builds, pushes and signs every image by digest with keyless signing, the build action tagging each `latest` as well, so a release moves `latest` to its own commit's images, packages the chart, refusing one whose `version` or `appVersion` is not the release version, pushes and signs it, and builds the command-line program, `cli/`, as `mediated-mailbox-cli-<os>-<arch>` for linux and darwin on amd64 and arm64, signs each binary keylessly into a Sigstore bundle, and attaches each binary and its bundle to the release ([ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md), [ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md), [ADR-0088](./docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)). Once the binaries are attached it runs the key-generation check of the release, which downloads every attached binary, verifies each bundle against this workflow's identity at the release's tag, sees a copy with one byte changed refused, runs the linux amd64 binary's `keygen` subcommand and has `go tool keypaircheck` seal to and open with the pair it wrote ([ADR-0088](./docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)). The release cut itself, release-please's pull request, writes the release version into `process/version` and into the chart's `version` and `appVersion` through the `extra-files` of `release-please-config.json`. Every release builds every image, including a release cut by documentation alone, because the chart it publishes points at images of that version |
| `lint` | Every pull request | The repository's existing hygiene checks, `commit-messages`, commitlint over the branch commits, and `commit-taxonomy`, which derives every header Renovate and release-please can emit and lints it, requires each to be true of its file, and checks a pull request's headers against its diff ([ADR-0141](./docs/adr/engineering/0141-commit-header-type-sizes-release-scope-names-surface.md)). `commit-taxonomy` carries no path filter, `needs:` or `if:`, because a skipped job satisfies a required check. Inside it, the steps that judge the tree rather than the pull request run only when the pull request changes a file they read, as a step of the job decides, and on every scheduled or manual run, so a change to Go code alone that moves a dependency file into or out of a binary's build graph is caught by the weekly run rather than on its pull request, while the steps over the pull request's own commits run on every pull request |
| `pr-title` | Every pull request, on open, edit, synchronize and reopen | commitlint over the pull request title, the string that lands on `main` for a multi-commit pull request ([ADR-0141](./docs/adr/engineering/0141-commit-header-type-sizes-release-scope-names-surface.md)). Never gated, for the same reason |
| `pr-labels` | Every pull request, on open, edit, synchronize, reopen, label and unlabel | Sets the pull request's `component:` labels to exactly the components its diff touches, read from the [component table](#components) at the pull request's head, creating a missing label in the component color. A new component needs only its table row. A closed or merged pull request's labels are left alone, decided in the script from the event's pull request state, because the run is that pull request's own old head and would put back a label since renamed. A pull request merged before the guard landed keeps its old copy, so its labels are moved with the workflow disabled for the window. A pull request from a fork cannot write labels, so its run fails |
| `renovate` | A schedule | Dependency updates |

A job skipped by its own condition counts as passed under a required check, which is why the three
checks that gate the commit vocabulary carry no condition. No test workflow carries a job
condition either. Its triggers alone decide when it runs, and the workflows that gate nothing,
`go-property-deep`, `go-crash-deep` and `mutation-demonstrations`, are never made required checks
([ADR-0124](./docs/adr/engineering/0124-each-kind-of-test-runs-in-a-workflow-of-its-own.md)).

### Tools and versions

- **Every tool and dependency is at its latest version** unless a document explicitly states
  otherwise. The exceptions the documents state are minimums that latest already meets, and the
  TypeScript copy the browser's type generation needs
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
  read the `integration`, `banproof` and `gmail_live` build tags so every Go file is analysed, run
  tests with the gating run's property settings, add the oxc extension for the browser, open
  generated files read-only, and give the workflow, chart and chainsaw files their schemas.

## Repository process

- `docs` is a visible release type here, so a documentation pull request proposes a release when
  merged. Expected, not accidental. The commit vocabulary, the closed set of types and scopes a
  commit header may carry, the pairing rule between them and what each type does to a release, is
  [.claude/rules/commits.md](./.claude/rules/commits.md)'s, decided by
  [ADR-0141](./docs/adr/engineering/0141-commit-header-type-sizes-release-scope-names-surface.md).
  The `commit-messages`, `pr-title` and `commit-taxonomy` checks of the
  [workflow table](#ci-workflows) gate every pull request on it.
- **Tickets.** A ticket ([Glossary](./DESIGN.md#glossary)) is cut from the body of the template at
  [.github/ISSUE_TEMPLATE/ticket.md](./.github/ISSUE_TEMPLATE/ticket.md), whose header line and
  sections are its format and whose placeholders say what fills each slot. Units are cut for finish
  lines, tickets for parallel work inside a unit, so a ticket takes a slice of the unit's body and
  says what stays with the unit's other tickets. Its header line names the one unit it serves and
  the value increment or production point its work belongs in. A production point's issue takes
  whatever belongs to the point, such as its release, the verification of that release, preparatory
  work and fixes, and never work of a later point's increments, a point's increments being those
  before it and after any previous point. Before a point's release is cut, every test and lint
  workflow, the mutation demonstrations included, is run on `main`, and once it is cut no `feat` or
  breaking change is placed under the point. Each value increment has one issue, titled by the
  increment's name. Each production point has one too, titled by the point's name and created only
  once a ticket of the point's increments has started. Each lists its tickets by unit with each
  ticket's State, and every ticket is a sub-issue of the issue its work is placed in. A point's
  issue can also hold a checklist, and tickets of the operator's deployment repositories as
  sub-issues. These issues and [#118](https://github.com/ppat/mediated-mailbox-mcp/issues/118), of
  which they are sub-issues and which lists them with their status, their units and their ticket counts, are the
  epics ([Glossary](./DESIGN.md#glossary)). The issues of a point's increments close together once
  ROADMAP.md on `main` lists every unit of those increments as delivered and every ticket under them
  is built. Closing the point's issue crosses the point, which the operator alone decides, and the
  first work after it is the collapse of ROADMAP.md that its [preamble](./ROADMAP.md) describes. A
  unit recut in ROADMAP.md recuts its open tickets. Its title says what lands, in the words the pull
  request title will use without the commit type and scope, followed by the word unit and the unit's
  identifier in parentheses. Its links are full URLs to files on `main`. A discovery is a ticket
  under the unit whose mechanism it concerns, whether or not the unit is delivered, and work no unit
  covers gets its unit in ROADMAP.md first, since what is built and in what order has no other home
  and documents change before code. No ticket is added to a closed issue. The epics' bodies are
  written only by the control session that [.claude/loop.md](./.claude/loop.md) runs, in the formats
  of [roadmap.md](./.github/ISSUE_TEMPLATE/roadmap.md),
  [value-increment.md](./.github/ISSUE_TEMPLATE/value-increment.md) and
  [production-point.md](./.github/ISSUE_TEMPLATE/production-point.md) beside the ticket template. It
  also creates the epics, closes the increments' issues, and moves a ticket whose work is placed in
  another increment or point to that issue, as its `reconcile-epics` skill lays out. A ticket the
  operator holds carries the State `hold`, and is not launched until the operator lifts the hold.
  The control session marks a ticket in-progress when it launches the ticket's session, and built
  only once the ticket's pull request has merged, so no dependent ticket starts before its blocker
  is on `main`. A session that cuts a ticket links it as a sub-issue of the issue its work is placed
  in and sends the control session its row. The control session derives every State again on each
  iteration, so a change made while none runs is caught when it next runs.
- **No real mail content in issues or pull requests.** A subject, a sender, a body or an address
  from the real mailbox goes into no issue or pull request, nor a comment on either, because the
  repository is public.
- **A ticket is done** when its pull request has merged with CI green, the unit's acceptance in
  [ROADMAP.md](./ROADMAP.md) is met for the part keyed to what the ticket lands, and every document
  its work touched is updated in that pull request, with its GitHub labels matching its diff. The
  pull request closes the ticket, and the one at which the unit meets its acceptance in ROADMAP.md
  at its finish line also carries the unit's move to the roadmap's delivered register. A closed
  ticket means its work has merged to `main`. Released and deployed are later states, tracked apart
  in ROADMAP.md.
- **A builder subagent builds a pull request through the `builder` skill,** launched with the
  Skill tool and used in full, following the approach it lays out, never bits and pieces of it. The
  skill holds the brief a builder gets, the order it works in, the proof and gates it runs, and the
  report it hands back.
- **A pull request an agent builds leaves draft** only once the **`adversarial-review` skill**'s
  loop has found nothing that stands, its CI is green, and it merges cleanly onto current `main`.
  The main session launches that skill with the Skill tool and uses it in full, following the
  approach it lays out, never bits and pieces of it. The skill holds who reviews, what every
  review checks and how findings are ruled on.
- **GitHub labels.** A ticket carries `unit:<ID>` for the unit it serves and one
  `component:<directory>` per component of the [component table](#components) it touches, with the
  directory spelled as that table's first column spells it without the trailing slash, so
  `packaging/chart` keeps every segment. A row listing several directories is one component with
  the label of its first, so a change under `packaging/tests/chainsaw/` carries
  `component:packaging/chart`. The pull request that closes
  it carries the unit label and one component label per component its diff touches. A change
  touching no component carries the unit label alone. The author applies the ticket's labels and
  the pull request's unit label at creation. The `pr-labels` workflow of the [workflow
  table](#ci-workflows) sets the pull request's component labels from its diff and keeps them in
  line with it, and the author corrects the ticket's component labels to match the pull request's
  before it merges. A missing GitHub label is created at first use in its key's color, `unit` in
  blue `1D76DB` and `component` in purple `5319E7`. The commit scope never carries the component
  ([ADR-0141](./docs/adr/engineering/0141-commit-header-type-sizes-release-scope-names-surface.md)),
  so the component labels are the one channel that does. Tickets carry no other GitHub label, and
  no milestone or project. Each epic carries the label `epic`.
