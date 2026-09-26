# 0079. Every secret other than an account's provider credential arrives as a mounted file

**Status:** Accepted (supersedes [ADR-0038](./0038-credentials-as-mounted-files.md), jointly with
[ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)) ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The deployables hold secrets besides an account's provider credential. They are the database
password, TLS material, the client surface's bearer token material, the key behind the UI's request
token ([ADR-0061](./0061-ui-browser-security-posture.md)), and the keys that seal and open account
credentials ([ADR-0081](./0081-credentials-sealed-to-a-public-key.md)). An account's provider
credential is not among them. It lives in the database
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)). These secrets must
reach the processes that use them without living in the configuration repository, and survive
restarts.

## Decision

- **Files, not environment variables.** Environment variables leak through `/proc`, crash dumps,
  and child-process inheritance. File mounts do not.
- **No deployable fetches these secrets itself.** Something outside the deployables delivers the
  files.
- **Only the deployables that use a secret can read its file.** In particular, only the
  deployables that open credentials to call a provider, the mediator, backfill, delta sync and the
  reorg workload, can read the private key that opens an account's credential.

## Alternatives considered

- **Secrets in environment variables.** No case was tabled for them. Rejected for the leak surfaces
  above.
- **Secrets baked into the configuration repository (sealed or encrypted).** No case was tabled for
  it. ADR-0038 rejected it on an argument about rotation write-back, which concerns account
  credentials only, and no argument was made for these secrets either way. The rule carries over
  from ADR-0038 unchanged.

## Consequences

- Delivery happens outside the deployables that read the files. If it stalls, the mounted files go
  stale. Accepted.
- Assumptions about other components. Something outside the deployables delivers the files and
  enforces the read restriction.
