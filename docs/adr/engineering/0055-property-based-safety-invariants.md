# 0055. Property-based tests are safety invariants transcribed from the outcomes' falsifiers

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

Property-based testing runs one stated invariant against many generated inputs. The research
the operator brought into the testing discussion locates its value precisely. It finds the
inputs nobody thinks to write down, the orderings, the interactions, and the stacked unlikely
coincidences. The same research locates the traps. Logic with no independent oracle defeats
it, since any property there either mirrors the implementation or checks nothing. And no tool
reports whether a generator ever reached the region where the bugs are.

## Decision

- **Each property is one of the outcome contract's falsifiers, made executable.** The contract
  already states what would falsify every outcome, and those statements are the properties
  worth running. Two of the shape, from commitments already landed, are that no message-body
  value carrying restricted sensitivity can be constructed and that scanner outputs and logs
  never contain fixture body text. Properties are transcribed from that material, not
  invented.
- **No oracle, no property.** Policy and selection logic goes to curated examples instead
  ([ADR-0044](./0044-layered-testing-strategy.md)).
- **Gating runs are bounded and deterministic.** The deep randomized search happens out of
  band. A failing example, once found, is stored and replayed forever.
- **The mutation obligation is the vacuity check.** No tooling can see a generator that misses
  the interesting region, so the red demonstration of
  [ADR-0046](./0046-mutation-obligation.md) is what catches a property that could never fail.

## Alternatives considered

- **Using properties on the policy and selection logic.** Weighed against the research and
  rejected. That logic is the technique's worst case for want of an oracle, and the curated
  example tables carry it.
- **Hunting for properties, as the project this doctrine was adapted from did.** That project
  hunted and found a single denylist-oracle property worth keeping. Not this project's method,
  because a design built on falsifiable deny-by-default outcomes already carries its
  invariants in the contract, ready to transcribe.

## Consequences

- Properties bound to the contract survive rewrites that preserve the outcomes.
- Crash-injection testing ([ADR-0045](./0045-crash-injection-testing.md)) is a different
  subject under test, not a neighboring flavor of this record. Properties here run ordinary
  inputs against invariants. The crash harness runs crash points inside operation sequences
  and checks recovery afterward. The finalized tooling note has them sharing a property
  library's generative machinery, and they share nothing else.
- The property library chosen at implementation time must supply bounded deterministic runs
  and stored-example replay.
