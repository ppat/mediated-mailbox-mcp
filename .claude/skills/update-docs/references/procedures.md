# Procedures, by change type

Step-by-step mechanics for each kind of document change. Format authorities remain the documents
themselves (each preamble, and `docs/adr/README.md` for records) — these procedures sequence the
work; they do not replace reading the authority for the file you touch.

## Contents

- [Mint a new decision record](#mint-a-new-decision-record)
- [Record a selection](#record-a-selection)
- [Supersede an existing decision](#supersede-an-existing-decision)
- [Add a new outcome](#add-a-new-outcome)
- [Change an existing outcome](#change-an-existing-outcome)
- [Add or update a pillar, known limit, or failure mode](#add-or-update-a-pillar-known-limit-or-failure-mode)
- [Glossary changes](#glossary-changes)
- [Roadmap changes](#roadmap-changes)
- [Verification rows](#verification-rows)
- [Mutation-ledger rows](#mutation-ledger-rows)
- [Testing-strategy changes](#testing-strategy-changes)
- [Front-door files (README.md, CLAUDE.md)](#front-door-files-readmemd-claudemd)

## Mint a new decision record

1. Find the highest number in `docs/adr/README.md` across ALL groups; the new record takes the
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
   checked, or called testable? Each yes is a control, and each control is decided with its
   injection row in `docs/VERIFICATIONS.md` in this same change — or with a written parking or
   rides-on-a-unit disposition. Controls include rules the record restates from other records with
   widened scope (a posture "applied identically" to a new surface widens the older controls' scope
   — disposition those too, typically as riding the existing rows). A record introducing N controls
   while the catalogue gains fewer than N dispositions is an unfinished mint.
6. Status: `Accepted` if the operator has agreed to this decision (in conversation counts);
   `Proposed` if adopted by the documents but awaiting ratification — and then also add a row to
   ROADMAP.md's Open decisions table naming what it gates.
7. Add the index row in `docs/adr/README.md`: number, link whose text is a shortened restatement
   of the decision, status (bold if not Accepted).
8. If stable documents need to cite it, they cite "ADR-NNNN" with the number linked to the
   record.

## Record a selection

A selection is a choice between real alternatives for a language, framework, library, runtime,
database, broker, toolchain, test stack, or platform. Mint it by the procedure above. This section
says what its four sections hold and replaces none of that procedure's steps.

It holds more than most records because the reader has to be able to re-argue the choice from it
alone. The research that produced it is not in the repository and the reader cannot open it.

### Context

The Context establishes what was being chosen between and what the choice was judged on. Four
things do that.

- **What the rest of the design has already settled, and what that leaves.** A tool's ecosystem
  offers a great deal that other records have already ruled out, so the real choice is usually
  narrower than the category suggests. State what is already settled, then state what was actually
  left to choose on. Without it a reader judges the decision against a question nobody was asking.
- **Whether the choice looks like a different problem than it is.** Where a requirement reads as
  one kind of problem and turns out to be another, say so and say why, because a reader who does
  not know will measure the decision against the wrong problem and find it wanting.
- **The requirements, as a table.** Each row names the requirement, says what it demands, and names
  the record it comes from. Where this record introduces a requirement rather than taking one from
  elsewhere, the row says so, because a reader re-arguing the decision needs to know which
  constraints are inherited and which are this record's to defend.
- **How the requirements were weighted.** Which one ordered the field and why that one. What broke
  ties. What was deliberately left out of the grading and why.

Nothing else belongs here. Not dates, not who decided what, not how the research was organised, not
what a research document concluded. A reader judging the decision needs the criteria and cannot use
any of that. Record numbers carry no ordering either, so do not write as though one record came
before another.

### Decision

A cold reader finishes this section knowing why this option was chosen. Not able to work it out.
Knowing it, because every choice is stated with its reason attached.

It holds five things.

1. **What was chosen, each part with its reason.** Versions and minimums belong here when a
   requirement demands them, with the reason for the minimum rather than the bare number.
2. **What the tool's ordinary path does that other records forbid, and what stops it.** Every tool
   has defaults and an idiomatic way of being used, and some of it will conflict with decisions the
   documents already hold. Some of that is configuration to switch off. Some is a construction to
   ban outright. State each one, what goes wrong if it happens, and what catches it. A table with
   the construction, the harm, and the check works well, and naming the harm rather than the rule
   is what lets a later reader judge whether the ban still earns its keep. Step 5c above applies to
   every one of these.
3. **Anything deliberately left open**, written as named options with what differs between them
   and what each costs. An open question stated as options is a decision deferred. An open question
   stated as prose is a gap.
4. **A table mapping every requirement to the mechanism that meets it.** One row per requirement
   from the Context's table, naming the concrete mechanism rather than restating the requirement.
   Where two mechanisms catch different things, say which catches what, because a reader who
   assumes one covers the other will not add the second.
5. **What the implementer would otherwise pay to discover.** The agent that builds this will not
   revisit the choice and has no access to the work behind it. A fact belongs here when getting it
   wrong produces a silently wrong result or an expensive rediscovery. A fact does not belong here
   when the build fails immediately and teaches the same thing, and it does not belong here merely
   because it was surprising to find.

### Alternatives considered

The grid first, then what it shows, then the table of what it cannot show, then a reading of both,
then one passage per candidate.

- **The grid.** Every candidate scored against every requirement the decision loaded. Spell the
  scale out in the record, level by level. Name every candidate, rejected ones included, since a
  reader cannot check a claim about a tool the record does not name. Where a column is graded on
  one configuration and measured on another, or stands for a family rather than one tool, the
  record says so beside the table rather than leaving it to be discovered.
- **What the grid shows.** Which requirements separated nobody, and why they did not, since a
  requirement that sorts the field evenly is a finding about the decision rather than a wasted row.
  Which requirement looks like it separated nobody and did not, if one does. Which rows were
  measured and which were read from documentation, and for which candidates, because a reader
  weighs those differently. And where the chosen candidate stands alone.
- **The second table.** Candidates the requirements sort into the same tier are separated by facts
  the requirements do not capture, and this table carries those facts. Its columns are whichever
  facts did the separating for this decision. There is no standard set of them.
- **The reading.** Four questions, answered in prose, each about the whole field rather than one
  candidate. Whose worst cell is the best, and why that matters more than a higher ceiling
  somewhere else. Which of the chosen candidate's strengths are already guarded elsewhere in the
  design and which guard something nothing else does. What regretting each candidate would cost,
  narrowed to what actually survives the exit rather than to "the code". And which of a candidate's
  strengths could be had without choosing it, and which of its costs could be confined to one
  component.
- **The passages.** One per candidate, the chosen one included. The strongest honest case for it,
  the strongest honest case against it, and nothing the tables already carry. The chosen
  candidate's case against is the one worth writing carefully, because it is the one a reader will
  suspect of having been softened.

Four rules for the grid.

- **A grade is derivable from the evidence behind it.** Where this record grades a candidate
  differently from that evidence, it gives the reason as a fact about the candidate. It does not
  say that a grade was changed.
- **Do not build a grid the evidence cannot fill.** Say what the field reduced to and carry the
  comparison in the passages instead. A grid with invented cells is worse than no grid, because it
  invites checking and does not survive it.
- **Reject on a mechanism, not on a citation.** A rejection that rests on a rule dies when the rule
  is edited. Where a candidate's own documentation disqualifies it, quoting that is the strongest
  sentence available.
- **Where the grid does not separate the leaders, say so.** Then state the preference that decided
  it, the grounds for that preference, and what would land on a different answer. A preference
  written as a deduction invites a reader to refute it. A preference written as a preference lets a
  reader disagree with it, which is the honest transaction.

### Consequences

What the procedure above requires of every record, and four things a selection tends to owe.

- **What leaving this choice would cost**, narrowed to what actually survives the exit.
- **What would re-argue the decision.** Usually an assumption the choice rests on, named with what
  happens when it moves. Prose, not a table of conditions.
- **What this decision adds to the cost of something else.** The procedure above already asks what
  a decision assumes about other components, and this is the same kind of claim pointing the other
  way. Write it as what is true now rather than as a change from what another record used to say,
  and correct that record in the same change so the two agree. A record that describes the edit it
  caused is narrating its own history, which goes stale the moment the other record is read.
- **What resolving an open question would move**, where the Decision left one open, so whoever
  resolves it knows what else changes.

### How much of this a given selection needs

The depth follows the decision. How many components depend on the choice, what reversing it would
cost, how many candidates were genuinely in play, and how much was measured rather than read. A
cheap and reversible choice over a small field gets the same sections at a fraction of the length,
and may carry no grid at all where the field reduces to one disqualifying fact.

### Vocabulary, and one home per fact

A selection record coins terms. Any term it uses that also appears elsewhere gets an entry in
DESIGN.md's Glossary, identifying what the term refers to and pointing at the record that governs
it. The entry never restates the rule.

A selection record is prone to restating arguments that belong to other records, because the
argument feels like part of the case being made here. Where an argument belongs to another record,
point at that record rather than repeating it.

### The record is the whole of the work

Research produces a report and then the work continues. Grades move, claims get corrected, and
mechanisms turn out not to exist. What lands in the record is where the decision stood at the end,
not where the report left it. A record that disagrees with the research behind it is not wrong for
that reason alone.

## Supersede an existing decision

1. Never edit the old record's substance. Mint the replacement as a NEW record (procedure above)
   whose Decision restates everything that now holds — the new record must stand alone, because
   readers are redirected away from the old one.
2. Old record: change only the header line — `**Status:** Superseded — **Superseded by:**
   [ADR-NNNN](./relative-link.md) ·`. Body untouched.
3. New record's header may carry `(supersedes [ADR-MMMM](./relative-link.md))` after its status.
4. Update both index rows.
5. Hunt every citation of the old number in the stable documents and re-point or reword — a
   stable document citing a superseded record for a claim the successor changed is a coherence
   break (the whole-set check will catch it, but fix it now).
6. Partial supersession (one record carried two separable decisions and only one changed): split
   at this touch — mint two records, one restating the unchanged decision, one carrying the
   change; the old record is superseded by both, named in its header.

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

## Add a new outcome

The highest-burden change in the repository along with pillar changes: it alters the near-frozen
contract. Requires the operator's explicit agreement — never add an outcome on your own judgment.

1. Assign the identifier: next number within the axis (letter = axis). Heading form:
   `### X9 — Short name`.
2. Section shape: bolded claim; `*Falsified by any of:*` list of concrete, observable failures;
   scope notes where a criterion would otherwise be misread (including "this looks like failure
   but is success" cases).
2a. The outcome's claim, falsifiers, and any scope note get the same written claim-by-claim
   source walk a record mint gets (step 5b there), quoted from the agreement — this is the
   highest-burden document; nothing lands in it unwalked.
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
- A pillar addition gets the same written claim-by-claim source walk a record mint gets: every
  sentence names its source line in the agreement or the existing documents before commit. A
  known limit especially must carry the agreement's hedge exactly — softening or hardening a
  limit is meaning drift at the design's highest-burden level.
- The pillar's known limit also gets a pointer row in §3's Known limits table (where the
  disposition lives — never the fact restated). New failure modes get a §4 row: name,
  likelihood/impact, disposition pointer.
- Renaming a pillar heading changes its anchor: update every record's `**Pillar:**` deep link.

## Glossary changes

- New coined term: define it in the Glossary AND at first use in whatever narrative introduced
  it. One definition; the narrative introduction is context, not a second definition.
- A named parameter or threshold used in more than one document is a term even when its name is
  plain and descriptive: it gets an identifying entry — the referent, and where its rule and
  value live — never the rule restated.
- **An entry identifies the referent; the rules governing it stay in their record.** An entry
  that would need rewriting if one decision record were superseded is holding that record's
  content — replace the rule text with a pointer ("shape and rules: ADR-NNNN").
- Retiring a term: remove its uses everywhere in the same change; if old external references may
  exist (tickets), keep a "retired synonym" note on the successor term's entry.
- Two meanings colliding on one word: both entries carry explicit cross-referenced
  disambiguation.

## Roadmap changes

- Cutting tickets: a ticket is cut from the template to the rules of
  [CLAUDE.md](../../../../CLAUDE.md#repository-process), and the unit's header names its tickets
  in the `ticket(s)` slot as linked issue numbers, in the same change. A unit recut here recuts
  its open tickets, as CLAUDE.md states.
- Delivered work: move the unit to the delivered register with `[x]`, its outcomes, its tickets or
  that it was delivered before any were cut, and state what it did NOT deliver. Re-date
  `**Position:**` whenever checklists are reconciled against reality.
- New unit: group letter + next index, and a retired identifier is never reused; header
  `**ID — name** → <one outcome> · ticket(s) · <value increment> · finishes at <tested | image |
  packaged>`; prose body naming the records whose mechanisms the unit carries, what proves it, and
  which of its proofs wait for a production point; `*Criteria:*` for observability riders. One
  outcome per unit — a genuine exception is flagged in the group preamble, out loud.
- New value increment: `**Units:** / **Value shipped:** / **Why it is …:**` — an increment that
  cannot name the value shipped is not an increment.
- Update the mapping table (outcome ↔ units, with the Gaps column honest), the dependency table
  (structural edges only — each edge names what the dependency supplies), the parallel-build
  table and its graph, and the production point the unit sits behind.

## Verification rows

- Shape: `| <deliberate violation> → <expected refusal> | what it proves, ADR-NNNN, and the wrong
  reading it rules out | <unit link> |`. Pending rows key to the unit that delivers the control, or
  to the production point at which a drill or manual exercise runs; unit identifiers link to their
  roadmap group anchors and production points to their sections.
- Before writing the row, name the deliberate violation. If the row's left side is an
  enumeration, an inspection, or a comparison, it is not an injection — restate it as the
  violation that must be refused (introduce the drift, break the rule, plant the fixture — and
  watch the control fire). A control whose violation you cannot name is not yet a testable
  control; take it back to the record.
- A row states only what the record decides. Never encode an ordering, mechanism, or
  implementation detail the record leaves open. And construct the fixture so no other control
  could produce the expected refusal — an injection whose refusal two controls could each explain
  proves neither.
- An absence row — plant a fixture, then search an output surface and expect it absent — is a
  sanctioned shape with two extra duties: the planted fixture must be something only the control
  under test suppresses (no other control may explain the absence), and the search must cover the
  entire output surface the guarantee spans.
- A control dispositioned as parked or as riding a unit's criterion still leaves a trace in the
  catalogue: a row in its parked/answerable section naming the control, the disposition, and
  where the proof rides. The catalogue claims every control; a disposition living only in a
  record is invisible from the catalogue's side.
- A new control is decided with its row in the same change. Proving a row later: status gains the
  date and an evidence pointer, and the row moves to (or is re-labelled under) the proven section.
- Parking a row: the standing reason goes in the row; re-opening appends, never rewrites.

## Mutation-ledger rows

- The ledger's own preamble is the format authority. A row names the control, how the
  mechanism was removed or disabled, the tests that went red, the date, and an evidence
  pointer.
- A row is written when its control lands, as ADR-0046 defines landing, and rewritten only when the
  control or its tests change.
- A surviving mutant keeps its row open as a defect until the tests are fixed or the mechanism
  is deliberately removed as redundant.
- A control with no standing automated test gets no row. The line is ADR-0046's.

## Testing-strategy changes

1. The decision is a record. Mint a new one or change an existing one per the procedures
   above, and the in-place test decides which.
2. Reconcile TESTING.md in the same change. A new or retired test kind changes the "What to
   test, with what" table, and a changed rule changes the sentence citing it. That sentence
   must be recoverable from the record without meaning drift.
3. Walk the record's rules for controls (step 5c of the mint procedure). Each new control is decided
   with its `docs/VERIFICATIONS.md` disposition in the same change.

## Front-door files (README.md, CLAUDE.md)

README.md holds pointers only. CLAUDE.md holds its standing reminder, its document map, the code's
layout and conventions, and the repository process, and no other fact takes up residence there.
When a change elsewhere alters what these point at (a document's role, the resolution order, a
rule's location), update the pointer.

CLAUDE.md's code layout section is mirrored by the rules under `.claude/rules/` that load when code
is touched (`go.md`, `ci.md`, `db.md`, `browser.md`). A convention changed in either place is
changed in the other in the same change. The section's headings are link anchors, and the rules also
cite them by name in plain text, so renaming a heading re-points every link and every such citation.
