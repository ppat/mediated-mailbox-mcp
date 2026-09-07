# 0033. Timestamps on the client surface are UTC-only; non-UTC input is rejected, never converted

**Status:** Accepted ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)

## Context

Timestamps cross the client surface in both directions, and offset handling is where several
classes of time bugs live — silent defaulting, double conversion, ambiguous local times. The
canonical model already stores UTC internally
([ADR-0010](../provider/0010-one-provider-port.md)).

## Decision

- **Every timestamp entering or leaving the client surface
  ([ADR-0030](./0030-api-core-mcp-thin-adapter.md)) is ISO 8601 UTC, `Z` suffix.**
- **Input carrying any other offset is rejected loudly, with a clear error — never silently
  converted.** A non-UTC input is evidence the client misunderstands the contract, and silent
  conversion would hide that.
- This extends to the surface contract the rule the canonical model already applies internally.

## Alternatives considered

- **Accept any ISO 8601 offset and normalize to UTC.** No case was tabled for it in the
  originating agreement. Rejected: normalization is exactly the silent conversion that hides a
  client's misunderstanding of the contract.
- **Echo the resolved time back to the caller** (the companion practice in the advice this rule
  came from). Not taken: it serves scheduled publishing, and this system schedules nothing.

## Consequences

- The rule binds both roots at once: the API contract states it once, and the one-to-one MCP
  mirror ([ADR-0030](./0030-api-core-mcp-thin-adapter.md)) carries it unchanged.
- Assumptions about other components: the canonical model stores UTC internally
  ([ADR-0010](../provider/0010-one-provider-port.md)), so the surface serializes and parses — it
  never converts.
- The rejection rule is a control; its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
