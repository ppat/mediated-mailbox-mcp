// The app's dependencies for a test, bound to recorded answers, the shim's storage, timers the test
// advances itself and the configuration's defaults. The event stream's transport runs only in a browser
// (ADR-0063, ADR-0064), so connect hands the test each connection's objects and refetch, through which
// the test delivers what the transport would, a handled event or a poll.
import { signal, type Signal } from "@preact/signals";
import { configDefaults } from "../src/app/config.ts";
import { makeDeps, regionTiming, type Deps, type Timers } from "../src/app/deps.ts";
import type { LiveObjects } from "../src/app/stream.ts";
import type { LiveStatus } from "../src/app/transport.ts";
import type { Recorded } from "./fixtures/fetch.ts";

export type Connection = {
  account: string;
  objects: LiveObjects;
  refetch: () => void;
  open: boolean;
  // status is what the transport reports, which a test sets as the transport would.
  status: Signal<LiveStatus>;
};

// ManualTimers are the app's timers under a test, which fire only when the test advances them, so no
// test waits on the machine's clock and none races its load. The regions keep their real thresholds.
export class ManualTimers implements Timers {
  #now = 0;
  #next = 0;
  readonly #due = new Map<number, { at: number; run: () => void }>();

  set = (run: () => void, ms: number): number => {
    this.#next += 1;
    this.#due.set(this.#next, { at: this.#now + ms, run });
    return this.#next;
  };

  clear = (id: number | undefined): void => {
    if (id !== undefined) {
      this.#due.delete(id);
    }
  };

  // advance moves the timers' time on by ms, running each timer that falls due, in the order they
  // fall due, including any a run sets within the span.
  advance(ms: number): void {
    const end = this.#now + ms;
    for (;;) {
      const due = [...this.#due.entries()]
        .filter(([, t]) => t.at <= end)
        .toSorted(([a, x], [b, y]) => x.at - y.at || a - b)[0];
      if (due === undefined) {
        break;
      }
      const [id, timer] = due;
      this.#due.delete(id);
      this.#now = timer.at;
      timer.run();
    }
    this.#now = end;
  }
}

export function testDeps(
  server: Recorded,
  connections: Connection[] = [],
  now = () => Date.parse("2026-09-10T10:16:04Z"),
  timers: Timers = new ManualTimers(),
): Deps {
  localStorage.clear();
  return makeDeps(
    server.fetch,
    now,
    localStorage,
    regionTiming,
    timers,
    configDefaults,
    (account, objects, refetch) => {
      const connection: Connection = {
        account,
        objects,
        refetch,
        open: true,
        status: signal<LiveStatus>("connecting"),
      };
      connections.push(connection);
      return {
        status: connection.status,
        stop: () => {
          connection.open = false;
        },
      };
    },
  );
}
