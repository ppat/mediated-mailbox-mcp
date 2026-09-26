# 0081. A stored credential is sealed to a public key the UI holds, and only the deployables that call a provider can open it

**Status:** Accepted ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

Accounts and their provider credentials live in the database, created and repaired through the UI
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)). The credential is a
full-mailbox grant, so it is stored encrypted following standard practice. Two kinds of process
touch it. The UI writes it when an account is connected or re-authorized. The deployables that call
a provider read it on every use, and write it back when the provider rotates it. Only the second
kind needs to read it.

## Decision

- **The credential is sealed with public-key encryption**, and so is the secret of an
  installation's OAuth client, for a provider that has one. The UI holds only the public key. It seals a credential or a client
  secret when it stores one and cannot open a stored one. The deployables that call a provider hold
  the private key to open them, and the public key to seal a rotated credential before writing it
  back ([ADR-0082](./0082-rotation-writeback-to-the-database.md)).
- **The construction is an authenticated one, as standard practice has it**, so an altered sealed
  credential is refused rather than opened.
- **The keys are secrets delivered as mounted files**
  ([ADR-0079](./0079-secrets-arrive-as-mounted-files.md)). Only the deployables that call a
  provider can read the private key's file.
- **A sealed credential records the key it was sealed to**, so a key can be replaced while
  credentials sealed to the old one are still stored.
- **The code that seals and opens is the narrow shared library `credential/`**, which argues its
  own case in its README ([ADR-0050](../engineering/0050-shared-code-pure-or-narrow.md)).

## Alternatives considered

- **One symmetric key shared by the UI and the deployables that call a provider.** For it, the
  simplest standard construction, one key and one operation each way. Against it, the UI could
  open every stored credential, so a compromised UI would read every mailbox's grant.
- **Encryption inside PostgreSQL.** No case was tabled for it. The key would have to reach the
  database, and [ADR-0060](../engineering/0060-no-code-in-the-database.md) keeps the system's logic
  out of it.

## Consequences

- A compromised UI cannot read back a stored credential. It still sees a credential in plaintext
  while it completes a connection or a re-authorization, since it runs that exchange
  ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)).
- Losing the private key loses every stored credential and every OAuth client's secret. The
  operator recovers by setting up each OAuth client again and re-authorizing each account through
  the UI.
- The construction is [ADR-0088](./0088-credentials-sealed-with-hpke-x-wing.md)'s, and how a key
  is replaced is [ADR-0092](./0092-key-replacement-by-keyring-and-re-seal.md)'s.
- Assumptions about other components. The platform delivering the key files keeps the private key
  away from every deployable that does not call a provider.
