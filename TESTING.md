# Mediated Mailbox MCP — Testing

How this project tests: the stance, the layers, and the instruments — the implementer-facing
map of the strategy. This document states the strategy and cites the decisions behind it by
number, resolved through the [decision-record index](./docs/adr/README.md); it never restates a
record's argument. The scenario list — every control's proving injection — lives in
[docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md), and every control's mutation demonstration
lives in [docs/MUTATIONS.md](./docs/MUTATIONS.md); this document holds neither. It sits between
[DESIGN.md](./DESIGN.md) and the records in rate of change: it moves when the strategy moves,
and a layer or instrument enters here only when a record mints it.

## The stance

Confirmation is weak; refutation is strong. A green result is evidence that nothing errored,
never that the thing happened — so for any signal about to be trusted, the question is what
would make it go red, and a signal with no answer is not evidence yet. Controls are proven by
violation injection (the catalogue's standard); the tests themselves are proven by the mutation
obligation — mechanism removed, tests red (ADR-0046). The full stance and its consequences are
ADR-0044.

## The layers

Each layer exists because it catches a bug class the others structurally cannot. That is the
whole justification — no layer is habit.

| Layer | The bug class only it catches | Governing decision |
| --- | --- | --- |
| Curated example tests, primary for policy and selection logic | Wrong decisions in logic that has no independent oracle; the examples double as documentation of intent | ADR-0044 |
| Property-based tests phrased as safety invariants, transcribed from the outcomes' falsifiers | Inputs nobody enumerates: interaction and ordering space, and conjunctions of independently-unlikely conditions — the crash-spanning share belongs to the stateful row below | ADR-0044 |
| Integration tests against real dependencies or contract-tested fakes — never mocks | Semantic surprises in the real dependency's behavior | ADR-0043 |
| Crash-injection stateful testing | Sequence-dependent failures across a crash and recovery | ADR-0045 |
| The per-control mutation demonstration | Vacuous tests — a green that could never go red | ADR-0046 |
| Physical drills and manual adversarial exercises | What only the real substrate and a live adversary can show; the rows in [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) | ADR-0044, ADR-0045 |

## The instruments and their disciplines

- **Real dependencies or contract-tested fakes, never mocks** (ADR-0043). The database layer
  runs against real, containerized Postgres. The provider fake implements the provider port,
  held honest by one contract suite that every port implementation — the fake and every real
  adapter alike — must pass. The rate machinery's throttling provider is an instance of that
  fake.
- **Fixtures are synthetic, always** (ADR-0044). No real mail content enters the repository;
  fixture bodies carry designed marker text so leak-search tests can sweep any output surface
  deterministically.
- **Automatable injections run forever** (ADR-0044). A catalogue row of the automatable kind
  becomes a permanent CI test; drills and manual exercises are proven as of their date. The
  kind semantics are stated in the catalogue's preamble.
- **Randomized layers run bounded and deterministic where they gate, and deep out of band**
  (ADR-0044, ADR-0045). Known failing examples are remembered and replayed, never rolled for
  again.
- **The mutation obligation is per-control and event-driven** (ADR-0046): the table is produced
  by an automated apply-the-patch-expect-red harness when a control lands, and re-produced only
  when the control or its tests change — an absent table is a missing artifact, never a silent
  green; a surviving mutant on a control is an open defect. Demonstrations land in
  [docs/MUTATIONS.md](./docs/MUTATIONS.md).

## Deliberately absent

- **No coverage-percentage targets** — the ledger of what must hold is the verification
  catalogue plus each unit's criteria, mapped test by test (ADR-0044).
- **No test-first mandate** — the stance plus the mutation obligation already force every
  trusted green to have a demonstrable red (ADR-0044).

## Tracking

The strategy is a set of predictions about which layer pays. Which layer actually catches each
real defect is recorded as work proceeds, so tuning rests on data rather than the original
reasoning (ADR-0044).
