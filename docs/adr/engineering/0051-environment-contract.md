# 0051. The app knows its environment contract, never its platform

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[O2](../../../USE_CASES.md#o2--observable)

## Context

The project expects to grow additional deployment mechanisms, so nothing in the application may
bind to the one that exists first. The contracts-only pillar already forbids components knowing
each other beyond contracts, and the same discipline has to hold one level out, between the
application and whatever runs it. And
[ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md) draws the sharpest piece of
the boundary, that secrets arrive as mounted files and how they got there is not the
application's business.

## Decision

- **The app does not know its platform.** Not which orchestrator runs it, not what invokes its
  batch jobs, not what network or cluster surrounds it. Everything arrives through the
  environment contract, which has these parts.
  - Configuration as files, environment variables and flags, layered as
    [ADR-0078](./0078-configuration-layers-through-an-owned-library.md) decides.
  - Secret material, always as files
    ([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)).
  - Plain HTTP health and readiness probes.
  - Structured logs to standard output.
  - Prometheus-format metrics on a metrics endpoint.
  - The standing assumption that any process can be killed at any point.
- **An unavoidable platform assumption gets the port treatment.** That is a neutral contract on
  the application side with the environment-specific implementation behind it, swappable at
  deploy, configuration, or run time, which is
  [ADR-0010](../provider/0010-one-provider-port.md)'s pattern, generalized. No platform
  assumption requires it. Probes are plain HTTP any healthcheck can hit, and every job resumes
  idempotently whatever starts it, a platform's scheduler or the long-running worker that
  schedules its own jobs
  ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)).
- **Settings are user-switchable within the ranges the design permits, with reasonable
  defaults everywhere. Safety dispositions are never settings.** No configuration flag exists
  that weakens deny-by-default, disables the gate, or skips masking.
  [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released) already forbids an
  override parameter on the client surface, and the same logic holds on the configuration
  surface, where the dangerous knob does not exist rather than defaulting safe.
- **The observability split.** The app emits its metrics on the metrics endpoint, its logs to
  standard output and its audit rows to the database, and the collection, shipping, and
  retention of what it emits are the platform's.

## Alternatives considered

- **A platform-aware application, querying the orchestrator's API for configuration, state, or
  coordination.** No case was tabled for it. Rejected: it binds the application to the platform
  it queries and forecloses the additional deployment mechanisms the project expects to add.
- **Safety toggles for operational flexibility.** No case was tabled for them. Rejected on
  [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)'s own logic, since a
  knob that can weaken the invariant is an override parameter wearing configuration's clothing.

## Consequences

- Any deployment mechanism that can supply files, environment variables, and process lifecycle
  can run the system. Further mechanisms are addable without application changes.
- Assumptions about other components: the platform captures standard output and scrapes the
  metrics endpoint, something delivers configuration and secret files and keeps them current
  ([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md),
  [ADR-0041](./0041-policy-as-immutable-snapshots.md)), and the health and readiness probes are
  HTTP endpoints any healthcheck can drive.
