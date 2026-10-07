#!/usr/bin/env bash
# The self-test the chainsaw workflow runs before it trusts passed.sh, so the check that a run tested
# something is proven on every run, whether or not a release exists to run the suite against. It runs
# the pinned chainsaw itself, with no cluster, over two scratch suites: one holding no test, which
# chainsaw reports as a success and passed.sh must refuse, and one holding a single test that passes,
# which passed.sh must accept. Modeled on .github/scripts/mutation-patches/self-test.sh.
#
# Each case captures passed.sh's output rather than letting it reach this step's stdout, so a case
# expected to fail leaves no workflow annotation on a green run. It is printed only when a case's own
# assertion fails.
#
#   self-test.sh CONFIG   where CONFIG is the suite's chainsaw configuration
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
config="${1:?usage: self-test.sh CONFIG}"
scratch="$(mktemp -d)"
trap 'rm -rf "$scratch"' EXIT

# Runs chainsaw over suite with no cluster and writes its log to log, requiring chainsaw itself to exit
# zero, so the verdict below is passed.sh's alone.
run_suite() {
  local suite="$1" log="$2"
  if ! mise exec -- chainsaw test "$suite" --config "$config" --no-cluster > "$log" 2>&1; then
    echo "self-test failed: chainsaw itself failed on ${suite}" >&2
    cat "$log" >&2
    exit 1
  fi
}

mkdir -p "$scratch/empty" "$scratch/one/passes"
cat > "$scratch/one/passes/chainsaw-test.yaml" << 'EOF'
apiVersion: chainsaw.kyverno.io/v1alpha1
kind: Test
metadata:
  name: passes
spec:
  steps:
  - try:
    - script:
        content: "true"
EOF

echo "self-test: a run that found no test, which chainsaw reports as a success, is refused"
run_suite "$scratch/empty" "$scratch/empty.log"
if output="$("$here/passed.sh" "$scratch/empty.log" 2>&1)"; then
  echo "self-test failed: passed.sh accepted a run that passed no test" >&2
  echo "$output" >&2
  exit 1
fi

echo "self-test: a run that passed one test is accepted"
run_suite "$scratch/one" "$scratch/one.log"
if ! output="$("$here/passed.sh" "$scratch/one.log" 2>&1)"; then
  echo "self-test failed: passed.sh refused a run that passed a test" >&2
  echo "$output" >&2
  exit 1
fi
echo "self-test: passed"
