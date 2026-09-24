# ratelimit

A narrow, named shared library, published as `mediated-mailbox-ratelimit`. Shared code is pure, or
it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Every spender runs the same rate rules, the cap at lease issuance, the priority split, the adaptive
controller and lease expiry ([ADR-0024](../docs/adr/operability/0024-conservative-target-aimd.md),
[ADR-0025](../docs/adr/operability/0025-priority-classes-and-leases.md)). Those rules are pure and
sit in `ratelimit/core/`. The lease code that reads and writes the shared coordination row and its grants table is impure
and sits in `ratelimit/lease/`. Keeping both in one library means the rules are not split from the
code that applies them and the lease code is not duplicated per deployable.

The library connects to the database as no role of its own. Its statements in `db/ratestate` run
under the role of each deployable that spends from the budget, which is why those roles hold the
grants the statements need
([ADR-0075](../docs/adr/data/0075-one-runtime-role-per-deployable.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)).

## Series and the rules that read them

The lease code also emits the rate limiter's metrics through the registry each process passes it
([ADR-0076](../docs/adr/engineering/0076-metrics-emitted-through-client-golang.md)). Every series is a
gauge or counter labelled by `account`. The alerting rules that read them are
`packaging/chart/alerting-rules.yaml`, whose promtool tests sit in `ratelimit/lease/testdata/`
([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)).

| Series | Kind and other labels | Emitted by | Read by |
| --- | --- | --- | --- |
| `mediated_mailbox_ratelimit_rate` | Gauge | Each spending process, as it last read or set the rate | Dashboards |
| `mediated_mailbox_ratelimit_granted_total` | Counter, `class` | Each spending process, in the provider's units | Dashboards |
| `mediated_mailbox_ratelimit_throttles_total` | Counter, `scope` of `user`, `project` or `unknown` | Each spending process | Dashboards |
| `mediated_mailbox_ratelimit_account_rate` | Gauge | The mediator's collector, from each account's rate state | The collapse rule and the absence rule |
| `mediated_mailbox_ratelimit_account_floor` | Gauge | The mediator's collector | The collapse rule |
| `mediated_mailbox_ratelimit_account_bucket_level` | Gauge | The mediator's collector | Dashboards |
| `mediated_mailbox_ratelimit_account_seconds_since_ask` | Gauge, infinite when no class has asked | The mediator's collector | Both collapse rules |
| `mediated_mailbox_ratelimit_account_seconds_since_grant` | Gauge, infinite when nothing was granted | The mediator's collector | The stall rule |

The runaway rule reads none of these, because a runaway is the failure in which lease accounting is
wrong. It reads the cost of provider requests counted where each request is sent, which the
alerting rules file names.
