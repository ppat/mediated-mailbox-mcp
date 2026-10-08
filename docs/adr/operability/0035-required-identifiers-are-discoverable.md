# 0035. Every identifier an operation requires is discoverable on the same surface; accounts gain a listing

**Status:** Accepted ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account)

## Context

Requiring an explicit account identifier on every operation is load-bearing for multi-account
isolation ([P3](../../../USE_CASES.md#p3--multi-account)), and it stays exactly as decided.
[ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md) decides it in
the words "There is no implicit current account. Omission is an error, not a default." But a
required parameter the caller cannot derive from the surface becomes a guessed, or hallucinated,
value.

## Decision

- **Every identifier a caller must supply is obtainable from some read operation on the same
  surface.** The rule is deliberately general. It binds every identifier kind the surface requires
  now and every kind added later. Introducing an operation that requires a new identifier kind
  means providing, in the same change, a read operation that supplies it. No further decision
  record is needed per identifier kind, because this one covers them all.
- **Current instances:** message identifiers come from the thread and message listings, label
  identifiers from a labels listing, and account identifiers from an accounts listing that returns
  each account's identifier and provider.

## Alternatives considered

- **Default the account identifier when there is only one account.** No case was tabled for it.
  Declined outright, because it contradicts the decided falsifier. An operation succeeding without
  an explicit account identifier falsifies [P3](../../../USE_CASES.md#p3--multi-account).

## Consequences

- The listing is part of the canonical API surface, so
  [ADR-0030](./0030-api-core-mcp-thin-adapter.md)'s one-to-one parity applies to it
  automatically.
- A granular per-client access model is deliberately not planned, and is a
  [non-outcome](../../../USE_CASES.md#non-outcomes). One bearer token grants all accounts, so the
  listings are trivial. If per-client token scoping ever arrives alongside multi-account, the
  listings return what the token can reach.
- Assumptions about other components: the sets the listings enumerate, which are the accounts and
  the account's labels, are enumerable by the service layer.
- The discoverability rule is proven as an acceptance criterion, held in
  [ROADMAP.md](../../../ROADMAP.md), not by a standalone injection. Its violation, shipping an
  operation that requires an identifier no read operation on the surface supplies, is a contract
  defect, refused when the surface contract is reviewed.
