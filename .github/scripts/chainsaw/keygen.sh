#!/usr/bin/env bash
# Checks the key-generation binary a release attaches (ADR-0088). It downloads the linux/amd64 binary and
# its Sigstore bundle, verifies the keyless signature against the release workflow's identity at the
# release's tag, shows a copy with one byte appended refused, runs the binary to write a key pair, and
# runs the test that seals to that pair and opens with it through the library
# (credential/cmd/keygen/released_test.go). The test skips unless it is given the pair's directory, so
# its verbose output must show it passed rather than skipped.
#
#   keygen.sh VERSION REPOSITORY   such as keygen.sh 0.1.0 owner/name
set -euo pipefail

version="${1:?usage: keygen.sh VERSION REPOSITORY}"
repository="${2:?usage: keygen.sh VERSION REPOSITORY}"
tag="v${version}"
asset="mediated-mailbox-keygen-linux-amd64"
work="$(mktemp -d)"
trap 'rm -rf "$work"' EXIT

gh release download "$tag" --repo "$repository" --dir "$work" --pattern "$asset" --pattern "${asset}.sigstore.json"

verify() {
  mise exec -- cosign verify-blob --bundle "$work/${asset}.sigstore.json" \
    --certificate-identity "https://github.com/${repository}/.github/workflows/release.yaml@refs/tags/${tag}" \
    --certificate-oidc-issuer https://token.actions.githubusercontent.com "$1"
}

verify "$work/$asset"
cp "$work/$asset" "$work/tampered"
printf '\0' >> "$work/tampered"
if verify "$work/tampered" > "$work/tampered.log" 2>&1; then
  echo "::error::cosign accepted the key-generation binary of ${tag} with a byte appended."
  exit 1
fi
echo "A copy of the binary with a byte appended was refused."

chmod +x "$work/$asset"
mkdir "$work/pair"
"$work/$asset" -private-key-file "$work/pair/credential.key" -public-key-file "$work/pair/credential.pub"
MEDIATED_MAILBOX_RELEASED_KEY_DIR="$work/pair" mise exec -- go test -count=1 -v -run '^TestAReleasedPairSealsAndOpens$' \
  ./credential/cmd/keygen | tee "$work/test.log"
grep -q '^--- PASS: TestAReleasedPairSealsAndOpens ' "$work/test.log" ||
  { echo "::error::The test of the released key pair did not pass."; exit 1; }
