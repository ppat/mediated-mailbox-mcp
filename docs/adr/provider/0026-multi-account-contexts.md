# 0026. Multi-account is one process holding N account contexts — isolation by construction

**Status:** Accepted ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account)

## Context

The system is multi-account by architecture with a single account deployed today. Accounts may
span organizations — a personal Gmail and a work mailbox with no common administrator — so nothing
may assume a shared tenant, credential, or admin. The deployment-shape question (one pod holding
all accounts versus one pod per account) is separate from the isolation property, which must hold
under either.

## Decision

**One process, one pod, N account contexts** — each context a self-contained bundle:

```python
@dataclass(frozen=True)
class AccountContext:
    account_id:     str              # "personal", "work"
    provider:       ProviderKind
    credential_ref: str
    policy_overlay: str | None
    mail:           MailProvider     # own authenticated client
    calendar:       CalendarProvider | None
    rate_profile:   RateLimitProfile        # provider cost model (ADR-0023)
    rate_limiter:   AdaptiveRateController  # shared via rate_state (ADR-0025)
```

The rules that keep accounts from bleeding:

- **`account_id` is required on every MCP tool.** There is no implicit current account; omission
  is an error, not a default.
- **Every table is partitioned or indexed on `account_id`**, all queries go through a repository
  layer that requires it, and row-level security stands behind that as a second, independent
  layer ([ADR-0016](../data/0016-schema.md)).
- **Per-account clients, never a shared pool.** A shared HTTP client with a mutable auth header is
  the classic way cross-account leakage happens under concurrency; giving each context its own
  authenticated client makes the bleed unrepresentable.
- **Policy composes as base plus overlay, and overlays only add restrictions** — a misconfigured
  overlay can over-restrict, never under-restrict.
- **No shared OAuth client, no domain-wide delegation, no assumed common admin**
  ([ADR-0011](./0011-gmail-auth-installed-app-oauth.md)) — each account is an independent grant.

Because only one account exists today, adding the second is scheduled as a deliberate
architectural test: if anything above the provider port needs changing to support it, the account
model was wrong — and finding that out cheaply is the point ([ROADMAP.md](../../../ROADMAP.md)
carries the unit).

## Alternatives considered

- **One pod per account.** Not rejected as a *deployment* option — the isolation property holds
  under it, and it remains available operationally. Rejected as the *architecture* because it
  would let per-account separation substitute for in-process isolation, leaving the code unsafe to
  ever co-locate accounts.
- **An implicit "current account" with a switch operation.** Rejected: ambient state plus
  concurrency is the recipe for acting on the wrong account; explicitness is enforced by making
  the parameter mandatory everywhere.
- **Shared OAuth app / domain-wide delegation across accounts.** Rejected in
  [ADR-0011](./0011-gmail-auth-installed-app-oauth.md); restated here because the account model is
  where it would have crept back in.

## Consequences

- Horizontal growth needs no redesign: a new account is a new context, credential, and partition,
  not new machinery.
