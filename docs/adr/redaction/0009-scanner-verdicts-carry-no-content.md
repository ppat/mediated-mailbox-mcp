# 0009. The scanner runs out-of-band, holds bodies only in memory, and emits verdicts that cannot carry content

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The content scanner is the one component that deliberately reads bodies it intends to withhold,
the inverse of every other component's relationship to content. Its boundary therefore needs more
care than any other, because a scanner that leaks (through its output, its logs, its spill files,
or its placement in the request path) defeats the system from inside.

## Decision

Three constraints, one per leak direction.

**Placement: outside the synchronous serving path.** The scanner runs only in the batch subsystems.
Inline scanning would put a scanner bug or timeout directly between the agent and content, creating
pressure to fail open under latency. Out of band, the only failure available is "not yet scanned",
which is a deny state. This placement rule binds the full scanner. The serve-time pattern check of
[ADR-0002](./0002-fetch-time-re-evaluation.md) is not the scanner but a pure pattern check on a
body already in memory, with no external calls and no backlog, and it does not sit under this rule.

**Output: the verdict type cannot represent content.**

```python
@dataclass(frozen=True)
class ScanVerdict:
    content_flags:    frozenset[ContentFlag]
    rule_ids:         tuple[str, ...]
    tier_reached:     int
    scanner_version:  int
    scanner_revision: str                 # the scanner configuration's revision
    # No body. No excerpt. No matched text. Not representable.

@dataclass(frozen=True)
class MaskedSubject:                      # subject masking's output, a value of its own
    subject: str                          # the only content-derived field
    events:  tuple[MaskingEvent, ...]     # one per mask, naming the rule and tier, never the text
```

The scanner reads a body into memory, evaluates the tiers, emits a verdict, drops the body. There
is no field a body could hide in, so the unsafe state is unconstructable rather than merely untaken.

The scan verdict holds the content flags, the identifiers of the rules that fired, the tier
reached, the scanner version and the revision of the scanner's configuration
([ADR-0005](../classification/0005-tiered-detection.md)). The message identifier and the scan time
are added by the shell that stores it. The masked subject is subject masking's output, a value of
its own holding the masked subject and one event per mask naming the rule and tier, because masking
runs on every message and the body scan does not
([ADR-0003](./0003-subject-masking.md), [ADR-0008](./0008-restricted-senders-are-never-scanned.md)).

**Residue: nothing body-derived persists or spills.**

- Bodies live in memory only, with no body to disk, no spill path, and memory-backed scratch space
  (`emptyDir: {medium: Memory}`) for anything that could spill. How long the process holding a
  body lives adds nothing to this guard, since the mediator and the process running delta sync run
  until stopped.
- Nothing body-derived is persisted except the masked subject and the flags, so no matched text,
  no offsets and no excerpts. Scanner logs record counts and rule identifiers only.
- `scanner_version` is stamped on every verdict so that when rules improve, affected rows are
  marked stale and reprocessed, which makes re-scanning a planned operation, not a migration. The
  configuration's revision is stamped beside it, so a change to the vocabulary or the tuning marks
  rows stale the same way. How the operation runs, and how a masked subject is masked again, is
  [ADR-0096](./0096-a-scanner-change-reopens-backfill.md)'s.

**Accepted residual, stated plainly:** non-restricted, gated-in bodies transit mediator memory.
That channel exists regardless, since the mediator fetches bodies to serve them at all. The scanner
increases volume through an existing channel, and it does not create a new one.

## Alternatives considered

- **Inline scanning at fetch time.** No case was tabled for it. Rejected for the failure-pressure
  reason above: a scanner timeout in the serving path invites exactly the fail-open shortcut the
  design forbids, and scan latency would land on every interactive body fetch.
- **Persisting matched text or offsets** to make verdicts explainable and re-checkable. Rejected:
  it stores fragments of the very content the verdict exists to withhold. Rule identifiers plus the
  masking-events view give the operator explainability without content.
- **A verdict type with an optional debug/excerpt field, disabled in production.** No case was
  tabled for it. Rejected as the canonical example of "untaken, not unconstructable", because a
  field that exists will eventually be filled.

## Consequences

- A scanner compromise or bug is bounded to counts, rule identifiers, and masked subjects, because
  the persisted surface simply has nowhere to put a body.
- Re-scanning after rule improvements is targeted via `scanner_version`, at the verdicts an earlier
  scanner made, and is an operation rather than a migration. What one change costs is
  [ADR-0096](./0096-a-scanner-change-reopens-backfill.md)'s.
- The scanner's correctness is testable by inspection of its output types plus a test that greps
  scanner output and logs for fixture body text. Absence is verifiable, and that verification is
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
