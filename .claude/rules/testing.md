---
paths:
  - "TESTING.md"
  - "docs/MUTATIONS.md"
---

# Rules for TESTING.md and docs/MUTATIONS.md

You are touching the testing layer's documents: the strategy narrative (TESTING.md) and the
mutation ledger (docs/MUTATIONS.md). The strategy's decisions live in records (ADR-0043 through
ADR-0046); the scenario list lives in docs/VERIFICATIONS.md.

- **TESTING.md states strategy and cites; it never argues.** Decisions and their alternatives
  live in the records, cited by number through the index; scenario rows live in the
  verification catalogue; demonstrations live in the mutation ledger. A paragraph weighing
  alternatives here, or a concrete injection here, is routed wrong even if true.
- **A new layer, instrument, or discipline enters TESTING.md only on the back of a record** —
  strategy changes are decisions, and this document moves when they are minted, never ahead of
  them.
- **A mutation-ledger row exists only from implementation time**: it names the control, how the
  mechanism was removed or disabled, which tests went red, the date, and an evidence pointer.
  The table is the artifact — never a pass rate, never a score.
- **A surviving mutant on a control is an open defect row**, held open until the tests are
  fixed or the mechanism is deliberately removed as redundant; it is never quietly dropped.
- **Rows are re-produced only when the control or its tests change — never as a standing
  gate** — a demonstration's date is a claim about that date, and stale demonstrations are the
  ledger's version of an unfalsifiable green.
- **The pairing duty runs both ways**: a control minted in a record lands with its verification
  row at design time (the catalogue's own rule) and gains its mutation demonstration at
  implementation time. A control present in the catalogue but never demonstrated here is
  unfinished implementation, not an oversight to paper over.
