# Decision records

Used whenever a decision lands, which is most tickets. Touches the records under `docs/adr/`, their
index in `docs/adr/README.md`, and every stable document that cites a changed record. A mint hands
off to [state-and-proof.md](state-and-proof.md) for each control's verification row, and to
[glossary.md](glossary.md) for any term it coins.

## Mint a new decision record

1. Find the highest number in `docs/adr/README.md` across ALL groups; the new record takes the
   next one. Never reuse, never leave gaps deliberately. When the control loop runs sessions in
   parallel, the number is instead the one the control session grants on request, since several
   next one. Never reuse, never leave gaps deliberately.
2. Pick the folder by theme (`redaction/`, `classification/`, `provider/`, `data/`, `mutation/`,
   `operability/`). Folders are navigation only — do not agonize; a record can move later without
   renumbering. Filename: `NNNN-short-slug.md`.
3. Check granularity with the re-argue test: decisions merge into one record when reversing one
   would force re-arguing the others; they stay separate records when independently reversible.
4. Write the record: H1 as the decision stated as a claim (`# NNNN. <claim>`); metadata header
   line (`**Status:** … · **Pillar:** [name](deep link), when one applies · **Serves:**
   [outcome](deep link)s`); then Context, Decision, Alternatives considered, Consequences.
   Derive `Serves:` from which outcomes' falsifiers the decision's rules protect; when the record
   generalizes or extends existing records, start from their `Serves:` lists and account for any
   outcome dropped. When no outcome's falsifiers are protected by the decision, do not force a
   fit — take the Serves question to the operator.
5. In Consequences, state what the decision assumes about other components — implicit coupling
   named here is reviewable; left unnamed it is the thing that breaks future evolution.
5a. In the record's prose, deep-link every decision-record and outcome identifier to its file or
   section.
5b. Walk the Decision and Consequences claim by claim against the source (the operator agreement
   or discussion being recorded) and name each claim's source before committing. Walk against the
   source's own words, quoted — checking your sentence against a paraphrase of it verifies
   nothing, and a verb or qualifier present in your sentence but absent from the quoted source is
   itself a finding against the walk. The walk covers the header line too — Status, Pillar, and
   Serves are claims like any other. A claim with no source is removed or taken to the
   operator — no exception for claims that "obviously follow."
5c. Walk the Decision's bullets once more asking: does this bullet state a rule that is enforced,
   checked, or called testable? Each yes is a control, and each control lands with its injection
   row in `docs/VERIFICATIONS.md` in this same change — or with a written parking or
   rides-on-a-unit disposition. Controls include rules the record restates from other records
   with widened scope (a posture "applied identically" to a new surface widens the older
   controls' scope — disposition those too, typically as riding the existing rows). A record
   introducing N controls while the catalogue gains fewer than N dispositions is an unfinished
   mint.
6. Give the record the status `docs/adr/README.md`'s statuses rule sets, which depends on whether
   the same pull request implements the decision. A `Proposed` record still awaiting the operator's
   ratification also gets a row in ROADMAP.md's Open decisions table naming what it gates.
7. Add the index row in `docs/adr/README.md`: number, link whose text is a shortened restatement
   of the decision, status (bold if not Accepted).
8. If stable documents need to cite it, they cite "ADR-NNNN" with the number linked to the
   record.

## Changing an existing record: in place, or by supersession?

Apply the in-place test from the adr rule: **a record may change in place when the change stays
true to the original decision in spirit AND is backwards compatible with the previous
interpretation** — everything true or permitted under the old reading stays true or permitted.
Broadening a referent because a new decision widened the world (e.g. "the agent" → "every
client" after a second client surface was decided), clarifying wording, or adding a consequence
the decision always implied are in-place changes. Reversing, narrowing, or re-arguing what was
decided is a supersession (procedure below). Unsure which side a change is on → ask the
operator.

When a new decision broadens what older records name, the new record still states the
generalization in its own terms and deep-links the records it touches — the reader gets the
current picture from either end. The stable documents always state the current form, so the
generalization sweep (the coherence check's enumerate-and-disposition step) applies to them and
to the affected records alike.

Relocating decision content between records (an operator-directed carve): the moved content's
old home must not survive anywhere. Sweep for references to the moved decision both by the
mechanism's name and by its effect — a pointer row citing the old record "for the narrowing"
escapes a name-only sweep — and re-point every one to the new home in the same change.

## Supersede an existing decision

1. Never edit the old record's substance. Mint the replacement as a NEW record (the mint procedure)
   whose Decision restates everything that now holds — the new record must stand alone, because
   readers are redirected away from the old one.
2. Old record: change only the header line — `**Status:** Superseded — **Superseded by:**
   [ADR-NNNN](./relative-link.md) ·`. Body untouched. Make this change, step 4's for the old row
   and step 5's re-pointing, only in the pull request where the new record becomes `Accepted`, as
   `docs/adr/README.md`'s statuses state.
3. New record's header may carry `(supersedes [ADR-MMMM](./relative-link.md))` after its status,
   and must while it is `Proposed`, as `docs/adr/README.md`'s statuses state.
4. Update both index rows.
5. Hunt every citation of the old number in the stable documents and re-point or reword — a
   stable document citing a superseded record for a claim the successor changed is a coherence
   break (the whole-set check will catch it, but fix it now).
6. Partial supersession (one record carried two separable decisions and only one changed): split
   at this touch — mint two records, one restating the unchanged decision, one carrying the
   change; the old record is superseded by both, named in its header.
