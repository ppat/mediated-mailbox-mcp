# credential

A narrow, named shared library, published as `mediated-mailbox-credential`. Shared code is pure, or
it is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

An account's provider credential is stored in the database sealed to a public key
([ADR-0080](../docs/adr/data/0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0081](../docs/adr/operability/0081-credentials-sealed-to-a-public-key.md)). The UI seals a
credential when an account is connected or re-authorized. Backfill, the mediator, delta sync and the
reorg workload open it with the private key, and seal a rotated one before writing it back
([ADR-0082](../docs/adr/operability/0082-rotation-writeback-to-the-database.md)). This library is
that sealing and opening, and the loading of each key from its mounted file.

The case for one library over per-deployable code is that a mistake here fails open or loses every
mailbox. A copy that accepted an altered credential, opened one sealed to a key it should not hold,
or sealed to the wrong key would do it silently, and five copies would drift. Written once, the
construction and its refusals hold in every deployable that touches a credential.

Its subsections are cut so an import list can admit the sealing half without the opening half, so
the UI links no code that opens a credential.
