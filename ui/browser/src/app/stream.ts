// The stream client's handler, the half that runs under the test runner (ADR-0063, ADR-0064). The
// stream carries one event per changed object with the object's whole current state (docs/UI.md
// section 17.5), and the handler replaces that object's signal with it, so a node bound to the object
// redraws and no component re-runs. A missed event costs nothing, since the next one is whole.
//
// transport.ts is the other half, the one place the stream is opened, and it is exercised in a real
// browser because the test runner has no EventSource.
//
// Streams shares one connection per account among every surface in the tab that follows it (docs/UI.md
// section 9).
import { batch, effect, signal, untracked, type Signal } from "@preact/signals";
import type { PlanEvent, RateEvent, RunEvent } from "./api.ts";
import type { LiveStatus } from "./transport.ts";

// eventNames are the stream's event names, the three objects ADR-0058 streams.
export const eventNames = ["run", "rate", "plan"] as const;
export type EventName = (typeof eventNames)[number];

// LiveObjects holds one account's streamed objects, each in its own signal, and when the last event
// arrived.
export class LiveObjects {
  readonly updatedAt: Signal<number | undefined> = signal(undefined);
  readonly rate: Signal<RateEvent | undefined> = signal(undefined);
  // lastRun is the run the latest run event carried, for a reader that follows every run rather than
  // one it already knows.
  readonly lastRun: Signal<RunEvent | undefined> = signal(undefined);
  readonly #runs = new Map<string, Signal<RunEvent | undefined>>();
  readonly #plans = new Map<string, Signal<PlanEvent | undefined>>();

  constructor(readonly account: string) {}

  // run is one run's signal, holding nothing until the stream has sent the run.
  run(id: string): Signal<RunEvent | undefined> {
    return entry(this.#runs, id);
  }

  // plan is one applying plan's signal, holding nothing until the stream has sent the plan.
  plan(id: string): Signal<PlanEvent | undefined> {
    return entry(this.#plans, id);
  }

  // clear empties every object, keeping each signal a surface may have bound, so a connection opened
  // later starts from nothing, as a new one does.
  clear(): void {
    batch(() => {
      for (const s of [...this.#runs.values(), this.updatedAt, this.rate, this.lastRun]) {
        s.value = undefined;
      }
      for (const s of this.#plans.values()) {
        s.value = undefined;
      }
    });
  }

  // handle applies one event and reports whether it did. An event for another account, or one this
  // client does not know, is dropped.
  handle(name: string, data: string, now: number): boolean {
    // JSON.parse answers any. The contract declares each event's data by its name.
    const parsed = JSON.parse(data);
    if (typeof parsed !== "object" || parsed === null || parsed.account !== this.account) {
      return false;
    }
    switch (name) {
      case "run": {
        const run: RunEvent = parsed;
        this.run(run.run_id).value = run;
        this.lastRun.value = run;
        break;
      }
      case "rate": {
        const rate: RateEvent = parsed;
        this.rate.value = rate;
        break;
      }
      case "plan": {
        const plan: PlanEvent = parsed;
        this.plan(plan.plan_id).value = plan;
        break;
      }
      default:
        return false;
    }
    this.updatedAt.value = now;
    return true;
  }
}

function entry<T>(map: Map<string, Signal<T | undefined>>, id: string): Signal<T | undefined> {
  let s = map.get(id);
  if (s === undefined) {
    s = signal<T | undefined>(undefined);
    map.set(id, s);
  }
  return s;
}

// Connect opens an account's event stream into objects, calling refetch on a reconnect and on every
// poll of the fallback, and returns what the transport says of the stream and how to close it.
export type Connect = (
  account: string,
  objects: LiveObjects,
  refetch: () => void,
) => { status: Signal<LiveStatus>; stop: () => void };

// Follower is one surface following an account's stream. onRun is called with each run event, and
// refetch on a reconnect and on every poll of the fallback, each untracked, so neither subscribes the
// surface to what it reads.
export type Follower = { onRun?: (run: RunEvent) => void; refetch: () => void };

// Following is one surface's hold on its account's stream, until it stops.
export type Following = { objects: LiveObjects; status: Signal<LiveStatus>; stop: () => void };

type Shared = {
  objects: LiveObjects;
  followers: Set<Follower>;
  open?: { status: Signal<LiveStatus>; stop: () => void; stopRuns: () => void };
};

// Streams holds one connection per account for the whole tab, opened when the first surface follows
// the account and closed when the last stops, so surfaces on one screen, and several screens' surfaces,
// share one stream (docs/UI.md section 9). Each account keeps one LiveObjects for the tab's life, which
// a surface binds when it renders, before it follows, and which is cleared when its connection closes,
// so a connection opened later starts from nothing and every bound signal still receives its events.
export class Streams {
  readonly #connect: Connect;
  readonly #accounts = new Map<string, Shared>();

  constructor(connect: Connect) {
    this.#connect = connect;
  }

  // objects is the account's streamed objects, which a surface binds while it renders.
  objects(account: string): LiveObjects {
    return this.#shared(account).objects;
  }

  // follow adds a surface to the account's stream, opening the connection when no surface follows it.
  follow(account: string, follower: Follower): Following {
    const shared = this.#shared(account);
    const own: Follower = { ...follower };
    shared.followers.add(own);
    const open = shared.open ?? this.#open(account, shared);
    let stopped = false;
    return {
      objects: shared.objects,
      status: open.status,
      stop: () => {
        if (stopped) {
          return;
        }
        stopped = true;
        shared.followers.delete(own);
        if (shared.followers.size === 0 && shared.open === open) {
          shared.open = undefined;
          open.stopRuns();
          open.stop();
          shared.objects.clear();
        }
      },
    };
  }

  #shared(account: string): Shared {
    let shared = this.#accounts.get(account);
    if (shared === undefined) {
      shared = { objects: new LiveObjects(account), followers: new Set() };
      this.#accounts.set(account, shared);
    }
    return shared;
  }

  #open(account: string, shared: Shared): NonNullable<Shared["open"]> {
    // Each callback reads the followers when it is called, so a surface that has stopped, even during
    // the same fan-out, is not called, and a connection already closed calls none, since the followers
    // it would reach are a later connection's.
    const open: NonNullable<Shared["open"]> = {
      status: signal<LiveStatus>("connecting"),
      stop: () => undefined,
      stopRuns: () => undefined,
    };
    const current = () => shared.open === open;
    const subscription = this.#connect(account, shared.objects, () =>
      untracked(() => {
        for (const f of current() ? shared.followers : []) {
          f.refetch();
        }
      }),
    );
    open.status = subscription.status;
    open.stop = subscription.stop;
    shared.open = open;
    open.stopRuns = effect(() => {
      const run = shared.objects.lastRun.value;
      if (run !== undefined) {
        untracked(() => {
          for (const f of shared.followers) {
            f.onRun?.(run);
          }
        });
      }
    });
    return open;
  }
}
