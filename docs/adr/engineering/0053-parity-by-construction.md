# 0053. Both client roots are generated from one operation registry — a one-sided operation is unrepresentable

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released),
[G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)

## Context

[ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md) fixes the surface's shape — one
canonical API with an MCP tool set mirroring it one-to-one, parity exact in both directions —
and makes parity verifiable by violation: a one-sided operation must fail a check. A check
verifies diligence after the fact; the stronger form available is to remove the way a one-sided
operation could be written at all. No off-the-shelf machinery generates one surface from the
other in either direction, so the mechanism is small code this project owns either way.

## Decision

- **One operation registry, in code, is the single source of the client surface.** The API's
  contract document and the MCP tool set are both generated from it. An operation exists on
  both roots or on neither — a one-sided operation is not caught but unrepresentable, which is
  the unconstructability pillar applied to the surface itself.
- **Hand-registering an operation on either root, outside the registry, fails the build.** The
  parity check [ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md) demands becomes a
  check on the generator's monopoly rather than on two hand-maintained lists agreeing.
- **The registry carries what both roots need** — each operation's schema and identity — so
  the generated tool set registers programmatically from data.

## Alternatives considered

- **Two hand-maintained surfaces with a parity check.** The case for it: no generator to write,
  and [ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md)'s rows already demanded the
  check. Rejected: the check catches drift after it is written; generation removes the place
  drift could be written, and the check that remains (the generator's monopoly) is smaller than
  the check it replaces.
- **Generating one root from the other** (the tool set from the contract document, or the
  reverse). The case for it: reuses a standard artifact as the source. Rejected: no
  off-the-shelf generator exists in either direction, so this is own-code either way — and a
  registry both roots consume keeps the source symmetric instead of privileging one root's
  format as the master.

## Consequences

- Adding an operation is one registry entry; the surface cannot gain an operation any other
  way. Anything deliberately beyond parity (operational endpoints, per
  [ADR-0030](../operability/0030-api-core-mcp-thin-adapter.md)) lives outside the registry and
  is bounded by the same structural layering check as everything else.
- Assumptions about other components: the MCP side's tooling accepts programmatic, data-driven
  tool registration.
