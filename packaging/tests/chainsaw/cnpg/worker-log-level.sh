#!/usr/bin/env bash
# Checks the worker's rendered file holds the debug log level the upgrade step sets. The release is named by RELEASE.
set -euo pipefail
kubectl --namespace "$NAMESPACE" get configmap "${RELEASE}-mediated-mailbox-config" --output jsonpath='{.data.worker\.yaml}' | grep -qx 'log_level: debug'
