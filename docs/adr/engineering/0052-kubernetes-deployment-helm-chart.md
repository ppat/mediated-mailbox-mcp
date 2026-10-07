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
  but does not own arrives as user-supplied inputs.** Postgres and the Secrets holding the system's
  secrets ([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)) arrive as Helm
  values or pre-existing Secrets. Policy is not a chart input. It lives in the database, and it is
  imported from a file and exported to one through the UI
  ([ADR-0004](../classification/0004-sender-list-decides.md),
  [ADR-0041](./0041-policy-as-immutable-snapshots.md)). Beyond core Kubernetes, neither the chart
  nor its tests assume anything about the cluster they land on — no external-secrets, no
  cert-manager. This is ADR-0079's boundary made concrete: a chart that accepts only values and
  pre-existing objects structurally cannot know the secret machinery. The exceptions are resources
  the chart renders only when a value switched off by default turns them on: the alerting rules of
  [ADR-0077](../operability/0077-conditions-raised-as-alerting-rules.md), as the Prometheus
  Operator's resource, and the Gateway API's HTTPRoute for a surface the chart exposes. The
  Ingress the chart can render for the same surfaces is core Kubernetes, and is off by default too.
- **Each deployable's configuration is rendered as
  [ADR-0078](./0078-configuration-layers-through-an-owned-library.md) requires.** The chart renders
  each deployable's configuration file from structured values into one ConfigMap, a file per
  deployable holding only the keys that deployable declares, mounts into each deployable's pods only
  its own file, gives each deployable only its own environment variables, sets
  `enableServiceLinks: false`, and restarts a deployable's pods when its own file changes.
- **Project-owned configuration and policy hardening ship in the chart; their enforcement is the
  platform's.** The pod security contexts are core fields and comply with the hardening
  posture ([ADR-0028](../operability/0028-trust-anchor-hardening.md)). Admission machinery and
  policy enforcement belong to the cluster the chart lands on: being deployed somewhere does not
  mean the controls hold there — the verification catalogue, not the chart, carries the proof
  obligations.
- **Tests travel with the chart:** standard Helm tests in this repository, rule unit tests over
  the alerting rules it ships, render tests that render the chart with the pinned Helm and check
  the objects it would hand a cluster, and a chainsaw suite —
  declarative Kubernetes end-to-end tests — that deploys the chart onto an ephemeral kind cluster
  and exercises it. The chainsaw harness deploys the chart via flux, which its workflow installs on
  the cluster as harness machinery, not something the chart assumes exists. On a pull
  request that changes the chart or the suite it runs against the images built from the pull
  request's own tree in its job, so the pull request is proven by its own run, and it never runs on
  a pull request that changes only a deployable. Started by hand, it takes a release version and
  deploys the chart with the images already published for it, and fails when the version lacks one.
  A suite with no tests fails.
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
- Assumptions about other components: the release process sets the chart `version` and
  `appVersion` to the release version; the registry serves OCI chart artifacts and the signing
  machinery covers them as it covers images; the consuming platform supplies Postgres and the
  Secrets at deploy time. Accounts are connected and policy is imported through the UI once the
  system runs ([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md),
  [ADR-0004](../classification/0004-sender-list-decides.md)).
