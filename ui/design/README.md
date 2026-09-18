# UI design sources

The mockups behind [docs/UI.md](../../docs/UI.md). This directory holds sources and instruments,
never reasoning. Why the UI is shaped this way lives in [docs/UI.md](../../docs/UI.md) and in the
decision records it cites (among them ADR-0056 through ADR-0065 and ADR-0072, via the
[decision-record index](../../docs/adr/README.md)).

## What is here

| File | What it is |
| --- | --- |
| `DESIGN-NOTES.md` | What a design session needs beyond docs/UI.md. Read it before changing a board |
| `*.dc.html` | Claude Design artboards, one screen each. Plain HTML, viewable in any browser. Every number on them is sample data |
| `canvas.json` | The canvas layout, two pages. `ui` holds the deliverable, `Directions explored` holds the alternatives the shape was chosen from |
| `palette/palette.mjs` | Derives both palettes in OKLCH and prints WCAG 2 contrast ratios. Run with `node palette/palette.mjs` |
| `palette/palette.json`, `palette/palette-report.txt` | The derived tokens and the contrast report the tables in docs/UI.md were written from |
| `palette/series.mjs` | The attempt at a derived categorical chart series, kept because its failure is why the reference series was adopted (ADR-0059) |

The deliverable boards are `Main` (home), `PlanReview`, `Jobs`, `JobRun`, and `Palette`. The
boards `Foundation`, `ConsoleHome`, `ConsolePlan`, `ExplorerHome`, `ExplorerPlan`, `DeskHome`, and
`DeskPlan` are the shared foundation and the three directions explored. `DeskHome` and `DeskPlan`
are the light sketches the deliverable was rebuilt from.

## The published canvas

<https://claude.ai/code/artifact/b2faa4f1-a76c-400e-a74e-8edc04f984b9>

The canvas is a rendering of these files. When the two disagree, these files are the source.

## Resuming the design

Read [DESIGN-NOTES.md](./DESIGN-NOTES.md). It carries how the shape was chosen, the mockup
conventions, how to re-seed and republish, the palette derivation procedure, what must not change
without a record, and the open design questions. To look, open the link above or any `.dc.html`
file in a browser.
