# 0129. The chart runs the migration step as one Helm hook Job before install and before every upgrade, holding the migration role's credential alone

**Status:** Accepted ·
**Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

[ADR-0048](./0048-forward-only-migrations.md) runs migrations as their own step, under their own
role that owns the DDL, before the deployables roll, whatever the deployment mechanism is. The step
runs the migrator, the image [ADR-0049](../engineering/0049-image-per-component-lockstep.md)
lists, which holds the goose command of [ADR-0067](./0067-migration-runner-goose.md) and the
chain, and reads its connection string from `GOOSE_DBSTRING`. The image is distroless, with no
shell, so a container cannot assemble the string from files at run time.

The chart of [ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md) has to place that
step. Three things constrain it. The migration role's credential must reach only the migrating
container, never a serving pod. Several pods starting together must not run the chain at once.
And a password must not travel in an environment variable
([ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)), so the connection string can
carry the role's name and the server but not its password.

Two facts of the tools shape the answer. A Helm `pre-install` hook runs after rendering and before
any of the release's ordinary objects exist, and a `pre-upgrade` hook before any is updated, so on
upgrade the hook still sees the previous release's objects. And pgx, the driver goose opens
PostgreSQL through, reads a password from the file a `passfile` parameter of the connection string
names, in the libpq password file format, `host:port:database:user:password`.

## Decision

- **The migration step is one Kubernetes Job, annotated as a Helm `pre-install` and `pre-upgrade`
  hook.** Helm waits for it to complete before it creates or updates any deployable, and a failed
  Job fails the release, so no deployable rolls onto a schema the chain did not reach.
- **The Job depends only on Secrets the user supplies and on its own spec.** It reads no ConfigMap,
  Secret or ServiceAccount the chart renders, because none of those exists yet at `pre-install`,
  and at `pre-upgrade` they are still the previous release's. It runs under the namespace's default
  ServiceAccount with no token mounted.
- **The connection string carries no password.** The chart writes `GOOSE_DBSTRING` into the Job's
  spec with the migration role's name, the server, the database, the TLS mode, the CA file when one
  is given, and `passfile` naming a file the Job mounts from a key of the user's Secret holding the
  migration role's password in the libpq password file format. A CloudNativePG cluster writes such
  a key, `pgpass`, into the Secret it generates for the database's owner.
- **The migration role's credential is mounted into the Job's pod and no other**, and the Job
  mounts nothing else secret beyond the database's CA certificate.
- **One run at a time, as far as Kubernetes holds it.** The Job runs one pod to one completion,
  replaces a pod only once it has failed, not while it is still terminating, and retries a failed
  pod enough times to outlast roles created within the release's timeout. Helm deletes the previous
  Job before it creates the next and deletes a Job that succeeded, and leaves a failed one for its
  logs.

## Alternatives considered

- **An init container in each deployable's pod.** The case for it is that it needs no hook and
  works under any renderer, since every pod migrates before it starts. Rejected: it puts the
  migration role's credential into every serving pod, against
  [ADR-0048](./0048-forward-only-migrations.md)'s role split, and pods starting together, a UI with
  replicas beside the API server and the worker, run the chain at once.
- **A plain Job, named by a hash of the release, with the deployables waiting on it.** The case for
  it is that it works under any renderer, since nothing is a hook. Rejected: each deployable would
  need a container that watches the Job, the Kubernetes API access to do it, and a binary the
  images do not carry, and every change leaves another Job behind. The hook gives the ordering
  directly.
- **A `post-install` and `post-upgrade` hook.** The case for it is that the release's own objects
  exist by then, so the Job could read a chart-rendered ConfigMap. Rejected: the new deployables
  start on the old schema before the chain runs, against
  [ADR-0048](./0048-forward-only-migrations.md)'s order.
- **The password in `GOOSE_DBSTRING`, taken from the Secret into the environment.** The case for it
  is that it needs no password file format. Rejected by
  [ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md).

## Consequences

- **Renderers that are not Helm.** Flux's helm-controller runs Helm itself, so the hook behaves as
  above. Argo CD maps both hooks to its `PreSync` phase and cannot tell an install from an upgrade,
  so the Job runs on every sync, which applies nothing when no migration is pending.
- **A concurrent run is rare, not impossible.** A Job that runs one pod and replaces it only after
  it fails still may start the same program twice, as Kubernetes documents, and the goose command
  takes no lock of its own when it applies a chain. The PostgreSQL advisory lock
  [ADR-0067](./0067-migration-runner-goose.md) credits goose with is taken only by goose's library
  form, when a session locker is given to it. A second path is the release's own retry. Helm
  deletes the previous hook Job with background propagation and waits only for the Job object to be
  gone, not for its pod, so when a release times out while the Job still retries and the release is
  run again, the old Job's pod can still be running as the new Job's starts. Two runs at once could
  both apply one migration. A guard against that is a change to the migrator, which would
  take that lock, and is not the chart's.
- **The roles exist before the chain runs, and the chart cannot order that.** A grant to a role not
  yet created fails its migration's transaction, and the Job's retries converge once the platform
  has created the roles. Helm, and Flux running Helm, wait on the hook only up to the release's
  timeout, five minutes unless set otherwise, after which the release fails while the Job retries on
  alone. A platform that creates the roles after the database therefore gives the release a timeout
  longer than that, such as `helm install --timeout` or a HelmRelease's `spec.timeout`, or creates
  the roles first.
- Assumptions about other components. The user's Secret for the migration role holds a key in the
  libpq password file format whose host, port, database and user match the connection string, or a
  `*` in their place. The migrator keeps reading `GOOSE_DBSTRING`, applying the chain with
  its default command, and opening PostgreSQL through pgx.
