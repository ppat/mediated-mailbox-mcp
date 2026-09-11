# 0060. No code runs inside the database: no triggers, stored procedures, or user-defined functions

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O2](../../../USE_CASES.md#o2--observable),
[O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

The UI's decisions ([ADR-0021](../mutation/0021-approval-surface.md)) need more than one row
written per decision. A confirmed candidate becomes a policy rule
([ADR-0004](../classification/0004-sender-list-decides.md)), and the design wants the
identity behind every decision recorded. The first UI design met that with database triggers
running as a privileged definer, so that the UI's grant could stay at the two column sets
ADR-0021 named while the trigger wrote the rest. That put logic inside the database, invisible
from the code that caused it, versioned by migration rather than with the code that depends on
it, and firing on every write path whether intended or not. The operator ruled against the
whole class rather than the instance.

## Decision

- **No code runs inside the database.** No triggers, no stored procedures, no user-defined
  functions, in any migration, for any purpose. Every write the system makes is made by
  application code, in a transaction the application opens, closes, and can be read and tested
  as ordinary code.
- **Declarative database features stay.** Constraints, indexes, partitioning, row-level
  security policies, and grants are not code. They express a rule the database checks. They do
  not execute logic the application did not write. Row-level security remains the second
  isolation layer [ADR-0016](../data/0016-schema.md) names, and the per-transaction account
  setting its policies read is set by application code like any other statement.
- **Where a single decision needs several rows, the application writes them in one
  transaction, and tests carry the guarantee** that no path writes one without the others
  ([ADR-0043](./0043-no-mocking.md)'s real-database tests, with every path that writes such a
  decision exercised with a fault injected between its writes and required to leave nothing
  behind).
- **A grant widens rather than a trigger appearing.** When a verb's effect needs a write the
  role does not hold, the role gains that write and the record naming the grant says so
  ([ADR-0021](../mutation/0021-approval-surface.md) for the UI).

## Alternatives considered

- **Definer-owned triggers writing the companion rows.** The case for it was that the audit row
  and the rule row are written by code the low-privilege role cannot alter, atomically with the
  status change, and the role's grant stays literally two column sets. Rejected by the operator
  on 2026-09-10 on the class of problems database-resident code brings. Hidden control flow,
  separate versioning, side effects on every write path, and troubleshooting that starts
  somewhere other than where the symptom shows. In the operator's words, "we can avoid a
  multiple classes of problems and troubleshooting them just by avoiding them all together."
- **Stored procedures as the write API**, so the application calls one procedure per verb. The
  case for it was one place per verb. Rejected for the same reason. It moves the verb's logic out
  of the application and into the database.
- **Forbidding row-level security as well**, for consistency. No case was tabled. Policies are
  declarative and were kept, with the distinction stated in the Decision.

## Consequences

- The UI's decisions are recorded by the columns its verbs set and the rule row its confirm
  verb inserts, all written by the UI's own code in one transaction
  ([ADR-0021](../mutation/0021-approval-surface.md)).
- The migration lint refuses any migration that creates a trigger, procedure, or function. Its
  violation injection is catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- Assumptions about other components: every process that reads account-scoped rows sets the
  account for its transaction before reading, which the row-level security policies of
  [ADR-0016](../data/0016-schema.md) require, and the migration role of
  [ADR-0048](../data/0048-forward-only-migrations.md) owns the schema and has no functions to
  own.
