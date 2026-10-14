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
  automatable control it delivers is missing its mutation demonstration in the mutation ledger,
  docs/MUTATIONS.md and the files it lists (ADR-0046). A demonstration recorded with later work that touches the control's surface area
  does not hold open the unit that built the control's mechanism. A verification row keyed to a
  delivered unit as later work, filed under the unit whose mechanism it concerns as a discovery is,
  keys that later work and does not hold the unit open either.
- **Delivered work states what it did NOT deliver** alongside what it did, so a cold reader
  cannot over-assume, until the collapse after its production point's crossing carries it in the
  point's summary.
- The delivery posture's non-deferrables list is edited only with the operator's agreement — the
  membership test is "deferral is irreversible, or the failure it permits is silent."
- **Every unit names its finish line**, tested, image, or packaged, the lowest one that proves
  what it delivers. A unit is done at that line and no further.
- **A unit recut here recuts its open tickets** (CLAUDE.md, Repository process).
- **Production is touched only at the production points**, and a proof that needs the real
  mailbox or the deployed system keys to one of them, never to a unit. A point names what is
  supplied there and nothing about how it is supplied.
- **A production point is crossed when its issue closes, on the operator's decision alone.** The
  first pull request after it replaces the rows of the work behind the point in the table by pull
  request and its entries in the delivered register with a summary per outcome, which keeps each
  unit as one `[x]` line with its identifier and name under its outcome, after moving the later work
  they record, as the preamble states, and no work is placed in an increment whose issue is closed
  (CLAUDE.md, Repository process).
