# 0050. Shared code is pure, or it is a narrow, named exception

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Shared libraries are how "no adapter, frontend, tool handler, or batch path holds its own copy
of the rules" is honored across separate processes, since classification logic alone runs in at
least two deployables. But the shared layer is also where the braid regrows. A grab-bag core
library becomes a dumping ground where every component transitively depends on everything, and
the monolith is rebuilt one layer down. What accretes into such a library is machinery, the
database helpers, clients, configuration loading and connection lifecycle, and that machinery is
what creates operational coupling and drags dependencies into every image, the trust anchor's
included.

Read as one separate top-level library per concern, the rule produced a library count that grows
with every ticket. The libraries were each placed by one ticket, with that ticket's view alone,
and the count would keep growing as deployables and core functions arrive. Looked at whole, they
are four kinds shown as one flat list.

1. The environment contract a process meets.
2. The account context with its credentials.
3. Provider access, together with rate limiting, loaded policy and content conversion.
4. Test tooling.

Which libraries change together, and how their packages cluster, agrees with those kinds where
there is enough history to say. The project already uses depth where a role or capability
differs, in `db/`, `provider/` and `testsupport/`, while each one-package library was founded by
one ticket, so the difference is history, not a rule. The fence a separate library was meant to
give, that a component cannot link what it does not need, is held per package by the closed
import lists and by Go's linker, whatever the library boundary is
([ADR-0071](./0071-static-enforcement-toolchain.md)). And the default of per-component glue left
rules copied. The code that re-reads a refused credential, records the latest
authentication attempt and pairs an account with its credentials sat identically, or as variants
of one rule, in every root that calls a provider, and one copy drifted, leaving a hand-over
without the adoption stamp the records said it carried.

## Decision

- **The shared pure library, `mediated-mailbox-core`, contains only pure core code.** It has no
  I/O, no composition and no side effects, so depending on it costs a component nothing
  operationally, with no connection held, no availability inherited and no transitive driver
  ([ADR-0040](./0040-pure-core-decisions-as-values.md) defines the purity).
- **Irreducibly-impure shared needs never widen the core.** Glue that encodes no rule stays per
  component, because duplicating trivial impure glue is cheaper than coupling. Code that encodes a
  rule more than one deployable applies is shared code, not glue, from its second consumer,
  because a rule written in several places fails silently when one copy drifts. Shared impure code
  is a narrow library per concept that argues its own case once, in its own README, and whose
  README also says what does not belong in it. Every package in such a library is admitted to each
  component by an exact entry in that component's import list, so a library holding several
  packages never lets a component link more than it names
  ([ADR-0071](./0071-static-enforcement-toolchain.md)). One such library is the shared
  data-access library ([ADR-0047](../data/0047-schema-first-data-access.md)), which argues its
  case in that record. The others are the libraries the
  [component table in CLAUDE.md](../../../CLAUDE.md#components) names, each arguing its case in its
  own README. A library may hold pure packages of its own, and those sit under the same core import
  check as `mediated-mailbox-core`.
- **Where pure code goes.** Pure code is a component's own when it states that component's
  concern, whoever calls it, such as the rate rules or a configuration section, and it sits in that
  component's `core` sub-package. It is shared, in `mediated-mailbox-core`, when no single
  component owns its concern, such as the classifier or the gate.
- **Purity is the first fence, and cutting by concern is the second, made only when needed.** A
  wholly pure library can still lump unrelated concerns into one dependency unit, so
  `mediated-mailbox-core` is cut by concern once a second concern actually shows up, and never
  speculatively in advance.

**The rules that keep shared libraries from sprawling.** Grouping by concept rather than by ticket
is what stops each new need from founding a library of its own, and grouping by the decision a
library hides is what lets it change for one reason.

- **The principle.** Shared impure code is grouped by the concept it serves and the decision it
  hides, after Parnas's criterion of design decisions likely to change. One family per concept, a
  family being one library of this record holding several packages of one concept, with one README
  case, and its packages fenced exactly by the import lists.
- **When a family exists.** For a concept the Glossary or DESIGN.md's component table names, which
  has its own reason to change.
- **When a concern joins a family.** When it hides a decision of that family's concept, even with
  one consumer. It becomes a package there, admitted exactly by the import lists, so joining a
  family never widens what any component may import.
- **When a family is founded.** Only for a concept the Glossary names that no family covers, by a
  decision record or the operator's ruling, never by one ticket's need. One ticket's view never
  creates a top-level library.
- **Repeated assembly**, logic several composition roots repeat, is a package of its concept's
  family. Composition stays in each root ([ADR-0040](./0040-pure-core-decisions-as-values.md)).
- **A concern's configuration section stays with its concern**, as the process family's README
  says, so the code that owns a section also owns its validation, wherever the
  layering mechanism sits ([ADR-0078](./0078-configuration-layers-through-an-owned-library.md)).
- **The guard against a family turning into a grab bag** is the founding rule, the exact import list
  per package, and each family's README naming what does not belong. A package growing code for a
  concept its family does not serve shows in its import list first, which is where review catches
  it.
- **The falsifier.** A growth item that needs a home no family names. If one appears, the rule set
  has failed for that item, and a family is founded for it by the founding rule, not placed wherever
  the ticket finds convenient.

Placing a concern by these rules is an application of rules already decided, not a decision a
ticket makes, so nothing is created ahead of its consumer. How packages and families are named is
[ADR-0054](./0054-one-repository-flat-layout-naming-convention.md)'s.

## Alternatives considered

- **One general shared library holding whatever every component needs.** The case for it: one
  obvious home, the least ceremony. Rejected: it is the dumping ground. Impure machinery is what
  accretes, every component ends up transitively depending on everything, and the monolith
  returns one layer down with its dependency tree inside every image.
- **No shared code, with batch workloads calling the mediator for shared decisions.** The case
  for it: nothing shared at all. Rejected: it couples every batch workload to the mediator's
  availability, defeating the point of separate workloads
  ([ADR-0022](../operability/0022-four-workloads.md)).
- **Per-component copies of the shared logic.** No case was tabled for it. Not weighable, because
  the one-gate pillar already forbids any path holding its own copy of the rules.
- **One separate top-level library per concern, founded at its second consumer by the ticket that
  needs it.** The case for it: each library's case is argued with a real consumer in hand, and no
  library holds more than one concern. Not chosen, because it turns every new concern into a new
  top-level name and a ticket-local placement argument, adds no fence the package does not already
  carry, and its default of per-component glue left rules copied until one copy drifted.
- **Grouping by lifecycle**, top-level directories for commands, deployables and libraries, with
  the libraries cut as before, one per concern. The case for it: a conventional Go shape, a reader
  sees from a path what runs, what is linked and what never ships, and a ready place for a root
  that composes every deployable. Not chosen, because it leaves the libraries' count and their
  placement ticket by ticket as they were, and costs the largest move.
- **Grouping by capability**, shared impure libraries that put a process inside the trust anchor
  under one directory. The case for it: the trust anchor visible in every import path. Not chosen,
  because a concept whose capability changes would move across directories, which has already
  happened twice, a concept would be split across two places, and the import lists already show
  which side of the anchor each component's code is on.

## Consequences

- The most-depended-on code sits in the most-testable layer. `mediated-mailbox-core` gets the
  exhaustive fixture-and-property treatment, so the widest blast radius lives under the strongest
  tests.
- Every named impure exception is its own visible dependency decision, reviewable at the moment
  it is created, which is exactly what the contracts-only pillar wants surfaced.
- A family's risk is that it becomes a grab bag one level down, a package its other consumers never
  link that changes for a different reason. The founding rule, the exact list per package and the
  README's statement of what does not belong are what guard it, and the import list of a growing
  package is where it shows.
- Assumptions about other components: composition happens in each component's own shell
  ([ADR-0040](./0040-pure-core-decisions-as-values.md)). Composition is never shared, reusable
  assembly is library code, and the named libraries here are the only shared impure code. The
  import boundaries that keep the core pure, and the exact admission of each package of a library,
  are checked in CI ([ADR-0071](./0071-static-enforcement-toolchain.md)).
