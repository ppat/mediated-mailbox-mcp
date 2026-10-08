# 0100. Message text with no HTML form is released as a fenced code block that shows it exactly

**Status:** Accepted ·
**Pillar:** [Metadata always flows; sensitive bodies never do](../../../DESIGN.md#metadata-always-flows-sensitive-bodies-never-do) ·
**Serves:** [A4](../../../USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything)

## Context

Every released body is clean Markdown, converted from its HTML by the shared conversion
([ADR-0036](./0036-released-bodies-are-clean-markdown.md), [ADR-0074](./0074-html-to-markdown-v2-converts-bodies.md)).
ADR-0036 leaves one case to where serving is built: a body with no HTML part must not pass through
the conversion as HTML, which would drop any text in angle brackets. The same question reaches two
fields the redaction matrix releases with the body
([ADR-0001](./0001-redaction-matrix.md)). The snippet is the provider's plain-text preview, and the
Gmail adapter decodes its character references, so it can hold any characters, markup included. An
attachment's filename is whatever the sender named it.

Plain text read as Markdown is not inert. A line can be a heading, `[label](target)` is a link,
`![](https://…)` is a remote image a client may fetch, and a tag is raw HTML that CommonMark passes
through. Each one falsifies [A4](../../../USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything).

## Decision

- **Text from a message that has no HTML form is released as one CommonMark fenced code block.** The
  fence is a run of backticks longer than the longest run of backticks in the text, and at least
  three, with no info string, so no line of the text can close the block. A CommonMark reader shows
  the text exactly, as code, and sees no link, image, heading or raw HTML in it.
- **It applies to three texts.** The text part of a body that has no HTML part, the snippet, and each
  attachment filename. A body that has an HTML part is released as that part's conversion, and its
  text part is not released.
- **The form lives beside the conversion,** in `content/markdown`, so every Markdown a client
  receives from a message comes from one library, and its proof reads the output with the same
  independent CommonMark parser the conversion's proof uses.

## Alternatives considered

- **Escape the text as HTML and run it through the conversion.** For it, one path for every body,
  and the library already exercised. Against it, the conversion escapes Markdown punctuation with
  backslashes, which lands inside URLs, so an underscore in a link's query reaches the client
  changed, and the serve-time pattern check of
  [ADR-0002](./0002-fetch-time-re-evaluation.md) would read a text different from the one backfill's
  scanner read, which scans the text part as it arrived.
- **Release the text as it is.** For it, nothing changes it. Against it, raw HTML and a Markdown
  remote image reach the client, which A4 names as falsifying.
- **Deny a body with no HTML part.** For it, nothing to decide. Against it, every plain-text message
  is withheld though the gate released it, which is over-redaction with no safety gained.
- **Release the snippet and filenames as plain JSON strings.** For it, short values need no
  formatting. Against it, both are message text that can hold markup, and a client rendering
  Markdown would render it.

## Consequences

- An agent reads a plain-text message, its snippet and its filenames as code blocks. The snippet
  and the filenames are released beside the wrapped body, outside its untrusted-content delimiters,
  with the delimiters' words replaced in them as in the body
  ([ADR-0036](./0036-released-bodies-are-clean-markdown.md)).
- The text inside the block is the text as it arrived, so a URL in it survives intact, and the
  serve-time pattern check reads the same characters backfill's scanner read in the text part.
- Assumptions about other components. Clients read released text as CommonMark, or as plain text.
  A reader that does not honour fenced code blocks sees the text with a fence line before and after
  it, which is still the text.
