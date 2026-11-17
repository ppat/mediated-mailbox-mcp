# 0143. The programs connect as one application user beside the migrator's, and each program's import lists bound what its code can do in the database

**Status:** Accepted (supersedes [ADR-0118](./0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md)) ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

The API server, the UI and the worker connect to the database, and the migrator connects to apply
the migration chain ([ADR-0048](./0048-forward-only-migrations.md)). The worker gives each job kind
a connection pool of its own, so one job kind cannot starve another of connections
([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md)). How many
database users these connect as turns on what a database user can protect here and what each one
costs.

A database user can bound two things.

- What a compromised process can do in the database, when each program has a user of its own. The
  API server and the worker each hold the private key that opens every stored credential, and open
  the credentials to call a provider, so a compromise of either reaches every mailbox directly,
  whatever its database user. The UI holds the same key. A user of its own would keep a
  compromised UI from reading the stored credentials, and would let the database tell which
  program wrote a policy change.
- What a buggy job can do in the database, when each job kind has a user of its own. The code
  bounds that too. Each program's import lists name exactly the data-access subsections it may use
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)), and an analyser refuses a
  statement the code of the API server, the UI or the worker runs outside the data-access library.

Each database user costs something.

- A credential a deployment provisions and delivers, and a section of grants in the migration
  chain.
- A user per job kind needs each job kind's code handed the pool connected as that user. Only the
  code that wires the program can do that, and only a test of that code can hold it. The worker's
  configuration would carry a database user per job kind.
- A user granted fewer columns than another needs its statements placed one directory below their
  table, and those placements shape the data-access library around the users.

The grant check of [ADR-0066](./0066-data-access-generated-from-sql.md) plans every statement an
import list admits under the user that list's program connects as, which proves the user's grants
are enough for them.

## Decision

- **Two database users.**

  | User | Who connects as it | What it holds |
  | --- | --- | --- |
  | The application user | The API server, the UI and every job kind of the worker | The grants the chain makes to its name, below |
  | The migrator's user | The migration step ([ADR-0048](./0048-forward-only-migrations.md), [ADR-0139](./0139-the-migration-runner-is-goose-linked-into-the-migrator.md)) | The schema. The migrator owns what it creates, as whichever user it connects as |

- **The project names neither user.** The deployment platform creates both before the chain runs,
  under names the deployment chooses, and gives each name as configuration to what connects as
  it. The `database.user` of the API server, the UI, the worker and the migrator is a required
  setting with no default
  ([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)).
- **Every grant the chain makes is made to the application user's name.** The chain's grants file
  holds one section, for that one user, and a migration that creates a table grants on it to the
  same name. A grant names its grantee, so the migrator takes the application user's name as a
  required setting and makes the chain's grants to it. How the migrator hands that name to the
  migrations is not yet chosen, and the first change to the chain's grants chooses it.
- **The application user's line.** Its writes are exactly what the data-access library's
  statements write. It holds no DDL right ([ADR-0048](./0048-forward-only-migrations.md)), and no
  update or delete on the audit log or the policy history ([ADR-0016](./0016-schema.md),
  [ADR-0102](../mutation/0102-policy-changes-recorded-in-an-append-only-history.md)). Reads beyond
  what its statements read are acceptable. It reads the sealed credentials and the OAuth clients'
  sealed secrets, since code of its programs opens them. The grant check plans every statement of
  every subsection an import list admits under it.
- **A program's import lists bound what its code can do in the database.** A program's code runs a
  statement only by calling a function of a data-access subsection its exact list admits, each list
  proven by its violation files. The raw SQL analyser refuses any other statement in the code of the
  API server, the UI and the worker
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)). So a buggy job, or a bug in the
  API server or the UI, reaches only the statements its own list admits. One user's grants cannot
  tell programs apart, so a column a record forbids one program to act on is held from that program
  by its import list. The process family's code runs as the application user in the API server,
  the UI and the worker, and as the migrator's user in the migrator. Among it, the Session's
  packages run in the API server and the worker, and so do the connections, the one part of it that
  reads the stored credentials, and the provider library's rate limiter with its statements in
  `db/ratestate/limiter`. The analyser does
  not read that code, so a statement written by hand there is held by review.
- **What each program's subsections may write and read of the policy and the stored credentials.**
  - The data-access subsections the UI's import list admits hold no write outside the UI's list of
    writes, and no read of an account's stored credential. The UI's list of writes is
    [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)'s, and its bound on what a
    UI bug can do rests on this rule.
  - No subsection another program's import list admits writes `policy_rules` or `policy_changes`,
    or reads a stored credential beyond what that program's opener needs. So only the UI's code
    writes the policy.

  Both rules are held by review of every statement in a subsection a program's list admits. A user
  per program would let the grant check hold them, each user being granted only its own program's
  statements.
- **In the UI, only the code that sets up an OAuth client and an account can open a stored
  value.** The import rules below hold it, and they stay in force, in these words or in others of
  the same intent. That is the condition on which the operator accepted the UI's standing
  membership of the trust anchor, below.
  - The opening code, `credential/open`, is admitted in the UI only to
    `ui/internal/setup/clientsecret`. That is the one package that opens an OAuth client's secret,
    by the client's name, and it sits under the setup code, its one user.
  - The setup package itself is refused `credential/open`.
  - The file patterns of the list of the packages holding cryptography that the UI's build may
    link leave out the opener's directory, so that list holds the UI's other non-test code away
    from the public-key code an opening is built on.

  Each of these is proven by its violation file
  ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md),
  [ADR-0071](../engineering/0071-static-enforcement-toolchain.md)).
- **The UI is a standing member of the trust anchor.** Its process can read every stored
  credential through the application user and holds the private key that opens them, so it is a
  member at all times. It runs under the same hardening as the processes that call a provider
  ([ADR-0028](../operability/0028-trust-anchor-hardening.md)).
- **What one application user does not protect.**

  | Unprotected | What holds instead |
  | --- | --- |
  | A compromised UI can read the stored credentials | Nothing in the database. The UI also holds the key that opens them |
  | The database does not tell which program wrote a policy change | That only the UI writes the policy rests on the import lists and the rules on the subsections above, which bind each program's code and not a compromised process |
  | A compromised program reaches everything the application user is granted | A compromised API server or worker already reaches every mailbox through the key it holds |

- **Each job kind has a pool of its own**, connected as the one user, so one job kind cannot
  starve another of connections
  ([ADR-0119](../operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md),
  [ADR-0142](../engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).
- **The configuration of each program that connects names one database user**, the API server's,
  the UI's, the worker's whatever job kinds it runs, and the migrator's
  ([ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)).

## Alternatives considered

- **A user per program, and inside the worker per job kind**, six users beside the migrator's, as
  [ADR-0118](./0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md) decided. The case for it
  is that the database bounds what a compromised process and a buggy job can do there, keeps a
  compromised UI from reading a stored credential, and tells by its grants which program wrote a
  policy change, a row of the policy history being the UI's because only the UI's user can write
  the policy. Not chosen, because six users cost more than they protect.
  - What they protect beyond two users. A bug's reach is held by the exact import lists and the
    raw SQL analyser in the API server, the UI and the worker. A compromised API server or worker
    already reaches every mailbox through the key it holds, so its user bounds only its reach in the
    database. Beyond that bound, what is left is the UI's two protections in the table above.
  - What they cost beyond two users. Five more credentials to provision and deliver, five more
    grants sections, each job kind's code handed the pool connected as its own user, which only a
    test of the wiring can hold, and statements split so that a user can be granted fewer columns.
- **A user per program, one for the whole worker**, three users beside the migrator's. The case for
  it is a bound per process, with one credential each for the API server, the UI and the worker. It
  keeps the UI's two protections, and bounds a compromised API server's or worker's reach in the
  database. Not chosen, by the operator's ruling of two users, which stated the protections given up
  as the table above states them. No case beyond that ruling was weighed against this alternative.
- **The UI with a database user of its own, and every other program sharing one**, two users beside
  the migrator's. The case for it is that a compromised UI cannot read a stored credential, and the
  database tells the UI's policy writes from any other program's. It is what remains if the UI is
  not made a standing member of the trust anchor. Not chosen, by the operator's ruling of two users,
  which stated that a compromised UI could then read the stored credentials with the key it holds.
  The operator accepted the UI's standing membership on the condition this record states for the
  UI's import rules. No case beyond that ruling was weighed against this alternative.
- **A user per credential reach**, one for the job kinds that call a provider and one for the
  heuristics run, an alternative ADR-0118 weighed. The case for it, as ADR-0118 made it, is two
  credentials, with the database holding the heuristics run away from stored credentials. Not
  chosen, because the heuristics run's code is held away from them. Its import list refuses the
  connections' package and the subsections that read a stored credential, and in the worker only
  the connections' package may import the opening code. A second user would add a credential for a
  bound the code already holds.
- **Users grouped by what the job kinds write**, backfill and delta sync sharing one, an alternative
  ADR-0118 weighed. The case for it, as ADR-0118 made it, is one credential fewer than a user per
  job kind. Not chosen, because any grouping keeps credentials and grants sections for a bound on
  bugs that the import lists already hold.
- **Grants of exactly what each program's own statements need**, an alternative ADR-0118 weighed.
  The case for it, as ADR-0118 made it, is the tightest bound on reads. Not chosen. It needs a user
  per program, which the first alternative's reasons rule out, and it narrows how the data-access
  library can be organized and named, splitting a table's statements into a subsection per reader,
  which ADR-0118 also held against it.

## Consequences

- A deployment provisions two database users with their credentials, the application user's and
  the migrator's. It delivers the migrator's to the migration step and the application user's to
  the API server, the UI and the worker, each user's name as a setting beside its password, and
  also gives the migrator the application user's name
  ([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)).
- The grant check proves the application user's grants are enough for every admitted statement
  ([ADR-0066](./0066-data-access-generated-from-sql.md)). It does not tell one program's reach from
  another's, which the import lists do.
- Where a data-access subsection sits, below its table's subsection or folded into it, is the rule
  [ADR-0066](./0066-data-access-generated-from-sql.md) states, so no subsection sits below its
  table only so that one user can be granted fewer columns.
- [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)'s bound on a UI bug rests on
  this record's rule for the UI's subsections, and its bound on a compromised UI is what this record
  states.
- Every statement still reads its account from the transaction, and no table carries a row-level
  security policy, so the one user changes nothing in how accounts are kept apart
  ([ADR-0135](./0135-every-statement-reads-its-account-from-the-transaction.md)).
- Assumptions about other components. The deployment platform creates the two users once per
  cluster, under names it chooses, before the chain runs, and gives the migrator the application
  user's name, to which the chain's grant statements are made
  ([ADR-0048](./0048-forward-only-migrations.md)). The decision holds only while the deployment
  platform gives the migration step the migrator's credential alone, and every other process the
  application user's ([ADR-0129](./0129-the-chart-runs-the-migration-step-as-a-pre-install-and-pre-upgrade-hook-job.md)).
- Four rules here have no automated check. They are that the application user's grants stay within
  its line, stated in the Decision, that the UI's subsections hold no write outside its list and
  no read of an account's stored credential, that no other program's subsections write the policy
  or read a stored credential beyond what its opener needs, and that the process family's code and
  the rate limiter's run no statement written by hand. Their dispositions are in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md), beside the injections of the rules above that a
  test can break.
