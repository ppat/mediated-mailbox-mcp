---
paths:
  - "docs/MUTATIONS.md"
---

# Rules for docs/MUTATIONS.md

You are touching the mutation ledger. It holds each automatable control's demonstration that
removing its mechanism made its tests go red. The obligation's decision lives in ADR-0046. The
scenario rows live in docs/VERIFICATIONS.md. This ledger holds only demonstrations.

- **A row exists only from implementation time.** It names the control, how the mechanism was
  removed or disabled, which tests went red, the date, and an evidence pointer. The table is
  the artifact, never a pass rate and never a score.
- **A surviving mutant on a control is an open defect row.** It stays open until the tests are
  fixed or the mechanism is deliberately removed as redundant. It is never quietly dropped.
- **Rows are re-produced only when the control or its tests change, never as a standing
  gate.** A demonstration's date is a claim about that date. A stale demonstration is the
  ledger's version of an unfalsifiable green.
- **The pairing duty runs both ways.** A control minted in a record lands with its
  verification row at design time, and gains its mutation demonstration at implementation
  time. A control present in docs/VERIFICATIONS.md but never demonstrated here is unfinished
  implementation.
- **Controls proven only by drill or manual exercise never appear here.** There is no standing
  test to demand red. The split is ADR-0046's.
