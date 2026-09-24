# 0017. Backfill is full-history and two-pass: metadata first, gated body scanning second

**Status:** Accepted ·
**Pillar:** [Metadata always flows; sensitive bodies never do](../../../DESIGN.md#metadata-always-flows-sensitive-bodies-never-do) ·
**Serves:** [G2](../../../USE_CASES.md#g2--historical-understanding), [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Historical understanding requires the whole corpus — a recent window cannot answer "what patterns
already exist here." The corpus ceiling is on the order of 100k messages, likely less. And the scan
gate ([ADR-0007](../redaction/0007-composite-scan-gate.md)) evaluates sender statistics — volume,
`List-Id` ratios, prior hits — that do not exist on a cold start, so scanning cannot begin until
the metadata that feeds the gate has been built.

## Decision

**Backfill covers full history and runs as two passes.**

```
PASS 1 — metadata only
  ├─ enumerate_all → metadata pages, cursor checkpointed per page
  ├─ upsert messages, threads, participants
  ├─ sender classification (deterministic, metadata-only)
  ├─ subject masking (all messages, restricted included)
  └─ build sender aggregates: volume, first/last seen,
     label distribution, List-Id presence, local-part patterns

  ► The agent has FULL organizational visibility at this point.
    Bodies are PENDING (denied). Organizing works; reading waits.

PASS 2 — gated body scan
  ├─ restricted senders → SKIPPED_RESTRICTED, never fetched
  ├─ scan gate evaluated per message using pass-1 statistics
  ├─ gated-in → fetch body, scan the tiers, emit verdict, drop body
  ├─ gated-out → SKIPPED_GATE, reason recorded
  └─ mark backfill complete
```

Properties the split buys:

- **Resumable at page granularity.** The cursor is checkpointed to the database every page, so a
  pod eviction costs one page of rework, not a run.
- **Pass 1 delivers value immediately.** Before pass 2 finishes, the agent can already answer
  "what patterns already exist here": label distributions, senders with no label, the observation
  that receipts are filed by vendor but newsletters by topic, the two hundred threads accounting
  for most unfiled volume. None of it needs a body — which is precisely why this design can offer
  strong organizational capability alongside strong content restriction.
- **The gate gets real statistics**, not cold-start guesses.
- The cost is roughly doubled wall-clock, which at this corpus size is hours, under a day rather
  than days — the arithmetic lives with the rate posture in
  [ADR-0024](../operability/0024-conservative-target-aimd.md).

## Alternatives considered

- **One pass, scanning as messages are enumerated.** Rejected: the gate would evaluate against
  statistics that do not exist yet, forcing either scan-everything (the cost
  [ADR-0007](../redaction/0007-composite-scan-gate.md) avoids) or gate decisions made blind.
- **Recent-window backfill, extending on demand.** Rejected: it fails historical understanding
  outright — the design commits to full history precisely so corpus-wide analysis is possible —
  and it would make sender statistics unrepresentative exactly where the gate leans on them.
- **Restart-from-zero on interruption.** Rejected: pod eviction during a long run is an expected
  condition, not a hypothetical; the checkpoint turns it into a page of rework. Its proving
  injection (kill the pod mid-run, confirm clean resume) is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).

## Consequences

- Backfill is the first workload long enough to trip real provider limits, which is why the rate
  controller exists before it runs (an ordering fact carried by [ROADMAP.md](../../../ROADMAP.md)).
- Pass-1-then-pass-2 is also the corpus's first real test of the canonical mapping against messy
  data — a mapping flaw found here costs a re-run; found after agent workflows exist, a redesign.
