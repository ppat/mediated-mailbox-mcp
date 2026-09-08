# CLAUDE.md

Orientation for agents working in this repository. This file points at documents; it does not
duplicate them — one home per fact, everywhere.

## The documents

| Read | For |
| --- | --- |
| [USE_CASES.md](./USE_CASES.md) | What the system is for: the outcomes on five axes, each with a falsifiable acceptance criterion |
| [DESIGN.md](./DESIGN.md) | The pillars and invariants — and the **Glossary**, the single home for vocabulary. If a term needs defining, it gets defined there, never locally |
| [ROADMAP.md](./ROADMAP.md) | All the work in one place: delivery posture, value path, units, dependencies, open decisions. The only top-level document that tracks build state |
| [docs/adr/README.md](./docs/adr/README.md) | The decision records: index, record format, statuses, and the granularity rule (one decision per record, cut by the re-argue test) |
| [TESTING.md](./TESTING.md) | The testing strategy: stance, layers, instruments — narrative citing the records; scenario rows and demonstrations live elsewhere |
| [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) | Every control's proving injection, past and pending. A new control lands with its injection row |
| [docs/MUTATIONS.md](./docs/MUTATIONS.md) | Each automatable control's mutation demonstration — the proof its tests go red when the mechanism is removed. Rows exist only from implementation time |

Design questions resolve there, in that order: outcome → pillar/glossary → decision record.

## Keep the documents current — a standing duty

When something new comes up during ANY work — a decision made in conversation, a fact learned
while implementing, a plan change, a new term, a discovered risk — updating the relevant document
is part of that work, not a follow-up. Nothing about the system lives only in chat, code, or
commit messages. Use the **`update-docs` skill** to do it: it routes content to the right
document, carries the authoring procedures, and ends with a whole-set coherence check.

The binding conventions per document live in `.claude/rules/` and load automatically when the
matching file is read; the format authorities are the documents' own preambles and the decision-
record index.

## Repository process

- `docs` is a visible release type here — a documentation PR proposes a release when merged.
  Expected, not accidental.
