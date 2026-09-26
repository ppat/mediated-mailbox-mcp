---
paths:
  - "credential/README.md"
  - "db/README.md"
  - "policyload/README.md"
  - "provider/README.md"
  - "ratelimit/README.md"
  - "sanitize/README.md"
  - "settings/README.md"
  - "testsupport/README.md"
---

# Rules for a library's README

You are touching the README of the data-access library or of a narrow shared library. Hold these
lines:

- **It describes its own library only.** A convention every component follows is CLAUDE.md's, and a
  fact about the system is stated by a decision record or DESIGN.md and cited by number.
- **A narrow shared library's README argues its own case**, as ADR-0050 requires. A choice with real
  alternatives that a record decides stays in the record.
- **It reflects the code, and the rules under `.claude/rules/` that load on its library mirror it.**
  A convention changed here is changed in those rules in the same change, and the reverse.
- **No build state.** What exists, when, or in what order lives in ROADMAP.md.
- **Same prose rules as every document**, and the same coherence check afterwards.
