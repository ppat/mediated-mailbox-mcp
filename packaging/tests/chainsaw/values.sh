#!/usr/bin/env bash
# Stores the chart values file named as the first argument in the ConfigMap the release reads its values from, in
# flux-system, named for the release chainsaw passes as RELEASE. Flux watches the labelled ConfigMap, so storing new
# values upgrades the release.
set -euo pipefail
kubectl --namespace flux-system create configmap "$RELEASE" --from-file=values.yaml="$1" --dry-run=client --output yaml \
| kubectl label --local --filename - reconcile.fluxcd.io/watch=Enabled --output yaml \
| kubectl apply --filename -
