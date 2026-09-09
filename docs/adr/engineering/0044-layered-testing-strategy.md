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
generator can pass without ever reaching the interesting region.

## Decision

- **The falsificationist stance governs every test signal.** Before trusting any signal, ask
  what would make it go red. If there is no answer, it is not evidence yet. Distrust anything
  green until it has been seen red. For an automatable control this includes the red
  demonstration at acceptance. The mechanism is removed and the tests must fail. That
  obligation is [ADR-0046](./0046-mutation-obligation.md).
- **Tests are layered, and each layer is kept because it catches a bug class the others
  structurally cannot.**
  - Curated example tests are primary for policy and selection logic. The classifier and its
    kin have no independent oracle, and that is where property-based testing is weakest. The
    examples double as documentation of intent.
  - Property-based tests are safety invariants transcribed from the outcomes' falsifiers
    ([ADR-0055](./0055-property-based-safety-invariants.md)).
  - Integration tests run against real dependencies or contract-tested fakes
    ([ADR-0043](./0043-no-mocking.md)). This is the only layer that finds semantic surprises.
  - Crash-injection stateful testing covers sequence-dependent failure
    ([ADR-0045](./0045-crash-injection-testing.md)).
  - The per-control mutation table checks the layers themselves
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
  test. A percentage measures lines touched and not violations proven, and it spends equally
  on trivial and load-bearing code.
- **Track what each layer actually catches.** The strategy is a set of predictions. Recording
  which layer catches each real defect lets the next tuning pass rest on data. That is the
  measured-risk pillar applied to the test strategy itself.

## Alternatives considered

- **A test-first (test-driven development) mandate.** The operator weighed and rejected it.
  The complete coverage is the falsificationist stance everywhere, with crash injection,
  mutation testing, and property-based testing applied where they have purchase. A mandate on
  authoring order adds doctrine without adding protection, since the stance and the mutation
  obligation already force the red to be demonstrable.
- **A coverage-percentage gate.** No case was tabled for it. Rejected because a percentage is
  exactly the kind of green that cannot be made to go red meaningfully.

## Consequences

- The layering leans on code shape. Pure cores take the exhaustive cheap layers and the thin
  shells take the few integration tests, and the operator's ruling that the integration
  surface stays small through how code is organized is
  [ADR-0040](./0040-pure-core-decisions-as-values.md)'s territory. This division of labour
  presupposes that shape holds everywhere.
- A mechanism not worth testing well raises the question of whether it is worth having.
  Deleting an unsafe mechanism is preferred over testing it badly.
- The messy-real-data job stays where the roadmap put it. The first backfill validates the
  canonical mapping against the live corpus, and fixtures do not pretend to carry that load.
