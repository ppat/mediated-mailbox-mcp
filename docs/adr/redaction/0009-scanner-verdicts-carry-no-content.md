# 0009. The scanner runs out-of-band, holds bodies only in memory, and emits verdicts that cannot carry content

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The content scanner is the one component that deliberately reads bodies it intends to withhold —
the inverse of every other component's relationship to content. Its boundary therefore needs more
care than any other: a scanner that leaks (through its output, its logs, its spill files, or its
placement in the request path) defeats the system from inside.

## Decision

Three constraints, one per leak direction.

**Placement: outside the synchronous MCP path.** The scanner runs only in the batch subsystems.
Inline scanning would put a scanner bug or timeout directly between the agent and content, creating
pressure to fail open under latency. Out of band, the only failure available is "not yet scanned" —
which is a deny state.

**Output: the verdict type cannot represent content.**

```python
@dataclass(frozen=True)
class ScanVerdict:
    message_id:      str
    content_flags:   frozenset[ContentFlag]
    masked_subject:  str | None      # the only content-derived field
    rule_ids:        tuple[str, ...]
    tier_reached:    int
    scanned_at:      datetime
    scanner_version: int
    # No body. No excerpt. No matched text. Not representable.
```

The scanner reads a body into memory, evaluates the tiers, emits a verdict, drops the body. There
is no field a body could hide in — the unsafe state is unconstructable rather than merely untaken.

**Residue: nothing body-derived persists or spills.**

- Bodies live in memory only: no body to disk, no spill path, memory-backed scratch space
  (`emptyDir: {medium: Memory}`) for anything that could spill, short-lived process.
- Nothing body-derived is persisted except the masked subject and the flags — no matched text, no
  offsets, no excerpts. Scanner logs record counts and rule identifiers only.
- `scanner_version` is stamped on every verdict so that when rules improve, affected rows are
  marked stale and reprocessed — re-scanning is a planned operation, not a migration.

**Accepted residual, stated plainly:** non-restricted, gated-in bodies transit mediator memory.
That channel exists regardless — the mediator fetches bodies to serve them at all. The scanner
increases volume through an existing channel; it does not create a new one.

## Alternatives considered

- **Inline scanning at fetch time.** Rejected for the failure-pressure reason above: a scanner
  timeout in the serving path invites exactly the fail-open shortcut the design forbids, and scan
  latency would land on every interactive body fetch.
- **Persisting matched text or offsets** to make verdicts explainable and re-checkable. Rejected:
  it stores fragments of the very content the verdict exists to withhold. Rule identifiers plus the
  masking-events view give the operator explainability without content.
- **A verdict type with an optional debug/excerpt field, disabled in production.** Rejected as the
  canonical example of "untaken, not unconstructable" — a field that exists will eventually be
  filled.

## Consequences

- A scanner compromise or bug is bounded to counts, rule identifiers, and masked subjects — the
  persisted surface simply has nowhere to put a body.
- Re-scanning after rule improvements is cheap and targeted via `scanner_version`.
- The scanner's correctness is testable by inspection of its output types plus a test that greps
  scanner output and logs for fixture body text — absence is verifiable, and that verification is
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
