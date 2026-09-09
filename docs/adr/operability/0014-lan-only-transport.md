# 0014. LAN-only transport — and egress restriction is the control that actually matters

**Status:** Deprecated ·
**Pillar:** [Network position never substitutes for the gate](../../../DESIGN.md#network-position-never-substitutes-for-the-gate) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

The agent runs inside the homelab, on the same LAN as the mediator. Nothing outside the LAN needs
to reach the system. Exposing the endpoint publicly would add an entire class of risk —
"someone on the internet reaches my mediator" — purely to serve callers that do not exist.

## Decision

**The client surface is reachable only from the LAN.** No public ingress, no push-notification
webhook, no inbound path from the internet of any kind. Within that scope:

- **TLS on the listener** — internal CA (cert-manager) or self-signed with the client pinning.
  Encrypts LAN traffic and prevents casual interception.
- **Bearer-token auth for every client** — the agent over MCP, or any caller of the API —
  distinct from all provider credentials
  ([ADR-0038](./0038-credentials-as-mounted-files.md)).
- **NetworkPolicy restricts egress** to provider API endpoints, the database, and DNS. This is the
  anti-exfiltration control, and it matters *more* than ingress restriction: the realistic attack
  is not an outsider reaching in but a prompt-injected agent trying to send data out — and a
  suborned agent with nowhere to send data is contained even when persuaded.
- **The UI sits on the same LAN**, TLS, its own auth; its write surface is two verbs, so its blast
  radius is small by construction ([ADR-0021](../mutation/0021-approval-surface.md)).

What LAN-only does *not* buy: it is not a substitute for the Redaction Gate. Anyone or anything on
the LAN reaching the client surface gets exactly what the gate permits and nothing more — the
invariant does not depend on network position.

## Alternatives considered

- **Public ingress with strong auth.** Rejected: it trades a removed risk class for a managed one,
  and no caller needs it.
- **Provider push notifications (webhook delivery) for sync.** Rejected here as a *transport*
  consequence — push requires an inbound path that this decision removes; the sync-cadence side of
  that trade is [ADR-0018](../data/0018-delta-sync-polls.md).
- **Mesh/overlay access (e.g. Tailscale) from day one.** Not adopted, but named as the likely
  answer if the assumption breaks: should the agent ever run outside the homelab, the ingress
  question reopens as a decision, and the answer is probably an overlay network (Tailscale or
  similar) rather than public ingress.

## Consequences

- The design assumes the agent is inside the LAN; that assumption is listed in
  [DESIGN.md's Known limits](../../../DESIGN.md#3-known-limits) with this record as its
  disposition.
- Removing the inbound path removes the awkwardness that made push-based sync unattractive — the
  polling decision ([ADR-0018](../data/0018-delta-sync-polls.md)) leans on this one.
- Egress restriction constrains future features too: anything wanting to call a new external
  endpoint must widen the NetworkPolicy deliberately, in review.
