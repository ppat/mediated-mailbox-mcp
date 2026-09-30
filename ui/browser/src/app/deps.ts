// The app's dependencies, built once by the composition root in main.ts and reached by every component
// through one context (ADR-0063). A route receives only its path's parameters as props, so no signal is
// ever reachable from a route's props, where the router's memoization would re-render the whole route on
// every write.
import { createContext } from "preact";
import { useContext } from "preact/hooks";
import {
  readAccounts,
  readLens,
  readSystem,
  type Accounts,
  type Fetch,
  type LensAnswer,
  type System,
} from "./api.ts";
import { Cache } from "./cache.ts";
import type { BrowserConfig } from "./config.ts";
import type { LiveObjects } from "./stream.ts";
import type { Subscription } from "./transport.ts";

// Timing is when a region still loading says so, and when it shows its error card while the request
// stays open (docs/UI.md section 12).
export type Timing = { slow: number; timeout: number };

export const regionTiming: Timing = { slow: 1_000, timeout: 10_000 };

export type Deps = {
  fetch: Fetch;
  now: () => number;
  // storage holds what is kept per browser, the theme override and the account last used. It is
  // undefined where the browser refuses storage, and nothing is kept then.
  storage: Storage | undefined;
  timing: Timing;
  // config is what the entry document carries for the browser (docs/UI.md section 18.1).
  config: BrowserConfig;
  // connect opens an account's event stream into objects, calling refetch on a reconnect and on every
  // poll of the fallback. The composition root binds it to the transport, which only a browser runs.
  connect: (account: string, objects: LiveObjects, refetch: () => void) => Subscription;
  accounts: Cache<Accounts>;
  system: Cache<System>;
  lens: Cache<LensAnswer>;
};

export function makeDeps(
  fetch: Fetch,
  now: () => number,
  storage: Storage | undefined,
  timing: Timing,
  config: BrowserConfig,
  connect: Deps["connect"],
): Deps {
  return {
    fetch,
    now,
    storage,
    timing,
    config,
    connect,
    accounts: new Cache((path, signal) => readAccounts(fetch, path, signal), now),
    system: new Cache((path, signal) => readSystem(fetch, path, signal), now),
    lens: new Cache((path, signal) => readLens(fetch, path, signal), now),
  };
}

export const DepsContext = createContext<Deps | undefined>(undefined);

export function useDeps(): Deps {
  const deps = useContext(DepsContext);
  if (deps === undefined) {
    throw new Error("a component read the app's dependencies outside DepsContext");
  }
  return deps;
}
