# The whole-set coherence check

Run after authoring, before the mechanical checks. The obligation: after any change or addition,
the ENTIRE document set remains internally consistent and logically, epistemically, and
ontologically coherent. Concretely — no two statements a cold reader could not hold as true at
once; no reference pointing at something that no longer says what the reference claims; no term
meaning two things; no claim whose evidence the change just removed.

Scale the walk to the blast radius, not the diff size: a one-line rename can have a larger radius
than a new section. For each check, actually open the files involved — do not assert from memory.

## 1. Direct contradictions

- Re-read every section your change cites, and every section that cites what you changed (the
  index and the stable documents' plain-text "ADR-NNNN" mentions are the map for records;
  identifier links are the map for outcomes and units).
- Ask of each pair: could a cold reader hold both as true? Watch especially for: a claim your
  change generalized that another document still states narrowly (or vice versa); numbers or
  thresholds stated in two places (there should be exactly one home — fix the second into a
  pointer); a superseded decision still cited as current authority.
- Overlap without contradiction is also a defect: when two outcomes or rules could both plausibly
  claim the same failure, check that the final texts let a cold reader tell which one a given
  failure falsifies — a boundary that lives only in the author's head is not in the documents.
- **When a change alters a rule or doctrine — not just content — enumerate every statement of
  that rule across the whole set, front doors included** (CLAUDE.md and README.md restate
  doctrine even though they hold no facts), exactly as §4 treats renamed terms: every statement
  gets a written disposition. A doctrine stated in two places with two readings is the
  highest-priority contradiction this check exists to catch, because the identifier-shaped map
  above never reaches it. This enumeration carries §4's ledger, counts, and reconciliation
  duties in full — the machinery is written there around terms, but it binds a doctrine sweep
  identically. The same principle runs in reverse when *introducing* a convention: a
  rule stated in the enforcement layer (`.claude/`) but absent from the in-repo authority it
  mirrors is a doctrine split — land both statements together.

## 2. The identifier system

- Every identifier used (outcome, unit, increment, record number) resolves: defined exactly once,
  linked from prose uses, present in its home table (axis table for outcomes; group table for
  units; index for records).
- Nothing orphaned: an outcome with no unit and no gap-column explanation; a unit pointing at an
  outcome that no longer exists; a verification row keyed to a renamed unit.

## 3. The tables that mirror

Several tables are derived views and drift silently — if your change touched their subject,
reconcile them in the same change:

- `docs/adr/README.md` group tables ↔ each record's actual status and title.
- USE_CASES.md axis table ↔ the outcome sections.
- ROADMAP.md mapping table (outcome ↔ units, Gaps column) and dependency tables ↔ the unit
  entries.
- DESIGN.md §3 Known limits ↔ the pillars' stated limits; §4 Failure modes ↔ where dispositions
  actually live.

## 4. Vocabulary

- New terms: defined in the Glossary, introduced at first narrative use, used consistently
  everywhere (not one meaning in the record and a drifted one in the design).
- A term arriving from an operator agreement that the Glossary cannot resolve is escalated or
  normalized to standard vernacular — never imported silently. Fidelity to the agreement's
  meaning does not require importing its coinages.
- Changed or retired terms: no stale uses left anywhere in the set.
- **When a change renames, generalizes, or narrows a term or referent — adds a member to an
  enumerated set** (an axis, an identifier range, a timing enumeration — including adding a
  document to the set) — **or closes an open decision** (the close retroactively constrains
  everything written while the question was open, so the sweep covers the code sketches,
  examples, and placeholders written meanwhile): **enumerate, then disposition.** List every
  occurrence of the OLD phrasing across the whole set — write down the phrase families first
  (for a client-surface change they were: "MCP surface", "MCP endpoint", "MCP tool", "tool
  surface", "the agent" as caller, "last hop before") and search for each;
  this enumeration is the one sanctioned use of search tools in this skill, because a sweep
  performed from memory demonstrably does not complete itself. Then disposition every occurrence
  in writing: **update** (stable documents always; records when the edit passes the in-place
  test — spirit-true and backwards compatible), **keep deliberately** (say why — e.g. a claim
  genuinely about the agent specifically), or **escalate** (the in-place test's answer is
  unclear — ask the operator). An occurrence without a disposition is an unfinished change.
  Watch especially for half-generalized paragraphs — one sentence updated, its neighbor still
  narrow — which read as deliberate distinctions.
- **The disposition ledger is an artifact with a home**: a table (occurrence — file and phrase —
  → disposition → reason) in the session notes or as a pull-request-body appendix. Keeps may be
  grouped only where one reason genuinely covers the group, and a grouped keep lists its member
  files and sections — a group whose membership a cold reader cannot enumerate is not named by
  its members. The reviewer's completeness check runs against the ledger, not against the diff.
- **The ledger closes by reconciliation, not assertion.** Record the per-family hit count from
  the enumeration; every hit maps to exactly one row or one named group member; a file's rows
  close only when all of that file's hits are dispositioned. When one file carries several hits
  of a family, name each hit — a ledger row covering a file with N hits lists N dispositions —
  and check the arithmetic row by row against the enumeration output before declaring closure.
  An in-place broadening of a record dispositions the WHOLE record — re-read it end to end,
  because broadening one section while a sibling section keeps the narrow phrasing is the
  half-generalized-record failure, and it has recurred. These duties bind every enumeration a
  change performs, volunteered or obligatory — a ledger stating "everything dispositioned"
  without the counts is the assertion this rule exists to eliminate.
- **Specify the enumeration instrument, because a wrong instrument produces confident wrong
  counts:** search case-insensitively; count occurrences, not matching lines; tolerate the
  phrase wrapping across a line break (normalize each file's whitespace before counting, e.g.
  collapse newlines to spaces); and derive group member lists from the search output — never
  from memory. Counts produced by a bare line-oriented, case-sensitive search have twice been
  materially wrong.

## 5. Register and rate-of-change

- Nothing landed in a document whose burden it does not meet: build state outside the roadmap,
  alternatives-weighing outside records, mechanisms in the outcome contract, an outcome-level
  promise buried in a record's Consequences.
- If the change effectively promises something new (a guarantee, an invariant), it went through
  the operator, and it lives at the right level (outcome or pillar), not smuggled in prose.

## 6. Sourcing, one last pass

Re-read only the sentences you added, asking of each: what is the source — which conversation
agreement, which existing document, which operator decision? A sentence with no answer gets cut
or taken to the operator, however good it sounds. This check exists because plausible invented
text is the failure mode that survives every other check.

For a newly minted record, this pass is claim-by-claim and written down (see the mint
procedure's source walk): every Decision bullet and every Consequence names its source line
before commit. "It obviously follows" is not a source.
