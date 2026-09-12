# 0055. A property-based test exists only to execute a safety rule the documents already state

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

A property-based test states one rule and runs it against many generated inputs, where an
example test states one input and its expected answer. Generation finds the inputs nobody
writes by hand, meaning orderings, interactions, and several individually unlikely conditions
arriving together. Two things defeat the technique. The first is logic with no independent
oracle. An oracle here is a way to know the correct answer without running the code under
test, and the sender classifier has none, because the only definition of its right answer is
the policy rules the classifier itself implements, so any property checking its answers
either re-implements the classifier or checks nothing. The second is a vacuous generator. A
property passes identically whether the code is correct or the generator never produced an
input that could expose the bug, and no tooling reports which of the two happened.

## Decision

- **A property exists only to execute a safety rule the documents already state.** The outcome
  contract, the design, and [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) already say what
  must never happen, and a property turns one such statement into a test over generated
  inputs. Two committed examples are that no message-body value carrying restricted
  sensitivity can be constructed, and that no scanner output or log ever contains fixture body
  text. Nobody searches the code for new things to assert. A property worth running expresses
  a safety rule worth stating in the documents first.
- **No property without an independent oracle.** Policy and selection logic has none, and it
  is tested with example-based tests instead. The bound bars a property from checking that
  logic's answers. It does not bar a property executing a stated safety rule over the same
  logic, because the rule itself supplies the expected answer, the same for every input, that
  the violation never happens.
- **Gating runs are bounded and deterministic, and deep random search runs out of band.** A
  failing input, once found, is stored and replayed on every later run.
- **A property that cannot fail is caught by the mutation obligation.** The vacuous-generator
  case is invisible to property tooling, so the demand of
  [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md), that tests go red when the
  mechanism is removed, is the check.

## Alternatives considered

- **Properties that check the policy and selection logic's answers.** No case was tabled for
  one. Rejected because no independent oracle exists there, as the Context explains, and
  example-based tests carry that logic.
- **Searching the code for properties to write.** No case was tabled for one. Rejected
  because a design built on falsifiable deny-by-default outcomes already states its safety
  rules, and a rule worth testing is worth stating in the documents first.

## Consequences

- A property bound to a stated rule survives any rewrite that preserves the rule.
- Crash-injection testing ([ADR-0045](./0045-crash-injection-testing.md)) is a different
  subject under test. A property generates ordinary inputs and checks a rule over the
  results. The crash harness generates crash points inside operation sequences and checks
  recovery afterward.
- The property library must provide bounded deterministic runs and replay of stored failing
  examples. [ADR-0069](./0069-property-and-crash-sequences-from-rapid.md) settles which library,
  and records that a stored failing example is re-read against whatever the generator now
  produces, so editing a generator silently changes what a stored example means. That is why the
  reduced example is also written out as an ordinary example-based test.
