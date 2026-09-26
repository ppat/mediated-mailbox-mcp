# accountload

A narrow, named shared library, published as `mediated-mailbox-accountload`. Shared code is pure,
or it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

The mediator, backfill, delta sync and the reorg workload each hold their accounts, the
installation's OAuth client for each provider that has one, and the opened credentials as one
account snapshot
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).
Each writes a rotated credential back by compare-and-set on the bytes it last knew
([ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md)), and delta
sync re-seals what it opens with a key that is not the current one and reports the scan the key's
retirement waits on
([ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)). This library
is that loading, that write-back and that re-seal. The sealing and opening themselves are
[`credential/`](../credential/README.md)'s.

The case for one library over per-deployable code is rules a copy could get wrong without anyone
noticing.

- **A read it cannot trust never replaces the snapshot.** A reload whose read fails keeps the
  previous snapshot, and one that succeeds with no account serves none. A copy that treated a
  failed read as an empty list, or kept stale accounts after an empty one, would serve the wrong
  set.
- **A write-back never puts back a value someone else replaced.** Every write of a sealed value is
  a compare-and-set on the bytes the process last read or wrote, and a reload keeps a rotated
  credential whose write-back failed only while the stored bytes are still those. A copy that
  wrote unconditionally would put a revoked grant back after the operator re-authorized the
  account.
- **An OAuth client is optional per provider.** An account whose provider authenticates without one
  loads without one, and nothing looks for a client that does not exist. A copy that assumed every
  provider has a client would refuse such an account or fail on its load.
- **Only delta sync re-seals, and its scan never passes on silence.** It reports a series for every
  account and every stored client secret, and a missing series never reads as done.

Four copies of these rules would drift, and one that got any of them wrong would fail silently.
Written once, they hold in every deployable that calls a provider.

The library connects to the database as no role of its own. Its statements run under the role of
each deployable that imports it
([ADR-0075](../docs/adr/data/0075-one-runtime-role-per-deployable.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)). The listing comes from
`db/accounts` and each account's state and credential from `db/accountstate`
([ADR-0091](../docs/adr/data/0091-accounts-listed-apart-from-their-state.md)).

When a process reloads is its caller's. The library loads when asked and swaps only on a read it
can trust. A unit of work takes the snapshot once.
