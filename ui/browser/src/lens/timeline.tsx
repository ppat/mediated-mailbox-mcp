// The run timeline of docs/UI.md sections 7.3 and 8.4, inline SVG 120 pixels tall. x is wall time
// from the run's start to its finish, or to now while it runs. A run is paged when any of its events
// records a page, and then y is the page and the progress events trace a line. The other marks sit on
// the line at their page, or on one line for a run without pages. Each mark kind has its own shape, so
// no kind is told by color alone, and each mark's hover names its kind, its time and the event's
// detail as recorded, as text.
import type { TimelineEvent } from "../app/api.ts";
import { utc } from "../app/format.ts";

// The marks, in the legend's order, each with its shape.
export const markKinds = ["start", "backoff", "retry", "failure", "resume", "finish"] as const;
export type MarkKind = (typeof markKinds)[number];

const width = 960;
const height = 120;
const pad = 12;

// Point is one event placed on the chart.
export type Point = { kind: string; x: number; y: number; title: string };

// place puts a run's events on the chart. from and to are the run's start and its finish or now, in
// milliseconds. A span of nothing draws every event at the left edge.
export function place(
  events: readonly TimelineEvent[],
  from: number,
  to: number,
): { points: Point[]; paged: boolean } {
  const pages = events.map((e) => e.page).filter((p): p is number => p !== null);
  const paged = pages.length > 0;
  const low = Math.min(...pages, 0);
  const high = Math.max(...pages, 1);
  let last = low;
  const points = events.map((e) => {
    const at = Date.parse(e.at);
    const x =
      to > from
        ? pad + ((Math.min(Math.max(at, from), to) - from) / (to - from)) * (width - 2 * pad)
        : pad;
    if (e.page !== null) {
      last = e.page;
    }
    const y = paged
      ? height - pad - ((last - low) / (high - low || 1)) * (height - 2 * pad)
      : height / 2;
    const detail = e.detail === null ? "" : `: ${e.detail}`;
    return {
      kind: e.kind,
      x,
      y,
      title: `${e.kind} at ${utc(e.at)}${e.page === null ? "" : `, page ${e.page}`}${detail}`,
    };
  });
  return { points, paged };
}

type TimelineProps = {
  events: readonly TimelineEvent[];
  startedAt: string;
  // endedAt is the run's finish, or now while it runs.
  endedAt: string;
};

export function Timeline(props: TimelineProps) {
  // A run with no events shows start and finish only (docs/UI.md section 8.4).
  const events: readonly TimelineEvent[] =
    props.events.length > 0
      ? props.events
      : [
          { kind: "start", at: props.startedAt, page: null, detail: null },
          { kind: "finish", at: props.endedAt, page: null, detail: null },
        ];
  const { points } = place(events, Date.parse(props.startedAt), Date.parse(props.endedAt));
  const line = points
    .filter((p) => p.kind === "progress")
    .map((p) => `${p.x},${p.y}`)
    .join(" ");
  return (
    <figure class="timeline" aria-label="Run timeline">
      <svg
        width={width}
        height={height}
        viewBox={`0 0 ${width} ${height}`}
        role="img"
        aria-label="Run timeline"
      >
        <line class="timeline-axis" x1={pad} y1={height - pad} x2={width - pad} y2={height - pad} />
        {line === "" ? null : <polyline class="timeline-progress" points={line} />}
        {points
          .filter((p) => p.kind !== "progress")
          .map((p, i) => (
            <Mark key={i} point={p} />
          ))}
      </svg>
      <figcaption class="legend">
        {markKinds.map((k) => (
          <span key={k} class="legend-item">
            <svg width={14} height={14} viewBox="-7 -7 14 14" aria-hidden="true">
              <Shape kind={k} x={0} y={0} />
            </svg>
            {k}
          </span>
        ))}
      </figcaption>
    </figure>
  );
}

function Mark(props: { point: Point }) {
  const { point } = props;
  return (
    <g class="timeline-mark" data-kind={point.kind}>
      <title>{point.title}</title>
      <Shape kind={point.kind} x={point.x} y={point.y} />
    </g>
  );
}

// Shape is a mark kind's shape centred on x and y. A kind the legend does not name is a small dot.
function Shape(props: { kind: string; x: number; y: number }) {
  const { x, y } = props;
  switch (props.kind) {
    case "start":
      return <circle class="mark-start" cx={x} cy={y} r={5} />;
    case "finish":
      return <rect class="mark-finish" x={x - 5} y={y - 5} width={10} height={10} />;
    case "failure":
      return (
        <path
          class="mark-failure"
          d={`M${x - 5},${y - 5}L${x + 5},${y + 5}M${x + 5},${y - 5}L${x - 5},${y + 5}`}
        />
      );
    case "backoff":
      return (
        <path
          class="mark-backoff"
          d={`M${x},${y - 6}L${x + 6},${y}L${x},${y + 6}L${x - 6},${y}Z`}
        />
      );
    case "retry":
      return <path class="mark-retry" d={`M${x - 5},${y + 5}L${x},${y - 5}L${x + 5},${y + 5}Z`} />;
    case "resume":
      return <path class="mark-resume" d={`M${x},${y - 7}L${x},${y + 7}`} />;
    default:
      return <circle class="mark-other" cx={x} cy={y} r={2} />;
  }
}
