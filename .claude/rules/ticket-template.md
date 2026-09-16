---
paths:
  - ".github/ISSUE_TEMPLATE/ticket.md"
---

# Rules for the ticket template

You are touching the template every ticket is cut from. It is the home of the ticket format, and
the rules for tickets are CLAUDE.md's, under Repository process. Invoke the `update-docs` skill
before editing it, as for every member of the document set. Hold these lines:

- **The template holds slots, not facts.** Every slot is a backticked angle-bracket placeholder
  saying what fills it, and the file carries no links and no build state.
- **The header line and the sections are the format.** Adding, removing or renaming one changes
  what every ticket cut afterwards carries, and CLAUDE.md's Repository process is reconciled in
  the same change.
- **The prose rules of CLAUDE.md bind every placeholder**, and every ticket cut from the template.
- The `update-docs` skill's coherence check and mechanical checks cover this file.
