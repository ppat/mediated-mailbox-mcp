# 0054. Everything the project produces lives in one flat repository, and one convention names what it publishes

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes),
[O6](../../../USE_CASES.md#o6--deployable)

## Context

The deployables ([ADR-0049](./0049-image-per-component-lockstep.md)), the shared libraries
([ADR-0050](./0050-shared-code-pure-or-narrow.md)), and the Helm chart with its test suites
([ADR-0052](./0052-kubernetes-deployment-helm-chart.md)) all need a home, and the things the
project publishes need names. The names are settled as a convention rather than as a list,
because components keep being added throughout design and implementation and the project is
intended for eventual open sourcing.

## Decision

- **Everything the project produces lives in this one repository.** The deployables' code, the
  UI whatever its language, the shared libraries, the packaging, the system test suites, and
  the document set. Consumption and rollout belong to the consuming platform, outside this
  repository.
- **The top level is flat.** Each deployable and each shared library is one top-level
  directory beside `docs/`, `packaging/`, and `tests/`, with no grouping directories. A
  library is told apart from a deployable only by having no Dockerfile and by documentation,
  a convention accepted in place of structure for the simpler layout. The UI's directory
  holds both of its halves ([ADR-0042](./0042-implementation-stack.md)), the browser app as a
  subdirectory, arranged at authoring time. A top-level component may be a family, one shared
  library holding several packages of one concept with depth below its directory
  ([ADR-0050](./0050-shared-code-pure-or-narrow.md)), so the top level stays a flat list of
  components while a component has depth.
- **The Helm chart sits at `packaging/chart/`**, under a common `packaging/` directory so a
  later packaging format lands beside it and nothing moves.
- **Tests sit with what they test.** A deployable's tests live in its own directory, a library's in
  its own, and both stay out of every image under
  [ADR-0049](./0049-image-per-component-lockstep.md)'s runtime-artifacts-only rule. The chart's Helm
  tests live inside the chart and ride inside the published chart artifact, which is Helm's standard
  design and an accepted, knowing exception to keeping tests out of distributions. The chainsaw
  suite tests the assembled system, so it belongs to no single deployable and sits at
  `tests/chainsaw/`.
- **CI path filters are allow lists**, each a short static enumeration of the directories it
  watches, extended by one entry per new component, never an exclusion of everything else.
- **Deployables share code only through the shared libraries and never import each other.** A
  deployable's imports reach its own code, `core/`, the data-access library, and the other narrow
  named exceptions of [ADR-0050](./0050-shared-code-pure-or-narrow.md), whose directories
  [CLAUDE.md](../../../CLAUDE.md#components) names. One short lint rule enforces this, the same
  shape as the core purity check of [ADR-0040](./0040-pure-core-decisions-as-values.md), so
  [ADR-0049](./0049-image-per-component-lockstep.md)'s own-code-only posture holds at the source as
  well as in the image.
- **One convention names everything the project publishes.** A deployable and its image share
  one name, `mediated-mailbox-` plus the deployable's role in one plain word, such as the worker's,
  `worker`, which runs every background job kind. Libraries take the same shape named by their single
  concern, and the shared pure library publishes as `mediated-mailbox-core`. In-repo directories carry the
  bare role, because the repository scopes them and the prefix binds published artifacts. The
  chart stands up the whole system rather than any one deployable, so it carries the project's
  name, `mediated-mailbox`. A library whose chosen name a package registry already holds takes
  an alternate name under the same convention, and images publish under a registry namespace
  and cannot collide. The convention extends to any future component by naming its role.
- **Names inside the code follow a convention that guides, and builders choose them.** The
  convention is a generally applicable pattern that guides naming by the lessons below without
  being overtly prescriptive, presumptive or unnecessarily inhibitive. The operator is presented
  with the chosen names for approval with the pull request that implements them, so
  implementation is never blocked on the operator picking names, and the operator does not name
  each library and directory one at a time. The convention guides and does not dictate, and a
  builder's judgement of fit decides inside it.
  - **Fit comes first.** A name says what the code provides or holds. As guidance from the lessons
    below, a family is named for its concept and a package for what it provides, and one plain word
    is used where one fits, with no abbreviations.
  - **Collisions are a light touch.** A name that clashes with another is a cost to weigh, not a
    ban, because avoiding a name already in use can push toward one that fits worse. A clash is
    avoided only where the two same-named packages would routinely be imported in the same files,
    as Go's `context` would be beside a package named `context`. Otherwise an import alias is an
    acceptable price for a name that fits.
  - **The ban on `util`, `common` and `helpers` stays**
    ([CLAUDE.md](../../../CLAUDE.md#inside-a-component)), since those names fail on fit.
  - **Fixed names stay fixed.** They are `core` for pure code, a deployable's entry package,
    whose name is the same in every deployable and is the one
    [CLAUDE.md](../../../CLAUDE.md#inside-a-component) gives, and `cmd/<name>` for commands.
  - **A package that moves keeps its name** where that name still fits, to keep churn down.
  - **Builders choose names, and the operator approves them** in the pull request that introduces
    them, which lists each name with what it holds and the alternatives considered.

  The lessons it draws on. Names approved one at a time, with no shared pattern behind them, made
  every name a ticket-local decision. Collisions between package names forced import aliases in
  non-test code, `dbconnectcore` 4 times, `ratecore` 4, `credentialcore` 3, `senderclasses` 3 and
  `clientsetup` 3. A name invented inside a ticket did not last, as `policyload` was invented in a
  ticket, removed, and named by the operator eight days later, and one unit waited a night for
  names. A name that clashes with a standard-library package the same files routinely import costs
  an alias in nearly every file, as `context` would have beside Go's `context.Context`, and
  `runtime` beside Go's `runtime`. The Glossary's own term "account context" proved misleading,
  since "context" reads as Go's `context.Context` and no package holds the bundle it named, so a
  name does not lean on a term just because it exists. A name must fit every caller, so
  `requestcontext` misfit the batch jobs, which serve no request. And `policyload` kept its name
  because `policy` would clash with `core/policy`, which files routinely import beside it, the case
  where a clash was a real cost.
- **The repository keeps its name.** The `-mcp` suffix names the system for its protocol adapter
  even though the serving layer is an API
  ([ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md)), and a rename is nearly free
  whenever taken, so the question waits for the open-sourcing phase.

## Alternatives considered

- **More than one repository.** No case was tabled. It is listed so that the single repository
  is a decision rather than drift.
- **Grouping directories, one for deployables and one for libraries.** The case for it: a
  structural deployable-versus-library distinction and coarse one-directory path filters. Not
  chosen, for the simpler flat layout, with both of its costs accepted, which are allow-list
  filters and a conventional rather than structural distinction.
- **A different naming scheme.** No case for another scheme was tabled.
- **The operator names each library, directory and package.** The case for it: names are taste, and
  taste is the operator's. Not chosen, because approving names one at a time, with no pattern
  behind them, makes every name a ticket-local decision and blocks work while names wait.
- **A naming grammar ruled once, from which a builder derives each name**, a family named by the
  Glossary's noun for its concept, never a standard-library package's name, and each package's name
  unique within its family. The case for it: names follow mechanically, with the operator ruling
  the grammar once. Not chosen as a rule, because a grammar that dictates pushes toward a name that
  fits worse wherever its rule and fit disagree. Its points survive in the convention as guidance.

## Consequences

- A new component costs a directory, one allow-list entry per watching filter, an image entry
  in the parameterized build ([ADR-0049](./0049-image-per-component-lockstep.md)) when it is a
  deployable, and a name the convention produces.
- Assumptions about other components. The per-component build context
  ([ADR-0049](./0049-image-per-component-lockstep.md)) copies a deployable's directory plus
  the shared libraries from this layout. The release machinery publishes images under a
  registry namespace. The import prohibition is checkable at build time.
- The import prohibition between deployables is a control. Its violation injection is
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
