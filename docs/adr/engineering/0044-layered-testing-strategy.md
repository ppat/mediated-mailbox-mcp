# 0044. Tests are layered by disjoint bug class, and every green must be able to go red

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The catalogue's acceptance standard — a control is proven by making it fire — governs controls;
it needs an equivalent governing the tests themselves. A green result is evidence that nothing
errored, never that the thing happened, and the worst signals in practice are unfalsifiable
signals read as confirmation: a test that passes by construction, a check that passes while
producing nothing, a generator that never reached the interesting region. Each testing layer
also catches a bug class the others structurally cannot, so the layers are chosen for
disjointness, not accumulated by habit.

## Decision

- **The falsificationist stance governs every test signal: for any signal about to be trusted,
  ask what would make it go red — if there is no answer, it is not evidence yet.** Distrust
  anything green until it has been seen red. An automatable control's acceptance therefore
  includes the red demonstration — mechanism removed, tests failed — which is the mutation
  obligation, [ADR-0046](./0046-mutation-obligation.md).
- **The layers, each carrying the bug class the others cannot reach:**
  - **curated example tests, primary for policy and selection logic** — the classifier and its
    kin have no independent oracle, which is where property-based testing is weakest; the
    examples double as documentation of intent;
  - **a small number of property-based tests phrased as safety invariants**, transcribed from
    the outcomes' own falsifiers rather than hunted for, only where an oracle exists — a
    property that restates the implementation proves nothing. The bug class this family
    reaches is inputs nobody enumerates — interaction and ordering space, and conjunctions of
    independently-unlikely conditions — with the crash-spanning, sequence-dependent share
    belonging to its stateful sibling, [ADR-0045](./0045-crash-injection-testing.md);
  - **integration tests against real dependencies or contract-tested fakes**
    ([ADR-0043](./0043-no-mocking.md)) — the only layer that finds semantic surprises;
  - **crash-injection stateful testing** for sequence-dependent failure
    ([ADR-0045](./0045-crash-injection-testing.md));
  - **the per-control mutation table** as the meta-check on all of the above
    ([ADR-0046](./0046-mutation-obligation.md)).
- **Fixtures are synthetic, always.** No real mail content ever enters the repository — real
  mail in fixtures would be the system's own protected content leaking through its test suite,
  and it is unremovable from history once committed. Fixture bodies carry designed marker text,
  so any test can search an output surface for leakage deterministically. The distinction that
  keeps this compatible with building the real detection corpus: what is harvested from the
  operator's own mail is formats and patterns; what lands in fixtures is synthetic
  reconstructions, never the messages.
- **An automatable injection becomes a permanent test that runs forever in CI** — proof is
  continuous, not a one-time acceptance. Drills on real substrate and manual adversarial
  exercises keep the catalogue's proven-is-a-claim-about-that-date semantics.
- **No coverage-percentage targets.** The ledger of what must hold is the verification
  catalogue plus each unit's criteria, mapped test by test; a percentage measures lines
  touched, not violations proven.
- **Track what each layer actually catches.** The strategy is a set of predictions; recording
  which layer catches each real defect is what lets the next tuning pass rest on data — the
  measured-risk pillar applied to the test strategy itself, which is this record's pillar
  contact point.

## Alternatives considered

- **A test-first (test-driven development) mandate.** Weighed and rejected by the operator; no
  case was tabled for it beyond convention. The rejection: the falsificationist stance plus the
  mutation obligation already force the red to be demonstrable — a general mandate on authoring
  order adds doctrine without adding protection.
- **A coverage-percentage gate.** No case was tabled for it. Rejected as anti-selective: it
  spends equally on trivial and load-bearing code, and a percentage is exactly the kind of
  green that cannot be made to go red meaningfully.
- **One layer stretched to cover everything** (example tests everywhere, or properties
  everywhere). Never weighed as a proposal; stated so the layering's justification is
  re-arguable. The layers' bug classes are near-disjoint — policy logic resists properties for
  want of an oracle, example tests cannot explore interaction and ordering space, and neither
  can find a semantic surprise in a real dependency — so any single layer leaves whole classes
  unreachable.

## Consequences

- Property-based layers run bounded and deterministic where they gate, with deep randomized
  search out of band — a discovered failing example is remembered and replayed, never rolled
  for again.
- The safety-invariant properties are transcribed from the outcomes' falsifiers, so they track
  the outcome contract rather than the implementation.
- Assumptions about other components: every enforcement component follows
  [ADR-0040](./0040-pure-core-decisions-as-values.md)'s shape — pure cores get the exhaustive,
  cheap layers and shells get the few integration tests, and that division of labour
  presupposes the shape holds everywhere.
