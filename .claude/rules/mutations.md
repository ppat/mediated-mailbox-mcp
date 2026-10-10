---
paths:
  - "docs/MUTATIONS.md"
  - "docs/mutations/*.md"
---

# Rules for docs/MUTATIONS.md

You are touching the mutation ledger, the per-control record that breaking a mechanism made its
tests go red. The ledger is docs/MUTATIONS.md and the files under docs/mutations/ it lists, and
docs/MUTATIONS.md states a row's form and which file holds it. ADR-0046 holds the decision.
Scenario rows belong to docs/VERIFICATIONS.md, and only demonstrations belong here.

- **No row exists before implementation.** A row names the control, how its mechanism was
  broken, once so it does less and once so it does the wrong thing, the tests that went red, the
  date, and an evidence pointer. The rows are the whole artifact, never a pass rate or a score.
- **A surviving mutant keeps its row open as a defect.** The row closes when the tests are
  fixed or the mechanism is deliberately deleted as redundant, and never by being dropped
  quietly.
- **Rows are written on change only.** A row is written when the control, its tests, or a
  generator its tests draw from changes. The `mutation-demonstrations` workflow also runs every
  demonstration on a schedule and on demand, and that run writes no row. A demonstration that fails
  there is a defect, handled as a discovery under the unit that owns the control (ADR-0046,
  ADR-0124). A stale demonstration is this ledger's version of a green that cannot fail.
- **The pairing with docs/VERIFICATIONS.md runs both ways.** A decided control carries its
  verification row from design time and owes its demonstration from implementation time. A
  control listed in docs/VERIFICATIONS.md with no row here is unfinished implementation.
- **A control with no standing automated test stays out.** Nothing exists to demand red from
  it, and proof only by drill or by manual exercise is that case. A drill-proven control that
  also carries standing automated tests owes its row. The line is ADR-0046's.
