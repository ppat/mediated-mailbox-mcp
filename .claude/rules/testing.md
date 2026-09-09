---
paths:
  - "TESTING.md"
---

# Rules for TESTING.md

You are touching the testing-strategy narrative. It states how this project tests and cites the
decisions behind the strategy by number. The decisions live in records, the scenarios in
docs/VERIFICATIONS.md, and the mutation demonstrations in docs/MUTATIONS.md.

- **This document states and cites. It never argues.** A paragraph weighing alternatives here
  is routed wrong even if true, and belongs in a record.
- **No scenario rows and no demonstrations.** A concrete injection belongs in
  docs/VERIFICATIONS.md, and a mutation table belongs in docs/MUTATIONS.md.
- **A layer, instrument, or discipline enters only when a record decides it.** This document
  moves when the strategy's records move, never ahead of them.
- **No build state.** Which tests exist and which rows are proven are ROADMAP.md and
  docs/VERIFICATIONS.md facts.
- **The Not yet here section empties as records land.** When a change lands a record identified
  there as awaited, the same change updates this document.
