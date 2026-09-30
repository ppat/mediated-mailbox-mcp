// The region patterns of docs/UI.md section 12. A region reads one path's data state and shows a
// skeleton of its shape while loading, the words "still loading" after a second, and its error card
// on a failed read or after ten seconds without an answer. The request stays open past that card, so
// a late answer still replaces it, and the retry abandons it and sends a new one (cache.ts). The other regions of the screen are left alone, and nothing
// ever covers the whole page.
import type { Signal } from "@preact/signals";
import type { ComponentChild } from "preact";
import { useEffect, useState } from "preact/hooks";
import type { Failure } from "./api.ts";
import type { State } from "./cache.ts";
import { useDeps } from "./deps.ts";

// Shape is what a skeleton draws, bars for a chart and lines for a table or a block of values.
export type Shape = "chart" | "table" | "block";

type RegionProps<T> = {
  name: string;
  state: Signal<State<T>>;
  shape: Shape;
  retry: () => void;
  children: (value: T) => ComponentChild;
};

// Region re-renders when its data state changes between loading, answered and failed, which changes
// its structure. What its children bind below that is theirs.
export function Region<T>(props: RegionProps<T>) {
  const state = props.state.value;
  if (state.status === "ok") {
    return <>{props.children(state.answer)}</>;
  }
  if (state.status === "error") {
    return <ErrorCard name={props.name} failure={state.failure} retry={props.retry} />;
  }
  // Loading is keyed by the read as well as the attempt, since each path's attempts count from zero, so
  // a region that moves to another read while one is loading starts its timers again (docs/UI.md
  // section 12).
  return (
    <Loading
      key={`${readId(props.state)}:${state.attempt}`}
      name={props.name}
      shape={props.shape}
      retry={props.retry}
    />
  );
}

// reads number each path's state signal the first time a region shows it, so a region can tell one
// read from another. A signal no region shows any more is forgotten with it.
const reads = new WeakMap<object, number>();
let nextRead = 0;

function readId(state: object): number {
  let id = reads.get(state);
  if (id === undefined) {
    nextRead += 1;
    id = nextRead;
    reads.set(state, id);
  }
  return id;
}

type Phase = "loading" | "slow" | "timeout";

// Loading waits on one attempt at the read. A retry starts a new attempt, which mounts a new Loading
// and so restarts its timers.
function Loading(props: { name: string; shape: Shape; retry: () => void }) {
  const { timing, timers } = useDeps();
  const [phase, setPhase] = useState<Phase>("loading");
  useEffect(() => {
    const slow = timers.set(() => setPhase("slow"), timing.slow);
    const timeout = timers.set(() => setPhase("timeout"), timing.timeout);
    return () => {
      timers.clear(slow);
      timers.clear(timeout);
    };
  }, [timing, timers]);
  if (phase === "timeout") {
    const failure: Failure = {
      origin: "ui",
      code: "timeout",
      message: `no answer within ${timing.timeout / 1_000} seconds, and the request is still open`,
      request_id: "",
      status: 0,
    };
    return <ErrorCard name={props.name} failure={failure} retry={props.retry} />;
  }
  const lines = props.shape === "block" ? 3 : 5;
  return (
    <div
      class="skeleton"
      aria-busy="true"
      aria-label={`${props.name} loading`}
      data-shape={props.shape}
    >
      {Array.from({ length: lines }, (_, i) => (
        <div key={i} class="skeleton-line" />
      ))}
      {phase === "slow" ? <p class="muted">still loading</p> : null}
    </div>
  );
}

const headings = {
  client: "This request was refused",
  ui: "The UI server failed",
  database: "The database did not answer",
} as const satisfies Record<Failure["origin"], string>;

// ErrorCard stands in a region's place when its read failed, naming the failure's origin by the error
// contract (docs/UI.md section 17.3), with the request id and a retry. A read unanswered past the
// region's timeout has no known origin, since the network, a proxy, the UI server or the database could
// each be the one that has not answered, so its heading names none.
export function ErrorCard(props: { name: string; failure: Failure; retry: () => void }) {
  const { failure } = props;
  return (
    <div class="error-card" role="alert">
      <h2>{failure.code === "timeout" ? "No answer came in time" : headings[failure.origin]}</h2>
      <p>{failure.message}</p>
      {failure.request_id === "" ? null : (
        <p class="muted">
          Request <span class="mono">{failure.request_id}</span>
        </p>
      )}
      <button type="button" onClick={props.retry}>
        Retry {props.name}
      </button>
    </div>
  );
}
