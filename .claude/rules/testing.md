---
paths:
  - "TESTING.md"
---

# Rules for TESTING.md

You are touching the testing-strategy document. It answers, for the implementer, what tests a
piece of work must have and what proves the work done, and it links each answer's decision
record. Its owned content is the assembled view no single record can hold. That is the "What
to test, with what" table, the "In standard vocabulary" paragraph, when tests run, and the
proof system.

- **This document states and cites. It never argues.** A paragraph weighing alternatives here
  is routed wrong even if true, and belongs in a record.
- **More than a sentence of any record's content here is overstepping.** Each cited decision
  gets at most one sentence here, recoverable from the record without meaning drift. The
  stance and the proof system are the exceptions. The stance carries the whole seen-to-fail
  discipline, and the proof-system chain assembles across records by design. A missing
  answer to "what test do I write for this, and what proves it done" is understepping.
- **A test kind, instrument, or discipline enters or leaves only when a record decides it.**
  The change to the record and the change to this document land together. This document moves
  when the strategy's records move, never ahead of them.
- **No scenario rows and no demonstrations.** A concrete injection belongs in
  docs/VERIFICATIONS.md, and a mutation table belongs in docs/MUTATIONS.md.
- **No build state.** Which tests exist and which rows are proven are ROADMAP.md and
  docs/VERIFICATIONS.md facts.
- **An awaited-items section exists only while something is awaited.** When a change lands a
  record identified there as awaited, the same change updates this document, and the section
  is removed once nothing remains awaited.
