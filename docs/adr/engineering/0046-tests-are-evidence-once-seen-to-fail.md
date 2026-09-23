# 0046. A test is evidence only once it has been seen to fail, and only when its expectation is independent of the code under test

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

A passing test shows only that nothing errored. The known ways a test passes while proving nothing
are a test whose setup avoids the case it claims to cover, a check that passes while its step
silently produced nothing, a generated test whose generator never produced an input that could
expose the bug, a test that keeps passing with its control's mechanism deleted because what it
really exercises is something else, a test whose expected value is read from the code under test so
that breaking the code breaks the expectation with it, and a lint ban that reads correctly and
matches nothing. The outcome contract requires every acceptance criterion to be falsifiable, and
this record applies the same requirement to the tests. The stakes peak at the controls. The
fail-closed pillar names its own limit. Its paths are exercised by tests or not at all, and where
tests are the only evidence, a test that proves nothing is worse than none, since
[docs/VERIFICATIONS.md](../../VERIFICATIONS.md) would show a green row over a hole in the
invariant's proof. The scopes below are deliberate. The stance binds every test in the suite and no
coverage percentage is imposed anywhere, the never-retire rule binds the tests that prove
verification rows, the mutation table binds only an automatable control's tests, the
independent-expectation rule binds a control's tests, and the violation-file rule binds a lint ban
that stands in for a control.

## Decision

- **Ask of every test, what would make it fail?** A test with no answer proves nothing, and no
  test is trusted until it has been seen to fail. Seeing it fail takes no ceremony. The author
  makes the test fail once, by handing it the violation it exists to catch or by breaking the
  thing it exercises, watches it go red, and restores the green. For an ordinary test nothing
  records this. An automatable control's tests repeat the act on the record, as the mutation
  table below demands.
- **A control's test never reads the value it asserts from the code under test.** A test that
  compares a served value to the constant the code under test was built from stays green when that
  constant is broken. The expected value is written out in the test, independently.
- **A lint ban that stands in for a control is itself proven by a checked-in file that violates
  it**, with a script that demands the linter go red on each, because a ban can read correctly and
  match nothing.
- **No coverage percentage.** What must be tested is the rows of
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) and each roadmap unit's criteria. A
  percentage of lines executed measures neither.
- **An automatable control's acceptance includes its mutation table.** Break the mechanism both
  ways, once so it does less and once so it does the wrong thing, each break its own patch, demand
  the tests go red on each, and record which tests failed. A check against an absolute value catches
  the first kind of break, and often only a comparison catches the second, so one break leaves half
  of what the tests must catch unproven
  ([ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) measured both in its crash
  harness). The harness is a checked-in patch per break and a script that applies each, runs the
  tests, demands red, and records which tests went red before restoring green. The table is the
  whole artifact, and a pass rate never is. A control with no standing
  automated test carries no table, since nothing exists to demand red from, and proof only by drill
  or by manual exercise is that case.
- **A surviving mutant is a defect on the spot.** Either the tests are vacuous or the
  mechanism is redundant, and each finding demands its own action. A mechanism not worth
  testing well is a candidate for removal.
- **The obligation is event-driven per control, never a standing gate.** The table is produced
  when the control lands and reproduced when the control, its tests, or a generator its tests
  draw from changes, at no other time. A generator is named because one edited until its report
  passed moved a planted failure out of the gating run's reach while every test stayed green
  ([ADR-0069](./0069-property-and-crash-sequences-from-rapid.md)). Controls are defined in the
  design, or when an outcome (or feature) is added, and verifications are
  defined with them. A mutation demonstration proves the control and its verification work, and
  lands at implementation time.
- **A test that proves a [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) row never retires.**
  Once it passes for the first time, it runs in CI on every change from then on, so the proof
  stays current instead of decaying into a claim about one past date. Rows proven by a drill
  on real infrastructure or by a manual exercise stay claims about their date, as that
  document records them.
- **The demonstrations live beside [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), in their
  own document, [docs/MUTATIONS.md](../../MUTATIONS.md).** The split is by authoring moment.
  A verification row is minted at design time, when the control is decided. A demonstration
  can exist only at implementation time, once a mechanism exists to remove and tests exist to
  fail.

## Alternatives considered

- **A coverage-percentage gate.** No case was tabled for one. Rejected because a percentage
  spends equally on trivial and load-bearing code.
- **Whole-codebase mutation scoring as a standing gate.** The case for it is coverage of
  everything, the default of mutation tooling. Rejected as heavy and noisy, test
  infrastructure whose complexity exceeds the risk it covers, and because a score hides the
  one fact that matters, namely which control's tests survived.
- **Reading a test's expected value from the code under test.** The case for it is one home for a
  constant such as a policy string. Rejected because a test that compares a served value to the
  constant it was built from stays green when the constant is broken, which the first policy test
  written in the spike of [ADR-0063](./0063-browser-app-is-preact-with-signals.md) demonstrated.
- **Trusting a lint ban's selector as written.** The case for it is no extra files to keep. Rejected
  because a ban can read correctly and match nothing, which a ban on the browser's raw-markup hatch
  demonstrated in the same spike by catching nothing until a violation file exposed it.
- **A mutation column on the verification rows.** The case for it is one fewer document.
  Rejected because most row kinds could never fill it, controls with no standing automated
  test having no table, and because it braids a design-time table into an
  implementation-time one.

## Consequences

- The ledger starts empty and stays empty until implementation.
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) began the same way, and that is the correct
  starting state.
- A red that cannot be demonstrated again is a memory, not a proof, which is why the proving
  tests never retire.
- Attributing a red to the removed mechanism assumes each control's tests run in isolation
  well enough to make the attribution.
- The violation-file rule is a control. Its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md). The independent-expectation rule has no
  injection and stands as review discipline, dispositioned there.
- The obligation reaches every automatable control, whichever outcome its row serves. The
  header names C2 because the fail-closed paths, which are C2's, are where tests are the only
  evidence and a test that proves nothing costs the most.
