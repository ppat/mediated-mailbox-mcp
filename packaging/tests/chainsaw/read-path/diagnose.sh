#!/usr/bin/env bash
# What a failed step leaves to read: the release, the objects and events in the chart's namespace, and each pod's
# last log lines. The deployables log no message content, so nothing here carries any.
namespace="${1:?usage: diagnose.sh NAMESPACE}"
kubectl -n flux-system get helmrelease,helmchart,gitrepository -o wide
kubectl -n flux-system describe helmrelease mediated-mailbox | tail -40
kubectl -n "$namespace" get all,configmaps -o wide
kubectl -n "$namespace" get events --sort-by=.lastTimestamp | tail -40
for pod in $(kubectl -n "$namespace" get pods -o name); do
  echo "--- ${pod}"
  kubectl -n "$namespace" logs "$pod" --all-containers --tail=20
done
