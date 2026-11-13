# 0143. The API server, the UI and the worker connect to the database as one application user, and the migrator as a user of its own

**Status:** Proposed (supersedes
[ADR-0118](./0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md), and in part
[ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md) and
[ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)) ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

[ADR-0118](./0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md) gives the API server, the
UI, and inside the worker each job kind, a runtime role of its own, six roles beside the migration
role of [ADR-0048](./0048-forward-only-migrations.md). Between processes a role bounds what a
compromised process can do in the database, and inside the worker it bounds what a buggy job can
do. The UI's role reads no stored credential and writes exactly what
[ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md) lists, so a UI bug cannot
become a mailbox mutation. [ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)
rests on that role. A compromised UI process holds the private key that opens every stored refresh
token, but its role never reads a stored credential, so the key yields one only with a credential's
bytes taken from somewhere else.

The six users cost more than they protect. Each needs a credential provisioned and delivered to the
process that connects as it. The worker opens a pool per job kind. The data-access library holds
three packages that exist only so one user could be granted fewer columns,
`db/accountstate/authentication`, `db/jobruns/classification` and `db/maskingevents/senders`.

The import lists hold which code reaches which statements and which packages, by component and by
package ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)). The operator ruled that
their value is not in protecting against a compromised process, but "in how it shapes and
influences design and organization. and preventing unintentional braiding of things that shouldn't
be braided by providing structural safety and structural design lock".

## Decision

- **The database has two users.** The application's, which the API server, the UI and the worker
  connect as, and the migrator's, which runs the migration chain
  ([ADR-0048](./0048-forward-only-migrations.md),
  [ADR-0139](./0139-the-migration-runner-is-goose-linked-into-the-migrator.md)). The application's
  grants are one section of the migration chain, and the grant check plans every statement the
  programs run under the application's user
  ([ADR-0066](./0066-data-access-generated-from-sql.md)).
- **Each program connects through one pool.** The worker's pool serves every job. Its pool per job
  kind had two reasons. Each job kind connected as its own user, which is gone, and one job must not
  starve another of connections
  ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)), which is
  met by each job's limit on its runs and a pool no smaller than those limits together with what
  the Session's reload and the rate limiter's leases use. The worker's configuration check refuses
  a pool smaller than its jobs' concurrency together, and the worker's database section loses its
  block per job kind.
- **What a grant fenced, an import list fences.** Which code runs which statement is held by each
  package's import list of the data-access library's subsections, so the fence on the UI's writes
  is the one import list that admits them. The raw-SQL analyser covers the API server's and the
  worker's code as well as the UI's
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)). Between programs and between
  the worker's job kinds, what code can do in the database is held by what it links, not by a
  grant.
- **A data-access package one directory below a table's stays where it names an owner**, one area
  of code that alone runs those statements, and folds into its table's package where it existed
  only so one user could be granted fewer columns. The three that fold are
  `db/accountstate/authentication`, `db/jobruns/classification` and `db/maskingevents/senders`.
- **Two protections are given up, stated plainly.**

  | Protection | What held it | Under one application user |
  | --- | --- | --- |
  | A compromised UI process cannot read a stored credential, so the private key it holds yields a refresh token only with bytes taken from elsewhere | The UI's role, whose read never covers `credential` ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md), [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)) | A compromised UI process reads stored credentials and opens them with the key it holds |
  | Telling which process wrote a policy change | Only the UI's role may write `policy_rules` and insert into `policy_changes` | Every program's code runs under the same grants, so a policy row no longer shows by its grant that the UI wrote it |

  The operator ruled the two users on 12 November 2026, with this cost stated.
- **The UI is a standing member of the trust anchor.** Under one application user the UI's process
  can read every stored credential and holds the key that opens them, so it is inside the
  [trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) for as
  long as it runs, not only for the moment it completes a consent. The operator approved this on 14
  November 2026 on one condition, which this decision holds.
  - The import rules in play stay as they are, or equivalent in intent.
  - In the UI, a stored value is opened only in the smallest possible area of code, the code
    responsible for "setting up and configuring a google oauth", in the operator's words. That is
    the one package that opens a client's secret by its name, today `ui/internal/clientsecret`,
    which stays a package of its own with its own import list, the only part of the UI's code
    admitted the opening half of the credential code ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)).

## Alternatives considered

- **A runtime role per program and per job kind, as
  [ADR-0118](./0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md) decides.** The case for
  it is the bounds it gives. A compromised process reaches only its own roles, a buggy job reaches
  only its own role's grants, and the two protections above hold. It loses because the six users
  cost more than they protect. They need a credential each, a pool per job kind and packages of the
  data-access library that exist only to narrow a grant, while the import lists already hold which
  code runs which statement.
- No other alternative was tabled.

## Consequences

- This record supersedes these records and statements.
  - [ADR-0118](./0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md).
  - [ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)'s statement that a job
    kind that calls no provider is held away from a credential by its role holding no grant on a
    sealed credential and by its entry constructor taking only what the job needs, and its cost
    bullet's statement that the UI's database role still never reads a stored credential, so the
    key yields a refresh token only with a credential's bytes taken from somewhere else.
  - [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)'s separate database role
    for the UI, whose read of `account_state` never covers `credential` and whose grant is exactly
    the list it gives and nothing else, and its statement that the limit is enforced by database
    permissions, so a UI bug cannot become a mailbox mutation.
- A job kind that calls no provider, the heuristics run, keeps one of its three fences against
  reaching a credential, its import list, which admits no code that opens a credential. Its role's
  grants go with this record, and its entry constructor goes with [ADR-0142](../engineering/0142-a-programs-spec-lists-its-pieces-and-the-process-builds-and-runs-them.md).
- [ADR-0117](../operability/0117-one-background-worker-runs-every-job-kind.md)'s role per job kind
  and its fence of a job kind's own role's grants no longer hold. Its import list per job kind, no
  cgo and no `unsafe` in what is added stay the fences of code isolation per job kind.
- A deployment provisions two database credentials. The API server, the UI and the worker receive
  the application's, and the migration step alone receives the migrator's
  ([ADR-0129](./0129-the-chart-runs-the-migration-step-as-a-pre-install-and-pre-upgrade-hook-job.md)).
- Assumptions about other components. The import lists and the raw-SQL analyser hold which code
  runs which statement, and what the analyser leaves to review is
  [ADR-0071](../engineering/0071-static-enforcement-toolchain.md)'s. The platform delivers the
  application's credential to the API server, the UI and the worker, and the migrator's to the
  migration step alone. The UI receives the private key's file, as it does under
  [ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md).
- Each rule above that a test can break is a control, with its violation injection in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
