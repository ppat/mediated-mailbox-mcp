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
  (ADR-0040). The core checks match a `core` path element at any depth, and each package of a family
  keeps its own `core`, such as `process/dbconnect/core`, never one pooled for the family. A
  deployable's composition root is its entry package `<deployable>/app`, which its `main.go` calls,
  and everything else it holds sits under `internal/`, apart from the entry and an `importtarget`
  package holding a violation file (CLAUDE.md, Inside a component).
- **The environment is read only by a deployable's `main.go`**, which hands it through the entry
  package to the configuration library in `process/settings/`. The `go vet` environment analyser
  refuses, anywhere else outside test files and `testsupport`, every standard-library function that
  returns an environment variable's value by a name its caller gives, or the environment as a whole,
  such as `os.Getenv`. Functions that read fixed platform variables for their own purpose, such as
  `os.UserHomeDir`, stay allowed (ADR-0078, CLAUDE.md, Static analysis and formatting).
- **A shell logs through the logger it is handed.** A deployable's `main.go` builds the logger and
  its level variable through `process/logging`, makes the logger the process default for code the
  project does not own, and hands both to the entry, which sets the level from `log_level` and
  passes the logger to every shell that logs. A `forbidigo` ban refuses, in every Go file, reading a
  default logger through `slog`'s or `log`'s package-level functions and the built-in `print` and
  `println` (ADR-0122, CLAUDE.md, Inside a component).
- **A generated data-access function runs only inside the transaction helper.** The `go vet`
  txhelper analyser enforces it, by the rules and the three exempt statements CLAUDE.md states under
  Static analysis and formatting (ADR-0047). The base policy's statements run only in the helper's
  base-policy transaction, and no other subsection's do (ADR-0112).
- **The UI registers a route only through its recording mux.** In a non-test file under `ui/`, the
  `go vet` routes analyser refuses a use of a mux's `Handle` or `HandleFunc`, of `http.Handle` or
  `http.HandleFunc`, or of an interface's or type parameter's method of the same name and
  parameters, outside the recording mux's own `Handle`. Registration through reflection, by an
  imported package on the default mux, or through an interface method one of whose parameter types
  is a type parameter is left to review (ADR-0071, CLAUDE.md, Static analysis and formatting).
- **The worker's job code starts a goroutine only through `schedule.Go`.** The `go vet` goroutines
  analyser refuses, in a non-test file under `worker/` outside `worker/internal/schedule`, a `go`
  statement and any use of a function that starts a goroutine running what it is given, the
  standard library's and those of `golang.org/x/sync`, because a panic is recovered only on the
  goroutine that raised it, and one on any other goroutine would stop every job. Such a function
  reached through an interface value or reflection, a dependency that calls what it was given on a
  goroutine of its own, and any other module's goroutine starter are left to review (ADR-0119,
  CLAUDE.md, Static analysis and formatting).
- **No code of this project imports `unsafe`**, a `go:linkname`'s included, **or takes an unsafe
  pointer from `reflect`**. The `go vet` unsafeimport analyser refuses an import of `unsafe` and any
  use of `reflect.NewAt`, `reflect.SliceAt`, `(reflect.Value).UnsafePointer` and
  `(reflect.Value).SetPointer` in every non-test file of the module, because code isolation per job
  kind rests on memory safety. A use through an interface value or reflect's own method lookup, and
  writing the process's own memory through the operating system, such as through `/proc/self/mem`,
  are left to review. The worker's own test holds the dependencies of its build that import `unsafe`
  or hold assembly or a `.syso` object to the list it names (ADR-0117, CLAUDE.md, Static analysis
  and formatting).
- **Deployables never import each other**, and a component imports only the data-access subsections
  its import list names (ADR-0054, ADR-0066, ADR-0071).
- **Test files are named by kind** (`_property_test.go`, `_crash_test.go`, `_integration_test.go`
  with its build tag, which a crash-sequence file replaying against PostgreSQL also carries) and
  `testdata/` is per package (CLAUDE.md, Tests). Integration tests against PostgreSQL run only
  through `go tool pgrun`.
- **Code that ships reaches only packages `./...` lists**, beside the standard library and other
  modules' packages in the module cache. No lint reads any other package, and banproof's unlinted
  check refuses an import of one from anything but a test file (ADR-0071, CLAUDE.md, Static
  analysis and formatting).
- **Every non-test Go file that builds without a tag is one the gating lint and vet read.** A file
  whose build constraint excludes a tag `.golangci.yaml` sets, such as `//go:build !integration`, or
  that builds only for another operating system or architecture, or only without cgo, is refused by
  the same check. The `go vet` step's `-tags` stay equal to `.golangci.yaml`'s build tags, which
  banproof checks (ADR-0071).
- **A finding of a linter standing in for a control is never suppressed**, at the line, by its own
  ignore comment, or in configuration. An ordinary linter's false positive is suppressed only as
  `//nolint:<linter> // <reason>` (ADR-0071, CLAUDE.md, Static analysis and formatting).
- **Every lint ban and import rule keeps its violation file, and `go tool banproof` stays green.** A
  new list or ban lands with its violation file (ADR-0046).
- **No mocks, no container library, and no `Equal` method added for a test**
  (ADR-0043, ADR-0068, ADR-0070). Comparisons pass the shared options from `testsupport/compare`.
