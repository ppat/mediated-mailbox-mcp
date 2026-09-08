# 0046. Every automatable control's tests are shown to go red when the control is removed

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The fail-closed pillar states the limit — fail-closed paths are exercised by tests or not at
all — so for those paths the tests are the only evidence, and a vacuous test is a silent hole
in the invariant's proof: worse than absent, because it is read as evidence. A test that
survives its mechanism's removal is unfalsifiable. Nothing in the catalogue's standard
otherwise stops an injection row from being satisfied by exactly such a test — and this is the
outcome contract's own constraint, acceptance criteria must be falsifiable, applied to the
tests themselves.

## Decision

- **Every control whose tests can be run automatically carries a mutation table at acceptance:
  the mechanism removed, the tests demanded red.** The table lists which tests failed when the
  control was deleted or disabled — the table, never a pass rate. A control proven only by
  drill or by manual exercise has no mutation table: there is no standing test to demand red.
- **A surviving mutant on a control is a defect** — either the tests are vacuous or the
  mechanism is redundant, and both demand action ("not worth testing well" triggers "then is it
  worth having?").
- **The obligation is per-control and event-driven, never a standing gate:** the table is
  produced when the control lands, and re-produced only when the control or its tests change.
  It is produced by an automated apply-the-patch-expect-red harness, so an absent table is a
  missing artifact rather than a silent green.
- **The demonstrations live in their own ledger,** [docs/MUTATIONS.md](../../MUTATIONS.md) —
  the sibling of the verification catalogue. The split follows authoring moments: a
  verification row is minted at design time, when the control is decided; a mutation
  demonstration exists only at implementation time, when the mechanism and its tests both exist
  to be broken.

## Alternatives considered

- **Codebase-wide mutation scoring as a standing gate.** The case for it: it is what mutation
  tooling does by default, and it covers everything. Rejected: heavy and noisy — test
  infrastructure whose complexity exceeds the risk it covers — and a score hides exactly the
  fact that matters, which control's tests survived. Ecosystem mutation tools remain sanctioned
  for deep, scheduled, out-of-band searches.
- **A column on the verification catalogue's rows instead of a sibling ledger.** The case for
  it: one fewer document. Rejected: it forces every row to carry a field most row kinds
  cannot use (drills and manual exercises have no mutation table), and it braids two lifecycles
  — design-time rows and implementation-time demonstrations — into one table.

## Consequences

- The ledger starts empty and stays empty until implementation begins; that is its correct
  starting state, exactly as the verification catalogue's was.
- The obligation presupposes [ADR-0044](./0044-layered-testing-strategy.md)'s permanent-test
  rule: distrust of green only holds while the red stays demonstrable, and a mutation table is
  only meaningful over tests that keep running.
- Mutation results stay interpretable because wiring is hand-written
  ([ADR-0040](./0040-pure-core-decisions-as-values.md)): a mutant fails by breaking behavior,
  not by breaking a reflection container's resolution.
- Assumptions about other components: each control's tests are runnable in isolation
  sufficient to attribute a red to the removed mechanism.
