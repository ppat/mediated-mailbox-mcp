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
([ADR-0023](../docs/adr/operability/0023-adapter-declares-cost.md)). It also holds `fake`, the
provider fake of [ADR-0043](../docs/adr/engineering/0043-no-mocking.md), which only test files may
import, and `contract`, the contract suite every implementation passes, ordinary code so each
implementation's tests can run it. The Provider Port interface and the canonical model are pure and
sit in `core/mail`.
