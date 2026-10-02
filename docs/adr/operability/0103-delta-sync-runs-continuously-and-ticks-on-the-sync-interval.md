# 0103. Delta sync runs until stopped, ticks on the sync interval, and serves its metrics between ticks

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [O1](../../../USE_CASES.md#o1--rate-limited-politely), [O2](../../../USE_CASES.md#o2--observable), [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox)

## Context

Delta sync runs every five minutes, for seconds at a time
([ADR-0018](../data/0018-delta-sync-polls.md), [ADR-0022](./0022-four-workloads.md)). Two of the
series it emits are read across scrapes. The runaway rule sums the cost of provider requests counted
in each spending process, and reads the hard cap each process emits beside its count
([ADR-0077](./0077-conditions-raised-as-alerting-rules.md)). A counter's increase needs two samples
inside the rule's window, so a process that exits between two scrapes leaves its requests uncounted.
Key replacement waits until delta sync reports a scan series for every listed account and every
OAuth client, all reading 0, and a missing series never counts as 0
([ADR-0092](./0092-key-replacement-by-keyring-and-re-seal.md)). A process that exits between scrapes
leaves those series missing most of the time.

[ADR-0090](./0090-accounts-reach-deployables-as-reloaded-snapshots.md) left delta sync's form open,
a process that runs until stopped or one that runs and exits, and required the account snapshot to
be loaded within each five-minute run either way. The application does not know its platform, so it
cannot know the platform's scrape interval or reach a push gateway the platform runs
([ADR-0051](../engineering/0051-environment-contract.md)).

## Decision

- **Delta sync is a process that runs until stopped.** It ticks on the sync interval, a value of its
  configuration whose default is ADR-0018's five minutes
  ([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)). A tick is the
  unit of work. A tick that outlasts the interval delays the next one rather than overlapping it.
- **A tick that starts records as failed the account's latest tick and latest gap recovery still
  recorded as running.** Ticks do not overlap, so a run still running when the next tick starts
  was stopped before it recorded its end, as backfill records a run it resumes after one stopped
  ([ADR-0017](../data/0017-two-pass-backfill.md), [ADR-0022](./0022-four-workloads.md)).
- **It serves the health probe and the metrics endpoint for its whole life**, so every series it
  emits stays in the scrape between ticks. The request cost and the hard cap of
  [ADR-0077](./0077-conditions-raised-as-alerting-rules.md) are counted on one registry for the
  process, and a counter keeps its value from one tick to the next.
- **Each tick takes the account snapshot again**, re-seals what it opens with a key that is not the
  current one and sets the key-scan series from what it found
  ([ADR-0090](./0090-accounts-reach-deployables-as-reloaded-snapshots.md),
  [ADR-0092](./0092-key-replacement-by-keyring-and-re-seal.md)). It also loads the policy of the
  accounts the snapshot lists, and builds the rate limiter under the target each account's state row
  sets. A snapshot whose read fails keeps the previous one and is logged, as ADR-0090 decides.
- **The key-scan series are two gauges.** `mediated_mailbox_sync_credential_on_old_key` carries one
  series per account `accounts` lists, labelled by the account, and
  `mediated_mailbox_sync_client_secret_on_old_key` one series per row of `oauth_clients`, labelled by
  the provider. Each reads 1 while its value is sealed to a key other than the current one or cannot
  be opened, and 0 once it is sealed to the current key or when there is no value to seal. A series
  whose account or provider a later tick no longer lists is removed.

The deciding argument. A process that stays up keeps its counters and gauges in every scrape, so the
runaway rule and the key-retirement gate read delta sync the way they read the mediator, with no
assumption about the platform's scrape interval and no component the chart does not already assume.

## Alternatives considered

- **A job the platform starts every five minutes, pushing its series to a push gateway before it
  exits.** For it, a process that holds nothing between ticks and a platform scheduler doing the
  scheduling. Against it, the application would have to know a gateway the platform runs, against
  [ADR-0051](../engineering/0051-environment-contract.md), and the chart would need a component
  [ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md) does not assume.
- **A job that stays up for two scrape intervals before it exits.** For it, a scheduled job that is
  still scraped twice. Against it, the application would have to know the platform's scrape interval,
  which it cannot, and the key-scan series would still be missing between runs.
- **Writing the counts to the database for the mediator to emit.** For it, the mediator already
  runs continuously and emits each account's rate state. Against it, delta sync's measurement would
  be braided into another component, and the runaway rule must read the cost counted where each
  request is sent ([ADR-0077](./0077-conditions-raised-as-alerting-rules.md)).

## Consequences

- Delta sync holds a database connection and its keyring for as long as it runs, as the mediator
  does. A crash between ticks loses nothing, since every tick's writes are durable before it ends.
  A crash during a tick leaves its run, and a recovery it started, recorded as running until the
  account's next tick records them failed.
- A runaway spread across delta sync's ticks reaches the runaway rule, since its count is scraped
  like the mediator's. ADR-0077's remaining gap, a process living less than two scrape intervals,
  no longer applies to delta sync.
- Assumptions about other components. The platform runs one delta sync process at a time, a
  rollout included, keeps it running and restarts it when it stops, and scrapes its metrics
  endpoint as it does the mediator's
  ([ADR-0051](../engineering/0051-environment-contract.md)).
