# 0073. A commit header's type sizes the release and its scope names the maintenance surface, never the component

**Status:** Accepted · **Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

Every pull request squash-merges, and the header that lands on `main` is what release-please
parses: a single-commit pull request lands its commit header and a multi-commit one lands its
title. The repository releases one lockstep set of images and the chart at one version
([ADR-0049](./0049-image-per-component-lockstep.md)), and every release rebuilds and publishes all
of them, a documentation-only release included, because the chart names images of that version.
There is one release route, and nothing reads a path or a scope to choose it. The release layer
sizes the bump from the type and the breaking marker, and cuts no release from a window holding
only hidden types.

The repository layout is flat, one directory per component ([ADR-0054](./0054-one-repository-flat-layout-naming-convention.md)),
and every pull request already carries one `component:` label per component its diff touches, as
[CLAUDE.md's Repository process](../../../CLAUDE.md#repository-process) requires. Whether a workflow
should produce those labels from the diff waited on this decision, because it turns on whether the
commit scope carries the component.

Three things are true of the machinery before this decision, each established by running it or
reading its pinned source rather than its documentation. No status check is required on `main`, and
the pull request title is never linted, so the vocabulary is advisory and a title changed after the
last push lands unchecked. Renovate's headers close over the enum today but make false claims in
three places. A major of an internal tool, an action or the runner renders a breaking marker, which
release-please bumps on and renders as breaking. The test PostgreSQL image's digest renders `fix`,
which cuts a release for a test substrate. A shared-preset pin major renders `feat` with a
marker. And commitlint parses `feat():` as no scope.

The requirements a vocabulary must meet here:

| Requirement | What it demands | Source |
| --- | --- | --- |
| The header lands true | A future session picks the right header from the diff alone, and no header a person or a bot can write asserts something the diff does not do | This record's requirement, from the release layer reading the header |
| One vocabulary across the operator's repositories | The internal surfaces take the names every ppat repository shares, and the shared Renovate presets emit two of them | This record's requirement, from the shared presets |
| Every emittable header is checked before it is emitted | Renovate and release-please compile headers from configuration, so the set they can compile is derived and linted on every run, and the derivation is proven able to catch an injected defect | [ADR-0046](./0046-tests-are-evidence-once-seen-to-fail.md) |
| The gates block | A check the merge path does not respect is a suggestion, so each gate is a required status context that cannot report skipped | This record's requirement |

## Decision

- **The type is operative and the scope is a claim.** release-please sizes the version bump from
  the type and the breaking marker and decides from the type whether a release happens. Nothing
  routes on the scope. The scope's only job is to be true of the diff. The deciding rules for both
  fields are `.claude/rules/commits.md`'s, and `commitlint.config.js` holds the closed sets.
- **Scopes name the internal maintenance surfaces, and the empty scope is the shipped surface and
  the repository-level residue.** The scopes are `release`, `renovate`, `github-actions`,
  `internal-dependencies`, `internal-workflows` and `agents`, the names every ppat repository
  shares, applied in that order and stopping at the first match. Something ships if an image's
  Dockerfile copies it in, if it is a Dockerfile, or if it sits in the chart's directory.
- **The component is never in the scope.** The `component:` labels carry it, one per component a
  diff touches, and a workflow produces them from the diff, as
  [CLAUDE.md's Repository process](../../../CLAUDE.md#repository-process) states.
- **The types are `feat`, `fix`, `perf`, `refactor`, `revert`, `docs`, `test`, `chore` and `ci`.**
  `build` and `style` are refused. `docs` and `test` render and cut a release, by the standing
  ruling that a documentation pull request proposes a release when merged. `chore` and `ci` are
  hidden and cut none.
- **The pairing rule, enforced by commitlint.** A claim type (`feat`, `fix`, `perf`, `refactor`,
  `revert`) and any spelling of a breaking marker sit only on the empty scope. `ci` sits only on
  `internal-workflows`, `docs` on the empty scope or `agents`, `test` on the empty scope, and
  `chore` anywhere. Empty parentheses are refused as a scope.
- **The half commitlint cannot see is checked against the diff.** A claim type or a breaking
  marker on the empty scope must touch at least one path that ships. The check runs in the
  `commit-taxonomy` job over the pull request's own commits.
- **Every header Renovate and release-please can emit is derived and linted on every run.** The
  `commit-taxonomy` job reads the Renovate configuration with its shared presets at the pinned
  tag, extracts every dependency the tracked tree holds for each modelled manager, folds the rules
  for every dependency under every update type, and lints each rendered header with the same
  commitlint the gates run. It also requires each rendered header to be true: a dependency in a
  file that never ships never renders a claim type or a marker, a shipped dependency's major renders
  the marker, and every scope is claimed in this repository's own configuration, so a preset bump
  moves no header. Its self-test re-introduces every defect found while deriving this decision and
  must catch each before the check's verdict counts.
- **Renovate's local rules make the claims true and invariant.** Every internal surface is claimed
  by file path, manager or update type with the breaking marker written out of the prefix. The
  shipped empty scope is claimed at the top level. The go and bun groups restate the type, the
  scope and the prefix so every member of a group renders one header, and their majors keep the
  marker. Every update type that moves a shipped version renders a claim type, where the presets
  would type some of them hidden, with two exceptions. A pin or a digest pin records the version
  already in use and stays hidden. A lock file refresh is claimed internal, because it is not reviewable per
  dependency, and the browser bundle's lock file can move a shipped transitive dependency under
  that hidden header. A shipped dependency's major keeps the marker as the mechanical
  read-before-deploying signal, and an internal dependency's major never carries one.
- **Three required status contexts.** `commit-messages` over the branch commits, `pr-title` over
  the title on every edit including a title edit after the last push, and `commit-taxonomy`. The
  two local jobs carry no `paths:` filter, no `needs:` and no `if:`, and the reusable
  `commit-messages` job carries only the condition that is true on every pull request event,
  because a skipped job satisfies a required check. One check inside `commit-taxonomy`, whether a
  named scope's footprint contains any path the diff touches, is advisory with a stated sunset,
  because nothing else in the operator's repositories yet enforces it.
- **The checker is a node script beside a root `package.json` and `bun.lock`** that pin the
  commitlint every gate runs, so one engine judges every string that can land on `main`, and the
  local commit-msg hook pins the same version.

## Alternatives considered

- **Component scopes.** The tabled alternative, and the reason the labels question waited. One
  scope per component directory when a diff sits in exactly one, the empty scope when it spans
  more, and the surface scopes above for everything internal. The case for it: a bold component
  prefix on the release notes, and a component to search the log by. Rejected. Of the fifty-three
  open tickets, thirty-four span two to five components, and every code pull request also touches
  the documents the standing update duty requires, so most feature headers would carry the empty
  scope and the notes would render inconsistently. The labels already carry the component
  multi-valued and corrected to the diff, which a single-valued scope cannot. The enum would track
  the component table as components are added, and two component directories contain a `/`, which
  commitlint reads as a scope delimiter, so they would need names of their own.
- **Advisory checks.** One ppat repository keeps its commit lint deliberately unrequired. The case
  for it: the taxonomy exists to tell authors what to write, not to block a merge. Rejected by the
  operator: this repository's pull requests are agent-authored and review-ready when presented, and
  a check the merge path does not respect is a suggestion.
- **No breaking marker on any bot header.** The case for it: a bot cannot read a changelog, so
  its marker asserts nothing. Rejected by the operator. On a shipped dependency the marker is the
  mechanical signal that a human reads before deploying, and at 0.x it cuts a minor rather than a
  major. On an internal surface it is stripped, because nothing there reaches a consumer.
- **A Go checker under `go tool`.** The repository's own tooling programs run that way. The case
  for it: one convention for every check. Rejected by the operator. The checker's judge is
  commitlint, a node program, and a Go program would reimplement the Renovate resolver while still
  shelling out to it, whereas a node script beside the manifest that pins commitlint runs the same
  engine as the gates.

## Consequences

- **Adding a scope is a design change.** It touches `commitlint.config.js`, the rule document, and
  the Renovate rules if a bot can emit it, and the closure check says whether the enum still holds.
- **A change to the Renovate configuration or a shared-preset bump is checked before it lands.**
  The closure check resolves the presets at the pin in the diff, so a bump that would move a header
  turns the pull request red rather than a later bot pull request.
- **A new dependency file is a new set of cells.** The check derives occupancy from the tracked
  tree, and refuses a file that only a manager it does not model would read.
- **Assumptions about other components.** release-please parses the squash title and the whole
  commit message, honours every breaking-marker spelling, `Release-As:` footers and the pull
  request body's override block, and cuts no release from hidden types alone. Renovate composes its
  title from its commit message and assigns a grouped branch the first upgrade's header after
  sorting by file position and dependency name. The reusable commit-lint workflow installs this
  repository's root manifest when a root lock file exists. Branch protection treats a skipped job
  as satisfying a required context.
- **The controls this record introduces** are the pairing rule, the empty-scope check against the
  diff, and the emission closure and truth check with its self-test. Their injections are
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
