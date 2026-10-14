---
name: reconcile-epics
description: Keep the epics true, meaning the roadmap ticket, the value increment issues and the production point issues. Create the epics ROADMAP.md and the work call for, close a production point's increment issues when they are complete, move misplaced tickets, add missing rows, keep the roadmap ticket's overview current, and derive every ticket's State. Use it in the control session on every iteration of the control loop, and when a ticket session sends a ticket row.
---

# Reconciling the epics

The epics are the roadmap ticket that [ROADMAP.md](../../../ROADMAP.md) names, one issue per value
increment and one per production point, as [CLAUDE.md](../../../CLAUDE.md#repository-process)'s
Repository process states. The control session writes their bodies, each in the form of its
template under [.github/ISSUE_TEMPLATE/](../../../.github/ISSUE_TEMPLATE/), `roadmap.md`,
`value-increment.md` and `production-point.md`, and gives each epic it creates the label `epic`.
The `identify-ticket-dependencies` skill's script reads every epic and ticket in a few calls. Its
`refresh` and `diff` name each ticket with no row in its epic, each row with no sub-issue, each open
ticket under no epic and each ticket whose header names another epic than its own, which are the
cases the steps below act on. The script's `add-ticket`, `move-ticket`, `remove-row` and `update-overview` make those edits,
printing what they change and writing it with `--write`, so no epic body is edited by hand. Which
epic a ticket belongs in stays a judgement made here.

## Epics and rows

- **Increments.** When ROADMAP.md on `main` adds a value increment, create its increment issue,
  link it as a sub-issue of the roadmap ticket, and run the script's `update-overview --write`, which adds its row.
- **Points.** Once a ticket under the issue of one of a production point's increments, those
  before it and after any previous point, has started, create the point's issue if it has none,
  link it as a sub-issue of the roadmap ticket, and run the script's `update-overview --write`, which links it from
  its row in place of the ROADMAP.md section the row linked. Once ROADMAP.md on `main` lists every
  unit of those increments as delivered and every ticket under
  their issues is built, close those increment issues.
- **Placement.** When a ticket's work is placed in another increment or point than the issue it
  sits under, as its header line and ROADMAP.md on `main` read, move the ticket to that issue,
  both its sub-issue link and its row, with the script's `move-ticket TICKET --epic EPIC --write`. When the two disagree, move nothing and name the ticket
  in the report.
- **Rows.** Add a row to an increment or point issue for every ticket under it that has none, and
  for every row a ticket session sent, with the script's `add-ticket TICKET --epic EPIC --write` and by the rule for [a ticket row](#a-ticket-row), which also
  says which rows are refused.
- **Overview.** Keep each row of the roadmap ticket's overview true to its issue's tickets and
  to ROADMAP.md on `main`, with the script's `update-overview --write`, which recomputes each row's Status, Units
  cell and ticket count from the epics. A row with no sub-issue is removed with `remove-row TICKET --epic EPIC --write`.

Then set every row's State from what is true now, by the first rule that fits. The script's `update-states` prints the rows whose State these rules change, leaving hold and
in-progress rows of open tickets alone, and `update-states --write` writes only those State cells. For an open ticket, read its `Blocked by`
line from the issue body. Each blocker it names is a ticket, a pull request or a unit, and counts as
built when the ticket is closed with its pull request merged, the pull request has merged, or
ROADMAP.md on `main` lists the unit as delivered.

- **Built** when the ticket is closed with its pull request merged.
- **Left as it is** when it is closed without a merged pull request, and named once in the report
  for the user to settle. It is never launched.
- **Left as it is** when it is open and reads hold, as the `launch-ticket` skill's pause and holds
  state.
- **In-progress** when it is open and has a live session, an open pull request, or was in-progress
  already. Only the user moves a stranded ticket on, as [loop.md](../../loop.md#stranded-tickets)
  states.
- **Pending** when it is open and every blocker its `Blocked by` line names is built.
- **Blocked** when it is open and a blocker its `Blocked by` line names is not built.

So a ticket that landed this iteration lets its dependents become pending in the same pass. Name
in the report any row you could not set. Because every State is derived again each iteration, a
change missed while no control session ran is caught on the next one.

## A ticket row

A ticket row a ticket session sends is added to the issue of the increment or point its work is
placed in, once you have checked on GitHub that the ticket exists and is a sub-issue of that issue,
with the State its `Blocked by` line gives it. Then run the script's `update-overview --write`, which raises that
issue's ticket count in the roadmap ticket's overview and keeps its Status and Units columns as the
overview step states. Refuse a row, whether a session sent it or you found the sub-issue without
one, when that issue is closed, since CLAUDE.md's Repository process adds no ticket there. Name the refused sub-issue in the report, and tell the
session that cut it, if one is live, to place the work in the open epic it belongs in.
