# content

A family, one narrow, named shared library holding several packages of one concept, published as
`mediated-mailbox-content`. Shared code is pure, or it is a library like this one that argues its
own case ([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its
case. The conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Its concept is [message content](../DESIGN.md#glossary), the code that reads a message's content in
memory and stores none of it, which bodies living in memory only governs
([ADR-0009](../docs/adr/redaction/0009-scanner-verdicts-carry-no-content.md)). The decision it hides
is how content is converted, so every component that reads it reads the same form.

| Package | Holds |
| --- | --- |
| `markdown/` | The conversion of a body's HTML to clean Markdown, and the literal form of a body that has no HTML |

The root package `content` holds no code. Its list, `content-without-converter` in
`.golangci.yaml`, covers every file of the family outside `markdown`, so the converter is admitted
in `markdown` alone.

**What does not belong.** Anything that stores content, any statement, and any decision about what
is released. What is released around the Markdown, the untrusted-content delimiters and the
serve-time pattern check ([ADR-0002](../docs/adr/redaction/0002-fetch-time-re-evaluation.md)), is
the mediator's alone and sits in its own pure core, and what is withheld is the Content Scanner's
and the gates'.

## The conversion to Markdown, `markdown`

Every released body is HTML converted to clean Markdown by an existing library
([ADR-0036](../docs/adr/redaction/0036-released-bodies-are-clean-markdown.md)), and the Content
Scanner reads the same Markdown ([ADR-0005](../docs/adr/classification/0005-tiered-detection.md)).
The mediator converts a body when it serves it, and backfill and delta sync convert it before
scanning. A verdict recorded at either is trusted when the mediator later serves the body, so the
conversions must produce the same Markdown from the same HTML. The conversion calls outside code, so
it cannot sit in `core/`, and a copy of its configuration in each deployable could drift apart
unseen. So one package holds it, `content/markdown`, and the mediator, backfill and delta sync each
import it. Text from a message that has no HTML form is released as a fenced code block that shows
it exactly
([ADR-0100](../docs/adr/redaction/0100-message-text-without-html-is-released-as-a-literal-code-block.md)),
and that form is `content/markdown`'s too, so every Markdown a client receives from a message comes
from this package.

Only `content/markdown` may import the converter library. Its property test reads the output with
[goldmark](https://github.com/yuin/goldmark), an independent CommonMark parser, to find what a
Markdown reader would see as a link, an image or raw HTML. Only this package's tests may import it.
