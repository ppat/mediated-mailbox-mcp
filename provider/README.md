# provider

A narrow, named shared library, published as `mediated-mailbox-provider`. Shared code is pure, or it
is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

The mediator fetches released bodies, backfill and delta sync build and refresh the index, and the
reorg job kind applies plans, all through the Provider Port
([ADR-0010](../docs/adr/provider/0010-one-provider-port.md)). Deployables never import each other,
so the adapters are shared code, and they do network I/O, so they cannot sit in `core/`. Writing an
adapter per deployable would break the one-adapter-per-provider contract. So one library holds
`gmail`, `gcal`, `jmap` and `caldav`, each with the rate profile its provider needs
([ADR-0023](../docs/adr/operability/0023-adapter-declares-cost.md)). Each adapter counts the cost of
every request where it sends it and emits the account's hard cap beside the count, the series the
runaway rule reads
([ADR-0077](../docs/adr/operability/0077-conditions-raised-as-alerting-rules.md)), so the library
registers metrics through client_golang on a registry its caller passes in
([ADR-0076](../docs/adr/engineering/0076-metrics-emitted-through-client-golang.md)). It also holds
`fake`, the provider fake of [ADR-0043](../docs/adr/engineering/0043-no-mocking.md), which only test
files may import, and `contract`, the contract suite every implementation passes, ordinary code so
each implementation's tests can run it. The Provider Port interface and the canonical model are
pure and sit in `core/mail`.

The fake keeps a mailbox in memory. Its throttle is a wrapper around any port that applies a
schedule, so the rate limiter's tests can meet throttling without the fake's storage knowing of it,
and a throttled call changes nothing. The contract suite seeds the mailbox it runs against from the
synthetic fixtures, adding what a mailbox holds beside a message, its identifiers, thread, date,
labels and flags, and never assumes an implementation keeps the identifiers it was seeded with.
Against a real provider it adds its messages to a test account it does not control, each marked as
that run's, and checks and reports only those, never assuming or touching the account's other mail
([ADR-0043](../docs/adr/engineering/0043-no-mocking.md)). The Gmail consent for the test account's
token runs from `gmail/cmd/consent`, a developer's command that no deployable runs and no image
ships, over the same `gmail/consent` package the UI runs
([ADR-0107](../docs/adr/provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)).
It takes the address of the account the grant is meant for and refuses a grant that belongs to
another account or holds any scope but the modify scope.

Google's consent sits apart from the adapter, in `gmail/consent`, a package holding no Provider
Port code, so the UI links it without linking the adapter, whose import the UI's list refuses
([ADR-0107](../docs/adr/provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)).
It holds the consent page's address with its PKCE challenge and state, the code exchange with its
check that the grant is the modify scope alone, the read of the profile that names the mailbox a
grant belongs to, the comparison of that mailbox with the one named, which ignores case and, for
`gmail.com` and `googlemail.com`, dots and the choice between the two domains, the reading of the
loopback address the operator pastes back, and the check of a client's identifier and secret
against Google's token endpoint. It also holds the requests to the token endpoint and the reading
of its answers, which the adapter's token source sends its refreshes through. The UI reaches it
through the consent interface of `core/mail`, which `fake` implements too, so the UI's setups are
tested over the fake. A new package beside the adapter was chosen over a narrow library of its own,
which would put Google's grant outside this library and argue a case of its own, and over admitting
the adapter to the UI's list, which would let the UI link the code that reads a mailbox.

The Gmail adapter also holds the handling of Google's grant, which every deployable that calls
Google shares, the Google Calendar adapter included. That is the token source, and the consent
command beside it. The token source is built from the OAuth client the account connects through
and the account's refresh token, which the deployable opened from the database, and reads no
credential from anywhere else
([ADR-0106](../docs/adr/provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)).
A Gmail account's stored credential is the refresh token itself, the one the account setup stored
from the grant. When Google rotates the refresh token, the source holds the new one and hands it
over as its current refresh token. The source writes nothing. The deployable reads that token at
the end of each unit of work and writes a rotated one back to the account's state row through
`executioncontext/accountload/`
([ADR-0080](../docs/adr/data/0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0082](../docs/adr/operability/0082-rotation-writeback-to-the-database.md),
[ADR-0107](../docs/adr/provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)).
The source also holds its latest request to Google's token endpoint with its outcome, succeeded,
refused for a 400 or a 401, or failed for anything else, a passed deadline included, and holds
nothing for a request its caller cancelled or for an access token it already held. The deployable
records that attempt on the account's state row at the end of each unit of work
([ADR-0097](../docs/adr/operability/0097-authentication-outcome-reported-by-the-adapter-recorded-by-the-deployable.md)).
A port call that could not obtain an access token returns the error that reading names, a refused
credential for a refusal and the provider failing the request for any other failure, apart from a
call cancelled or past its deadline, which returns the context's error. So a caller is never told
its credential was refused when Google's token endpoint was unreachable
([O5](../USE_CASES.md#o5--clients-can-tell-failures-apart)).

## Series and the rules that read them

Every adapter emits both series, labelled by `account` and by `provider`, whose value is the
adapter's own name, such as `gmail`. It also emits each request's latency, labelled by `provider`,
by the provider method the request calls and by its outcome, and not by account, which no rule
reads ([ADR-0125](../docs/adr/engineering/0125-performance-is-measured-once-at-each-place-time-goes.md)).
In the worker each job kind passes a registry that also labels each series with `job_kind`, the
job kind that produced it
([ADR-0117](../docs/adr/operability/0117-one-background-worker-runs-every-job-kind.md)). Two
alerting rules in `packaging/chart/alerting-rules.yaml` read the cost and the hard cap, and neither
holds a provider's number of its own.

| Rule | Reads | Fires when |
| --- | --- | --- |
| `MediatedMailboxRateRunaway` | Both | The cost summed over every spending process for the account over two minutes passes the highest hard cap emitted for it over those two minutes, times 120 seconds |
| `MediatedMailboxRateHardCapAbsent` | Both | The account's cost grew over the last five minutes and no hard cap was emitted for it in the last two, held for five minutes, since the runaway rule has no threshold for that account |

| Series | Kind | Value |
| --- | --- | --- |
| `mediated_mailbox_provider_request_cost_total` | Counter | The cost of every request the process sent for the account, in the provider's units, failed requests included |
| `mediated_mailbox_provider_hard_cap` | Gauge | The account's hard cap in the provider's units per second, `mail.HardCapFraction` of the ceiling the adapter's rate profile declares, set each time a request is counted |
| `mediated_mailbox_provider_request_duration_seconds` | Histogram, `endpoint` and `outcome` of `ok`, `throttled` or `failed` | Each request's latency, from sending it to reading its answer, the access token obtained before |
