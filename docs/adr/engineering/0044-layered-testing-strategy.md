# 0044. Tests are layered by disjoint bug class, and every green must be able to go red

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The system's controls have a proof standard. A control is a rule the system enforces, and
[docs/VERIFICATIONS.md](../../VERIFICATIONS.md) proves one by creating the violation it exists
to stop and watching the refusal happen. The tests themselves have had no equivalent standard.
A passing test shows only that nothing errored, and a signal that cannot fail carries no
information. The known shapes of that failure are a test whose setup avoids the case and passes
by construction, a check that passes while its step silently produced nothing, and a generated
test that passes because its generator never produced an input that could expose the bug.

## Decision

- **One question governs every test signal.** What would make this signal go red? A test or
  check with no answer is not evidence, and any green is distrusted until it has been seen to
  fail at least once.
- **Four test layers, each kept because it alone catches one class of bug.**

  | Layer | The bug class only it catches | Decided in |
  | --- | --- | --- |
  | Curated example tables | Wrong answers in policy and selection logic. No independent way to compute the expected answer exists there, so hand-picked cases with hand-checked answers are the only honest test, and the tables double as documentation of intent | this record |
  | Property-based tests | Failures on inputs nobody writes by hand, meaning orderings, interactions, and several individually unlikely conditions arriving together. Sequences that include a crash belong to the crash layer | [ADR-0055](./0055-property-based-safety-invariants.md) |
  | Integration tests against the real dependency or a contract-tested fake | The real dependency behaving differently from what the code assumed. Only the real-dependency half catches this, the bound the no-mocking record states | [ADR-0043](./0043-no-mocking.md) |
  | Crash-injection stateful tests | Corrupted state or stuck recovery when a crash lands inside an operation sequence | [ADR-0045](./0045-crash-injection-testing.md) |

- **The mutation table is the check on the layers, not a layer.** It catches no system bugs.
  For every control whose tests run automatically, it demands that the tests fail when the
  control's mechanism is removed ([ADR-0046](./0046-mutation-obligation.md)).
- **Test data is synthetic.** The fixtures the tests run against, meaning the made-up email
  messages, senders, and bodies, never include real mail, because a committed real message is
  the very content this system exists to protect and git history keeps it forever. Fixture
  bodies carry designed marker text, so a test can search any output for leaked content and
  get a deterministic answer. The operator's real mail contributes only formats and patterns
  to reconstruct synthetically. The messages themselves never land.
- **A test that proves a [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) row never retires.**
  Many rows there describe violations an automated test can inject. Once such a test passes
  for the first time, it runs in CI on every change from then on, so the proof stays current
  instead of decaying into a claim about one past date. Rows proven by a drill on real
  infrastructure or by a manual exercise stay claims about their date, as that document
  records them.
- **No coverage percentage.** What must be tested is the rows of
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) and each roadmap unit's criteria. A
  percentage of lines executed measures neither.
- **Every real defect is attributed to the layer that caught it.** The table above is a
  prediction. Recording the actual catches is what lets the strategy be tuned from evidence,
  which is the measured-risk pillar applied to the strategy itself. Where these attributions
  are kept is decided when the first real defect arrives.

## Alternatives considered

- **A coverage-percentage gate.** No case was tabled for one. Rejected because a percentage
  spends equally on trivial and load-bearing code.

## Consequences

- The layering depends on code shape. Per the operator's ruling, the integration surface stays
  small through how code is organized and structured, the split
  [ADR-0040](./0040-pure-core-decisions-as-values.md) records. Pure cores take the cheap
  exhaustive layers, shells take the few integration tests, and this assumes that shape holds
  everywhere.
- A mechanism not worth testing well is a candidate for deletion rather than for bad tests.
- The first backfill run keeps the job of validating the canonical mapping against messy real
  data, where the roadmap placed it. Fixtures never pretend to carry that load.
- The layer records keep their own outcomes. The contract-suite outcomes stay with
  [ADR-0043](./0043-no-mocking.md) and the recovery outcomes with
  [ADR-0045](./0045-crash-injection-testing.md). What this record's own rules protect directly
  is C2, through the fixture and marker discipline.
