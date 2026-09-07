# 0031. Every mutating operation is dry-runnable — a preflight that writes nothing

**Status:** Accepted ·
**Serves:** [A1](../../../USE_CASES.md#a1--asymmetric-mutation), [A3](../../../USE_CASES.md#a3--bulk-change-is-reversible)

## Context

Agents call destructive operations happily. A preview that costs nothing builds trust and catches
mistakes before they land.

## Decision

**Every mutating API endpoint and MCP tool accepts a dry-run mode.** A dry-run reports what would
change, which operations the Mutation Authorizer would refuse, and whether a batch would fail
whole — and makes **zero state changes of any kind**: no provider call that mutates, no index
write, no audit row, nothing. If it changed any state, it would not be a dry-run.

Scope: mutating operations only. Reads have no side effect to preview, so they are out of scope.

## Alternatives considered

- **Dry-run on every operation, reads included.** Considered in the originating discussion and
  scoped down: a read has no side effect to preview, so a read's dry-run would report nothing its
  normal response does not.
- **No dry-run mode.** Rejected: without a preview, a client's first sight of a mistake is its
  effect.

## Consequences

- The mutating surface's trust story improves where agents need it most: a client can see the
  full effect and every refusal before committing to anything.
- Assumptions about other components: the Mutation Authorizer's verdicts are computable without
  side effects (its matrix is a pure check,
  [ADR-0019](./0019-asymmetric-mutation.md)), and the whether-a-batch-fails-whole preview
  evaluates the same all-or-nothing rule the real call enforces — dry-run and real execution
  share one evaluation path, or the preview lies.
- The writes-nothing property is a control; its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
