# 0052. The Kubernetes deployment is one Helm chart that stands up everything and assumes nothing about the cluster

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[O3](../../../USE_CASES.md#o3--survives-its-failure-modes),
[O6](../../../USE_CASES.md#o6--deployable)

## Context

The system runs on Kubernetes
([USE_CASES.md](../../../USE_CASES.md#governing-constraints)). The application knows nothing
about any platform ([ADR-0051](./0051-environment-contract.md)), so something outside it must
turn the per-deployable images ([ADR-0049](./0049-image-per-component-lockstep.md)) into a
running system. This record decides that mechanism's shape for Kubernetes. It is deliberately
Kubernetes-specific and speaks in the platform's terms;
[ADR-0051](./0051-environment-contract.md) binds it in exactly one limited form — what the chart
may assume about the user's cluster.

## Decision

- **One Helm chart stands up every deployable** ([ADR-0049](./0049-image-per-component-lockstep.md))
  and its project-owned wiring, referencing the per-deployable images at the lockstep version.
- **The chart deploys this project's workloads and nothing else; what the system depends on
  but does not own arrives as user-supplied inputs.** Postgres, the credential Secrets
  ([ADR-0038](../operability/0038-credentials-as-mounted-files.md)), and the policy
  configuration arrive as Helm values or pre-existing ConfigMaps and Secrets — the policy as a
  user-supplied ConfigMap or mounted-file reference. Beyond core Kubernetes, neither the chart
  nor its tests assume anything about the cluster they land on — no external-secrets, no
  cert-manager. This is ADR-0038's boundary made concrete: a chart that accepts only values and
  pre-existing objects structurally cannot know the secret machinery.
- **Project-owned policy objects ship in the chart; their enforcement is the platform's.** The
  egress NetworkPolicy ([ADR-0014](../operability/0014-lan-only-transport.md)) is core
  Kubernetes and ships; pod security contexts are core fields and comply with the hardening
  posture ([ADR-0028](../operability/0028-trust-anchor-hardening.md)). Admission machinery and
  policy enforcement belong to the cluster the chart lands on: being deployed somewhere does not
  mean the controls hold there — the verification catalogue, not the chart, carries the proof
  obligations.
- **Tests travel with the chart:** standard Helm tests in this repository, and a chainsaw
  suite — declarative Kubernetes end-to-end tests — that deploys the chart onto an ephemeral
  kind cluster and exercises it. The chainsaw harness deploys the chart via flux because the
  reusable workflow it runs under already stands flux up — harness machinery, not something the
  chart assumes exists.
- **Distribution is OCI, in the same registry as the images, at the lockstep version** — no
  hosted chart repository, no index.
  [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s signed-and-verified rule extends
  to the chart as another signed OCI artifact under the same machinery. Helm's chart `version`
  and `appVersion` are both set to the release version — the chart's version is the app's
  version ([ADR-0049](./0049-image-per-component-lockstep.md)).

## Alternatives considered

- **A hosted chart repository with an index.** No case was tabled for it. Rejected: OCI in the
  images' registry serves the artifact with machinery already in place, and flux and renovate
  consume OCI chart references — a hosted repository would be a channel for nothing.
- **An independently versioned chart.** No case was tabled for it. Rejected: everything moves in
  lockstep ([ADR-0049](./0049-image-per-component-lockstep.md)), and an independently versioned
  chart recreates the compatibility matrix one artifact over.

## Consequences

- The chainsaw suite on a bare kind cluster is where the no-assumptions edge is exercised: a
  chart template that silently depends on cluster machinery the chart does not ship fails the
  install there.
- NetworkPolicy enforcement is not this project's to test — the operator's ruling: enforcement
  in the running environment is proven continuously by the platform's standing network-policy
  falsifiability probe. The chainsaw suite's kind cluster runs a CNI that does not enforce
  NetworkPolicy and this project installs none, so the chart's own allowlist — provider APIs,
  the database, DNS, and nothing else
  ([ADR-0014](../operability/0014-lan-only-transport.md)) — has no injection and stands as
  review discipline on the chart template. The egress-refusal verification row parks with
  exactly that standing.
- Assumptions about other components: the release process sets the chart `version` and
  `appVersion` to the release version; the registry serves OCI chart artifacts and the signing
  machinery covers them as it covers images; the consuming platform supplies Postgres, the
  credential Secrets, and the policy data at deploy time.
