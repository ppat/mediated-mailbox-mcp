# 0043. No mocking: tests run against the real dependency or a contract-tested fake

**Status:** Accepted ·
**Serves:** [P1](../../../USE_CASES.md#p1--one-contract),
[P2](../../../USE_CASES.md#p2--backend-swap)

## Context

A mock encodes its author's belief about a dependency and then tests that belief: the suite goes
green by measuring the accuracy of the assumptions it was built from, which is exactly what
there was no reason to trust. The dependencies this system leans on carry claims that only the
real thing can prove — account-scoped query enforcement, schema constraints, and role grants are
claims about the database's behavior, not about ours — while the mail provider is impractical to
run inside a test suite at all.

## Decision

- **Absolutely no mocking, anywhere in the test suite.**
- **Use the real thing where practical.** The database layer tests against real Postgres,
  containerized and ephemeral.
- **Where the real thing is too hard to use, or the complexity it brings is not worth it — the
  mail provider — a fake implements the same contract**: the provider port. The fake gets tests
  of its own, in the form of **one contract suite that every port implementation must pass**,
  the fake and every real adapter alike. That suite is what keeps
  [P1](../../../USE_CASES.md#p1--one-contract)'s single contract an executable artifact rather
  than prose, and it is the standing instrument a future backend's adapter is tested against.
- **The simulated provider that throttles on schedule** — the rate-limiter's test dependency —
  **is an instance of the fake**: the same contract, plus a throttle schedule. One test asset,
  not two.
- **Whether the contract suite ever runs against the real provider, and against what mailbox,
  is deliberately deferred** until the first real adapter is implemented — the complexity and
  the payoff at that point drive it. The open decision is registered in
  [ROADMAP.md](../../../ROADMAP.md).

## Alternatives considered

- **Mocking where provisioning a real fixture is genuinely expensive** — the conventional
  escape hatch, taken on an explicit cost argument. Rejected as a deliberate departure,
  stricter here: the cost case routes to the contract-tested fake instead, because the fake is
  one honest, tested statement of the dependency's behavior where per-test mocks scatter
  untested beliefs across the suite.
- **Conventional mock-based unit isolation.** The case for it: standard practice, fast, no
  test infrastructure. Rejected twice over: a green mock-based suite measures assumption
  accuracy, and reaching for a mock is itself the tripwire that the code under test is braided
  ([ADR-0040](./0040-pure-core-decisions-as-values.md)) — the decision was not separated from
  the I/O feeding it.

## Consequences

- Test quality rests on fakes and containers, not stub frameworks. The fake is a real component
  with maintenance weight, priced in by its own contract-suite obligation.
- A fake built from the provider's documentation encodes beliefs about the provider; the
  semantic-surprise class of defect is caught only by the real thing. That bound is accepted,
  and it is why the real-provider question above stays open rather than closed.
- Assumptions about other components: the provider port's contract
  ([ADR-0010](../provider/0010-one-provider-port.md)) is explicit enough to write the contract
  suite against.
