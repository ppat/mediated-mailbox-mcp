# 0135. Every statement reads its account from the transaction, and no table carries a row-level security policy

**Status:** Accepted · **Pillar:** [Accounts are isolated by structure, not
convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) · **Serves:**
[P3](../../../USE_CASES.md#p3--multi-account)

## Context

Three layers kept one account's rows from another. Every generated data-access function took the
account, which the compiler checked. The statement check in `db/check` required every statement
against an account-keyed table to compare `account_id` with a parameter
([ADR-0047](./0047-schema-first-data-access.md),
[ADR-0066](./0066-data-access-generated-from-sql.md)). And a row-level security policy on every
account-keyed table filtered it to the transaction-local setting `app.account`, which the shared
transaction helper sets and reads back ([ADR-0016](./0016-schema.md)).

The transaction helper took the account once, and each statement took it again as its own parameter.
A statement bound to account B inside a transaction set to account A passed the compiler, the
statement check and the txhelper analyser, and only the policy stopped it. That was the one case
row-level security caught that nothing else did. It also stood behind a statement the check misread
and SQL written by hand outside the data-access library.

Row-level security bounds no compromised process. Roles and grants do that
([ADR-0118](./0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md)), and any role can call
`set_config` itself. An account's credential is sealed with the account bound into its additional
data, and the loader opens it naming the account it asked for, so a credential read from another
account's row fails to open
([ADR-0088](../operability/0088-credentials-sealed-with-hpke-x-wing.md)). The audit log and the
policy history are append-only by grants.

Its cost was measured on PostgreSQL 18.6, the image the test substrate pins, with the repository's
migrations and two accounts of 100,000 synthetic messages each, running the repository's own
statements as the owning role and as the runtime roles, in custom and in forced generic plans.

- **Query time was not a cost.** On every statement under 5 ms the difference with and without
  row-level security was under 0.6 ms, because the statement's own account comparison implies the
  policy, which then filters once.
- **Indexes and statistics were.** PostgreSQL uses a condition as an index condition ahead of a
  policy only when its function is leakproof. Array operators (`@>`, `&&`, `<@`, `=`), `ILIKE`,
  `citext` comparison, JSON field access and row comparison are not. Under the runtime roles a GIN
  index on labels was ignored (a rare label took 17 ms against 0.16 ms as the owner), a trigram
  index on the subject was ignored (27 ms against 0.6 ms), and an index on `(account_id,
  from_email)` was ignored for filtering (24 ms against 0.8 ms). The policy also hid column
  statistics from those operators, so a common subject word was estimated at 4 rows against an
  actual 18,011, and that search took 46 ms against 1.3 ms. Marking the functions leakproof in a
  scratch database made every one of those indexes serve at the owner's speed, so leakproofness was
  the whole cause.
- **Design and code were.** The base policy needed a transaction setting of its own and two write
  policies ([ADR-0112](./0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)),
  the accounts listing a policy naming five roles
  ([ADR-0091](./0091-accounts-listed-apart-from-their-state.md)), and the operation log a policy
  looking up its plan, so every role touching the log read plan columns. `COPY` was refused on every
  table with a policy. Every new account-keyed table carried a policy and its tests.

## Decision

- **Every statement against an account-keyed table compares `account_id` with
  `current_setting('app.account')`, and every insert into one takes its account from there.** No
  statement takes an account as a parameter, so no statement can be given an account other than its
  transaction's. The generated functions take no account, and the account enters only through the
  transaction helper's `Run`, which the txhelper analyser forces every data-access call through
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)), so every call names its account
  where the compiler checks it.
- **The statement check requires that form.** It reads selects, inserts, updates and deletes and the
  queries nested in the clauses it reads, refuses every other kind of statement, `MERGE` and
  `TRUNCATE` among them, and refuses a reference to an account-keyed table in any clause it does not
  read, so a table the statement names reaches it only where the check read it and tied it. A table
  named only inside a string of SQL that a function runs, such as `query_to_xml`, is outside the
  statement's parse tree and is left to review, as SQL a component writes by hand is. It refuses a
  reference to an account-keyed table that no conjunction of equalities ties to the transaction's
  account, the setting read with exactly one argument, an account column compared with a parameter,
  an insert taking its account from anything but that setting or a column it ties to it, with only
  the base policy's statements inserting the literal null, and any update of an account column, an
  upsert's included, whatever the value, since such a write moves a row to another account or
  between the base policy and an account. No runtime role holds an update grant on an account column
  either. Its stated exceptions are the accounts listing, the base policy's statements, which
  compare the account `IS NULL`, and the operation log, which it requires to be reached through a
  join from its plan column to a plan tied to the transaction's account, and to be appended to from
  such a plan's row ([ADR-0066](./0066-data-access-generated-from-sql.md),
  [db/README.md](../../../db/README.md#the-checks)).
- **No table carries a row-level security policy.** The accounts listing is held by its column
  grants alone, the operation log by the statement check, and the base policy's writes by the
  grants, the statement check and the base policy's own transaction
  ([ADR-0112](./0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)).
- **The transaction helper reads the account back.** A transaction that never set the setting
  reads it as the empty string on a connection that set it before, and the form then matches no row
  and raises nothing, exactly as a policy did. The read-back fails such a transaction before any
  data access ([ADR-0047](./0047-schema-first-data-access.md)).

The form keeps its indexes, measured as above. `account_id = current_setting('app.account')` was an
index condition in custom plans and in forced generic plans, on every index leading with the
account. As an index condition it is computed once per scan, so an index-only count of 100,000 rows
took 3.82 ms with it and 3.91 ms with a parameter. In a forced generic plan it estimated the
account's rows correctly (100,260 against an actual 100,000), where a parameter gets a default
estimate. Only a plan with no index leading on the account pays about 35 ns per row to evaluate it.

## Alternatives considered

- **Keep row-level security, and reconsider it after the third production point.** For it, no work
  now, and the cross-account test of the second account runs against all three layers. Against it,
  every unit until then pays the costs above, and a filter over labels, the subject or a sender
  address can never use an index.
- **Keep row-level security on `account_state` and `audit_log` only.** For it, a database barrier
  stays where the stakes look highest. Against it, it adds nothing measurable, since a credential
  read from another account's row fails to open and the audit log is append-only by grants, and it
  keeps the special cases on those two tables.

## Consequences

- Indexes a filter over labels, the subject or a sender address would use, such as a GIN index on
  labels or a trigram index on the subject, serve every runtime role. Which are added is decided
  by the unit that makes those filters frequent ([ROADMAP.md](../../../ROADMAP.md)).
- A write whose predicate matches no row succeeds having changed nothing, whether the row belongs to
  another account or its state moved on, so every such write keeps counting its rows
  ([ADR-0066](./0066-data-access-generated-from-sql.md)).
- A new account-keyed table needs no policy and no policy test. Its statements are held by the
  statement check like every other.
- `COPY` is not refused by the database, and does not serve the batched writes
  ([ADR-0025](../operability/0025-priority-classes-and-leases.md)).
- Assumptions about other components. Statement mode is refused for a pooler, since
  transaction-local settings need transactions. SQL a component writes by hand outside the
  data-access library is held by the import lists, the UI's raw-SQL analyser and review, with no
  database barrier behind them ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)). A
  reference the check cannot show tied to the transaction's account, such as one tied only under
  `OR`, through a subquery's comparison or through a common table expression's output, is refused
  rather than accepted, and so is any statement of a kind the check does not read and any
  account-keyed table the statement names in a clause the check does not read. SQL a function runs
  from a string is review's.
- The statement check's transaction-form rule, its refusal of an update of an account column and its
  operation-log rule are controls, and so are the grant confining the policy's writes to the UI's
  role and the grants confining the accounts listing to the roles that list accounts. Their
  violation injections are catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
