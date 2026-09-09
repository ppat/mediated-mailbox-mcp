# 0046. Every automatable control's tests are shown to go red when the control is removed

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The fail-closed pillar states its own limit. Fail-closed paths are exercised by tests or not at
all. For those paths the tests are the only evidence, and a vacuous test is a silent hole in
the invariant's proof. It is worse than absent because it is read as evidence. A test that
survives its mechanism's removal is unfalsifiable. Nothing in
[docs/VERIFICATIONS.md](../../VERIFICATIONS.md) otherwise stops an injection row from being
satisfied by exactly such a test. The outcome contract requires acceptance criteria to be
falsifiable, and this record applies that requirement to the tests themselves.

## Decision

- **Every control whose tests can be run automatically carries a mutation table at
  acceptance.** The mechanism is removed and the tests are demanded red. The table lists which
  tests failed when the control was deleted or disabled. The table is the artifact and never a
  pass rate. A control proven only by drill or by manual exercise has no mutation table
  because there is no standing test to demand red.
- **A surviving mutant on a control is a defect.** Either the tests are vacuous or the
  mechanism is redundant. Both demand action. A mechanism not worth testing well raises the
  question of whether it is worth having.
- **The obligation is per-control and event-driven, never a standing gate.** The table is
  produced when the control lands. It is re-produced only when the control or its tests
  change. An automated apply-the-patch-expect-red harness produces it, so an absent table is a
  missing artifact rather than a silent green.
- **The demonstrations live in their own ledger,
  [docs/MUTATIONS.md](../../MUTATIONS.md).** It is the sibling of
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), and the split follows authoring moments. A
  verification row is minted at design time, when the control is decided. A mutation
  demonstration exists only at implementation time, when the mechanism and its tests both
  exist to be broken.

## Alternatives considered

- **Codebase-wide mutation scoring as a standing gate.** The case for it is that mutation
  tooling does this by default and covers everything. Rejected because it is heavy and noisy,
  test infrastructure whose complexity exceeds the risk it covers, and because a score hides
  which control's tests survived, which is the one fact that matters. Ecosystem mutation tools
  remain sanctioned for deep, scheduled, out-of-band searches.
- **A column on the verification rows instead of a sibling ledger.** The case for it is one
  fewer document. Rejected because it forces every row to carry a field most row kinds cannot
  use, since drills and manual exercises have no mutation table, and because it braids
  design-time rows and implementation-time demonstrations into one table.

## Consequences

- The ledger starts empty and stays empty until implementation begins. That is its correct
  starting state, exactly as it was for [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- The obligation presupposes [ADR-0044](./0044-layered-testing-strategy.md)'s permanent-test
  rule. Distrust of green only holds while the red stays demonstrable, and a mutation table is
  only meaningful over tests that keep running.
- Mutation results stay interpretable because wiring is hand-written
  ([ADR-0040](./0040-pure-core-decisions-as-values.md)). A mutant fails by breaking behavior
  and not by breaking a reflection container's resolution.
- This decision assumes each control's tests are runnable in isolation well enough to
  attribute a red to the removed mechanism.
