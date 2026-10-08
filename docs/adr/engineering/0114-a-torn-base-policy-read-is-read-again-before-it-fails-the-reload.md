# 0114. A policy reload whose accounts read different base rules reads them all once more, and fails only if they still disagree

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [O2](../../../USE_CASES.md#o2--observable)

## Context

The policy loader reads each account's rules and the base rules together, one account's
transaction at a time, and refuses to compose accounts from different base policies, so a reload
whose accounts read different base rules is a read it cannot trust
([executioncontext/README.md](../../../executioncontext/README.md#the-policy-loader-policyload),
[ADR-0041](./0041-policy-as-immutable-snapshots.md)). A reload that fails raises the reload-failure
alarm, which pages at once and clears when a reload next succeeds
([ADR-0077](../operability/0077-conditions-raised-as-alerting-rules.md)).

The UI writes base rules
([ADR-0112](../data/0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)). An
edit to the base policy landing between two accounts' reads would fail that reload and page, though
nothing is wrong and the next reload succeeds. Whether that page stands is this record's to decide.

## Decision

- **A reload whose accounts read different base rules reads every account again, once, at once.**
  If the second read's accounts agree, its rows are the policy and the reload goes on to validate
  them. If they disagree again, the reload fails as a read it cannot trust, and the alarm fires as
  for any failed reload.
- **The second read is the last.** A reload makes at most two reads, so a base policy edited
  without pause fails the reload rather than holding it in a loop.

## Alternatives considered

- **Accept the page.** For it, no change, and the alarm clears at the next reload. Against it, a
  page the operator learns to ignore teaches them to ignore the alarm that says an edit never took
  effect.
- **The alerting rule waits before it pages.** For it, the loader stays as it is. Against it, every
  real reload failure then pages late too, and ADR-0041 requires the failure to be loud and
  immediate.
- **Read the base rules once, in a base-policy transaction, and each account's own rules in its
  own.** For it, nothing to compare. Against it, moving a rule between scopes adds it to one scope
  and then lifts it from the other ([docs/UI.md section 8.7](../../UI.md#87-policy)), and reading the
  two scopes at different instants can see the lift without the add, a policy restricting less than
  either state the operator left.
- **Every account's read shares one exported snapshot of the database.** For it, every read sees one
  instant. Against it, each reload would hold a transaction open to export the snapshot and import it
  into one transaction per account, which the pooled connections the loader is given do not provide.

## Consequences

- An edit to the base policy landing between two accounts' reads does not page. Two edits landing
  inside one reload's two reads still fail it, which is rare and clears at the next reload.
- A reload that reads again takes up to twice as long, and the reload-failure series reads only its
  outcome.
- The re-read is a control. Its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
