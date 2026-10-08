# 0081. A stored credential is sealed to a public key, only code that calls a provider opens an account's credential, and one isolated part of the UI opens an OAuth client's secret

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

An installation's OAuth client's secret is stored the same way. Google's token endpoint refuses an
installed-app client's code exchange without the client's secret, PKCE notwithstanding, so the UI,
which runs the consent's code exchange
([ADR-0107](../provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)),
needs the secret of the client a consent was issued to.

## Decision

- **The credential is sealed with public-key encryption**, and so is the secret of each of the
  installation's OAuth clients, for a provider that has them. The UI seals a credential or a client
  secret when it stores one. The deployables that call a provider hold the private key to open them,
  and the public key to seal a rotated credential before writing it back
  ([ADR-0082](./0082-rotation-writeback-to-the-database.md)).
- **Only code that calls a provider opens an account's credential.** The UI's code never opens
  one. That is held by the UI's import lists and by what its one opening part can open, not by
  keeping the key from the UI. Inside the worker that runs every background job kind
  ([ADR-0117](./0117-one-background-worker-runs-every-job-kind.md)), a job kind that calls no
  provider, heuristics today, never opens one either. That is held by its own import list, which
  admits no opening code, by its role holding no grant on a sealed credential, and by its entry
  constructor taking only what the job needs, not by keeping the key from the worker.
- **One isolated part of the UI opens an OAuth client's secret, and nothing else.** The secret stays
  in its one sealed column and is never stored in a second form. The UI holds the same private keys
  the deployables that call a provider hold, and one package of the UI, `ui/internal/clientsecret`,
  opens with them a value bound to the client-secret purpose and to a client's row, for a consent's
  code exchange. Its one operation takes a client's name and no sealing context, so it cannot be
  asked to open an account's credential, and a credential's bytes copied into a client's row fail to
  open, since the purpose is bound into the value
  ([ADR-0088](./0088-credentials-sealed-with-hpke-x-wing.md)). The UI's import lists admit the
  opening half of the credential code, and the read of a client's sealed secret, to that package
  alone. They admit to the rest of the UI's shipped code only the packages under `crypto/` it
  uses, so it links none of the public-key code an opening is built on. The project's own `go vet`
  analyser refuses a statement the UI's shipped code runs other than through the data-access
  library ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)), so the role's read of
  the secret stays with that package too. The plaintext secret lives for the exchange that asked
  for it, and for client setup's check of a secret the operator brought back, and reaches no log
  line, no response and no cookie.
- **The construction is an authenticated one, as standard practice has it**, so an altered sealed
  credential is refused rather than opened.
- **The keys are secrets delivered as mounted files**
  ([ADR-0079](./0079-secrets-arrive-as-mounted-files.md)). The deployables that call a provider and
  the UI read the private key's file.
- **A sealed credential records the key it was sealed to**, so a key can be replaced while
  credentials sealed to the old one are still stored.
- **The code that seals and opens is `executioncontext/credential/`**, packages of the execution
  context family, a narrow shared library that argues its own case in its README
  ([ADR-0050](../engineering/0050-shared-code-pure-or-narrow.md)).

## Alternatives considered

- **One symmetric key shared by the UI and the deployables that call a provider.** For it, the
  simplest standard construction, one key and one operation each way. Against it, the UI's code
  would open every stored value through the operation it seals with, with nothing telling an
  account's credential from a client's secret.
- **A key pair of the UI's own for client secrets, apart from the credentials' key.** For it, a
  compromised UI process would hold no key that opens a refresh token. Not chosen. The decision
  keeps one key, and isolates the UI's access by code.
- **The client's secret stored a second time, in a form the UI reads.** For it, no private key in
  the UI. Not chosen. The decision keeps the secret in its one sealed column.
- **Encryption inside PostgreSQL.** No case was tabled for it. The key would have to reach the
  database, and [ADR-0060](../engineering/0060-no-code-in-the-database.md) keeps the system's logic
  out of it.

## Consequences

- The UI's code cannot open a stored credential. Its import lists and its opening part's one
  operation hold that, and a credential's bytes injected into a client's row are proven refused. The
  UI still sees a credential in plaintext while it completes a connection or a re-authorization,
  since it runs that exchange
  ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)).
- **The cost.** A compromised UI process holds the private key that opens every stored refresh
  token, since it is the same key. The guarantee that the UI opens no credential is its code's,
  not key custody, so code an attacker runs inside the UI's process is held by neither. Its
  database role still never reads a stored credential
  ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)), so the key yields a
  refresh token only with a credential's bytes taken from somewhere else.
- **What stays with review.** The import lists and the analyser hold the UI's code to its one
  opening part by what it links and what statements it runs. Code that sets out to get around them
  is held by review alone. Such code would open a value with primitives written from scratch, or
  reached through `go:linkname` or `unsafe`, with the key files whose paths the composition root
  holds, or run a statement through the paths the analyser leaves to review, which
  [ADR-0071](../engineering/0071-static-enforcement-toolchain.md) lists. Test code is outside both
  rules, since it is never served. The same holds for a job kind of the worker that calls no
  provider. Its import list holds it to the code it may link, and code that sets out to read the
  process's memory or its key files is held by Go's memory safety and review, which is why the
  worker's build carries no cgo and no dependency added for a job uses `unsafe`
  ([ADR-0117](./0117-one-background-worker-runs-every-job-kind.md)).
- Losing the private key loses every stored credential and every OAuth client's secret. The
  operator recovers by setting up each OAuth client again and re-authorizing each account through
  the UI.
- The construction is [ADR-0088](./0088-credentials-sealed-with-hpke-x-wing.md)'s, and how a key
  is replaced is [ADR-0092](./0092-key-replacement-by-keyring-and-re-seal.md)'s.
- Assumptions about other components. The platform delivering the key files keeps the private key
  away from every process that neither runs code that calls a provider nor is the UI.
