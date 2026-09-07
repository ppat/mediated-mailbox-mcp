# 0008. Restricted-sender bodies are never scanned — denial makes scanning pure exposure

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

The content scanner exists to find secrets in bodies that might be released. Restricted-sender
bodies are never released — their denial is decided by sender class alone. Scanning them would
mean fetching the most sensitive bodies in the corpus into mediator memory to compute a verdict
that changes nothing.

## Decision

**Restricted-sender bodies are never fetched and never scanned.** They are marked
`SKIPPED_RESTRICTED` when the scan gate evaluates them, without contacting the provider for
content.

Two boundaries this deliberately does not move:

- **Subject masking still runs on restricted senders**
  ([ADR-0003](./0003-subject-masking.md)) — subjects are visible, so a code in a subject still
  needs masking. The skip is of the *body scan* only.
- **The body denial does not depend on this record.** Denial follows from sender class at the
  Redaction Gate ([ADR-0002](./0002-fetch-time-re-evaluation.md)); this record only removes a
  pointless fetch.

## Alternatives considered

- **Scan everything uniformly, restricted included.** Rejected: it adds the highest-stakes bodies
  to the scanner's blast radius — memory, logs-adjacent surfaces, any future spill bug — in
  exchange for verdicts with no consumer. Uniformity is not worth exposure.
- **Scan restricted bodies for analytics** (e.g. to improve detection rules from the richest
  source of financial MFA formats). Rejected: it is the same exposure with a nicer motive, and the
  rule-improvement loop already has a supply of positives from normal-sender traffic.

## Consequences

- A large fraction of the corpus's sensitive volume is removed from the scanner's blast radius
  entirely — the scanner only ever holds bodies that were at least candidates for release.
- Backfill's body-scanning pass shrinks accordingly; restricted messages cost metadata work only.
- Removal from the sensitive list is a designed transition: the sender's messages are marked
  pending scan ([ADR-0037](./0037-delisting-transition.md)).
