# ratelimit

A narrow, named shared library, published as `mediated-mailbox-ratelimit`. Shared code is pure, or
it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Every spender runs the same rate rules, the cap at lease issuance, the priority split, the adaptive
controller and lease expiry ([ADR-0024](../docs/adr/operability/0024-conservative-target-aimd.md),
[ADR-0025](../docs/adr/operability/0025-priority-classes-and-leases.md)). Those rules are pure and
sit in `ratelimit/core/`. The lease code that reads and writes the shared coordination row and its
grants table is impure and sits in `ratelimit/lease/`. Keeping both in one library means the rules
are not split from the code that applies them and the lease code is not duplicated per deployable.
`lease.Call` makes one port call under a lease in a priority class and tells the controller how the
call went, a throttle or a server error included, so every spender reports its calls the same way.

The library connects to the database as no role of its own. Its statements in
`db/ratestate/limiter` run under the role of each deployable or job kind that spends from the
budget, which is why those roles hold the grants the statements need
([ADR-0118](../docs/adr/data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)).

## Series and the rules that read them

The lease code also emits the rate limiter's metrics through the registry each process passes it
([ADR-0076](../docs/adr/engineering/0076-metrics-emitted-through-client-golang.md)). Every series is a
gauge or counter labelled by `account`. In the worker each job kind passes a registry that also
labels each series with `job_kind`, the job kind that produced it
([ADR-0117](../docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)). The
alerting rules that read them are `packaging/chart/alerting-rules.yaml`, whose promtool tests sit
in `ratelimit/lease/testdata/`
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
wrong. It reads the cost of provider requests counted where each request is sent, against the
account's hard cap each spending process emits beside that count from its provider's declared
ceiling. A second rule fires when an account's cost is counted and no hard cap is, since the
runaway rule then has no threshold. The provider adapters emit both series, and
[provider/README.md](../provider/README.md) lists them with the two rules.
