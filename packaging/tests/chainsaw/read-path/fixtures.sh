#!/usr/bin/env bash
# The harness's own objects for the read path's suite, which the chart takes as inputs and never creates
# (ADR-0052): a PostgreSQL with the superuser bootstrap the deploying side runs (ADR-0048), and every Secret the
# chart mounts. Nothing here is the chart's, so nothing here is evidence about the chart.
#
# PostgreSQL runs in its own namespace, outside the restricted Pod Security level the chart's namespace enforces,
# from the image the integration tests start (testsupport/cmd/pgrun/image.go), so the pin has one home. Its init
# scripts run the repository's own bootstrap files, give each role a random password, and create the application
# database owned by the migration role.
#
# Every secret is generated for this run: the role passwords, the key pair from the key-generation command, the
# self-signed TLS material and the bearer token. Needs kubectl, openssl and go.
set -euo pipefail

namespace="${1:?usage: fixtures.sh NAMESPACE}"
root="$(git rev-parse --show-toplevel)"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

image="$(sed -n 's/^const image = "\(.*\)"$/\1/p' "$root/testsupport/cmd/pgrun/image.go")"
[[ -n "$image" ]] || { echo "no image found in testsupport/cmd/pgrun/image.go" >&2; exit 1; }

roles=(migrate mediate backfill sync ui)
for role in "${roles[@]}"; do
  openssl rand -hex 24 > "$work/$role"
done

kubectl create namespace postgres --dry-run=client -o yaml | kubectl apply -f -
kubectl create namespace "$namespace" --dry-run=client -o yaml | kubectl apply -f -
# The chart's pods run under the restricted Pod Security level, a core admission control, so any pod the chart
# renders without ADR-0028's hardening is refused and the install fails.
kubectl label namespace "$namespace" --overwrite \
  pod-security.kubernetes.io/enforce=restricted pod-security.kubernetes.io/enforce-version=latest

passwords=()
for role in "${roles[@]}"; do passwords+=(--from-file="$role=$work/$role"); done
kubectl -n postgres create secret generic role-passwords "${passwords[@]}" --dry-run=client -o yaml | kubectl apply -f -

cat > "$work/00-bootstrap.sh" <<'EOF'
#!/bin/bash
# Runs once, as the superuser, when the data directory is first initialised.
set -euo pipefail
psql -v ON_ERROR_STOP=1 --username postgres --dbname postgres -f /bootstrap/roles.sql
for role in migrate mediate backfill sync ui; do
  psql -v ON_ERROR_STOP=1 --username postgres --dbname postgres \
    -c "ALTER ROLE mediated_mailbox_${role} PASSWORD '$(cat "/passwords/${role}")'"
done
psql -v ON_ERROR_STOP=1 --username postgres --dbname postgres -c "CREATE DATABASE mailbox OWNER mediated_mailbox_migrate"
psql -v ON_ERROR_STOP=1 --username postgres --dbname mailbox -f /bootstrap/extensions.sql
EOF
kubectl -n postgres create configmap bootstrap \
  --from-file=roles.sql="$root/db/bootstrap/roles.sql" \
  --from-file=extensions.sql="$root/db/bootstrap/extensions.sql" \
  --from-file=00-bootstrap.sh="$work/00-bootstrap.sh" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl apply -f - <<EOF
apiVersion: apps/v1
kind: Deployment
metadata:
  name: postgres
  namespace: postgres
spec:
  replicas: 1
  selector:
    matchLabels: {app: postgres}
  template:
    metadata:
      labels: {app: postgres}
    spec:
      containers:
      - name: postgres
        image: ${image}
        env:
        - {name: POSTGRES_HOST_AUTH_METHOD, value: scram-sha-256}
        - {name: POSTGRES_INITDB_ARGS, value: --auth-host=scram-sha-256}
        - {name: POSTGRES_PASSWORD_FILE, value: /passwords/migrate}
        ports:
        - {name: postgres, containerPort: 5432}
        readinessProbe:
          exec: {command: [pg_isready, --username, postgres, --dbname, mailbox]}
        volumeMounts:
        - {name: bootstrap, mountPath: /bootstrap, readOnly: true}
        - {name: initdb, mountPath: /docker-entrypoint-initdb.d, readOnly: true}
        - {name: passwords, mountPath: /passwords, readOnly: true}
      volumes:
      - name: bootstrap
        configMap: {name: bootstrap, items: [{key: roles.sql, path: roles.sql}, {key: extensions.sql, path: extensions.sql}]}
      - name: initdb
        configMap: {name: bootstrap, items: [{key: 00-bootstrap.sh, path: 00-bootstrap.sh}]}
      - name: passwords
        secret: {secretName: role-passwords}
---
apiVersion: v1
kind: Service
metadata:
  name: postgres
  namespace: postgres
spec:
  selector: {app: postgres}
  ports:
  - {name: postgres, port: 5432, targetPort: postgres}
EOF

# Each deployable's own password, and the migration role's as a password file in libpq's format.
for role in mediate backfill sync ui; do
  kubectl -n "$namespace" create secret generic "database-${role}" --from-file=password="$work/$role" \
    --dry-run=client -o yaml | kubectl apply -f -
done
printf '*:*:*:mediated_mailbox_migrate:%s\n' "$(cat "$work/migrate")" > "$work/pgpass"
kubectl -n "$namespace" create secret generic database-migrate --from-file=pgpass="$work/pgpass" \
  --dry-run=client -o yaml | kubectl apply -f -

# The key pair credentials are sealed to (ADR-0088), from the key-generation command.
(cd "$root" && go build -o "$work/keygen" ./credential/cmd/keygen)
"$work/keygen" -private-key-file "$work/credential.key" -public-key-file "$work/credential.pub"
kubectl -n "$namespace" create secret generic sealing-keys \
  --from-file=credential.key="$work/credential.key" --from-file=credential.pub="$work/credential.pub" \
  --dry-run=client -o yaml | kubectl apply -f -

# Self-signed TLS material for the mediator's surface and the UI, and the bearer token clients present.
for component in mediate ui; do
  openssl req -x509 -newkey ec -pkeyopt ec_paramgen_curve:prime256v1 -nodes -days 1 \
    -subj "/CN=${component}" -keyout "$work/${component}.key" -out "$work/${component}.crt" 2> /dev/null
  kubectl -n "$namespace" create secret tls "tls-${component}" --cert="$work/${component}.crt" \
    --key="$work/${component}.key" --dry-run=client -o yaml | kubectl apply -f -
done
openssl rand -hex 32 > "$work/token"
kubectl -n "$namespace" create secret generic bearer-token --from-file=token="$work/token" \
  --dry-run=client -o yaml | kubectl apply -f -

kubectl -n postgres rollout status deployment/postgres --timeout=180s
