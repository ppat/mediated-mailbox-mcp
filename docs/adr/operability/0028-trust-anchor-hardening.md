# 0028. Hardening the trust anchor: minimal pod, detectable drift, evidence that survives compromise

**Status:** Accepted ·
**Pillar:** [The mediator is the irreducible trust anchor](../../../DESIGN.md#the-mediator-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [O2](../../../USE_CASES.md#o2--observable)

## Context

The mediator holds full mailbox credentials; if its pod is owned, redaction is irrelevant — the
attacker calls the provider directly. The design accepts this as the irreducible trust anchor,
which converts the question from "how do we prevent it" into "how expensive is it, how fast is it
noticed, and what evidence survives."

## Decision

Layered hardening, each layer answering one of those three questions:

- **Raising the cost:** credentials as mounted files that only the mediator can read
  ([ADR-0038](./0038-credentials-as-mounted-files.md)); the mediator runs hardened and minimal —
  no elevated privileges, an immutable filesystem, no shell, nothing beyond what it needs — with
  the hardening enforced by policy, and images signed and verified before they run.
- **Audit trail is maintained:** A record of changes is maintained in state store and significant
  events or changes are logged at a corresponding log level.
- **Recovery documented:** a credential-rotation runbook, so revoke-and-reissue follows a written
  procedure rather than an improvisation during an incident.

## Alternatives considered

- **Splitting credentials across multiple services to dilute the anchor.** Rejected: some process
  must ultimately wield full-mailbox credentials (that is what redaction-in-code means); splitting
  multiplies hardening surfaces without removing the anchor, and the extra hops are themselves
  attack surface.

## Consequences

- Alerting based on logs is a deployment platform and environment specific concern as is the backups
  of state store.
- Evidence written before a compromise survives it without depending on anything outside the
  cluster, because the audit log is append-only to every runtime role
  ([ADR-0016](../data/0016-schema.md)). That bounds the claim rather than absolutising it. An
  attacker holding the process governs what is written from that moment on, and what happens to
  the emitted rows and logs afterwards is the platform's
  ([ADR-0051](../engineering/0051-environment-contract.md)), not this project's.
