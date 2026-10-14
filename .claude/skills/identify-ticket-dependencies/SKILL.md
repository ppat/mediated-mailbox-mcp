---
name: identify-ticket-dependencies
description: A script for reading and managing tickets' States, dependencies and the epics' tables, with a few batched GitHub calls instead of reading issues one at a time. It shows which tickets are pending or blocked, what changed on GitHub since the last read, which tickets may be launched, and it names ticket bodies that break the template. It also sets States and edits the epics' rows. Use it whenever those facts or edits are needed.
---

# Identifying ticket dependencies

[tickets.mjs](./tickets.mjs) reads every epic and ticket in a few GitHub calls and prints tickets'
States, blockers and what changed, so the caller reads a short table instead of issue bodies. It
gives facts. What to launch, hold or place stays with the skills named below. `--help` has every
command, option, column and value, so this page only says when to use what.

Run it as `bun .claude/skills/identify-ticket-dependencies/tickets.mjs COMMAND ...`, with the bun
that `mise.toml` pins, from the main checkout so it reads ROADMAP.md on `main`, after
`eval "$(~/.local/bin/gh-app-env)"`. MAP below is the map file,
`~/code/.session-notes/mediated-mailbox-control/ticket-map.json`
([loop.md](../../loop.md#where-the-state-lives)), kept outside the checkout.

## Which command, when

| Situation | Command |
| --- | --- |
| Find what changed on GitHub since the last look, including edits made outside the loop. The first run has no map yet and builds it | `diff --map MAP --save` |
| Answer a question about States or blockers from the last saved map, with no GitHub call | `show --map MAP [TICKET ...]` |
| Check a few named tickets against GitHub right now, for example before acting on one | `deps TICKET ...` |
| Rebuild the whole map after something looks wrong with it | `refresh --map MAP` |
| Decide what to launch. Give `--exclude` the tickets with a live session or owned by a session the user runs. Which one launches is `launch-ticket`'s choice | `candidates --exclude N,N` |
| After a landing, or whenever States in the epics have drifted: sets built and each dependent's pending or blocked. The rows of open tickets reading hold or in-progress are left alone | `update-states` |
| Launch a ticket, or hold one | `set-state TICKET in-progress` and `set-state TICKET hold` |
| Lift a hold, or set a stranded ticket back | `reset-state TICKET` |
| Keep the roadmap ticket's overview true to the epics | `update-overview` |

A STRUCTURE row from `diff` or `refresh` names what to fix:

| Row | Fix |
| --- | --- |
| no row: a sub-issue of an epic has no row | `add-ticket TICKET --epic EPIC`, once `reconcile-epics` confirms the ticket belongs in that epic. If it belongs in another, `move-ticket TICKET --epic OTHER` |
| no sub-issue: an epic has a row for a ticket that is not its sub-issue | `remove-row TICKET --epic EPIC` |
| under no epic: an open ticket is in none | `add-ticket TICKET --epic EPIC`, where `reconcile-epics` decides EPIC |
| misplaced: a ticket's header names another increment or point than its epic | `move-ticket TICKET --epic EPIC`, where `reconcile-epics` decides EPIC |

## Writing safely

A writing command only prints a plan until it is given `--write`. Run it without, read the plan,
then run it again with `--write`. Before `--write` replaces an issue body it saves the old body and
prints its path under `/tmp/tickets/`. `--help` has the command that restores one.

## When it fails

- **Exit 2.** A named open ticket's body breaks the ticket template, so the script mapped it with
  no blockers. Do not treat that ticket as launchable until its body is fixed.
- **Exit 1.** The arguments were wrong or a `gh` call failed. A 401 means the token expired, so run
  `eval "$(~/.local/bin/gh-app-env)"` again.
