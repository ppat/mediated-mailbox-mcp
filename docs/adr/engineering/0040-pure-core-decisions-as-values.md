# 0040. Cores are pure and decisions are values; thin impure shells enact them

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O2](../../../USE_CASES.md#o2--observable),
[C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[P3](../../../USE_CASES.md#p3--multi-account)

## Context

The un-braided pillar demands that concerns which change on different schedules stay separable,
and that components know nothing of each other beyond contracts. The system's enforcement
components — the redaction gate, the sender classifier, the mutation authorizer, plan
validation — produce decisions whose fidelity must be provable and whose record must be exact:
an audit row that reconstructs a decision after the fact can drift from what was actually
decided. And the testing strategy demands that fail-closed paths, which production never
exercises, be provable exhaustively and cheaply.

## Decision

- **Every component separates a pure core from an impure shell.** Core code is pure functions:
  no side effects, no I/O, results captured in return values. Everything a core function needs
  arrives as a parameter — the clock included; the core never looks at the environment.
- **Decisions are values.** An enforcement core returns its decision as data — a verdict — and
  never performs the enforcement. The shell enacts the verdict: strips fields, writes the audit
  row, sends the response. Verdict types are built so they structurally cannot carry the content
  they withhold ([ADR-0009](../redaction/0009-scanner-verdicts-carry-no-content.md)'s technique,
  applied to every enforcement core).
- **Nothing is ambient.**
  [ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md) already rules
  that nothing about an account is ambient; this record extends the same rule to every
  dependency: the account identifier and every other dependency arrives as an explicit
  parameter, so what a decision used is visible in its parameter list. The sharp illustrative
  case: no enforcement path reads a thread-local or an implicit "current" anything.
- **Wiring is explicit.** Each deployable has one hand-written composition root that constructs
  its object graph in ordinary code. There is no runtime dependency-injection container: nothing
  resolves implementations by registry, reflection, annotation, or naming convention at runtime.
- **A framework may own the event loop; it may not own the object graph, the transaction
  boundaries, or the invariants.**
- **The core's import boundary is checked, not trusted.** Core code cannot depend on I/O
  machinery, and a build-time import check in CI refuses the dependency. The standing tripwire
  behind the check: a test that reaches for a mock has found a braid — the decision under test
  was not separated from the I/O that feeds it.

## Alternatives considered

- **Braided enforcement** — deciding interleaved with enacting: a function that, while
  classifying or gating, also throws for denials, logs, writes audit rows, and mutates the
  response as it goes. The common shape, and the easy one. Rejected: neither half is testable
  alone, the audit becomes a reconstruction of what probably happened rather than a
  transcription of what was decided, and a dry-run requires carefully skipping side effects
  scattered through the deciding path.
- **A runtime dependency-injection container for the wiring.** The case for it is assembling
  components written by parties who do not know each other, and deferring implementation choice
  to deployment time — neither condition holds here (one team, one operator, adapters chosen at
  build time). Rejected: it trades compile-time wiring errors for runtime resolution, and it
  makes the question "does every path to a client go through the gate" unanswerable by reading
  the code — the answer would live in a registry that only exists at runtime.
- **Purity by review discipline alone, without the structural check.** The cheapest option, and
  the operator's own starting position — on the doubt that a structural check could be easy,
  meaningful, and maintainable. Superseded at finalization once the check proved to be one
  short configuration list: a checked boundary does not erode.

## Consequences

- Four existing commitments stop being disciplines and fall out of the shape: the audit row is
  the serialized verdict (transcription, not reconstruction); a dry-run
  ([ADR-0031](../mutation/0031-dry-run-on-mutating-operations.md)) is running the deciding half
  and skipping the enacting half; whole-batch validation
  ([ADR-0032](../mutation/0032-whole-batch-validation.md)) is collecting every verdict before
  the shell enacts anything; fail-closed paths are exercised by feeding error states in and
  asserting deny-verdicts out — plain data against pure functions.
- Composition happens in each component's own shell, and a deployable's composition root and wiring
  are never shared with another deployable. The only impure code shared between deployables is the
  narrow named libraries of [ADR-0050](./0050-shared-code-pure-or-narrow.md). The testing strategy's
  division of labour — exhaustive cheap tests on cores, few integration tests on shells — leans on
  exactly this shape.
- Assumptions about other components: every enforcement component exposes its decision logic in
  a form callable with plain values, and the shell that enacts a verdict records it without
  re-deriving it.
