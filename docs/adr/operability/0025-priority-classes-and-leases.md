# 0025. One budget per account, split by priority class, shared across processes by database leases

**Status:** Accepted ·
**Serves:** [O1](../../../USE_CASES.md#o1--rate-limited-politely), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

The rate budget is per account, but the spenders are separate processes — the mediator pod serving
interactive queries, the backfill job, the sync job, a reorg apply. Two problems follow: a naive
shared queue lets a running backfill starve the agent's live queries behind hours of enqueued
work, and separate processes can collectively overrun a budget each respects individually.

## Decision

**One shared budget per account, divided by priority class rather than served first-in-first-out:**

| Class | Reservation | Behavior under contention |
| --- | --- | --- |
| **Interactive** (MCP: body fetches, listings) | 30% of current rate, guaranteed | never yields |
| **Sync** (delta job) | 20% | brief queuing acceptable |
| **Batch** (backfill, reorg apply) | remaining 50%, yields | absorbs all decrease first |

When the controller halves the rate after a throttle
([ADR-0024](./0024-conservative-target-aimd.md)), batch absorbs the entire cut before interactive
loses anything: the agent stays responsive while backfill quietly slows — the correct priority,
since backfill has no deadline and the operator does.

**Cross-process coordination is a Postgres row, not new infrastructure.** A `rate_state` row per
account ([ADR-0016](../data/0016-schema.md)) holds the current rate, throttle state, and leased
tokens. Processes acquire it via advisory lock or `SELECT ... FOR UPDATE SKIP LOCKED`, lease
roughly one second's worth of tokens, and work from the lease locally. One round trip per second
per worker is negligible and requires nothing beyond the database already present.

Two rules keep the leasing honest:

- **Every lease carries an expiry**, so a crashed worker's tokens return to the pool instead of
  being lost — token loss otherwise presents as slow, mysterious rate collapse.
- **The hard cap is enforced at lease issuance** — the issuer never hands out tokens past the cap,
  so no controller bug or worker bug can collectively exceed it.

Throughput inside the budget comes from overlap, not rate: pipelined stages (fetch → classify →
scan → persist) as bounded async queues so network waits overlap CPU work, and batched database
writes (multi-row upserts or `COPY` every 500–1000 rows) so single-row inserts never become the
bottleneck the API is not. At steady state the controller sits at target with nothing to adapt to,
and the sync tick fits comfortably inside its reservation.

**A Redis-shaped store is the eventual fit for distributed token buckets — and is deliberately not
added now.** Postgres leasing holds until several accounts and several concurrent workers
demonstrate a real bottleneck; by then the shape of the need is known rather than predicted (the
store-choice reasoning is [ADR-0015](../data/0015-postgres-not-a-kv-store.md)).

## Alternatives considered

- **First-in-first-out over one bucket.** Rejected: a backfill enqueues hours of work; every
  interactive query would wait behind it. Starvation of the human's agent by the human's own batch
  job is the exact inversion of correct priority.
- **Static per-process budgets** (give backfill 60%, mediator 30%, sync 10%, permanently).
  Rejected: idle reservations are waste — when no backfill runs, most of the budget would sit
  unspendable; classes over one live budget reassign headroom automatically.
- **A dedicated coordination service or Redis-style store now.** Rejected as premature: it adds an
  infrastructure dependency to solve a contention level that does not yet exist.

## Consequences

- Lease accounting is load-bearing for politeness: its runaway failure mode and the corresponding
  alert are defined in [ADR-0024](./0024-conservative-target-aimd.md), and its drills in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
