# Testing

How this project tests, for the implementer. This document answers two questions in one place.
What tests must a piece of work have, and what proves the work done? Each answer links the
decision record that argued it, and the records hold the arguments. Test scenarios live in
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md), and mutation demonstrations live in
[docs/MUTATIONS.md](./docs/MUTATIONS.md). This document moves when a record mints a change to
the strategy, never ahead of one.

## The stance

Ask of every test, what would make it fail? A test with no answer proves nothing, and no test
is trusted until it has been seen to fail. Seeing it fail takes no ceremony. Make the test
fail once, by handing it the violation it exists to catch or by breaking the thing it
exercises, watch it go red, then restore the green. A drill or a manual exercise has nothing
to demand red from, and the discipline does not reach it. For an ordinary test nothing
records this, and an automatable control's tests repeat the act on the record
([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)).

## What to test, with what

Every row whose description matches applies, and the control row applies on top of whichever
row describes the control's code.

| The thing under test | The tests it gets | Decided in |
| --- | --- | --- |
| Pure-core logic | Example-based tests. Where the logic has no independent oracle (the sender classifier's policy decisions, the scan gate's skip decisions), no property checks its answers | [ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md) |
| A safety rule the documents already state | A property-based test executing that rule against generated inputs | [ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md) |
| A shell against a real dependency | An integration test against the real thing (real Postgres, containerized and ephemeral) or against the provider fake. Never a mock | [ADR-0043](./docs/adr/engineering/0043-no-mocking.md), on [ADR-0040](./docs/adr/engineering/0040-pure-core-decisions-as-values.md)'s core/shell shape |
| Any implementation of the provider port, the fake included | The one contract suite every port implementation must pass | [ADR-0043](./docs/adr/engineering/0043-no-mocking.md) |
| Sequence-dependent stateful machinery where a crash is silent and hard to reverse | Generated crash-injection sequences from the crash harness, plus the physical drill the verification catalogue demands. ADR-0045 commits the first two targets, the reorg apply/rollback path and backfill resume, and defers any others | [ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md) |
| The assembled system on a bare cluster | The chart's own tests and the top-level chainsaw suite | [ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md), [ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md) |
| A control, meaning a rule the system enforces | The tests that prove its [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) row where the row is automatable, plus its mutation demonstration at acceptance | [ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md) |

**In standard vocabulary.** This project's unit tests are the example-based and the
property-based tests. Every property-based test is a unit test, and not every unit test is
property-based. Its integration tests are the shell tests against real dependencies or the
contract-tested fake, and mocks do not exist here in any form. The chart and chainsaw suites
are its end-to-end tests.

## When tests run

The gating CI suite carries the permanent verification-proving tests, the bounded
deterministic property runs, and the bounded fixed-seed crash runs if they prove fast enough.
Deep random search and deep crash-sequence exploration run scheduled, never gating
([ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md),
[ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)). A failing property
input, once found, is stored and replayed on every later run
([ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md)). Drills run at
the unit that owes them, and their proof holds only for that date.

## The proof system

The chain from outcome to evidence, stated once.

1. Every outcome in [USE_CASES.md](./USE_CASES.md) carries falsifiable acceptance criteria,
   and controls are the rules the system enforces to meet them.
2. Deciding a control mints its proving injection as a row in
   [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md), at design time. The row states what
   deliberate violation must be refused and what the refusal proves.
3. A row is proven in one of three ways, and the way decides whether the proof stays current
   or holds only for its date
   ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)). An
   automatable injection becomes a permanent CI test that never retires. A drill and a manual
   exercise prove only that the control worked on the day they ran, as
   [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) records them.
4. At implementation, every automatable control also gets its mutation demonstration in
   [docs/MUTATIONS.md](./docs/MUTATIONS.md). The mechanism is removed and the tests must go
   red ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)).

The two catalogues differ by what they prove and when they can be written. A verification row
proves the control stops the violation it exists to stop, and it is minted at design time. A
mutation demonstration proves the control's tests would notice if the mechanism were removed,
and it can exist only at implementation time. The first tests the system. The second audits
the tests, and catches no system bugs of its own.

## Test data

Every mail fixture is synthetic, and real mail never enters the repository. Fixture bodies carry
designed marker text, so a leak search over any output surface is deterministic
([ADR-0044](./docs/adr/engineering/0044-synthetic-fixtures-marker-text.md)).

## Deliberately absent

- **Mocks, anywhere** ([ADR-0043](./docs/adr/engineering/0043-no-mocking.md)).
- **Coverage-percentage targets**
  ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)).

## Where tests live

Each deployable's tests live inside that deployable's own directory, and shipped images carry
no tests ([ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md),
[ADR-0049](./docs/adr/engineering/0049-image-per-component-lockstep.md)). The chart's Helm
tests live inside the chart and travel in the published chart artifact, an exception ADR-0054
accepts knowingly. The chainsaw suite lives at the repository's top level because it tests
the assembled system rather than any one deployable
([ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md),
[ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)).
