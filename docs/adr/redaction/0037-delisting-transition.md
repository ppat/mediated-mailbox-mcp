# 0037. Removing a sender from the sensitive list marks its messages pending scan — delisting is a designed transition

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

Restricted-sender bodies are never scanned while the sender is restricted
([ADR-0008](./0008-restricted-senders-are-never-scanned.md)) — correct, since their denial
follows from sender class alone. On removal from the list, all that sender's messages sit with no
scan verdict, and fail-closed means unscanned denies. Without a designed transition, their bodies
stay denied forever even though the sender is now normal, and no path finds those messages or
queues them for scanning — safe (over-denial, never a leak) but silent and permanent: it reads as
a bug, and the agent's denial reason stops making sense.

## Decision

**Removing a sender from the sensitive list marks that sender's messages as pending scan.** The
normal machinery — scan-gate evaluation, then scanning for the gated-in — picks them up like any
other unscanned mail. Bodies remain denied until scanned, which fail-closed already guarantees.
One state transition, plus machinery that already exists.

One interaction, bounded deliberately: the serve-time pattern check
([ADR-0002](./0002-fetch-time-re-evaluation.md)) would incidentally check an
ex-restricted body at release, but it is deliberately narrow — login-link and code patterns,
deny-on-hit — and does not count as scan clearance. This transition is the real path.

## Alternatives considered

- **No designed transition.** Its case: the resulting state is safe — over-denial, never a leak.
  Rejected: silent and permanent over-denial reads as a bug, and the agent's denial reason stops
  making sense.

## Consequences

- Fail-closed holds through the transition: pending denies until scanned — the `PENDING` state of
  [ADR-0007](./0007-composite-scan-gate.md), denied at fetch per
  [ADR-0002](./0002-fetch-time-re-evaluation.md) — so the transition can never release a body
  early.
- Assumptions about other components: the index can enumerate a sender's messages; the scan gate
  and scanner treat the marked messages as ordinary unscanned mail, with no special path; and
  whatever applies policy-list edits can observe a removal, since the marking is triggered by it.
- The transition is a control; its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
