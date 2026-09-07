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
- **Noticing:** all manifests GitOps-managed, so drift between declared and running state is
  detectable rather than silent; egress locked to provider APIs, database, and DNS
  ([ADR-0014](./0014-lan-only-transport.md)), so a compromised pod's call home is a policy
  violation, not background noise.
- **Evidence surviving:** **the audit log ships off-cluster.** An attacker who owns the pod can
  otherwise erase the record of what was read — and an audit trail a compromised subject can
  destroy is not an audit trail. Off-cluster shipping is what makes "what did it read while owned"
  answerable afterward.
- **Recovery documented:** a credential-rotation runbook, so revoke-and-reissue follows a written
  procedure rather than an improvisation during an incident.

## Alternatives considered

- **Treating mediator compromise as out of scope.** Rejected: unstated, it becomes an implicit
  "cannot happen," and the audit log would quietly remain erasable by the one actor most motivated
  to erase it.
- **Splitting credentials across multiple services to dilute the anchor.** Rejected: some process
  must ultimately wield full-mailbox credentials (that is what redaction-in-code means); splitting
  multiplies hardening surfaces without removing the anchor, and the extra hops are themselves
  attack surface.

## Consequences

- The hardening list is admission-enforced where possible (policy engine), so regressions fail
  deployment rather than relying on review vigilance.
- Off-cluster audit shipping becomes a standing infrastructure dependency, accepted for what it
  buys; its delivery is scheduled work in [ROADMAP.md](../../../ROADMAP.md).
