---
paths:
  - "docs/UI.md"
---

# Rules for docs/UI.md

You are touching the UI's design document, one component's design held to DESIGN.md's split test
and rate of change. Hold these lines:

- **It states what the UI is, never what the system is.** Every fact about the system it relies
  on (a table, a column, a status, a grant, a policy rule) is stated by a decision record or by
  DESIGN.md and cited here by number. A system fact that appears only here is routed wrong.
- **The session that builds the UI reads this document and the records it cites, and nothing
  else.** Nothing here may depend on a mockup under `ui/design/`.
- **No build state.** What is built, when, or in what order lives in ROADMAP.md. Sequencing that
  reads as a plan belongs with the UI's units there, M3 and M5, or with their tickets.
- **No alternatives weighed.** A choice with real alternatives is a record, and this document
  cites it.
- **Section numbers are anchors.** Other documents link into sections by number, so a section is
  renamed or renumbered only with every link repointed in the same change.
- **Same prose rules as every document**, and the same coherence check afterwards.
