# 0099. A body request loads the policy before it decides, sharing a load only with the requests that arrived before it started

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

[C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)'s scope note requires a
domain added to the deny list to take effect on the next call, not the next cache refresh, and
[ADR-0002](../redaction/0002-fetch-time-re-evaluation.md) re-derives the sender class at fetch time
against current policy so that it does. Policy reaches a process as an immutable snapshot it loads
on a cadence of its own, and a request takes the active snapshot once at entry
([ADR-0041](./0041-policy-as-immutable-snapshots.md)). An API server that loaded the policy only
when the set of accounts it serves changed would never see an edit to the deny list while it ran. A
body request is the first decision that releases content, so the API server's load cadence is
decided here.

What can be read cheaply is limited. The policy rules carry no revision a process could compare,
and a deleted rule leaves no trace in the table. A connection pooler in transaction mode, which
[ADR-0066](../data/0066-data-access-generated-from-sql.md) allows, drops `LISTEN`, which is why
[ADR-0090](../operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md) did not use it
for accounts. The Session's policies, `process/policies`
([process/README.md](../../../process/README.md#the-policies-policies)), validate what they read,
keep the active snapshot on any failure, and raise the reload-failure alarm.

## Decision

- **Each body request has the policy loaded before it decides.** The load runs through the
  Session's policies, and the request then takes the active snapshot once, as ADR-0041 requires.
  The snapshot it takes is the one a load that started after the request arrived left active.
- **Concurrent requests share a load.** Every request waiting when a load starts is answered by that
  load. A load already running when a request arrives does not count for it, since it may have read
  the tables before the edit the request must see, so the request waits for the next load, which
  starts once the running one ends.
- **A failed load decides nothing differently.** The request is decided against the active valid
  policy, and the policies raise the reload-failure alarm, as ADR-0041 states. Before any load has
  succeeded there is no policy, and every sender is restricted.
- **The scheduled reload of the process's Session also loads the policy, every time.** The
  metadata reads and the reload-failure alarm stay current without a body request, and a failed
  load is retried at the next reload. The body request's load and the scheduled reload's run on the
  policies of the process's one Session
  ([ADR-0090](../operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md),
  [ADR-0142](./0142-each-program-is-a-spec-that-one-process-runs.md)), so the gate in the API
  server and the scan gate in the worker decide under the same policy wherever they share a
  process.

The accepted cost is about one small read of the policy tables per body request, fewer under a
burst, since concurrent requests share one.

## Alternatives considered

- **A periodic load alone.** For it, performance, since no request reads the policy tables. Against
  it, a deny-list edit binds only at the next load, which is exactly the "next cache refresh" C2's
  scope note names as falsifying, so taking it would need an amendment to the outcome contract.
- **A periodic load with a probe of a revision marker on each request.** For it, a request reads one
  small value and loads only when the policy changed. Against it, it needs a marker every writer of
  the policy tables bumps. The schema has none, a trigger maintaining one is code in the database,
  which [ADR-0060](./0060-no-code-in-the-database.md) forbids, and a hand edit that does not bump
  the marker goes unseen.
- **`LISTEN` and `NOTIFY` on policy changes.** For it, a change arrives at once. Against it, a
  transaction-mode pooler drops the notice silently, the reason ADR-0090 gives.
- **Reading the rules directly on each request, beside the Session's policies.** For it, no shared
  load to coordinate. Against it, it repeats the policies' validation and their rule for telling a
  read they cannot trust from an empty policy, which the policies exist to hold in one place.
- **Each request runs its own load.** For it, no coordination between requests. Against it, a burst
  of requests runs a burst of loads one after another, since the policies take their loads in
  turn, for no request's benefit.

## Consequences

- A deny-list edit binds on the first body request that arrives after the edit is written, once the
  edit validates. An edit that does not validate is not yet policy, as ADR-0041 states.
- A request waits for a load before it is decided, so a slow read of the policy tables slows body
  requests. A load that never finishes holds them until each one's own deadline.
- Releasing gated calendar content follows the same answer.
- Assumptions about other components. The Session's policies load every served account in one
  load and take their loads in turn, read and swap together, so a load a body request started is
  never overtaken by an older one the scheduled reload ran beside it. A load the API server starts
  is cancelled only when the API server stops, so a client that abandons its request never cancels
  a load another request waits on.
