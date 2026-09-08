# 0054. One repository, flat at the top, one module; published names follow `mail-<role>`

**Status:** Accepted ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Everything versions and deploys in lockstep
([ADR-0049](./0049-image-per-component-lockstep.md)), which is trivially natural inside one
repository and awkward across several. The naming the landed documents carried (`mail-mediator`,
`mail-ui`) was placeholder until a convention existed — and more packages and components will
appear before design and implementation finish, so what needs settling is a convention that
extends to things not yet conceived, not names for the first few. The project is also intended
for eventual open sourcing; nothing at this stage plans for it, but names published early are
names kept.

## Decision

- **Everything lives in this one repository:** the deployables' code, the shared libraries, the
  deployment artifact, the system-level tests, and the document set.
- **Published names follow one convention.** Deployables and their images share one name:
  `mail-` plus the deployable's one job in a plain English word — `mail-mediator`, `mail-ui`,
  `mail-backfill`, `mail-sync`, `mail-organize`, `mail-heuristics`. Shared libraries are
  `mail-<concern>`, with the shared pure library `mail-core`
  ([ADR-0050](./0050-shared-code-pure-or-narrow.md)). The deployment artifact is named for the
  project: `mediated-mailbox`. A future component gets its name mechanically: name the role,
  prefix it. If a package registry already holds a published library's name, an alternate is
  picked; the convention holds.
- **The top level is flat.** Each deployable and each shared library is a top-level directory
  under its bare role or concern name (the repository scopes them; the prefix binds published
  artifacts); `packaging/` holds deployment formats, with the chart under it; `tests/` holds
  the system-level suites; `docs/` as today. Deployable versus library is a convention — a
  deployable's directory carries its image build — not a structural fact, chosen knowingly.
- **One module at the repository root; each top-level directory is a package tree within it.**
  Deployables import the shared libraries and never each other — the cross-deployable import
  prohibition is a checked rule in CI, the source-level half of
  [ADR-0049](./0049-image-per-component-lockstep.md)'s own-code-only images.
- **Component tests live inside their component's directory** and never ship
  ([ADR-0049](./0049-image-per-component-lockstep.md)'s final-stage rule). The code-test
  trigger's path filter is an allow-list enumerating the code directories — one entry added per
  new directory, deliberately, rather than a pattern that silently widens.
- **The UI's directory holds both halves:** the server side as the package tree, the browser
  app as a subdirectory within it ([ADR-0042](./0042-implementation-stack.md)).

## Alternatives considered

- **Nested grouping directories** (a components directory and a libraries directory). The case
  for it: the deployable-versus-library distinction becomes structural, and path filters can
  use one wildcard forever. Rejected by the operator for the flat form: less structure to
  understand, and gating by path still works — with the two costs accepted knowingly (an
  allow-list edit per new directory; the distinction living in convention).
- **A module per component, stitched by a workspace.** The case for it: stronger boundaries
  enforced by the module system itself. Rejected: real ceremony for boundaries the checked
  import rules already hold ([ADR-0042](./0042-implementation-stack.md)'s deputized
  enforcement).
- **Names picked ad hoc as components appear.** No case was tabled for it. Rejected on the
  operator's requirement: the convention must extend to components not yet conceived, so the
  rule is settled once rather than re-argued per name.

## Consequences

- The landed documents' component names follow the convention — the four batch workloads carry
  their `mail-` names, and the reorganization workload is `mail-organize`, while the
  reorganization *plan* vocabulary is unchanged: the plan names the artifact, not the workload.
- The repository's own name is unchanged; whether it should one day match the artifact naming
  is an open question for the open-sourcing phase, deliberately not decided here.
- Assumptions about other components: the code-test workflow's allow-list is updated when a
  directory is added; the import rules are enforced in CI.
