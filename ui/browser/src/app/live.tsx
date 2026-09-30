// The live indicator every live surface shows in the top bar (docs/UI.md section 9). Its text is a
// computed signal passed into JSX, so a new event or a tick of the clock redraws the text alone and the
// component never re-runs (ADR-0063).
import { useComputed, type Signal } from "@preact/signals";
import { duration } from "./format.ts";
import type { LiveStatus } from "./transport.ts";

type LiveProps = {
  status: Signal<LiveStatus>;
  updatedAt: Signal<number | undefined>;
  // clock is the current time, advanced every second while the indicator shows.
  clock: Signal<number>;
};

const words = {
  connecting: "live · connecting",
  reconnecting: "live · reconnecting",
  polling: "live · polling",
  live: "live",
} as const satisfies Record<LiveStatus, string>;

// liveText is the indicator's wording for a status and the age of the last update. A live stream that
// has sent nothing yet is still connecting.
export function liveText(status: LiveStatus, updatedAt: number | undefined, now: number): string {
  if (status !== "live") {
    return words[status];
  }
  return updatedAt === undefined
    ? words.connecting
    : `${words.live} · updated ${duration(now - updatedAt)} ago`;
}

export function LiveIndicator(props: LiveProps) {
  const text = useComputed(() =>
    liveText(props.status.value, props.updatedAt.value, props.clock.value),
  );
  return (
    <span class="live" data-state={props.status}>
      {text}
    </span>
  );
}
