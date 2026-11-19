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
  broken, once so it does less and once so it does the wrong thing, the tests that went red, and
  one date and one evidence pointer, those of the latest demonstration the row was written from,
  which ran every one of its current patches. The rows are the whole artifact, never a pass rate
  or a score. Every statement in a row must be true of the current code, and a row holds no
  history.
- **The ledger holds every mutation of the current code base.** Each patch under a
  `testdata/mutations` directory, apart from the runner's own fixtures, is one break of its
  control's row, and no break is without its patch.
- **A surviving mutant keeps its row open as a defect.** The row closes when the tests are
  fixed or the mechanism is deliberately deleted as redundant, and never by being dropped
  quietly.
- **Rows are written on change only.** A demonstration runs again when the control, its tests,
  or a generator its tests draw from changes. A row is rewritten only when its patches change or
  need to change, or when a statement in it would otherwise be false of the current code, and then
  every patch of its control runs again in one demonstration, so its one date covers every break.
  The `mutation-demonstrations` workflow also runs every demonstration on a schedule and on demand,
  and such a run writes no row by itself. A row is written from one only by a change that rewrites
  the row. A demonstration that fails there is a defect, handled as a discovery under the unit that
  owns the control (ADR-0046, ADR-0124). A stale demonstration is this ledger's version of a green
  that cannot fail.
- **The pairing with docs/VERIFICATIONS.md runs both ways.** A decided control carries its
  verification row from design time and owes its demonstration from implementation time. A
  control listed in docs/VERIFICATIONS.md with no row here is unfinished implementation.
- **A control with no standing automated test stays out.** Nothing exists to demand red from
  it, and proof only by drill or by manual exercise is that case. A drill-proven control that
  also carries standing automated tests owes its row. The line is ADR-0046's.
