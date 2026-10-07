---
name: update-docs
description: Route, author, and verify changes to this repository's document set — DESIGN.md, USE_CASES.md, ROADMAP.md, TESTING.md, docs/UI.md, the decision records under docs/adr/, docs/VERIFICATIONS.md, docs/MUTATIONS.md, README.md, CLAUDE.md, the README files of the data-access library and the narrow shared libraries, and the ticket template. Use this skill whenever a task adds, changes, or records ANY decision, outcome, plan item, term, test plan, risk, or fact about the system — including decisions made mid-conversation that merely need recording, discoveries made during implementation, superseding an existing decision, and build-state updates. Use it even when the user does not mention documentation: if new information about the system came into existence during the work, this skill is how it gets a home. Also use it when asked where something should be documented, or whether a document change is allowed.
---

# Updating the document set

This repository's documents are one system, organized by rate of change: the outcome contract is
near-frozen, the design moves slowly, decision records are fluid, the roadmap changes with every
unit of work, and the verification catalogue grows one row per control. A change made in the
wrong document — or in the right document but breaking its conventions — is how documents come to
disagree, and two disagreeing documents are worse than none. This skill exists so every change
lands in the right place, in the right shape, and leaves the whole set coherent.

Work through four steps, in order: **route → author → check the whole set → verify and commit.**
The procedure files carry detail this file only points at. Read them when the step tells you to.

## Step 1 — Route: which document does this belong in?

Classify what you are about to write, before opening any file:

| The content is… | It belongs in | Because |
| --- | --- | --- |
| An outcome the system exists to produce, with a statement of what would falsify it | `USE_CASES.md` | It is contract, not mechanism |
| Something that would still be true if any individual reversible decision had gone the other way | `DESIGN.md` (a pillar, a known limit, a failure mode) | The design document's own split rule |
| One choice that had real alternatives and could be re-argued | A decision record under `docs/adr/` | It is reversible; the alternatives must survive |
| The UI's own design: its screens, the lens model and zoom ladder, its read API shapes, its tokens, its configuration, its build guidance | `docs/UI.md` | One component's design at DESIGN.md's altitude and split test; the system's facts it relies on stay in the records it cites |
| How the code is laid out and the conventions every component follows: the module, the components, organization inside a component, tests, static analysis and formatting, images, CI workflows, tool versions, editor settings | `CLAUDE.md`, its code layout section | Every agent working on code needs them, and the rules loaded when code is touched point at them. A choice with real alternatives that a record decides stays in the record, and CLAUDE.md cites it |
| One library's own layout, or the case a narrow shared library argues for existing | That library's `README.md` (`accountload/`, `credential/`, `db/`, `dbconnect/`, `policyload/`, `provider/`, `ratelimit/`, `sanitize/`, `settings/`, `testsupport/`) | It belongs to that component alone, and ADR-0050 requires each narrow shared library to argue its own case |
| A definition of a term used in more than one place | `DESIGN.md`'s Glossary | The single home for vocabulary |
| Anything about what is built, when, in what order, or what work remains | `ROADMAP.md` | Build state is a roadmap fact, nowhere else, apart from a decision record's status as `docs/adr/README.md` states |
| The header line and sections a ticket carries | `.github/ISSUE_TEMPLATE/ticket.md` | The template is the format's home. `CLAUDE.md`'s Repository process holds the rules and points at it |
| How work moves through the repository: a change's release type, how a ticket is cut and when it is done | `CLAUDE.md`, its Repository process section | Every agent working here needs it, and it is process, not design or build state |
| A change to which test kinds exist, when each applies, or how proof works | A decision record first, then `TESTING.md` in the same change | TESTING.md assembles the strategy and links the records that decide it |
| A deliberate violation that would prove a control works | `docs/VERIFICATIONS.md` | It is a test plan, keyed to a work unit |
| A control's mutation demonstration, the record that its tests went red with the mechanism broken | `docs/MUTATIONS.md` | It is the implementation-time ledger, one row per control |
| Orientation for a newcomer (what the project is, where facts live) | `README.md` / `CLAUDE.md` | Front doors point, and hold no fact about the system. CLAUDE.md also holds the code's conventions |

The ambiguous cases, resolved the way this document set resolves them:

- **A new record's status** is set to **Accepted**, in the pull request that implements it. So if a new
  record is being created along with its implementation in the same PR, it gets ratified by approval
  and landing of the PR. Otherwise a new record starts off **Proposed**, and becomes ratified with
  the first PR whose implementation is at least partly influenced by that record. That is the general
  practice though its not unacceptable to records to be ratified by prior to implementation, just
  unnecessary.
- **A pillar's limitation** → stated with the pillar itself, plus a pointer row in the design's
  Known limits table saying where the disposition lives. Pointer duplication is sanctioned; fact
  duplication is not.
- **A decision that touches build sequencing** → the sequencing part goes to the roadmap; only
  what is genuinely a consequence of the decision stays in the record.
- **A change spanning several documents** (most real changes) → route each piece separately,
  and never write the same fact twice. Every reference links to its target, and a record cite
  links the number to the record.
- **A test scenario versus a mutation demonstration** → a deliberate violation and its expected
  refusal is a `docs/VERIFICATIONS.md` row, minted at design time. The record that tests went
  red with a mechanism broken is a `docs/MUTATIONS.md` row, possible only at implementation
  time. One proves the control, and the other audits the control's tests.
- **A maybe-someday idea, neither targeted nor deliberately excluded** → nowhere. Non-outcomes
  are for deliberate exclusions only; recording a possibility would misstate it either way.

## Step 2 — Author, through the front door

Every change type has a procedure. Read the one you need before authoring.

| The change | Procedure | Often also needs |
| --- | --- | --- |
| Minting a record, changing a record in place, superseding a record | [references/records.md](references/records.md) | state-and-proof, glossary |
| Recording a selection between real alternatives | [references/selection.md](references/selection.md), after records | glossary |
| Roadmap state, cutting tickets, verification rows, mutation-ledger rows, README.md and CLAUDE.md | [references/state-and-proof.md](references/state-and-proof.md) | |
| Adding or changing an outcome or a pillar, a known limit or failure mode, a test-strategy change | [references/contract-and-design.md](references/contract-and-design.md) | records, state-and-proof, glossary |
| Coining, changing or retiring a term | [references/glossary.md](references/glossary.md) | |

Do not improvise the mechanics; the procedures encode details that are easy to get wrong
(numbering, index updates, identifier linking, status vocabulary). Format authorities remain the
documents themselves (each preamble, and `docs/adr/README.md` for records) — the procedures
sequence the work; they do not replace reading the authority for the file you touch.

Standards that apply to all authoring, regardless of change type — the per-file rules in
`.claude/rules/` bind automatically, and these deserve stating here because they are where
past work went wrong:

- **The document set stands on itself.** No document may need something outside this repository to
  be understood. Session notes, research reports, measurements, and conversations are where content
  comes from, not where a reader is sent. When work outside the repository produced the content,
  the document carries what a reader needs to follow and to disagree with it, and cites the outside
  artifact nowhere. A reader a year from now has the repository and nothing else.

- **Write only what is sourced.** Every sentence records a decision actually made or reasoning
  actually held — by the operator, or agreed with the operator. If you find yourself writing a
  plausible-sounding rationale, alternative, or mechanism that nobody actually stated, stop: that
  is invention wearing a record's clothing, and it is the single failure mode that has caused the
  most rework in this repository. When something genuinely needs deciding, take it to the
  operator; do not author it into existence.
- **Compression must not change meaning.** When restating agreed content, watch the ways meaning
  leaks: a dropped qualifier or quantifier ("every", "only", "if Y"), a strengthened or weakened
  verb (documented ≠ rehearsed; deferred ≠ maybe-never), a number attached to the wrong referent,
  a coined phrase whose expansion is not uniquely recoverable, a sentence moved to a new context
  where it reads differently. An example in the source ("e.g.", "say", "such as") stays an
  example — restating it as the decided value is a meaning change, and stable documents
  especially never state a record's example as fact. When in doubt, use the source's own words.
- **Match the register of the target document.** The design declares; records argue; the
  outcome contract falsifies; the roadmap tracks. A paragraph that argues alternatives inside
  DESIGN.md, or declares timeless truth inside a record's Context, is routed wrong even if the
  file is right.
- **A conflict between an instruction and the change you want to make goes to the operator
  before authoring — never resolved unilaterally.** Deviating and flagging the deviation in a
  pull-request body is still deviating; the flag does not make it sanctioned.

## Step 3 — Check the whole set

A change is not done when its file is correct; it is done when the entire document set is still
internally consistent and coherent afterward. Every change ripples: a new outcome changes the
axis table, the roadmap's unit pointers and mapping table, and which verification rows key where;
a superseded record invalidates link text in the index and possibly citations in three
documents; a renamed heading breaks anchors repository-wide.

Launch the **`coherence-check` skill** with the Skill tool and use it in full, walking its
checklist by the approach it lays out, never bits and pieces of it.
Its core question, asked of the whole set: *could both of any two statements now be read as true by
the same cold reader, and does every cross-reference still point at what it claims to?* Do not skip
this step because the change was small — one-line changes have broken tables of contents,
identifier schemes, and mapping tables before.

## Step 4 — Verify and commit

Run the mechanical checks the CI gate will run, before committing:

```bash
lychee --offline --no-progress README.md CLAUDE.md DESIGN.md USE_CASES.md ROADMAP.md TESTING.md 'docs/**/*.md' '*/README.md' '.github/ISSUE_TEMPLATE/*.md'
mise exec node -- npx --yes markdownlint-cli2 'README.md' 'CLAUDE.md' 'DESIGN.md' 'USE_CASES.md' 'ROADMAP.md' 'TESTING.md' 'docs/**/*.md' '*/README.md' '.github/ISSUE_TEMPLATE/*.md'
```

Both must pass clean. Also reflow any wrapped paragraph you edited to the file's wrap width — a
short line mid-paragraph is diff residue, and the standing record should not read like a patch.
Then commit with the documentation lifecycle verbs, which make the history
scannable: **mint** (new record), **supersede** (record replaced by a new number), **catalogue**
(verification rows), **reconcile** (roadmap brought back in line with reality, with the Position
line re-dated), **retire** (something removed deliberately), **record** (anything else worth a
verb). Example: `docs: mint ADR-0030 — the serving layer is an API; MCP adapts it`. The commit
message also carries a short body — what changed and why, for the log's cold reader — never a
bare title. After any review round changes the content, re-derive the commit message and the
pull-request body from the tree as it now stands — a patched body accretes stale quotes and
session-scoped narration, and a squash-merge lands the stale commit message on the default
branch.

One ordering rule with teeth: **documents change before the code that implements them** — a wrong
line corrected in the spec after code shipped means the code faithfully implemented the wrong
line. When implementation work discovers the documents are wrong, fix the document first, then
the code.

## Reference files

- The five procedure files under `references/`, grouped by when each is used, as the table in step
  2 lists. Read the one for your change type, and the ones it hands off to, before authoring.
