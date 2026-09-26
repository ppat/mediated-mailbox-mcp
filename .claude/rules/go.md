---
paths:
  - "**/*.go"
  - "go.mod"
  - "go.sum"
  - ".golangci.yaml"
---

# Rules for Go code

You are touching Go code or its configuration. The conventions are [CLAUDE.md's code
layout](../../CLAUDE.md#code-layout-and-conventions), and what tests the work needs is
[TESTING.md](../../TESTING.md). These are the tripwires:

- **One module at the root, a flat top level, and each component in its own directory** (CLAUDE.md,
  The Go module and Components). A new component gets a directory, import lists in `.golangci.yaml`
  together with its exclusion from the list over files outside every component, and a path-filter
  entry in each workflow that watches it.
- **Pure-core code sits under a directory named `core`**, and does no I/O and reads nothing ambient
  (ADR-0040). A deployable's composition root is its `main.go`, and everything else it holds sits
  under `internal/`, apart from an `importtarget` package holding a violation file (CLAUDE.md,
  Inside a component).
- **The environment is read only by a deployable's composition root**, which hands it to the
  configuration library in `settings/`. The `go vet` environment analyser refuses, anywhere else
  outside test files and `testsupport`, every standard-library function that returns an environment
  variable's value by a name its caller gives, or the environment as a whole, such as `os.Getenv`.
  Functions that read fixed platform variables for their own purpose, such as `os.UserHomeDir`, stay
  allowed (ADR-0078, CLAUDE.md, Static analysis and formatting).
- **Deployables never import each other**, and a component imports only the data-access subsections
  its import list names (ADR-0054, ADR-0066, ADR-0071).
- **Test files are named by kind** (`_property_test.go`, `_crash_test.go`, `_integration_test.go`
  with its build tag, which a crash-sequence file replaying against PostgreSQL also carries) and
  `testdata/` is per package (CLAUDE.md, Tests). Integration tests against PostgreSQL run only
  through `go tool pgrun`.
- **A finding of a linter standing in for a control is never suppressed**, at the line, by its own
  ignore comment, or in configuration. An ordinary linter's false positive is suppressed only as
  `//nolint:<linter> // <reason>` (ADR-0071, CLAUDE.md, Static analysis and formatting).
- **Every lint ban and import rule keeps its violation file, and `go tool banproof` stays green.** A
  new list or ban lands with its violation file (ADR-0046).
- **No mocks, no container library, and no `Equal` method added for a test**
  (ADR-0043, ADR-0068, ADR-0070). Comparisons pass the shared options from `testsupport/compare`.
