# 0003. Subject masking is aggressive, runs on every message, and uses only the pattern tiers

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

MFA codes appear in subjects, not just bodies — and subjects are always visible, including for
restricted senders. Subjects are short, which makes entropy scoring less reliable than it is on
bodies, and the error costs are asymmetric: a masked tracking number is an annoyance; a leaked live
OTP is a compromise.

## Decision

- **Masking runs on every message, restricted senders included.** Skipping the restricted-sender
  *body* scan ([ADR-0008](./0008-restricted-senders-are-never-scanned.md)) never skips subject
  masking.
- **Tiers 1–2 only** ([ADR-0005](../classification/0005-tiered-detection.md)) — structural patterns
  and scoring; the short-text regime is where fixed patterns are strongest and statistical signals
  weakest.
- **Tuned aggressively toward masking.** Over-masking is the correct failure direction; thresholds
  favor recall.
- **Every mask is audited.** Each event records the rule and tier that fired (never the matched
  text), surfaced via `list_masking_events` and the UI's masking-events view, so tuning is driven
  by observed traffic rather than intuition.
- Masked subjects are masked **at rest** in the index — the stored subject is already safe to
  serve.

What the agent receives, concretely:

```json
{ "account_id": "personal", "thread_id": "t_a41b",
  "subject": "Your Acme verification code is ██████",
  "from": {"email": "noreply@acme.io", "display_name": "Acme"},
  "date": "2026-07-23T11:40:00Z", "labels": ["INBOX"],
  "snippet": null,
  "sensitivity": {"sender_class": "normal", "content_flags": ["mfa_code"],
                  "scan_state": "scanned",
                  "rule_ids": ["content.mfa.subject_numeric_6"]},
  "body_available": false,
  "allowed_mutations": ["label", "move", "archive", "trash", "spam"],
  "note": "Verification code redacted. Body withheld." }
```

The axes composing: sender is normal, so full mutation rights apply — the agent can archive expired
MFA mail, genuinely useful hygiene — while the code is unreachable.

## Alternatives considered

- **Conservative masking, tuned for precision.** Rejected by the cost asymmetry: the failure it
  optimizes against (an over-masked subject) is cheap and visible, while the failure it permits (a
  live code served) is expensive and silent.
- **Full tier stack on subjects, including the scoring/model tiers at body thresholds.** Rejected:
  short text starves entropy and position features; it would add cost and false confidence, not
  recall.
- **Masking off by default, enabled per sender.** Rejected: codes come overwhelmingly from senders
  the operator has never thought about; an opt-in list is a list of yesterday's senders.

## Consequences

- Some legitimate numbers (order IDs, tracking numbers) will be masked; the masking-events view
  exists so those rules get refined from evidence, and the audit trail makes every mask
  explainable.
- Because masking runs during ingest, a *detected* code never exists unmasked downstream of the
  provider — not in the index, not in the UI, not in agent responses. Detection misses are the
  masking-events review loop's business; they are why the posture is aggressive and audited.
