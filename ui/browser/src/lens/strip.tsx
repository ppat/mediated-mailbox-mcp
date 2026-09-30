// The summary strip, level 0 of the zoom ladder (docs/UI.md section 4). It shows the lens's named
// figures from the registry's summary query and the time the read began, so a stale tab is visibly
// stale. It stays above every level, and while backfill pass 1 runs each figure is a count so far.
import type { Figure, LensFigures } from "../app/api.ts";
import { count, local, utc } from "../app/format.ts";

export function Strip(props: { summary: LensFigures; soFar: boolean }) {
  const { summary } = props;
  const figures = summary.figures.map((f) => ({ ...f, text: figureText(f) }));
  return (
    <section class="strip" aria-label="Summary">
      {figures.map((f) => (
        <a key={f.key} class="figure" href={f.link}>
          <span class="figure-value">{f.text}</span>
          <span class="label">
            {f.wording}
            {f.unit === null ? null : ` (${f.unit})`}
            {props.soFar ? " so far" : null}
          </span>
        </a>
      ))}
      <span class="as-of muted" title={local(summary.as_of)}>
        as of {utc(summary.as_of)}
      </span>
    </section>
  );
}

// figureText is a figure as the strip shows it, a count, or a time in UTC, or "none" for a time with
// nothing to show (docs/UI.md section 17.1).
export function figureText(f: Figure): string {
  if (f.value !== null) {
    return count(f.value);
  }
  return f.at === null ? "none" : utc(f.at);
}
