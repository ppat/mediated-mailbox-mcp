# credential

A family, one narrow, named shared library holding several packages of one concept, published as
`mediated-mailbox-credential`. Shared code is pure, or it is a library like this one that argues
its own case ([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is
its case. The conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Its concept is [credential encryption](../DESIGN.md#glossary), the sealing of a stored credential
to the public key and its opening with the private key. The decision it hides is how a stored
credential is sealed and opened, which is the construction, the bytes of a sealed value and its
refusals, and how a key is loaded from its mounted file.

Three parts of the system use it, the connections of each process that calls a provider, the UI's
setup and the command-line program. None of the three is the provider library, so the family is a
top-level library of its own rather than a part of any one of them.

| Package | Holds |
| --- | --- |
| `seal/` | The public key, the sealed value's bytes and the contexts a value is bound to. It never imports `credential/open/`, and since `credential/open/` imports it the compiler refuses the reverse as a cycle |
| `open/` | The private keys, the keyring, opening, the refusals and the re-seal, and the opening of OAuth clients' secrets alone, `ClientSecrets`, which the UI's `ui/internal/setup/clientsecret` holds in place of the keyring |

Each package is admitted to each component by its own entry in that component's import list, and
each has a list over its non-test files and a list over its test files in `.golangci.yaml`, so
joining this family never widens what any component may import. A package added here without a list of its own falls under the list over
files outside every component, which admits only the standard library. Opening is admitted to the
connections, `process/connections`, to the UI's `ui/internal/setup/clientsecret` and to the test
tool `testsupport/cmd/keypaircheck` alone among non-test code, and among test files to the
connections' own, the opening half's own and the command-line program's, which opens the pair its
`keygen` subcommand writes ([ADR-0071](../docs/adr/engineering/0071-static-enforcement-toolchain.md)).

**What does not belong.**

- **The configuration section naming the key files.** It is `config.Keys` in `core/config`, a part
  every program that reads the key files holds under the same keys
  ([ADR-0078](../docs/adr/engineering/0078-configuration-layers-through-an-owned-library.md)).
- **The reading and writing of a stored credential's row.** That is the connections' and the UI's
  setup's.
- **When a value is re-sealed.** That is delta sync's
  ([ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)).

Nothing here runs a statement, reads an account or calls a provider.

## The sealed credentials, `seal` and `open`

An account's provider credential is stored in the database sealed to a public key
([ADR-0080](../docs/adr/data/0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0081](../docs/adr/operability/0081-credentials-sealed-to-a-public-key.md)). The UI seals a
credential when an account is connected or re-authorized. The connections of each process that
calls a provider, the API server and the worker, open it with the private key, and seal a rotated
one before writing it back
([ADR-0082](../docs/adr/operability/0082-rotation-writeback-to-the-database.md)). The UI's
`ui/internal/setup/clientsecret` opens an OAuth client's secret with the same keys, for a consent's
code exchange, and nothing else. These packages are that sealing and opening, and the loading of
each key from its mounted file.

The case for one copy over per-program code is that a mistake here fails open or loses every
mailbox. A copy that accepted an altered credential, opened one sealed to a key it should not hold,
or sealed to the wrong key would do it silently, and copies in each program would drift. Written
once, the construction and its refusals hold in every program that touches a credential.

Its packages are cut so an import list can admit the sealing half without the opening half, so the
UI's code outside its one opening part links no code that opens a value. `seal` seals to the current
public key. `open` holds the keyring of private keys and opens a value by the key its header names.
The key pair comes from the `keygen` subcommand of the command-line program, `cli/`, since no
standard tool writes X-Wing keys, and the subcommand takes the suite and the key identifier from
`seal`. The construction and the sealed value's bytes are
[ADR-0088](../docs/adr/operability/0088-credentials-sealed-with-hpke-x-wing.md)'s, and how a key is
replaced is
[ADR-0092](../docs/adr/operability/0092-key-replacement-by-keyring-and-re-seal.md)'s. Every write of
a sealed value by a program is the compare-and-set of
[ADR-0089](../docs/adr/operability/0089-sealed-values-written-by-compare-and-set.md).

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
value is not sealed under and which `credential/seal`'s additional-data test pins, then fails the
test before it ships. So does a Go release whose X-Wing derives other keys or another public key.
Without the test, either would refuse every stored value or every opener's start. The files are the
expectation, so they are never regenerated, and the key pair was generated for the test and
protects nothing.

The `keygen` subcommand takes two flags, both required. `-private-key-file` names the file the
32-byte seed is written to with mode 0600, and `-public-key-file` the file the 1216-byte public key
is written to with mode 0644. Each file holds the raw key and nothing else, and the subcommand
refuses a path that already exists, so a key in use is never overwritten. It prints the new key's
identifier.
