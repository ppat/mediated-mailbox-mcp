---
name: Roadmap ticket
about: The one issue that lists the value increments' and production points' issues
title: Roadmap tickets, by value increment and production point
---

Every ticket cut from [ROADMAP.md](https://github.com/ppat/mediated-mailbox-mcp/blob/main/ROADMAP.md), grouped by the roadmap's [value increments](https://github.com/ppat/mediated-mailbox-mcp/blob/main/ROADMAP.md#the-value-path) and [production points](https://github.com/ppat/mediated-mailbox-mcp/blob/main/ROADMAP.md#production-points). Each value increment and each production point has an issue of its own, a sub-issue of this one, which lists its tickets by unit with each ticket's State, and each ticket is a sub-issue of its increment's or point's issue. Neither this issue nor an increment's or a point's issue is a ticket.

## Overview

| Value increment or production point | Status | Units | Tickets |
| --- | --- | --- | --- |
| [V`<n>` — `<increment name>`](https://github.com/ppat/mediated-mailbox-mcp/issues/INCREMENT-ISSUE) | `open` or `closed` | `<unit>` · `<unit>`, and later work under `<unit>` · `<unit>` | `<built>` / `<total>` |
| [Production point `<n>` — `<point name>`](https://github.com/ppat/mediated-mailbox-mcp/issues/POINT-ISSUE) | `open` or `crossed` | `<unit>` · `<unit>` | `<built>` / `<total>` |
| [Production point `<n>` — `<point name>`](https://github.com/ppat/mediated-mailbox-mcp/blob/main/ROADMAP.md#production-point-N--SECTION) | `not created` | `<unit>` · `<unit>` | `<built>` / `<total>` |

Status is the state of the row's issue, `crossed` once a point's issue has closed, and `not created` for a point that has no issue yet, whose row links its ROADMAP.md section. Tickets is the number built over the number of the issue's tickets, leaving out those closed as a duplicate or as not planned.
