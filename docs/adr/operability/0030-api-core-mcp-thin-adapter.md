# 0030. The serving layer is an API; MCP is a thin protocol adapter over it

**Status:** Accepted ·
**Pillar:** [One gate, N dumb adapters](../../../DESIGN.md#one-gate-n-dumb-adapters) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)

## Context

The component stack already separates the tool surface from enforcement — the Redaction Gate sits
below it. That seam was internal: MCP was the only way in, so every document equated "client"
with "the agent over MCP." Making the seam a real, independently callable API lets clients choose
their protocol and makes future clients possible without server changes — added extensibility and
utility on the client side, with the rest of the design untouched.

## Decision

**One deployment, two roots, one service library.** A single Kubernetes Service/Deployment
exposes both the API (e.g. `/api`) and the streamable-HTTP MCP endpoint (e.g. `/mcp`). Both roots
are thin protocol adapters over one shared service library.

- **Neither frontend contains business or enforcement logic, and neither reaches below the
  service layer.** Enforcement — the Redaction Gate and Mutation Authorizer — lives in the shared
  library beneath both roots, so no frontend can drift on redaction. This rule is structural and
  testable, like the no-body-columns rule.
- **The API is the canonical definition of the surface**: one endpoint per operation, defined
  once, with an OpenAPI document as the contract. The MCP tool set mirrors it one-to-one.
  **Parity is exact in both directions** — anything beyond parity (operational endpoints, say) is
  a separate future decision. The OpenAPI contract and the tool descriptions are contract-grade
  text, reviewed like code.
- **Same transport posture on both roots**: bearer-token authentication, TLS — same controls
  applied to both (API and MCP) identically.
- **Every client of the serving surface is untrusted.** The trust posture keys on "any client,"
  not "the agent": the agent over MCP today; any caller of the API tomorrow. Every control that
  assumed a persuadable agent assumes a hostile client generally.
- **Approval stays out of the entire client-facing surface's vocabulary.** A caller with the
  bearer token can reach the API directly, so the plan-approval transition must not exist as an
  API endpoint any more than it exists as an MCP tool. The operator's interface keeps writing
  directly to the database ([ADR-0021](../mutation/0021-approval-surface.md)), unchanged.

## Alternatives considered

- **Two deployments — an API service, plus a separate MCP service calling it over the network.**
  Rejected: the second deployment buys almost no isolation (the MCP wrapper holds no provider
  credentials and no database access either way; compromising it yields exactly what the bearer
  token already grants) while doubling the operational surface — two Deployments, Services,
  certificates, and network policies plus an extra hop, which is negative value under the
  single-operator constraint. The single shape also fits the existing one-pod architecture
  ([ADR-0026](../provider/0026-multi-account-contexts.md)) and is the smallest shape that ships.
  The single-funnel-by-construction property the split offers is achieved instead by the
  no-logic-in-frontends rule above — and if a real reason to split ever appears, the migration is
  mechanical, because the API is already the boundary.
- **MCP only, as before.** Rejected by this decision's purpose: it welds the one seam that gives
  clients a choice and future clients a path.

## Consequences

- The MCP adapter joins the provider adapters in the deliberately-dumb category: protocol
  translation only, zero redaction responsibility — on the client side of the line instead of the
  provider side. The one-gate pillar's line is held below *every* client surface.
- Both roots funnel through the same service library, so rate limiting spends from the same
  per-account budget and every serve, denial, and mutation writes the same audit rows regardless
  of protocol.
- A command-line client that talks to the API becomes possible without any server change; it is
  deliberately not planned.
- Assumptions about other components: the service library exposes the complete operation set
  (frontends add nothing); [ADR-0014](./0014-lan-only-transport.md)'s listener-level controls
  cover both roots; [ADR-0021](../mutation/0021-approval-surface.md)'s approval path remains
  database-direct and is the only approval path.
- Surface parity is verifiable by violation: introducing a one-sided operation on either root
  must fail the parity check — catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
