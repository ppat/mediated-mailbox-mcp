# 0054. Everything the project produces lives in one flat repository, and one convention names what it publishes

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes),
[O6](../../../USE_CASES.md#o6--deployable)

## Context

The deployables ([ADR-0049](./0049-image-per-component-lockstep.md)), the shared libraries
([ADR-0050](./0050-shared-code-pure-or-narrow.md)), and the Helm chart with its test suites
([ADR-0052](./0052-kubernetes-deployment-helm-chart.md)) all need a home, and the things the
project publishes need names. The names deserved settling early as a convention rather than as
a list, because components keep being added throughout design and implementation and the
project is intended for eventual open sourcing.

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
  subdirectory, arranged at authoring time.
- **The Helm chart sits at `packaging/chart/`**, under a common `packaging/` directory so a
  later packaging format lands beside it and nothing moves.
- **Tests sit with what they test.** A deployable's tests live in its own directory and stay
  out of its image under [ADR-0049](./0049-image-per-component-lockstep.md)'s
  runtime-artifacts-only rule. The chart's Helm tests live inside the chart and ride inside
  the published chart artifact, which is Helm's standard design and an accepted, knowing
  exception to keeping tests out of distributions. The chainsaw suite tests the assembled
  system, so it belongs to no single deployable and sits at `tests/chainsaw/`.
- **CI path filters are allow lists**, each a short static enumeration of the directories it
  watches, extended by one entry per new component, never an exclusion of everything else.
- **Deployables share code only through the shared libraries and never import each other.**
  A deployable's imports reach its own code, `core/`, the data-access library, and any future
  narrow named exception ([ADR-0050](./0050-shared-code-pure-or-narrow.md)). One short lint
  rule enforces this, the same shape as the core purity check of
  [ADR-0040](./0040-pure-core-decisions-as-values.md), so
  [ADR-0049](./0049-image-per-component-lockstep.md)'s own-code-only posture holds at the
  source as well as in the image.
- **One convention names everything the project publishes.** A deployable and its image share
  one name, `mediated-mailbox-` plus the deployable's one job in one plain word, and the job word for the
  reorganization workload is organize. Libraries take the same shape named by their single
  concern, and the shared pure library publishes as `mediated-mailbox-core`. In-repo directories carry the
  bare role, because the repository scopes them and the prefix binds published artifacts. The
  chart stands up the whole system rather than any one deployable, so it carries the project's
  name, `mediated-mailbox`. A library whose chosen name a package registry already holds takes
  an alternate name under the same convention, and images publish under a registry namespace
  and cannot collide. The convention extends to any future component by naming its role.
- **The repository keeps its current name.** The `-mcp` suffix names the system for its
  protocol adapter even though the serving layer is an API
  ([ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md)), and a rename is nearly free
  whenever taken, so the question waits for the open-sourcing phase.

## Alternatives considered

- **More than one repository.** No case was tabled. The question was raised so the single
  repository would be a decision rather than drift.
- **Grouping directories, one for deployables and one for libraries.** The tabled
  recommendation. Its case was a structural deployable-versus-library distinction and coarse
  one-directory path filters. The operator ruled for the flat form with both costs named and
  accepted, allow-list filters and a conventional distinction.
- **A different naming scheme.** No case for another scheme was tabled. The convention was
  adopted as recommended, with the one substitution the Decision records, organize as the
  reorganization workload's job word.

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
