# 0005. Secret detection is tiered and structural — and no LLM ever sits in the scanning path

**Status:** Accepted ·
**Pillar:** [Redaction is enforced by code, not by the provider's token](../../../DESIGN.md#redaction-is-enforced-by-code-not-by-the-providers-token) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

No list can enumerate MFA formats, so content-based detection must be heuristic. But full machine
learning is also the wrong first tool: one-time-code detection is a high-precision *pattern*
problem, and patterns handle the bulk at negligible cost.

## Decision

Detection is tiered, cost-ascending, each tier seeing only what the previous could not settle.

**Tier 1 — structural patterns (~85–90% of cases, microseconds).** One-time-code mail is
structurally distinctive, not merely lexically:

- A digit run of 4–8 within a token window of a trigger word (`code`, `OTP`, `verification`,
  `PIN`, `passcode`, `2FA`, `one-time`, `security code`, plus localized forms).
- A short line whose entire content is a 4–8 digit run — extremely high precision; almost nothing
  else formats that way.
- A digit run inside `<h1>`/`<h2>` or a table cell with letter-spacing CSS — the visual
  one-time-code idiom in HTML mail.
- A URL with a high-entropy path segment plus `token`/`confirm`/`verify`/`reset`/`magic`/`auth`.
- A URL query parameter named `token`, `code`, `key`, `auth`, `t`, or `otp` carrying ≥16
  entropy-dense characters.

**Tier 2 — entropy and position scoring.** Candidate spans scored on Shannon entropy,
character-class mix, length, distance to the nearest trigger, and structural position (own line,
heading, bold), with the threshold tuned for recall. Catches alphanumeric and unusual formats Tier
1's fixed patterns miss.

**Tier 3 — a small local model**, in scope but deferred:
[ADR-0006](./0006-tier-3-local-model-deferred.md).

Implementation constraints that keep the tiers fast:

- **Set-matching, not sequential alternation** — the keyword layer is an Aho-Corasick automaton
  compiled once at startup; sequential regex alternation over dozens of patterns is the classic way
  this gets slow.
- **Cost-ordered evaluation** — within detection, Tier 1 before Tier 2 before Tier 3, each stage
  eliminating most of what reaches it, so the expensive tiers should be rare; the full
  predicate-to-tier chain lives with the scan gate
  ([ADR-0007](../redaction/0007-composite-scan-gate.md)).

**Explicitly not: an LLM in the scanning path.** Slow, expensive, non-deterministic, and —
decisively — it means sending body content to an inference endpoint, which is the exact exposure
this system exists to prevent. LLM help writing *rules* is fine: done offline on a curated sample,
shipping the rules, never the model.

## Alternatives considered

- **A single ML classifier for all content detection.** Rejected: it spends the hard cases' tool
  on the easy cases, adds nondeterminism where patterns are near-perfect, and requires labeled data
  that does not exist on day one.
- **Patterns only, no scoring tier.** Rejected: fixed patterns miss alphanumeric and novel formats;
  the scoring tier is what covers the tail without a model.
- **An LLM scanner (local or hosted).** Rejected as above; the hosted variant is
  self-contradictory for this system, and even a local LLM is slow and non-deterministic where the
  tiers are fast and inspectable.

## Consequences

- Detection quality is inspectable per tier: every hit records which tier and rule fired, so
  precision problems localize to a rule rather than a model.
- The tier boundary gives Tier 3 a natural insertion point later without touching Tiers 1–2.
- Trigger vocabularies and patterns are code, so improving them is a reviewed change with a
  version bump — which is what makes re-scanning after improvements tractable
  ([ADR-0009](../redaction/0009-scanner-verdicts-carry-no-content.md)).
