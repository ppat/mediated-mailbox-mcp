# commit-taxonomy

The checks behind the `commit-taxonomy` status context, run by the job of that name in
[`.github/workflows/lint.yaml`](../../workflows/lint.yaml). This file is for whoever has to change
them. The vocabulary itself is [`.claude/rules/commits.md`](../../../.claude/rules/commits.md)'s,
and the decision is ADR-0073's, resolved through the [decision-record index](../../../docs/adr/README.md).

## Why the checks exist

Renovate and release-please compile commit headers from configuration. commitlint only ever sees
headers that already exist. Nothing else compares the two, so a configuration capable of emitting a
header commitlint would reject, or a header whose claim is false, stays green until the bot opens
that pull request, possibly months later. The checks derive the emittable set and judge it now.

## Running them

```bash
mise exec -- bun install --frozen-lockfile
mise exec node -- node .github/scripts/commit-taxonomy/self-test.mjs
mise exec node -- node .github/scripts/commit-taxonomy/check-commit-taxonomy.mjs --list
mise exec node -- node .github/scripts/commit-taxonomy/check-commit-taxonomy.mjs closure
mise exec node -- node .github/scripts/commit-taxonomy/check-commit-taxonomy.mjs --dump-headers
```

commitlint comes from the root `package.json` and `bun.lock`, which are also what the
`commit-messages` and `pr-title` jobs install, so the checks and the gates they predict judge with
one engine. `--offline-presets DIR` replaces the fetch of every `ppat/renovate-presets` file with a
read from `DIR`, which is how a preset bump is rehearsed before it is taken. The self-test uses it to
inject an upstream scope rename.

## The checks

| Check | Reads | Asserts |
| --- | --- | --- |
| `closure` | the Renovate configuration with its presets at the pinned tag, every dependency the tracked tree holds, `release-please-config.json` | every header-shaped key is classified, every local rule uses only modelled matchers, no tracked file is read only by an unmodelled manager, every rendered header for every dependency under every update type passes commitlint, semantic commits resolve to enabled, and every release-please title pattern renders a header commitlint accepts |
| `truth` | the same, plus the release workflow and the Dockerfiles it names | a dependency in a file that never ships never renders a claim type or a breaking marker, a shipped dependency's version-moving updates render a claim type and only its major renders the marker, and every scope is claimed by this repository's own configuration |
| `self-consistency` | `commitlint.config.js`, `release-please-config.json`, the two gate workflows, `.pre-commit-config.yaml`, `package.json` | the type enum equals the changelog sections, the local rules name only enum members, the named footprints do not overlap, the two gate jobs cannot report skipped and run on the events that matter, and the local hook pins the gates' commitlint |
| `message-shape` | the pull request's commits and body | no body paragraph reads as a second header, no `Release-As:` footer, no override block, and the release parser is still the major the patterns were derived against |
| `empty-scope` | the pull request's commits and their paths | a claim type or a breaking marker on the empty scope touches a path that ships |
| `named-scope` | the same | a line-level scope's changed paths all sit inside its footprint, and a path scope's footprint contains at least one changed path. Advisory, with the sunset stated in the code |

What ships is read from `release.yaml`: what each image's Dockerfile copies in, the Dockerfiles
themselves, and the directory the chart is packaged from. A bun devDependency is internal wherever
its manifest sits.

## What is modelled, and what is refused

Occupancy is extracted from the tracked tree for the managers this repository uses: `gomod`,
`dockerfile`, `mise`, `bun`, `pre-commit`, `github-actions` including the runner, the `custom.regex`
managers declared in `.github/renovate.json`, and `renovate-config`. A file only another manager
would read is a hard failure, so a new kind of dependency file is taught to the model before its
headers can be emitted unenumerated. The matchers the fold evaluates are `matchManagers`,
`matchFileNames`, `matchDepNames`, `matchPackageNames`, `matchDepTypes`, `matchUpdateTypes` and
`matchDatasources`. Any other matcher on a local rule is refused, and `matchDatasources` is refused
on a cell whose datasource the extractor does not assign unless another matcher already excludes the
rule. Prefix templates render `semanticCommitType`, `semanticCommitScope` and the `#if` over the
scope, and anything else is refused.

Not modelled, and stated as such: which of a grouped branch's headers Renovate emits when the group
mixes update types. Every candidate is in the closure, and the merge-time gates lint whatever the
branch produced. A pin or a digest pin of a shipped dependency may render a hidden type, because it
records the version already in use rather than moving it, and a lock file refresh renders the
internal header by policy, the accepted leak the Renovate rule states.

The presets and the pinned release workflow are fetched from `raw.githubusercontent.com`. A
transient failure is retried with backoff, and a 404 fails on the first attempt, because at a
pinned ref it means the pin or the path is wrong.

## The self-test

Every defect found while deriving the vocabulary is re-introduced and must be caught before any
check's verdict counts. Configuration defects are injected by overriding the readers a check calls,
verified green before and red after with the expected message. Rule defects are injected into a
generated commitlint configuration that weakens one rule, verified accepted under the weakened
configuration and rejected under the real one. Occupancy is guarded against vacuity first. When
modelling is added, an injection is added with it.
