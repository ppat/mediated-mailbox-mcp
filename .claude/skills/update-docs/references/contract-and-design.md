# The outcome contract, the design and the test strategy

Used rarely, and only with the operator's agreement for outcomes and pillars. Touches
`USE_CASES.md`, `DESIGN.md` and `TESTING.md`, and ripples widely. An outcome hands off to
[state-and-proof.md](state-and-proof.md) for the roadmap and the verification rows. A pillar
reaches every record's `Pillar:` link, and a test-strategy change is a record change by
[records.md](records.md).

## Add a new outcome

The highest-burden change in the repository along with pillar changes: it alters the near-frozen
contract. Requires the operator's explicit agreement — never add an outcome on your own judgment.

1. Assign the identifier: next number within the axis (letter = axis). Heading form:
   `### X9 — Short name`.
2. Section shape: bolded claim; `*Falsified by any of:*` list of concrete, observable failures;
   scope notes where a criterion would otherwise be misread (including "this looks like failure
   but is success" cases).
2a. The outcome's claim, falsifiers, and any scope note get the same written claim-by-claim
   source walk a record mint gets (step 5b of the mint procedure in [records.md](records.md)),
   quoted from the agreement — this is the highest-burden document; nothing lands in it unwalked.
3. Add the outcome to the axis table at the top of USE_CASES.md, with its anchor link — and
   update every other statement of the identifier scheme in the same change: the "Why these axes"
   diagram, any member-enumerating axis preamble, DESIGN.md's Glossary "Outcome identifiers"
   entry, and ROADMAP.md's Identifiers preamble. The ranges ("C1–C3") are derived views that go
   stale silently.
4. Re-point what serves it: the roadmap unit(s) building it (`→` pointer and the mapping table),
   and any verification rows that prove it. Records implementing it add it to their `Serves:`
   header when next touched — do not mass-edit records for this alone. When a unit's header
   outcome is re-pointed, account for the displaced outcome from its own side too: state where
   its machinery now rides, in the group preamble and the mapping table's Gaps column.
5. No decision-record citations inside USE_CASES.md, ever.

## Change an existing outcome

Falsifier additions and scope-note clarifications keep the outcome's identity; anything that
changes what the outcome promises is contract change and needs the operator's explicit agreement.
Check every place the outcome is cited (roadmap unit pointers, mapping table, verification rows,
records' `Serves:` headers) for meaning drift against the new text.

## Add or update a pillar, known limit, or failure mode

- A new pillar needs the operator's explicit agreement and must pass the split test: still true
  no matter which way any single reversible decision goes. Shape: heading as a claim; body =
  claim (2–4 sentences) → `Why:` → `Known limit, stated rather than hidden:`.
- A pillar addition gets the same written claim-by-claim source walk a record mint gets (step 5b of
  the mint procedure in [records.md](records.md)): every sentence names its source line in the
  agreement or the existing documents before commit. A known limit especially must carry the
  agreement's hedge exactly — softening or hardening a limit is meaning drift at the design's
  highest-burden level.
- The pillar's known limit also gets a pointer row in §3's Known limits table (where the
  disposition lives — never the fact restated). New failure modes get a §4 row: name,
  likelihood/impact, disposition pointer.
- Renaming a pillar heading changes its anchor: update every record's `**Pillar:**` deep link.

## Testing-strategy changes

1. The decision is a record. Mint a new one or change an existing one per
   [records.md](records.md), and the in-place test decides which.
2. Reconcile TESTING.md in the same change. A new or retired test kind changes the "What to
   test, with what" table, and a changed rule changes the sentence citing it. That sentence
   must be recoverable from the record without meaning drift.
3. Walk the record's rules for controls (step 5c of the mint procedure in
   [records.md](records.md)). Each new control lands with its `docs/VERIFICATIONS.md` disposition
   in the same change.
