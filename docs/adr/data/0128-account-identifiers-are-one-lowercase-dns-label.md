# 0128. Account identifiers are one lowercase DNS label, which the UI's proposal, its server check and the database hold alike

**Status:** Accepted ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account)

## Context

An account's identifier, `accounts.account_id`, is the text the operator gives the account at
account setup ([ADR-0080](./0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)). It spreads further than any
other value the system holds.

- It is a key column in 17 tables besides `accounts`, two of them append-only, `audit_log`
  ([ADR-0016](./0016-schema.md)) and `policy_changes`
  ([ADR-0102](../mutation/0102-policy-changes-recorded-in-an-append-only-history.md)),
  and no reference to it cascades an update ([ADR-0016](./0016-schema.md)).
- It is bound into the sealed credential's additional data, so changing it means sealing the
  credential again by a process holding the private key
  ([ADR-0088](../operability/0088-credentials-sealed-with-hpke-x-wing.md)).
- It is the `account` label on most metric series, the mediator's paths
  `/api/accounts/{account_id}/…` and every MCP call's `account_id`
  ([ADR-0087](../operability/0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)),
  the first segment of the UI's URLs and so of every bookmark, row-level security's `app.account`
  setting, and the candidate rule identifier `candidate.{account}.{domain}`.
- The first identifier becomes permanent at production point 1, since every record of the account
  is keyed on it and nothing rewrites stored accounts
  ([ADR-0048](./0048-forward-only-migrations.md)).

The database once refused only an identifier exactly `.`, `..` or `/`, which the mediator's API
root cannot hold as a path segment
([ADR-0087](../operability/0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)),
and the UI's server refused those and the UI's own top-level words. Spaces, capitals, apostrophes, `?`, `#`, `%`, a `/` inside the string, dots,
control characters and any length passed both. The UI already proposed a slug from the mailbox's
local part, so the design held a grammar as a proposal only, and the operator could edit the
proposal into anything.

## Decision

- **The grammar.** An account identifier matches `^[a-z0-9]([a-z0-9-]{0,61}[a-z0-9])?$`: one to
  63 lowercase ASCII letters, digits and hyphens, starting and ending with a letter or a digit, the
  grammar of one DNS label. A leading digit is allowed.
- **Three sides hold it alike.** The database's check on `accounts.account_id`
  ([ADR-0016](./0016-schema.md)), the UI's server check at account setup, and the connect page's
  proposal accept the same set of identifiers on the grammar.
  - The UI's server alone also refuses the words its own top-level paths use, which the database
    does not know, so every account's screens can be reached.
  - Every identifier the UI proposes is one its server accepts, so the UI never refuses an
    identifier it proposed. How the proposal keeps to that is
    [docs/UI.md section 8.12](../../UI.md#812-connect-an-account-and-re-authorize)'s.
- **One shared table of identifier cases proves the agreement**
  ([docs/VERIFICATIONS.md](../../VERIFICATIONS.md)).

## Alternatives considered

- **Free text, checked only for what routing needs.** The database refused only `.`, `..` and
  `/`, ADR-0087's addressability, and the server those and the top-level words. The case made for it: no outcome fails for a
  spaced or capitalised identifier, since the mediator routes it (a request to
  `/api/accounts/Peter%27s%20Gmail/messages` reaches the account `Peter's Gmail`), the UI encodes
  it as one path segment, and row-level security and Prometheus accept any string. It lost on
  legibility in labels, URLs and prompts, on the ambiguity of `candidate.{account}.{domain}` once
  an identifier may hold a dot, and because the first identifier typed is permanent, so the
  asymmetric payoff favours a few lines of check now.
- **A surrogate key with a display name.** The case made for it: renames become free. Rejected
  because it touches every table, every statement and both surfaces, for a rename nobody has
  asked for.
- **A grammar requiring a leading letter, with the proposal changed so it never starts with a
  digit.** No case was tabled for it beyond being the other answer to whether an identifier may
  start with a digit. The operator ruled against it on 2026-10-07, since the proposal already
  produces an identifier starting with a digit, a mailbox `2fa@…` proposing `2fa`.

## Consequences

- **Assumed of the mediator.** Its router and its account check accept any identifier the grammar
  allows, and every such identifier is one path segment its API root can address
  ([ADR-0087](../operability/0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)).
- **Assumed of row-level security and the metrics.** `app.account` and the `account` label take
  any string, so the grammar narrows nothing they rely on.
- **Assumed of the UI's top-level words.** They are a closed list that tests pin
  ([docs/UI.md section 5](../../UI.md#5-information-architecture-and-the-url)), so a word the UI
  starts using later is a reviewed change, checked against the accounts already stored.
- **An account stored outside the grammar** stops the migration that adds the database's check,
  and nothing rewrites it, since an account is system-of-record data
  ([ADR-0048](./0048-forward-only-migrations.md)). That fails closed before production point 1.
- **A rename stays expensive**, as the Context lists, and is argued here if it is ever wanted.
