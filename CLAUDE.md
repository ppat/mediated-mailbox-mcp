# CLAUDE.md

Orientation for agents working in this repository. This file points at documents; it does not
duplicate them — one home per fact, everywhere.

## The documents

| Read | For |
| --- | --- |
| [USE_CASES.md](./USE_CASES.md) | What the system is for: the outcomes on five axes, each with a falsifiable acceptance criterion |
| [DESIGN.md](./DESIGN.md) | The pillars and invariants — and the **Glossary**, the single home for vocabulary. If a term needs defining, it gets defined there, never locally |
| [docs/UI.md](./docs/UI.md) | The UI's design: the lens model, the zoom ladder, the screens, the palettes, the framework requirements, and the build guidance. Same split test as DESIGN.md, one component |
| [ROADMAP.md](./ROADMAP.md) | All the work in one place: delivery posture, value path, units, dependencies, open decisions. The only top-level document that tracks build state |
| [docs/adr/README.md](./docs/adr/README.md) | The decision records: index, record format, statuses, and the granularity rule (one decision per record, cut by the re-argue test) |
| [TESTING.md](./TESTING.md) | What tests a piece of work must have and what proves it done, linking the records that decide it |
| [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) | Every control's proving injection, past and pending. A new control lands with its injection row |
| [docs/MUTATIONS.md](./docs/MUTATIONS.md) | The mutation ledger. Per-control proof that tests go red when the mechanism is removed, written only from implementation time |

Design questions resolve there, in that order: outcome → pillar/glossary → decision record.

## Until v0.0.1 lands

Nothing is implemented yet, so nothing in the document set is final. v0.0.1 is the first
release, cut when the release pull request
[#2](https://github.com/ppat/mediated-mailbox-mcp/pull/2) lands. Until then the design, the
outcomes, the roadmap, and every decision record may be edited in place, including edits that
reverse a decision, with no supersession and no new number, as long as the entire set stays
internally consistent after the edit, checked as the `update-docs` skill's coherence check
describes. The in-place-or-supersede rule stated in the decision-record index takes effect when
v0.0.1 lands, and this section is removed then.

## Keep the documents current — a standing duty

When something new comes up during ANY work — a decision made in conversation, a fact learned
while implementing, a plan change, a new term, a discovered risk — updating the relevant document
is part of that work, not a follow-up. Nothing about the system lives only in chat, code, or
commit messages. Use the **`update-docs` skill** to do it: it routes content to the right
document, carries the authoring procedures, and ends with a whole-set coherence check.

The binding conventions per document live in `.claude/rules/` and load automatically when the
matching file is read; the format authorities are the documents' own preambles and the decision-
record index.

## The target form for prose

These rules bind all documentation, code comments, any other artifact containing prose, and
commentary output to the user.

- Crystal clear and understandable to a cold reader, human or LLM.
- Plain English, or technical English from industry-standard or open-source vernacular. No
  invented terminology. The project's own established terminology, explicitly defined in
  DESIGN.md's Glossary, is exempt.
- Referencing common or widely understood technical or OSS concepts and constructs is fine when
  it aids understanding.
- No stating the obvious, and no restating non-novel concepts, such as explaining how a
  well-known application, platform, or tool works.
- No shorthand. Claude in particular tends to compress an idea into an invented term to save
  tokens. That compression is banned.
- Brevity is valued, but never at the expense of fidelity:
  - Fidelity loss is unacceptable.
  - Compressing into shorthand is not the way to brevity, as stated above.
  - Reach brevity by reducing filler (dropping anything not needed to convey the idea), by using
    widely understood concepts and idioms from general English or from industry-standard or
    OSS-community technical English where they genuinely add value, by avoiding rambling, walls
    of text, and stream-of-consciousness output (anything that interrupts the document's flow or
    sits outside its narrative), and by using structure to your advantage.
  - Beyond that, do not overshoot toward brevity. Overshooting ends in compression that loses
    fidelity.
- Never try to sound smart or convey the writer's ingenuity to the reader. That is a HARD NO, an
  anti-pattern to avoid always.
- Do not write in the standard corporate drone register Claude defaults to. Just as LLM
  attention wanes over a long context window, human attention wanes too, and for a human it
  wanes even in a short context the moment the drone register appears. Drone prose goes in one
  ear and out the other without any information registering.
- No adjectives unless the adjective has merit within its sentence (i.e. losing it would inhibit
  fidelity). The same holds for adverbs, a little less strictly, though still stricter than
  Claude's default.
- No em dashes, colons, or semicolons within a sentence or a phrase. The ban there is total. The
  marks are allowed where they separate a bullet header from bullet content, and within headings
  as long as the headings stay short.

## Repository process

- `docs` is a visible release type here — a documentation PR proposes a release when merged.
  Expected, not accidental.
