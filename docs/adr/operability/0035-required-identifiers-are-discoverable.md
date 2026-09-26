# 0035. Every identifier an operation requires is discoverable on the same surface; accounts gain a listing

**Status:** Accepted ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account)

## Context

Requiring an explicit account identifier on every operation is load-bearing for multi-account
isolation ([P3](../../../USE_CASES.md#p3--multi-account); decided in
[ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md): "There is no
implicit current account. Omission is an error, not a default"), and it stays exactly as decided.
But a required parameter the caller cannot derive from the surface becomes a guessed —
hallucinated — value.

## Decision

- **Every identifier a caller must supply is obtainable from some read operation on the same
  surface.** The rule is deliberately general: it binds every identifier kind the surface
  requires now and every kind added later. Introducing an operation that requires a new
  identifier kind means providing, in the same change, a read operation that supplies it — no
  further decision record per identifier kind; this one covers them all.
- **Current instances:** message identifiers come from the thread and message listings; label
  identifiers from a labels listing; account identifiers from an accounts listing that returns
  each account's identifier and provider.

## Alternatives considered

- **Default the account identifier when there is only one account** (the advice this corollary
  came from). No case was tabled for it in the agreement beyond the advice itself. Declined
  outright: it contradicts the decided falsifier — an operation succeeding without an explicit
  account identifier falsifies [P3](../../../USE_CASES.md#p3--multi-account).

## Consequences

- The listing is part of the canonical API surface, so
  [ADR-0030](./0030-api-core-mcp-thin-adapter.md)'s one-to-one parity applies to it
  automatically.
- A granular per-client access model is deliberately not planned — a
  [non-outcome](../../../USE_CASES.md#non-outcomes): one bearer token grants all accounts, so the
  listings are trivial. If per-client token scoping ever arrives alongside multi-account, the
  listings return what the token can reach.
- Assumptions about other components: the sets the listings enumerate — the accounts, the
  account's labels — are enumerable by the service layer.
- The discoverability rule is proven as an acceptance criterion riding the unit that builds the
  read surface (its home in [ROADMAP.md](../../../ROADMAP.md)), not by a standalone injection:
  the violation — shipping an operation that requires an identifier no read operation on the
  surface supplies — is a contract defect, refused when the surface contract is reviewed.
