# 0028. Hardening the trust anchor: minimal pod, detectable drift, evidence that survives compromise

**Status:** Accepted ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [O2](../../../USE_CASES.md#o2--observable)

## Context

Every deployable that calls a provider holds full mailbox credentials
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)). If one of their pods
is owned, redaction is irrelevant, because the attacker calls the provider directly. The design
accepts this as the irreducible trust anchor, which converts the question from "how do we prevent
it" into "how expensive is it, how fast is it noticed, and what evidence survives."

## Decision

Layered hardening, each layer answering one of those three questions:

- **Raising the cost:** credentials sealed so that only the deployables calling a provider can
  open them ([ADR-0081](./0081-credentials-sealed-to-a-public-key.md)). Each of those deployables
  runs hardened and minimal, with no elevated privileges, an immutable filesystem, no shell, and
  nothing beyond what it needs. The hardening is enforced by policy, and images are signed and
  verified before they run. The UI runs under the same hardening, because it holds a fresh grant
  while it completes a consent
  ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)).
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
