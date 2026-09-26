# 0091. The accounts table holds only what every listing needs, and each account's state lives apart under row-level security

**Status:** Accepted ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

Every table keys on its account and row-level security confines each transaction to the account it
names ([ADR-0016](./0016-schema.md)). Several processes need the list of every account. The UI's
account selector, the mediator's accounts listing and the account snapshot each deployable loads
([ADR-0090](../operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)) all need each
account's identifier and provider, and nothing else. An account also carries its own state, the
backfill flags, the sync cursor and the last authentication, and the sealed credential and rate
target the account setup writes
([ADR-0080](./0080-accounts-and-credentials-live-in-the-database.md)).

A read policy that showed every row of one table holding both would show every column the listing
role may read, and those roles also read an account's state in that account's own transactions. A
column grant cannot tell the two reads apart.

## Decision

- **`accounts` holds each account's identifier and provider only.** The roles that list accounts
  read all of its rows. They are the UI's, for its account selector, and the mediator's, backfill's,
  delta sync's and the reorg workload's, for their account snapshots. How the heuristics job finds
  its accounts is decided where it is built. Writes stay confined to the transaction's account, as
  on every other account-keyed table.
- **`account_state` holds everything else an account carries**, one row per account under the
  per-account row-level security policy. That is the sealed credential, the rate target, the
  backfill flags, the sync cursor and the last authentication. The account setup writes both rows
  in one transaction.
- **A reader that finds an account with no state row treats it as not connected.**

## Alternatives considered

- **A read-all policy on the one table, narrowed by column grants.** For it, no change to the
  table. Against it, the same role reads the state columns in its own account's transactions, so
  the grant that lets it do so would also let a listing read every account's state.
- **A read-all policy on the one table, applied only while a listing setting is on in the
  transaction.** For it, the listing and the state reads are told apart by the transaction rather
  than by the table. Against it, the separation then rests on every listing statement setting and
  clearing that flag correctly, where a separate table makes a listing unable to reach a state
  column at all.
- **A view exposing identifier and provider, owned by the migration role.** For it, the table
  stays whole. Against it, the schema checks in `db/check` model tables only, so the view would be
  a blind spot for the predicate and account checks.
- **A listing role that bypasses row-level security.** For it, nothing else changes. Against it,
  [ADR-0075](./0075-one-runtime-role-per-deployable.md) gives roles to deployables only, and the
  roles test refuses any runtime role that bypasses row-level security.

## Consequences

- `accounts` is an exception to the rule that every row is confined to its account, for reads
  only. No listing can reach a credential, because the credential is not in a row a listing reads.
- The two tables' statements sit in separate data-access subsections, the listing in
  `db/accounts` and `account_state`'s in `db/accountstate`, so a role whose import list admits the
  listing is never planned against a statement that reads or writes a credential
  ([ADR-0066](./0066-data-access-generated-from-sql.md)).
- The isolation test names `accounts` as the read exception and requires every other account-keyed
  table, `account_state` among them, to stay confined.
- Assumptions about other components. The account setup writes both rows together, and the tests
  of [ADR-0060](../engineering/0060-no-code-in-the-database.md) cover a fault between the two
  writes.
