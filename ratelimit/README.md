# ratelimit

A narrow, named shared library, published as `mediated-mailbox-ratelimit`. Shared code is pure, or
it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Every spender runs the same rate rules, the cap at lease issuance, the priority split, the adaptive
controller and lease expiry ([ADR-0024](../docs/adr/operability/0024-conservative-target-aimd.md),
[ADR-0025](../docs/adr/operability/0025-priority-classes-and-leases.md)). Those rules are pure and
sit in `ratelimit/core/`. The lease code that reads and writes the shared coordination row is impure
and sits in `ratelimit/lease/`. Keeping both in one library means the rules are not split from the
code that applies them and the lease code is not duplicated per deployable.
