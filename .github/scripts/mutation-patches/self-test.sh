#!/usr/bin/env bash
# The self-test the mutation-patches workflow runs before it trusts check.sh, so a broken checker is
# caught before it stands as evidence. Modeled on this repository's other self-tested workflow
# checks (.github/scripts/pr-labels/self-test.mjs, .github/scripts/commit-taxonomy/self-test.mjs),
# it builds a scratch case, proves the checker's verdict on it, and only then lets the checker judge
# the real patches.
#
# Each case captures check.sh's combined output rather than letting it reach this step's own
# stdout, so a case expected to fail never leaves a workflow-command annotation on an otherwise
# green run. The captured output is printed only when a case's own assertion fails.
set -euo pipefail

here="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
scratch_dirs=()
trap 'rm -rf "${scratch_dirs[@]}"' EXIT

expect_pass() {
  local root="$1" description="$2" output
  echo "self-test: $description"
  if ! output=$("$here/check.sh" "$root" 2>&1); then
    echo "self-test failed: check.sh refused a root it should have accepted" >&2
    echo "$output" >&2
    exit 1
  fi
}

expect_fail() {
  local root="$1" description="$2" output
  echo "self-test: $description"
  if output=$("$here/check.sh" "$root" 2>&1); then
    echo "self-test failed: check.sh accepted a root it should have refused" >&2
    echo "$output" >&2
    exit 1
  fi
}

# Seeds root, a fresh git repository, with one commit and writes a patch to patch_path that
# applies against it, generated with git diff so its shape matches a real mutation patch's.
seed_applying_patch() {
  local root="$1" file="$2" patch_path="$3"
  git -C "$root" init -q -b main
  git -C "$root" config user.name self-test
  git -C "$root" config user.email self-test@example.invalid
  mkdir -p "$root/$(dirname "$file")" "$(dirname "$patch_path")"
  printf 'line one\nline two\nline three\n' > "$root/$file"
  git -C "$root" add "$file"
  git -C "$root" commit -q -m baseline
  sed -i 's/line two/line two, mutated/' "$root/$file"
  git -C "$root" diff > "$patch_path"
  git -C "$root" checkout -q -- "$file"
}

scratch="$(mktemp -d)"
scratch_dirs+=("$scratch")
seed_applying_patch "$scratch" "pkg/file.go" "$scratch/pkg/testdata/mutations/demo.patch"

expect_pass "$scratch" "a patch whose context matches the working tree is accepted"

# The same patch once the file it targets has moved on, the failure mode a stale diff context
# produces.
sed -i 's/line one/line one, moved on/' "$scratch/pkg/file.go"
expect_fail "$scratch" "a patch whose context no longer matches the working tree is refused"

# A root with no mutation patches at all, since the check must never pass on nothing checked.
scratch_empty="$(mktemp -d)"
scratch_dirs+=("$scratch_empty")
expect_fail "$scratch_empty" "a root with no mutation patches is refused"

# A testdata/mutations directory the check cannot read, when the self-test itself is not root
# (chmod has no effect on root, so the case would prove nothing there). A second, readable
# testdata/mutations holds a patch that applies, so the count of patches found is never zero and
# only a refusal to trust an unreadable find is what can make this case refuse.
if [ "$(id -u)" -ne 0 ]; then
  scratch_unreadable="$(mktemp -d)"
  scratch_dirs+=("$scratch_unreadable")
  seed_applying_patch "$scratch_unreadable" "other/file.go" "$scratch_unreadable/other/testdata/mutations/good.patch"
  mkdir -p "$scratch_unreadable/pkg/testdata/mutations"
  printf 'line one\n' > "$scratch_unreadable/pkg/testdata/mutations/demo.patch"
  chmod 000 "$scratch_unreadable/pkg/testdata/mutations"
  expect_fail "$scratch_unreadable" "a testdata/mutations directory the check cannot read is refused, beside a patch that applies"
  chmod 755 "$scratch_unreadable/pkg/testdata/mutations"
else
  echo "self-test: skipped the unreadable-directory case, running as root"
fi

echo "self-test passed"
