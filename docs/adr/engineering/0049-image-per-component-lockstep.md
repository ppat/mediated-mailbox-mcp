# 0049. One image per deployable, all moving in lockstep

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

The system is deliberately several workloads
([ADR-0022](../operability/0022-four-workloads.md)), the trust anchor's runtime is hardened to
carry nothing beyond what it needs
([ADR-0028](../operability/0028-trust-anchor-hardening.md)), and every dependency inside the
process holding full-mailbox credentials is attack surface. Packaging is where those commitments
either extend to the artifact layer or quietly stop at the process boundary.

## Decision

- **One image per deployable.** The default position is an image per deployable, held until
  reality bites and argues for a different cut. The deployables today are the mediator, the UI,
  and the four batch workloads; the other named components — the gate, the classifier, the
  scanner, the scan gate, the rate limiter — are not deployables and ride inside deployables as
  code. Each image bakes only its deployable's own code plus the shared libraries it imports —
  one deployable's image cannot contain another's code, so "nothing beyond what it needs" reads
  all the way down to image contents.
- **Everything moves in lockstep.** Images and the deployment artifact that consumes them carry
  one version — the release's. Deployed-together always means built-together; no
  version-compatibility matrix exists to maintain.
- **Images are minimal and hardened:** multi-stage builds producing one static binary per image
  on a minimal base, no shell — the runtime posture
  [ADR-0028](../operability/0028-trust-anchor-hardening.md) commits to, which the chosen
  stack's native form ([ADR-0042](./0042-implementation-stack.md)) makes the default rather
  than an achievement. Tests never ship: the final stage copies only runtime artifacts.
- **Images are published signed**, extending
  [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s signed-and-verified rule to every
  deployable's artifact.

## Alternatives considered

- **One image for everything, the workload selected by entrypoint arguments.** The case for it:
  a single build and signature, and version skew made impossible by construction. Rejected: it
  buys that convenience by putting all code everywhere — the content scanner's body-reading
  machinery inside the UI's container, the mutation engine inside the heuristics job — which is
  the braid at the artifact layer and the inverse of the minimal-contents posture.
- **Independent per-deployable versioning.** The case for it: deployables could release on their
  own cadence. Rejected: it creates a version-compatibility matrix that a single-operator
  system would never keep honest, for a cadence freedom nothing here needs.
- **One shared image for the four batch workloads.** The case for it: fewer artifacts, and the
  batch workloads overlap heavily through the shared libraries anyway. Rejected by the default
  heuristic: after the first parameterized build the marginal cost of an image is near zero,
  and per-deployable images keep every container's contents exact. The overlap through declared
  shared libraries is the sanctioned form of sharing and needs no shared image to exist.

## Consequences

- Adding a deployable means adding an image — one entry in the parameterized build, per the
  default-until-reality-bites heuristic; switching the cut later is a packaging change, not a
  redesign.
- Packaging isolation is never asked to carry credential isolation: which process can read the
  credential files stays a deployment-construct property
  ([ADR-0038](../operability/0038-credentials-as-mounted-files.md)), whatever the images look
  like.
- Assumptions about other components: the build-and-publish machinery is parameterized per
  deployable; the deployment artifact consumes every image at the same lockstep version.
