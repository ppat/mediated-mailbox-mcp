---
name: builder
description: Build one change in this repository as a delegated builder subagent, from an empty worktree to a green draft pull request and a report, and through every fix round after review. Use it whenever a subagent builds a ticket or any other pull request here, code or documents, and when the main session briefs one. Part 1 is the main session's, meaning the brief and what it expects back. Part 2 is the builder's procedure, meaning the order of work, the rules every change keeps, the proof a control needs, the gates, the pull request, the report and the fix rounds. Part 3 adds what browser work needs.
---

# Building a change

A builder is a subagent that turns one ticket, or one piece of work the main session hands it, into
a draft pull request that is ready for the `adversarial-review` skill's loop. It works alone in its
own worktree while other builders and reviewers work in theirs. What it hands back is a branch that
passes every gate CI runs, and a report that lets the main session rule on every choice the builder
made without re-deriving it.

| Role | Who | Does | Never does |
| --- | --- | --- | --- |
| Builder | A subagent | Builds the change, documents first, proves it, opens a draft pull request, reports, fixes findings as ruled | Squashes, marks ready, merges, rules on a finding, touches another worktree |
| Main session | The orchestrator and adjudicator | Briefs the builder, rules on its questions and on review findings, squashes and presents | |
| Operator | The person | Decides design choices, departures from a record's words and names, approves names at review, and merges | |

## Part 1. The main session

### The brief

Tell the builder to follow part 2 of this skill, and part 3 when the change touches the browser.
Then fill this in.

```text
Ticket, or the work, and the unit it serves.
Branch name, and the base (current origin/main unless the change stacks on another branch).
Sources: the records, documents and code the work rests on.
Reasoning: what is believed true and on what evidence, what was ruled out and why.
Rulings already made, by the operator or the main session, verbatim where the operator gave them.
Hypotheses: the ways this change is most likely to go wrong.
Where to write the report.
```

### What comes back

Rule on every question the report raises before the review starts, and take to the operator what is
the operator's, which includes every name and unratified decision the builder reports (part 2,
[Choices the builder makes](#choices-the-builder-makes)).
Send every fix round to the **same builder** through `SendMessage`, since it holds what it built and
why. A new builder is started only when the old one's context has drifted or been compacted so it no
longer holds that.

## Part 2. The builder

### The order of work

1. **Set up.** Create your own worktree from the base the brief names, on the branch it names. Never
   touch another worktree, or the main checkout, which stays on `main`. When the change touches the
   browser's TypeScript, install its modules with `bun install --frozen-lockfile` in `ui/browser`
   before the first commit, since the pre-commit formatter hook fails without them.
2. **Read the sources,** the ticket, the records and documents it rests on, and the code it changes,
   before writing anything. Derive the constraints from the records' own words.
3. **Change the documents first,** through the `update-docs` skill, invoked with the Skill tool and
   not paraphrased. It ends with the `coherence-check` skill, which covers the implementation as much
   as the documents. Every document the work touches changes in the same pull request. `ROADMAP.md`'s
   state changes and section moves for this change land in it too, written for the state after it
   merges. A decision that lives only in code is a defect.
4. **Build** to the documents. When building shows a document is wrong, fix the document first, then
   the code.
5. **Prove it** ([Proof](#proof)).
6. **Run the `coherence-check` skill again** over the whole repository as the change now leaves it,
   since step 3 ran before the code existed.
7. **Run the gates** ([Gates](#gates)).
8. **Open the draft pull request** ([The pull request](#the-pull-request)).
9. **Report** ([The report](#the-report)).

### Rules every change keeps

- **A record's own words.** Implement what the decision records, `DESIGN.md`, `USE_CASES.md` and
  `docs/UI.md` say, not a narrowed or reworded reading. A compatible clarification is edited into its
  record in place. A conflict between a record and the work goes to the main session, never resolved
  on your own.
- **Decide at the first consumer.** A mechanism or choice the documents leave open is settled in the
  first change that needs it, by that change, never ahead of it.
- **No relaxed check, no widening without a consumer.** Never relax a check the records require. No
  import list, grant, ban, allow entry or expected finding is loosened unless something in the same
  change needs it, and never beyond what the records allow.
- **No busy work.** Fix what is a defect, meaning a rule broken, a claim false, or a gap measured.
  Something merely wider than needed is not work.
- **Nothing lost from `main`.** Every line `main` has that the change removes is listed in the report
  with its reason. Undoing landed work or a settled design needs the reason the thing was there and a
  documented need to change it.
- **Process rules are not yours to change.** Never edit `.claude/rules/commits.md`, another process
  rule or the user-level instructions to clear a finding or a gate.
- **No invented tooling.** Add no hook, script or tool nobody asked for. When a guard hook refuses a
  command, report it, and do not work around it.
- **Nothing deleted outside your worktree and scratch space** without asking the main session.

### Choices the builder makes

Building never waits on a choice. Make it, keep going, and put it in the report for review.

- **Names.** The operator names components, directories, packages, labels and tables. When the work
  needs a new one, pick a name that is coherent with the whole document set and the code as they
  stand, use it, and report it with the reasoning and the alternatives considered, each with its
  reason, so the operator can approve or rename at review. A rename at review is a mechanical change,
  so it costs less than a stalled build.
- **Libraries and tools.** Every choice of a library or a tool runs the `select-framework-or-tool`
  skill in full. Nothing is picked in passing.
- **Design questions the documents leave open.** Decide as the first consumer, record the decision
  the way `update-docs` routes a decision the operator has not yet ratified, with its alternative,
  and list it in the report for ruling. A choice the operator made earlier is not reopened.

### Proof

- **Every new control lands with its proof,** as `TESTING.md`,
  [ADR-0046](../../../docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md) and the
  preamble of `docs/MUTATIONS.md` require of its kind. That is its `docs/VERIFICATIONS.md` row, a
  violation file for a lint ban, and for a control with a standing automated test its
  `docs/MUTATIONS.md` row, with each break its own patch run through `go tool mutproof`. Read the
  failing assertion, not only the runner's summary. A patch's description says exactly what it
  breaks. A control's test never reads the value it asserts from the code under test.
- **A test counts once it is seen to fail.** Every test the change adds goes red without the change
  it covers, whether or not it proves a control. It watches the outcome the user or the next stage
  sees, not a count of calls or renders.
- **Patches keep applying.** After editing any file, run `git apply --check` on every mutation patch
  whose diff headers name it, a comment edit included, since patches carry context lines. Regenerate
  and re-demonstrate each one that no longer applies.

### Gates

Before reporting, run the steps of every workflow in CLAUDE.md's workflow table that runs on this
pull request, those whose paths it touches and those that run on every pull request, and read the
result of each. Then read `gh pr checks` once the pull request is open, and check that
`git diff origin/main --stat` lists only your change's files.

Integration tests with a remote docker daemon need a tunnel for both ports `pgrun` uses. Close your
tunnel and containers when done.

The workspace is shared and short on memory, so run heavy Go commands one at a time.

The contract run against a real provider runs only when the change touches that provider's adapter,
and then through its workflow started by hand on your branch. Never read, print or touch the test
account's credential files.

### The pull request

- Commit and push only your own branch, with an explicit refspec (`git push origin HEAD:<branch>`).
  Never squash, since the main session squashes after review.
- Open it as a **draft**. Its title follows `.claude/rules/commits.md`, and its body says `Closes #N`
  for the ticket.
- Apply the unit label at creation, and correct the ticket's component labels to match the pull
  request's, as CLAUDE.md's Repository process states.

### Rebasing

Rebase onto current `main` when the main session asks, or when `main` has moved so the branch
conflicts. The rebase meets the standard of the `adversarial-review` skill's pass 5, losing nothing
on either side, which its review checks. Conflicts in `ROADMAP.md`, `docs/MUTATIONS.md` and
`docs/VERIFICATIONS.md` usually sit inside one long table cell that both sides edited, so git
reports the whole line. Compare the two sides word by word to find what each changed, and keep both.
List every conflict in the report with what each side meant and how it was resolved. After the
rebase, list the unmerged files and grep for conflict markers before continuing, re-run the gates,
and re-check every patch the rebase touched.

### The report

Write it where the brief says, and reply with its path and a short digest.

```text
Commit and branch. Gates: each with its result.
Decisions: decision, alternatives considered, the deciding argument, the cost of reversing it.
Names chosen: name, reasoning, alternatives with their reasons.
For the main session to rule: each question with its context and options.
Removed from main: each line with its reason.
Controls: each with its VERIFICATIONS row and its patches, each patch red for its stated reason.
New tests: each with how it was seen to fail.
Rebase conflicts: each with what each side meant and the resolution.
```

### Fix rounds

A round's rulings come from the main session. Fix as ruled, documents first. When a finding names a
class, fix every instance of it, not only the one reported. Re-run the `coherence-check` skill, the
gates and every patch the fix touched, and report the new commit, what each ruling changed, and any
line of `main` the fix removed. A ruling you believe is wrong is argued back with evidence, not
quietly left undone.

## Part 3. Browser work

These add to part 2 when the change touches `ui/browser/`.

- **`docs/UI.md` records every behaviour a test pins down,** such as a state, a failure's wording or
  what refreshes live, each with its alternative. The operator is a backend engineer without UI
  experience, so each UI decision carries a reason a reader outside UI work can follow.
- **Fixtures are recorded from the real server at exactly the path the app requests.** No test
  answers one path with another path's recording, and a recording nothing reads is removed.
- **Message-derived text renders inert,** proven by the test form of
  [ADR-0064](../../../docs/adr/engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md)
  over every surface a message-derived field reaches, and broken under mutproof's `runner: bun`.
  When a break stays green, widen the test rather than pick an easier break.
- **Live surfaces are the ones `docs/UI.md` names,** and no others
  ([ADR-0058](../../../docs/adr/operability/0058-live-surfaces-stream-over-server-sent-events.md)).
  Each has a render-counter test.
- **Reuse what exists.** The cache and the region component under `app/`, and the patterns of
  `docs/UI.md`'s section 12 are used as they are, never copied.
- **A screen's state never outlives what it was derived from.** Data state is keyed by the URL, as
  `docs/UI.md`'s section 19 states, so state computed from the route or from a request is recomputed
  when either changes, and a screen never shows one route's or one account's data under another.
- **A new server dataset or endpoint goes through the registry**
  ([ADR-0057](../../../docs/adr/operability/0057-one-dataset-endpoint-behind-a-registry.md)), with
  integration tests under `pgrun` and the contract, types and descriptor drift checks of
  [ADR-0065](../../../docs/adr/engineering/0065-contract-built-from-registry-consumed-as-generated-types.md).
  A new route registers through the recording mux, which the routes analyser enforces.
