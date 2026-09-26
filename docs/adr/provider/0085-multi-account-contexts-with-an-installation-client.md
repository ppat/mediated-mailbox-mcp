# 0085. Multi-account is one process holding N account contexts, isolated by construction, each account its own grant through the installation's OAuth client

**Status:** Accepted (supersedes [ADR-0026](./0026-multi-account-contexts.md)) ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account)

## Context

The system is multi-account by architecture with a single account deployed today. Accounts may
span organizations, a personal Gmail and a work mailbox with no common administrator, so nothing
may assume a shared tenant, grant, or admin. The deployment-shape question (one pod holding all
accounts versus one pod per account) is separate from the isolation property, which must hold under
either. An installation connects its accounts of one provider through one OAuth client of its own
([ADR-0083](./0083-gmail-through-an-installation-oauth-client.md)), set up apart from the accounts
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)).

## Decision

**One process, one pod, N account contexts**, each context a self-contained bundle:

```python
@dataclass(frozen=True)
class AccountContext:
    account_id:     str              # "personal", "work"
    provider:       ProviderKind
    credential_ref: str              # the account's sealed grant in its row (ADR-0080)
    policy_overlay: OverlayRules | None  # the account's own rows in policy_rules (ADR-0016)
    mail:           MailProvider     # own authenticated client
    calendar:       CalendarProvider | None
    rate_profile:   RateLimitProfile        # provider cost model (ADR-0023)
    rate_limiter:   AdaptiveRateController  # shared via rate_state (ADR-0025)
```

The rules that keep accounts from bleeding:

- **`account_id` is required on every client-surface operation** (API endpoint or MCP tool).
  There is no implicit current account. Omission is an error, not a default.
- **Every table is indexed on `account_id`**, with the two exceptions
  [ADR-0016](../data/0016-schema.md) names (the op log, which carries no account column and is
  reached through its plan, and the base policy rows every account inherits). All queries go
  through a repository layer that requires the account, every statement against an account-keyed
  table carries an account predicate
  ([ADR-0047](../data/0047-schema-first-data-access.md)), and row-level security stands behind
  both as a third, independent layer.
- **Per-account clients, never a shared pool.** A shared HTTP client with a mutable auth header is
  the classic way cross-account leakage happens under concurrency. Giving each context its own
  authenticated client makes the bleed unrepresentable.
- **Policy composes as base plus overlay, and overlays only add restrictions.** A misconfigured
  overlay can over-restrict, never under-restrict.
- **No shared grant, no domain-wide delegation, no assumed common admin.** Each account is an
  independent grant, its own refresh token, revocable alone. The accounts of one provider in one
  installation connect through that installation's one OAuth client, which is shared, while no
  grant is.

Because only one account exists today, adding the second is scheduled as a deliberate
architectural test. If anything above the provider port needs changing to support it, the account
model was wrong, and finding that out cheaply is the point ([ROADMAP.md](../../../ROADMAP.md)
carries the unit).

## Alternatives considered

- **One pod per account.** Not rejected as a *deployment* option. The isolation property holds
  under it, and it remains available operationally. Rejected as the *architecture* because it
  would let per-account separation substitute for in-process isolation, leaving the code unsafe to
  ever co-locate accounts.
- **An implicit "current account" with a switch operation.** Rejected. Ambient state plus
  concurrency is the recipe for acting on the wrong account, and explicitness is enforced by
  making the parameter mandatory everywhere.
- **Domain-wide delegation across accounts.** Rejected in
  [ADR-0083](./0083-gmail-through-an-installation-oauth-client.md), and restated here because the
  account model is where it would have crept back in.
- **An OAuth client per account** (the superseded rule, which forbade a shared client). For it, no
  two accounts share anything at the provider. Against it, the person running the system repeats
  the whole client setup for every mailbox, and a shared client does not share a grant, which is
  what isolation needs.

## Consequences

- Horizontal growth needs no redesign, because a new account is a new context and a new grant
  rather than new machinery.
- The accounts of one installation draw on the one Cloud project their client belongs to, so the
  provider's per-project limits are shared across them.
