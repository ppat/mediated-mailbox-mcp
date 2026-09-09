---
paths:
  - "docs/MUTATIONS.md"
---

# Rules for docs/MUTATIONS.md

You are touching the mutation ledger, the per-control record that removing a mechanism made its
tests go red. ADR-0046 holds the decision. Scenario rows belong to docs/VERIFICATIONS.md, and
only demonstrations belong here.

- **No row exists before implementation.** A row names the control, how its mechanism was
  removed or disabled, the tests that went red, the date, and an evidence pointer. The table
  is the whole artifact, never a pass rate or a score.
- **A surviving mutant keeps its row open as a defect.** The row closes when the tests are
  fixed or the mechanism is deliberately deleted as redundant, and never by being dropped
  quietly.
- **Rows regenerate on change only.** The control or its tests changing is the sole trigger,
  never a schedule. A stale demonstration is this ledger's version of a green that cannot
  fail.
- **The pairing with docs/VERIFICATIONS.md runs both ways.** A decided control carries its
  verification row from design time and owes its demonstration from implementation time. A
  control listed in docs/VERIFICATIONS.md with no row here is unfinished implementation.
- **A control with no standing automated test stays out.** Nothing exists to demand red from
  it, and proof only by drill or by manual exercise is that case. A drill-proven control that
  also carries standing automated tests owes its row. The line is ADR-0046's.
