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
- **A new analysis view is a registry entry, a row renderer, and the statements that serve it.**
  No new endpoint and no new hand-written types. The statements are not free, because the endpoint
  is served by an enumerated set rather than by a composer
  ([ADR-0066](../data/0066-data-access-generated-from-sql.md)), so a new dataset also costs one
  statement per groupable dimension plus a summary and a rows query.
- **A paged read is totally ordered.** Every sort the endpoint serves ends with the row's own
  identity as its final key. Every default sort the datasets declare is over a column whose values
  repeat, the rules dataset's sort by rule identifier excepted, so without a unique final key two
  rows with equal sort values have no defined order between them, and the same query run twice can
  place a row on either side of a page boundary. The reader then sees a row twice or never sees
  it. How the rule is checked is a mechanism question and belongs to
  [ADR-0066](../data/0066-data-access-generated-from-sql.md).
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
  [ADR-0053](../engineering/0053-parity-by-construction.md). The generator's tools are
  [ADR-0065](../engineering/0065-contract-built-from-registry-consumed-as-generated-types.md)'s.
- Assumptions about other components: the datasets are the tables of
  [ADR-0016](../data/0016-schema.md). The UI's database role reads them under
  [ADR-0021](../mutation/0021-approval-surface.md)'s grants. The account identifier is mandatory
  on every request, as [ADR-0047](../data/0047-schema-first-data-access.md) already requires of
  every data-access function.
- The registry's refusal of an undeclared dataset or dimension is a control. Its violation
  injection is catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
