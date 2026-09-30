#!/usr/bin/env bash
# git apply --check over every mutation patch under a testdata/mutations directory below the given
# root, the current directory by default, excluding testsupport/cmd/mutproof/testdata/'s own
# fixtures. The same invocation testsupport/cmd/mutproof runs as its own first check on a patch
# (testsupport/cmd/mutproof/copy.go's gitApply), so a pass here means mutproof's own first check on
# that patch would also pass. Takes the root as $1, used by both self-test.sh, which points it at a
# scratch repository, and the mutation-patches workflow, which runs it with no argument from the
# repository root.
#
# The patch list is collected before anything is checked, so a find that fails (an unreadable
# directory, for example) or a find that succeeds but matches nothing is refused rather than left
# to pass with nothing checked.
set -euo pipefail

root="${1:-.}"
cd "$root"

list="$(mktemp)"
trap 'rm -f "$list"' EXIT

if ! find . -path '*/testdata/mutations/*.patch' -not -path '*/testsupport/cmd/mutproof/testdata/*' -print0 > "$list"; then
  echo "::error::find failed while listing mutation patches" >&2
  exit 1
fi

mapfile -d '' -t patches < "$list"

if [ "${#patches[@]}" -eq 0 ]; then
  echo "::error::found no mutation patches under a testdata/mutations directory; the check would pass vacuously" >&2
  exit 1
fi

echo "checking ${#patches[@]} mutation patches"

status=0
for patch in "${patches[@]}"; do
  if ! git apply --check "$patch"; then
    echo "::error file=${patch#./}::no longer applies to the working tree" >&2
    status=1
  fi
done

exit "$status"
