// The bars of docs/UI.md section 7.3, levels 1 and 2 of the zoom ladder. One horizontal bar per group,
// sorted by count as the endpoint sends them, the top 20 then one other bar for the rest, which is not
// clickable. Each bar is a link that applies its group as a filter. The label is the group's value as
// text beside the bar, so it truncates like a cell and shows its full value on hover, and the count sits
// at the bar's end. A message-derived dataset draws its restricted share as an inner segment. Widths are
// SVG attributes, so no inline style is needed under the content security policy.
import { count } from "../app/format.ts";

// Bar is one group as the bars draw it. href is where a click on it leads, and is undefined for a
// group no filter can name, which is drawn but not clickable.
export type Bar = {
  key: string;
  label: string;
  known: boolean;
  count: number;
  restricted?: number | undefined;
  href: string | undefined;
};

// shownBars is how many groups get a bar before the rest are drawn as other.
export const shownBars = 20;

// widthOf is a bar's length as a percent of the longest bar's, which fills the region.
export function widthOf(n: number, longest: number): string {
  return `${longest <= 0 ? 0 : (n / longest) * 100}%`;
}

export function Bars(props: { name: string; bars: readonly Bar[]; note?: string }) {
  const top = props.bars.slice(0, shownBars);
  const rest = props.bars.slice(shownBars).reduce((sum, b) => sum + b.count, 0);
  const longest = Math.max(0, ...top.map((b) => b.count), rest);
  return (
    <figure class="bars" aria-label={props.name}>
      {top.map((b) => {
        const inner = (
          <>
            <span class="bar-label">
              {b.label}
              {b.known ? null : <span class="badge">unknown</span>}
            </span>
            <BarShape n={b.count} restricted={b.restricted} longest={longest} />
            <span class="bar-count numeric">{count(b.count)}</span>
          </>
        );
        return b.href === undefined ? (
          <div key={b.key} class="bar" title={b.label}>
            {inner}
          </div>
        ) : (
          <a key={b.key} class="bar" href={b.href} title={b.label}>
            {inner}
          </a>
        );
      })}
      {rest > 0 ? (
        <div class="bar bar-other">
          <span class="bar-label muted">other</span>
          <BarShape n={rest} longest={longest} />
          <span class="bar-count numeric">{count(rest)}</span>
        </div>
      ) : null}
      {props.note === undefined ? null : <figcaption class="muted">{props.note}</figcaption>}
    </figure>
  );
}

function BarShape(props: { n: number; restricted?: number; longest: number }) {
  return (
    <svg class="bar-shape" width="100%" height={20} aria-hidden="true">
      <line class="bar-baseline" x1={0} y1={0} x2={0} y2={20} />
      <rect class="bar-fill" x={0} y={0} height={20} width={widthOf(props.n, props.longest)} />
      {props.restricted === undefined || props.restricted <= 0 ? null : (
        <rect
          class="bar-restricted"
          x={0}
          y={6}
          height={8}
          width={widthOf(props.restricted, props.longest)}
        />
      )}
    </svg>
  );
}
