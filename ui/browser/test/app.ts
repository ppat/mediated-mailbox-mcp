// The app's dependencies for a test, bound to recorded answers, the shim's storage, timers the test
// advances itself and the configuration's defaults. The event stream's transport runs only in a browser
// (ADR-0063, ADR-0064), so connect hands the test each connection's objects and refetch, through which
// the test delivers what the transport would, a handled event or a poll.
import { signal, type Signal } from "@preact/signals";
import { configDefaults } from "../src/app/config.ts";
import { makeDeps, regionTiming, type Deps, type Timers } from "../src/app/deps.ts";
import type { Channel, GuideMessage, Guides } from "../src/app/guide.ts";
import type { LiveObjects } from "../src/app/stream.ts";
import type { LiveStatus } from "../src/app/transport.ts";
import { attentionPath, jobsPath } from "../src/app/api.ts";
import { oldestCandidates, pendingCandidates } from "../src/screens/home.tsx";
import { ok, type Answer, type Recorded } from "./fixtures/fetch.ts";

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

// Hub is the guide's channel under a test, delivering what one document posts to every other channel
// of the hub, as a BroadcastChannel of one origin does.
export class Hub {
  readonly #listeners = new Set<{ from: number; on: (m: GuideMessage) => void }>();
  readonly sent: GuideMessage[] = [];
  #next = 0;

  // channel is one document's end of the hub.
  channel(): Channel {
    this.#next += 1;
    const me = this.#next;
    return {
      post: (message) => {
        this.sent.push(message);
        for (const l of this.#listeners) {
          if (l.from !== me) {
            l.on(message);
          }
        }
      },
      listen: (on) => {
        const l = { from: me, on };
        this.#listeners.add(l);
        return () => this.#listeners.delete(l);
      },
    };
  }
}

// TestGuides records what the app asked of the browser's windows and clipboard. pip, when given, opens
// a document the test reads, standing in for the small window that stays on top.
export class TestGuides {
  readonly opened: { url: string; name: string }[] = [];
  readonly copied: string[] = [];
  readonly consoles: string[] = [];
  readonly windows: Document[] = [];
  clipboard = "";
  blockWindows = false;

  constructor(
    readonly hub: Hub = new Hub(),
    readonly withPip = false,
  ) {}

  guides(): Guides {
    return {
      channel: this.hub.channel(),
      pip: this.withPip
        ? async () => {
            const doc = document.implementation.createHTMLDocument("guide");
            this.windows.push(doc);
            const fake: unknown = {
              document: doc,
              addEventListener: () => undefined,
              removeEventListener: () => undefined,
            };
            // The small window is a Window to the app, and the test reads only its document.
            return fake as Window; // oxlint-disable-line typescript/no-unsafe-type-assertion -- a test's stand-in for the browser's window
          }
        : undefined,
      open: (url, name) => {
        this.opened.push({ url, name });
        return this.blockWindows ? null : window;
      },
      copy: async (text) => {
        this.copied.push(text);
      },
      paste: async () => this.clipboard,
      console: (url) => {
        this.consoles.push(url);
      },
    };
  }
}

export function testDeps(
  server: Recorded,
  connections: Connection[] = [],
  now = () => Date.parse("2026-09-10T10:16:04Z"),
  timers: Timers = new ManualTimers(),
  guides: Guides = new TestGuides().guides(),
): Deps {
  localStorage.clear();
  return makeDeps(
    server.fetch,
    server.post,
    guides,
    now,
    localStorage,
    regionTiming,
    timers,
    // The entry document's consent redirect, as the server renders it from its configuration.
    { ...configDefaults, consentRedirect: "http://127.0.0.1:47823/" },
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

// homeAnswers are the recordings of the reads Home makes for an account beside the system endpoint's,
// which a test landing on Home serves with the chrome's (docs/UI.md section 8.1).
export function homeAnswers(account: "personal" | "other"): Record<string, Answer> {
  const suffix = account === "personal" ? "" : "-other";
  return {
    [jobsPath(account)]: ok(`jobs${suffix}.json`),
    [attentionPath(account)]: ok(`attention${suffix}.json`),
    [pendingCandidates(account)]: ok(`candidates-rows-pending${suffix}.json`),
    [oldestCandidates(account)]: ok(`candidates-rows-pending-oldest${suffix}.json`),
  };
}
