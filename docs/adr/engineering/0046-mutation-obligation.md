# 0046. Every automatable control's tests are shown to go red when the control is removed

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The fail-closed pillar names its own limit. Its paths are exercised by tests or not at all,
because production never visits them. Where tests are the only evidence, a vacuous test is
worse than none, since [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) would show a green row
over a hole in the invariant's proof. Before this record, an injection row could be satisfied
by a test that keeps passing with its mechanism deleted, and such a test is unfalsifiable. The
outcome contract's own constraint, that acceptance criteria must be falsifiable, is applied
here to the tests.

## Decision

- **An automatable control's acceptance includes its mutation table.** Remove the mechanism,
  demand the tests go red, and record which tests failed. The table is the whole artifact, and
  a pass rate never is. A control with no standing automated test carries no table, since
  nothing exists to demand red from, and proof only by drill or by manual exercise is that
  case.
- **A surviving mutant is a defect on the spot.** Either the tests are vacuous or the
  mechanism is redundant, and each finding demands its own action. A mechanism not worth
  testing well is a candidate for removal.
- **The obligation is event-driven per control, never a standing gate.** The table is produced
  when the control lands and reproduced when the control or its tests change, at no other
  time.
- **The demonstrations live beside [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), in their
  own document, [docs/MUTATIONS.md](../../MUTATIONS.md).** The split is by authoring moment.
  A verification row is minted at design time, when the control is decided. A demonstration
  can exist only at implementation time, once a mechanism exists to remove and tests exist to
  fail.

## Alternatives considered

- **Whole-codebase mutation scoring as a standing gate.** The case for it is coverage of
  everything, the default of mutation tooling. Rejected as heavy and noisy, test
  infrastructure whose complexity exceeds the risk it covers, and because a score hides the
  one fact that matters, which control's tests survived.
- **A mutation column on the verification rows.** The case for it is one fewer document.
  Rejected because most row kinds could never fill it, controls with no standing automated
  test having no table, and because it braids a design-time table into an
  implementation-time one.

## Consequences

- The ledger starts empty and stays empty until implementation.
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) began the same way, and that is the correct
  starting state.
- The obligation presupposes [ADR-0044](./0044-layered-testing-strategy.md)'s
  forever-running tests. A red that cannot be demonstrated again is a memory, not a proof.
- Attributing a red to the removed mechanism assumes each control's tests run in isolation
  well enough to make the attribution.
- The obligation reaches every automatable control, whichever outcome its row serves. The
  header names C2 because the fail-closed paths, which are C2's, are where tests are the only
  evidence and a vacuous test costs the most.
