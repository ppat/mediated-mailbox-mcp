# 0022. The batch work is four workloads, not one background process

**Status:** Accepted ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Four kinds of background work exist: the one-time backfill, the recurring delta sync, the
human-triggered reorg apply, and the periodic heuristics run. They could share one long-running
worker process or run as separate Kubernetes workloads.

## Decision

**Four separate workloads**, because they differ on every operational axis that matters:

| | Backfill | Delta sync | Reorg apply | Heuristics |
| --- | --- | --- | --- | --- |
| Runtime | minutes–hours | seconds | minutes | seconds |
| Trigger | manual, once | every 5 min | human-approved | daily |
| Reversible | n/a (read-only) | n/a | **must be** | n/a |
| Writes provider | no | no | **yes, bulk** | no |

The row that settles it: reorg apply is the only provider-mutating path, and it needs approval
gating and rollback machinery ([ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md))
that the read-only paths would only be burdened by.

## Alternatives considered

One process carrying all four (whether a dedicated worker or the mediator itself) is the
alternative the table rejects: the four differ on runtime, trigger, reversibility, and whether
they mutate the provider, and the settling row means any shared packaging burdens three read-only
paths with the fourth's approval and rollback machinery. No other packaging was seriously weighed.

## Consequences

- Cross-workload coordination becomes a real requirement rather than shared process state — which
  is exactly what the shared rate budget solves
  ([ADR-0025](./0025-priority-classes-and-leases.md)).
- Each workload is individually killable, resumable, and observable, matching how the failure
  drills are defined in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
