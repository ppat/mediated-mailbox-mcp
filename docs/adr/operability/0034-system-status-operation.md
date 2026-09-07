# 0034. One read-only system-status operation exposes recorded per-account operational state

**Status:** Accepted ·
**Serves:** [O5](../../../USE_CASES.md#o5--clients-can-tell-failures-apart)

## Context

Several recorded states produce client-visible behavior that reads as failure: a backfill pass
still running, a sync cursor gone stale, a scan backlog, a rate controller in backoff, a failed
provider authentication. The design already commits, in miniature, to explaining rather than
misreporting — the denial envelope carries "pending content scan" specifically so the agent
explains a backlog instead of reporting a permissions bug
([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)).

## Decision

**One read-only operation on the client surface returns per-account operational state**: backfill
pass flags, sync cursor age and last successful tick, scan backlog depth, rate-controller state
(current rate versus target, whether in backoff), and the last provider authentication outcome.
It applies the denial envelope's commitment to the other states that produce client-visible
weirdness, letting a client answer "why did this fail, why is this denied" on its own instead of
the operator opening dashboards. The cost is one read-only operation over existing rows.

Two bounds keep it honest:

- **It reads only recorded state** — the tables that already exist — never reaching into
  components: the
  [contracts-only pillar](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts)
  applied at the operation's birth.
- **It exposes operational state only** — no content, no policy details.

## Alternatives considered

- **The fuller "health as a tool" surface** proposed by the advice this operation came from,
  including provider-account and OAuth health for agent self-debugging. Its case: richer agent
  self-debugging. Adopted in this reduced form instead: the philosophy was already the design's
  own (the denial envelope's reasons), the fuller surface was judged optional, and the two bounds
  above are what made the reduced form acceptable.

## Consequences

- The operation is part of the canonical API surface, so
  [ADR-0030](./0030-api-core-mcp-thin-adapter.md)'s one-to-one parity applies to it
  automatically: it exists identically on both roots.
- Assumptions about other components: each state listed is already recorded in a queryable table
  by its owning subsystem — the operation adds no instrumentation and no new obligation on any
  component.
- Both bounds are controls, dispositioned in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md): the operational-state-only bound has its
  violation injection; the recorded-state-only bound is parked there with its standing reason.
