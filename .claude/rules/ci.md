---
paths:
  - ".github/workflows/**"
  - ".github/renovate.json"
  - ".github/scripts/**"
  - "commitlint.config.js"
  - "release-please-config.json"
  - "package.json"
  - "bun.lock"
  - "mise.toml"
  - "mise.lock"
  - "**/Dockerfile"
  - ".pre-commit-config.yaml"
  - "packaging/**"
  - "tests/chainsaw/**"
---

# Rules for CI, images, packaging and tool pins

You are touching a workflow, an image, the chart, the chainsaw suite, or a tool pin. The layout is
[CLAUDE.md's code layout](../../CLAUDE.md#code-layout-and-conventions), from Static analysis and
formatting to Tools and versions. These are the tripwires:

- **A path-filtered workflow's filter is an allow list of the directories it watches** (ADR-0054). A
  new component adds its entry to every workflow that watches it, and the Go workflows also watch
  every Go file.
- **Go lint covers the whole module whenever its paths match**, never only the changed lines or
  packages (ADR-0071).
- **Integration tests against PostgreSQL run only under `go tool pgrun` with `-tags integration`**,
  and a gating property run sets `RAPID_NOFAILFILE=true`, passes no `-short`, and takes its case
  count and seed from the environment (ADR-0068, ADR-0069).
- **Every workflow line that runs `go test` passes what `go tool testkinds args <kind> <level>`
  prints**, so each kind of test runs in its own workflows and in no other, and `go tool testkinds
  check` in the lint workflow refuses any other `go test` line, any flag it adds beyond `-race`,
  `-count`, `-v` and `-timeout`, and a run asked for only narrowed. A test workflow holds one role
  and its triggers alone decide when it runs, with no job or step condition, no
  `continue-on-error`, no `GOFLAGS` and no job deciding for another. Only `gmail-contract` has a
  line naming the livecontract command or its marker variable. The check reads workflow text only
  for these forms and parses neither YAML nor the shell, so a step whose shell, error handling,
  environment or selection is changed by any other means, `set +e` or a `GOENV` file for example,
  is left to review, as is a `go test` reached through a variable, a script or another program
  (ADR-0124).
- **The chainsaw workflow builds no images**, takes a version, and runs on packaging and suite
  changes only (ADR-0052).
- **Every release builds, pushes and signs every image and the chart at the release version**
  (ADR-0049, ADR-0052). Each image is built from the repository root with its own Dockerfile.
- **Every tool is at its latest version unless a document says otherwise.** The tools this project's
  own jobs and hooks run are pinned in `mise.toml`, with `mise.lock` committed in the lock format
  CI's mise reads (CLAUDE.md, Tools and versions).
- **The `commit-taxonomy` and `pr-title` jobs carry no `paths:`, `needs:` or `if:`**, and none may
  be added, and the `commit-messages` job carries only the condition that is true on every pull
  request event: a skipped job satisfies a required status check. The commit vocabulary, the pairing
  rule and what the closure check proves are `.claude/rules/commits.md`'s (ADR-0073).
- **Every tool a local pre-commit hook runs also runs in a CI workflow**, and generated, recorded
  and fixture files stay excluded from the text fixers and formatters (CLAUDE.md, Static analysis
  and formatting).
