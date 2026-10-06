# 0112. The base policy is read and written in a transaction of its own, which names no account and reaches only the base policy's rows

**Status:** Accepted ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account), [C4](../../../USE_CASES.md#c4--the-sensitive-sender-list-keeps-pace)

## Context

The base policy is the rules every account inherits, rows of `policy_rules` with a null account, and
each change to it is a row of `policy_changes` with a null account
([ADR-0004](../classification/0004-sender-list-decides.md),
[ADR-0102](../mutation/0102-policy-changes-recorded-in-an-append-only-history.md)). Every unit of
data access runs in a transaction that set the account and read it back, and the shared
transaction helper fails one whose account reads empty, because row-level security on an empty
account returns no rows and raises nothing ([ADR-0047](./0047-schema-first-data-access.md),
[ADR-0016](./0016-schema.md)). Row-level security lets every account read the base rules and lets
a writer name only its own account, so no role could write a base rule.

The UI writes base rules, from an account's policy screen and from the base policy's installation
screen, which needs no account to exist ([docs/UI.md sections 8.7](../../UI.md#87-policy) and
[8.14](../../UI.md#814-base-policy)). The installation screen and the installation endpoint read the
base rules and their history with no account named
([docs/UI.md section 17.4](../../UI.md#174-the-bespoke-endpoints)). The UI's policy management is the
first work that does either, so it decides how.

## Decision

- **The transaction helper has a second entry point, for the base policy.** It sets the account to
  the empty string and a second transaction-local setting, `app.base`, to `on`, reads both back, and
  fails the transaction before any data access unless they read so.
- **Row-level security lets the UI's role write a row with a null account in `policy_rules` and
  `policy_changes` only in such a transaction**, one where `app.base` reads `on` and the account
  reads empty. Every account still reads the base rules and the base history. An account's
  transaction, which sets an account, writes no base row, and a base-policy transaction, whose
  account is empty, reads and writes no account's row. One transaction is one scope.
- **A base-policy transaction runs only the base policy's statements, and they run in no other
  transaction.** They sit in a data-access subsection of their own, `db/policyrules/base`, and the
  check that holds every data-access call to the transaction helper
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)) refuses that subsection's
  statements outside a base-policy transaction and any other subsection's inside one. So the empty
  account, which silently returns no rows from an account's table, is never read where an account's
  rows were meant.
- **A base rule written from an account's policy screen is written in a base-policy transaction**,
  and the counts the screen shows for it are read in the account's own transaction.

## Alternatives considered

- **A reserved account value meaning the base policy**, such as `/`, which account setup refuses.
  For it, one setting and no change to the helper. Against it, one setting then carries two meanings,
  and every policy reading the account would have to tell them apart.
- **A second database role for the base policy's writes.** For it, the grant alone separates the two
  scopes. Against it, the UI holds and rotates a second credential, and the chart mounts it.
- **Bypassing row-level security for the UI's role.** For it, no new setting. Against it, every
  statement the UI runs loses the third layer of account isolation, the reads included.
- **Letting an account's transaction also write base rows.** For it, no second entry point.
  Against it, one transaction then spans two scopes, and any account's transaction becomes a path to
  the base policy every account inherits.

## Consequences

- The verification rows that held "the base rules are written by none" and "a transaction that
  did not set the account fails" are revisited. The base rules are written by the UI's role in a
  base-policy transaction and by nothing else, and the only transaction that runs with the account
  empty is a base-policy transaction, which reaches only the base policy's statements.
- The policy loader reads the base rules inside each account's transaction as before
  ([ADR-0114](../engineering/0114-a-torn-base-policy-read-is-read-again-before-it-fails-the-reload.md)).
  A base-policy transaction is the UI's alone.
- Assumptions about other components. The settings are transaction-local, so a pooled connection
  carries neither into its next transaction, as the account setting does not. A statement written
  by hand outside the data-access library could set `app.base` itself, and the UI's raw-SQL check
  refuses such statements in the UI ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)).
- The scope rule and the check confining the base subsection are controls. Their violation
  injections are catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
