#!/usr/bin/env bash
# Delta sync reads the database as its own role. Its probes answer whether or not the database does, and a
# tick over no account reads nothing a probe shows, so this stores one OAuth client as the harness's
# superuser and waits for the series delta sync sets for each client a tick read from `oauth_clients`
# (ADR-0103). Only a successful read of the snapshot under delta sync's role produces it, so a wrong
# password, user or database leaves it absent and the step fails. The client's secret opens with no key,
# which delta sync logs and reports as 1, so the series exists whatever its value.
set -euo pipefail

namespace="${1:?usage: sync-reads.sh NAMESPACE}"
service="mediated-mailbox-sync:probes"

kubectl -n postgres exec deploy/postgres -- psql -v ON_ERROR_STOP=1 --username postgres --dbname mailbox -c \
  "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('chainsaw', 'gmail', 'chainsaw-client', '\\x00') ON CONFLICT DO NOTHING"

for _ in $(seq 1 90); do
  if kubectl get --raw "/api/v1/namespaces/${namespace}/services/${service}/proxy/metrics" 2> /dev/null |
    grep -q '^mediated_mailbox_sync_client_secret_on_old_key{client="chainsaw"}'; then
    echo "delta sync read the stored OAuth client through its own role"
    exit 0
  fi
  sleep 2
done
echo "delta sync set no series for the stored OAuth client, so no tick read the database through its role" >&2
kubectl -n "$namespace" logs statefulset/mediated-mailbox-sync --tail=20 >&2
exit 1
