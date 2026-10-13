#!/usr/bin/env bash
# Saves the API server's bearer token and the UI's request token key from the Secret the chart generated, or compares
# the values now held with the saved ones, so an upgrade is seen to keep them. The release is named by RELEASE and
# its namespace by NAMESPACE, both of which chainsaw passes.
set -euo pipefail
saved="${TMPDIR:-/tmp}/${RELEASE}-generated-values-${NAMESPACE}"
secret="${RELEASE}-mediated-mailbox-access-credentials"
current="$(kubectl --namespace "$NAMESPACE" get secret "$secret" --output jsonpath='{.data.api-bearer-token} {.data.ui-request-token-key}')"
read -r token key <<< "$current"
[ -n "$token" ] && [ -n "$key" ]
case "$1" in
save) printf '%s' "$current" > "$saved" ;;
compare) [ "$current" = "$(cat "$saved")" ] ;;
esac
