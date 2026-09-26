# Design notes for a design session

Read [docs/UI.md](../../docs/UI.md) first. It is the design. This file is what a session working
on the mockups needs in addition, and nothing here is authority over the document set. Where this
file and the document set disagree, the document set wins and this file is wrong.

## How the shape was chosen

Three directions were sketched on one canvas on 2026-09-09 and compared by the operator on
2026-09-10. They differ only in what the UI is organized around. All three share the lens model
and the zoom ladder of docs/UI.md, drawn on the `Foundation` board. The canvas notes, in their own
words:

| | Path A · Console | Path B · Explorer | Path C · Review Desk |
| --- | --- | --- | --- |
| Organized around | the data sources, one page per ADR-0021 view | one query model, dataset × group-by × level × filters, the six views as saved lenses, verbs attached to row types | the operator's work, decisions first, then what is worth a look, then system health |
| Case for it | familiar, literal, nothing clever to get wrong, cheapest first version, the fallback if the M3 budget is tight | one mechanism for every view, present and future. Calendar, a second account, audit-by-actor all arrive as registry entries. Analysis is first-class everywhere | matches the actual loop (open, see what needs me, decide). The load-bearing screen gets the most design. Attention items make accepted risks visible without asking |
| Tradeoff | drill-down is built per page and drifts; growth is linear in pages; the plan page is a form with a table, not a review | generic surfaces go bland and strain on the one screen that matters (label ops have no home in a ladder). The read API becomes a small query engine that must stay bounded by a registry | "worth a look" rules are new design work nothing in the documents defines. Analysis views are second-class until pulled forward. Highest first-version cost |

The operator chose C ("Path C mockups look good and i think it serves the use case the best") with
B's engine underneath, on the designer's recommendation ("C's shell over B's engine"). The records
are ADR-0056 (the shape, with A and B as alternatives) and ADR-0057 (the engine).

## The canvas

Published at <https://claude.ai/code/artifact/b2faa4f1-a76c-400e-a74e-8edc04f984b9>. Two pages.

| Page | Boards | What they are |
| --- | --- | --- |
| `ui` | `Main` (home), `PlanReview`, `Jobs`, `JobRun`, `Palette` | the deliverable, dark palette |
| `Directions explored` | `Foundation`, `ConsoleHome`, `ConsolePlan`, `ExplorerHome`, `ExplorerPlan`, `DeskHome`, `DeskPlan` | the shared foundation and the three directions as light sketches. `DeskHome` and `DeskPlan` are what the deliverable was rebuilt from |

The canvas is a rendering of the files in this directory. When they disagree, the files are the
source. Pre-commit hooks normalize quotation marks and final newlines, so a freshly seeded canvas
can differ from the published one in those glyphs only.

## Mockup conventions

- Every board is a static Design Components artboard (`.dc.html`), one screen each, root element
  1440 px wide, height flowing. No logic script. Inline styles, flex and grid with gaps, inline
  stroke SVG icons only, no emoji, no gradients.
- The tokens are inlined as literal hex from docs/UI.md §14. The dark boards use only the dark
  tokens plus the six categorical series. `Palette` shows both palettes and is exempt.
- Fonts load from Google Fonts on the boards for convenience. The product self-hosts them. The
  handwritten face (Patrick Hand) is used for annotations only and is not product. Every board
  carries a small annotation label top-left naming the board.
- Every number is sample data from one consistent set, so boards can be compared. The set is in
  the session brief that produced them (account `personal`, 84,212 messages, one DRAFT plan of
  12,480 messages at 14.8% of corpus, four candidates, run r-0913 with 141 failures, and so on).
  Nothing on a board is a specification. docs/UI.md §8 is.
- Account selector always visible, `personal` selected, `work (not configured)` in the menu, no
  "all accounts" anywhere. Timestamps carry `Z`.
- The `canvas.json` layout leaves 80 px between boards in a row and 120 px between rows. Frame
  heights are set with slack. A frame paints its ground beyond the content and clipping is the
  only failure.

## Re-seeding and republishing

In Claude Code, run `/design`. Edit the `.dc.html` files and `canvas.json` here, then seed a
fresh copy of the canvas from all of them and republish to the URL above. The seeded output
(`ui.html`) is a build product, ignored by git, and never edited by hand. To change a board
that someone edited in the canvas itself, read the artifact back and extract its files into an
empty directory first, then re-seed from those.

## The palette derivation

`palette/palette.mjs` derives both palettes in OKLCH (lightness and chroma held per role set, hue
varying) and prints WCAG 2 contrast ratios against the surfaces. `palette/palette-report.txt` is
its output and the source of the tables in docs/UI.md §14. `palette/series.mjs` enumerates hue
orderings for a derived six-hue categorical series and runs each through the data-visualization
skill's validator (`scripts/validate_palette.js` in that skill, run as
`node validate_palette.js "<hex,…>" --mode dark --surface <hex>`). Every ordering at equal
lightness failed the colorblind-separation check in both modes and the normal-vision floor in
dark, which is why the categorical series in the product is the validator's reference palette,
re-validated on these surfaces (all checks pass in dark, while in light three slots sit under 3:1 and
need labels). To change a token, change the derivation, re-run it, re-run the validator for any
chart series, and update docs/UI.md §14 from the report. ADR-0059 records the decision.

## What must not change without a record

- The two decision verbs, OAuth client setup and account setup, and their database grant
  (ADR-0084). No third verb, no "retry", no "rollback", no policy editing, however small.
- Per account, never aggregated across accounts (the operator's ruling, ADR-0056).
- Never a body, never a snippet, never a preview (ADR-0016, ADR-0084).
- The lens model and the zoom ladder as the structure of every analytical view (ADR-0056).
- The dataset endpoint behind a registry as the engine (ADR-0057).
- The palette tokens and the categorical series (ADR-0059).
- Message-derived text rendered as text only (ADR-0056).

A change to any of these is a new decision record, minted through the `update-docs` skill, before
the mockup changes.

## Open design questions

- The "worth a look" attention rules and their thresholds. docs/UI.md §8.1 carries a starting
  set as the UI's own design. Nothing else in the documents defines them.
- A feedback verb on masking and gate events (true or false positive), which would also be the
  labeled-data source the learned detection tier waits for. A third verb, needing its own record.
- A mobile layout. None in the first version. The boards are desktop-only at 1440.
- The live-update transport (ADR-0058 is Proposed).

## Requirements this design answers

Traceability only. Each row points at where the requirement is met and restates nothing that lives
there. A requirement with no home elsewhere would be stated in full here and marked so. After the
third pass none remains.

**The operator's stated requirements.**

| Date | Requirement | Met in |
| --- | --- | --- |
| 2026-09-09 | drill-down and zoom in and out, aggregate views, as if analysing a dataset or a proposed change | docs/UI.md §3, §4; ADR-0056 |
| 2026-09-09 | structured to take on use cases beyond those described today | docs/UI.md §3, §5 (growth slots); ADR-0057 |
| 2026-09-09 | Go on the server, TypeScript in the browser, framework unchosen and to be influenced by this design | ADR-0042; docs/UI.md §16, §18 |
| 2026-09-10 | every view per account, explicit selection, nothing aggregated across accounts | docs/UI.md §1, §5, §6; ADR-0056; the per-account row in docs/VERIFICATIONS.md |
| 2026-09-10 | two palettes, dark primary, distinguishable and easy on the eyes by industry practice | docs/UI.md §14.1; ADR-0059 |
| 2026-09-10 | live, auto-refreshing status and progress of running batch work | docs/UI.md §8.1, §8.3, §9; ADR-0058 |
| 2026-09-10 | inspecting a failed batch run down to its individual failures | docs/UI.md §8.4 |
| 2026-09-10 | no authentication in the first version, an optional authenticating proxy in front | docs/UI.md §15, §18.1; ADR-0084 |
| 2026-09-10 | policy in the database, with import and export to a file | docs/UI.md §8.7; ADR-0004, ADR-0041, ADR-0016 |
| 2026-09-10 | the framework requirements documented explicitly | docs/UI.md §16 |
| 2026-09-10 | the implementer reads docs/UI.md and the records only; the design session reads ui/design/ in addition | docs/UI.md preamble; this file |

**Requirements the design work surfaced.**

| Requirement | Met in |
| --- | --- |
| every view state is a URL | docs/UI.md §4, §5 |
| message-derived text is hostile and renders inert | docs/UI.md §11, §15; ADR-0056; its verification row |
| friction scales with blast radius | docs/UI.md §8.2, §10 |
| empty, loading, partial, and error states are designed | docs/UI.md §12 |
| every zoom step is answered from server-side aggregates | docs/UI.md §11, §17.1; ADR-0057 |
| density and keyboard first | docs/UI.md §13, §14.3 |
| the sample is stratified | docs/UI.md §8.2 |
| timestamps UTC with local on hover | docs/UI.md §5, §11 |
| charts are navigation and tables the workhorse | docs/UI.md §4 |
| one message row everywhere | docs/UI.md §7 |
| the read API is a registry-bounded dataset endpoint | docs/UI.md §17; ADR-0057 |
| a failed run is a dataset of failures | docs/UI.md §8.4 |
| no retry verb and no rollback verb | docs/UI.md §5, §8.4; ADR-0084 |
| fonts self-hosted and a strict content security policy | docs/UI.md §14.3, §15; ADR-0062 |
| request tokens on the verbs | docs/UI.md §15, §17.4; ADR-0061 |
| the identity header trusted only when declared | docs/UI.md §15, §18.1; ADR-0084 |
| TLS with the client surface's posture | docs/UI.md §15; ADR-0084 |
| the UI's own configuration contract | docs/UI.md §18.1 |
| the UI's own observability | docs/UI.md §18.2 |
| the plan reviewer as the framework's proving screen | docs/UI.md §16 |
| the database additions the UI implies | ADR-0016, ADR-0020, ADR-0022 |
| data freshness visible on every screen | docs/UI.md §4 (the as-of time), §9 |
