# 0115. The chart runs the migration step as one Job that Helm runs to completion before each install's and upgrade's other objects

**Status:** Accepted ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [O6](../../../USE_CASES.md#o6--deployable), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

[ADR-0048](./0048-forward-only-migrations.md) runs the migration chain as its own step, under a role
that owns the schema, and assumes the deployment runs that step before it rolls the deployables.
[ADR-0067](./0067-migration-runner-goose.md) makes the step goose, invoked as a command from the
migration image [ADR-0049](../engineering/0049-image-per-component-lockstep.md) lists, and accepts a
failed step as a failed deployment. The Helm chart of
[ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md) is the deployment that has to
run it. Two constraints bind the shape. The migration role's credential must reach only the
container that migrates, since every secret reaches only what uses it
([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)) and the runtime roles hold no
right to change the schema ([ADR-0075](./0075-one-runtime-role-per-deployable.md)). And several pods
starting together must not run the chain at once.

goose reads its connection string from `GOOSE_DBSTRING` or its arguments and has no option naming a
file to read a password from, and the password may not travel in an environment variable or an
argument ([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)). goose connects to
PostgreSQL through pgx, which reads a password file in libpq's format when a connection string
names one with `passfile`.

## Decision

- **One Job runs the migration step, as a Helm hook on `pre-install` and `pre-upgrade`.** Helm
  creates it before the install's or the upgrade's other objects and waits for it to complete, so
  no deployable rolls before the chain has run, and one container runs the chain per install or
  upgrade. A failed run fails the install or the upgrade, and nothing else rolls.
- **The hook's delete policy is `before-hook-creation`.** The Job of the last run stays until the
  next install or upgrade replaces it, so what it did, its status and its log, can be read after it
  ran.
- **It runs on every install and upgrade.** goose applies only what is pending, so a release that
  adds no migration runs a step that changes nothing.
- **The step holds the migration role's credential and nothing else.** Its pod mounts one Secret,
  the migration role's credential, and the database's CA when the deployment names one, and no key,
  no runtime role's password and no configuration file. No other pod the chart runs mounts the
  migration role's credential.
- **The credential is a password file in libpq's format, mounted as a file.** The connection string
  in `GOOSE_DBSTRING` names the host, port, database, the role `mediated_mailbox_migrate`, the TLS
  mode and the file's path through `passfile`, and no password. The deploying side writes the
  Secret's key as one line such as `*:*:*:mediated_mailbox_migrate:<password>`. Each runtime role's
  Secret holds its password alone, as [ADR-0078](../engineering/0078-configuration-layers-through-an-owned-library.md)'s
  `password_file` reads it, so the migration role's Secret is the one written in another format.

## Alternatives considered

- **An init container in each deployable's pod.** For it, every pod starts only once the chain it
  needs has run, with no ordering to rely on. Against it, every deployable's pod spec would hold the
  schema-owning credential, against ADR-0079's rule that only the deployable using a secret reads it,
  and every pod starting together would race to run the chain, which goose's lock serialises but
  which still runs the chain as often as pods start. Not chosen.
- **A plain Job beside the deployables, with each deployable's pod waiting for it.** For it, no
  dependence on Helm's hooks, so a tool that applies the rendered objects without running hooks
  still orders the step. Against it, the wait needs an init container that watches the Job, which
  needs a service account token and a role to read Jobs, against the hardening that gives no pod a
  token ([ADR-0028](../operability/0028-trust-anchor-hardening.md)), or a check of the schema's
  version against the database, which is a second migration-aware component. Not chosen.
- **A Job the operator runs by hand before each upgrade.** No case was tabled for it beyond its
  simplicity. Rejected, because new versions flow between production points with no manual step
  ([ROADMAP.md](../../../ROADMAP.md)).
- **goose's environment file for the credential.** goose reads variables from a file its `-env`
  flag names, so the Secret could hold `GOOSE_DBSTRING` with the password in it. For it, the whole
  connection string is one supplied file. Against it, the host, database and TLS settings would move
  out of the chart's values into a secret the deploying side writes by hand, and each runtime
  role's connection settings would diverge from the migration step's. Not chosen.
- **The delete policy `hook-succeeded`.** For it, no finished Job stays behind. Against it, a
  successful run leaves nothing to read, and the chainsaw suite could not see that the step ran.
  Not chosen.

## Consequences

- The migration step needs a deployment tool that runs Helm's hooks. Helm does, and so does flux's
  helm-controller, which the chainsaw suite installs the chart with, unless its hooks are switched
  off. A tool that renders the chart and applies the objects without hooks runs the migration Job
  beside the deployables, whose first starts then fail until it ends, and restart.
- A hook is not a release object, so Helm's uninstall leaves the last migration Job in place, and
  removing it is the deploying side's.
- The step's connection string follows the chart's database values, so the migration step and the
  deployables always reach the same database with the same TLS settings.
- Assumptions about other components. The deploying side creates the migration role's Secret in
  libpq's password-file format, and the bootstrap, the roles and the database owned by the
  migration role before the first install ([ADR-0048](./0048-forward-only-migrations.md)). goose
  connects through pgx, which reads the password file `passfile` names. A change of runner keeps
  the same file only while the new runner reads it, a cost ADR-0067's "changing the runner is
  changing a command" now carries.
- The rules above are controls, the step running before the deployables and its credential's reach.
  Their injections are catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
