# 0077. Rate collapse, rate runaway, a failed policy reload and a cursor gap are Prometheus alerting rules, shipped with the chart and tested in CI

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [O2](../../../USE_CASES.md#o2--observable), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Several records name a condition that must reach a person.
[ADR-0024](./0024-conservative-target-aimd.md) names two for the rate limiter, collapse and
runaway, and says runaway must page rather than show on a dashboard.
[ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md) wants a failed policy reload loud
and immediate, and [ADR-0018](../data/0018-delta-sync-polls.md) alerts on a cursor gap. None of
them says how a condition leaves the application.

[ADR-0051](../engineering/0051-environment-contract.md) bounds the answer. The application emits
metrics and logs, and collecting them and routing anything to a person are the platform's. The
application does not know its platform. [ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)'s
chart assumes nothing beyond core Kubernetes. So the repository can prove that a condition's
signal is emitted and that a rule over it fires, and cannot on its own prove that someone was
paged.

## Decision

- **Each condition above, rate collapse, rate runaway, a failed policy reload and a cursor gap,
  is a Prometheus alerting rule over metrics the application emits** through
  [ADR-0076](../engineering/0076-metrics-emitted-through-client-golang.md). The application
  computes no condition itself and sends no notification. Conditions other records place
  elsewhere stay there, such as a failed write-back, which is loud in the deployable's logs
  ([ADR-0082](./0082-rotation-writeback-to-the-database.md)), and the body-fetch anomaly, which the
  UI shows from recorded rows.
- **The rules live in the chart's directory and ship as a `PrometheusRule` resource behind a values
  switch that is off by default.** The chart therefore still assumes only core Kubernetes, and a
  platform without the Prometheus Operator loads the same rules file its own way.
- **Every rule is tested with `promtool test rules`**, run from a Go test so its mutation
  demonstration runs like any other, firing above its threshold and silent below it.
- **The account-level series come from the mediator, which runs continuously.** It reads each
  account's rate-state row and emits the rate, the floor, the bucket's level, how long since any
  class last asked and how long since the last grant. A worker still waiting asks again at least
  once a lease period, so an abandoned request stops counting as demand within a minute. The job workloads run only now and then, so their series are absent between runs
  by design. A rule that fires when the mediator's series are absent covers missing data, so it
  never reads as a healthy account.
- **The runaway rule reads the cost of provider requests counted where each request is sent,**
  never a figure derived from lease accounting, because a runaway is the failure in which lease
  accounting is wrong. Each spending process emits its own count, and the rule sums them for the
  account over two minutes. A lease is spent up to a second after it is issued, so a window of
  seconds fires on correct operation, and a counter's increase needs two samples inside its window,
  so a one-minute window never fires on a platform that scrapes once a minute. The rule fires when
  those two minutes hold more than `hard_cap` times 120 seconds of cost. Each spending process
  emits its account's `hard_cap` beside its count, from the ceiling its provider's adapter
  declares, and the rule reads the highest `hard_cap` seen over the same two minutes, so a process
  that exits or misses a scrape does not take the threshold with it. A second rule fires when an
  account's cost is counted and no `hard_cap` is, so a process that forgets it cannot silence the
  first. The runaway rule serves every provider.
- **Collapse is two rules, each only while a class has asked within the last minute**, so an idle
  account never trips them. One fires when the rate stays at the floor for more than five minutes,
  the case [ADR-0024](./0024-conservative-target-aimd.md) names. The other fires when nothing has
  been granted for more than five minutes, which catches issuance stopped while the rate is
  untouched, as a stored fill instant ahead of the clock does.
- **Routing a firing rule to a person is the platform's**, as is proving that a page arrived. The
  repository proves the signal and the rule.
- **The policy loader's reload alarm and delta sync's gap alert follow the same form**, each a rule
  over a metric its unit emits.

## Alternatives considered

- **Publish the rules file as a release artifact instead of in the chart.** The case for it is that
  the chart stays free of an operator's resource entirely. Rejected because a file kept outside the
  chart drifts from the metric names the chart's images emit, and the switch already keeps the
  resource out of a cluster that lacks the operator.
- **The application computes each condition and exposes it as a yes-or-no gauge.** The case for it
  is that the logic is Go code under ordinary tests. Rejected because a condition held for five
  minutes or summed across processes needs every process's data, which no single process holds,
  and the platform's rule engine already does exactly that.
- **Surface the conditions only through recorded rows, the status operation and the UI.** The case
  for it is that it needs no monitoring stack. Rejected because a record read on request is not
  loud and immediate, and a page is required. The UI may still show the same states, as it does
  a recovered sync gap.
- **The application notifies a person itself.** The case for it is that the page is then proven in
  this repository. Rejected because it makes the application know its platform, against
  [ADR-0051](../engineering/0051-environment-contract.md), and gives the application a new
  outbound destination with a credential of its own.

## Consequences

- The chart carries a template for the Prometheus Operator's `PrometheusRule`, rendered only when
  its switch is on, and the Go tests run `promtool` over the rules, so `promtool` is one of the
  pinned tools.
- The page reaching a person is proven on the deploying side.
- Each spending process counts the cost of the provider requests it sends. A process that exits
  between two scrapes can leave its last requests uncounted, which matters for delta sync's short
  ticks, and how its count reaches the runaway rule is decided with delta sync. More generally the
  rules see a process's cost only once it has been scraped twice inside their window, so a runaway
  spread across processes that each live less than two scrape intervals reaches neither rule.
- The runaway rule's two minutes do not see the one more bucket a forward step of the database
  clock or a stale stored instant can release ([ADR-0024](./0024-conservative-target-aimd.md)),
  since two minutes at the target stay far below its threshold. A stored instant that stays stale
  holds issuance at `hard_cap`, which at most equals the threshold over two whole minutes, so the
  rule, which fires only above it, stays silent.
- Assumptions about other components: the platform scrapes every spending process and the
  mediator, keeps the series long enough for a five-minute rule, and routes the runaway rule to a
  person.
