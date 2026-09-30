// The stream client's transport, the one place the event stream is opened (ADR-0063), and the only
// browser module that depends on ADR-0058. It reconnects with exponential backoff from 1 second up to
// the reconnection ceiling, 30 seconds by default, and refetches the view on reconnect. After three
// failed reconnects it also polls, refetching the view at the polling interval, 5 seconds by default,
// while the backoff keeps doubling up to its ceiling, and it stops polling when the stream opens
// (docs/UI.md section 9). Both values are the configuration of docs/UI.md section 18.1. While the tab is hidden it
// holds no connection and polls nothing, and when the tab is visible again it refetches and reconnects.
//
// The test runner has no EventSource, so this module is exercised in a real browser. The schedule it
// follows is a pure function the tests drive.
import { signal, type Signal } from "@preact/signals";
import { eventNames, type LiveObjects } from "./stream.ts";

// LiveStatus is what the live indicator says of the stream.
export type LiveStatus = "connecting" | "live" | "reconnecting" | "polling";

const reconnectsBeforePolling = 3;
const firstDelay = 1_000;

// Timing is the reconnection ceiling and the polling interval, in milliseconds.
export type Timing = { reconnectMax: number; pollInterval: number };

// schedule is what follows the stream's failure count, the number of times in a row it dropped or
// failed to open, under a reconnection ceiling. The first failure is the drop and every later one a
// failed reconnect, so polling starts once three reconnects have failed.
export function schedule(failures: number, ceiling: number): { delay: number; polling: boolean } {
  return {
    delay: Math.min(ceiling, firstDelay * 2 ** Math.max(0, failures - 1)),
    polling: failures > reconnectsBeforePolling,
  };
}

export type Subscription = { status: Signal<LiveStatus>; stop: () => void };

// subscribe opens the account's stream at url into objects. refetch reloads the view's data, and now
// stamps each applied event.
export function subscribe(
  url: string,
  objects: LiveObjects,
  refetch: () => void,
  now: () => number,
  timing: Timing,
): Subscription {
  const status = signal<LiveStatus>("connecting");
  let source: EventSource | undefined;
  let failures = 0;
  let reconnect: ReturnType<typeof setTimeout> | undefined;
  let poll: ReturnType<typeof setInterval> | undefined;

  const stopPolling = () => {
    clearInterval(poll);
    poll = undefined;
  };

  const close = () => {
    source?.close();
    source = undefined;
    clearTimeout(reconnect);
    reconnect = undefined;
  };

  const open = () => {
    const current = new EventSource(url);
    source = current;
    current.addEventListener("open", () => {
      if (failures > 0) {
        refetch();
      }
      failures = 0;
      stopPolling();
      status.value = objects.updatedAt.peek() === undefined ? "connecting" : "live";
    });
    for (const name of eventNames) {
      current.addEventListener(name, (event: MessageEvent<string>) => {
        if (objects.handle(name, event.data, now())) {
          status.value = "live";
        }
      });
    }
    current.addEventListener("error", () => {
      // The browser's own reconnection is replaced by the schedule, so the source is closed here.
      close();
      failures += 1;
      const next = schedule(failures, timing.reconnectMax);
      if (next.polling && poll === undefined) {
        poll = setInterval(refetch, timing.pollInterval);
      }
      status.value = next.polling ? "polling" : "reconnecting";
      reconnect = setTimeout(open, next.delay);
    });
  };

  const onVisibility = () => {
    if (document.visibilityState === "hidden") {
      close();
      stopPolling();
      return;
    }
    if (source === undefined) {
      failures = 0;
      refetch();
      open();
    }
  };

  document.addEventListener("visibilitychange", onVisibility);
  if (document.visibilityState !== "hidden") {
    open();
  }
  return {
    status,
    stop: () => {
      document.removeEventListener("visibilitychange", onVisibility);
      close();
      stopPolling();
    },
  };
}
