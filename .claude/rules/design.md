---
paths:
  - "DESIGN.md"
---

# Rules for DESIGN.md

You are touching the design document — the slowest-changing document after USE_CASES.md. Its own
preamble states the governing rule; hold these lines:

- **The split test governs every addition:** DESIGN.md holds only what would still be true if any
  individual reversible decision had gone the other way. If flipping one decision would delete or
  rewrite the paragraph you are writing, it belongs in a decision record, not here.
- **A pillar changes only when the design genuinely changed** — overturning or adding a pillar is
  among the highest-burden edits in the repository and needs the operator's explicit agreement,
  never a drive-by improvement.
- Pillar headings are claims (often "X, not Y"); a pillar body is claim → Why → "Known limit,
  stated rather than hidden." A new pillar's known limit also gets a row in §3's table; a new
  failure mode gets a row in §4. Both tables are pointer-level: they say where a disposition
  lives, never restate it.
- **The Glossary is the single home for vocabulary.** New coined terms get defined here; retired
  terms are removed everywhere in the same change; two meanings for one word get explicit
  cross-referenced disambiguation.
- **No build state** (the roadmap's job), **no alternatives-weighing** (the records' job), **no
  test plans** (the verification catalogue's job), **no historical narration**.
