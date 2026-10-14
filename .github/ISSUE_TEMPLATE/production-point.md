---
name: Production point
about: The issue of one production point, listing its tickets. Closing it crosses the point
title: Production point <n> — <point name>
---

Tickets of production point `<n>`, grouped by unit. Each ticket is a sub-issue of this issue, a deployment repository's ticket included, and this issue is a sub-issue of the roadmap ticket [#118](https://github.com/ppat/mediated-mailbox-mcp/issues/118), which lists the value increments and production points. Closing this issue crosses the point. This issue is not itself a ticket.

Each ticket's **State** is one of: **pending**, which can start any time because every ticket it is blocked by is built; **blocked**, which waits on a ticket not yet built; **in-progress**, being worked; **hold**, held by the operator and not started until the operator lifts the hold; **built**, merged to `main`.

[Production point `<n>` in ROADMAP.md](https://github.com/ppat/mediated-mailbox-mcp/blob/main/ROADMAP.md#POINT-SECTION-ANCHOR)

| Unit | Ticket | What lands | State |
| --- | --- | --- | --- |
| **`<unit ID>` — `<unit title>`** | #`<ticket>` | `<what lands>` | `<State>` |
| | #`<ticket>` | `<what lands>` | `<State>` |
| **`<deployment repository>`** | `<repository>`#`<ticket>` | `<what lands>` | `<State>` |

## Checklist

- [ ] `<optional, a step of the point's release or its verification that no ticket carries>`
