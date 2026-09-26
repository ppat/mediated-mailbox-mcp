# 0088. A credential is sealed with HPKE's X-Wing suite from Go's standard library, under a header naming its key

**Status:** Accepted ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

[ADR-0081](./0081-credentials-sealed-to-a-public-key.md) settles that an account's credential and
an OAuth client's secret are sealed to a public key the UI holds, that only the deployables that
call a provider hold the private key, that the construction is authenticated, and that a sealed
value records the key it was sealed to. [ADR-0079](./0079-secrets-arrive-as-mounted-files.md)
settles that both keys arrive as mounted files. What is left to choose is the construction, and the
bytes a sealed value carries so its key can be found and a key can be replaced.

Symmetric encryption and encryption inside PostgreSQL are ruled out by ADR-0081. ADR-0081 also has
the UI seal with a public key and the deployables that call a provider open with a private key they
hold as a mounted file, so the private key is in those processes and nowhere else. A key service
such as a cloud KMS or Vault, which holds the key itself and decrypts on request, is a different
shape from that decision. The field is the public-key constructions a Go program can run in
process.

The choice looks like a question of which library to trust with encryption. It is mostly a question
of time. A sealed value is a full-mailbox grant that stays valid until it is revoked, so a copy of
the database taken today stays worth decrypting for as long as the grant lives. That makes
confidentiality against later decryption, including by a quantum computer, a requirement rather
than a preference.

| # | Requirement | Demands | From |
| --- | --- | --- | --- |
| R1 | Authenticated public-key sealing | An altered, truncated or wrongly keyed value is refused, never opened | ADR-0081 |
| R2 | The value names its key | A sealed value identifies the key it was sealed to, so a key can be replaced while old values are stored | ADR-0081 |
| R3 | Bound to its row and purpose | A value moved to another row or used for another purpose is refused | This record |
| R4 | Footprint | Few or no modules added to the processes that hold full-mailbox credentials | [ADR-0042](../engineering/0042-implementation-stack.md), [ADR-0028](./0028-trust-anchor-hardening.md) |
| R5 | Keys load from plain files | A key is a mounted file holding the key itself, not a key-management format | ADR-0079 for the mounted file, this record for its plain content |
| R6 | A standard construction | Specified publicly with test vectors, so it is not the project's own cryptography | ADR-0081 |
| R7 | Confidentiality against later decryption | A stolen database copy stays sealed against an attacker with a quantum computer | This record |
| R8 | Keys from a standard tool | An operator can generate the key pair without project code | This record |
| R9 | The sealing half importable alone | The UI links no code that opens a credential | ADR-0081, [credential/README.md](../../../credential/README.md) |
| R10 | Maintained and current | Released and maintained today | This record |

R1 is a gate. R2, R4, R5, R6 and R7 order the field. R7 orders it because of the lifetime argument
above. R3 and R8 break ties. R9 and R10 separated no candidate. Every candidate ships sealing and
opening together and the cut lives in this project's own subsections, and every candidate was
released recently and none is deprecated.

## Decision

- **Go's standard-library `crypto/hpke`**, which implements
  [RFC 9180](https://www.rfc-editor.org/rfc/rfc9180) and is part of the Go release the module
  already pins, so it adds no module ([crypto/hpke](https://pkg.go.dev/crypto/hpke)). It is used in
  base mode through its single-shot sender and recipient.
- **The suite is X-Wing, HKDF-SHA256 and ChaCha20-Poly1305.** X-Wing is the hybrid of ML-KEM-768 and
  X25519, so a stored value stays sealed unless both are broken. It holds the registered HPKE KEM
  identifier 0x647A ([IANA HPKE registry](https://www.iana.org/assignments/hpke/hpke.xhtml)), its
  HPKE binding is the working-group draft
  [draft-ietf-hpke-pq](https://datatracker.ietf.org/doc/draft-ietf-hpke-pq/), and the CFRG's
  [draft-irtf-cfrg-concrete-hybrid-kems](https://datatracker.ietf.org/doc/draft-irtf-cfrg-concrete-hybrid-kems/)
  calls its MLKEM768-X25519 identical to it. Go ships that draft's test vectors for exactly this
  suite. It is also implemented in
  [BoringSSL](https://github.com/google/boringssl/blob/main/include/openssl/xwing.h),
  [libsodium](https://github.com/jedisct1/libsodium/releases/tag/1.0.22-RELEASE),
  [Cloudflare CIRCL](https://github.com/cloudflare/circl/tree/main/kem/xwing),
  [RustCrypto](https://crates.io/crates/x-wing),
  [Bouncy Castle](https://github.com/bcgit/bc-java/blob/main/core/src/main/java/org/bouncycastle/pqc/crypto/xwing/XWingKEMGenerator.java)
  and [Apple CryptoKit](https://developer.apple.com/documentation/cryptokit/xwingmlkem768x25519).
- **A sealed value is a version byte, a 16-byte key identifier, the encapsulated key, and the
  ciphertext with its tag.** The version byte names the suite. The key identifier is the first 16
  bytes of SHA-256 over a fixed domain string and the public key. It is derived, so it cannot
  disagree with the key file. A reader looks the key up by it. A value whose identifier names no
  key it holds is refused with an error of its own, and a value naming a key it holds that fails
  authentication is refused as altered, so a missing key is never reported as tampering. The
  identifier is authenticated only once its key is found, so a value whose identifier bytes were
  altered is refused as naming an unknown key.
- **The additional data binds the header and a context.** It is the version byte and key identifier,
  a zero byte, and a context naming the value's purpose and its row, each part prefixed with its
  length so no two contexts encode alike. A value copied to another account or used as the other
  kind of secret then fails to open. The HPKE `info` is a constant.
- **The private key file holds the 32-byte X-Wing seed, and the public key file the 1216-byte
  public key.** An opener derives the public key from each of its seeds at start and refuses to
  start unless its mounted public key matches one, because an opener seals rotated credentials to
  that public key ([ADR-0082](./0082-rotation-writeback-to-the-database.md)) and a mismatched one
  would store values no opener can read.
- **A command in the library, `credential/cmd/keygen`, generates the key pair**, because no
  standard tool writes X-Wing keys. It ships as static binaries attached to each GitHub release,
  at the lockstep release version and signed with the same keyless signing the release applies to
  the images and the chart ([ADR-0049](../engineering/0049-image-per-component-lockstep.md)). An
  operator command sits under its library, and a library holds no Dockerfile, so it has no image
  of its own. Carrying it in an existing image would give that image a second, unrelated job. The
  UI must never carry key material, and the migration image holds none of this project's Go code
  by design. A signed release binary is a published artifact, so the
  system still comes up from published artifacts alone
  ([O6](../../../USE_CASES.md#o6--deployable)). Which platforms the binaries are built for is left
  to the release step that builds them.

What the ordinary path does that the design forbids, and what stops it:

| Construction | Harm | What stops it |
| --- | --- | --- |
| The UI importing the opening subsection | A compromised UI reads every stored grant | The UI's import list and its violation file ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)) |
| A public key passed where the private key belongs | Code that compiles and opens nothing, or a confusing failure at runtime | Distinct key types, and a must-not-compile case |
| Sealing without the context, or opening with a different one | A value moved between rows or purposes opens | A test of every context swap, and a mutation patch dropping the context |
| An altered, truncated or wrong-version value | Garbage or a forged grant reaches a provider | Tests of every single-bit flip, every truncation and a wrong version |
| A value sealed to a key the reader does not hold, reported as tampering | The operator hunts an attack when a key file is missing | A test that the unknown-key refusal is its own error |
| A reused sender across values | Two values share key material | One sender per value, and a test that two seals of the same plaintext differ |
| An opener started with a public key that matches none of its seeds | The deployable seals rotated credentials no opener can read | The start-up check on the key pair, with a failing test |

| Requirement | Met by |
| --- | --- |
| R1 | HPKE's AEAD, with the refusals tested bit by bit |
| R2 | The key identifier in the header |
| R3 | The context in the additional data |
| R4 | The standard library, no module added |
| R5 | Raw seed and public key files |
| R6 | RFC 9180 with draft-ietf-hpke-pq's registered suite and Go's bundled vectors |
| R7 | X-Wing's ML-KEM-768 half |
| R8 | Not met. The library's key-generation command stands in for a standard tool |
| R9 | The library's sealing and opening subsections |
| R10 | The Go release the module pins |

What an implementer would otherwise pay to discover:

- OpenSSL generates ML-KEM and X25519 keys but has no encoder for an X-Wing key, so the key pair
  comes from the library's command.
- A seal-only binary links no HPKE opening code, but it does link ML-KEM decapsulation primitives
  through the standard library's self-test. That does not breach the rule that the UI links no
  opening code, which concerns this project's opening subsection.
- Base mode authenticates the value, not who sealed it. Anyone holding the public key and able to
  write a row can plant a validly sealed value for that row. ADR-0081 claims only that an altered
  value is refused, and that holds.
- A 27-byte token seals to 1180 bytes, and a seal or an open takes about a tenth of a millisecond.

## Alternatives considered

The grades run from 4, strong, through 3, adequate, and 2, weak, to 1, which fails. R9 and R10 are
omitted, since every candidate scores 4 on both. Tink is graded in its HPKE X-Wing mode, the
configuration comparable with the chosen row.

| Candidate | R1 | R2 | R4 | R5 | R6 | R7 | R3 | R8 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- |
| **`crypto/hpke`, X-Wing** | 4 | 2 | 4 | 3 | 3 | 4 | 4 | 2 |
| `crypto/hpke`, ML-KEM-768 | 4 | 2 | 4 | 3 | 3 | 3 | 4 | 4 |
| `crypto/hpke`, DHKEM(X25519) | 4 | 2 | 4 | 4 | 4 | 1 | 4 | 4 |
| `golang.org/x/crypto/nacl/box` sealed box | 4 | 2 | 3 | 3 | 3 | 1 | 2 | 3 |
| `filippo.io/age` | 4 | 2 | 2 | 4 | 3 | 4 | 2 | 4 |
| `tink-go`, hybrid HPKE, X-Wing | 4 | 4 | 2 | 2 | 3 | 4 | 4 | 2 |

Every candidate but Tink scores 2 on R2, because its key identifier would be this project's header,
not the library's. Tink alone carries key identifiers in its keysets. R5 is 3 where the file needs
the project's own parsing, a raw seed for X-Wing or a PKCS#8 wrapper around an ML-KEM seed, and 4
where a standard library reads it as it stands. R4 was measured as stripped seal-only binaries
against a 1.59 MB baseline, 2.29 to 2.31 MB for `crypto/hpke`, 2.05 MB for `nacl/box`, 2.65 MB for
age and 6.83 MB for Tink, and graded on modules added. The rest were read from each project's
source and documentation.

| Candidate | Modules added | Key identity | Additional data | Primitive's standing |
| --- | --- | --- | --- | --- |
| `crypto/hpke`, X-Wing | None | Project header | Yes | Registered HPKE KEM, draft binding |
| `crypto/hpke`, ML-KEM-768 | None | Project header | Yes | FIPS 203, draft binding |
| `crypto/hpke`, DHKEM(X25519) | None | Project header | Yes | RFC 9180 |
| `nacl/box` | `x/crypto` | None | No | libsodium sealed box |
| age | Six | None, every identity is tried | No | age format |
| Tink | `x/crypto` and protobuf | Native | Yes | RFC 9180 and draft binding |

The grid does not separate the two post-quantum rows of `crypto/hpke`. X-Wing scores higher on R7
and ML-KEM-768 on R8, and the second table adds that ML-KEM-768's primitive is a published standard
where X-Wing's is a registered draft. The choice between them is a preference. A classical half
behind the post-quantum one, so that a break in ML-KEM alone leaves the stored grants sealed, is
valued above a published primitive and keys from a standard tool. A reader who weighs the published
primitive higher lands on ML-KEM-768, with the same header and the same exit.

Every other candidate has a weaker cell on something it cannot fix from outside: R7 for
DHKEM(X25519) and `nacl/box`, footprint and missing additional data for age, footprint and key
loading for Tink. Of the chosen row's strengths, the sealing cut is guarded elsewhere too, by the
UI's import list, while the context binding of R3 is guarded nowhere else. Tink's one outright
win, key identity, is had without Tink through the header. Its costs, protobuf and an API marked
insecure for cleartext keysets, cannot be separated from it. Regretting the choice costs a new
version byte and a re-seal, since the header, the key identifier and the key replacement of
[ADR-0092](./0092-key-replacement-by-keyring-and-re-seal.md) carry over to any HPKE suite.

- **`crypto/hpke`, X-Wing.** For it, no module, the context binding, and a hybrid that survives the
  failure of either half. Against it, its HPKE binding is a working-group draft and X-Wing's own
  specification is an individual draft
  ([draft-connolly-cfrg-xwing-kem](https://datatracker.ietf.org/doc/draft-connolly-cfrg-xwing-kem/)),
  so the standard it rests on is not yet an RFC. A later Go release that follows a changed
  specification could stop opening values sealed under the draft, so moving past such a release
  takes a re-seal while the old one still opens them. No standard tool writes its keys, so the
  project owns that command. The suite as a whole is not a FIPS-approved construction, though its
  ML-KEM-768 half is FIPS 203, and no record requires FIPS.
- **`crypto/hpke`, ML-KEM-768.** For it, its primitive is a published standard,
  [FIPS 203](https://csrc.nist.gov/pubs/fips/203/final), OpenSSL generates its keys, and it is the
  most widely implemented post-quantum KEM. Against it, it has no classical half, so a break in
  ML-KEM itself leaves nothing behind it, and its HPKE binding sits in the same draft as X-Wing's.
- **`crypto/hpke`, DHKEM(X25519).** For it, RFC 9180 from end to end, and keys straight from
  `openssl genpkey`. Against it, a stolen copy of the database is open to later quantum decryption
  for as long as its grants live.
- **[`nacl/box`](https://pkg.go.dev/golang.org/x/crypto/nacl/box).** For it, the libsodium
  sealed box, familiar and small. Against it, it takes no additional data, so a value cannot be
  bound to its row, and it is X25519 only. DHKEM(X25519) in the standard library is equal or better
  on every requirement.
- **[age](https://github.com/FiloSottile/age).** For it, a maintained format with a post-quantum
  mode and its own key tool. Against it, six modules, no additional data, and no key identifier,
  since [it tries every identity it holds](https://pkg.go.dev/filippo.io/age#Decrypt).
- **[Tink](https://github.com/tink-crypto/tink-go).** For it, key identity and rotation built in,
  with an HPKE X-Wing mode. Against it, protobuf in the trust anchor, key files that are keysets
  rather than plain keys, and cleartext private keysets only through
  [a package its own documentation calls dangerous](https://pkg.go.dev/github.com/tink-crypto/tink-go/v2/insecurecleartextkeyset).

## Consequences

- Leaving this construction costs a new version byte and re-sealing the stored values with the
  machinery of ADR-0092. The header, the key identifier and the context rule survive the exit.
- What would re-argue the decision. If draft-ietf-hpke-pq is published with a different identifier
  or combiner, the fix is a new version byte and a re-seal. If a FIPS requirement appears, an
  approved suite replaces this one the same way. If post-quantum protection stops being wanted,
  DHKEM(X25519) takes over with the same header.
- A stored value grows by about 1.1 KB over a classical suite.
- The context binds a value to its row, so moving a value, or changing an account's identifier,
  means re-sealing it.
- Losing the private key loses every stored credential and every OAuth client's secret, and the
  operator recovers as ADR-0081 states, by setting up each OAuth client again and re-authorizing
  each account.
- Assumptions about other components. The deployables that open credentials pin a Go release with
  `crypto/hpke`. The platform mounts the seed only into those deployables and the public key into
  the UI and them.
