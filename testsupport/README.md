# testsupport

A narrow, named shared library, published as `mediated-mailbox-testsupport`. Shared code is pure, or
it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Tests in many components need the same tooling, and most of it does I/O. The failing-case store and
the golden-file helper write files, and the integration run starts a container. None of it reaches
an image, because only test files and the tooling programs import it. Its packages are these.

- `compare`, the shared comparison options of
  [ADR-0070](../docs/adr/engineering/0070-unit-comparison-through-one-options-value.md), and its
  golden-file helper, `Golden`. It compares `got` against the file at `testdata/golden/name` in the
  calling test's own package directory, through the shared options, and fails a golden file that
  does not exist rather than creating one. A name that would leave `testdata/golden/` is refused.
  Run with `-update`, it writes `got` to that file, byte for byte, instead of comparing. `GoldenAt`
  is the same for a recorded file read outside the recording test's package, at a path relative to
  that package's directory that may leave it, and refuses an absolute or unclean path. The two share
  the one `-update` flag this package registers.
- `property`, the generator report, failing-case store and operation sampler of
  [ADR-0069](../docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md), and the one
  recorder that stands in for a test where a failure is read rather than reported, which the crash
  harness shares.
- `crash`, the crash harness of [ADR-0045](../docs/adr/engineering/0045-crash-injection-testing.md).
- `marker`, the marker text of
  [ADR-0044](../docs/adr/engineering/0044-synthetic-fixtures-marker-text.md) that fixtures and
  generated mail values carry.
- `fixture`, the synthetic fixtures.
- `postgres`, what an integration test package needs to reach its database, the application of the
  migration chain to an empty database that `pgrun` and the chain's own tests share, and the chain
  applied up to one migration, for a test that stores rows under the chain before a migration and
  then applies it ([ADR-0048](../docs/adr/data/0048-forward-only-migrations.md)).
- `mustnotcompile`, the helpers that assert a forbidden construction fails to compile, that a
  type exposes no field, that a type has exactly the fields or a function exactly the
  parameters named, that an interface has exactly the methods named with exactly their
  signatures, that exactly the named methods of an interface return a given type, with exactly
  the results named, and that exactly the named functions, methods and package-level variables of a
  package can hold a value carrying a given type, through anything but a function type.
- `analysis`, the `go vet` analysers, for these rules.
  - [ADR-0069](../docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md)'s placement
    rules.
  - [ADR-0071](../docs/adr/engineering/0071-static-enforcement-toolchain.md)'s rules against
    package-level state in a pure core, against a route the UI registers other than through its
    recording mux, against a statement the API server, the UI or the worker runs other than through
    the data-access library, against a call of the connections' attach to the Session outside the
    connections' package, and against a call of the connections' re-seal outside delta sync's
    package.
  - [ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)'s
    rule against reading the environment outside a deployable's or the migrator's `main.go`.
  - [ADR-0047](../docs/adr/data/0047-schema-first-data-access.md)'s rule that every generated
    data-access function runs inside the transaction helper.
  - The goroutines rule, which refuses, in the non-test code under `api/`, `ui/` and `worker/`, a
    `go` statement and any use of a function that starts a goroutine running what it is given, so
    that code starts one only through a helper that recovers it. It refuses a call of neither
    helper, so which one code uses, `schedule.Go` in the worker's job code and the process's `Go`
    in the API server's and the UI's, is held by review. So is a goroutine a thing the process
    shares starts, which goes through the process's `Go`, since the rule does not read the process
    family's code
    ([ADR-0119](../docs/adr/operability/0119-the-workers-jobs-are-scheduled-from-recorded-state.md),
    [ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)).
  - [ADR-0117](../docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)'s rule
    that no code of this project imports `unsafe` or takes an unsafe pointer from `reflect`.
- `helmchart`, the render of the chart under `packaging/chart` with the pinned `helm` and the chart's
  required values, which the chart's own tests and each deployable's test of the file the chart
  renders for it share, with the reads of the values schema's keys and a configuration type's keys
  those tests compare ([ADR-0052](../docs/adr/engineering/0052-kubernetes-deployment-helm-chart.md)).
- `alertingrules`, the run of `promtool test rules` over a package's test file and the check that
  every series the chart's alerting rules read is one the package emits, which each package that
  emits a series a rule reads shares ([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)).
- `livecontract`, the guard a contract run against a real provider calls first, which skips the
  run unless `cmd/livecontract` started it, and the invocation that command runs
  ([ADR-0043](../docs/adr/engineering/0043-no-mocking.md)).
- `cmd/banproof` (the ban-proof script), `cmd/pgrun` (the integration run), `cmd/vetcheck` (the
  analysers' program), `cmd/mutproof` (the runner that records mutation demonstrations, of
  [ADR-0046](../docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md), and with
  `-check` the check that every mutation patch applies at one place only),
  `cmd/livecontract` (the one command that runs a provider's contract run against the real
  provider) and `cmd/testkinds` (the selection of each kind of test for its workflows, and the
  check that every test runs in its kind's workflows and no other, of
  [ADR-0124](../docs/adr/engineering/0124-each-kind-of-test-runs-in-a-workflow-of-its-own.md)),
  and `cmd/keypaircheck` (the check that a key pair a released key-generation binary writes is one
  the credential family seals to and opens with, which the release workflow runs, of
  [ADR-0088](../docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)), each run
  through `go tool`. `cmd/keypaircheck` is the one part of this library that links the credential
  family, `credential/`, and its import list admits that family to it alone.

The import rules are written per file, so packages importing the property-testing library and
packages every test may import sit in the one library without one reaching the other.
