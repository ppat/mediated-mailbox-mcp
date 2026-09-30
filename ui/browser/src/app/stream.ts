// The stream client's handler, the half that runs under the test runner (ADR-0063, ADR-0064). The
// stream carries one event per changed object with the object's whole current state (docs/UI.md
// section 17.5), and the handler replaces that object's signal with it, so a node bound to the object
// redraws and no component re-runs. A missed event costs nothing, since the next one is whole.
//
// transport.ts is the other half, the one place the stream is opened, and it is exercised in a real
// browser because the test runner has no EventSource.
import { signal, type Signal } from "@preact/signals";
import type { PlanEvent, RateEvent, RunEvent } from "./api.ts";

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
