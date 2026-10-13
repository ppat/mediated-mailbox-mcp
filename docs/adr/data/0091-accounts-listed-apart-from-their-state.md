# 0091. The accounts table holds only what every listing needs, and each account's state lives apart

**Status:** Accepted ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

Every table keys on its account, and every statement reaches only the account its transaction names
([ADR-0016](./0016-schema.md),
[ADR-0135](./0135-every-statement-reads-its-account-from-the-transaction.md)). Several processes
need the list of every account. The UI's account selector, the API server's accounts listing and
the account snapshot each deployable loads
([ADR-0090](../operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)) all need each
account's identifier and provider. Where the provider authenticates through an OAuth client, two
readers also need the client the account connects through
([ADR-0106](../provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)).
The account snapshot builds each account's token source from that client, and the UI's installation
screens name the accounts a client serves before it is replaced or removed. The API server's
accounts listing gives its clients the identifier and provider alone. No listing needs anything
else. An account also carries its own state, the backfill flags, the sync cursor and the last
authentication, and the sealed credential and rate target the account setup writes
([ADR-0080](./0080-accounts-and-credentials-live-in-the-database.md)).

A listing that read every row of one table holding both would reach every column the listing role
may read, and those roles also read an account's state in that account's own transactions. A column
grant cannot tell the two reads apart.

## Decision

- **`accounts` holds each account's identifier, its provider and the OAuth client it connects
  through, and nothing else.** The roles that list accounts read all of its rows, through the one
  statement the account predicate exempts and their column grants. They are the UI's, for its
  account selector, and the API server's, backfill's, delta sync's and the reorg workload's, for
  their account snapshots. How the heuristics job finds its accounts is not decided here and is
  tracked in [ROADMAP.md's open decisions](../../../ROADMAP.md#open-decisions). Writes stay confined
  to the transaction's account, as on every other account-keyed table.
- **`account_state` holds everything else an account carries**, one row per account, which every
  statement reaches through the transaction's account. That is the sealed credential, the mailbox
  address it remembers, the rate target, the backfill flags, the sync cursor and the last
  authentication. The account setup writes both rows in one transaction.
- **A reader that finds an account with no state row treats it as not connected.**

## Alternatives considered

- **One table, listed by a statement exempt from the account predicate and narrowed by column
  grants.** For it, no change to the table. Against it, the same role reads the state columns in
  its own account's transactions, so the grant that lets it do so would also let a listing read
  every account's state.
- **A view exposing identifier and provider, owned by the migration role.** For it, the table
  stays whole. Against it, the schema checks in `db/check` model tables only, so the view would be
  a blind spot for the predicate and account checks.

## Consequences

- `accounts` is an exception to the rule that every row is confined to its account, for reads
  only. No listing can reach a credential, because the credential is not in a row a listing reads.
- The two tables' statements sit in separate data-access subsections, the listing in
  `db/accounts` and `account_state`'s under `db/accountstate`. The reads of the account's other
  state sit in `db/accountstate` itself, and the credential's statements one directory down, in
  `db/accountstate/credential`. So a role whose import list admits the listing or the other state
  is never planned against a statement that reads or writes a credential
  ([ADR-0066](./0066-data-access-generated-from-sql.md)).
- The statement check names the accounts listing as the one statement exempt from the account
  predicate, and every other statement, `account_state`'s among them, stays confined to its
  transaction's account. The listing test holds every listing role to reading every account.
- Assumptions about other components. The account setup writes both rows together, and the tests
  of [ADR-0060](../engineering/0060-no-code-in-the-database.md) cover a fault between the two
  writes.
