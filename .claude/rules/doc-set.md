---
paths:
  - "README.md"
  - "CLAUDE.md"
  - "DESIGN.md"
  - "USE_CASES.md"
  - "ROADMAP.md"
  - "TESTING.md"
  - "docs/**/*.md"
  - "db/README.md"
  - "provider/README.md"
  - "ratelimit/README.md"
  - "sanitize/README.md"
  - "testsupport/README.md"
---

# Rules for every document in the set

You are reading or editing part of this repository's document set. Before writing anything into
it, invoke the `update-docs` skill — it routes content to the right document and carries the
authoring procedure. These rules bind even when the skill was not invoked.

- **One home per fact.** No document restates another's content; cross-references point, never
  restate. Duplication is how two documents come to disagree. (A pointer row that says *where* a
  fact lives is fine. A second statement of the fact is not. TESTING.md's stance and proof
  system are the set's only sanctioned restatements, per the TESTING.md rule.)
- **Content must be sourced.** These documents record decisions actually made and reasoning
  actually held — by the operator, or agreed with the operator in conversation. Never author
  filler that reads like a record: an alternative nobody weighed, a rationale nobody gave, a
  mechanism nobody designed. If the source for a sentence is "it sounded plausible," the sentence
  does not go in.
- **Plain English.** Industry-standard or open-source-community vernacular is fine; invented
  shorthand is not. A concept this project coins is defined in the narrative at first use AND in
  DESIGN.md's Glossary — the Glossary is the single home for vocabulary; nothing is defined
  locally elsewhere.
- **No historical narration in the stable documents** (DESIGN.md, USE_CASES.md, TESTING.md,
  README.md): they state only the current form. Decision records may carry limited history
  where it earns its place; the roadmap's delivered-work register is the sanctioned home for
  "what happened."
- **Linking.** Every reference links to its target. A record is cited by number ("ADR-0007")
  with the number linked to the record, and every link into a record is re-pointed in the same
  change that renames or relocates it. USE_CASES.md cites no individual records at all. Every
  identifier (C1, G2, ADR-0015, S1, V3, …) links to the section defining it, except inside
  this document set's tables and dependency edges where bare identifiers are allowed.
- **Build state lives only in ROADMAP.md.** Never in DESIGN.md, never in a decision record.
- Nobody hand-edits `CHANGELOG.md` — release tooling generates it.
- Before committing document changes, run the offline link/anchor check and markdown lint (the
  `update-docs` skill's verification step has the commands).
