# 0051. The app knows its environment contract, never its platform

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[O2](../../../USE_CASES.md#o2--observable)

## Context

The project expects to grow additional deployment mechanisms, so nothing in the application may
bind to the one that exists first. The contracts-only pillar
already forbids components knowing each other beyond contracts; the same discipline has to hold
one level out, between the application and whatever runs it. And
[ADR-0038](../operability/0038-credentials-as-mounted-files.md) has already drawn the sharpest
piece of the boundary: credentials arrive as mounted files, and how they got there is not the
application's business.

## Decision

- **The app does not know its platform.** Not which orchestrator runs it, not what invokes its
  batch jobs, not what network or cluster surrounds it. Everything arrives through the
  environment contract: configuration as files and/or environment variables; secret material
  always as files ([ADR-0038](../operability/0038-credentials-as-mounted-files.md)); plain HTTP
  health and readiness probes; structured logs to standard output; Prometheus-format metrics on
  a metrics endpoint; and the standing assumption that any process can be killed at any point.
- **An unavoidable platform assumption gets the port treatment:** a neutral contract on the
  application side, the environment-specific implementation behind it, swappable at deploy,
  configuration, or run time — [ADR-0010](../provider/0010-one-provider-port.md)'s pattern,
  generalized. The honest count of assumptions requiring it today: zero — probes are plain
  HTTP any healthcheck can hit, and batch jobs are exit codes plus idempotent resume, which any
  scheduler can drive.
- **Settings are user-switchable within the ranges the design permits, with reasonable
  defaults everywhere. Safety dispositions are never settings:** no configuration flag exists that
  weakens deny-by-default, disables the gate, or skips masking.
  [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released) already forbids an
  override parameter on the client surface; the same logic holds on the configuration
  surface — the dangerous knob does not exist, rather than defaulting safe.
- **The observability split:** the app emits — the metrics endpoint, logs to standard output,
  audit rows to the database — and the collection, shipping, and retention of what it emits are
  the platform's.

## Alternatives considered

- **A platform-aware application** — querying the orchestrator's API for configuration, state,
  or coordination. No case was tabled for it. Rejected: it binds the application to the
  platform it queries and forecloses the additional deployment mechanisms the project expects
  to add.
- **Safety toggles for operational flexibility.** No case was tabled for them. Rejected on
  [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)'s own logic: a knob
  that can weaken the invariant is an override parameter wearing configuration's clothing.

## Consequences

- Any deployment mechanism that can supply files, environment variables, and process lifecycle
  can run the system; further mechanisms are addable without application changes.
- Assumptions about other components: the platform captures standard output and scrapes the
  metrics endpoint; something delivers configuration and secret files and keeps them current
  ([ADR-0038](../operability/0038-credentials-as-mounted-files.md),
  [ADR-0041](./0041-policy-as-immutable-snapshots.md)); and the readiness probe's contract —
  refuse traffic while a known-sensitive fixture is not denied — is an HTTP endpoint any
  healthcheck can drive.
