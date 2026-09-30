// The live indicator every live surface shows in the top bar (docs/UI.md section 9). Its text is a
// computed signal passed into JSX, so a new event or a tick of the clock redraws the text alone and the
// component never re-runs (ADR-0063).
import {
  computed,
  useComputed,
  effect,
  untracked,
  useSignal,
  type ReadonlySignal,
  type Signal,
} from "@preact/signals";
import { useEffect, useMemo } from "preact/hooks";
import type { RunEvent } from "./api.ts";
import { useDeps } from "./deps.ts";
import { duration } from "./format.ts";
import { LiveObjects } from "./stream.ts";
import { filled, ProgressTrack, type ProgressState } from "../lens/progress.tsx";
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

// Stream is one live surface's subscription to its account's stream, what the live indicator shows,
// and a clock that ticks every second for the ages and durations the surface shows.
export type Stream = {
  objects: LiveObjects;
  status: Signal<LiveStatus>;
  clock: Signal<number>;
};

// useStream subscribes a live surface to its account's stream while it is mounted (ADR-0058). onRun is
// called with each run event and refetch on a reconnect and on every poll of the fallback. Neither
// callback subscribes the surface to what it reads, since each runs untracked. A new callback opens a
// new subscription, so each is made once per account and object, and reads the view it acts on when it
// is called. enabled holds the
// subscription back, for an account the accounts endpoint has not listed.
export function useStream(
  account: string,
  onRun: (run: RunEvent) => void,
  refetch: () => void,
  enabled = true,
): Stream {
  const { connect, now, timers } = useDeps();
  const objects = useMemo(() => new LiveObjects(account), [account]);
  const status = useSignal<LiveStatus>("connecting");
  const clock = useSignal(now());
  useEffect(() => {
    if (!enabled) {
      return undefined;
    }
    const subscription = connect(account, objects, () => untracked(refetch));
    const stopStatus = effect(() => {
      status.value = subscription.status.value;
    });
    const stopRun = effect(() => {
      const run = objects.lastRun.value;
      if (run !== undefined) {
        untracked(() => onRun(run));
      }
    });
    let tick: number | undefined;
    const advance = () => {
      clock.value = now();
      tick = timers.set(advance, 1_000);
    };
    tick = timers.set(advance, 1_000);
    return () => {
      timers.clear(tick);
      stopRun();
      stopStatus();
      subscription.stop();
    };
  }, [enabled, account, objects, connect, now, timers, status, clock, onRun, refetch]);
  return { objects, status, clock };
}

// Live is a stream's live indicator, for the top bar of the surface that holds the stream.
export function Live(props: { stream: Stream }) {
  const { stream } = props;
  return (
    <LiveIndicator
      status={stream.status}
      updatedAt={stream.objects.updatedAt}
      clock={stream.clock}
    />
  );
}

// LiveText is text drawn from an object the stream replaces, the latest event's state or, before any
// event, the state the page read. Its text is a computed signal passed into JSX, so an event or a tick
// of the clock redraws the text alone and the component never re-runs (ADR-0063). The computed is
// rebuilt whenever the component is given another object, another fallback or another way to word
// them, so an answer read again, or a new run in the text's place, reaches the text.
export function LiveText<T>(props: {
  value: ReadonlySignal<T | undefined>;
  fallback: T;
  text: (value: T, now: number) => string;
  clock?: Signal<number>;
}) {
  const { value, fallback, text, clock } = props;
  const shown = useMemo(
    () => computed(() => text(value.value ?? fallback, clock?.value ?? 0)),
    [value, fallback, text, clock],
  );
  return <>{shown}</>;
}

// Measure is how far a live object's work has come, part of whole, and the state its fill is drawn in.
export type Measure = { part: number; whole: number; state: ProgressState };

// LiveProgress is the progress bar of docs/UI.md section 7.3 drawn from an object the stream replaces,
// as LiveText draws its text. Its fill and state are computed signals bound to their attributes, so an
// event moves the fill and the component never re-runs. They are rebuilt whenever the component is
// given another object, fallback or measure, so an answer read again reaches the bar.
export function LiveProgress<T>(props: {
  value: ReadonlySignal<T | undefined>;
  fallback: T;
  measure: (value: T) => Measure;
  label: string;
}) {
  const { value, fallback, measure } = props;
  const shown = useMemo(
    () => computed(() => measure(value.value ?? fallback)),
    [value, fallback, measure],
  );
  const fill = useMemo(() => computed(() => filled(shown.value.part, shown.value.whole)), [shown]);
  const state = useMemo(() => computed(() => shown.value.state), [shown]);
  return <ProgressTrack fill={fill} state={state} label={props.label} />;
}
