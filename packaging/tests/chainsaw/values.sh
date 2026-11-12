#!/usr/bin/env bash
# Stores the chart values file named as the first argument in the ConfigMap the release reads its values from, in
# flux-system, named for the release chainsaw passes as RELEASE. Each ${NAMESPACE} in the file becomes the test's
# namespace, which chainsaw passes as NAMESPACE, so a value can name a Service by its fully qualified name. Flux
# watches the labelled ConfigMap, so storing new values upgrades the release.
set -euo pipefail
values="$(mktemp)"
trap 'rm -f "$values"' EXIT
sed "s/\${NAMESPACE}/${NAMESPACE}/g" "$1" > "$values"
kubectl --namespace flux-system create configmap "$RELEASE" --from-file=values.yaml="$values" --dry-run=client --output yaml \
| kubectl label --local --filename - reconcile.fluxcd.io/watch=Enabled --output yaml \
| kubectl apply --filename -
