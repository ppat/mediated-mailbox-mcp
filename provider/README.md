# provider

A narrow, named shared library, published as `mediated-mailbox-provider`. Shared code is pure, or it
is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

The API server fetches released bodies, backfill and delta sync build and refresh the index, and the
reorg job kind applies plans, all through the Provider Port
([ADR-0010](../docs/adr/provider/0010-one-provider-port.md)). Deployables never import each other,
so the adapters are shared code, and they do network I/O, so they cannot sit in `core/`. Writing an
adapter per deployable would break the one-adapter-per-provider contract. So one library holds
everything about talking to a provider.

- **The adapters**, `mail/gmail`, `mail/jmap`, `calendar/google` and `calendar/caldav`, each with
  the rate profile its provider needs
  ([ADR-0023](../docs/adr/operability/0023-adapter-declares-cost.md)).
- **The calling side**, which the API server and the worker reach the adapters through.
- **The rate limiter**, which every call spends from.

The cost of every request is counted where it is sent, and the account's hard cap is emitted beside
the count. Those are the series the runaway rule reads
([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)), so the library
registers metrics through client_golang, on a registerer its caller passes in
([ADR-0076](../docs/adr/engineering/0076-metrics-emitted-through-client-golang.md)).

| Package | Holds |
| --- | --- |
| `provider` itself | The calling side, the caller, the token, the one wrapper of a mailbox's operations, the list of shipped adapters, the facts about each provider, and the shared provider metrics |
| `mail/` | The Provider Port's interface for mail, `mail.Mailbox`, with its ten operations, the consent contract and the call sizes a caller needs, which backfill and delta sync's tick read. Each is what `core/mail`'s `CallSize` gives within the hard-cap fraction `core/mail` holds ([ADR-0010](../docs/adr/provider/0010-one-provider-port.md)). It also holds `mail.Body`, the one home of a body's fetch over a `mail.Mailbox`, which asks once more for a part the provider holds apart as `core/mail`'s decision on a second body call says, and which every program that fetches a body uses ([ADR-0131](../docs/adr/provider/0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md)). The port's errors, the part of a body a call names, the decision on a second body call, the size bound, `CallSize` and the types the rate profiles are stated in are `core/mail`'s, for the reasons [CLAUDE.md](../CLAUDE.md#what-coremail-holds) gives for each |
| `mail/gmail/`, `mail/gmail/core/` | The Gmail adapter, and its pure decisions on a body, which the adapter alone imports |
| `mail/jmap/` | The JMAP adapter |
| `internal/adapters/` | The representation of the list of adapters and of a token, each adapter's facts, how it makes a token and builds a mailbox, which `provider` exports as `provider.Adapters` and `provider.Token`. Go admits a package under `internal/` to the provider library alone |
| `mail/fake/` | The provider fake |
| `mail/contract/` | The contract suite every implementation passes |
| `google/` | Google's grant, its token source and token endpoint, its consent, and the consent command under `google/cmd/consent` |
| `consent/` | Each provider's consent by name, for setting an account up |
| `ratelimit/`, `ratelimit/core/` | The rate limiter, its pure rules, the code that reads and writes the shared rate state, and the leased call |
| `calendar/google/`, `calendar/caldav/` | The calendar adapters |

Each package is admitted to each component by its own entry in that component's import list. An
adapter is admitted to `provider` alone, so no program names an adapter, and `ratelimit` to
`provider` and to the API server's registry of operations, `api/internal/registry`, for its
rate-state collector, so no other package of a program names the limiter. `ratelimit/core`, which
holds the limiter's own rules, limits and state, is admitted to `ratelimit` and to `provider`,
which builds each caller's limiter, and `mail/gmail/core` to the Gmail adapter alone. The UI's
setup reaches the provider library through `consent` and the consent contract in `mail`
([ADR-0071](../docs/adr/engineering/0071-static-enforcement-toolchain.md)). The library does two
jobs, calls in `provider` and setup in `consent`, so the UI links the consent and neither an
adapter nor the limiter.

**What does not belong.** The adapters use no database, and `ratelimit` uses the rate state's
statements, in `db/ratestate/limiter`, and the collector reads an account's lowered target through
`db/accountstate`, the one statement it takes from the account tables. The library knows nothing
else of them.
Credentials arrive as arguments, and a refreshed credential and an attempt leave as values.

- **Reading the accounts, opening their credentials and writing a credential back.** Reading the
  accounts is `process/accounts`', and opening and writing back a credential are the connections',
  both in the [process family](../process/README.md#the-connections-connections).
- **Sealing and opening a credential.** Those are the
  [credential family's](../credential/README.md).

`mail/fake` is the provider fake of [ADR-0043](../docs/adr/engineering/0043-no-mocking.md), which
only test files may import, and `mail/contract` the contract suite every implementation passes,
ordinary code so each implementation's tests can run it. The canonical model is pure and sits in
`core/mail`, and the Provider Port for mail, `mail.Mailbox`, is `mail`'s
([ADR-0010](../docs/adr/provider/0010-one-provider-port.md)). An adapter's own decisions that read
its provider's shapes are pure too, and sit in a `core` package beside it, so the core import check
and the globals analyser hold them
([ADR-0040](../docs/adr/engineering/0040-pure-core-decisions-as-values.md)). That is
`mail/gmail/core`, the Gmail adapter's decisions on a body from Gmail's part tree. They are which
part is rendered, whether it is returned, fetched or refused, the decoding of a part's data, each
read's cap and the refusal of an answer past it
([ADR-0131](../docs/adr/provider/0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md),
[ADR-0134](../docs/adr/provider/0134-a-held-apart-part-is-decoded-by-the-whatwg-label-table.md)).
It returns each decision as a value, and the adapter makes the requests, reads each answer up to
the cap the core gives it and builds the port's errors. The media-type parser and the charset label
table reach the operating system through what they import, so the adapter passes them in.

Only the provider library builds a list holding an adapter. Code outside it can name the empty list
and nothing more. `provider.Shipped` builds the shipped list, and the fake's `fake.Adapters` builds
one over the fake for a test, through `internal/adapters`, which no package outside the library can
import. A list and a token expose no field, so code that
holds one cannot add an adapter or reach a token's source. A test hands such a list to the
connections through their one option for a test, `connections.WithAdapters`, so a process it runs
calls the fake wherever the program calls its provider, and nothing that ships can select the fake,
since only test files may import it
([ADR-0142](../docs/adr/engineering/0142-each-program-is-a-spec-that-one-process-runs.md)). Each
adapter of a list declares the facts about its provider, so a caller builds an account's limits
from the facts of the list it was built over. `provider.FactsOf` gives code that holds no list the
facts the shipped list declares, read from that list, so each provider's facts have one home.

The fake keeps a mailbox in memory. Its throttle is a wrapper around any mailbox that applies a
schedule, so the rate limiter's tests can meet throttling without the fake's storage knowing of it,
and a throttled call changes nothing. It returns a body's rendered part alone, refuses one past the
port's bound, and can hold a message's rendered part apart so that only a second call naming it
returns it, as Gmail holds a large part, so the fetch by part runs without a real provider
([ADR-0131](../docs/adr/provider/0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md)).
The contract suite seeds the mailbox it runs against from the synthetic fixtures, adding what a
mailbox holds beside a message, its identifiers, thread, date, labels and flags, and never assumes
an implementation keeps the identifiers it was seeded with.
Against a real provider it adds its messages to a test account it does not control, each marked as
that run's, and checks and reports only those, never assuming or touching the account's other mail
([ADR-0043](../docs/adr/engineering/0043-no-mocking.md)). The Gmail consent for the test account's
token runs from `google/cmd/consent`, a developer's command that no deployable runs and no image
ships, over the same Google consent in `google` that the UI runs
([ADR-0107](../docs/adr/provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)).
It takes the address of the account the grant is meant for and refuses a grant that belongs to
another account or holds any scope but the modify scope.

## The calling side, `provider`

A caller is one job kind's, or the API server's body operation's, own equipment for calling a
provider, which is its adapter clients, its limiter, its provider metrics, its priority class,
interactive, sync or batch, and its timeout
([ADR-0117](../docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)). A caller
has one class.

This package builds a caller through `provider.NewCaller`, over the list of shipped adapters, the
registerer the job kind or operation was built with, the pool its leases run on, and the caller's
options. It registers the adapter's and the limiter's series on that registerer, so they carry its
labels.

For one account's unit of work, a caller returns a mailbox, a `mail.Mailbox`. It is given the
account's connection as this library needs it, a struct with named fields.

| Field | What it carries |
| --- | --- |
| The provider | Which adapter serves the account |
| The account | The account the calls are for |
| The token | What the calls authenticate with |
| The account's target, and whether it is lowered | The rate the calls lease at |
| The time the leases may wait until | Zero for none |
| The read of the account again | What a refused credential calls before the call is made once more |

Every operation of that mailbox takes a lease sized by the operation's cost at the account's
target, waits for it no later than that time, runs under the caller's timeout, and tells the
limiter how it went. One type does all of this, so the ten operations are wrapped once.

The package also makes an account's token from the provider's name, the client and the credential,
and the token reports its current credential and its latest attempt, which the connections read at
the end of each unit of work. And it holds the facts about each provider, whether it uses an OAuth
client and its ceiling, which the connections and the API server's rate-state collector read
rather than each program listing them.

The case for one package over per-program code is that the wrap of every operation in a lease, a
class and a timeout, with the report of each call's outcome to the limiter, would otherwise be
written in each program that calls a provider, once in the API server and once per job kind, and
each copy could drift on its own. Written once, every spender's calls lease and report the same
way.

## Google's grant, `google`, and the consents, `consent`

Google's grant sits apart from the adapter, in `google`, a package holding no Provider Port code,
so the UI links it without linking the adapter, whose import the UI's list refuses
([ADR-0107](../docs/adr/provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)).
The UI's setup reaches it through `consent`, which returns each provider's consent by its name, as
`consent.For(name, client, redirect)`. Google's consent holds these.

- The consent page's address, with its PKCE challenge and state.
- The code exchange, with its check that the grant is the modify scope alone.
- The read of the profile that names the mailbox a grant belongs to.
- The comparison of that mailbox with the one named, which ignores case and, for `gmail.com` and
  `googlemail.com`, dots and the choice between the two domains.
- The reading of the loopback address the operator pastes back.
- The check of a client's identifier and secret against Google's token endpoint.

`google` also holds the requests to the token endpoint and the reading of its answers, which the
token source sends its refreshes through. Google's consent implements the consent contract of
`mail`, which `mail/fake` implements too, so the UI's setups are tested over the fake. A package of
this library apart from the adapter was chosen over a narrow library of its own, which would put
Google's grant outside this library and argue a case of its own, and over admitting the adapter to
the UI's list, which would let the UI link the code that reads a mailbox.

`google` holds the rest of the handling of Google's grant too, which every adapter that calls
Google shares, the Google Calendar adapter included. That is the token source, and the consent
command under it. The token source is built from the OAuth client the account connects through and
the account's refresh token, which the connections opened from the database, and reads no
credential from anywhere else
([ADR-0106](../docs/adr/provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)).
A Gmail account's stored credential is the refresh token itself, the one the account setup stored
from the grant. When Google rotates the refresh token, the source holds the new one and hands it
over as its current refresh token. The source writes nothing. The connections read that token at
the end of each unit of work and write a rotated one back to the account's state row
([process/README.md](../process/README.md#the-connections-connections),
[ADR-0080](../docs/adr/data/0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0082](../docs/adr/operability/0082-rotation-writeback-to-the-database.md),
[ADR-0107](../docs/adr/provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)).
The source also holds its latest request to Google's token endpoint with its outcome, succeeded,
refused for a 400 or a 401, or failed for anything else, a passed deadline included, and holds
nothing for a request its caller cancelled or for an access token it already held. The connections
record that attempt on the account's state row at the end of each unit of work
([ADR-0097](../docs/adr/operability/0097-authentication-outcome-reported-by-the-adapter-recorded-by-the-deployable.md)).
A mailbox call that could not obtain an access token returns the error that reading names, a refused
credential for a refusal and the provider failing the request for any other failure, apart from a
call cancelled or past its deadline, which returns the context's error. So a caller is never told
its credential was refused when Google's token endpoint was unreachable
([O5](../USE_CASES.md#o5--clients-can-tell-failures-apart)).

## Rate limiting, `ratelimit`

Every spender runs the same rate rules, the cap at lease issuance, the priority split, the adaptive
controller and lease expiry ([ADR-0024](../docs/adr/operability/0024-conservative-target-aimd.md),
[ADR-0025](../docs/adr/operability/0025-priority-classes-and-leases.md)). Those rules are pure and
sit in `ratelimit/core`, with the limiter's limits and state. The hard-cap fraction, the cost type
and the throttle signal they read are `core/mail`'s, since others read them too. The adapters and
the provider fake read the cost type and the throttle signal, and the contract suite those and the
hard-cap fraction. The lease code that reads and writes the shared coordination row and its grants
table is impure and sits in `ratelimit`. Keeping both in this library means the rules are not split
from the code that applies them, the lease code is not duplicated per program, and the limiter sits
beside the one wrapper that spends from it. Its leased call takes a cost, not a mailbox, makes one
call under a lease in a priority class and tells the controller how the call went, a throttle or a
server error included, so every spender reports its calls the same way.

The collector reads an account's lowered target as the connections do, through `db/accountstate`,
and decides a stored target with `core.LimitsWithTarget`, the rule the limiter applies, so an
account with no rate state is reported at the target it will spend under, and a stored target the
limiter refuses fails the whole scrape, as a failed read does, so every series of the process goes
absent and the absence rule fires, as the account's work stops
([ADR-0024](../docs/adr/operability/0024-conservative-target-aimd.md)).

Its statements in `db/ratestate/limiter` and `db/accountstate` run under the application user, in
the API server and the worker, whose grants the grant check proves enough for them
([ADR-0143](../docs/adr/data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md),
[ADR-0066](../docs/adr/data/0066-data-access-generated-from-sql.md)).

### The rate limiter's series and the rules that read them

The lease code also emits the rate limiter's metrics, on the registerer each caller was built with
([ADR-0076](../docs/adr/engineering/0076-metrics-emitted-through-client-golang.md)). Every series
but the lease wait is a gauge or counter labelled by `account`. In the worker each job kind's
registerer also labels each series with `job_kind`, the job kind that produced it
([ADR-0117](../docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)). The
alerting rules that read them are `packaging/chart/rules/alerting-rules.yaml`, whose promtool tests
sit in `ratelimit/testdata/`
([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)).

| Series | Kind and other labels | Emitted by | Read by |
| --- | --- | --- | --- |
| `mediated_mailbox_ratelimit_rate` | Gauge | Each spending process, as it last read or set the rate | Dashboards |
| `mediated_mailbox_ratelimit_granted_total` | Counter, `class` | Each spending process, in the provider's units | Dashboards |
| `mediated_mailbox_ratelimit_throttles_total` | Counter, `scope` of `user`, `project` or `unknown` | Each spending process | Dashboards |
| `mediated_mailbox_ratelimit_lease_wait_seconds` | Histogram, `class`, and no `account` | Each spending process, from an ask's start to its granted lease ([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)) | Dashboards |
| `mediated_mailbox_ratelimit_account_rate` | Gauge | The API server's collector, from each account's rate state, and for an account with none, at the target the account spends under, its lowered one or the default | The collapse rule and the absence rule |
| `mediated_mailbox_ratelimit_account_floor` | Gauge | The API server's collector | The collapse rule |
| `mediated_mailbox_ratelimit_account_bucket_level` | Gauge | The API server's collector | Dashboards |
| `mediated_mailbox_ratelimit_account_seconds_since_ask` | Gauge, infinite when no class has asked | The API server's collector | Both collapse rules |
| `mediated_mailbox_ratelimit_account_seconds_since_grant` | Gauge, infinite when nothing was granted | The API server's collector | The stall rule |

The runaway rule reads none of these, because a runaway is the failure in which lease accounting is
wrong. It reads the cost of provider requests counted where each request is sent, against the
account's hard cap each spending process emits beside that count from its provider's declared
ceiling. A second rule fires when an account's cost is counted and no hard cap is, since the
runaway rule then has no threshold. This library emits both series, and
[the next section](#the-provider-request-series-and-the-rules-that-read-them) lists them with the
two rules.

## The provider request series and the rules that read them

The request cost and the hard cap are each labelled by `account` and by `provider`, whose value is
the adapter's own name, such as `gmail`. Each request's latency is emitted too, labelled by
`provider`, by the provider method the request calls and by its outcome, and not by account, which
no rule reads
([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
Each caller registers them on the registerer its job kind or operation was built with, so in the
worker each series also carries `job_kind`, the job kind that produced it
([ADR-0117](../docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)). Two
alerting rules in `packaging/chart/rules/alerting-rules.yaml` read the cost and the hard cap, and
neither holds a provider's number of its own.

| Rule | Reads | Fires when |
| --- | --- | --- |
| `MediatedMailboxRateRunaway` | Both | The cost summed over every spending process for the account over two minutes passes the highest hard cap emitted for it over those two minutes, times 120 seconds |
| `MediatedMailboxRateHardCapAbsent` | Both | The account's cost grew over the last five minutes and no hard cap was emitted for it in the last two, held for five minutes, since the runaway rule has no threshold for that account |

| Series | Kind | Value |
| --- | --- | --- |
| `mediated_mailbox_provider_request_cost_total` | Counter | The cost of every request the process sent for the account, in the provider's units, failed requests included |
| `mediated_mailbox_provider_hard_cap` | Gauge | The account's hard cap in the provider's units per second, the hard-cap fraction `core/mail` holds of the ceiling the adapter's rate profile declares, set each time a request is counted |
| `mediated_mailbox_provider_request_duration_seconds` | Histogram, `endpoint` and `outcome` of `ok`, `throttled` or `failed` | Each request's latency, from sending it to reading its answer, the access token obtained before |
