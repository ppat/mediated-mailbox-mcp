# 0092. A key is replaced by delta sync re-sealing what it opens, and the old key retires once a scan finds nothing left on it

**Status:** Accepted ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

Stored credentials and each stored OAuth client's secret are sealed to a public key and name that
key ([ADR-0081](./0081-credentials-sealed-to-a-public-key.md),
[ADR-0088](./0088-credentials-sealed-with-hpke-x-wing.md)). A key is replaced now and then, and
every stored value then has to be sealed again to the new key before the old private key can go.
Re-sealing needs the plaintext, so only a process holding the private key can do it, and only the
four deployables that call a provider hold it
([ADR-0079](./0079-secrets-arrive-as-mounted-files.md)). Each of those processes serves every
account ([ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md)), and
delta sync runs every five minutes ([ADR-0022](./0022-four-workloads.md)) and opens every account's
credential and each stored client secret each time it loads its accounts
([ADR-0090](./0090-accounts-reach-deployables-as-reloaded-snapshots.md)).

## Decision

- **The deployables that open credentials hold a keyring**, every private key mounted for them, and
  look a value's key up by the identifier in its header. Every sealer, the UI and those
  deployables, seals to one current public key.
- **Delta sync re-seals.** A value it opens with a key that is not the current one is sealed again
  to the current key and written through the compare-and-set of
  [ADR-0089](./0089-sealed-values-written-by-compare-and-set.md). No other process re-seals, and no
  role or command exists for it.
- **Delta sync scans on every run.** It reports one series for each account `accounts` lists and one
  for each row of `oauth_clients`. A provider with no OAuth client has no row and no series. A
  series reads 1 while its value is sealed to a key other than the current one or cannot be opened,
  and 0 once it is sealed to the current key or when there is no value to seal, for an account
  that is not connected. How
  delta sync's series reach the metrics scrape when a run ends between scrapes is D4's open
  decision, and the gate below waits on its answer
  ([ROADMAP.md](../../../ROADMAP.md#open-decisions)). The UI logs the key identifier it seals to at
  start.
- **A key is replaced in this order.**
  1. A new key pair is generated with the key-generation binary attached to the release
     ([ADR-0088](./0088-credentials-sealed-with-hpke-x-wing.md)).
  2. The new private key is mounted beside the old one in every deployable that opens credentials.
  3. The public key is replaced in the UI and in those deployables.
  4. The UI and those deployables are restarted, because a key file is read at start.
  5. The operator waits until the UI logs the new identifier and delta sync reports a series for
     every listed account and for every row of `oauth_clients`, all reading 0. A missing series
     never counts as 0.
  6. Only then is the old private key removed, and the deployables that open credentials are
     restarted, so none holds it any longer.

## Alternatives considered

- **Every opener re-seals what it opens.** For it, a value is re-sealed on its first use by anyone.
  Against it, each opener's role would need a write on the client secrets, beyond the credential
  write ADR-0082 grants, and delta sync already opens every value on each run.
- **A command that re-seals every account under a role of its own.** For it, key replacement would
  not wait on a sync run. Against it, the command would need the private key that only the four
  deployables hold, and a role reading every account, which
  [ADR-0075](../data/0075-one-runtime-role-per-deployable.md) gives no process that is not a
  deployable.
- **Retire on silence, when no deployable reports an old key.** For it, no series per account.
  Against it, a value no process opens reports nothing, and the gate would pass while it is still
  sealed to the old key.

## Consequences

- Key replacement takes at least one run of delta sync after the restart, and nothing is removed
  while any listed account or row of `oauth_clients` has no series or reads 1.
- A value that cannot be opened keeps the old key in service until the operator re-authorizes that
  account or sets up that OAuth client again.
- Delta sync's role writes the client secrets as well as the credential
  ([ADR-0075](../data/0075-one-runtime-role-per-deployable.md)). The shared library
  [`accountload/`](../../../accountload/README.md) carries the re-seal.
- Assumptions about other components. Delta sync runs for every account on its schedule. The
  platform can mount two private key files at once and restarts a deployable when told to.
