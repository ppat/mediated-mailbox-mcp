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
| **Interactive** (client surface: body fetches, listings) | 30% of the target, guaranteed while the rate is at least that | never yields |
| **Sync** (delta job) | 20% of the target | brief queuing acceptable |
| **Batch** (backfill, reorg apply) | remaining 50%, yields | absorbs all decrease first |

When the controller halves the rate after a throttle
([ADR-0024](./0024-conservative-target-aimd.md)), batch absorbs the entire cut before interactive
loses anything: the agent stays responsive while backfill quietly slows — the correct priority,
since backfill has no deadline and the operator does. A cut comes out of batch first, then sync,
then interactive. A class's reservation is held against the classes below it and not against
those above, while that class asks for it. A class that asked for no lease in the last second
lends its share, so interactive waits on a lower class at most until a lent lease is spent,
within one second, and the bucket refills to its call's cost. Batch yields to both.

**Cross-process coordination is a Postgres row, not new infrastructure.** A `rate_state` row per
account ([ADR-0016](../data/0016-schema.md)) holds the current rate, throttle state, and the token
bucket, and a table beside it holds every grant of the last second, which are the leased tokens.
Processes acquire it via advisory lock or `SELECT ... FOR UPDATE SKIP LOCKED`, lease roughly one
second's worth of tokens, and work from the lease locally. The issuer reads the database's clock
after the lock is held, because `now()` is fixed when the transaction starts and a worker that
waited would stamp its lease early. The transaction runs at read committed, where each statement
sees what the lock's previous holder committed. One round trip per second per worker is negligible
and requires nothing beyond the database already present.

Two rules keep the leasing honest:

- **Every lease carries an expiry**, so a crashed worker's tokens return to the pool instead of
  being lost — token loss otherwise presents as slow, mysterious rate collapse. A lease is drawn
  from [ADR-0024](./0024-conservative-target-aimd.md)'s token bucket, holds at least the cost of
  the call it is for, and is spent within one second. It expires one second after it is issued,
  judged by the database's clock, which every process shares. An expired lease stops counting against its class's share,
  which is how a crashed worker's tokens return to the pool. Nothing it drew is put back into the
  bucket, because tokens a worker spent before it crashed would then be issued twice. Every ask
  records its class's ask instant whether or not it is granted, and a worker still waiting asks
  again at least once a lease period, so an abandoned ask stops counting as demand.
- **The hard cap is enforced at lease issuance** — the issuer never hands out tokens past the cap,
  so no controller bug or worker bug can collectively exceed it.

Throughput inside the budget comes from overlap, not rate: pipelined stages (fetch → classify →
scan → persist) as bounded async queues so network waits overlap CPU work, and batched database
writes of 500 to 1000 rows so single-row inserts never become the bottleneck the API is not. The
batched write is a multi-row upsert, expressed as one insert selecting from unnested array
parameters. `COPY` is not available and never was. PostgreSQL refuses it on a table with row-level
security for any role the policy applies to, which is every runtime role here
([ADR-0016](../data/0016-schema.md)). It would not serve this design in any case, because backfill
resumes at page granularity and replays the partial page
([ADR-0017](../data/0017-two-pass-backfill.md),
[ADR-0045](../engineering/0045-crash-injection-testing.md)) so every bulk write is idempotent by
decision, and `COPY` cannot upsert. At steady state the controller sits at target with nothing to
adapt to, and the sync tick fits comfortably inside its reservation.

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
- Every worker holds a lease of its own, one row per grant in the grants table
  [ADR-0016](../data/0016-schema.md) draws, and the issuer deletes rows more than a second old.
