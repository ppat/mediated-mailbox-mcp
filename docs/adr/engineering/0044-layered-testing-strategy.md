# 0044. Tests are layered by disjoint bug class, and every green must be able to go red

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

[docs/VERIFICATIONS.md](../../VERIFICATIONS.md) holds the standard for proving the system's
controls. A control is a rule the system enforces, and it is proven by deliberately creating
the violation it exists to stop and watching it refuse. Nothing equivalent governs the tests
themselves. A green test result is evidence that nothing errored. It is never evidence that the
mechanism did its job. The worst signals in practice are unfalsifiable signals read as
confirmation. A test can pass by construction. A check can pass while producing nothing. A
generator can pass without ever reaching the interesting region. Each testing layer in this
record also catches a class of bug the other layers cannot reach, so the layers are chosen for
that reason and not accumulated by habit.

## Decision

- **The falsificationist stance governs every test signal.** Before trusting any signal, ask
  what would make it go red. If there is no answer, it is not evidence yet. Distrust anything
  green until it has been seen red. For an automatable control this includes the red
  demonstration at acceptance. The mechanism is removed and the tests must fail. That
  obligation is [ADR-0046](./0046-mutation-obligation.md).
- **The layers each catch a bug class the others cannot reach.**
  - Curated example tests are primary for policy and selection logic. The classifier and its
    kin have no independent oracle, and that is where property-based testing is weakest. The
    examples double as documentation of intent.
  - A small number of property-based tests are phrased as safety invariants. They are
    transcribed from the outcomes' own falsifiers rather than hunted for, and only where an
    oracle exists. A property that restates the implementation proves nothing. This family
    reaches the inputs nobody enumerates, which are interaction and ordering space and
    conjunctions of independently unlikely conditions. The crash-spanning share of that space
    belongs to [ADR-0045](./0045-crash-injection-testing.md).
  - Integration tests run against real dependencies or contract-tested fakes
    ([ADR-0043](./0043-no-mocking.md)). This is the only layer that finds semantic surprises.
  - Crash-injection stateful testing covers sequence-dependent failure
    ([ADR-0045](./0045-crash-injection-testing.md)).
  - The per-control mutation table is the check on all of the above
    ([ADR-0046](./0046-mutation-obligation.md)).
- **Fixtures are synthetic, always.** No real mail content ever enters the repository. Real
  mail in fixtures would be the system's own protected content leaking through its test suite,
  and it cannot be removed from history once committed. Fixture bodies carry designed marker
  text, so any test can search an output surface for leakage deterministically. What is
  harvested from the operator's own mail is formats and patterns. What lands in fixtures is
  synthetic reconstructions of those formats and never the messages.
- **An automatable injection becomes a permanent test that runs forever in CI.** Its proof is
  continuous rather than a one-time acceptance. Drills on real substrate and manual
  adversarial exercises remain claims about their date, as
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) records them.
- **No coverage-percentage targets.** The ledger of what must hold is
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) plus each unit's criteria, mapped test by
  test. A percentage measures lines touched and not violations proven.
- **Track what each layer actually catches.** The strategy is a set of predictions. Recording
  which layer catches each real defect lets the next tuning pass rest on data. That is the
  measured-risk pillar applied to the test strategy itself.

## Alternatives considered

- **A test-first (test-driven development) mandate.** The operator weighed and rejected it. No
  case was tabled beyond convention. The falsificationist stance plus the mutation obligation
  already force the red to be demonstrable, so a mandate on authoring order adds doctrine
  without adding protection.
- **A coverage-percentage gate.** No case was tabled for it. Rejected because it spends equally
  on trivial and load-bearing code, and a percentage is exactly the kind of green that cannot
  be made to go red meaningfully.
- **One layer stretched to cover everything.** Never weighed as a proposal. It is stated here
  so the layering's justification stays re-arguable. Policy logic resists properties for want
  of an oracle. Example tests cannot explore interaction and ordering space. Neither can find
  a semantic surprise in a real dependency. Any single layer leaves whole bug classes
  unreachable.

## Consequences

- Property-based layers run bounded and deterministic where they gate, with deep randomized
  search out of band. A discovered failing example is remembered and replayed and never rolled
  for again.
- The safety-invariant properties are transcribed from the outcomes' falsifiers, so they track
  the outcome contract rather than the implementation.
- This decision assumes every enforcement component follows
  [ADR-0040](./0040-pure-core-decisions-as-values.md)'s shape. Pure cores get the exhaustive
  cheap layers and shells get the few integration tests, and that division of labour
  presupposes the shape holds everywhere.
