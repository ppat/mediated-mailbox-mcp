# 0057. Every analysis lens reads through one dataset endpoint behind a registry

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

[ADR-0021](../mutation/0021-approval-surface.md) puts a thin read API under the UI, and
[ADR-0042](../engineering/0042-implementation-stack.md) generates the browser's types from that
API's contract. The lens model of [docs/UI.md](../../UI.md#3-the-lens-model) means every
analytical view asks the same question of a different dataset. This grouping, these filters, this
level. The read API can answer that with one endpoint per screen or with one endpoint for all of
them. The choice decides what adding a view costs and how bounded the read surface stays.

## Decision

- **One dataset endpoint serves every analysis lens.** It takes the account, a dataset name, a
  grouping, filters, a level, and a page, and returns aggregates or rows in one fixed shape.
- **A registry in Go is the single source of what can be asked.** It declares, per dataset, the
  allowed dimensions, the allowed filters, the row type, and the query that serves each. The
  contract document and the browser's types are generated from it. A dataset or dimension the
  registry does not declare cannot be requested. The registry is what keeps the endpoint from
  becoming an open query surface.
- **A new analysis view is a registry entry and a row renderer.** No new endpoint, no new
  hand-written types.
- **Bespoke endpoints exist only where a screen needs a shape the ladder does not produce.** The
  plan reviewer's summary and label operations, the jobs surfaces, and the two verbs.

This is the move [ADR-0053](../engineering/0053-parity-by-construction.md) makes for the client
surface, applied to the UI's read API.

## Alternatives considered

- **One endpoint per screen**, returning exactly what that screen shows. Its case was simple, typed,
  nothing generic to design. Not chosen because every new screen is new Go code, a new contract
  entry, new generated types, and its own drill-down code in the browser, so the UI's growth is
  linear in pages and the ladder is re-implemented per screen.

## Consequences

- Aggregates are computed in the database and the corpus never ships to the browser. Every zoom
  step is one request against the registry's query for that dataset.
- The registry bounds the read surface structurally. Two enumerated sources feed one contract
  generator, the dataset registry and the list of bespoke handlers, and a route outside both
  fails the build. The check that guards the surface is the generator's monopoly, as in
  [ADR-0053](../engineering/0053-parity-by-construction.md).
- Assumptions about other components: the datasets are the tables of
  [ADR-0016](../data/0016-schema.md). The UI's database role reads them under
  [ADR-0021](../mutation/0021-approval-surface.md)'s grants. The account identifier is mandatory
  on every request, as [ADR-0047](../data/0047-schema-first-data-access.md) already requires of
  every data-access function.
- The registry's refusal of an undeclared dataset or dimension is a control. Its violation
  injection is catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
