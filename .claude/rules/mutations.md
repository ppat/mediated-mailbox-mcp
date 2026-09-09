---
paths:
  - "docs/MUTATIONS.md"
---

# Rules for docs/MUTATIONS.md

You are touching the mutation ledger, the per-control record that removing a mechanism made its
tests go red. ADR-0046 holds the decision behind it. Scenario rows belong to
docs/VERIFICATIONS.md. Demonstrations alone belong here.

- **No row before implementation.** A row names the control, the way its mechanism was removed
  or disabled, the tests that went red, the date, and an evidence pointer. The table is the
  whole artifact. Never a pass rate, never a score.
- **A surviving mutant holds its row open as a defect.** It closes only when the tests are
  fixed or the mechanism is deliberately deleted as redundant, and it is never dropped
  quietly.
- **Rows regenerate on change, not on a schedule.** A control or its tests changing is the
  only trigger. A stale demonstration is this ledger's version of a green that cannot fail.
- **Both directions of the pairing hold.** Every decided control has its verification row from
  design time and gains its demonstration at implementation time. A control in the catalogue
  with no row here is unfinished implementation.
- **Drill-proven and manually-proven controls stay out.** No standing test exists for them to
  fail. ADR-0046 draws the line.
