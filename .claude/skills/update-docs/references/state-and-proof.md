# Build state, proof ledgers and front doors

Used by nearly every ticket's pull request. Touches `ROADMAP.md` and the ticket epic on GitHub,
`docs/VERIFICATIONS.md`, `docs/MUTATIONS.md`, `README.md`, `CLAUDE.md`, and the rules under
`.claude/rules/` that mirror CLAUDE.md's code conventions.

## Roadmap changes

- Cutting tickets: a ticket is cut from the template to the rules of
  [CLAUDE.md](../../../../CLAUDE.md#repository-process), and the ticket is added as a sub-issue of
  [#118](https://github.com/ppat/mediated-mailbox-mcp/issues/118), and its row under its unit in the
  epic's body goes to the control session, which writes the body, as CLAUDE.md states. A unit recut
  here recuts its open tickets, as CLAUDE.md states.
- Delivered work: move the unit to the delivered register with `[x]`, its outcomes, its tickets or
  that it was delivered before any were cut, and state what it did NOT deliver. Re-date
  `**Position:**` whenever checklists are reconciled against reality.
- New unit: group letter + next index, and a retired identifier is never reused; header
  `**ID — name** → <one outcome> · <value increment> · finishes at <tested | image | packaged>`;
  prose body naming the records whose mechanisms the unit carries, what proves it, and which of its
  proofs wait for a production point; `*Criteria:*` for observability riders. One outcome per unit
  — a genuine exception is flagged in the group preamble, out loud.
- New value increment: `**Units:** / **Value shipped:** / **Why it is …:**` — an increment that
  cannot name the value shipped is not an increment.
- Update the mapping table (outcome ↔ units, with the Gaps column honest), the dependency table
  (structural edges only — each edge names what the dependency supplies), the parallel-build
  table and its graph, and the production point the unit sits behind.

## Verification rows

- Shape: `| <deliberate violation> → <expected refusal> | what it proves, ADR-NNNN, and the wrong
  reading it rules out | <unit link> |`. Pending rows key to the unit that delivers the control, or
  to the production point at which a drill or manual exercise runs; unit identifiers link to their
  roadmap group anchors and production points to their sections.
- Before writing the row, name the deliberate violation. If the row's left side is an
  enumeration, an inspection, or a comparison, it is not an injection — restate it as the
  violation that must be refused (introduce the drift, break the rule, plant the fixture — and
  watch the control fire). A control whose violation you cannot name is not yet a testable
  control; take it back to the record.
- A row states only what the record decides. Never encode an ordering, mechanism, or
  implementation detail the record leaves open. And construct the fixture so no other control
  could produce the expected refusal — an injection whose refusal two controls could each explain
  proves neither.
- An absence row — plant a fixture, then search an output surface and expect it absent — is a
  sanctioned shape with two extra duties: the planted fixture must be something only the control
  under test suppresses (no other control may explain the absence), and the search must cover the
  entire output surface the guarantee spans.
- A control dispositioned as parked or as riding a unit's criterion still leaves a trace in the
  catalogue: a row in its parked/answerable section naming the control, the disposition, and
  where the proof rides. The catalogue claims every control; a disposition living only in a
  record is invisible from the catalogue's side.
- A new control lands with its row in the same change. Proving a row later: its Status cell gains
  the date and an evidence pointer, and the row stays where it is.
- Parking a row: the standing reason goes in the row; re-opening appends, never rewrites.

## Mutation-ledger rows

- The ledger's own preamble is the format authority. A row names the control, how the
  mechanism was broken both ways, the tests that went red, the date, and an evidence pointer.
- A row is written at implementation time, when its control lands, and rewritten only when the
  control, its tests, or a generator its tests draw from changes.
- A surviving mutant keeps its row open as a defect until the tests are fixed or the mechanism
  is deliberately removed as redundant.
- A control with no standing automated test gets no row. The line is ADR-0046's.

## Front-door files (README.md, CLAUDE.md)

README.md holds pointers only. CLAUDE.md holds its standing reminder, its document map, the code's
layout and conventions, and the repository process, and no other fact takes up residence there.
When a change elsewhere alters what these point at (a document's role, the resolution order, a
rule's location), update the pointer.

CLAUDE.md's code layout section is mirrored by the rules under `.claude/rules/` that load when code
is touched (`go.md`, `ci.md`, `db.md`, `browser.md`). A convention changed in either place is
changed in the other in the same change. The section's headings are link anchors, and the rules also
cite them by name in plain text, so renaming a heading re-points every link and every such citation.
