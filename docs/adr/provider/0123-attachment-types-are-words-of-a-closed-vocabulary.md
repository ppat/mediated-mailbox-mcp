# 0123. A message's attachment types are words of a closed vocabulary, derived when read from each attachment's media type and extension, which ingest stores normalized

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [C1](../../../USE_CASES.md#c1--metadata-always-visible), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [P1](../../../USE_CASES.md#p1--one-contract)

## Context

[ADR-0001](../redaction/0001-redaction-matrix.md)'s matrix shows an attachment's type in every
sensitivity state and its filename only where the body is visible, because filenames are
body-derived and follow the body. Its example message carries `"attachment_types": ["pdf"]`, and
the mediator serves that field on every message. [ADR-0010](./0010-one-provider-port.md)'s
canonical model carried each message's attachment names and no types, so every message was served
with no types. The operator ruled on 2026-10-07 to give messages their attachment types before
[production point 1](../../../ROADMAP.md#production-point-1--the-read-path), from the adapter
through the canonical model, over dropping the field. That ruling left open what a type is and what
is stored. Ingest at backfill's first pass is the first consumer.

Both things a provider says of an attachment, its media type from the part's `Content-Type` and its
filename, are written by the sender. A type served as either one stands is sender-written text in
a field served in every state. A filename `invoice.419283` has the extension `419283`, and a
`Content-Type` of `application/x-419283` is as easy to send. No scanner reads the field and no
serve-time check reads it, so a raw type would carry body-derived text past the matrix that
withholds the filename.

Pass 1 leaves a message it already holds as it is, so whatever ingest stores about a message is
what the index has for it until the provider is asked again. A value fixed at ingest that a later
version of the mapping would compute differently costs a refetch of the corpus's metadata to
correct. The operator ruled on 2026-10-10 that ingest stores the inputs the mapping reads and that
the mapping runs in code when the types are read, so a change to the vocabulary or to the mapping
needs no migration and no refetch, and that the inputs stored are each attachment's media type and
extension, with no filename at rest.

## Decision

- **A type is one word of a closed vocabulary**, never the media type or the extension as the
  message gives it. The words are `pdf`, `image`, `audio`, `video`, `text`, `calendar`, `contact`,
  `document`, `spreadsheet`, `presentation`, `archive`, `message`, `signature` and `other`. A value
  outside the vocabulary cannot be produced by the mapping, so no text the sender wrote reaches the
  field.
- **Ingest stores each attachment's normalized media type and extension, and no type.** The table
  `attachment_media` holds one row per distinct pair a message's attachments carry, keyed by the
  account and the message, and nothing else of the attachment. The filename is not stored, since
  ADR-0001 treats it as body-derived, and it reaches a client only fetched from the provider with a
  released body. The two inputs are the least of the attachment that any rule of the mapping reads
  or a later rule plausibly would. The extension is stored for every attachment, not only for one
  sent as `application/octet-stream`, so a later rule may read it.
- **The inputs are normalized in the canonical model before they are stored**, so the sender text
  at rest is short and inert. The media type is cut at its first `;`, trimmed, lowercased, and kept
  only when it is a type and a subtype, each one to 127 characters of the restricted-name
  characters [RFC 6838](https://www.rfc-editor.org/rfc/rfc6838#section-4.2) allows, and stored
  empty otherwise. The extension is the text after the filename's last `.`, lowercased, and kept
  only when it is one to 16 ASCII letters and digits, and stored empty otherwise. No standard bounds
  an extension's length, and 16 is twice the longest one the mapping names, `numbers`, with room to
  spare. A check on each column of `attachment_media` holds it to the normalized form, so a writer
  that bypassed the normalization fails at insert.
- **The mapping is one pure function of the canonical model** in `core/mail`, which runs where the
  types are served, over the stored pairs. Media types are
  [RFC 2046](https://www.rfc-editor.org/rfc/rfc2046) and
  [IANA](https://www.iana.org/assignments/media-types/media-types.xhtml) vocabulary rather than any
  provider's, so no adapter invents its own mapping, and Gmail and JMAP both report a part's
  `Content-Type` and name. The Go type of a word is a named string type whose values the package
  declares as constants, and the mapping is the only function that returns one. Every operation
  that serves a message's types, on both roots, serves the mapping's output and never a stored
  value.
- **The media type decides,** by this table, the first matching row winning.

  | Media type | Word |
  | --- | --- |
  | `application/pdf` | `pdf` |
  | `text/calendar`, `application/ics` ([RFC 5545](https://www.rfc-editor.org/rfc/rfc5545)) | `calendar` |
  | `text/vcard`, `text/x-vcard`, `text/directory` ([RFC 6350](https://www.rfc-editor.org/rfc/rfc6350)) | `contact` |
  | `application/pkcs7-signature`, `application/x-pkcs7-signature` ([RFC 8551](https://www.rfc-editor.org/rfc/rfc8551)), `application/pgp-signature` ([RFC 3156](https://www.rfc-editor.org/rfc/rfc3156)) | `signature` |
  | `text/csv` ([RFC 4180](https://www.rfc-editor.org/rfc/rfc4180)), `text/tab-separated-values` | `spreadsheet` |
  | `application/msword`, `application/rtf`, `text/rtf`, `application/vnd.oasis.opendocument.text`, `application/vnd.apple.pages`, and any type starting `application/vnd.openxmlformats-officedocument.wordprocessingml.` or `application/vnd.ms-word.` | `document` |
  | `application/vnd.ms-excel`, `application/vnd.oasis.opendocument.spreadsheet`, `application/vnd.apple.numbers`, and any type starting `application/vnd.openxmlformats-officedocument.spreadsheetml.` or `application/vnd.ms-excel.` | `spreadsheet` |
  | `application/vnd.ms-powerpoint`, `application/vnd.oasis.opendocument.presentation`, `application/vnd.apple.keynote`, and any type starting `application/vnd.openxmlformats-officedocument.presentationml.` or `application/vnd.ms-powerpoint.` | `presentation` |
  | `application/zip`, `application/x-zip-compressed`, `application/gzip`, `application/x-gzip`, `application/x-tar`, `application/x-bzip2`, `application/x-7z-compressed`, `application/vnd.rar`, `application/x-rar-compressed` | `archive` |
  | `image/*` | `image` |
  | `audio/*` | `audio` |
  | `video/*` | `video` |
  | `message/*`, which holds a forwarded message | `message` |
  | `text/*` not named above | `text` |
  | `application/octet-stream`, or no media type | the extension decides, below |
  | Anything else | `other` |

- **The extension decides only when the media type says nothing.** RFC 2046 makes
  `application/octet-stream` the type of arbitrary binary data, and mail clients send it for a file
  they cannot name a type for. Then the extension picks a word, and an extension the table does not
  name, or none, is `other`.

  | Extension | Word |
  | --- | --- |
  | `pdf` | `pdf` |
  | `jpg`, `jpeg`, `png`, `gif`, `bmp`, `tif`, `tiff`, `webp`, `heic`, `heif`, `svg` | `image` |
  | `mp3`, `m4a`, `wav`, `ogg`, `oga`, `flac`, `aac`, `amr` | `audio` |
  | `mp4`, `m4v`, `mov`, `avi`, `mkv`, `webm`, `3gp` | `video` |
  | `txt`, `log`, `md` | `text` |
  | `ics`, `vcs` | `calendar` |
  | `vcf` | `contact` |
  | `p7s`, `sig` | `signature` |
  | `doc`, `docx`, `docm`, `dot`, `dotx`, `odt`, `rtf`, `pages` | `document` |
  | `xls`, `xlsx`, `xlsm`, `xlt`, `xltx`, `ods`, `csv`, `tsv`, `numbers` | `spreadsheet` |
  | `ppt`, `pptx`, `pptm`, `pps`, `ppsx`, `odp`, `key` | `presentation` |
  | `zip`, `gz`, `tgz`, `tar`, `bz2`, `xz`, `7z`, `rar` | `archive` |
  | `eml`, `msg` | `message` |

- **Each word earns its place by a question an agent asks of metadata.** `pdf` is ADR-0001's own
  example and the commonest document attachment. `document`, `spreadsheet` and `presentation` split
  office files the way a person describes them. `image`, `audio` and `video` are the IANA top-level
  types, and `text` and `message` the two others an attachment carries. `calendar` is an
  invitation and `contact` a contact card, each a `text/` subtype that `text` would misdescribe.
  `archive` is a file that holds files. `signature` is the S/MIME or PGP signature a signed message
  carries as an attachment, which would otherwise make every signed message read as one with an
  attachment of no known kind. `other` is everything else.
- **A message's types are a set**, each word once, sorted, and empty when the message has no
  attachment, and its stored pairs are a set too, each pair once. A list with an entry per
  attachment would run parallel to the attachment names by position, braiding the two fields, and
  providers order parts differently, so a set keeps the contract's comparison independent of part
  order. A count of attachments by type is not metadata the matrix names, and a row per attachment
  belongs to a design for retrieving attachments, which this record leaves open.
- **The pairs, the names and whether the message has attachments describe the same parts.** The
  canonical model sets all three from one list of the message's attachments as its adapter reads
  them, so a message never has attachments with no pairs or pairs with no attachment. An
  attachment is a part with a filename, as the adapters already counted it.
- **The pairs are written once, when a message is first stored**, by backfill's first pass and by
  delta sync's insert of an added message, both through the decisions in `core/index`. A held
  message's attachments do not change, so no runtime role may update or delete a pair. A message
  delta sync removes takes its pairs with it, through the table's foreign key, since they are part
  of the message's metadata as a column of it would be.

## Alternatives considered

- **The type stored in a column of `messages`, held to the vocabulary by a check.** The case for it
  is that the stored value is already inert and a read needs no join. A word added, or a rule of
  the mapping changed, after production point 1 would cost a migration replacing the check and a
  refetch of the metadata of every message the change reaches, which for Gmail is 61 quota units
  for each three messages. The operator ruled it out on 2026-10-10 for the stored inputs.
- **The full filename stored beside the media type.** The case for it is that a later mapping rule
  could read any part of the name. ADR-0001 treats filenames as body-derived, and today they reach
  a client only fetched from the provider with a released body, after the serve-time pattern check
  reads them. Storing them would put unscanned sender text at rest for the first time. The operator
  ruled for the extension alone.
- **A row per attachment, with its name's position.** The case for it is that a count of attachments
  by type falls out of it. No reader of the index asks for one, and a row per attachment belongs to
  the design for retrieving attachments, which this record leaves open.
- **The media type served as the message gives it, parameters dropped.** The case for it is
  fidelity, since `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` says more
  than `spreadsheet`, and no table needs keeping. It is sender-written text in a field served in
  every state, an open channel past the matrix.
- **The filename's extension served as it stands, which ADR-0001's example `pdf` could be read
  as.** The case for it is that `pdf` is what a person calls the file. The extension is part of the
  filename ADR-0001 withholds outside a released body, so it carries body-derived text the same
  way.
- **The media type's top-level type alone** (`application`, `image`, `text` and the others). The case
  for it is that the vocabulary is IANA's own, with no table. Nearly every document, spreadsheet,
  presentation, archive and PDF is `application`, so the word would tell an agent nothing about the
  attachments it most often asks about.
- **A wider vocabulary,** such as words for executables, fonts, scripts or keys. The case for it is
  completeness. Each was left out for lack of a question an agent asks with it, and with the inputs
  stored a word added later is a change to the mapping alone.
- **The extension consulted whenever the media type maps to `other`,** not only when it is
  `application/octet-stream` or missing. The case for it is fewer `other` words. A specific media
  type the table does not know is a statement about the part that the extension should not
  overrule, and the generic type is the one RFC 2046 defines as saying nothing.
- **More generic media types,** such as `application/force-download` or `binary/octet-stream`,
  treated like `application/octet-stream`. The case for it is that some servers send them. Their
  frequency in the operator's mail has not been measured, and adding one later is a change to the
  mapping alone, which reaches every stored message.
- **The media type and extension stored as the message gives them.** The case for it is that a later
  rule could read anything the sender wrote there. Parameters and an over-long or non-ASCII value
  carry nothing a mapping from media types and extensions reads, and keeping them would put
  arbitrary sender text at rest.
- **A Go type that no conversion can make,** such as a struct holding an unexported word, with the
  values returned by functions. The case for it is that `AttachmentType("x")` would not compile
  outside the package. A pure core declares no package-level value but a constant or an error
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)), so each word would be a
  function, and the words are served only as the mapping's output.
- **The mapping in each adapter.** The case for it is that an adapter knows its provider's shapes.
  Media types are not a provider's concept, and two adapters with two tables would give one
  attachment two types, which the contract suite could then catch only by comparing their tables.

## Consequences

- An adapter supplies each attachment's media type and filename to the canonical model, which
  normalizes them, and the contract suite checks the pairs every implementation reports against
  pairs written out in the shared fixtures, so an adapter that drops or misreads a part's media type
  fails it. Gmail's metadata mask already names each part's type and filename, so reading them costs
  no quota.
- The closed vocabulary rests on code, the mapping returning only its constants and every serving
  path calling it, which tests hold, and not on a check in the schema. The vocabulary is held in the
  mapping and in the tests that write each word out, and a word added is a change to both.
- A change to the mapping or the vocabulary reaches every stored message at its next read, since
  nothing derived is stored. A change to the normalization, or to what is stored, reaches only
  messages stored afterwards, which is why the inputs land before production point 1, while no
  corpus is held.
- A forwarded message, a signed message's signature and an inline image each count as an
  attachment when its part has a filename, as the names already did.
