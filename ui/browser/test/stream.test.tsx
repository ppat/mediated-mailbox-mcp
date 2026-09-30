// The stream client's handler and schedule, and the live indicator bound to what the handler replaces.
// The transport opens an EventSource, which the test runner does not provide, so it is exercised in a
// real browser (ADR-0064). Each event's data here is built from a recorded answer, since a run's event
// is the jobs endpoint's run with its account and the rate's the system endpoint's rate block with its
// account, which the server's stream test proves.
import { signal } from "@preact/signals";
import { afterEach, expect, test } from "bun:test";
import { options, type VNode } from "preact";
import { act } from "preact/test-utils";
import type { PlansPage, System } from "../src/app/api.ts";
import { LiveIndicator, liveText } from "../src/app/live.tsx";
import { LiveObjects } from "../src/app/stream.ts";
import { schedule, type LiveStatus } from "../src/app/transport.ts";
import { mount, type Mounted } from "./render.ts";

let mounted: Mounted | undefined;
afterEach(() => mounted?.unmount());

// recording reads a recorded answer, which the contract types.
async function recording(file: string) {
  return JSON.parse(await Bun.file(new URL(`fixtures/${file}`, import.meta.url)).text());
}

async function system(): Promise<System> {
  return recording("system.json");
}

// rateEvent is the recorded rate block as the stream sends it, with its account.
async function rateEvent(account: string): Promise<string> {
  const rate = (await system()).operational.rate;
  if (rate === null) {
    throw new Error("the recording holds no rate block");
  }
  return JSON.stringify({ ...rate, account });
}

test("a run event replaces the run's whole state", async () => {
  const s = await system();
  const run = s.operational.backfill_pass2_run;
  if (run === null) {
    throw new Error("the recording holds no pass 2 run");
  }
  const objects = new LiveObjects("personal");
  const bound = objects.run(run.run_id);
  expect(bound.value).toBeUndefined();
  // A run's checkpoint is free-form JSON in the contract, so it is written as JSON.
  const checkpoint = JSON.parse('{"page": 3100, "of": 3368}');
  const moved = { ...run, account: "personal", checkpoint };
  expect(objects.handle("run", JSON.stringify(moved), 7)).toBe(true);
  expect(bound.value).toEqual(moved);
  expect(objects.run(run.run_id)).toBe(bound);
  expect(objects.updatedAt.value).toBe(7);
});

test("a rate event replaces the rate state", async () => {
  const objects = new LiveObjects("personal");
  const event = await rateEvent("personal");
  expect(objects.handle("rate", event, 1)).toBe(true);
  expect(objects.rate.value).toEqual(JSON.parse(event));
});

test("a plan event replaces the applying plan's progress", async () => {
  const page: PlansPage = await recording("plans-rows.json");
  const applying = page.rows.find((row) => row.status === "APPLYING");
  if (applying === undefined) {
    throw new Error("the recording holds no applying plan");
  }
  const objects = new LiveObjects("personal");
  const event = {
    account: "personal",
    plan_id: applying.plan_id,
    status: applying.status,
    applied: 1,
    of: applying.messages,
  };
  expect(objects.handle("plan", JSON.stringify(event), 1)).toBe(true);
  expect(objects.plan(applying.plan_id).value).toEqual(event);
});

test("an event for another account, or of an unknown name, is dropped", async () => {
  const objects = new LiveObjects("personal");
  expect(objects.handle("rate", await rateEvent("other"), 1)).toBe(false);
  expect(objects.handle("mood", JSON.stringify({ account: "personal" }), 1)).toBe(false);
  expect(objects.rate.value).toBeUndefined();
  expect(objects.updatedAt.value).toBeUndefined();
});

test("the reconnect schedule backs off from 1s to 30s and polls after three failed reconnects", () => {
  expect([1, 2, 3, 4, 5, 6, 7, 9].map((n) => schedule(n, 30_000).delay)).toEqual([
    1_000, 2_000, 4_000, 8_000, 16_000, 30_000, 30_000, 30_000,
  ]);
  expect([1, 2, 3, 4, 5].map((n) => schedule(n, 30_000).polling)).toEqual([
    false,
    false,
    false,
    true,
    true,
  ]);
});

test("the backoff stops at the configured ceiling", () => {
  expect([1, 2, 3, 4, 5].map((n) => schedule(n, 5_000).delay)).toEqual([
    1_000, 2_000, 4_000, 5_000, 5_000,
  ]);
});

test("the indicator's wording", () => {
  expect(liveText("connecting", undefined, 0)).toBe("live · connecting");
  expect(liveText("live", undefined, 0)).toBe("live · connecting");
  expect(liveText("live", 1_000, 13_500)).toBe("live · updated 12s ago");
  expect(liveText("reconnecting", 1_000, 2_000)).toBe("live · reconnecting");
  expect(liveText("polling", 1_000, 2_000)).toBe("live · polling");
});

test("an event and the clock redraw the indicator's text and never re-run the component", async () => {
  let renders = 0;
  // The hook signals installs is kept and called, so the counter changes nothing it counts.
  const previous: ((vnode: VNode) => void) | undefined = Object.getOwnPropertyDescriptor(
    options,
    "diffed",
  )?.value;
  options.diffed = (vnode: VNode) => {
    if (vnode.type === LiveIndicator) {
      renders += 1;
    }
    previous?.(vnode);
  };
  try {
    const event = await rateEvent("personal");
    const objects = new LiveObjects("personal");
    const status = signal<LiveStatus>("connecting");
    const clock = signal(10_000);
    mounted = mount(<LiveIndicator status={status} updatedAt={objects.updatedAt} clock={clock} />);
    const text = () => mounted?.root.textContent;
    expect(text()).toBe("live · connecting");

    await act(() => {
      objects.handle("rate", event, 10_000);
      status.value = "live";
    });
    expect(text()).toBe("live · updated 0s ago");
    await act(() => {
      clock.value = 15_000;
    });
    expect(text()).toBe("live · updated 5s ago");
    await act(() => {
      status.value = "reconnecting";
    });
    expect(text()).toBe("live · reconnecting");
    expect(mounted.root.querySelector(".live")?.getAttribute("data-state")).toBe("reconnecting");
    expect(renders).toBe(1);
  } finally {
    options.diffed = previous;
  }
});
