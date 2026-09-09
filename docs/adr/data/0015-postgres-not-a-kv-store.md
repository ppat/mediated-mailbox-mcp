# 0015. The metadata & state store is Postgres — a key-value store answers the wrong access pattern

**Status:** Accepted ·
**Pillar:** [Metadata always flows; sensitive bodies never do](../../../DESIGN.md#metadata-always-flows-sensitive-bodies-never-do) ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [G2](../../../USE_CASES.md#g2--historical-understanding), [A3](../../../USE_CASES.md#a3--bulk-change-is-reversible), [O2](../../../USE_CASES.md#o2--observable)

## Context

Whole-corpus analysis and reorganization planning require a cached metadata index — the provider
APIs cannot answer "every sender with no label, ranked by volume" without enumerating the mailbox
per question. The project needs a store for the metadata index as well as its own internal state.

## Decision

**Postgres holds everything: metadata, senders, plans, audit.**

| Requirement | Postgres | Dragonfly/Redis |
| --- | --- | --- |
| Ad-hoc relational analytics | native SQL | manual index per query shape |
| Multi-column filtering | query planner | pre-computed key patterns |
| Transactional bulk update during reorg | native | no multi-key transactions with rollback |
| Durable audit log with retention | partitioned tables | awkward |
| Schema evolution | migrations | rewrite key layout |
| Subject search | trigram indexes (`pg_trgm`) | none |
| Sender embeddings | `pgvector` | separate store |
| Backup to existing object storage | native with CNPG | snapshot juggling |

The decisive row is none of them singly but what they share: the access pattern is **exploratory
relational analytics, not key-value lookup**. The agent's most valuable queries during corpus
analysis and reorg planning are ones nobody can enumerate in advance, and a query planner handles
those without pre-designing an index per question. Dragonfly's strengths — latency and throughput
at scale — do not bind here: the corpus is small (on the order of a hundred thousand messages, a
few hundred megabytes) and perceived latency is dominated by agent inference, not storage.

**Dragonfly remains legitimate for a different job** — rate-limit buckets, job coordination,
ephemeral session state are Redis-shaped problems. It is still not added now: Postgres advisory
locks cover coordination ([ADR-0025](../operability/0025-priority-classes-and-leases.md)) until
several accounts and several concurrent workers demonstrate a real bottleneck rather than a
predicted one.

## Alternatives considered

The comparison table is the analysis; Dragonfly/Redis is rejected *for this job* on access-pattern
grounds while explicitly not dominated for the coordination job it may earn later. A third option,
no cache at all (query providers live), was rejected before it reached the table: corpus-wide
questions would each cost a full enumeration against rate-limited APIs, making
[G2](../../../USE_CASES.md#g2--historical-understanding) unmeetable in practice.

## Consequences

- One store means one backup story, one access-control story, and transactional integrity between
  the index, plans, and audit rows that reference each other.
- Release decisions re-derive classification against current policy at fetch time
  ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)), so losing the index means
  re-backfilling, not losing mail — and never leaking it.
- Adding a second store later is a contained decision: it would take over the coordination rows,
  not the relational corpus.
