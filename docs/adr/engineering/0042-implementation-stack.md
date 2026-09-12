# 0042. The stack is Go end to end on the server, TypeScript only in the browser

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

No other document in the set chooses a language. The choice is the least reversible decision in
the project — the shared libraries make per-component language migration a fiction — and the
requirements pull in different directions: a type system that can make unsafe states
unconstructable, a testing ecosystem that carries property-based, mutation, and crash-injection
machinery, minimal hardened images for the process holding full-mailbox credentials
([ADR-0028](../operability/0028-trust-anchor-hardening.md)), a delivery posture that sizes units
in days, and an eventual open-source audience.

## Decision

- **Go, for all six deployables and the shared libraries — including the UI's server side.**
- **TypeScript in the browser, and only there:** a strict-mode single-page app, bundled by bun,
  its types generated in CI from the read API's contract so drift fails the build, and shipped
  as static files served by the UI's Go binary. No JavaScript server runtime runs in
  production.
- **Python is sanctioned in exactly two supporting roles, taken only if the need is determined
  necessary at that point in time:** offline model training and experimentation (never
  deployed), and a single-purpose embedding workload — its own deployable that other components
  know only through its contract — if Go's local-inference path proves too awkward when that
  work arrives. The default assumption is Go plus browser TypeScript.
- **The conventions that make Go carry the unconstructability pillar** — adequacy by design
  rather than by hope, and part of this decision rather than folklore:
  - every sensitivity-carrying type has unexported fields and smart constructors, so
    construction outside the blessed path is a compile error and unavailable at runtime;
  - the zero value of every sensitivity-carrying and verdict type is its most restrictive
    state, so an uninitialized value fails toward deny — the language's best-known type hazard
    aligned with the design's chosen failure direction;
  - every default branch in verdict handling denies, so an unhandled new variant fails toward
    deny as well;
  - a fixture package that attempts the forbidden construction, asserted to fail compilation,
    stays in CI forever;
  - exhaustiveness, import-boundary, and unchecked-error linters run in CI as deputized
    enforcement for what the compiler does not check natively, with the tools chosen in
    [ADR-0071](./0071-static-enforcement-toolchain.md).
- **The browser framework choice within the TypeScript layer is deliberately not made here.**
  It is [ADR-0063](./0063-browser-app-is-preact-with-signals.md)'s.

## Alternatives considered

- **Rust.** The case for it: it is the one language where the project's central sentence is
  literally true — a restricted body that cannot be constructed, a verdict the compiler refuses
  to leave unhandled, a core crate that cannot import I/O because its build graph has none; the
  guarantees are shown in types rather than told in conventions. Rejected on two arguments.
  First, redundancy: the type-level ceiling protects against plumbing mistakes that the
  design's layered absences — no body columns
  ([ADR-0016](../data/0016-schema.md)), the port's split metadata and body methods and its
  metadata-only fetch shape ([ADR-0010](../provider/0010-one-provider-port.md)), fail-closed
  defaults — each independently also guard, so the increment lands on multiply-reinforced
  layers; whereas its costs (the slowest iteration of the candidates, concentrated exactly in
  the shell code that the foundation, data-flow, and mutation work mostly is) land on the
  outcomes nothing else backs up — a project that ships and stays shipped. Second, asymmetry under
  inversion: regretting Go is a bounded, discoverable annoyance in which any actual leak still
  requires the same logic bug in the same gate code; regretting Rust is day-sized units bending
  multi-day across the whole schedule — the stall failure that kills solo projects.
- **TypeScript everywhere.** The case for it: one language from the redaction gate to the
  approval button, the protocol's reference SDK, the largest UI ecosystem, the highest agent
  fluency. Rejected: it is weakest on the two vectors the design calls load-bearing. Its types
  are erased at runtime, so the runtime half of the unconstructability obligation would need a
  permanently maintained parallel validation layer — the untaken-not-unconstructable shape the
  design refuses wherever it can. And it puts a JavaScript runtime plus a deep third-party
  package tree inside the trust anchor — the process holding full-mailbox credentials — drawn
  from the ecosystem with the worst supply-chain incident record of the candidates. The benefit
  purchased lands in the least safety-critical component and is substantially replaced by
  contract-generated types.
- **Python.** The case for it: the best property-based testing tool in any ecosystem, the
  native home of the ML tail, maximal iteration speed. Rejected: the weakest static guarantee
  of the four on the project's most emphasized commitment, the hardest path to minimal hardened
  images, and everything it is best at is either available elsewhere at slight discount or
  detachable into the narrow sanctioned workload above — while nothing it is worst at is
  detachable.

## Consequences

- The repository carries two languages, with the second confined to the browser, where no
  server language reaches anyway. The shared-library prohibition on a different-language UI
  therefore bites only at the browser boundary, and is bridged there by contract-generated
  types whose drift is a build failure.
- The unconstructability commitment holds at package granularity, with the conventions above
  carrying what the compiler does not check natively — and both of the language's classic
  weaknesses (zero values, non-exhaustive handling) deliberately degrade toward deny.
- Every major library pick beyond the JavaScript toolchain remains its own decision at
  implementation time; this record chooses the platform, not the tools.
- The code sketches that landed records carry fix the shape of a contract, never its
  implementation language — a frozen-dataclass sketch of a verdict type
  ([ADR-0009](../redaction/0009-scanner-verdicts-carry-no-content.md) among them) reads as the
  structure this record's conventions now realize in Go's terms.
- Assumptions about other components: the UI's read API
  ([ADR-0021](../mutation/0021-approval-surface.md)) exposes a generated contract for the
  browser layer to type against; the sanctioned embedding workload, if ever taken, stays its
  own deployable that other components know only through its contract; verdicts are the values
  [ADR-0040](./0040-pure-core-decisions-as-values.md) makes them.
