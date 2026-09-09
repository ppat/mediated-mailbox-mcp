# Testing

How this project tests, for the implementer. This document states the strategy and cites the
decisions behind it by number, resolved through the
[decision-record index](./docs/adr/README.md). It never restates a record's argument. It holds
no test scenarios, which live in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md), and no
mutation demonstrations, which live in [docs/MUTATIONS.md](./docs/MUTATIONS.md). It moves when
a record mints a change to the strategy, never ahead of one.

## The stance

One question governs every test signal. What would make this signal go red? A test or check
with no answer is not evidence, and any green is distrusted until it has been seen to fail at
least once (ADR-0044).

## The layers

Four layers, each kept because it alone catches one class of bug (ADR-0044).

| Layer | Catches | Decided in |
| --- | --- | --- |
| Curated example tables | Wrong answers in policy and selection logic, where no independent way to compute the expected answer exists | ADR-0044 |
| Property-based tests | Failures on inputs nobody writes by hand, meaning orderings, interactions, and several individually unlikely conditions arriving together. Sequences that include a crash belong to the crash layer | ADR-0055 |
| Integration tests against the real dependency or a contract-tested fake | The real dependency behaving differently from what the code assumed, caught only against the real dependency | ADR-0043 |
| Crash-injection stateful tests | Corrupted state or stuck recovery when a crash lands inside an operation sequence | ADR-0045 |

The per-control mutation table is the check on the layers, not a layer. It demands that an
automatable control's tests fail when the control's mechanism is removed (ADR-0046).

## The instruments

- **Real Postgres, containerized and ephemeral**, for database claims (ADR-0043).
- **The provider fake**, one implementation of the provider port held honest by the contract
  suite every port implementation must pass, the fake and each real adapter alike (ADR-0043).
- **Synthetic fixtures carrying marker text**, the made-up messages the tests run against, so a
  leak search over any output surface is deterministic. Real mail never enters the repository
  (ADR-0044).
- **The crash harness**, generating crash points inside operation sequences and checking
  persistence and forward progress after recovery (ADR-0045).
- **The mutation tables**, one per automatable control at acceptance, recorded in
  [docs/MUTATIONS.md](./docs/MUTATIONS.md) (ADR-0046).

## Proof lifecycle

A test that proves a [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) row runs in CI on every
change once it first passes. Rows proven by a drill on real infrastructure or by a manual
exercise stay claims about their date (ADR-0044).

## Deliberately absent

- **Mocks, anywhere** (ADR-0043).
- **Coverage-percentage targets** (ADR-0044).
- **A test-first mandate** (ADR-0044).

## Not yet here

Where each kind of test lives in the repository. The decision placing tests in directories has
not landed, and the section arrives with that record.
