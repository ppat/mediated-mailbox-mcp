# Selection records

Used rarely, when a choice between real alternatives for a tool, library or platform is recorded.
It adds to the mint procedure in [records.md](records.md), which a selection record follows first,
and it hands off to [glossary.md](glossary.md) for the terms it coins.

## Record a selection

A selection is a choice between real alternatives for a language, framework, library, runtime,
database, broker, toolchain, test stack, or platform. Mint it by the procedure in
[records.md](records.md). This section says what its four sections hold and replaces none of that
procedure's steps.

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
   is what lets a later reader judge whether the ban still earns its keep. Step 5c of the mint
   procedure applies to every one of these.
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

What the mint procedure requires of every record, and four things a selection tends to owe.

- **What leaving this choice would cost**, narrowed to what actually survives the exit.
- **What would re-argue the decision.** Usually an assumption the choice rests on, named with what
  happens when it moves. Prose, not a table of conditions.
- **What this decision adds to the cost of something else.** The mint procedure already asks what
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
