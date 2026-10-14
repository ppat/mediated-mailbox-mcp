---
name: identify-ticket-dependencies
description: A script that reads every epic and ticket from GitHub in a few batched calls and keeps a local map of each ticket's epic, unit, State and blockers. It shows which tickets are pending or blocked, which changed on GitHub since the last read, including changes made outside the control loop, and which ticket bodies break the ticket template. Use it in the control session whenever the loop needs ticket States or dependencies, instead of reading issues one at a time.
---

# Identifying ticket dependencies

[tickets.mjs](./tickets.mjs) is a capability, not a process. Run it whenever a fresh read is worth
its few calls, and read its compact output rather than issue bodies, so the control session's
context holds conclusions instead of bodies. Every command prints one line per item and nothing when
there is nothing, and `--json` prints the full data instead. It has no dependencies. Run it with the
bun that `mise.toml` pins, from the repository's main checkout so it reads ROADMAP.md on `main`,
after `eval "$(~/.local/bin/gh-app-env)"`.

| Command | Reads GitHub | Does |
| --- | --- | --- |
| `bun .claude/skills/identify-ticket-dependencies/tickets.mjs refresh --state FILE` | Yes | Writes the whole map to FILE and prints the count per derived State, and the tickets whose State in their epic's table differs from the derived one |
| `... diff --state FILE [--write]` | Yes | Prints each ticket added, removed or changed since FILE was written, every ticket as added when FILE does not exist yet, marking a ticket that became pending UNBLOCKED and one that became blocked BLOCKED. With `--write` it then replaces FILE |
| `... show --state FILE [N ...]` | No | Prints the map, or the named tickets, with the blockers each blocked ticket waits on |
| `... deps N [N ...]` | Yes | Reads the named tickets and prints each one's blockers and whether each is built |
| `... candidates [--busy N,N]` | Yes | Lists by ticket number the tickets eligible to launch, each with its unit, its epic and that epic's place on the value path, its row in the epic, its blockers, the open tickets that wait on it, and where its `Blocked by` line and ROADMAP.md's dependency table disagree |
| `... sync [--set N=in-progress\|hold\|pending ...] [--diff] [--apply]` | Yes | Prints each State cell that differs from the derived State, and each one `--set` names. `pending` lifts a hold by writing the derived State. `--diff` adds the body line before and after. `--apply` writes only the changed epics, through `gh api` with the body as JSON on standard input |

What it reads and how it judges:

- **The epics.** The roadmap ticket's sub-issues, and each one's sub-issues with their bodies, in
  aliased GraphQL queries of up to a hundred issues each. Each ticket's State in its epic comes
  from the epic's table, in the form the templates under
  [.github/ISSUE_TEMPLATE/](../../../.github/ISSUE_TEMPLATE/) give it.
- **The blockers.** The `Blocked by` row of each open ticket's body, as the
  [ticket template](../../../.github/ISSUE_TEMPLATE/ticket.md) states it. A ticket or pull request
  is built when it closed through a merged pull request, and a unit when ROADMAP.md marks it `[x]`.
- **The derived State.** Built, pending or blocked, by the rules the `reconcile-epics` skill states,
  or closed-not-built for a ticket closed as a duplicate or as not planned, which is not reported as
  differing. In-progress and hold need the roster, so a table that reads either is not reported as
  differing.
- **Eligible to launch.** Open, of this repository, pending in its epic's table and by its blockers,
  with no open pull request that closes it and not named in `--busy`, under an epic the production
  point gate of the `launch-ticket` skill admits. Of the gate it applies only the value path's order
  and which epics are closed, not its collapse step. It reads the value path's order from ROADMAP.md
  and a point's issue from the roadmap ticket's sub-issues by its title.
- **The structure checks.** `refresh` and `diff` print a `STRUCTURE` line for a sub-issue with no row
  in its epic, a row with no sub-issue, an open issue with a ticket header line or a `unit:` label
  under no epic, and an open ticket whose header names another increment or point than its epic.
- **The format check.** An open ticket without the header line or the `Blocked by` row, or whose
  cell holds anything other than `none` or references and units with parenthesised notes, is named
  on standard error with its number, mapped with no dependencies, and makes the command exit 2. An
  empty `none` is valid. Closed tickets and tickets of other repositories are not checked.
