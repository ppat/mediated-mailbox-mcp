# 0046. Every automatable control's tests are shown to go red when the control is removed

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The fail-closed pillar admits its own limit. Production never visits the fail-closed paths, so
tests are their only evidence, and a vacuous test on such a path is worse than no test, because
[docs/VERIFICATIONS.md](../../VERIFICATIONS.md) would show a green row over a hole. Until this
record, nothing prevented an injection row from being satisfied by a test that keeps passing
after its mechanism is deleted. The outcome contract demands falsifiable acceptance criteria,
and here that demand reaches the tests.

## Decision

- **At acceptance, an automatable control's mechanism is removed and its tests must go red.**
  The result is a mutation table naming the tests that failed when the mechanism was deleted or
  disabled. Only the table counts. A pass rate is never the artifact. Controls proven by drill
  or by manual exercise carry no table, since no standing test exists to fail.
- **A mutant that survives is a defect.** The tests are vacuous or the mechanism is redundant,
  and each of those demands its own fix. A mechanism not worth testing well is a candidate for
  removal.
- **The obligation fires on events, never on a schedule.** A table is produced when the control
  lands and again when the control or its tests change, and at no other time. A small harness
  applies the removal patch and expects red, so a missing table is a visible missing artifact
  instead of a silent green.
- **The tables live in [docs/MUTATIONS.md](../../MUTATIONS.md), beside the verification
  catalogue.** The two documents split on when a row can be written. Deciding a control mints
  its verification row at design time. Its mutation demonstration cannot exist before
  implementation, when there is a mechanism to remove and tests to fail.

## Alternatives considered

- **Mutation scoring across the whole codebase as a standing gate.** The case for it is that
  mutation tooling does this by default and covers everything. Rejected as heavy and noisy,
  test infrastructure whose complexity exceeds the risk it covers, and because the score buries
  the one useful fact, which is whose tests survived. Ecosystem tools stay sanctioned for deep
  out-of-band runs.
- **A mutation column on the verification rows.** The case for it is one document fewer.
  Rejected because drills and manual exercises could never fill the column, and because it
  welds a design-time table to an implementation-time one.

## Consequences

- The ledger begins empty and remains empty until implementation, the same correct starting
  state the verification catalogue had.
- The obligation stands on [ADR-0044](./0044-layered-testing-strategy.md)'s forever-running
  tests. A red that cannot be re-demonstrated is a memory, not a proof.
- Attributing a red to the removed mechanism assumes each control's tests can run in isolation.
