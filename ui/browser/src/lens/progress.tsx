// The progress bar of docs/UI.md section 7.3, drawn as inline SVG like every chart. Its track is the
// raised token and its fill is info while the work runs, ok when it is complete and restricted when it
// failed. The count of the total is text the caller writes beside it. The fill's width is an SVG
// attribute, so no inline style is needed under the content security policy's style-src 'self'.

import type { ReadonlySignal } from "@preact/signals";

// ProgressState is what the fill's color says, the run state of the work the bar measures.
export type ProgressState = "running" | "complete" | "failed";

// width is the bar's drawn width in pixels, and height its thickness.
const width = 160;
const height = 8;

// filled is the fill's width for part of whole, never below zero or past the track. A whole of
// nothing draws no fill.
export function filled(part: number, whole: number): number {
  if (whole <= 0) {
    return 0;
  }
  return (Math.min(Math.max(part, 0), whole) / whole) * width;
}

export function ProgressBar(props: {
  part: number;
  whole: number;
  state: ProgressState;
  label: string;
}) {
  return (
    <ProgressTrack fill={filled(props.part, props.whole)} state={props.state} label={props.label} />
  );
}

// ProgressTrack draws the bar from its fill's width and its state, each a value or a signal. A signal
// is bound to its attribute, so a new value redraws the fill and the component never re-runs.
export function ProgressTrack(props: {
  fill: number | ReadonlySignal<number>;
  state: ProgressState | ReadonlySignal<ProgressState>;
  label: string;
}) {
  return (
    <svg
      class="progress"
      role="img"
      aria-label={props.label}
      width={width}
      height={height}
      viewBox={`0 0 ${width} ${height}`}
    >
      <rect class="progress-track" width={width} height={height} rx={4} />
      <rect
        class="progress-fill"
        data-state={props.state}
        width={props.fill}
        height={height}
        rx={4}
      />
    </svg>
  );
}
