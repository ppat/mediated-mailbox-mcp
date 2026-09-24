# sanitize

A narrow, named shared library, published as `mediated-mailbox-sanitize`. Shared code is pure, or it
is a library like this one that argues its own case
([ADR-0050](../docs/adr/engineering/0050-shared-code-pure-or-narrow.md)), and this is its case. The
conventions it shares with every component are
[CLAUDE.md](../CLAUDE.md#code-layout-and-conventions)'s.

Every released body is HTML converted to clean Markdown by an existing library
([ADR-0036](../docs/adr/redaction/0036-released-bodies-are-clean-markdown.md)), and the Content
Scanner reads the same Markdown ([ADR-0005](../docs/adr/classification/0005-tiered-detection.md)).
The mediator converts a body when it serves it, and backfill converts it before scanning. A verdict
recorded at backfill is trusted when the mediator later serves the body, so the two conversions
must produce the same Markdown from the same HTML. The conversion calls outside code, so it cannot
sit in `core/`, and a copy of its configuration in each deployable could drift apart unseen. So one
library holds it, in `sanitize/markdown`, and both deployables import it. What is released around
the Markdown, the untrusted-content delimiters and the serve-time pattern check
([ADR-0002](../docs/adr/redaction/0002-fetch-time-re-evaluation.md)), is the mediator's alone and
sits in its own pure core.

Only `sanitize/markdown` may import the converter library. Its property test reads the output
with [goldmark](https://github.com/yuin/goldmark), an independent CommonMark parser, to find what a
Markdown reader would see as a link, an image or raw HTML. Only this library's tests may import it.
