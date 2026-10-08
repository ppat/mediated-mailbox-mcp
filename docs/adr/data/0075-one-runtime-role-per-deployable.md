# 0075. Each deployable connects to the database as a runtime role of its own

**Status:** Accepted ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

[ADR-0048](./0048-forward-only-migrations.md) gives the migration step a role of its own that owns
the schema. [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md) gives the UI a
role of its own, read-only on most tables, whose write grant is the decision columns of reorg plans
and of policy candidates, insert on policy rules, the columns OAuth client setup and account setup
write, the writes on policy rules that policy management makes, and insert on the policy history.
No runtime role may update or delete an audit row ([ADR-0016](./0016-schema.md)).
The other five deployables, the mediator, backfill, delta sync, the reorganization workload and
the heuristics job, take their roles from this record. The grant check of
[ADR-0066](./0066-data-access-generated-from-sql.md) plans every statement of a subsection under the
role of each component whose import list admits it, and which role each component connects as is
held in one place in `db/check`, so no component's list can admit a subsection until that mapping
exists.

The deployables read and write different parts of the schema, and the records that decide each
deployable's work say which parts ([decision-record index](../README.md)). The
[pillar](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) this record
implements takes the stance that the anchor "is hardened, its blast radius is understood, and
evidence of its compromise survives outside its own reach."

## Decision

- **Every deployable connects as a runtime role of its own.** That is six runtime roles, for
  `mediate`, `backfill`, `sync`, `organize`, `propose` and `ui`, beside the migration role.
- **Each role is named `mediated_mailbox_<directory>`**, after the migration role
  `mediated_mailbox_migrate`, with the deployable's directory as the last word.
- **Each role's grants hold three lines.** Its writes are exactly what its own deployable's
  statements write. It holds no grant on a sealed credential or an OAuth client secret unless its
  deployable opens them. It holds no grant on a column a record forbids its deployable to act on,
  which is the stored sender class for the mediator
  ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)). Reads beyond those lines are
  acceptable. What a compromised deployable can do in the database is bounded by its own role's
  grants.

## Alternatives considered

- **Group the roles by what the deployables write.** Five roles, with backfill and delta sync
  sharing one, since they write the same tables, and the others each keeping their own. The case
  for it is one credential fewer to provision. A compromise of delta sync would reach everything
  backfill can write, which it mostly reaches already, and the shared role would need a name of its
  own. Not chosen, in favour of a role for each deployable held to the three lines.
- **One role the UI does not share, and one every other deployable shares.** The case for it is the
  simplest provisioning, two runtime roles. Any compromised deployable could write every table the
  others can, which weakens the understood blast radius the pillar asks for, and the grant check
  would separate nothing among them. Not chosen, for the same reason.
- **Grants of exactly what each deployable's own statements need.** For it, the tightest bound on
  reads. Against it, it narrows how the data-access library can be organized and named, splitting a
  table's statements into a subsection per reader, for little benefit in the risk it gates. Not
  chosen, in favour of the three lines.

## Consequences

- A deployment provisions six runtime credentials beside the migration role's.
- A new deployable brings a new role with grants for its own statements.
- A shared library, such as the rate limiter, runs its statements under the role of each
  deployable that uses it, so each of those roles holds the grants the library's statements need.
- [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md) states the UI's grant. It
  gives the UI reads on most tables, leaving which ones to what its screens read in
  [docs/UI.md](../../UI.md), and a write grant of exactly the columns its two decision verbs set,
  the row a confirmation inserts, the columns OAuth client setup and account setup write, the
  policy writes and the policy history's insert
  ([ADR-0102](../mutation/0102-policy-changes-recorded-in-an-append-only-history.md)). Those grants
  are what the UI's statements need, so the UI's role holds them from the schema's first grants.
- A deployable that calls a provider reads its accounts, their sealed credentials and the
  installation's OAuth clients, and updates the credential column of an account's state row when it
  writes back a rotation ([ADR-0082](../operability/0082-rotation-writeback-to-the-database.md)).
  Delta sync's role also updates each OAuth client's sealed secret, because delta sync re-seals it
  when a key is replaced ([ADR-0092](../operability/0092-key-replacement-by-keyring-and-re-seal.md)).
- A role whose statements touch the operation log also reads the plan columns that
  [ADR-0016](./0016-schema.md)'s policy on the log looks up, because PostgreSQL runs a policy's
  lookup with the querying role's privileges. Those columns are part of what its statements need.
- Assumptions about other components. The superuser bootstrap creates the roles once per cluster
  before the chain runs, and the chain's grant statements name them
  ([ADR-0048](./0048-forward-only-migrations.md)). The grant check shows a role's grants are enough
  for the statements its component's list admits
  ([ADR-0066](./0066-data-access-generated-from-sql.md)). The decision holds only while the
  deployment platform gives each deployable the credential of its own role.
- That a role's grants stay within the three lines has no automated check. Its disposition is in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
