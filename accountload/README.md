# accountload

A narrow, named shared library, published as `mediated-mailbox-accountload`. Shared code is pure,
or it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

The mediator, backfill, delta sync and the reorg workload each hold their accounts, each paired
with the OAuth client it connects through where its provider has one
([ADR-0106](../docs/adr/provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)),
and the opened credentials as one account snapshot
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).
Each writes a rotated credential back by compare-and-set on the bytes it last knew
([ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md)), and delta
sync re-seals what it opens with a key that is not the current one and reports the scan the key's
retirement waits on
([ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)). This library
is that loading, that write-back and that re-seal, and the account session in `session` that
holds each account's provider connection over them, argued below. The sealing and opening themselves are
[`credential/`](../credential/README.md)'s.

The case for one library over per-deployable code is rules a copy could get wrong without anyone
noticing.

- **A read it cannot trust never replaces the snapshot.** A reload whose read fails keeps the
  previous snapshot, and one that succeeds with no account serves none. A copy that treated a
  failed read as an empty list, or kept stale accounts after an empty one, would serve the wrong
  set.
- **A write-back never puts back a value someone else replaced.** Every write of a sealed value is
  a compare-and-set on the bytes the process last read or wrote, and a reload keeps a rotated
  credential whose write-back failed only while the stored bytes are still those. A hand-over from
  a unit of work that started before the loader adopted a value someone else stored, by a reload
  or a re-read, is discarded under the same lock that adopted it. The process's own write-backs
  never discard a hand-over, so overlapping rotations each land. A copy that wrote
  unconditionally, or that compared only the bytes it knows now, would put a revoked grant back
  after the operator re-authorized the account, and one that discarded on its own earlier
  write-back would lose a live rotation.
- **An OAuth client is optional per provider, and required where the provider uses one.** An
  account whose provider authenticates without one loads without one, and nothing looks for a
  client that does not exist. The deployable names the providers that authenticate through a
  client, since it knows its adapters, and the library stays blind to which they are. An account of
  such a provider that names no client, or whose client's secret did not open, loads as not
  connected. A copy that assumed every provider has a client would refuse an account that needs
  none, and one that left the refusal to each deployable would serve such an account wherever a
  deployable forgot it.
- **An account reaches only the client it names.** Each account carries the client its row in
  `accounts` names, and the snapshot offers no lookup of a client by provider or by name. A copy
  that took a provider's first or only client would pair an account with a client its grant was
  never issued to, and would look right for as long as each provider had one client. A re-read
  after a refusal pairs the credential with the client the account names at the time of the
  re-read.
- **Only delta sync re-seals, and its scan never passes on silence.** It reports a series for every
  account and every stored client secret, and a missing series never reads as done.

Four copies of these rules would drift, and one that got any of them wrong would fail silently.
Written once, they hold in every deployable that calls a provider.

The library connects to the database as no role of its own. Its statements run under the role of
each deployable that imports it
([ADR-0075](../docs/adr/data/0075-one-runtime-role-per-deployable.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)). The listing comes from
`db/accounts` and each account's credential from `db/accountstate/credential`
([ADR-0091](../docs/adr/data/0091-accounts-listed-apart-from-their-state.md)). The re-seal of an
OAuth client's secret writes through a compare-and-set its caller supplies, because only delta
sync's role may write a client secret ([ADR-0016](../docs/adr/data/0016-schema.md)), and a
statement this library ran would be planned under the role of every deployable that imports it.
Delta sync supplies the write from `db/oauthclients/secret`, which only its list admits.

When a process reloads is its caller's. The library loads when asked and swaps only on a read it
can trust. A unit of work takes the snapshot once.

## The account session

The mediator, backfill and delta sync each call a provider for an account in units of work, a body
request, a page of a pass, a tick. Each pairs the account with the credentials the snapshot opened,
holds a token source over them, reads a refused credential again from the account's row and makes
the call once more over a source built from a replaced one, and at the end of a unit hands the
source's credential over with the adoption stamp of the credential it was built from
([ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md)) and records
the latest authentication attempt
([ADR-0097](../docs/adr/operability/0097-authentication-outcome-reported-by-the-adapter-recorded-by-the-deployable.md)).
Three copies of these steps in three composition roots drifted once, when backfill handed over with
no adoption stamp while every other hand-over carried one. `session` writes them once, as a package
of this library, because it hides a decision of the same concept, how a process holds an account's
credential while it uses it.

It is one mechanism with two parameters, so it never branches on which deployable calls it.

- **How long a held source lives.** For one unit of work, or across units while the stored
  credential is the one the source was built from or the one it rotated to. The mediator holds
  across requests, so an access token the provider issued serves the requests after it. Backfill
  holds for its run, which is the same rule over one run's life. Delta sync builds a fresh source for
  each tick.
- **Which units run concurrently.** Only the mediator runs concurrent units for one account, so the
  holder takes a lock around what it holds, which costs the batch callers nothing.

The rules a copy could get wrong without anyone noticing are these.

- **A hand-over names the stamp of the credential its source was built from**, the snapshot's when
  the unit opened, or a re-read's once the unit goes on with the re-read credential. A copy that
  named no stamp, or the stamp the loader holds now, would let a unit that started before the
  operator re-authorized put the revoked grant back.
- **A refused credential is read again before the refusal stands, and the call is made once more
  only over a different credential**, paired with the client the account names at the time of the
  re-read. A re-read that finds the refused credential, or none, returns the refusal. A copy that
  retried over the same credential would spend a call the provider refuses, and one that kept the
  old client would build a source the new grant was never issued to.
- **A source and its port change only together.** When the port cannot be built over a re-read
  credential, the unit keeps the source and the stamp it had, so its hand-over is discarded under
  the stamp the re-read moved rather than writing the refused credential back.
- **Only a recorded attempt is recorded.** A source that made no attempt records nothing, and the
  statement keeps a later attempt another deployable recorded.

What each caller supplies keeps its own decisions its own. It passes the account loader it owns, so
when snapshots reload stays the caller's
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)), a
database handle the attempt is recorded through, and one connector per provider it links. A
connector is built in the composition root from the adapter's constructors over plain values, the
client's identifier and secret and the credential, so which adapters a deployable links stays its
root's decision and no adapter learns what an account is. The session picks the connector by the
account's provider, and an account whose provider has no connector is skipped and logged in one
place. Each call the caller makes through the session leases inside itself, so the session never
sees the rate limiter.

Its import list, `accountload-session` in `.golangci.yaml`, admits this library's root package,
`core/mail`, `db/tx`, `db/accountstate/authentication`, the database driver's root package and its
`pgtype` package, which the recording's statement takes, and the standard library. The recording
runs under the role of each deployable whose list admits the package, which `db/check` tests. No
adapter, no rate limiter, no policy and no scanner belong in it, and code for any of them would show
first as an entry its list does not hold. The limiter at each account's lowered target stays in each
composition root, since it belongs to the rate budget rather than to the account's credential.
