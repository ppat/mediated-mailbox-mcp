# 0058. Live surfaces stream over server-sent events from the UI's Go server, with polling as the fallback

**Status:** Proposed ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

The operator asked for visibility into running batch work as a dynamic view that refreshes as it
runs, and for the ability to inspect a failed run. The jobs surfaces and the home's running-work
strip therefore update without a reload. The UI's Go server reads Postgres directly
([ADR-0021](../mutation/0021-approval-surface.md)) and serves the browser as static files
([ADR-0042](../engineering/0042-implementation-stack.md)), so the transport is the UI's own choice
and touches no other component.

## Decision

- **The UI's Go server streams changes to live surfaces over server-sent events.** One stream per
  account at `/api/{account}/events`. The server polls the recorded state every 2 seconds and
  emits one event per changed object carrying that object's whole current state, never a delta,
  so a missed event costs nothing. The objects are job runs, the rate state, and plans while
  applying. The event shape is stated in [docs/UI.md](../../UI.md#175-the-streams-event-shape).
- **The browser reconnects with exponential backoff** from 1 second to 30 seconds, refetches the
  view on reconnect, and **falls back to polling every 5 seconds** after three failed reconnects.
  It shows a live indicator with the age of the last update, and pauses the subscription while the
  tab is hidden, resuming and refetching when the tab becomes visible.
- **Only recorded state is streamed.** The server reads the same tables the screens read, never a
  component, the bound [ADR-0034](./0034-system-status-operation.md) sets.

The operator asked for the behavior and did not rule on the transport, so this record is proposed
and its ratification is tracked in [ROADMAP.md](../../../ROADMAP.md#open-decisions).

## Alternatives considered

- **Browser polling only.** Its case was nothing to add to the server. Not proposed as the primary
  because each open view then polls independently and the interval trades freshness against load.
- **WebSockets.** Its case was bidirectional and widely supported. Not proposed because the UI sends
  nothing upstream on these surfaces, and a one-directional stream is the smaller mechanism.

## Consequences

- The browser framework must merge streamed updates into view state without a full re-render, a
  requirement listed in [docs/UI.md](../../UI.md#16-framework-requirements).
- Assumptions about other components: the workloads record their progress in the runs, events,
  and failures tables of [ADR-0016](../data/0016-schema.md), which the UI's role can read
  ([ADR-0022](./0022-four-workloads.md)).
- No control. Freshness of a live view is a behavior, and no violation injection is defined for it.
