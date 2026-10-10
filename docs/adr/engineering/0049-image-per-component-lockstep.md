# 0049. One image per deployable, all moving in lockstep

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

The system is deliberately several deployables
([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)), the trust anchor's
runtime is hardened to carry nothing beyond what it needs
([ADR-0028](../operability/0028-trust-anchor-hardening.md)), and every dependency inside a process
holding full-mailbox credentials is attack surface. Packaging is where those commitments either
extend to the artifact layer or quietly stop at the process boundary.

## Decision

- **One image per deployable.** The default position is an image per deployable, held until reality
  bites and argues for a different cut. The deployables are the mediator, the UI, and the worker
  that runs every background job kind
  ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)). The other named
  components, the gate, the classifier, the scanner, the scan gate and the rate limiter, are not
  deployables and ride inside deployables as code. The migration step of
  [ADR-0048](../data/0048-forward-only-migrations.md) is not a deployable either. It has an
  image of its own, which carries the migration runner and the migration chain and none of this
  project's Go code. Each image bakes only its deployable's own code plus the shared libraries it
  imports. One deployable's image cannot contain another's code, so "nothing beyond what it
  needs" reads all the way down to image contents. Exactness is a property of the binary and the
  final stage. Go links only the packages a binary imports, and the final stage copies only that
  binary, so which source a build stage copies is mechanism and never widens what the image
  holds.
- **Everything moves in lockstep.** Images, the deployment artifact that consumes them and the
  key-generation binaries attached to the release
  ([ADR-0088](../operability/0088-credentials-sealed-with-hpke-x-wing.md)) carry one version, the
  release's. Deployed-together always means built-together, and no version-compatibility matrix
  exists to maintain.
- **Images are minimal and hardened.** Multi-stage builds produce one static binary per image on
  a minimal base with no shell. That is the runtime posture
  [ADR-0028](../operability/0028-trust-anchor-hardening.md) commits to, which the chosen stack's
  native form ([ADR-0042](./0042-implementation-stack.md)) makes the default rather than an
  achievement. Tests never ship, because the final stage copies only runtime artifacts.
- **Images are published signed**, extending
  [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s signed-and-verified rule to every
  deployable's artifact. The key-generation binaries attached to a release are signed the same
  way.

## Alternatives considered

- **One image for everything, the workload selected by entrypoint arguments.** The case for it:
  a single build and signature, and version skew made impossible by construction. Rejected: it
  buys that convenience by putting all code everywhere, the content scanner's body-reading
  machinery inside the UI's container and the mutation engine inside the heuristics job, which is
  the braid at the artifact layer and the inverse of the minimal-contents posture.
  [ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md) runs reorg apply
  and rollback and the heuristics run as job kinds of one worker, so the mutation engine and the
  heuristics share the worker's image, and code isolation per job kind takes the place of image
  isolation between them.
- **Independent per-deployable versioning.** The case for it: deployables could release on their
  own cadence. Rejected: it creates a version-compatibility matrix that a single-operator
  system would never keep honest, for a cadence freedom nothing here needs.
- **One shared image for the four batch workloads.** The case for it: fewer artifacts, and the
  batch workloads overlap heavily through the shared libraries anyway. Rejected by the default
  heuristic, since after the first parameterized build the marginal cost of an image is near zero,
  and per-deployable images keep every container's contents exact. The overlap through declared
  shared libraries is the sanctioned form of sharing and needs no shared image to exist.

## Consequences

- Adding a deployable means adding an image, one entry in the parameterized build, per the
  default-until-reality-bites heuristic. Switching the cut later is a packaging change, not a
  redesign.
- Packaging isolation is never asked to carry credential isolation. Which process can open a
  stored credential stays a property of which processes receive the private key's file, and
  inside the UI and inside the worker it is a property of their code
  ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)), whatever the images
  look like.
- The worker's image holds the code of every job kind it runs, so the exactness this record keeps
  per deployable is the worker's as a whole, not each job kind's. The alternative of one shared
  image for four batch workloads has no subject, because the background work is one deployable
  ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)).
- Assumptions about other components: the build-and-publish machinery is parameterized per
  deployable, and the deployment artifact consumes every image at the same lockstep version.
