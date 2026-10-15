# executioncontext

A family, one narrow, named shared library holding several packages of one concept, published as
`mediated-mailbox-executioncontext`. Shared code is pure, or it is a library like this one that
argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Its concept is the [execution context](../DESIGN.md#glossary), what a job or a request executes
with, which is the account snapshot with its opened credentials, the account session over it, the
policy snapshot, and the sealed credentials they are opened from. The decision it hides is how a
process loads, holds and reloads what it executes with, so a read it cannot trust never replaces
what it holds, and how the credentials inside are opened, rotated and handed back. These sit
together because they change together. The account snapshot and the credential code have changed in
the same pull requests, the policy snapshot is loaded, held and reloaded exactly as the account
snapshot is, and the same composition roots link both. The account session is the repeated assembly
of the concept, the steps every provider-calling root repeated around the snapshot.

| Package | Holds |
| --- | --- |
| `accountload/` | The account snapshot, the compare-and-set write-back of a rotated credential, and delta sync's re-seal |
| `session/` | The account session every provider-calling composition root holds its accounts' provider connections through |
| `policyload/` | The policy snapshot's loading, and the reload-failure alarm |
| `credential/` | The sealing and opening of a credential, the loading of its keys, and the configuration section naming the key files in `credential/core` |
| `assembly/` | A process's one execution context, built once over the other packages, and its one reload |

Each package is admitted to each component by its own entry in that component's import list, and
each has a list of its own in `.golangci.yaml`, so joining this family never widens what any
component may import. A package added here without a list of its own falls under the list over
files outside every component, which admits only the standard library.

**What does not belong.** The rate limiter, whose leases are shared across processes and which
changes for different reasons, is `ratelimit/`'s. Editing or importing the policy is the UI's and
`db/policyrules`'s, never the policy loader's, which only holds a snapshot. Scanning is
`core/scan`'s, and no provider adapter belongs here, since a composition root passes the session a
connector per provider. The failure to watch is the session growing policy, scanning or leasing
code, or the policy loader growing behaviour unrelated to holding a snapshot. Either shows first as
an entry its import list does not hold.

The family connects to the database as no role of its own. The statements of the account snapshot,
the policy loader and `assembly` run under the role of the pool a composition root builds the
process's execution context over, the API server's own in the API server and delta sync's in the
worker, and the account session's recording runs under the role of each component that opens a
session ([ADR-0118](../docs/adr/data/0118-each-job-kind-connects-as-a-runtime-role-of-its-own.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)), which `db/check` tests. When a
process reloads is `assembly`'s. Each loader loads when asked and swaps only on a read it can trust.

## One execution context per process, `assembly`

A process holds one execution context, which every component of it that calls a provider shares,
and the UI never receives
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md),
[ADR-0081](../docs/adr/operability/0081-credentials-sealed-to-a-public-key.md),
[ADR-0084](../docs/adr/mutation/0084-ui-writes-decisions-and-account-setup.md)). `assembly` builds it
from what the composition root hands it, a pool under a role the process already connects as, the
keyring, the scanner, the classifier's lookups, the providers that authenticate through an OAuth
client, a registerer and a logger, and starts nothing. It holds these once in the process.

- One account loader and its account snapshot, and one policy loader and its policy snapshot,
  reloaded together in one reload, which also reads each connected account's lowered rate target.
- The sources of the account sessions, so an account has one token source and one rotation
  write-back in the process. Each component sees them through a session holder of its own, with its
  own connectors, so its provider calls are counted on its own series, and its own database handle,
  so its authentication attempts are recorded under its own role
  ([ADR-0117](../docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)).
- The scanner, so every verdict, mask and serve-time check in the process carries one revision
  ([ADR-0120](../docs/adr/redaction/0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)).

Its reload is a task the root hands the host, on the schedule
[ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md) states,
with its interval from the root's configuration, `account_reload_interval` in the API server and
`reload_interval` in the worker. Each reload its caller did not cancel is timed on one series per
process
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
A reload whose read of the accounts fails changes nothing. One whose policy load fails puts the
accounts it read in force all the same and reports the policy's failure, and the policy loader keeps
the policy it had
([ADR-0041](../docs/adr/engineering/0041-policy-as-immutable-snapshots.md)). Each component follows
the reloads it is told of, the API server serving the accounts of each and the worker waking each job
kind's reload, which adds and drops the kind's jobs. A body request has the policy loaded before it
decides, through the API server's coalesced load of the context's policy loader
([ADR-0099](../docs/adr/engineering/0099-a-body-request-loads-the-policy-before-it-decides.md)).

A unit of work's rate limiter is built by the component, on its own pool, so its leases run under
its own role and are counted on its own series, at the lowered target the context's latest reload
read for the account. An account whose target could not be read, or lies outside the range
[ADR-0024](../docs/adr/operability/0024-conservative-target-aimd.md) allows, gets no limiter, so its
work stops while every other account's goes on. The limiter's code and its leases stay
`ratelimit/`'s.

## The account snapshot, `accountload`

Each process that calls a provider holds its accounts, each paired with the OAuth client it connects
through where its provider has one
([ADR-0106](../docs/adr/provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)),
and the opened credentials as one account snapshot, in its one execution context
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

Copies of these rules in each deployable would drift, and one that got any of them wrong would
fail silently. Written once, they hold in every deployable that calls a provider.

The listing comes from `db/accounts` and each account's credential from `db/accountstate/credential`
([ADR-0091](../docs/adr/data/0091-accounts-listed-apart-from-their-state.md)). The re-seal of an
OAuth client's secret writes through a compare-and-set its caller supplies, because only delta
sync's role may write a client secret ([ADR-0016](../docs/adr/data/0016-schema.md)), and a
statement this package ran would be planned under the role of every execution context.
Delta sync supplies the write from `db/oauthclients/secret`, which only its list admits. A unit of
work takes the snapshot once.

## The account session, `session`

The API server, backfill and delta sync each call a provider for an account in units of work, a
body request, a page of a pass, a tick, through the sessions of their process's execution context. Each pairs the account with the credentials the snapshot
opened, holds a token source over them, reads a refused credential again from the account's row
and makes the call once more over a source built from a replaced one, and at the end of a unit
hands the source's credential over with the adoption stamp of the credential it was built from
([ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md)) and records
the latest authentication attempt
([ADR-0097](../docs/adr/operability/0097-authentication-outcome-reported-by-the-adapter-recorded-by-the-deployable.md)).
Three copies of these steps in three composition roots drifted once, when backfill handed over with
no adoption stamp while every other hand-over carried one. `session` writes them once, as a package
of this family, because it hides a decision of the same concept, how a process holds an account's
credential while it uses it.

It is one mechanism with two parameters, so it never branches on which caller holds it.

- **How long a held source lives.** For one unit of work, or across units while the stored
  credential is the one the source was built from or the one it rotated to. The execution context
  holds across units, so an access token the provider issued serves the units after it, the API
  server's requests, backfill's pages and delta sync's ticks alike. No record gives delta sync a
  reason for a source of its own per tick, which would cost a token refresh on every tick.
- **Which units run concurrently.** The API server runs concurrent units for one account, and in the
  worker backfill's and delta sync's do, so the holder takes a lock around what it holds.

Every holder a component sees through the execution context's holder, by `Through`, shares its
sources and its loader, and opens its sessions with the component's own connectors and records
through the component's own database handle.

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

What each caller supplies keeps its own decisions its own. The execution context passes the account
loader, so when snapshots reload stays the context's
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)), and
each component a database handle the attempt is recorded through and one connector per provider it
links. A connector is built in the composition root from the adapter's constructors over plain
values, the client's identifier and secret and the credential, so which adapters a deployable links
stays its root's decision and no adapter learns what an account is. The session picks the connector by the
account's provider, and an account whose provider has no connector is skipped and logged in one
place. Each call the caller makes through the session leases inside itself, so the session never
sees the rate limiter.

Its import list, `session` in `.golangci.yaml`, admits `accountload`, `core/mail`, `db/tx`,
`db/accountstate/authentication`, the database driver's root package and its `pgtype` package, which
the recording's statement takes, and the standard library. The recording runs under the role of each
deployable or job kind whose list admits the package. No adapter, no rate limiter, no policy and no scanner
belong in it. The limiter at each account's lowered target is built by each component through
`assembly`, since it belongs to the rate budget rather than to the account's credential.

## The policy loader, `policyload`

Each process's execution context holds the active policy as one immutable snapshot, which the API
server, backfill and delta sync decide under, and the reorg job kind and the heuristics job kind are
to decide under, and replaces it only with an update that validates
([ADR-0041](../docs/adr/engineering/0041-policy-as-immutable-snapshots.md)). The validation, the
composition of the base policy with an account's overlay and the atomic swap are pure and sit in
`core/policy`. What is left is impure and the same in every process. It reads the base rows and
each account's overlay rows from the policy tables, hands them to the pure half, and raises the
reload-failure alarm when a reload fails, whether the read or the validation. The reload that swaps
a snapshot in also prepares each account's policy for the sender classifier through
`classify.Prepare`, converting every rule's suffixes once, and the snapshot holds the prepared
policies, so they are swapped with it and never outlive it. A prepared policy also holds the
lookups its suffixes were converted with, which the classifier normalizes the sender with. A
classification then converts only the sender, however many suffixes the policy holds.

The case for one package over per-deployable glue is one rule the glue could get wrong. The pure
half accepts a policy with no rules as valid. A read of the tables that failed, or returned nothing
because of a fault, looks like that empty policy. Treated as one, it would classify the senders the
active policy lists as normal and release content the previous snapshot withheld. So the package
tells a read it cannot trust apart from a policy that is empty, and a read it cannot trust never
replaces the active snapshot. Written once, that rule holds in every deployable that loads policy.
Written in each deployable, one copy that gets it wrong fails open.

The alarm is the other shared part. Each loading process emits the same series, and one alerting
rule reads it ([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)), so
the series is defined once, in this package.

| Series | Kind | Emitted by | Read by |
| --- | --- | --- | --- |
| `mediated_mailbox_policyload_reload_failed` | Gauge, 1 while the latest reload failed and 0 once one succeeds | Each process, once, from its execution context's loader, on the registerer its root passes in, which labels it with `component` ([ADR-0138](../docs/adr/engineering/0138-one-host-runs-each-process-and-each-component-provides-its-parts.md)) | `MediatedMailboxPolicyReloadFailed` in `packaging/chart/rules/alerting-rules.yaml`, whose promtool tests sit in `policyload/testdata/` |

The package also times one load per reload, every one its caller did not cancel, succeeded or
failed, as `mediated_mailbox_policyload_reload_duration_seconds`, a histogram on the same registry
that no rule reads
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).

The reload-failure series is a gauge rather than a counter because a process first loads policy as it starts,
usually before its first scrape, and a rule over a counter whose first sample is already 1 sees no
increase.

Its statements are in `db/policyrules`. Which accounts it reads is its caller's. A process whose
accounts change between its account snapshots sets the new ones, and the next reload reads them
([ADR-0090](../docs/adr/operability/0090-accounts-reach-deployables-as-reloaded-snapshots.md)).
It reads each account's own rules and the base rules in one statement, and a reload whose accounts
read different base rules reads every account once more. Only when the second read's accounts
disagree too is the read one it cannot trust, so an edit to the base policy landing midway never
composes accounts from two versions of it, and raises the alarm only when edits keep landing
through both reads
([ADR-0114](../docs/adr/engineering/0114-a-torn-base-policy-read-is-read-again-before-it-fails-the-reload.md)).
A reload stopped because its caller cancelled it keeps the active policy and raises no alarm, and
one that ran out of time is a failed read like any other.

## The sealed credentials, `credential`

An account's provider credential is stored in the database sealed to a public key
([ADR-0080](../docs/adr/data/0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0081](../docs/adr/operability/0081-credentials-sealed-to-a-public-key.md)). The UI seals a
credential when an account is connected or re-authorized. The execution context of each process
that calls a provider, the API server and the worker, opens it with the private key, and seals a
rotated one before writing it back
([ADR-0082](../docs/adr/operability/0082-rotation-writeback-to-the-database.md)). The UI's
`ui/internal/clientsecret` opens an OAuth client's secret with the same keys, for a consent's code
exchange, and nothing else. These packages are that sealing and opening, the loading of each key
from its mounted file, and the configuration section naming those files with its validation, in
`credential/core`.

The case for one copy over per-deployable code is that a mistake here fails open or loses every
mailbox. A copy that accepted an altered credential, opened one sealed to a key it should not hold,
or sealed to the wrong key would do it silently, and copies in each deployable would drift. Written
once, the construction and its refusals hold in every deployable that touches a credential.

Its packages are cut so an import list can admit the sealing half without the opening half, so the
UI's code outside its one opening part links no code that opens a value. `seal` seals to the current
public key. `open` holds the keyring of private keys and opens a value by the key its header names.
`core` holds the configuration section naming the key files a deployable loads, which every process
that reads them holds under the same keys, the UI's composition root included, since it holds paths
and opens nothing. The key pair comes from the `keygen` subcommand of the command-line program,
`cli/`, since no standard tool writes X-Wing keys, and the subcommand takes the suite and the key
identifier from `seal`. The construction and the sealed value's bytes are
[ADR-0088](../docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)'s, and how a key is
replaced is
[ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)'s. Every write of
a sealed value by a deployable is the compare-and-set of
[ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md).

| Path | Holds |
| --- | --- |
| `credential/seal/` | The public key, the sealed value's bytes and the contexts a value is bound to. It never imports `credential/open/`, and since `credential/open/` imports it the compiler refuses the reverse as a cycle |
| `credential/open/` | The private keys, the keyring, opening, the refusals and the re-seal |
| `credential/core/` | The credential section of a deployable's configuration, naming the mounted public key file and every private key file, and its validation ([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)). It is the credential code's own pure core ([ADR-0040](../docs/adr/engineering/0040-pure-core-decisions-as-values.md)) |

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

A known-answer test in `credential/open/` holds, under `testdata/knownanswer/`, a key pair the
key-generation command wrote and a value an earlier build sealed to it. This build must open the
value to the plaintext sealed, and the seed must derive that public key and the identifier the value
names. A change to any value above other than the purpose spelling `oauth client secret`, which the
value is not sealed under and which `credential/seal`'s additional-data test pins, or a Go release
whose X-Wing derives other keys or another public key, then fails the test before it ships, where it
would otherwise refuse every stored value or every opener's start. The files are the expectation, so
they are never regenerated, and the key pair was generated for the test and protects nothing.

The `keygen` subcommand takes two flags, both required. `-private-key-file` names the file the
32-byte seed is written to with mode 0600, and `-public-key-file` the file the 1216-byte public key
is written to with mode 0644. Each file holds the raw key and nothing else, and the subcommand
refuses a path that already exists, so a key in use is never overwritten. It prints the new key's
identifier.
