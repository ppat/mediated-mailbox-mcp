---
paths:
  - "ROADMAP.md"
---

# Rules for ROADMAP.md

You are touching the one top-level document that tracks build state — the fastest-changing of the
set. Its preamble carries the reading rules; hold these lines:

- **This is the only home for build state**: what is delivered, remaining, sequenced, or
  blocked. Status vocabulary distinguishes authored → merged → released → deployed-and-observed —
  never collapsed. Claims are **[measured]** (read from a repo, an API, or a record) or
  **[inferred]**.
- **Re-date the `Position:` line whenever the checklists are reconciled against reality** — the
  date is the staleness beacon; an un-re-dated reconciliation defeats it.
- **Each work unit serves exactly one outcome** (`→` in its header line); where machinery for a
  second outcome rides along, flag it in the group preamble rather than splitting the unit
  silently. Value increments each state `**Value shipped:**` — an increment that cannot name what
  the user gets is not an increment.
- Checkboxes are the only state marker on units; nuance lives in prose. A unit is not done while
  the verification rows keyed to it are pending for the part they key to it, nor while any
  automatable control it delivers is missing its mutation demonstration in docs/MUTATIONS.md
  (ADR-0046).
- **Delivered work states what it did NOT deliver** alongside what it did, so a cold reader
  cannot over-assume.
- The delivery posture's non-deferrables list is edited only with the operator's agreement — the
  membership test is "deferral is irreversible, or the failure it permits is silent."
- **Every unit names its finish line**, tested, image, or packaged, the lowest one that proves
  what it delivers. A unit is done at that line and no further.
- **Production is touched only at the production points**, and a proof that needs the real
  mailbox or the deployed system keys to one of them, never to a unit. A point names what is
  supplied there and nothing about how it is supplied.
