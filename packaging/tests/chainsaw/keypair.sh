#!/usr/bin/env bash
# Writes a key pair with the keygen subcommand of the repository's command-line program and stores it as the Secret
# sealing-keys in the test's namespace, which chainsaw passes as NAMESPACE.
set -euo pipefail
dir="$(mktemp -d)"
trap 'rm -rf "$dir"' EXIT
(cd "$(git rev-parse --show-toplevel)" && go run ./cli keygen -private-key-file "$dir/credential.key" -public-key-file "$dir/credential.pub")
kubectl --namespace "$NAMESPACE" create secret generic sealing-keys --from-file=credential.key="$dir/credential.key" --from-file=credential.pub="$dir/credential.pub"
