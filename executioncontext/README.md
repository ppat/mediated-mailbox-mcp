# executioncontext

A family, one narrow, named shared library holding several packages of one concept, published as
`mediated-mailbox-executioncontext`. Shared code is pure, or it is a library like this one that
argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Its concept is the [execution context](../DESIGN.md#glossary), what a job or a request executes
with: the account snapshot with its opened credentials, the account session over it, the policy
snapshot, and the sealed credentials they are opened from. The decision it hides is how a process
loads, holds and reloads what it executes with, so a read it cannot trust never replaces what it
holds, and how the credentials inside are opened, rotated and handed back. These sit together
because they change together. The account snapshot and the credential code have changed in the same
pull requests, the policy snapshot is loaded, held and reloaded exactly as the account snapshot is,
and the same composition roots link both. The account session is the repeated assembly of the
concept, the steps every provider-calling root repeated around the snapshot.

| Package | Holds |
| --- | --- |
| `accountload/` | The account snapshot, the compare-and-set write-back of a rotated credential, and delta sync's re-seal |
| `session/` | The account session every provider-calling composition root holds its accounts' provider connections through |
| `policyload/` | The policy snapshot's loading, and the reload-failure alarm |
| `credential/` | The sealing and opening of a credential, the loading of its keys, the configuration section naming the key files in `credential/core`, and the key-generation command in `credential/cmd/keygen` |

Each package is admitted to each component by its own entry in that component's import list, and
each has a list of its own in `.golangci.yaml`, so joining this family never widens what any
component may import. A package added here without a list of its own falls under the list over files outside every
component, which admits only the standard library.

**What does not belong.** The rate limiter, whose leases are shared across processes and which
changes for different reasons, is `ratelimit/`'s. Editing or importing the policy is the UI's and
`db/policyrules`'s, never the policy loader's, which only holds a snapshot. Scanning is
`core/scan`'s, and no provider adapter belongs here, since a composition root passes the session a
connector per provider. The failure to watch is the session growing policy, scanning or leasing code,
or the policy loader growing behaviour unrelated to holding a snapshot. Either shows first as an entry
its import list does not hold.

The family connects to the database as no role of its own. Its statements run under the role of each
deployable that imports the package running them
([ADR-0075](../docs/adr/data/0075-one-runtime-role-per-deployable.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)), which `db/check` tests. When a
process reloads is its caller's. Each loader loads when asked and swaps only on a read it can trust.

## The account snapshot, `accountload`

The mediator, backfill, delta sync and the reorg workload each hold their accounts, each paired
with the OAuth client it connects through where its provider has one
([ADR-0106](../docs/adr/provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)),
and the opened credentials as one account snapshot
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).
Each writes a rotated credential back by compare-and-set on the bytes it last knew
([ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md)), and delta
sync re-seals what it opens with a key that is not the current one and reports the scan the key's
retirement waits on
([ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)). This package
is that loading, that write-back and that re-seal. The sealing and opening themselves are
[`credential`'s](#the-sealed-credentials-credential).

The case for one package over per-deployable code is rules a copy could get wrong without anyone
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
  client, since it knows its adapters, and the package stays blind to which they are. An account of
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

The listing comes from `db/accounts` and each account's credential from `db/accountstate/credential`
([ADR-0091](../docs/adr/data/0091-accounts-listed-apart-from-their-state.md)). The re-seal of an
OAuth client's secret writes through a compare-and-set its caller supplies, because only delta
sync's role may write a client secret ([ADR-0016](../docs/adr/data/0016-schema.md)), and a
statement this package ran would be planned under the role of every deployable that imports it.
Delta sync supplies the write from `db/oauthclients/secret`, which only its list admits. A unit of
work takes the snapshot once.

## The account session, `session`

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
of this family, because it hides a decision of the same concept, how a process holds an account's
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

Its import list, `session` in `.golangci.yaml`, admits `accountload`, `core/mail`, `db/tx`,
`db/accountstate/authentication`, the database driver's root package and its `pgtype` package, which
the recording's statement takes, and the standard library. The recording runs under the role of each
deployable whose list admits the package. No adapter, no rate limiter, no policy and no scanner
belong in it. The limiter at each account's lowered target stays in each composition root, since it
belongs to the rate budget rather than to the account's credential.

## The policy loader, `policyload`

Backfill, the mediator, delta sync, the reorg workload and the heuristics workload each hold the
active policy as one immutable snapshot and replace it only with an update that validates
([ADR-0041](../docs/adr/engineering/0041-policy-as-immutable-snapshots.md)). The validation, the
composition of the base policy with an account's overlay and the atomic swap are pure and sit in
`core/policy`. What is left is impure and the same in every one of them. It reads the base rows and
each account's overlay rows from the policy tables, hands them to the pure half, and raises the
reload-failure alarm when a reload fails, whether the read or the validation.

The case for one package over per-deployable glue is one rule the glue could get wrong. The pure half
accepts a policy with no rules as valid. A read of the tables that failed, or returned nothing
because of a fault, looks like that empty policy. Treated as one, it would classify the senders the
active policy lists as normal and release content the previous snapshot withheld. So the package tells
a read it cannot trust apart from a policy that is empty, and a read it cannot trust never replaces the
active snapshot. Written once, that rule holds in every deployable that loads policy. Written five
times, one copy that gets it wrong fails open.

The alarm is the other shared part. Each loading process emits the same series, and one alerting rule
reads it ([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)), so the
series is defined once, in this package.

| Series | Kind | Emitted by | Read by |
| --- | --- | --- | --- |
| `mediated_mailbox_policyload_reload_failed` | Gauge, 1 while the latest reload failed and 0 once one succeeds | Each loading process, on the registry it passes in | `MediatedMailboxPolicyReloadFailed` in `packaging/chart/alerting-rules.yaml`, whose promtool tests sit in `policyload/testdata/` |

The series is a gauge rather than a counter because a process first loads policy as it starts,
usually before its first scrape, and a rule over a counter whose first sample is already 1 sees no
increase.

Its statements are in `db/policyrules`. Which accounts it reads is its caller's. A process whose
accounts change between its account snapshots sets the new ones, and the next reload reads them
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).
It reads each account's own rules and the base rules in one statement, and a reload whose accounts
read different base rules reads every account once more. Only when the second read's accounts
disagree too is the read one it cannot trust, so an edit to the base policy landing midway never
composes accounts from two versions of it, and pages only when edits keep landing through both reads
([ADR-0114](../docs/adr/engineering/0114-a-torn-base-policy-read-is-read-again-before-it-fails-the-reload.md)).
A reload stopped because its caller cancelled it keeps the active policy and raises no alarm, and one
that ran out of time is a failed read like any other.

## The sealed credentials, `credential`

An account's provider credential is stored in the database sealed to a public key
([ADR-0080](../docs/adr/data/0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0081](../docs/adr/operability/0081-credentials-sealed-to-a-public-key.md)). The UI seals a
credential when an account is connected or re-authorized. Backfill, the mediator, delta sync and the
reorg workload open it with the private key, and seal a rotated one before writing it back
([ADR-0082](../docs/adr/operability/0082-rotation-writeback-to-the-database.md)). The UI's
`ui/internal/clientsecret` opens an OAuth client's secret with the same keys, for a consent's code
exchange, and nothing else. These packages are that sealing and opening, the loading of each key
from its mounted file, and the configuration section naming those files with its validation, in
`credential/core`.

The case for one copy over per-deployable code is that a mistake here fails open or loses every
mailbox. A copy that accepted an altered credential, opened one sealed to a key it should not hold,
or sealed to the wrong key would do it silently, and five copies would drift. Written once, the
construction and its refusals hold in every deployable that touches a credential.

Its packages are cut so an import list can admit the sealing half without the opening half, so the
UI's code outside its one opening part links no code that opens a value. `seal` seals to the current
public key. `open` holds the keyring of private keys and opens a value by the key its header names.
`core` holds the configuration section naming the key files a deployable loads. `cmd/keygen` writes a
key pair, since no standard tool writes X-Wing keys. The construction and the sealed value's bytes
are [ADR-0088](../docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)'s, and how a key
is replaced is
[ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)'s. Every write of
a sealed value by a deployable is the compare-and-set of
[ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md).

| Path | Holds |
| --- | --- |
| `credential/seal/` | The public key, the sealed value's bytes and the contexts a value is bound to. It never imports `credential/open/`, and since `credential/open/` imports it the compiler refuses the reverse as a cycle |
| `credential/open/` | The private keys, the keyring, opening, the refusals and the re-seal |
| `credential/core/` | The credential section of a deployable's configuration, naming the mounted public key file and every private key file, and its validation ([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)). It is the credential code's own pure core ([ADR-0040](../docs/adr/engineering/0040-pure-core-decisions-as-values.md)) |
| `credential/cmd/keygen/` | The key-generation command |

The values below are part of the sealed format. They feed the key identifier, the HPKE key
schedule or the additional data of every stored value, so changing any of them makes every value
already stored unopenable. A change to one takes a new version byte and a re-seal of every stored
value ([ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)).

| Value | Where it is used |
| --- | --- |
| The domain string `mediated-mailbox credential key identifier` followed by a zero byte | Hashed with the public key into the key identifier |
| The HPKE info string `mediated-mailbox credential` | HPKE's info parameter |
| The purpose spellings `account credential` and `oauth client secret` | The additional data, naming what the value holds |
| A four-byte big-endian length before the purpose and before the row | The additional data, so no two contexts encode alike |

`credential/cmd/keygen` takes two flags, both required. `-private-key-file` names the file the
32-byte seed is written to with mode 0600, and `-public-key-file` the file the 1216-byte public key
is written to with mode 0644. Each file holds the raw key and nothing else, and the command refuses a
path that already exists, so a key in use is never overwritten. It prints the new key's identifier.
