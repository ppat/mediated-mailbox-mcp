# 0018. Delta sync polls on a short cadence — push delivery rejected

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C1](../../../USE_CASES.md#c1--metadata-always-visible), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

The index must track the live mailbox: new mail classified and masked promptly, label changes
reflected, cursors advanced. Providers offer push notification (Gmail via Pub/Sub webhooks) and
pull (change feeds from a cursor).

## Decision

**A recurring sync job polls every 5 minutes:**

```
changes_since(cursor) → ChangeSet{added, modified, removed}
  ├─ classify added senders
  ├─ mask subjects
  ├─ scan gate → scan or skip
  ├─ apply label/flag changes to the index
  └─ advance cursor; on gap → bounded re-enumeration + alert
```

Short, idempotent, cheap. Push gains little at 5-minute staleness tolerance — mail triage is not a
real-time problem — and would require the inbound path the transport decision removed.

**Cursor gaps are an expected condition with a bounded recovery.** Gmail invalidates history
cursors older than roughly a week; JMAP signals `cannotCalculateChanges`. Both mean the same
thing — resync: detect the gap, re-enumerate over a bounded window, and alert, because *frequent*
gaps signal a stuck sync job rather than provider behavior.

At steady state the tick is small — 50–200 new messages per 5-minute tick — well inside the sync
class's 20% reservation ([ADR-0025](../operability/0025-priority-classes-and-leases.md)), each run
a few seconds dominated by API latency. Scanning is a backfill-scale concern, not a steady-state
one.

## Alternatives considered

- **Push via Pub/Sub webhook.** Rejected: requires public inbound delivery (or an outbound
  long-poll bridge to emulate it), adds a cloud dependency and its failure modes, and buys
  freshness beneath the staleness anyone notices.
- **Faster polling (sub-minute).** Rejected: cost without benefit at this problem's tempo;
  the cadence spends from the same budget as everything else.
- **Sync via periodic full re-enumeration.** Rejected: full traversal is the backfill primitive,
  deliberately separate from the delta feed on the port
  ([ADR-0010](../provider/0010-one-provider-port.md)); using it for sync would pay corpus-scale
  cost per tick.

## Consequences

- Freshness is bounded by the cadence: a new message may be up to one tick stale before it is
  classified and visible. Accepted.
- Gap recovery plus idempotency make the sync job safe to run alongside backfill; reconciling
  counts against the provider during early operation is the drift check.
