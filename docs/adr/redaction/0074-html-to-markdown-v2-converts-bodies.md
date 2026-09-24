# 0074. html-to-markdown v2 converts bodies to Markdown, configured through its own hooks

**Status:** Accepted ·
**Pillar:** [Metadata always flows; sensitive bodies never do](../../../DESIGN.md#metadata-always-flows-sensitive-bodies-never-do) ·
**Serves:** [A4](../../../USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything), [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

[ADR-0036](./0036-released-bodies-are-clean-markdown.md) settles the target format, clean Markdown,
and that an existing, well-exercised library produces it, never a homegrown converter. It leaves
the library to implementation. Plain-text converters are already out, because ADR-0036 displaced
plain text as the target, and so is anything written here. What is left is which Go
HTML-to-Markdown library to call. Three exist that have not been archived or superseded, one of
them with no tagged release since 2020.

**The choice looks like a formatting problem and is a safety one.** A converter is judged by how
readable its Markdown is, but here its output is released to an agent and read by the Content
Scanner ([ADR-0005](../classification/0005-tiered-detection.md)). What matters is what it lets
through, what hostile HTML does to it, and whether its structure is the one the scanner reads. How
pleasant the Markdown is was not graded.

### The requirements

| # | Requirement | What it demands | Source |
| --- | --- | --- | --- |
| R1 | Existing and exercised | A library in real use, not a converter this project writes | [ADR-0036](./0036-released-bodies-are-clean-markdown.md) |
| R2 | Links as `[label](target)` | The real target, so a label that disagrees with it is visible, and a target that cannot break out of the link syntax | [ADR-0036](./0036-released-bodies-are-clean-markdown.md) |
| R3 | Images dropped | Every image gone, remote ones and tracking pixels included | [ADR-0036](./0036-released-bodies-are-clean-markdown.md) |
| R4 | Markup gone | No script, style, form or other markup, and no raw HTML in the output | [ADR-0036](./0036-released-bodies-are-clean-markdown.md) |
| R5 | Structure the scanner reads | Headings as lines starting with `#` or `##`, bold as `**`, and a code in a table cell visible to the scanner's rules | [ADR-0005](../classification/0005-tiered-detection.md) |
| R6 | Bounded | Bounded time, memory and stack on hostile or huge HTML, at every serve and across a whole mailbox in backfill | This record's |
| R7 | Maintained at its latest version | Tagged releases to pin and follow | [CLAUDE.md](../../../CLAUDE.md#tools-and-versions) |
| R8 | Small footprint | Few modules linked into the binaries, and a clean vulnerability scan | This record's, under the vulnerability scan [CLAUDE.md](../../../CLAUDE.md#go) runs |
| R9 | Hooks | Configuration that closes any gap in R2 to R5 without a fork | This record's |

**How they were weighted.** R4 and R6 ordered the field, because a failure there releases active
markup or takes down the process serving bodies, and nothing downstream catches either. R2 came
next for the same reason on links. R5 was graded but can be met by configuration, so it separated
candidates only where configuration could not reach. R7 and R8 broke ties.

## Decision

- **[html-to-markdown v2](https://github.com/JohannesKaufmann/html-to-markdown) converts every
  body**, because it is the only candidate with no requirement it fails. Its link targets are
  percent-encoded, so a `)` in a URL cannot end the link and point it elsewhere. It links only a
  small DOM helper package beside `golang.org/x/net`, and it grew linearly on every input tried.
- **It is called from `sanitize/markdown` and nowhere else**
  ([sanitize/README.md](../../../sanitize/README.md)), so the mediator and backfill cannot configure
  it differently.
- **A body larger than 512 KiB is refused before conversion**, because a conversion's memory grows
  to about 150 times its input, 626 MB for 4.2 MB of text. The limit is about five times the
  [102 KB at which Gmail clips HTML mail](https://mailchimp.com/help/gmail-is-clipping-my-email/),
  which senders design to stay under, and no public measurement of real HTML mail sizes was found
  to set it against. 1 MiB and 2 MiB were weighed, at about 150 MB and 300 MB of memory for one
  body, against the risk of refusing a legitimate body heavy with inline images or quoted threads.
  The inline images a refused body most often carries would be dropped by the conversion anyway.
- **A body the parser refuses is refused too.** The parser
  [refuses more than 512 open elements](https://cs.opensource.google/go/x/net/+/refs/tags/v0.59.0:html/parse.go;l=238),
  about 128 nested layout tables. A refused body produces no Markdown at all, never a part of one,
  and ADR-0036 withholds it.

### What its ordinary path does that the records forbid, and what stops it

Its [README](https://github.com/JohannesKaufmann/html-to-markdown/blob/290df46a279e3d7d9011dbbb199658dfcb5ec272/README.md#security)
says "This library does NOT sanitize untrusted content", in advice about showing its Markdown as
HTML in a browser. Its defaults also leave these in the output, and its own hooks remove each.

| What the default does | The harm | What stops it |
| --- | --- | --- |
| Keeps `javascript:` and `data:` link targets | A released link that runs code or carries a document | A pass before rendering keeps a target only when it is an absolute `http` or `https` URL with a host, or a `mailto:` URL of bare addresses and nothing else, and any other link is written as its label |
| Keeps `data:` and `cid:` images, and every image unless removed | An embedded payload, or a remote fetch that tells the sender the mail was read | Every image element is removed |
| Keeps the text of form labels, buttons and select options, the fallback inside `object`, and the text of `svg` and `template` | Content the sender never displayed reaches the agent | Those elements, and every other element that executes, embeds, styles or collects input, are removed with their content |
| Writes the comment `<!--THE END-->` between two adjacent lists | Raw HTML in the output | The option that writes it is switched off |
| Writes a preformatted block inside a quote with only its first line quoted, so its later lines fall out of the quote and end the code fence early | Escaped markup on those lines, a `<script>` or a remote `<img>`, becomes live HTML | Every code element is written as ordinary escaped text before rendering, a preformatted block's line breaks kept as line breaks, so no fence exists to end early. Code loses its monospace layout |
| Its table plugin refuses layout tables and runs the refused cells together, as in `Your code482913` | The scanner cannot see a code that shares a word with the cell before it | Each table cell is written as a block, on its own line, where the scanner's rule for a code alone on a line finds it |

### How the decision meets each requirement

| Requirement | Met by |
| --- | --- |
| R1 Existing and exercised | The most starred of the three on GitHub, about 3,800 against 125 and 32, with releases through the current year and robustness defects fixed as reported |
| R2 Links | Percent-encoded targets, escaped labels, and the pass that empties a target that is not a web or mail address |
| R3 Images | The image elements removed by name, so the defaults upstream can change without reaching this |
| R4 Markup gone | The element removals, the list comment switched off, and code written as escaped text. Its default already turns escaped text such as `&lt;script&gt;` into text, not a tag |
| R5 Scanner structure | Headings with `#` and bold with `**` by default, and table cells as blocks |
| R6 Bounded | The parser refuses more than 512 open elements, which bounds recursion. `sanitize/markdown` bounds input size and recovers a panic into a refusal, which the library does not. The size limit also bounds time, because conversion time grew linearly on every input tried |
| R7 Maintained | Tagged releases, followed at the latest by the dependency updates |
| R8 Footprint | Two modules linked, and a clean vulnerability scan |
| R9 Hooks | Removal by element, a pass over the parsed document before rendering, and renderer options, all public |

### Implementation notes

- **The table plugin is not in the default converter.** It is added by hand, and it refuses any
  table marked `role="presentation"`, any cell holding a line break, and any table holding a
  heading, a list or another table. Most email layout tables are refused.
- **Keeping a line break inside a cell writes a literal `<br />`**, which is raw HTML, so that
  option stays off.
- **Link targets cannot be filtered where they are assembled.** The function that builds them is
  not public, so the filtering is a pass over the parsed document before rendering.
- **The parser's limit on open elements came with `golang.org/x/net` v0.45.0**, and before it
  nothing bounded the conversion's recursion.
- **Escaped bold is written as `\*\*x\**`**, which still contains the `**` the scanner's bold check
  looks for.

## Alternatives considered

Three candidates were graded on four levels. **4** means the candidate carries the requirement
natively. **3** means one small, bounded piece of configuration carries it. **2** means it is
carried only by convention or with a named gotcha. **1** means it cannot honestly be met without
writing converter-grade code. Each grade rests on a run against synthetic email HTML, marked M, or
on reading the source, marked S. The chosen candidate is the first row.

| | R1 | R2 | R3 | R4 | R5 | R6 | R7 | R8 | R9 |
| --- | --- | --- | --- | --- | --- | --- | --- | --- | --- |
| html-to-markdown v2 | 4 S | 3 M | 3 M | 3 M | 2 M | 3 M | 3 S | 4 M | 4 M |
| godown | 2 S | 1 M | 3 S | 2 M | 1 M | 1 M | 1 S | 3 M | 2 S |
| Firecrawl's fork of html-to-markdown v1 | 3 S | 1 M | 3 M | 1 M | 2 M | 2 M | 2 S | 3 M | 3 S |

The grades for html-to-markdown v2 are for its configured form, the hooks above in place. Its R5 is
2 even so, because table cells as blocks is a workaround for the plugin's refusals, not a table
conversion.

**What the grid shows.** R3 separated nobody, because every candidate removes an image by name. R6
looks as if the shared parser levels it, since all three parse with `golang.org/x/net/html` and
none can exhaust its stack, and it does not. What each converter does with a document the parser
accepts differs by orders of magnitude. The chosen candidate leads every row except R3 and R5,
where it ties. On R2 the gap is one of kind rather than degree, because it is the only one whose
link syntax a hostile target cannot break.

The second table carries what the grid cannot.

| Candidate | On hostile input | Releases | Links in |
| --- | --- | --- | --- |
| html-to-markdown v2 | No failure in a 90-second fuzzing run. Two panics reported upstream, both fixed | Tagged, the latest this year | A DOM helper package and `golang.org/x/net` |
| godown | Output doubles with each level of nested tables, so a 727-byte body came out at 20.9 MB and 40 levels exhausted the memory limit. A fuzzing run found a panic in under five seconds ([line 444](https://github.com/mattn/godown/blob/43ad2e5393f9e86687d7a14fd5a8d57903f711e0/godown.go#L444)) | The latest tag dates from 2020, and the fixes since are untagged, so following the latest release pins a version that prints comments | A text-width package and `golang.org/x/net` |
| Firecrawl's fork | About 1 GB allocated for 1.1 MB of repeated deep markup | None, so pinning is by commit only | goquery, cascadia, `golang.org/x/net` and a YAML parser, with helpers that fetch URLs in its root package |

**Reading the two tables.** The chosen candidate's worst cell is a 2, on R5, and it is the only
worst cell a rule of the scanner's already works around, since a code in a layout-table cell stands
on its own line. godown's worst cells are 1s on R2, R5, R6 and R7, and the
Firecrawl fork's are 1s on R2 and R4. Those are the requirements that ordered the field, and no
configuration reaches them. Of the chosen candidate's strengths, the percent-encoded link target
guards something nothing else in the design does, while its bounded depth comes from the parser
every candidate shares. Leaving it costs one package's configuration and its tests, because only
`sanitize/markdown` imports it. Its one real cost, a single maintainer, is confined to that
package too.

- **html-to-markdown v2.** The case for it is the one above, the only candidate that fails no
  requirement, with every gap in its defaults closed through its own public hooks. The case against
  is that one person maintains it, nobody fuzzes the whole conversion upstream, and its panic
  history means the caller must recover a panic rather than trust that none remains. Its table
  plugin turns few email tables into rows, so the scanner reads layout-table codes by a different
  rule than the one written for table cells.
- **[godown](https://github.com/mattn/godown).** The case for it is a small, readable converter
  that turns every table into rows, layout tables included, which the scanner's table rule reads
  directly. Rejected on its output doubling with each level of nested tables and a panic found in
  seconds, and because it writes a link target
  [as it finds it](https://github.com/mattn/godown/blob/43ad2e5393f9e86687d7a14fd5a8d57903f711e0/godown.go#L314-L322),
  so a target holding `)](` redirects the link.
- **[Firecrawl's fork](https://github.com/firecrawl/html-to-markdown).** The case for it is that a
  company tunes it for output bound for language models and runs it at scale, and it converts data
  tables well. Rejected because it writes escaped text such as `&lt;script&gt;` back out as a live
  tag, and it
  [decodes a `data:text/html` iframe](https://github.com/firecrawl/html-to-markdown/blob/1af9901a5d6101621120204f7ea3f5355fd5ea31/commonmark.go#L410-L425)
  and converts the hidden document into the body. Both happen after any sanitizer run beforehand
  could act.
- **A sanitizer such as [bluemonday](https://github.com/microcosm-cc/bluemonday) before the
  conversion.** The case for it is a second, independent layer. Not taken. The hooks already remove
  what it would, it reads no CSS either, so it would not remove hidden text, and its latest tag
  dates from 2024.

## Consequences

- **Leaving this library costs one package.** Only `sanitize/markdown` imports it, and the tests
  that prove the conversion state what any replacement has to meet.
- **What would re-argue it.** A new major version, because the gaps its hooks close were found in
  this one's defaults. A panic or a case that grows faster than its input, found by longer fuzzing
  or by a real mailbox's backfill, which would weaken R6. A year without a release while robustness
  issues go unanswered, which would weaken R7. And a move off a `golang.org/x/net` that limits open
  elements, because that limit is what bounds the conversion's depth.
- **What it adds to other costs.** Each body in flight can take about 80 MB while it converts, so
  how many bodies the mediator converts at once, and backfill's batch size, bound their memory.
- **Assumptions about other components.** The import lists let only `sanitize/markdown` import the
  library ([ADR-0071](../engineering/0071-static-enforcement-toolchain.md)). The caller withholds a
  refused body, as [ADR-0036](./0036-released-bodies-are-clean-markdown.md) requires.
- **Its controls.** The overrides in the table of defaults, and the refusal of a body the
  conversion cannot bound, each have an injection row in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
