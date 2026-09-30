// The data cache (ADR-0063, docs/UI.md section 19). Data state is per request, keyed by the path read,
// and each path's state is one signal, so a component bound to it redraws only what reads it. A read
// within the freshness window reuses the answer, and a read while a request for the same path is in
// flight joins it rather than sending a second one.
import { signal, type Signal } from "@preact/signals";
import type { Failure, Result } from "./api.ts";

// Freshness is how long an answer is reused, a few seconds as section 19 states.
export const freshness = 5_000;

// State is one path's data state. A loading state carries the number of the request it waits on, so a
// region waiting past its timeout starts waiting afresh when a retry sends a new request. A read that
// failed keeps the failure until it is retried. A read refetching an answer keeps showing the answer
// it has, so the region never blanks while it refreshes.
export type State<T> =
  | { status: "loading"; attempt: number }
  | { status: "ok"; answer: T; at: number }
  | { status: "error"; failure: Failure };

type Entry<T> = {
  state: Signal<State<T>>;
  inflight: Promise<void> | undefined;
  // controller aborts the request in flight, and identifies it, so the answer of a request a retry
  // replaced is dropped.
  controller: AbortController | undefined;
  attempt: number;
  at: number | undefined;
};

// Cache holds the data state of one kind of read, so each kind keeps its own answer type.
export class Cache<T> {
  readonly #entries = new Map<string, Entry<T>>();

  constructor(
    private readonly load: (path: string, signal: AbortSignal) => Promise<Result<T>>,
    private readonly now: () => number,
  ) {}

  // read returns the path's state, fetching it when it has no answer or its answer is older than the
  // freshness window.
  read(path: string): Signal<State<T>> {
    const entry = this.#entry(path);
    const stale = entry.at === undefined || this.now() - entry.at >= freshness;
    if (stale && entry.inflight === undefined && entry.state.peek().status !== "error") {
      void this.#fetch(path, entry);
    }
    return entry.state;
  }

  // refresh fetches the path again whatever its age, for a reconnected stream, a poll or a tab that
  // became visible. It joins a request already in flight.
  refresh(path: string): Promise<void> {
    const entry = this.#entry(path);
    return entry.inflight ?? this.#fetch(path, entry);
  }

  // retry is the operator's retry. It abandons a request still in flight, which a region may have
  // waited on past its timeout, and sends a new one (docs/UI.md section 12).
  retry(path: string): Promise<void> {
    const entry = this.#entry(path);
    entry.controller?.abort();
    return this.#fetch(path, entry, true);
  }

  #entry(path: string): Entry<T> {
    let entry = this.#entries.get(path);
    if (entry === undefined) {
      entry = {
        state: signal<State<T>>({ status: "loading", attempt: 0 }),
        inflight: undefined,
        controller: undefined,
        attempt: 0,
        at: undefined,
      };
      this.#entries.set(path, entry);
    }
    return entry;
  }

  // #fetch sends a request for the path. A failed read, and a retry of a read still loading, show
  // loading again under a new attempt. A first read already shows loading, and writes no signal, since
  // it runs while a component renders.
  #fetch(path: string, entry: Entry<T>, restart = false): Promise<void> {
    entry.attempt += 1;
    const status = entry.state.peek().status;
    if (status === "error" || (restart && status === "loading")) {
      entry.state.value = { status: "loading", attempt: entry.attempt };
    }
    const controller = new AbortController();
    entry.controller = controller;
    const inflight = this.load(path, controller.signal).then((result) => {
      if (entry.controller !== controller) {
        return;
      }
      entry.inflight = undefined;
      entry.controller = undefined;
      if (result.ok) {
        entry.at = this.now();
        entry.state.value = { status: "ok", answer: result.value, at: entry.at };
      } else {
        entry.at = undefined;
        entry.state.value = { status: "error", failure: result.failure };
      }
    });
    entry.inflight = inflight;
    return inflight;
  }
}
