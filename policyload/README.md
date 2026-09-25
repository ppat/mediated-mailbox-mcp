# policyload

A narrow, named shared library, published as `mediated-mailbox-policyload`. Shared code is pure, or
it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Backfill, the mediator, delta sync, the reorg workload and the heuristics workload each hold the
active policy as one immutable snapshot and replace it only with an update that validates
([ADR-0041](../docs/adr/engineering/0041-policy-as-immutable-snapshots.md)). The validation, the
composition of the base policy with an account's overlay and the atomic swap are pure and sit in
`core/policy`. What is left is impure and the same in every one of them. It reads the base rows and
each account's overlay rows from the policy tables, hands them to the pure half, and raises the
reload-failure alarm when a reload fails, whether the read or the validation.

The case for one library over per-deployable glue is one rule the glue could get wrong. The pure half
accepts a policy with no rules as valid. A read of the tables that failed, or returned nothing
because of a fault, looks like that empty policy. Treated as one, it would classify the senders the
active policy lists as normal and release content the previous snapshot withheld. So the library tells a
read it cannot trust apart from a policy that is empty, and a read it cannot trust never replaces the
active snapshot. Written once, that rule holds in every deployable that loads policy. Written five
times, one copy that gets it wrong fails open.

The alarm is the other shared part. Each loading process emits the same series, and one alerting rule
reads it ([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)), so the
series is defined once, in this library.

| Series | Kind | Emitted by | Read by |
| --- | --- | --- | --- |
| `mediated_mailbox_policyload_reload_failed` | Gauge, 1 while the latest reload failed and 0 once one succeeds | Each loading process, on the registry it passes in | `MediatedMailboxPolicyReloadFailed` in `packaging/chart/alerting-rules.yaml`, whose promtool tests sit in `policyload/testdata/` |

The series is a gauge rather than a counter because a process first loads policy as it starts,
usually before its first scrape, and a rule over a counter whose first sample is already 1 sees no
increase.

The library connects to the database as no role of its own. Its statements in `db/policyrules` run
under the role of each deployable that imports it
([ADR-0075](../docs/adr/data/0075-one-runtime-role-per-deployable.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)).

When a process reloads is its caller's. The library loads when asked and swaps only on success. It
reads each account's own rules and the base rules in one statement, and a reload whose accounts
read different base rules is a read it cannot trust, so an edit to the base policy landing midway
never composes accounts from two versions of it. A reload stopped because its caller cancelled it
keeps the active policy and raises no alarm, and one that ran out of time is a failed read like any
other.
