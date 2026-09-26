# provider

A narrow, named shared library, published as `mediated-mailbox-provider`. Shared code is pure, or it
is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

The mediator fetches released bodies, backfill and delta sync build and refresh the index, and the
reorganization workload applies plans, all through the Provider Port
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
files may import, and `contract`, the contract suite every implementation passes, ordinary code so each
implementation's tests can run it. The Provider Port interface and the canonical model are pure and
sit in `core/mail`.

The fake keeps a mailbox in memory. Its throttle is a wrapper around any port that applies a
schedule, so the rate limiter's tests can meet throttling without the fake's storage knowing of it,
and a throttled call changes nothing. The contract suite seeds the mailbox it runs against from the
synthetic fixtures, adding what a mailbox holds beside a message, its identifiers, thread, date,
labels and flags, and never assumes an implementation keeps the identifiers it was seeded with.
Against a real provider it adds its messages to a test account it does not control, each marked as
that run's, and checks and reports only those, never assuming or touching the account's other mail
([ADR-0043](../docs/adr/engineering/0043-no-mocking.md)). The Gmail consent for the test account's
token runs from `gmail/cmd/consent`, a developer's command that no deployable runs and no image
ships ([ADR-0083](../docs/adr/provider/0083-gmail-through-an-installation-oauth-client.md)). It
takes the address of the account the grant is meant for and refuses a grant that belongs to
another account or holds any scope but the modify scope.

The Gmail adapter also holds the handling of Google's grant, which every deployable that calls
Google shares, the Google Calendar adapter included. That is the one-time installed-app consent
and the token source. The token source is built from the installation's OAuth client and the
account's refresh token, which the deployable opened from the database, and reads no credential
from anywhere else. When Google rotates the refresh token, the source holds the new one and hands
it over as its current refresh token. The source writes nothing. The deployable reads that token
at the end of each unit of work and writes a rotated one back to the account's state row through
`accountload/`
([ADR-0080](../docs/adr/data/0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0082](../docs/adr/operability/0082-rotation-writeback-to-the-database.md),
[ADR-0083](../docs/adr/provider/0083-gmail-through-an-installation-oauth-client.md)).

## Series and the rules that read them

Every adapter emits both series, labelled by `account` and by `provider`, whose value is the
adapter's own name, such as `gmail`. Two alerting rules in `packaging/chart/alerting-rules.yaml`
read them, and neither holds a provider's number of its own.

| Rule | Reads | Fires when |
| --- | --- | --- |
| `MediatedMailboxRateRunaway` | Both | The cost summed over every spending process for the account over two minutes passes the highest hard cap emitted for it over those two minutes, times 120 seconds |
| `MediatedMailboxRateHardCapAbsent` | Both | The account's cost grew over the last five minutes and no hard cap was emitted for it in the last two, held for five minutes, since the runaway rule has no threshold for that account |

| Series | Kind | Value |
| --- | --- | --- |
| `mediated_mailbox_provider_request_cost_total` | Counter | The cost of every request the process sent for the account, in the provider's units, failed requests included |
| `mediated_mailbox_provider_hard_cap` | Gauge | The account's hard cap in the provider's units per second, `mail.HardCapFraction` of the ceiling the adapter's rate profile declares, set each time a request is counted |
