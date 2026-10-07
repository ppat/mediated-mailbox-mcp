#!/usr/bin/env bash
# The migration step ran before any deployable started (ADR-0048, ADR-0115): every pod of the mediator,
# backfill, delta sync and the UI was created no earlier than the migration Job completed. Run right after
# the install, so the pods it reads are the install's. The API's timestamps are whole seconds, so a pod
# created in the second the Job completed passes. A migration Job run beside the deployables completes
# seconds after they are created, which fails here, while the deployables' restarts would still let every
# later check pass.
set -euo pipefail

namespace="${1:?usage: migration-first.sh NAMESPACE}"

completed="$(kubectl -n "$namespace" get job mediated-mailbox-migrate -o jsonpath='{.status.completionTime}')"
[[ -n "$completed" ]] || { echo "the migration Job has not completed" >&2; exit 1; }

pods="$(kubectl -n "$namespace" get pods -l 'app.kubernetes.io/component in (mediate,backfill,sync,ui)' \
  -o jsonpath='{range .items[*]}{.metadata.creationTimestamp}{" "}{.metadata.name}{"\n"}{end}')"
[[ -n "$pods" ]] || { echo "no deployable pod exists to compare with the migration Job" >&2; exit 1; }

early=0
while read -r created name; do
  if [[ "$created" < "$completed" ]]; then
    echo "${name} was created at ${created}, before the migration Job completed at ${completed}" >&2
    early=1
  fi
done <<< "$pods"
(( early == 0 )) || exit 1
echo "every deployable pod was created at or after the migration Job completed at ${completed}"
