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
  [ADR-0070](../docs/adr/engineering/0070-unit-comparison-through-one-options-value.md) and the
  golden-file helper.
- `property`, the generator report, failing-case store and operation sampler of
  [ADR-0069](../docs/adr/engineering/0069-property-and-crash-sequences-from-rapid.md).
- `crash`, the crash harness of [ADR-0045](../docs/adr/engineering/0045-crash-injection-testing.md).
- `fixture`, the synthetic fixtures.
- `postgres`, what an integration test package needs to reach its database, and the application of
  the bootstrap and the migration chain to an empty database that `pgrun` and the chain's own tests
  share.
- `mustnotcompile`, the helpers that assert a forbidden construction fails to compile, that a
  type exposes no field, that a type has exactly the fields or a function exactly the
  parameters named, that an interface has exactly the methods named with exactly their
  signatures, and that exactly the named methods of an interface return a given type, with exactly
  the results named.
- `analysis`, the `go vet` analysers, for ADR-0069's placement rules and ADR-0071's rule against
  package-level state in a pure core.
- `livecontract`, the guard a contract run against a real provider calls first, which skips the
  run unless `cmd/livecontract` started it, and the invocation that command runs
  ([ADR-0043](../docs/adr/engineering/0043-no-mocking.md)).
- `cmd/banproof` (the ban-proof script), `cmd/pgrun` (the integration run), `cmd/vetcheck` (the
  analysers' program), `cmd/mutproof` (the runner that records mutation demonstrations, of
  [ADR-0046](../docs/adr/engineering/0046-tests-are-evidence-once-seen-to-fail.md)) and
  `cmd/livecontract` (the one command that runs a provider's contract run against the real
  provider), each run through `go tool`.

The import rules are written per file, so packages importing the property-testing library and
packages every test may import sit in the one library without one reaching the other.
