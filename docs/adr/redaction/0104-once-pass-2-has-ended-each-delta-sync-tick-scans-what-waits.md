# 0104. Once backfill's second pass has ended, each delta sync tick decides and scans the messages waiting for a scan, a bounded number per tick

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [O2](../../../USE_CASES.md#o2--observable)

## Context

Delta sync's tick runs the scan gate on added messages and scans the bodies the gate selects
([ADR-0018](../data/0018-delta-sync-polls.md)). A message also goes back to pending scan when its
sender is removed from the sensitive list, and the normal scanning machinery picks it up like any
other unscanned mail ([ADR-0037](./0037-delisting-transition.md)). Backfill's second pass scans
what waits while it runs, and it ends ([ADR-0017](../data/0017-two-pass-backfill.md)). Backfill runs
again only after a change of scanner or of the gate's thresholds
([ADR-0096](./0096-a-scanner-change-reopens-backfill.md),
[ADR-0098](./0098-every-backfill-run-decides-each-gate-skip-again.md)). A growing pending backlog is
a failure in its own right, since pending denies the body
([ADR-0093](./0093-composite-scan-gate.md)). The question is how a message that goes back to pending
after the second pass has ended is reached.

The gate reads sender statistics, so it cannot run on a cold index
([ADR-0093](./0093-composite-scan-gate.md)). Backfill's second pass reads the messages waiting for a
scan in the order of their identifiers and runs the delisting comparison at its start.

## Decision

- **Before backfill's second pass has ended for an account, delta sync scans nothing of it.** It
  classifies and masks the mail it adds and leaves it pending, so the second pass decides it. The
  account's completion flag of the second pass is the signal.
- **Once the second pass has ended, each tick runs the delisting comparison of
  [ADR-0037](./0037-delisting-transition.md) and then decides and scans the account's pending
  messages**, whatever made them pending. That is mail the tick added, a delisted sender's messages,
  and a message the second pass left waiting. Each is decided by the same gate, under the same
  thresholds and policy, and scanned by the same scanner and the same conversion as backfill's
  second pass, with the gate's decision recorded the same way.
- **A tick decides a bounded number of messages**, a value of delta sync's configuration. It reads
  them in the order of their identifiers from where the last tick stopped, kept in the tick's
  checkpoint, and starts again from the first waiting message once it reaches the end, or once the
  delisting comparison marked any message. What is left waits for the next tick.
- **A body fetch that fails leaves the message pending** with a failed item naming its error class,
  and the next pass over the backlog tries it again. A throttled fetch, or one whose credential the
  provider refuses once it was read again from the account's row, ends the tick's scanning for the
  account, since the provider is refusing the tick rather than the message. Nothing from that
  message on is decided, so no message is decided without the hits the unscanned bodies would have
  added, and the tick's checkpoint stays at the last message decided before it, so the next tick
  reads that message first.
- **Delta sync emits the account's scan backlog depth once the second pass has ended**, read after
  each tick's scanning.

The deciding argument. The normal machinery ADR-0037 names is the gate and the scanner the second
pass runs, and delta sync is the workload that runs after the second pass ends. Reading pending mail
whatever made it pending reaches every cause with one read, and waiting for the second pass keeps
the gate off a cold index and keeps the two workloads from deciding the same messages at once from
the start of each tick.

## Alternatives considered

- **Delta sync scans only the mail it added in the same tick.** For it, the smallest change to a
  tick. Against it, a delisted sender's messages and those the second pass left waiting stay pending
  for good once backfill has ended, which [ADR-0093](./0093-composite-scan-gate.md) counts as a
  failure.
- **Reopening backfill's second pass whenever a message goes back to pending.** For it, one workload
  keeps all scanning of what is stored. Against it, backfill runs only when the deployment starts it
  after a change ([ADR-0096](./0096-a-scanner-change-reopens-backfill.md)), so nothing would run the
  reopened pass.
- **A separate scanning workload.** For it, scanning would have a process of its own. Against it, a
  fifth workload beside the four [ADR-0022](../operability/0022-four-workloads.md) decides, with its
  own role, image and schedule, for work delta sync already runs on its cadence.
- **Scanning every pending message in each tick, unbounded.** For it, the backlog empties at once.
  Against it, a large delisting would make one tick last as long as a backfill pass, against the
  short tick [ADR-0018](../data/0018-delta-sync-polls.md) describes.

## Consequences

- A delisted sender's messages are scanned within as many ticks as the backlog takes at the bound,
  and their bodies are denied as pending until then.
- A body the conversion refuses or the provider keeps failing is fetched again once each time the
  reading position passes it, and recorded as a failed item each time. The bound caps that work per
  tick.
- Delta sync and backfill run the same scanner section and the same gate thresholds, or each
  reopens the other's work ([ADR-0096](./0096-a-scanner-change-reopens-backfill.md),
  [ADR-0098](./0098-every-backfill-run-decides-each-gate-skip-again.md)).
- Assumptions about other components. Backfill's second pass sets its completion flag when it ends
  and clears it when a backfill run reopens it, so delta sync stops scanning from its next tick while
  the second pass is due again. The flag is read once, at a tick's start, so a backfill run that
  reopens the second pass during a tick can decide the same messages as that tick's remaining
  reads. A message's decision is recorded only while it still waits for a scan, so the workload that
  commits second has its whole commit refused and fails, leaving each message as the other recorded
  it. Delta sync rebuilds the statistics of each sender whose messages it adds, so the gate
  reads them warm.
- The rules above are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
