# 0050. Shared code is pure, or it is a narrow, named exception

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Shared libraries are how "no adapter, frontend, tool handler, or batch path holds its own copy
of the rules" is honored across separate processes — classification logic alone runs in at
least two deployables. But the shared layer is also where the braid regrows: a grab-bag core
library becomes a dumping ground where every component transitively depends on everything, and
the monolith is rebuilt one layer down. What accretes into such a library is machinery —
database helpers, clients, configuration loading, connection lifecycle — and that machinery is
what creates operational coupling and drags dependencies into every image, the trust anchor's
included.

## Decision

- **The shared pure library — `mediated-mailbox-core` — contains only pure core code.** No I/O, no
  composition, no side effects: depending on it costs a component nothing operationally — no
  connection held, no availability inherited, no transitive driver
  ([ADR-0040](./0040-pure-core-decisions-as-values.md) defines the purity).
- **Irreducibly-impure shared needs never widen the core.** The default is thin per-component glue —
  duplicating trivial impure glue is cheaper than coupling. The exception is a separate, narrow,
  single-concern library that argues its own case. The first such exception, argued and accepted:
  the shared data-access library ([ADR-0047](../data/0047-schema-first-data-access.md)). Six more
  are argued each on its own case, in the README of the library's own directory. They are the
  [provider library](../../../provider/README.md), the [rate limiter](../../../ratelimit/README.md),
  the [shared test support](../../../testsupport/README.md), the
  [body sanitization library](../../../sanitize/README.md), the
  [policy loader](../../../policyload/README.md), and the
  [credential library](../../../credential/README.md). An exception library may hold pure
  packages of its own, and those sit under the same core import check as `mediated-mailbox-core`.
- **Purity is the first fence; the concern-cut is the second, and it is deferred.** A wholly
  pure library can still lump unrelated concerns into one dependency unit; cutting `mediated-mailbox-core`
  by concern waits until a second concern actually shows up, rather than being speculated in
  advance.

## Alternatives considered

- **One general shared library holding whatever every component needs.** The case for it: one
  obvious home, the least ceremony. Rejected: it is the dumping ground — impure machinery is
  what accretes, every component ends up transitively depending on everything, and the
  monolith returns one layer down with its dependency tree inside every image.
- **No shared code — batch workloads call the mediator for shared decisions.** The case for it:
  nothing shared at all. Rejected: it couples every batch workload to the mediator's
  availability, defeating the point of separate workloads
  ([ADR-0022](../operability/0022-four-workloads.md)).
- **Per-component copies of the shared logic.** Not weighable: the landed one-gate pillar
  already forbids any path holding its own copy of the rules.

## Consequences

- The most-depended-on code sits in the most-testable layer: `mediated-mailbox-core` gets the exhaustive
  fixture-and-property treatment, so the widest blast radius lives under the strongest tests.
- Every named impure exception is its own visible dependency decision — reviewable at the
  moment it is created, exactly what the contracts-only pillar wants surfaced.
- Assumptions about other components: composition happens in each component's own shell
  ([ADR-0040](./0040-pure-core-decisions-as-values.md)). Composition roots and wiring are never
  shared, and the named libraries here are the only shared impure code. The import boundaries that
  keep the core pure are checked in CI (the core-imports-no-I/O check).
