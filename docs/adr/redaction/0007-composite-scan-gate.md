# 0007. A composite scan gate with a measured, accepted residual — not scan-everything

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [O2](../../../USE_CASES.md#o2--observable)

## Context

Content flags require reading bodies, and scanning every body is not performant — it is a latency,
memory-exposure, and CPU cost paid mostly on messages (newsletters, high-volume senders) that never
carry a secret. Something must decide which non-restricted bodies are worth scanning. Facts that
constrain the answer:

- Restricted-sender bodies are already denied and never scanned
  ([ADR-0008](./0008-restricted-senders-are-never-scanned.md)); the gate governs *normal* senders
  only.
- Gmail charges the same quota for a metadata fetch as for a full fetch (see the cost table in
  [ADR-0023](../operability/0023-adapter-declares-cost.md)) — **body fetch is quota-free relative
  to metadata fetch**, so the gate is not a quota optimization and can afford to be generous.
- The metadata index already holds cheap, high-signal predicates: sender volume, `List-Id`
  presence, local-part patterns, prior scan hits.

## Decision

A composite predicate over metadata already in the index decides scan or skip, evaluated
cost-ascending — the full chain: `List-Id` check → sender-volume lookup → subject pattern → body
fetch → Tier 1 → Tier 2 → Tier 3, each stage eliminating most of what reaches it, so Tier 3 should
be rare — with **scan as the default** and the skip branches explicit and narrow:

```
skip if sender_class == RESTRICTED           → SKIPPED_RESTRICTED
scan if subject matches Tier-1 subject patterns
scan if List-Id absent AND local-part in {noreply, no-reply, security,
                                          accounts, verify, auth, support}
scan if size < 30KB AND age < 24h AND List-Id absent
scan if sender_volume < 20              (long tail: scan exhaustively)
scan if sender has any prior scan hit    (known OTP source)
skip if sender_volume > 500 AND zero prior hits AND List-Id present
otherwise → scan
```

Gate decisions are memoized per sender, keyed on sender plus subject shape, so the predicate's cost
amortizes across a sender's traffic.

A message's relation to the scanner is a first-class state, because "not scanned" has three
distinct meanings with different consequences:

| State | Meaning | Body available? |
| --- | --- | --- |
| `SCANNED` | Scanner ran, verdict recorded | Yes, if no flags and sender normal |
| `SKIPPED_RESTRICTED` | Sender restricted; scan pointless | No — denied by sender class |
| `SKIPPED_GATE` | Gate said don't scan | **Yes** — accepted risk |
| `PENDING` | Backfill/sync has not reached it | No — fail closed |

`SKIPPED_GATE` is the compromise made explicit in the type system: the one state where a body is
released without having been scanned.

**What this costs, honestly.** The residual leak is the intersection of: *non-sensitive sender* ×
*MFA code or login link present* × *subject and metadata give no signal* — typically a SaaS tool
sending `"Hello from Acme"` with the code body-only. Two layers make this acceptable rather than
merely tolerable:

| Layer | Catches | Miss mode |
| --- | --- | --- |
| Sender classification | All financial / government / infrastructure MFA | Unlisted sensitive sender |
| Composite gate → scan | MFA from normal senders with any signal | Zero-signal normal sender |

The load-bearing argument is not that reputable senders have predictable subjects. It is that **the
value of a leaked code is proportional to what it unlocks**, and the high-value senders are caught
by sender classification regardless of subject. A leaked code for a newsletter signup is close to
harmless. The compromise degrades precisely where the stakes are lowest.

**Making it auditable.** Every skip is recorded with its reason; the UI surfaces skip rates by
reason and by sender, so the gate is tuned from evidence. A growing `PENDING` backlog is its own
failure mode — it silently converts messages into body-denials that read like permission bugs — so
backlog depth is a watched metric, and the denial envelope for pending messages says "pending
content scan" so the agent explains rather than misreports.

## Alternatives considered

- **Scan every body.** Rejected on latency, memory exposure, and CPU — but note it is *not*
  rejected on quota: if the gate proves too leaky in practice it can be widened substantially
  without approaching a quota wall. The latency, memory-exposure, and CPU costs that motivated the
  gate still bind as it widens.
- **A subject-only gate** (scan only when the subject pattern fires). Rejected: materially worse
  recall at the same quota cost — the composite predicate exists because body fetch being
  quota-free makes generosity affordable.
- **No gate state, just a nullable scanned-at timestamp.** Rejected: it collapses three
  consequentially different "not scanned" meanings into one, and the release decision needs them
  distinguished.

## Consequences

- An accepted residual exists by design and is bounded, measured, and tunable. Its measurement is
  part of the acceptance: unmeasured, this decision would be out of compliance with
  [C3](../../../USE_CASES.md#c3--content-based-secrets-caught) even with zero leaks.
- The gate needs sender statistics to evaluate, which is why backfill runs metadata-first
  ([ADR-0017](../data/0017-two-pass-backfill.md)) — the gate cannot run on a cold index.
- Widening or narrowing the gate is a policy change with observable effect, not a redesign.
