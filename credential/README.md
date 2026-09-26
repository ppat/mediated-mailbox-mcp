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
the UI links no code that opens a credential. `seal` seals to the current public key. `open` holds
the keyring of private keys and opens a value by the key its header names. `cmd/keygen` writes a
key pair, since no standard tool writes X-Wing keys. The construction and the sealed value's bytes
are [ADR-0088](../docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)'s, and how a
key is replaced is [ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)'s.
Every write of a sealed value by a deployable is the compare-and-set of
[ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md).

## Layout

| Path | Holds |
| --- | --- |
| `seal/` | The public key, the sealed value's bytes and the contexts a value is bound to. It never imports `open/`, and since `open/` imports it the compiler refuses the reverse as a cycle |
| `open/` | The private keys, the keyring, opening, the refusals and the re-seal |
| `cmd/keygen/` | The key-generation command |

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

`cmd/keygen` takes two flags, both required. `-private-key-file` names the file the 32-byte seed is
written to with mode 0600, and `-public-key-file` the file the 1216-byte public key is written to
with mode 0644. Each file holds the raw key and nothing else, and the command refuses a path that
already exists, so a key in use is never overwritten. It prints the new key's identifier.
