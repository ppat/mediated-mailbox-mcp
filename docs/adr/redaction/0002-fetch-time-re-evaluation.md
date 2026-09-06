# 0002. Body release re-evaluates at fetch time, and a denial never contacts the provider

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

Sensitivity is computed during backfill and sync and stored in the index. That creates a choice
about what a body-fetch request trusts: the stored classification as of enumeration time, context
supplied by the agent, or a fresh evaluation. Meanwhile policy is live — the operator adds a domain
to the deny list and expects it to bind immediately.

## Decision

`get_message_body` re-classifies from the index at fetch time, against current policy, trusting
nothing the agent supplies:

```
Gate re-classifies from the index at fetch time
  │  (never trusts agent-supplied sensitivity context)
  ├─ scan_state = PENDING             → DENY ("pending content scan")
  ├─ scan_state = SKIPPED_RESTRICTED  → DENY (policy)
  ├─ sender_class = restricted        → DENY (policy)
  ├─ content_flags non-empty          → DENY (policy)
  │     all denials: provider never contacted, audit row written
  └─ otherwise (SCANNED clean, or SKIPPED_GATE)
        → adapter.get_message_body() → sanitize → audit ALLOW → return
```

Two properties are the point:

- **On deny, the provider is never contacted**, so no denied body ever enters mediator memory —
  the denial is decided entirely from the index.
- **Policy changes bind on the next call, not the next sync.** A newly deny-listed domain denies
  immediately, because classification is re-derived against current policy at fetch time.

`SKIPPED_GATE` allowing is deliberate and visible in the flow rather than hidden — it is the
accepted residual of [ADR-0007](./0007-composite-scan-gate.md).

## Alternatives considered

- **Trust the classification cached at enumeration time.** Rejected: a deny-list addition would
  keep leaking until a re-sync — exactly the failure
  [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released) names as falsifying.
- **Accept agent-supplied sensitivity context** (e.g. the sensitivity block the agent received with
  the metadata). Rejected: anything the agent supplies, an injected agent can forge. Inputs to the
  release decision must come only from state the agent cannot write.
- **Fetch the body first, then decide.** Rejected: it moves every denied body through mediator
  memory for no benefit, enlarging the blast radius of any spill or logging bug.

## Consequences

- Every fetch decision — allow and deny alike — writes an audit row; the denial envelope carries a
  reason distinct enough that "pending content scan" reads as backlog, not as a permissions bug the
  agent will misreport.
- The gate's deny branches depend only on the index, so gate behavior is fully testable offline
  with fixtures — no provider, no network.
- Fetch-time re-evaluation means the index's *cached classification* never decides release on its
  own: sender class is re-derived against current policy at fetch time. Scan state is index state
  and does gate release — re-read at fetch time, never trusted from enumeration.
