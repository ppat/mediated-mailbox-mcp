# 0044. Tests are layered by disjoint bug class, and every green must be able to go red

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

A control is a rule the system enforces. [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) proves
each control by creating the violation it stops and watching the refusal happen. The tests
themselves have had no such standard. When a test passes, the only thing shown is that nothing
errored, and a signal that cannot fail carries no information. Practice supplies the shapes this
takes. A test whose setup avoids the case passes by construction. A check passes while its step
silently produces nothing. A generator passes without reaching the region that matters.

## Decision

- **Ask of every test signal what would make it go red.** A signal with no answer is not
  evidence yet, and a green is distrusted until it has been seen red. At acceptance an
  automatable control must show the red directly. The mechanism is removed and the tests must
  fail, which is the obligation of [ADR-0046](./0046-mutation-obligation.md).
- **Each layer earns its place by catching what the others cannot.**
  - Policy and selection logic gets curated example tables first. No independent oracle exists
    for it, so the examples are the tests and the documentation of intent at once.
  - Safety-invariant properties come from the outcome contract
    ([ADR-0055](./0055-property-based-safety-invariants.md)).
  - The integration layer runs the real dependency or a contract-tested fake
    ([ADR-0043](./0043-no-mocking.md)), and it is the only place semantic surprises show up.
  - Sequence-dependent failure belongs to crash-injection testing
    ([ADR-0045](./0045-crash-injection-testing.md)).
  - The mutation table ([ADR-0046](./0046-mutation-obligation.md)) checks that the layers
    themselves can fail.
- **Every fixture is synthetic.** Real mail never enters the repository, because a committed
  message is the very content this system exists to protect and git history keeps it forever.
  Fixture bodies carry marker text designed for searching, which makes a leak check over any
  output surface deterministic. The operator's real mail contributes formats and patterns only,
  and fixtures hold reconstructions of those formats, never the messages.
- **An automatable injection runs in CI forever once proven.** Its proof is continuous, not a
  one-time event. Drills on real substrate and manual adversarial exercises stay dated claims,
  exactly as [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) treats them.
- **No coverage percentage anywhere.** What must hold is listed in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) and in each unit's criteria. A percentage
  counts lines and spends the same on trivial code as on load-bearing code.
- **Record which layer catches each real defect.** The layering is a set of predictions, and
  the measured-risk pillar applies to the strategy itself. Data from real defects drives the
  next tuning pass.

## Alternatives considered

- **A test-driven-development mandate.** Rejected by the operator. The standing set is the
  falsificationist stance everywhere plus crash injection, mutation testing, and
  property-based testing where they have purchase, and that set already covers what an
  authoring-order rule would add.
- **A coverage-percentage gate.** No case was tabled for one. A percentage is a green that
  cannot meaningfully go red.

## Consequences

- The strategy leans on code shape. The operator's ruling keeps the integration surface small
  through how code is organized, which is
  [ADR-0040](./0040-pure-core-decisions-as-values.md)'s split. Pure cores carry the cheap
  exhaustive layers and the shells carry the few integration tests, everywhere.
- A mechanism that is not worth testing well is a candidate for deletion, not for bad tests.
- Validating the canonical mapping against messy real data stays with the first backfill run,
  where the roadmap put it. Fixtures do not carry that job.
