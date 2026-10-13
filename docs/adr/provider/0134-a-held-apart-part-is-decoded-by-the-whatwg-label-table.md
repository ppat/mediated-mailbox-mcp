# 0134. The Gmail adapter decodes a part held apart in its declared charset through the WHATWG label table of `golang.org/x/text`, refusing a label the table does not name or a UTF-16 label

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)

## Context

A rendered part Gmail holds apart from the message is fetched by `messages.attachments.get`
([ADR-0131](./0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md)). Its data comes
in the charset the part declares in its `Content-Type`, while a part that comes inline comes
transcoded to UTF-8, as one observer of the Gmail API reported
([khuisman/mcp-gee-sweet issue #825](https://github.com/khuisman/mcp-gee-sweet/issues/825)). The
port returns text, so the adapter turns the declared charset's bytes into UTF-8 before it checks the
decoded length against the bound and returns the part.

What is settled already leaves a narrow choice. The adapter is the only place that reads the label,
in one function of its pure core, and the conversion and scanning above the port read UTF-8 alone. One Go module
serves the repository ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)), and the
adapter's import list names what it may link. So the choice is which table maps a declared label to
a decoder, and what happens to a label the table does not name.

It looks like a question of character sets and is a question of labels. Every candidate decodes the
common charsets. What separates them is which spellings of a label they accept, since mail clients
write labels such as `cp1252` and `utf8` that the
[IANA registry](https://www.iana.org/assignments/character-sets/character-sets.xhtml) does not
list, and `ks_c_5601-1987` and `gb2312`, which it lists but which `x/text`'s IANA index holds no
decoder for, and what they do when they meet a label they do not know. A second question hides
inside the first. A label a table resolves can still be read as other text than the part holds, so
the decision is also which resolved encodings the adapter refuses.

| Requirement | What it demands | From |
| --- | --- | --- |
| Labels mail declares | Resolve the labels real mail carries, the informal spellings mail clients write included | This record |
| Fail closed | A label that resolves to no decoder, or to one that decodes nothing, refuses the body rather than guessing | [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere), ADR-0131's refusal of a part the adapter cannot decode |
| No silent misreading | A label the table resolves decodes to the text the sender's mail client shows, or is refused | [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) |
| Footprint | No module outside those the repository already requires, and no HTML parser linked into the adapter, since the conversion's family is the one place the parser is admitted | ADR-0071, [content/README.md](../../../content/README.md) |
| Maintained | Maintained by an owner who will keep the table current | This record |

Fail closed and no silent misreading ordered the field together, because a guessed charset is a
body released or scanned as other text than it holds, and nothing above the adapter can tell
mojibake from text. Labels mail declares broke the tie between the candidates that met both.

## Decision

- **`golang.org/x/text/encoding/htmlindex`'s `Get` maps the label to a decoder.** It implements the
  label table of the [WHATWG Encoding Standard](https://encoding.spec.whatwg.org/#names-and-labels),
  whose encodings, UTF-7 apart, are the ones Thunderbird decodes mail by
  ([hsivonen/charset](https://docs.rs/crate/charset/latest)). When this was decided the package
  agreed with the standard's current
  [`encodings.json`](https://encoding.spec.whatwg.org/encodings.json) on every one of its 228
  labels. `golang.org/x/text` is already among the module's requirements, so no module is added, and
  the adapter's import list admits this package and `golang.org/x/text/encoding`, the encoding
  interface it returns. The table's package imports `fmt`, which imports `os`, so it fails the first
  of [ADR-0071](../engineering/0071-static-enforcement-toolchain.md)'s conditions for a pure core's
  imports. The adapter passes the table to its pure core, `provider/gmail/core`, which decides on
  the label and decodes through the encoding interface, a package that meets the conditions.
- **A label the table does not name is refused**, with the port's refusal, and so is a label the
  table maps to the `replacement` encoding, which decodes any input to a single replacement
  character. A missing label, a part with no `Content-Type` or one naming no charset, is read as
  `us-ascii`, the default [RFC 2045](https://www.rfc-editor.org/rfc/rfc2045#section-5.2) gives a
  text part. A `Content-Type` that is present and does not parse is refused rather than read as
  missing, since the charset it names, which cannot be read, may be any. Go's parser refuses forms
  real mail carries, RFC 2045's own comment form, a parameter given twice and an empty parameter
  among them.
- **A label the table maps to UTF-16 is refused.** MIME does not allow UTF-16 in a text part, as
  [RFC 2781 Appendix A](https://www.rfc-editor.org/rfc/rfc2781#appendix-A) states, and the table
  reads an unmarked `utf-16` as little-endian where
  [RFC 2781 section 4.3](https://www.rfc-editor.org/rfc/rfc2781#section-4.3) reads it as
  big-endian, with decoders that ignore a byte-order mark, so the byte order would be guessed and a
  big-endian part would read as CJK text it does not hold. That covers every label the table maps
  to `utf-16le` or `utf-16be`, `utf-16`, `ucs-2` and `unicode` among them.
- **A label the table maps to `x-user-defined` is refused.** It is a browser-internal encoding that
  maps every byte above 0x7F to a private-use character, so the text it decodes carries none of
  what those bytes held.
- **The decoded text is checked against the bound**, since a decoder can grow the input, up to three
  bytes of UTF-8 for each byte of a single-byte charset.

| Requirement | Mechanism |
| --- | --- |
| Labels mail declares | The WHATWG label table, which names the informal spellings mail clients write |
| Fail closed | An unknown label and the `replacement` encoding refuse the body with `ErrRefused` |
| No silent misreading | A label the table maps to `utf-16le`, `utf-16be` or `x-user-defined`, and a `Content-Type` that does not parse, refuse the body with `ErrRefused` |
| Footprint | The table's package and the encoding interface it returns, both of a module already required, no HTML parser |
| Maintained | The Go project's `x/text` repository |

What the implementer would otherwise pay to discover.

- The table maps `iso-8859-1` and `us-ascii` to windows-1252, as browsers do, so the bytes 0x80 to
  0x9F decode to the characters windows-1252 puts there, 0x80 to "€".
- Bytes a charset does not define decode to U+FFFD, so the result is always valid UTF-8.
- A byte-order mark is neither honoured nor stripped. The standard's own decode lets a BOM override
  the label, and `x/text` keeps that step apart, as
  [`unicode.BOMOverride`](https://pkg.go.dev/golang.org/x/text/encoding/unicode#BOMOverride), which
  the adapter does not apply. So a UTF-8 part that carries a BOM starts with U+FEFF.
- The package links every encoding's tables. A stripped static binary measured about 0.85 MiB
  larger with the decode than without it.

## Alternatives considered

The scale is 4 meets the requirement fully, 3 meets it with a cost, 2 meets it in part, and 1
fails it. The label, fail-closed and misreading columns were measured by resolving a list of labels
through each candidate and decoding bytes whose text is known, and the footprint and maintenance
columns read from each package's documentation. The `htmlindex` row grades it with the UTF-16 and
`x-user-defined` refusals this record adds, without which its misreading cell is 2.

| Candidate | Labels mail declares | Fail closed | No silent misreading | Footprint | Maintained |
| --- | --- | --- | --- | --- | --- |
| [`x/text/encoding/htmlindex`](https://pkg.go.dev/golang.org/x/text/encoding/htmlindex), as decided | 3 | 4 | 4 | 4 | 4 |
| [`x/net/html/charset`](https://pkg.go.dev/golang.org/x/net/html/charset)'s `Lookup` | 3 | 4 | 2 | 2 | 4 |
| [`x/text/encoding/ianaindex`](https://pkg.go.dev/golang.org/x/text/encoding/ianaindex)'s `MIME` index | 2 | 2 | 2 | 4 | 4 |
| The standard library alone, UTF-8, ASCII and Latin-1 | 1 | 4 | 4 | 4 | 4 |

Maintenance separated nobody, since every candidate is the Go project's. The misreading column, the
label column and the footprint column did the separating. The WHATWG table resolved every label
measured, among them `utf-8`, `us-ascii`, `iso-8859-1`, `windows-1252`, `cp1252`, `shift_jis`,
`iso-2022-jp`, `gb2312`, `gbk`, `big5`, `euc-kr`, `ks_c_5601-1987` and `koi8-r`, and it resolves
`utf-16`, which this record refuses. It refused `x-unknown`, and the labels some mail clients write
that it does not name, `utf-7`, `cp932`, `cp949` and `latin-9` among them, are refusals, never
misreadings. The IANA index refused `UTF8` and `cp1252`, and resolved `ks_c_5601-1987`, `gb2312`
and six other labels to no decoder while reporting no error, so its fail-closed cell rests on the
caller checking for a nil encoding as well as an error.

The reading. `htmlindex` with its refusals is the only candidate with no cell below 3, and the
standard library's fails the label column outright. Nothing above the adapter detects mojibake, so
a silent misreading has no other guard, which is why one outweighs the breadth of labels. Leaving
the choice costs one function. `x/net/html/charset` reads the same table and also sniffs a charset
from an HTML document, which the adapter has no use for, and it links the HTML parser into the
adapter.

- **`x/text/encoding/htmlindex`.** For it, the table Thunderbird decodes mail by, UTF-7 apart, with
  every guess removed. Against it, it refuses UTF-7 and the Java aliases Thunderbird accepts, so a
  mail-only label it does not name refuses a body a mail client decodes, and it keeps a U+FEFF at
  the start of a UTF-8 part that carries a BOM.
- **`x/net/html/charset`.** For it, the same table, and sniffing should a part ever declare no
  charset. Against it, it links an HTML parser into the adapter to read a label, and it misreads
  UTF-16 as the table does.
- **`x/text/encoding/ianaindex`.** For it, the registry mail standards cite, and it reads UTF-16 as
  RFC 2781 does, with the BOM honoured and big-endian as the default. Against it, it resolves
  registered labels to no decoder without an error, which a caller that checks only the error would
  read as success, and it reads `iso-8859-1` as true Latin-1, so the quotes of windows-1252 mail
  labelled `iso-8859-1`, a mislabel the standard and Thunderbird both read as windows-1252, become
  C1 control characters.
- **The standard library alone.** For it, nothing linked, about 0.85 MiB smaller. Against it, every
  body in windows-1252 or any East Asian charset is refused.
- **`htmlindex` with the standard's BOM override applied.** For it, the standard's full decode,
  under which a BOM settles a UTF-16 part's byte order. Not chosen, because it adds a second,
  data-driven charset decision, where the bytes can switch the decoder away from the declared label,
  for parts MIME does not allow in the first place.
- **A mail table of its own, `htmlindex` with UTF-7 and the JavaMail aliases added, as Thunderbird
  uses.** For it, the closest match to a real mail client. Not chosen, because it means an owned
  alias table and an owned UTF-7 decoder on a path real Gmail does not reach at the bound
  ([ADR-0131](./0131-the-body-is-one-rendered-part-bounded-by-one-size-constant.md)). The trigger to
  add an alias in front of `htmlindex.Get` is the live run recording a refused label that mail
  clients decode, once Gmail holds a part apart within the bound.

## Consequences

- Leaving the choice costs one function in the adapter's pure core and its test.
- A label the live run shows real mail carrying and the table does not name, or one the table maps
  to another charset than mail means by it, re-argues the choice.
- A UTF-16 text part held apart is refused. The live run's mail showing such parts re-argues this,
  toward the BOM override with RFC 2781's big-endian default.
- Every binary that links the adapter carries every encoding's tables, about 0.85 MiB.
- Assumptions about other components. The part's `Content-Type` reaches the adapter, which is why
  the body fetch's field mask keeps the headers (ADR-0131).
- The refusals of an unknown label, a UTF-16 label and `x-user-defined` are controls, catalogued
  with ADR-0131's decode row in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
