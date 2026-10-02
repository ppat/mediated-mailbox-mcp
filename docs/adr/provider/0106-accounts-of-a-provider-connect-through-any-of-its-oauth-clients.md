# 0106. An installation holds any number of OAuth clients for a provider, and each account connects through one of them, shared with other accounts or its own

**Status:** Accepted (supersedes [ADR-0083](./0083-gmail-through-an-installation-oauth-client.md),
jointly with [ADR-0107](./0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)) ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [P3](../../../USE_CASES.md#p3--multi-account), [O6](../../../USE_CASES.md#o6--deployable)

## Context

Where a provider authenticates through an OAuth client, as Gmail does
([ADR-0107](./0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)), every grant an
account holds is issued to one client and works only with it. The person running the installation
sets each client up apart from any account
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)). Accounts may span
organizations with no common administrator
([ADR-0085](./0085-multi-account-contexts-with-an-installation-client.md)).

How many clients an installation holds, and which accounts use each, is a choice of its own. The
operator ruled on 2026-10-02 that, within one provider, connecting several accounts through one
client and connecting accounts through different clients must both be options, and that a client
never spans providers.

## Decision

- **An installation holds any number of OAuth clients for each provider that authenticates through
  one, and each client belongs to exactly one provider.** The person running the installation names
  each client, and the name is unique in the installation and never changes. A provider's client
  identifier is held by one client only, so one client is never set up twice under two names.
- **Each account of such a provider connects through exactly one of that provider's clients.**
  Several accounts may share a client, or an account may have one of its own. The person chooses
  the client when connecting the account. The account's token source is built from that client and
  the account's own credential, and from no other client.
- **Which client an account connects through is part of what the account is**, held with its
  identifier and provider in `accounts`
  ([ADR-0091](../data/0091-accounts-listed-apart-from-their-state.md)). The reference carries the
  provider, so an account cannot name a client of another provider. An account of a provider
  without an OAuth client names none. The schema does not know which providers use a client, so
  that an account of such a provider names one is held by the account setup that writes it and by
  the account snapshot that loads it, which serves no such account without its client.
- **An account moves to another client only through a re-authorization through that client.** A
  grant works only with the client it was issued to, so the new client and the new credential are
  written in one transaction.
- **The UI removes a client no account connects through, and a client an account connects through
  cannot be removed.** Replacing a client's identifier breaks only the accounts connected through
  that client.

## Alternatives considered

- **One client per provider for the whole installation**
  ([ADR-0083](./0083-gmail-through-an-installation-oauth-client.md)). For it, the client is set up
  once and nothing is chosen when connecting an account. Against it, the operator ruled that
  connecting accounts through different clients must be an option.
- **A client for every account**, the rule [ADR-0026](./0026-multi-account-contexts.md) carried.
  For it, no two accounts share anything at the provider. Against it, the person repeats the whole
  client setup for every mailbox, and a shared client does not share a grant. Under this record an
  account may still have a client of its own.
- **A client serving several providers.** Not viable. A client is issued by one provider and
  authenticates only to it, and the operator ruled it out.
- **The client an account uses held in `account_state`, beside the credential.** For it, the grant
  and the client it was issued to sit in one row. Against it, the installation screens read no
  account's state ([ADR-0056](../operability/0056-ui-organized-around-the-operators-work.md)), and
  they must name the accounts a client serves before it is replaced or removed. The account
  snapshot also builds each account's token source from what it lists
  ([ADR-0090](../operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).

## Consequences

- Client setup runs once per client. A second client of a provider is set up the same way as the
  first.
- The accounts that share a client draw on the one Cloud project it belongs to, so the provider's
  per-project limits are shared across them. Accounts on different clients in different projects
  share none.
- The schema of [ADR-0016](../data/0016-schema.md) keys `oauth_clients` on the client's name, and
  `accounts` names each account's client. The account snapshot pairs each account with its own
  client ([ADR-0090](../operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).
- A client's sealed secret is bound to its row
  ([ADR-0088](../operability/0088-credentials-sealed-with-hpke-x-wing.md)), so the change of the
  table's key from the provider to the name re-seals the stored secret once. A name never changes,
  so nothing re-seals it again. The UI's role gains delete on `oauth_clients` for removing an
  unused client ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)).
- Assumptions about other components. The provider refuses a credential presented with a client
  other than the one it was issued to, so an account paired with the wrong client fails to
  authenticate rather than acting through it.
