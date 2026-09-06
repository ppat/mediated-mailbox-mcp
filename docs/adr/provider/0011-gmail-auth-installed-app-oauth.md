# 0011. Gmail auth: per-account installed-app OAuth with `gmail.modify` — domain-wide delegation rejected

**Status:** Accepted ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [A2](../../../USE_CASES.md#a2--no-destructive-action-on-sensitive-mail), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

The mediator needs durable, headless access to a Gmail mailbox with label-mutation rights. Google
offers three routes, and the account model requires that accounts may span organizations with no
shared administrator.

## Decision

| Route | Verdict |
| --- | --- |
| **Installed-app OAuth, offline refresh token** | **Chosen.** Consent once per account; minimal blast radius; revocable per account |
| Service account + domain-wide delegation | **Rejected** — a Workspace-wide skeleton key, and incoherent with accounts spanning organizations |
| Service account without delegation | Non-viable — cannot access user mailboxes at all |

Scope selection is a security decision in itself:

- **`gmail.modify` is required.** Label mutation is the point, and `gmail.readonly` +
  `gmail.labels` does not permit *applying* labels to messages.
- **Do not request `gmail.settings.*` or full `https://mail.google.com/`** — the latter grants
  IMAP access and permanent delete. The absence of permanent-delete capability from the token is a
  real guarantee, not a policy choice: it is one of the two structural halves of
  [ADR-0019](../mutation/0019-asymmetric-mutation.md)'s "never permanently delete."

Operational gotcha: **publish the OAuth consent screen to "In production."** Testing-mode refresh tokens expire on a 7-day clock — a
silent, delayed failure that presents as the mediator losing mailbox access a week after everything
worked.

## Alternatives considered

The route table above is the alternatives analysis; the two rejected rows carry their reasons.
Within the chosen route, requesting broader scopes "to be safe" was rejected because every scope
the token lacks is a capability no compromise of the mediator can exercise.

## Consequences

- Each account is an independent grant: revoking one revokes one, and no organization-level
  administration is assumed anywhere.
- The token can never permanently delete mail, regardless of any code bug — the tool surface's
  half of that guarantee is [ADR-0019](../mutation/0019-asymmetric-mutation.md).
- Refresh-token durability becomes a load-bearing operational concern; its handling is
  [ADR-0013](../operability/0013-credentials-and-rotation-writeback.md).
