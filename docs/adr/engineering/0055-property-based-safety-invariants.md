# 0055. Property-based tests are safety invariants transcribed from the outcomes' falsifiers

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

Property-based testing generates many inputs against one stated invariant instead of asserting
on hand-picked cases. The research brought into the testing discussion holds that what it
actually catches is the inputs nobody enumerates, meaning interaction and ordering space and
conjunctions of independently unlikely conditions. The same research names its limits. Policy
and selection logic has no independent oracle, so a property there either restates the
implementation or degenerates into asserting that nothing crashes. And the tooling gives no
signal that distinguishes a sharp property from a vacuous one whose generator never reached the
interesting region. The operator's ruling places the technique where it has purchase and
nowhere else.

## Decision

- **Properties are transcribed from the outcomes' falsifiers, never hunted for.** The outcome
  contract states, for each outcome, what would falsify it, and a property is the executable
  form of one such falsifier. Examples of the shape from the landed documents are that no
  constructable message-body value carries restricted sensitivity, that no scanner output or
  log line contains fixture body text, and that a gate denial produces zero provider calls.
  Because the falsifiers are implementation-independent, the properties constrain future
  rewrites instead of dying in them.
- **A property exists only where an oracle exists.** A property that restates the
  implementation proves nothing. Policy and selection logic is the technique's weakest case
  and is tested by curated examples instead
  ([ADR-0044](./0044-layered-testing-strategy.md)).
- **Where properties gate, they run bounded and deterministic.** Deep randomized search runs
  out of band, never in the gate. A discovered failing example is remembered and replayed on
  every run afterward, never rolled for again.
- **The vacuous-property risk is answered by the mutation obligation.** The tooling cannot
  tell a sharp property from one whose generator never reaches the interesting region, so the
  check is external. [ADR-0046](./0046-mutation-obligation.md) demands the red be
  demonstrated.

## Alternatives considered

- **Property-based testing for the policy and selection logic.** Considered against the
  research and rejected on the oracle problem above. Curated examples carry that logic
  ([ADR-0044](./0044-layered-testing-strategy.md)).
- **Hunting for properties, as the source project this doctrine was adapted from did.** That
  project hunted and found one denylist-oracle property worth having. Rejected as this
  project's method because the design is built on falsifiable outcomes with a deny-by-default
  direction, so the invariants worth executing already exist in the outcome contract and are
  transcribed rather than hunted.

## Consequences

- The properties track the outcome contract rather than the implementation, so a rewrite that
  preserves the outcomes keeps its tests.
- The line against crash-injection testing
  ([ADR-0045](./0045-crash-injection-testing.md)) is the subject under test. Properties here
  generate ordinary inputs and check invariants over the results. The crash harness generates
  crash points inside operation sequences and checks persistence and forward progress after
  recovery. The two share the property library's generative machinery and nothing else.
- This decision assumes the property library settled at implementation time provides bounded
  deterministic runs and replay of stored failing examples.
