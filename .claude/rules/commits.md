---
description: How to choose the type and scope of a commit in this repository. The type is the release decision, and the scope is a claim about the diff that must be kept true.
---

# Commit types and scopes

The two header fields have different natures, and every rule below depends on the distinction:

> **Type is operative.** release-please sizes the version bump from it and decides from it whether
> a release happens at all.
>
> **Scope is operative in nothing.** The repository releases one lockstep set of images and the
> chart, so nothing routes on it. Its only job is to be true.

Both sets are closed. `commitlint.config.js` is the source of truth for their membership, and
adding a member is a design change, not an improvisation.

Squash-merge lands a single-commit pull request's **commit header** and a multi-commit pull
request's **title**, so both are gated against this same vocabulary: the `commit-messages` job
reads the branch commits, and the `pr-title` job reads the title on every edit. The body that lands
is the concatenation of the branch commits' messages, so land one commit if the body should read as
one message.

## What ships

Everything below follows from one question, asked of whatever the commit touched:

> **Does a release put this in front of a consumer?** Something ships if an image's Dockerfile
> copies it in, if it is a Dockerfile, or if it sits in the chart's directory.

Two consequences that are easy to get backwards:

- **A tool is not shipped because it is pinned.** `mise.toml` and `.pre-commit-config.yaml` pin the
  authoring and continuous-integration toolchain. The Go and bun that build the images are pinned
  there too, and those two pins ride the shipped Dockerfile pins as one group. Ask where the pin
  lives and what the pin builds, not what the tool is for.
- **A file inside a shipped directory is not shipped by being there.** The browser's
  devDependencies, its lint configuration and the ban rules under `ui/browser/rules/` are build
  and check machinery that never enters the bundle. The test PostgreSQL image under `testsupport/`
  starts a container the tests use. Neither reaches a consumer.

## Scopes

Apply **in order, stopping at the first match.** The ordering is what makes the choice
deterministic.

| # | Scope | The diff | Ships |
| --- | --- | --- | --- |
| 1 | `release` | a release cut authored by release-please | — |
| 2 | `renovate` | `.github/renovate.json`, and nothing else, a shared-preset pin bump included | no |
| 3 | `github-actions` | moves a `uses:` reference or a runner version, anywhere, and nothing else | no |
| 4 | `internal-dependencies` | moves any other pin in a file that never ships, and nothing else: `mise.toml` and `mise.lock`, `.pre-commit-config.yaml`, the root `package.json` and `bun.lock`, the browser devDependencies and both browser lock files, the type-generation manifest under `ui/browser/codegen/`, a workflow's `env:` pin, the test PostgreSQL image | no |
| 5 | `internal-workflows` | this repository's own machinery, by hand: `.github/workflows/**`, `.github/scripts/**`, every linter and lint-ban configuration, `commitlint.config.js` and the root `package.json`, the release-please configuration and manifest, `mise.toml`, `.pre-commit-config.yaml` | no |
| 6 | `agents` | `CLAUDE.md` and `.claude/**`, anything else written for an AI coding agent | no |
| 7 | *(empty)* | everything else: the deployables, the libraries, the Dockerfiles, `go.mod` and `go.sum`, the browser dependencies, the chart, the test support, the chainsaw suite, the document set, the ticket template, repository-root residue | can ship |

Row 1 is a release cut, whose diff is exactly the files release-please writes. Rows 2 to 4 are
line-level, a version moved and nothing else did, so they sort above the path rows:
a workflow edit that also re-pins an action fails row 3 and lands on row 5. Row 6 is files written
for an agent, not documentation generally: `README.md` and `DESIGN.md` are row 7.

The empty scope claims one of two things, and the type says which: a claim type says the change
ships, and any other type says it is repository-level. The component a diff touches is never in
the scope. The `component:` labels carry it, one per component, which a scope cannot.

When a diff spans rows 5 to 7, scope it to what **motivated** it. A rule document, a commitlint
change and a workflow landing together to enforce one convention are `internal-workflows`. Code
that lands with the document updates the standing duty requires is the empty scope, and the docs
are subsumed. If a diff genuinely has two motivations, split it.

## Types

| Type | The diff | Changelog | Alone in a release window, at 0.x |
| --- | --- | --- | --- |
| `feat` | shipped behaviour gained | ✨ Features | minor |
| `fix` | shipped behaviour corrected | 🚀 Enhancements + Bug Fixes | patch |
| `perf` | shipped behaviour reworked to cost less, behaviour held | 🚀 Enhancements + Bug Fixes | patch |
| `refactor` | shipped code reworked, behaviour held | 🚀 Enhancements + Bug Fixes | patch |
| `revert` | undoing a shipped change | ⚙️ Other | patch |
| `docs` | prose: the document set, the records, READMEs, comments, agent instructions | 🛠 Improvements | patch |
| `test` | test code and test tooling, wherever it sits | 🛠 Improvements | patch |
| `chore` | pins, lock files, the Renovate configuration, repository-root residue | hidden | no release |
| `ci` | workflows, linter and lint-ban configuration, the release-please configuration | hidden | no release |

`build` and `style` are not legal. The build tooling here is either continuous-integration
machinery or a shipped Dockerfile, so `build` would be the wrong-but-tempting answer for a `ci` or
a `feat` change, and it is hidden, so the miss would be silent. `style` renders and cuts a release,
so a cosmetic header would republish every image.

`docs` and `test` render and cut a release, by the ruling that a documentation pull request
proposes a release when merged. Every release rebuilds every image, because the chart it publishes
names images of that version.

A breaking marker bumps the version and renders even for a hidden type, so it is a release decision
independent of the type it sits on. Every spelling counts: `type!:`, a `BREAKING CHANGE:` footer,
and `BREAKING-CHANGE:` anywhere in the body.

## The pairing rule

| # | Rule | Why |
| --- | --- | --- |
| 1 | `feat`, `fix`, `perf`, `refactor`, `revert` and any breaking marker ⇒ the empty scope only | they render a consumer-facing line asserting a shipped artifact changed, and no named scope can make that true |
| 2 | `ci` ⇒ `internal-workflows` only | continuous-integration machinery is always that surface |
| 3 | `docs` ⇒ the empty scope or `agents` | prose is repository-level, or written for an agent |
| 4 | `test` ⇒ the empty scope | test code sits in the tree beside what it tests |
| 5 | `chore` ⇒ any scope | housekeeping on any surface |

commitlint rejects a violation of every row. A bug fix in a workflow is `ci(internal-workflows):`
with the word "fix" in the subject, because `fix` would claim a shipped artifact changed.

**On the empty scope the pairing rule is only half the check.** commitlint sees the header and
never the diff. The `commit-taxonomy` job adds the other half: a claim type or a breaking marker on
the empty scope must touch at least one path that ships. A pull request that reworks only the test
support and the agent rules takes `test:` or `docs(agents):`, never `feat:`. The same job checks a
named scope against the diff, advisory for now: a line-level scope's diff must sit entirely inside
its footprint, and a path scope's diff must touch its footprint at all.

## Cases that would otherwise be guessed

| Situation | Header |
| --- | --- |
| A feature in one deployable, with the library change it needs and the document updates the standing duty requires | `feat:`, one pull request. The component labels say which directories |
| A change to the test support, the chainsaw suite or a mutation patch | `test:` |
| A change to `.golangci.yaml`, the browser ban rules or `commitlint.config.js` | `ci(internal-workflows):` |
| Editing `CLAUDE.md`, a rule under `.claude/rules/` or the update-docs skill | `docs(agents):` |
| Editing `DESIGN.md`, `ROADMAP.md`, a record or the ticket template | `docs:` |
| A comment-only edit inside shipped code | `docs:` if behaviour is provably unchanged. Otherwise it was never comment-only |
| Bumping a tool in `mise.toml` by hand | `chore(internal-dependencies):`. A hand edit to a mise task or setting is `ci(internal-workflows):` |
| A change to a Renovate rule that alters which header the bot emits | `chore(renovate):`, and the closure check says whether the enum still holds |
| A `revert` | takes the type's own rule: it is a claim type, so it sits on the empty scope and must touch a shipped path |
| Renovate and release-please headers | leave them alone. This repository's own Renovate rules and the release pull request title pattern assert them |

## Bot headers

Renovate compiles its headers from `.github/renovate.json`. What must stay true is **closure**:
every header the configuration can emit for a dependency this tree holds is inside the commitlint
enums and makes a true claim. The `commit-taxonomy` job derives that set on every run, from the
configuration and from the dependency files actually tracked, so a new dependency file, a moved
pin or a preset bump re-derives it with no config edit. When it goes red on a configuration or
preset change, the emitter or the enum is wrong, never the check's place in the pipeline.

Renovate's own behaviour on a grouped branch is a documented property, not a violation: the branch
takes the first upgrade's header after sorting by file position and dependency name. The go and bun
groups span the Dockerfiles, `go.mod` and `mise.toml`, and every member of each group renders the
same header, which is why their rules restate the type, the scope and the prefix.

Invariants to re-check when the shared-preset pin moves, rather than the preset's contents:

- **Every scope is claimed locally, by file path, manager or update type, with no
  `matchUpdateTypes` on the claim** unless the claim is the update type itself, so no update type
  falls through to the preset. The shipped empty scope is claimed at the top level of the file.
- **Every update type that moves a shipped version renders a claim type.** The presets type a
  rollback, a bump, a replacement and a lock file entry update `chore`, which is hidden, so a local
  rule types them `fix`, and the go and bun groups restate it. Two exceptions stay hidden. A pin
  or a digest pin records the version already in use. A lock file refresh is claimed internal
  because it is not reviewable per dependency, and the browser bundle's lock file can move a
  shipped transitive dependency under that header, an accepted leak.
- **The breaking marker is stripped on every internal surface by a local prefix**, and restored on
  the go and bun groups' majors by a local prefix. A shipped dependency's major keeps the marker as
  the mechanical read-before-deploying signal, and at 0.x it cuts a minor.
- **A version pinned in a file that never ships is `chore(internal-dependencies)`** whatever the
  tool is for, and the test PostgreSQL image and the preset pin are claimed by name.

## Gotchas

- **Empty parentheses are not a scope.** `feat():` parses as no scope, so a header that looks
  scoped would silently carry the empty scope's claim. commitlint rejects the spelling.
- **The gate and the release parser read different strings.** commitlint validates the header. The
  release parser reads the whole commit message and the pull request body: a body paragraph
  beginning with a header after a blank line becomes its own release entry, a `Release-As:` footer
  overrides the computed version, and a `BEGIN_COMMIT_OVERRIDE` block in the pull request body
  replaces the release-facing message. The `commit-taxonomy` job refuses all three.
- **Every legal type has a `changelog-sections` entry in `release-please-config.json`, and only
  those.** A type with no entry renders nothing, and a window holding only that type cuts no
  release, silently and green. The `commit-taxonomy` job asserts the equality.
- **A gated check is a silent pass.** The `commit-taxonomy` and `pr-title` jobs carry no `paths:`,
  `needs:` or `if:`. A skipped job reports `skipped`, and `skipped` satisfies a required status
  check.
