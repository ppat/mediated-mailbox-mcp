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
([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)). A control's test
never reads the value it asserts from the code under test, and a lint ban that stands in for a
control is proven by a checked-in file that violates it
([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)).

## What to test, with what

Every row whose description matches applies, and the control row applies on top of whichever
row describes the control's code.

| The thing under test | The tests it gets | Decided in |
| --- | --- | --- |
| Pure-core logic | Example-based tests, comparing the returned value against a literal expected one through the shared comparison options. No property checks what the sender classifier or the scan gate decides, because their policy rules are the only definition of the right answer | [ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md), with the comparison of [ADR-0070](./docs/adr/engineering/0070-unit-comparison-through-one-options-value.md) |
| A rule the documents already state | A property-based test executing that rule against generated inputs, of a kind ADR-0055 admits, generated and reduced by the library [ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md) chooses, alongside the generator report that record has the project write | [ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md), [ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md) |
| A shell against a real dependency | An integration test against the real thing (real Postgres, containerized and ephemeral) or against the provider fake. Never a mock | [ADR-0043](./docs/adr/engineering/0043-no-mocking.md), on [ADR-0040](./docs/adr/engineering/0040-pure-core-decisions-as-values.md)'s core/shell shape, with the container started as [ADR-0068](./docs/adr/engineering/0068-test-substrate-containers-directly.md) decides |
| Any implementation of the provider port, the fake included | The one contract suite every port implementation must pass | [ADR-0043](./docs/adr/engineering/0043-no-mocking.md) |
| Sequence-dependent stateful machinery where a crash is silent and hard to reverse | Generated crash-injection sequences from the crash harness, whose sequences and their reduction come from the same library the properties use, with a fresh operation mix drawn for each sequence in the scheduled run, plus the physical drill the verification catalogue demands. ADR-0045 commits the first two targets, the reorg apply/rollback path and backfill resume, and defers any others | [ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md), [ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md) |
| The assembled system on a bare cluster | The chart's own tests and the top-level chainsaw suite | [ADR-0052](./docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md), [ADR-0054](./docs/adr/engineering/0054-one-repository-flat-layout-naming-convention.md) |
| The UI's browser rendering layer, the rows, the ladder, and each screen | Example-based tests run under bun against a DOM shim, on fixture responses recorded from the real server and carrying the metadata marker text, asserting every marker arrives as text in the form ADR-0064 requires | [ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md), on [ADR-0044](./docs/adr/engineering/0044-synthetic-fixtures-marker-text.md)'s markers and [ADR-0056](./docs/adr/operability/0056-ui-organized-around-the-operators-work.md)'s rendering rule |
| A rule the compiler does not check, standing in for a control | The check that enforces it, usually a linter and sometimes a search where no linter reaches, plus the checked-in file that violates it and the script requiring the check to report it | [ADR-0071](./docs/adr/engineering/0071-static-enforcement-toolchain.md) on the server, [ADR-0072](./docs/adr/engineering/0072-browser-bans-under-oxlint-and-ast-grep.md) in the browser, on [ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)'s violation-file rule |
| A control, meaning a rule the system enforces | The tests that prove its [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) row where the row is automatable, plus its mutation demonstration at acceptance | [ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md) |

**In standard vocabulary.** This project's unit tests are the example-based and the
property-based tests. Every property-based test is a unit test, and not every unit test is
property-based. Its integration tests are the shell tests against real dependencies or the
contract-tested fake, and mocks do not exist here in any form. The chart and chainsaw suites
are its end-to-end tests.

## When tests run

The gating CI suite carries the permanent verification-proving tests, the bounded deterministic
property runs, and the bounded fixed-seed crash runs if they prove fast enough. Deep random search
and deep crash-sequence exploration run scheduled, never gating
([ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md),
[ADR-0045](./docs/adr/engineering/0045-crash-injection-testing.md)). A gating property run takes a
fixed seed and case count, and a failing property input, once found, is stored and replayed on every
later run ([ADR-0055](./docs/adr/engineering/0055-property-based-safety-invariants.md)). The seed
and count reach the gating run through the environment, the failing case is kept in a store that
survives an edit to its generator, and the reduced input is also written out as an example-based
test ([ADR-0069](./docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)). The
browser's rendering tests run in the gating suite on every change under the UI's directory, and the
one assertion of the content security policy that only a browser can make is a drill
([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)). Drills
run at the unit that owes them, and their proof holds only for that date.

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
   [docs/MUTATIONS.md](./docs/MUTATIONS.md). The mechanism is removed by a
   checked-in patch, the tests must go red, and the script records which ones did
   ([ADR-0046](./docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)).

The two catalogues differ by what they prove and when they can be written. A verification row
proves the control stops the violation it exists to stop, and it is minted at design time. A
mutation demonstration proves the control's tests would notice if the mechanism were removed,
and it can exist only at implementation time. The first tests the system. The second audits
the tests, and catches no system bugs of its own.

## Test data

Every mail fixture is synthetic, and real mail never enters the repository. Fixture bodies and
fixture metadata fields carry designed marker text, so a leak search over any output surface is
deterministic and a rendering surface can be checked for inert text
([ADR-0044](./docs/adr/engineering/0044-synthetic-fixtures-marker-text.md)). The browser's fixture
responses are recordings of the real server's output over those fixtures
([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)).

## Deliberately absent

- **Mocks, anywhere** ([ADR-0043](./docs/adr/engineering/0043-no-mocking.md)), the test runner's
  own mock, spy, and stub functions included, which lint forbids
  ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)).
- **Property-based tests in the browser**
  ([ADR-0064](./docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)).
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
