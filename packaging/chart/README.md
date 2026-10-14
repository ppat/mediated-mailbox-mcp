# mediated-mailbox chart

Stands up the mediated mailbox system on Kubernetes: the migration step, the API server, the worker
and the UI ([ADR-0052](https://github.com/ppat/mediated-mailbox-mcp/blob/main/docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)). The chart
deploys these workloads and nothing else. PostgreSQL, the key pair stored credentials are sealed to,
and each database role's password are inputs you supply. Accounts and policy are set up through the
UI once the system runs.

`values.yaml` documents every value, and `values.schema.json` refuses a value the chart or a process
would not read.

## What runs

| Object | Runs | Reads |
| --- | --- | --- |
| Job `<release>-mediated-mailbox-migration` | The migration chain, as a Helm hook before install and before every upgrade ([ADR-0129](https://github.com/ppat/mediated-mailbox-mcp/blob/main/docs/adr/data/0129-the-chart-runs-the-migration-step-as-a-pre-install-and-pre-upgrade-hook-job.md)) | The migration role's password file, the CA certificate |
| Deployment `<release>-mediated-mailbox-api` | The client surface agents call, over MCP and HTTP | Its role's password, the key pair, the bearer token, the CA certificate, its TLS pair |
| Deployment `<release>-mediated-mailbox-worker` | Every background job kind, as exactly one pod | Each job kind's role password, the key pair, the CA certificate |
| Deployment `<release>-mediated-mailbox-ui` | The operator's UI | Its role's password, the key pair, the request token's key, the CA certificate, its TLS pair |
| ConfigMap `<release>-mediated-mailbox-config` | One configuration file per process | |
| Secret `<release>-mediated-mailbox-access-credentials` | Whichever of the API server's bearer token and the UI's request token key you name no Secret for | |

Each pod mounts only the secret keys its process reads, and runs under the restricted Pod Security
Standard with a read-only root filesystem and no ServiceAccount token.

## Before you install

1. **PostgreSQL** with the roles the migration chain grants to, and a database owned by the migration
   role ([ADR-0048](https://github.com/ppat/mediated-mailbox-mcp/blob/main/docs/adr/data/0048-forward-only-migrations.md)). The roles are those of
   `db/bootstrap/roles.sql` in this repository: `mediated_mailbox_migrator` owns the database, and
   `mediated_mailbox_api`, `mediated_mailbox_ui`, `mediated_mailbox_backfill`,
   `mediated_mailbox_sync`, `mediated_mailbox_organize` and `mediated_mailbox_propose` are the runtime
   roles. The migration fails until every runtime role exists, and its Job retries. Helm, and Flux
   running Helm, wait on the migration only up to the release's timeout, five minutes unless set
   otherwise, so create the roles before installing, or give `helm install --timeout` or the
   HelmRelease's `spec.timeout` enough time for them to appear.
2. **A Secret per role password**, and **the key pair** stored credentials are sealed to, as the
   next section lists. Write the key pair with the `mediated-mailbox-keygen` binary attached to each
   release, and keep a copy elsewhere: losing the private key loses every stored credential.

   ```sh
   mediated-mailbox-keygen -private-key-file credential.key -public-key-file credential.pub
   kubectl create secret generic sealing-keys \
     --from-file=credential.key --from-file=credential.pub
   ```

## The Secrets the chart reads

Every Secret the chart reads is an entry under `secrets`, named for what it holds.
`global.imagePullSecrets`, and the `tls` entries of `api.ingress` and `ui.ingress`, each
`{secretName, hosts}`, keep Kubernetes' own shape outside `secrets`, because the chart never reads
those Secrets and only passes them through to kubelet and the ingress controller. An entry's
`existingSecret` names a Secret in the release's namespace, and its `secretKeys` name the key in it
that holds each value the chart reads. The Secret can come from anything, such as external-secrets,
sealed-secrets or `kubectl create secret`. The chart never creates or changes a Secret it is given,
reads only the keys it names, and mounts each one only into the pods whose process reads it.

| Entry under `secrets` | Holds, under the key named, with its default | Mounted into | Required |
| --- | --- | --- | --- |
| `databasePasswords.api` | The API server role's password alone, `secretKeys.password: password` | the API server | yes |
| `databasePasswords.ui` | The UI role's password alone, `secretKeys.password: password` | the UI | yes |
| `databasePasswords.worker.backfill`, `databasePasswords.worker.sync` | Each job kind's role password alone, `secretKeys.password: password` | the worker | yes |
| `databasePasswords.migration` | The migration role's password as a libpq password file, `host:port:database:user:password`, `secretKeys.passwordFile: pgpass`. Its host, port, database and user match `database.host`, `database.port`, `database.name` and `migration.database.user`, or are `*` | the migration Job | yes |
| `credentialSealingKeyPair` | The key pair stored credentials are sealed to: the public key, `secretKeys.publicKey: credential.pub`, and every private key of the keyring, `secretKeys.privateKeys: [credential.key]` | the API server, the worker and the UI | yes |
| `databaseCaCertificate` | The CA certificate the database server's certificate is checked against, `secretKeys.certificate: ca.crt` | every process and the migration Job | no |
| `apiBearerToken` | The bearer token clients present to the API server, `secretKeys.bearerToken: api-bearer-token` | the API server | no, the chart generates one when empty |
| `uiRequestTokenKey` | The key the UI signs its request tokens with, at least 32 bytes, `secretKeys.requestTokenKey: ui-request-token-key` | the UI | no, the chart generates one when empty |
| `apiTls`, `uiTls` | A certificate chain, `secretKeys.certificate: tls.crt`, and its private key, `secretKeys.privateKey: tls.key`, such as a `kubernetes.io/tls` Secret holds | the API server, the UI | no, plain HTTP when empty |

## With a CloudNativePG cluster

CloudNativePG creates the database owned by the migration role and writes that role's password, as a
libpq password file under `pgpass`, into the Secret `<cluster>-app`, and the server's CA certificate
into `<cluster>-ca`. The chart mounts only `ca.crt` from that Secret, so its CA private key stays out
of every pod.

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: api-db
  labels:
    cnpg.io/reload: "true"
type: kubernetes.io/basic-auth
stringData:
  username: mediated_mailbox_api
  password: <a password>
# ...and likewise ui-db, backfill-db and sync-db for mediated_mailbox_ui,
# mediated_mailbox_backfill and mediated_mailbox_sync.
---
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: mailbox-db
spec:
  instances: 1
  storage:
    size: 10Gi
  bootstrap:
    initdb:
      database: mediated_mailbox
      owner: mediated_mailbox_migrator
  managed:
    roles:
    - name: mediated_mailbox_api
      login: true
      passwordSecret:
        name: api-db
    - name: mediated_mailbox_ui
      login: true
      passwordSecret:
        name: ui-db
    - name: mediated_mailbox_backfill
      login: true
      passwordSecret:
        name: backfill-db
    - name: mediated_mailbox_sync
      login: true
      passwordSecret:
        name: sync-db
    - name: mediated_mailbox_organize
      login: true
    - name: mediated_mailbox_propose
      login: true
```

Wait for the cluster to report Ready before installing, as the chainsaw suite does, so the
migration's wait for the roles stays well inside the release's timeout:

```sh
kubectl wait cluster/mailbox-db --for=condition=Ready --timeout=10m
```

Then install the chart in the same namespace with these values:

```yaml
database:
  host: mailbox-db-rw
  name: mediated_mailbox
secrets:
  databasePasswords:
    api:
      existingSecret: api-db
    ui:
      existingSecret: ui-db
    worker:
      backfill:
        existingSecret: backfill-db
      sync:
        existingSecret: sync-db
    migration:
      existingSecret: mailbox-db-app
      secretKeys:
        passwordFile: pgpass
  databaseCaCertificate:
    existingSecret: mailbox-db-ca
  credentialSealingKeyPair:
    existingSecret: sealing-keys
```

```sh
helm install mailbox oci://ghcr.io/ppat/mediated-mailbox --version <version> --values values.yaml
```

`database.host` is the cluster's read-write Service, `<cluster>-rw`, which the server's certificate
names and the `pgpass` file's host field holds, so the default `sslmode: verify-full` checks the
server. The chainsaw suite in `packaging/tests/chainsaw/cnpg` installs exactly this.

## With any other PostgreSQL

Create the roles and the database as `db/bootstrap/roles.sql` and the list above describe, give each
role a password, and store each in its Secret. Set `database.sslmode` to what the server offers, and
`secrets.databaseCaCertificate` when the server's certificate is checked against a CA of your own. To the chart a
CloudNativePG database is no different: it takes a host, password files and a CA certificate from any
server, which is why the chainsaw suite tests with CloudNativePG alone.

## Values you will most often set

| Value | Holds |
| --- | --- |
| `sharedConfig.log_level`, `sharedConfig.sync_interval`, `sharedConfig.scanner` | Configuration several processes read, given once and rendered into each |
| `api.config`, `ui.config`, `worker.config`, `worker.jobKinds.<kind>.config` | Each process's own configuration keys, named as the process reads them |
| `secrets.apiTls`, `secrets.uiTls` | A Secret holding a certificate chain and its key, such as a `kubernetes.io/tls` Secret. Named, the process serves TLS, and empty, plain HTTP ([ADR-0127](https://github.com/ppat/mediated-mailbox-mcp/blob/main/docs/adr/operability/0127-tls-when-given-a-key-pair-plain-http-otherwise.md)) |
| `secrets.apiBearerToken`, `secrets.uiRequestTokenKey` | Your own Secrets holding the bearer token and the request token key, as [The Secrets the chart reads](#the-secrets-the-chart-reads) lists |
| `api.ingress`, `api.httpRoute`, `ui.ingress`, `ui.httpRoute` | Exposure outside the cluster, off by default |
| `monitoring` | The Prometheus Operator's resources and the Grafana dashboard |
| `*.resources` | Requests are set, and no limit is, so set limits to suit your nodes. |
| `*.goSoftMemoryLimit` | The Go runtime's soft memory limit for that container, its `GOMEMLIMIT`, unset by default. Go sets none by itself, so when you set a memory limit, set this below it, written in Go's units such as `460MiB`, not Kubernetes' `Mi`. Lower it with the limit, since an in-place resize changes only the limit |
| `api.database.poolSize`, `ui.database.poolSize`, `worker.jobKinds.<kind>.database.poolSize` | The most connections each pool opens. Their sum across replicas is what PostgreSQL's connection limit must allow. Unset, the API server and the UI open up to the larger of 4 and the node's processor count, and each of the worker's job kinds up to 4 |

### Quoting

Helm reads values with a YAML 1.1 parser, which turns an unquoted `no`, `yes`, `on`, `off`, `y` or
`n` into a boolean, and the same word as a map key into `false` or `true`, before the chart sees it.
The values schema refuses such a value rather than passing it on changed, so quote these words
wherever they are meant as text, such as a trigger word or the Norwegian language code:

```yaml
sharedConfig:
  scanner:
    triggers:
      "no": [kode, engangskode]
```

### The bearer token and the request token key

For whichever of `secrets.apiBearerToken` and `secrets.uiRequestTokenKey` names no Secret, the
chart generates the value into its own Secret, `<release>-mediated-mailbox-access-credentials`,
keeps that Secret when the release is uninstalled, and reads each value back on every later install and upgrade, so the
values stay the same. Read the bearer token agents present with:

```sh
kubectl get secret <release>-mediated-mailbox-access-credentials \
  --output jsonpath='{.data.api-bearer-token}' | base64 --decode
```

A renderer that cannot read the cluster, such as `helm template` or Argo CD, would generate new
values on every render. Under one, supply both Secrets by any means, such as external-secrets, and
name them in `secrets.apiBearerToken` and `secrets.uiRequestTokenKey`.

## Exposure

Nothing is exposed outside the cluster until you enable an Ingress or an HTTPRoute, for the API
server or the UI or both. Their annotations pass through untouched, so a controller's or
cert-manager's go there. An HTTPRoute's TLS terminates on its Gateway's listener.

The API server serves `/api` and `/mcp`, and the UI serves everything under `/ui` and answers `/`
with a redirect to `/ui/`, so the two can share one host name routed by path. The HTTPRoutes'
default rules route them that way, the UI's matching `/ui` and exactly `/`, and the API server's
matching every other path. An Ingress takes its paths from `hosts`, so on one host name give the UI
`/ui` as `Prefix` and `/` as `Exact`, and the API server `/` as `Prefix`. On a host name of its own
each takes `/`.

**The UI has no login of its own.** Expose it only behind an authenticating proxy, and set
`ui.config.identity_header` to the header the proxy forwards the operator's identity in.

## Monitoring

With the Prometheus Operator installed, the chart renders a ServiceMonitor per deployable and a
PrometheusRule holding the alerting rules of
[ADR-0077](https://github.com/ppat/mediated-mailbox-mcp/blob/main/docs/adr/operability/0077-conditions-raised-as-alerting-rules.md), which are
`rules/alerting-rules.yaml` in this chart. `monitoring.serviceMonitor.enabled` and
`monitoring.prometheusRule.enabled` are `auto` by default, which renders each where the cluster
serves its kind, and `true` or `false` overrides that. `helm template` sees no cluster, so pass it
`--api-versions monitoring.coreos.com/v1/ServiceMonitor` and
`--api-versions monitoring.coreos.com/v1/PrometheusRule`, or set the values to `true`.

`monitoring.dashboard.enabled` renders a Grafana dashboard as a ConfigMap labelled
`grafana_dashboard: "1"`, which Grafana's dashboard sidecar picks up. Set
`monitoring.dashboard.labels` and `monitoring.dashboard.annotations` to what your sidecar selects
on and reads, such as its folder annotation.

## Upgrades and the worker

Every upgrade runs the migration step first, and a failed migration stops the upgrade before any
deployable rolls. The migrations are forward-only, so a rollback to an older release runs older
binaries against the newer schema. Under Flux, set the release's upgrade remediation to retry rather
than roll back.

The worker runs as exactly one pod, and its rollout stops the old pod before it starts the new one.
Outside a rollout Kubernetes does not hold that guarantee. Any deletion of the pod, an eviction for
a node drain, a scale-down by a cluster autoscaler, a preemption or a deletion by hand, has the
replacement created at once, so two workers can run together for up to the termination grace
period, `worker.terminationGracePeriodSeconds`, and a lost node's pod can keep running until its
node stops it. The worker's runs tolerate that overlap, since each reads recorded state and is
idempotent or conditional.

## Tests

`helm test <release>` runs a pod that checks through each Service that the API server and the UI
report ready, the worker reports healthy, the API server refuses a request without the bearer token,
and the UI serves its root page at `/ui/`. The redirect at `/` is proven by the UI's own test, since
a pull request's suite runs this test against `main`'s images.
