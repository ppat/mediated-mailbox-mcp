#!/usr/bin/env bash
# Upgrades the installed release twice and starts backfill by hand, checking what each does on its own
# (ADR-0103, ADR-0116, ADR-0117).
#
# 1. A change reaching only the UI. The upgrade runs no backfill, so the one backfill Job keeps its name.
# 2. A change of the scanner's section. The upgrade runs backfill once, under a new Job that completes, and the
#    Job before it is removed. Delta sync, whose file holds the section, rolls to a new pod, and a watch over the
#    whole upgrade never sees two sync pods at once, a terminating one included.
# 3. A run started by hand from the suspended CronJob completes.
set -euo pipefail

namespace="${1:?usage: upgrades.sh NAMESPACE}"
release=mediated-mailbox

# The Jobs the chart runs backfill in, by name, leaving out the run started by hand.
backfill_jobs() {
  kubectl -n "$namespace" get jobs -l app.kubernetes.io/component=backfill -o name | grep -v '/manual-' | sort || true
}

# Patches the release's values and waits until flux has upgraded to the new generation and the release is ready.
upgrade() {
  local patch="$1"
  kubectl -n flux-system patch helmrelease "$release" --type merge -p "$patch"
  local generation
  generation="$(kubectl -n flux-system get helmrelease "$release" -o jsonpath='{.metadata.generation}')"
  for _ in $(seq 1 300); do
    observed="$(kubectl -n flux-system get helmrelease "$release" -o jsonpath='{.status.observedGeneration}')"
    ready="$(kubectl -n flux-system get helmrelease "$release" -o jsonpath='{.status.conditions[?(@.type=="Ready")].status}')"
    if [[ "$observed" == "$generation" && "$ready" == True ]]; then
      return 0
    fi
    sleep 2
  done
  echo "the release did not become ready at generation ${generation}" >&2
  kubectl -n flux-system get helmrelease "$release" -o yaml >&2
  return 1
}

before="$(backfill_jobs)"
[[ "$(wc -l <<< "$before")" -eq 1 && -n "$before" ]] || { echo "want one backfill Job before the upgrades, have: ${before}" >&2; exit 1; }

echo "1. an upgrade reaching only the UI"
upgrade '{"spec":{"values":{"ui":{"config":"operator_name: chainsaw\n"}}}}'
after="$(backfill_jobs)"
[[ "$after" == "$before" ]] || { echo "an upgrade reaching only the UI changed the backfill Jobs from ${before} to ${after}" >&2; exit 1; }

echo "2. an upgrade changing the scanner's section"
sync_before="$(kubectl -n "$namespace" get pod "${release}-sync-0" -o jsonpath='{.metadata.uid}')"
watch="$(mktemp)"
(
  while true; do
    kubectl -n "$namespace" get pods -l app.kubernetes.io/component=sync --no-headers 2> /dev/null | wc -l >> "$watch"
    sleep 0.2
  done
) &
watcher=$!
trap 'kill "$watcher" 2> /dev/null || true; rm -f "$watch"' EXIT
upgrade '{"spec":{"values":{"scannerConfig":"scanner:\n  triggers:\n    en: [verify, \"log in\", chainsaw]\n"}}}'
kubectl -n "$namespace" rollout status statefulset/"${release}-sync" --timeout=300s
kill "$watcher"
most="$(sort -n "$watch" | tail -1)"
samples="$(wc -l < "$watch")"
echo "sync pods seen at once, at most: ${most}, over ${samples} samples"
[[ "$most" -eq 1 ]] || { echo "delta sync ran ${most} pods at once during the upgrade" >&2; exit 1; }
sync_after="$(kubectl -n "$namespace" get pod "${release}-sync-0" -o jsonpath='{.metadata.uid}')"
[[ "$sync_after" != "$sync_before" ]] || { echo "delta sync did not roll after its configuration changed" >&2; exit 1; }

after="$(backfill_jobs)"
[[ "$(wc -l <<< "$after")" -eq 1 && "$after" != "$before" ]] ||
  { echo "want one new backfill Job after the scanner change, have: ${after} (before: ${before})" >&2; exit 1; }
kubectl -n "$namespace" wait --for=condition=complete "${after}" --timeout=300s

echo "3. backfill started by hand"
kubectl -n "$namespace" create job --from="cronjob/${release}-backfill" manual-chainsaw
kubectl -n "$namespace" wait --for=condition=complete job/manual-chainsaw --timeout=300s
