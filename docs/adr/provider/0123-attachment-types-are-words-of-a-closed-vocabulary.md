# 0123. A message's attachment types are words of a closed vocabulary, mapped once in the canonical model from each attachment's media type, and carried as a sorted set

**Status:** Accepted ·
**Pillar:** [Unsafe states are unconstructable, not merely untaken](../../../DESIGN.md#unsafe-states-are-unconstructable-not-merely-untaken) ·
**Serves:** [C1](../../../USE_CASES.md#c1--metadata-always-visible), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [P1](../../../USE_CASES.md#p1--one-contract)

## Context

[ADR-0001](../redaction/0001-redaction-matrix.md)'s matrix shows an attachment's type in every
sensitivity state and its filename only where the body is visible, because filenames are
body-derived and follow the body. Its example message carries `"attachment_types": ["pdf"]`.
[ADR-0016](../data/0016-schema.md) gives the types a column, `messages.attachment_types`, and the
mediator serves it on every message. [ADR-0010](./0010-one-provider-port.md)'s canonical model
carried each message's attachment names and no types, so nothing wrote the column and every
message was served with no types. The operator ruled on 2026-10-07 to populate the column before
[production point 1](../../../ROADMAP.md#production-point-1--the-read-path), from the adapter
through the canonical model, over dropping the column and the field it serves. That ruling left
open what a type is. Ingest at backfill's first pass is the first consumer.

Both things a provider says of an attachment, its media type from the part's `Content-Type` and its
filename, are written by the sender. A type taken from either as it stands is sender-written text
in a field served in every state. A filename `invoice.419283` has the extension `419283`, and a
`Content-Type` of `application/x-419283` is as easy to send. No scanner reads the field and no
serve-time check reads it, so a raw type would carry body-derived text past the matrix that
withholds the filename.

## Decision

- **A type is one word of a closed vocabulary**, never the media type or the extension as the
  message gives it. The words are `pdf`, `image`, `audio`, `video`, `text`, `calendar`, `contact`,
  `document`, `spreadsheet`, `presentation`, `archive`, `message`, `signature` and `other`. A value
  outside the vocabulary cannot be produced by the mapping, so no text the sender wrote reaches the
  field.
- **The mapping is one pure function of the canonical model** in `core/mail`, which every adapter
  calls with what its provider says of each attachment, its media type and its filename. Media
  types are [RFC 2046](https://www.rfc-editor.org/rfc/rfc2046) and
  [IANA](https://www.iana.org/assignments/media-types/media-types.xhtml) vocabulary rather than any
  provider's, so no adapter invents its own mapping, and Gmail and JMAP both report a part's
  `Content-Type` and name. The Go type of a word is a named string type whose values the package
  declares as constants, and the mapping is the only function that returns one.
- **The media type decides.** Its parameters are dropped, it is compared without regard to case,
  and it maps by this table, the first matching row winning.

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
  | `application/octet-stream`, or no media type | the filename decides, below |
  | Anything else | `other` |

- **The filename decides only when the media type says nothing.** RFC 2046 makes
  `application/octet-stream` the type of arbitrary binary data, and mail clients send it for a file
  they cannot name a type for. Then the filename's last extension, compared without regard to
  case, picks a word, and an extension the table does not name, or no extension, is `other`.

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
  attachment. A list with an entry per attachment would run parallel to the attachment names by
  position, braiding the two fields, and providers order parts differently, so a set keeps the
  contract's comparison independent of part order. A count of attachments by type is not
  metadata the matrix names, and a row per attachment with its own type belongs to a design for
  retrieving attachments, which this record leaves open.
- **The types, the names and whether the message has attachments describe the same parts.** The
  canonical model sets all three from one list of the message's attachments as its adapter reads
  them, so a message never has attachments with no types or types with no attachment. An
  attachment is a part with a filename, as the adapters already counted it.
- **The column holds only the vocabulary.** `messages.attachment_types` carries a check holding
  every element to the vocabulary, in ADR-0016's tier of checks next to safety, so a writer that
  bypassed the mapping fails at insert rather than storing text.
- **The types are written once, when a message is first stored**, by backfill's first pass and by
  delta sync's insert of an added message, both through the decisions in `core/index`. A held
  message's attachments do not change, so no runtime role may update the column.

## Alternatives considered

- **The media type as the message gives it, parameters dropped.** The case for it is fidelity, since
  `application/vnd.openxmlformats-officedocument.spreadsheetml.sheet` says more than
  `spreadsheet`, and no table needs keeping. It is sender-written text in a field served in every
  state, an open channel past the matrix, and a check on the column could not hold an open set.
- **The filename's extension as it stands, which ADR-0001's example `pdf` could be read as.** The
  case for it is that `pdf` is what a person calls the file. The extension is part of the filename
  ADR-0001 withholds outside a released body, so it carries body-derived text the same way.
- **The media type's top-level type alone** (`application`, `image`, `text` and the others). The case
  for it is that the vocabulary is IANA's own, with no table. Nearly every document, spreadsheet,
  presentation, archive and PDF is `application`, so the word would tell an agent nothing about the
  attachments it most often asks about.
- **A wider vocabulary,** such as words for executables, fonts, scripts or keys. The case for it is
  that a refetch is the cost of adding a word once the corpus is held, since a stored `other` cannot
  be told apart without the provider. Each was left out for lack of a question an agent asks with
  it. A word added later costs a migration replacing the check and a refetch of the metadata of
  every message stored as `other`, which for Gmail is 61 quota units for each three messages.
- **The extension consulted whenever the media type maps to `other`,** not only when it is
  `application/octet-stream` or missing. The case for it is fewer `other` words. A specific media
  type the table does not know is a statement about the part that the extension should not
  overrule, and the generic type is the one RFC 2046 defines as saying nothing.
- **More generic media types,** such as `application/force-download` or `binary/octet-stream`,
  treated like `application/octet-stream`. The case for it is that some servers send them. Their
  frequency in the operator's mail has not been measured, and adding one later is a change to the
  mapping alone, which reaches messages stored afterwards.
- **A list per message, one entry per attachment.** The case for it is that a count by type falls
  out of it. Rejected above, as braiding the names by position and making the contract compare part
  order.
- **A Go type that no conversion can make,** such as a struct holding an unexported word, with the
  values returned by functions. The case for it is that `AttachmentType("x")` would not compile
  outside the package. A pure core declares no package-level value but a constant or an error
  ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)), so each word would be a
  function, and the check on the column refuses a forged word at insert anyway.
- **The mapping in each adapter.** The case for it is that an adapter knows its provider's shapes.
  Media types are not a provider's concept, and two adapters with two tables would give one
  attachment two types, which the contract suite could then catch only by comparing their tables.

## Consequences

- An adapter supplies each attachment's media type and filename to the canonical model's mapping,
  and the contract suite checks the types every implementation reports against types written out in
  the suite, so a wrong mapping in an adapter fails it. Gmail's metadata mask already names each
  part's type and filename, so reading the types costs no quota.
- The vocabulary is held in three places that must agree, the mapping in `core/mail`, the check on
  `messages.attachment_types` in the schema, and the tests that write each word out. A word added
  is a change to all three.
- A message stored before its adapter reported a type it now reports keeps its stored types,
  because a held message's row is not rewritten. That is why the types land before production point
  1, while no corpus is held.
- The type of a forwarded message, a signed message's signature and an inline image each count as
  an attachment when its part has a filename, as the names already did.
