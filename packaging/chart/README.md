# mediated-mailbox Helm chart

The chart stands up the mediated mailbox system on Kubernetes: the migration step, the mediator,
backfill, delta sync and the UI. Why it is shaped as it is lives in the decision records it cites,
[ADR-0052](../../docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md) above all. This file
is how to install it.

## Prerequisites

- Kubernetes, with nothing beyond its core resources. The Ingress, the Gateway API HTTPRoute and the
  Prometheus Operator's PrometheusRule are each rendered only when a value switches them on.
- Helm 3.8 or later, which pulls charts from an OCI registry.
- PostgreSQL 18 with the `citext`, `pg_trgm` and `vector` extensions available, and a superuser
  bootstrap the chart does not run
  ([ADR-0048](../../docs/adr/data/0048-forward-only-migrations.md)):
  - the migration role `mediated_mailbox_migrate`, which owns the application database and runs the
    migration chain;
  - one runtime role per deployable, `mediated_mailbox_mediate`, `mediated_mailbox_backfill`,
    `mediated_mailbox_sync`, `mediated_mailbox_ui`, `mediated_mailbox_organize` and
    `mediated_mailbox_propose`
    ([ADR-0075](../../docs/adr/data/0075-one-runtime-role-per-deployable.md)), each able to log in.
    The migration chain grants to every one of them by name, so all six must exist, even those of
    deployables this chart does not run yet;
  - the three extensions created in the application database, since only a superuser may create
    `vector`.

  [`db/bootstrap/`](../../db/bootstrap) holds the bootstrap as SQL.
- The Secrets the chart mounts, which it never creates
  ([ADR-0079](../../docs/adr/operability/0079-secrets-arrive-as-mounted-files.md)):

  | Secret | Holds | Value naming it |
  | --- | --- | --- |
  | The migration role's credential | A password file in libpq's format, one line `*:*:*:mediated_mailbox_migrate:<password>` | `migrate.passwordFileSecret` |
  | Each runtime role's password | The password alone, under a key such as `password` | `<deployable>.passwordSecret` |
  | The key pair credentials are sealed to | The public key and every private key, from the key-generation command | `keys` |
  | The mediator's bearer token | The token clients present | `mediate.tokenSecret` |
  | TLS for the mediator and the UI | A `kubernetes.io/tls` Secret each | `mediate.tls.secretName`, `ui.tls.secretName` |
  | The database's CA, when TLS is verified | The CA certificate | `database.caSecret` |

The key pair comes from the key-generation command, `credential/cmd/keygen`, attached to each
release as `mediated-mailbox-keygen-<os>-<arch>`
([ADR-0088](../../docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)):

```bash
mediated-mailbox-keygen -private-key-file credential.key -public-key-file credential.pub
kubectl create secret generic sealing-keys \
  --from-file=credential.key --from-file=credential.pub
```

## Installation

```bash
helm install mediated-mailbox oci://ghcr.io/ppat/mediated-mailbox --version <version> \
  --namespace mailbox --values my-values.yaml
```

Helm runs the migration step before anything else, then starts the deployables. Once the UI is up,
set up an OAuth client, connect an account and import the policy through it, then start backfill's
first run, which the chart's notes print:

```bash
kubectl --namespace mailbox create job --from=cronjob/mediated-mailbox-backfill backfill-first-run
```

Every later release, and every change to the scanner's section, runs backfill again with no manual
step
([ADR-0116](../../docs/adr/operability/0116-the-chart-runs-backfill-whenever-its-pod-changes-and-the-operator-starts-it-from-a-suspended-cronjob.md)).
A deployment tool that waits for Jobs to finish, such as flux's helm-controller unless its release
sets `disableWaitForJobs`, would hold an upgrade for backfill's whole run.

## Values

[`values.yaml`](./values.yaml) documents every value beside its default, and
[`values.schema.json`](./values.schema.json) refuses a value of the wrong type or a key the chart
does not know. The groups:

| Values | What they set |
| --- | --- |
| `nameOverride`, `fullnameOverride`, `imagePullSecrets` | Names and pull secrets for every object |
| `database` | The PostgreSQL host, port, database name, TLS mode and CA Secret, shared by every deployable and the migration step |
| `keys` | The key-pair Secret and the keys in it |
| `scanner` | The scanner's section, rendered into the mediator's, backfill's and delta sync's files alike ([ADR-0096](../../docs/adr/redaction/0096-a-scanner-change-reopens-backfill.md)) |
| `podSecurityContext`, `securityContext` | The hardening every pod and container gets ([ADR-0028](../../docs/adr/operability/0028-trust-anchor-hardening.md)) |
| `migrate`, `mediate`, `backfill`, `sync`, `ui`, `tests` | Per component: its image, its Secrets, its own `config` keys, its service account, pod annotations and labels, security contexts merged over the shared ones, resources, node selector, tolerations and affinity |
| `mediate.service`, `ui.service` and their `ingress` and `httpRoute` | How the client surface and the UI are reached. The Ingress and the HTTPRoute are off by default |
| `alertingRules.enabled` | The alerting rules as a PrometheusRule ([ADR-0077](../../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)) |

Each deployable reads one configuration file, which the chart renders from these values into one
ConfigMap and mounts into that deployable's pods alone
([ADR-0078](../../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)). A
deployable's `config` takes only the keys it declares, which `values.yaml` lists. Write a word YAML
reads as a boolean, such as `no`, in quotes.

## Example: PostgreSQL from CloudNativePG

A [CloudNativePG](https://cloudnative-pg.io/) cluster can provide the database, its roles and its
extensions. This example runs in the namespace `mailbox` with CloudNativePG 1.30 installed. It
uses CloudNativePG's `standard` PostgreSQL image, which ships `pgvector`.

Generate a password per role, and create a `kubernetes.io/basic-auth` Secret for each, the form
CloudNativePG reads a role's password from. The chart reads the same Secrets' `password` key. The
migration role's password goes into a second Secret as a password file in libpq's format:

```bash
for role in migrate mediate backfill sync ui organize propose; do
  kubectl --namespace mailbox create secret generic "mailbox-db-${role}" \
    --type kubernetes.io/basic-auth \
    --from-literal=username="mediated_mailbox_${role}" \
    --from-literal=password="$(openssl rand -hex 24)"
done
password="$(kubectl --namespace mailbox get secret mailbox-db-migrate -o jsonpath='{.data.password}' | base64 -d)"
kubectl --namespace mailbox create secret generic mailbox-db-migrate-pgpass \
  --from-literal=pgpass="*:*:*:mediated_mailbox_migrate:${password}"
```

The cluster creates the application database owned by the migration role, the three extensions as
the superuser in that database, and every runtime role:

```yaml
apiVersion: postgresql.cnpg.io/v1
kind: Cluster
metadata:
  name: mailbox-db
  namespace: mailbox
spec:
  instances: 1
  imageName: ghcr.io/cloudnative-pg/postgresql:18.6-standard-trixie
  storage:
    size: 5Gi
  bootstrap:
    initdb:
      database: mailbox
      owner: mediated_mailbox_migrate
      secret:
        name: mailbox-db-migrate
      postInitApplicationSQL:
      - CREATE EXTENSION IF NOT EXISTS citext
      - CREATE EXTENSION IF NOT EXISTS pg_trgm
      - CREATE EXTENSION IF NOT EXISTS vector
  managed:
    roles:
    - name: mediated_mailbox_mediate
      ensure: present
      login: true
      passwordSecret:
        name: mailbox-db-mediate
    - name: mediated_mailbox_backfill
      ensure: present
      login: true
      passwordSecret:
        name: mailbox-db-backfill
    - name: mediated_mailbox_sync
      ensure: present
      login: true
      passwordSecret:
        name: mailbox-db-sync
    - name: mediated_mailbox_ui
      ensure: present
      login: true
      passwordSecret:
        name: mailbox-db-ui
    - name: mediated_mailbox_organize
      ensure: present
      login: true
      passwordSecret:
        name: mailbox-db-organize
    - name: mediated_mailbox_propose
      ensure: present
      login: true
      passwordSecret:
        name: mailbox-db-propose
```

CloudNativePG serves TLS with a certificate its own CA signs, and keeps the CA in the Secret
`<cluster>-ca`, so the deployables verify the server by that CA. With the key pair, the bearer token
and the TLS Secrets created as the prerequisites list, the chart's values are:

```yaml
database:
  host: mailbox-db-rw.mailbox.svc
  name: mailbox
  sslmode: verify-full
  caSecret: mailbox-db-ca
  caKey: ca.crt
keys:
  secretName: sealing-keys
migrate:
  passwordFileSecret:
    name: mailbox-db-migrate-pgpass
mediate:
  passwordSecret:
    name: mailbox-db-mediate
  tokenSecret:
    name: bearer-token
  tls:
    secretName: mediate-tls
backfill:
  passwordSecret:
    name: mailbox-db-backfill
sync:
  passwordSecret:
    name: mailbox-db-sync
ui:
  passwordSecret:
    name: mailbox-db-ui
  tls:
    secretName: ui-tls
```
