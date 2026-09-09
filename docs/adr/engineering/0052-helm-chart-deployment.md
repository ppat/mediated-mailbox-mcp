# 0052. The first deployment mechanism is a Helm chart that stands up everything and assumes nothing about the cluster

**Status:** Accepted

## Context

The project may someday have several deployment mechanisms; the first and only one built is
Kubernetes-based, and the others are deferred indefinitely. The application itself knows nothing
about any platform ([ADR-0051](./0051-environment-contract.md)); this record is the other half
of that boundary: the deliberately Kubernetes-specific deliverable this repository ships, and
where its edges are. The application-side rule still binds here in one limited form — the chart
may not assume anything about the *user's* cluster beyond core Kubernetes — but the deliverable
itself is a Helm chart and is described as one.

## Decision

- **One Helm chart, at `packaging/chart/`, stands up all six deployables** and their
  project-owned wiring, referencing the per-deployable images
  ([ADR-0049](./0049-image-per-component-lockstep.md)) at the lockstep version.
- **The chart contains the workloads, never the platform's services.** Postgres (a managed
  cluster, per the design), the credential Secrets
  ([ADR-0038](../operability/0038-credentials-as-mounted-files.md)), and the real policy
  configuration arrive as user-supplied inputs: Helm values, or pre-existing ConfigMaps and
  Secrets — the policy as a user-supplied ConfigMap or mounted-file reference. Beyond core
  Kubernetes the chart assumes nothing about the cluster it lands on — no external-secrets, no
  cert-manager. This is ADR-0038's boundary made concrete: a chart that accepts only values and
  pre-existing objects structurally cannot know the secret machinery. It also serves the
  open-sourcing intention for free, without planning for it.
- **Project-owned policy objects ship in the chart; their enforcement is the platform's.** The
  egress NetworkPolicy ([ADR-0014](../operability/0014-lan-only-transport.md)) is core Kubernetes
  and ships; pod security contexts are core fields and comply with the hardening posture
  ([ADR-0028](../operability/0028-trust-anchor-hardening.md)). Admission machinery and policy
  enforcement belong to the cluster the chart lands on: being deployed somewhere does not mean
  the controls hold there — the verification catalogue, not the chart, carries the proof
  obligations.
- **Tests travel with the chart:** standard Helm tests in this repository, and a chainsaw suite
  that deploys the chart onto an ephemeral kind cluster and exercises it, referencing nothing of
  the operator's platform repositories. The chainsaw harness deploys the chart via flux because
  the reusable workflow it runs under already stands flux up — harness machinery, not something
  the chart assumes exists.
- **Distribution is OCI, in the same registry as the images, at the lockstep version** — no
  hosted chart repository, no index.
  [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s signed-and-verified rule extends
  to the chart as another signed OCI artifact under the same machinery. Helm's chart `version`
  and `appVersion` are both set to the release version — the chart's version is the app's
  version ([ADR-0049](./0049-image-per-component-lockstep.md)).
- **Consumption is the platform side's business, as for any third-party chart:** flux and
  renovate handle OCI chart references, and the consuming platform repositories wire the secret
  machinery and the rollout around the chart exactly as they would for any open-source chart.

## Alternatives considered

- **The deployment module living in the operator's platform repositories** — the form factor the
  operator's other projects use: code in the project repository, infrastructure as a module in
  the platform repositories. The case for it: it is the estate's established pattern. Rejected
  for this project: a self-contained chart with its own tests keeps the product deployable and
  testable with no reference to the operator's repositories — and if this form factor proves the
  better one, the other projects can switch to it later; migrating them is out of scope here.
- **A hosted chart repository with an index.** No case was tabled for it. Rejected: OCI in the
  images' registry serves the artifact with machinery already in place, and flux and renovate
  consume OCI chart references — a hosted repository would be a channel for nothing.
- **An independently versioned chart.** No case was tabled for it. Rejected: everything moves in
  lockstep ([ADR-0049](./0049-image-per-component-lockstep.md)), and an independently versioned
  chart recreates the compatibility matrix one artifact over.

## Consequences

- The chainsaw suite on a bare kind cluster is where the no-assumptions edge is exercised: a
  chart template that silently depends on cluster machinery the chart does not ship fails the
  install there, from the first chart onward.
- NetworkPolicy enforcement is deliberately not tested by this project: kind cannot enforce
  NetworkPolicy, and enforcement in the running environment is proven continuously by the
  platform operator's standing network-policy falsifiability probe. The egress-refusal
  verification row parks with that standing reason.
- Assumptions about other components: the release process sets the chart `version` and
  `appVersion` to the release version; the registry serves OCI chart artifacts and the signing
  machinery covers them as it covers images; the consuming platform supplies Postgres, the
  credential Secrets, and the policy data at deploy time.
