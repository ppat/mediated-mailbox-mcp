# 0017. Backfill is full-history and two-pass: metadata first, gated body scanning second

**Status:** Accepted ·
**Pillar:** [Metadata always flows; sensitive bodies never do](../../../DESIGN.md#metadata-always-flows-sensitive-bodies-never-do) ·
**Serves:** [G2](../../../USE_CASES.md#g2--historical-understanding), [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Historical understanding requires the whole corpus, because a recent window cannot answer "what
patterns already exist here." The corpus ceiling is on the order of 100k messages, likely less. And
the scan gate ([ADR-0093](../redaction/0093-composite-scan-gate.md)) evaluates sender statistics
(volume, `List-Id` ratios, prior hits) that do not exist on a cold start, so scanning cannot begin
until the metadata that feeds the gate has been built.

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

Two rules set what pass 2 scans of a gated-in body, and what it records when it cannot.

- **Both parts of a body are scanned.** Pass 2 converts the body's HTML part to Markdown with the
  converter release uses ([ADR-0036](../redaction/0036-released-bodies-are-clean-markdown.md)),
  since the scanner reads Markdown, and scans that and the text part. A flag in either flags the
  message. The verdict clears the body for release, so it covers whichever part release serves.
- **A body the converter refuses stays pending.** It is never scanned, its message stays pending
  scan, which denies its body, and the run records it as a failed item of kind `message` with error
  class `validation` and disposition `abandoned`
  ([ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)). The pass moves on to the next message.

The split buys these properties.

- **Resumable at page granularity.** The cursor is checkpointed to the database every page, so
  killing or evicting the process that runs backfill costs one page of rework, not a run.
- **Pass 1 delivers value immediately.** Before pass 2 finishes, the agent can already answer
  "what patterns already exist here", with label distributions, senders with no label, the
  observation that receipts are filed by vendor but newsletters by topic, and the two hundred
  threads accounting for most unfiled volume. None of it needs a body, which is precisely why this
  design can offer strong organizational capability alongside strong content restriction.
- **The gate gets real statistics**, not cold-start guesses.
- The cost is roughly doubled wall-clock, which at this corpus size is hours, under a day rather
  than days. The arithmetic lives with the rate posture in
  [ADR-0024](../operability/0024-conservative-target-aimd.md).

## Alternatives considered

- **One pass, scanning as messages are enumerated.** No case was tabled for it. Rejected: the gate
  would evaluate against statistics that do not exist yet, forcing either scan-everything (the cost
  [ADR-0093](../redaction/0093-composite-scan-gate.md) avoids) or gate decisions made blind.
- **Recent-window backfill, extending on demand.** No case was tabled for it. Rejected: it fails
  historical understanding outright, and it would make sender statistics unrepresentative exactly
  where the gate leans on them. The design commits to full history precisely so corpus-wide analysis
  is possible.
- **Restart-from-zero on interruption.** No case was tabled for it. Rejected: eviction of the
  process running backfill during a long run is an expected condition, not a hypothetical, and the
  checkpoint turns it into a page of rework. Its proving injection (kill the process mid-run,
  confirm clean resume) is catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- **Scanning only the HTML part's Markdown.** Its case is that the scanner reads Markdown, which
  is what the converter produces. Rejected, because a body with no HTML part, or whose text
  part differs, would be cleared for release without its released part ever being scanned.
- **Recording a refused body as scanned with a flag.** Its case is that the body stays denied and
  leaves the backlog. Rejected, because it claims a scan that never ran. Scanning the raw HTML
  instead was rejected too, because the scanner reads Markdown and a refused body is withheld at
  release whatever it holds.

## Consequences

- Both passes run again after a change of scanner, the first to fetch again and mask again the
  stored subjects that were masked, once the run-start step has masked the others again from the
  store, and the second to scan again what an earlier scanner decided
  ([ADR-0120](../redaction/0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)).
  The second pass also runs again when backfill's run-start step returns to pending a gate skip
  the gate no longer decides as the same skip
  ([ADR-0121](../redaction/0121-the-run-start-step-decides-each-gate-skip-again.md)).
- Backfill is the first workload long enough to trip real provider limits, so it depends on the
  rate controller.
- Pass-1-then-pass-2 is also the corpus's first real test of the canonical mapping against messy
  data. A mapping flaw found here costs a re-run, and one found after agent workflows exist costs a
  redesign.
