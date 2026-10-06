// The app's dependencies, built once by the composition root in main.ts and reached by every component
// through one context (ADR-0063). A route receives only its path's parameters as props, so no signal is
// ever reachable from a route's props, where the router's memoization would re-render the whole route on
// every write.
import { createContext } from "preact";
import { useContext } from "preact/hooks";
import {
  readAccount,
  readAccounts,
  readAttempt,
  readInstallation,
  readAttention,
  readBaseHistory,
  readBaseMatch,
  readRelease,
  readBasePolicy,
  readFailure,
  readJobs,
  readLens,
  readMatch,
  readRule,
  readRun,
  readSystem,
  type AccountSettings,
  type Accounts,
  type AttemptAnswer,
  type Attention,
  type BaseHistory,
  type BaseMatchAnswer,
  type Counts,
  type BasePolicy,
  type FailureDetail,
  type Fetch,
  type Installation,
  type Jobs,
  type LensAnswer,
  type MatchAnswer,
  type Post,
  type RuleDetail,
  type RunSummary,
  type System,
} from "./api.ts";
import { Cache } from "./cache.ts";
import { Outcomes } from "./outcomes.ts";
import type { BrowserConfig } from "./config.ts";
import type { Guides } from "./guide.ts";
import { Streams, type Connect } from "./stream.ts";

// Timing is when a region still loading says so, and when it shows its error card while the request
// stays open (docs/UI.md section 12).
export type Timing = { slow: number; timeout: number };

export const regionTiming: Timing = { slow: 1_000, timeout: 10_000 };

// Timers schedule the app's own delays, a region's loading phases, the live clock's tick and the
// keyboard map's g window, as the clock is read through now. A test hands in timers it advances
// itself, so it moves time deliberately and never races the machine's load (test/app.ts).
export type Timers = {
  set: (run: () => void, ms: number) => number;
  clear: (id: number | undefined) => void;
};

export const browserTimers: Timers = {
  set: (run, ms) => window.setTimeout(run, ms),
  clear: (id) => window.clearTimeout(id),
};

export type Deps = {
  fetch: Fetch;
  // post sends a state-changing request with the page's request token.
  post: Post;
  // guides opens the setup guide's windows and carries what the guide and the page tell each other.
  guides: Guides;
  now: () => number;
  // storage holds what is kept per browser, the theme override and the account last used. It is
  // undefined where the browser refuses storage, and nothing is kept then.
  storage: Storage | undefined;
  timing: Timing;
  timers: Timers;
  // config is what the entry document carries for the browser (docs/UI.md section 18.1).
  config: BrowserConfig;
  // streams shares one event stream per account among the surfaces that follow it. The composition
  // root binds its connection to the transport, which only a browser runs.
  streams: Streams;
  accounts: Cache<Accounts>;
  system: Cache<System>;
  attention: Cache<Attention>;
  lens: Cache<LensAnswer>;
  jobs: Cache<Jobs>;
  runs: Cache<RunSummary>;
  failures: Cache<FailureDetail>;
  installation: Cache<Installation>;
  attempts: Cache<AttemptAnswer>;
  settings: Cache<AccountSettings>;
  rules: Cache<RuleDetail>;
  matches: Cache<MatchAnswer>;
  baseMatches: Cache<BaseMatchAnswer>;
  releases: Cache<Counts>;
  basePolicy: Cache<BasePolicy>;
  baseHistory: Cache<BaseHistory>;
  // outcomes are the policy writes' outcomes each policy screen shows in its status region, with what
  // Put it back adds back after a lift (docs/UI.md section 8.7).
  outcomes: Outcomes;
};

export function makeDeps(
  fetch: Fetch,
  post: Post,
  guides: Guides,
  now: () => number,
  storage: Storage | undefined,
  timing: Timing,
  timers: Timers,
  config: BrowserConfig,
  connect: Connect,
): Deps {
  return {
    fetch,
    post,
    guides,
    now,
    storage,
    timing,
    timers,
    config,
    streams: new Streams(connect),
    accounts: new Cache((path, signal) => readAccounts(fetch, path, signal), now),
    system: new Cache((path, signal) => readSystem(fetch, path, signal), now),
    attention: new Cache((path, signal) => readAttention(fetch, path, signal), now),
    lens: new Cache((path, signal) => readLens(fetch, path, signal), now),
    jobs: new Cache((path, signal) => readJobs(fetch, path, signal), now),
    runs: new Cache((path, signal) => readRun(fetch, path, signal), now),
    failures: new Cache((path, signal) => readFailure(fetch, path, signal), now),
    installation: new Cache((path, signal) => readInstallation(fetch, path, signal), now),
    attempts: new Cache((path, signal) => readAttempt(fetch, path, signal), now),
    settings: new Cache((path, signal) => readAccount(fetch, path, signal), now),
    rules: new Cache((path, signal) => readRule(fetch, path, signal), now),
    matches: new Cache((path, signal) => readMatch(fetch, path, signal), now),
    baseMatches: new Cache((path, signal) => readBaseMatch(fetch, path, signal), now),
    releases: new Cache((path, signal) => readRelease(fetch, path, signal), now),
    basePolicy: new Cache((path, signal) => readBasePolicy(fetch, path, signal), now),
    baseHistory: new Cache((path, signal) => readBaseHistory(fetch, path, signal), now),
    outcomes: new Outcomes(),
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
