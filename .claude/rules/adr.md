---
paths:
  - "docs/adr/**"
---

# Rules for decision records

You are touching decision records. The format authority is
[docs/adr/README.md](../../docs/adr/README.md) — read its "Record format" section before writing;
these are the tripwires that must hold even in a drive-by edit:

- **The in-place test: spirit and backwards compatibility.** Until the first release lands, the
  clause in CLAUDE.md ("Until v0.0.1 lands") overrides this test and any record may change in
  place. From that release on, an accepted record may change in
  place when the change stays true to the original decision in spirit AND is backwards
  compatible with the previous interpretation — everything that was true or permitted under the
  old reading remains true or permitted (broadening a referent, clarifying, adding a consequence
  the decision always implied). When a change fails that test — it reverses, narrows, or
  re-argues what was decided — the decision is superseded instead: a NEW record at the next
  global number, the old record's header gaining `**Superseded by:** ADR-NNNN`, its body
  otherwise untouched. When you cannot tell which side of the test a change is on, ask the
  operator; do not guess.
- **Numbers are global, minted in order, never reused.** Folders are navigation only — a record
  may move folders without renumbering. Check the index for the highest number before minting.
- **Every record update updates the index** (`docs/adr/README.md`) in the same change: the group
  table row, the link text (a shortened restatement of the decision, not the filename), and the
  status (non-Accepted statuses bolded).
- **The title is the decision stated as a claim**, never a topic. The header line under the H1 is
  the single home for the record's metadata (Status, Pillar when one applies, Serves, tickets).
- **Four fixed sections**: Context, Decision, Alternatives considered, Consequences. Alternatives
  may be an explicit "none seriously considered, because …" but never absent — and each
  alternative carries the case made for it, not only why it lost; where no case was tabled, say
  so plainly rather than inventing one.
- **Consequences state what the decision assumes about other components.** A dependency on how
  another component or slice behaves, left implicit, is the coupling that breaks future
  evolution; naming it here is what makes it reviewable.
- **A record never synthesizes other records.** A table, matrix, or taxonomy assembled
  across records is top-level-document content, not a record's. For the testing records, the
  assembled view is TESTING.md's, and an edit to any testing record reconciles TESTING.md's
  table rows and the sentences citing that record in the same change.
- **TESTING.md carries an awaited-items section only while something is awaited.** A change
  landing a record such a section identifies as awaited updates TESTING.md in the same change,
  and the section is removed once nothing remains awaited.
- **No build state in records** — which unit builds it and when is roadmap business.
