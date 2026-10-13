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
Kubernetes-specific and speaks in the platform's terms.
[ADR-0051](./0051-environment-contract.md) binds it in exactly one limited form, which is what
the chart may assume about the user's cluster.

## Decision

- **One Helm chart stands up every deployable** ([ADR-0049](./0049-image-per-component-lockstep.md))
  and its project-owned wiring, referencing the per-deployable images at the lockstep version.
- **The chart deploys this project's workloads and nothing else, and what the system depends on
  but does not own arrives as user-supplied inputs.** Postgres and the Secrets holding the
  system's secrets ([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)) arrive as
  Helm values or pre-existing Secrets. A pre-existing Secret can come from anything, and the chart
  reads only the keys it names in it. A secret value the system generates and must keep across
  upgrades, such as the mediator's bearer token, comes from a Secret the user names, or else from
  one Secret the chart creates once, keeps on uninstall, and reads back on every later render rather
  than generating it again. Policy is not a chart input. It lives in the
  database, and it is imported from a file and exported to one through the UI
  ([ADR-0004](../classification/0004-sender-list-decides.md),
  [ADR-0041](./0041-policy-as-immutable-snapshots.md)). Beyond core Kubernetes, neither the chart
  nor its tests assume anything about the cluster they land on, so there is no external-secrets
  and no cert-manager. This is
  [ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)'s boundary made concrete,
  since a chart that accepts only values and pre-existing objects structurally cannot know the
  secret machinery. The one exception is the Prometheus Operator's resources, the alerting rules of
  [ADR-0077](../operability/0077-conditions-raised-as-alerting-rules.md) and a scrape target per
  deployable, which the chart renders when the cluster serves the operator's API, or when a value
  forces them on or off, together with a Grafana dashboard over the series the deployables emit,
  rendered only when a value switched off by default turns it on. Rendering a resource only where
  its API exists assumes nothing about the cluster.
- **Each deployable's configuration is rendered as
  [ADR-0078](./0078-configuration-layers-through-an-owned-library.md) requires.** The chart renders
  one configuration file per process, all of them in one ConfigMap, from one structured values tree
  in which a value several processes read is given once. No value is coerced before the process
  reads it, because the chart's values schema refuses a value Helm's YAML parser has already turned
  into another type, such as an unquoted `no` read as a boolean. The chart gives each deployable
  only its own environment variables, sets `enableServiceLinks: false`, and restarts a deployable's
  pods when its own file changes, and not when another process's does. For the worker that runs
  every background job kind
  ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)), a change to any
  job kind's configuration therefore restarts every job it runs, and each resumes as a restart
  allows ([ADR-0051](./0051-environment-contract.md)).
- **Project-owned configuration and policy hardening ship in the chart, and their enforcement is
  the platform's.** The pod security contexts are core fields and comply with the hardening
  posture ([ADR-0028](../operability/0028-trust-anchor-hardening.md)). Admission machinery and
  policy enforcement belong to the cluster the chart lands on. Being deployed somewhere does not
  mean the controls hold there, and the verification catalogue, not the chart, carries the proof
  obligations.
- **Each workload gets only what its process uses.** Each pod mounts only the secret keys its
  process reads, runs with a read-only root filesystem and no ServiceAccount token, and is given
  resource requests the user can override and no limits, which the user may set. Exposure outside
  the cluster is the user's choice of an Ingress or a Gateway API HTTPRoute, for the mediator and the
  UI each, with the user's annotations passed through, and nothing is exposed until the user enables
  one.
- **Tests travel with the chart.** They are standard Helm tests in this repository, unit tests that
  render the chart, rule unit tests over the alerting rules it ships, and a chainsaw suite,
  declarative Kubernetes end-to-end tests, that deploys the chart onto an ephemeral kind cluster and exercises it. The
  chainsaw harness deploys the chart via flux, which its workflow installs on the cluster as
  harness machinery, not something the chart assumes exists. The suite builds no images. It
  deploys the chart with images already published, `main`'s `latest` images on a pull request, or
  the images of the release version a run started by hand names. `latest` has two writers. `main`
  publishes every image as `latest` whenever a push changes what an image is built from or the
  workflow that builds them, all four built together from one commit and signed as a release's
  are, and a release's own image builds tag its images `latest` as they push them. Either way the
  four come from one commit, which keeps the lockstep of
  [ADR-0049](./0049-image-per-component-lockstep.md). A set a failed write leaves mixed, or a
  `latest` a release left behind `main`, stays until the next push that changes what an image is
  built from or a build started by hand. The version the release cut writes is no reason for
  `main` to build, since the release publishes that commit's images as `latest` itself. A pull
  request's run deploys the one set of four it resolved at its start, and refuses a set built from
  different commits or holding an image that names no commit. The suite runs when the packaging or
  the suite changes, never because a deployable changed. A pull request's run that finds no
  `latest` images reports that it did not run rather than passing, a run started by hand whose
  version lacks any image fails, and a suite with no tests fails.
- **Distribution is OCI, in the same registry as the images, at the lockstep version,** with no
  hosted chart repository and no index.
  [ADR-0028](../operability/0028-trust-anchor-hardening.md)'s signed-and-verified rule extends
  to the chart as another signed OCI artifact under the same machinery. Helm's chart `version`
  and `appVersion` are both set to the release version, so the chart's version is the app's
  version ([ADR-0049](./0049-image-per-component-lockstep.md)).

## Alternatives considered

- **A hosted chart repository with an index.** No case was tabled for it. Rejected: OCI in the
  images' registry serves the artifact with machinery already in place, and flux and renovate
  consume OCI chart references, so a hosted repository would be a channel for nothing.
- **An independently versioned chart.** No case was tabled for it. Rejected: everything moves in
  lockstep ([ADR-0049](./0049-image-per-component-lockstep.md)), and an independently versioned
  chart recreates the compatibility matrix one artifact over.

## Consequences

- The chainsaw suite on a bare kind cluster is where the no-assumptions edge is exercised. A
  chart template that silently depends on cluster machinery the chart does not ship fails the
  install there.
- A pull request's run deploys the images of a commit already on `main`, and configuration is
  strict ([ADR-0078](./0078-configuration-layers-through-an-owned-library.md)). So a pull request that
  adds a configuration key to a deployable and renders it in the chart in the same change fails
  the suite against the older `latest`. The deployable's change lands first, admitting the new key
  in the chart's values schema, since the chart's tests compare the keys the schema admits with
  the deployable's configuration type, and a later change renders it in the chart.
- A `latest` image reports the version of the newest release at or before the commit it was built
  from, because the release cut writes the version every binary reports
  ([ADR-0078](./0078-configuration-layers-through-an-owned-library.md)). Its revision label names
  that commit.
- A hand edit to the version constant alone does not rebuild `latest` from `main`. Only the
  release cut writes it, and the release builds that commit's images itself.
- A renderer that cannot read the cluster, such as `helm template` and Argo CD, which renders
  through it, sees no existing Secret and no Prometheus Operator API. It would generate the chart's
  own Secret anew on every render, so under such a renderer the user names a Secret for each value
  that Secret would hold, and the chart renders the operator's resources only when told the API
  exists or when a value forces them.
- Assumptions about other components: the release process sets the chart `version` and
  `appVersion` to the release version, the registry serves OCI chart artifacts and the signing
  machinery covers them as it covers images, and the consuming platform supplies Postgres and the
  Secrets at deploy time. Accounts are connected and policy is imported through the UI once the
  system runs ([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md),
  [ADR-0004](../classification/0004-sender-list-decides.md)).
