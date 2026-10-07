#!/usr/bin/env bash
# Judges a chainsaw run from its log. chainsaw exits zero when it finds no test, so a run whose summary
# counts no passed test, or prints no summary, is refused: a suite that ran nothing proved nothing about
# the chart (ADR-0052). The run's own exit code is checked by the caller.
#
#   passed.sh LOG
set -euo pipefail

log="${1:?usage: passed.sh LOG}"
passed="$(sed -n 's/^- Passed  *tests \([0-9][0-9]*\)$/\1/p' "$log")"
if [[ -z "$passed" || "$passed" -eq 0 ]]; then
  echo "::error::The chainsaw suite passed no test, so the run proved nothing about the chart."
  exit 1
fi
echo "The chainsaw suite passed ${passed} tests."
