// The routes of docs/UI.md section 5 that exist, and the one effect at the route boundary that keeps a
// dataset view's URL canonical (ADR-0063). Fixed screens are matched before anything else, and each
// screen adds its route when it lands. A route receives only its path's parameters as props, and its
// dependencies through the context of deps.ts.
import { useCallback, useEffect, useMemo } from "preact/hooks";
import type { VNode } from "preact";
import { LocationProvider, Route, Router, useLocation } from "preact-iso";
import { JobsScreen, onJobsRun } from "../screens/jobs.tsx";
import { FailurePanel, onRunEvent, RunLive, RunScreen, runReads } from "../screens/run.tsx";
import { summaryView } from "../lens/lens.tsx";
import { accountsPath, jobsPath, lensPath, systemPath, type RunEvent } from "./api.ts";
import { DepsContext, useDeps, type Deps } from "./deps.ts";
import { SystemScreen } from "../screens/system.tsx";
import { Live, useStream } from "./live.tsx";
import { Frame } from "./frame.tsx";
import { Region } from "./region.tsx";
import { entryAccount, readLastAccount } from "./routes.ts";
import { apiQuery, canonical, canonicalize, parse, type DatasetName, type View } from "./url.ts";

export function App(props: { deps: Deps }) {
  return (
    <DepsContext.Provider value={props.deps}>
      <LocationProvider>
        <Routes />
      </LocationProvider>
    </DepsContext.Provider>
  );
}

// Routes is the route table. The screens' bodies land with their own work, so an account's route shows
// the frame around an empty body until the home screen does.
export function Routes() {
  return (
    <Router>
      <Route path="/" component={Entry} />
      <Route path="/:account" component={AccountHomeScreen} />
      <Route path="/:account/system" component={SystemScreenRoute} />
      <Route path="/:account/jobs" component={JobsScreenRoute} />
      <Route path="/:account/jobs/:run" component={RunScreenRoute} />
      <Route path="/:account/jobs/:run/failures/:seq" component={RunScreenRoute} />
      <Route default component={NotFound} />
    </Router>
  );
}

// screen mounts a route's component keyed by its route identity, the account and the object its path
// names, so another account or another object is another screen, which starts afresh with everything it
// holds (docs/UI.md section 6). The row a panel opens over a list is not part of the identity. Every
// route but the entry and not-found routes goes through it, which a test over Routes requires by
// asking isScreen of each route's component.
const screens = new WeakSet<object>();

function screen<P extends { account: string }>(
  component: (props: P) => VNode | null,
  identity: (props: P) => string,
): (props: P) => VNode {
  const Component = component;
  const keyed = (props: P) => <Component key={identity(props)} {...props} />;
  screens.add(keyed);
  return keyed;
}

// isScreen reports whether a route's component mounts its screen by its route identity.
export function isScreen(component: unknown): boolean {
  return typeof component === "function" && screens.has(component);
}

const AccountHomeScreen = screen(AccountHome, (p) => p.account);
const SystemScreenRoute = screen(System, (p) => p.account);
const JobsScreenRoute = screen(JobsRoute, (p) => p.account);
const RunScreenRoute = screen(RunRoute, (p) => `${p.account}/${p.run}`);

// Entry sends the browser to the account last used here, else the first account by identifier.
function Entry() {
  const { accounts, storage } = useDeps();
  const { route } = useLocation();
  const state = accounts.read(accountsPath());
  const answer = state.value;
  const target =
    answer.status === "ok"
      ? entryAccount(answer.answer.accounts, readLastAccount(storage))
      : undefined;
  useEffect(() => {
    if (target !== undefined) {
      route(`/${encodeURIComponent(target)}`, true);
    }
  }, [target, route]);
  return (
    <main class="body">
      <Region
        name="accounts"
        state={state}
        shape="block"
        retry={() => void accounts.retry(accountsPath())}
      >
        {(value) => (value.accounts.length === 0 ? <p>No account is set up yet.</p> : null)}
      </Region>
    </main>
  );
}

function AccountHome(props: { account: string }) {
  return <Frame account={props.account}>{null}</Frame>;
}

function System(props: { account: string }) {
  return (
    <Frame account={props.account}>
      <SystemScreen account={props.account} />
    </Frame>
  );
}

// listedIn reports whether the accounts endpoint lists the account, before which no live surface
// subscribes to its stream.
function useListed(account: string): boolean {
  const { accounts } = useDeps();
  const state = accounts.read(accountsPath()).value;
  return state.status === "ok" && state.answer.accounts.some((a) => a.account_id === account);
}

// currentView is the dataset view the location's query string holds, which a stream callback reads
// when an event arrives, so the callback is made once for the screen rather than once per view.
function currentView(dataset: DatasetName): View {
  return canonicalize(parse(dataset, location.search));
}

// JobsRoute is the jobs screen with its stream and live indicator (docs/UI.md sections 8.3 and 9).
function JobsRoute(props: { account: string }) {
  const { account } = props;
  const deps = useDeps();
  const listed = useListed(account);
  const onRun = useMemo(() => {
    const seen = new Map<string, string>();
    return (run: RunEvent) => onJobsRun(deps, account, currentView("runs"), seen)(run);
  }, [deps, account]);
  const refetch = useCallback(
    () => refetchJobs(deps, account, currentView("runs")),
    [deps, account],
  );
  const live = useStream(account, onRun, refetch, listed);
  return (
    <Frame account={account} live={<Live stream={live} />}>
      <JobsScreen account={account} live={live} />
    </Frame>
  );
}

// refetchJobs reads everything the jobs screen shows again, on a reconnect and on every poll.
function refetchJobs(deps: Deps, account: string, view: View): void {
  void deps.jobs.refresh(jobsPath(account));
  void deps.system.refresh(systemPath(account));
  void deps.lens.refresh(lensPath(account, apiQuery(summaryView(view))));
  void deps.lens.refresh(lensPath(account, apiQuery(view)));
}

// RunRoute is the run screen with its stream, and one failure's panel over it when the path names one
// (docs/UI.md sections 8.4 and 9). The live indicator shows while the run or its resumer runs.
function RunRoute(props: { account: string; run: string; seq?: string }) {
  const { account, run, seq } = props;
  const deps = useDeps();
  const view = useCanonicalView("failures");
  const listed = useListed(account);
  const onRun = useMemo(() => {
    const seen = new Map<string, string>();
    return (event: RunEvent) =>
      onRunEvent(deps, account, run, currentView("failures"), seen)(event);
  }, [deps, account, run]);
  const refetch = useCallback(() => {
    for (const p of runReads(account, run, currentView("failures"))) {
      void (p.includes("/lens?") ? deps.lens.refresh(p) : deps.runs.refresh(p));
    }
    void deps.system.refresh(systemPath(account));
  }, [deps, account, run]);
  const live = useStream(account, onRun, refetch, listed);
  const panel =
    seq === undefined ? undefined : (
      <FailurePanel account={account} run={run} seq={seq} view={view} />
    );
  return (
    <Frame
      account={account}
      live={<RunLive account={account} run={run} live={live} />}
      panel={panel}
    >
      <RunScreen
        account={account}
        run={run}
        view={view}
        live={live}
        panelOpen={seq !== undefined}
      />
    </Frame>
  );
}

function NotFound(props: { path?: string }) {
  const segments = (props.path ?? "").split("/").filter((s) => s !== "");
  const account = segments[0];
  const message = <p>No screen is at this address.</p>;
  return account === undefined ? (
    <main class="body">{message}</main>
  ) : (
    <Frame account={decodeURIComponent(account)}>{message}</Frame>
  );
}

// useCanonicalView reads a dataset view from the location's query string and, when the URL is not
// canonical, replaces it with the canonical form. It follows the location's full URL, because every
// navigation below a screen change is a query-string change.
export function useCanonicalView(dataset: DatasetName): View {
  const { path, url, route } = useLocation();
  const search = url.includes("?") ? url.slice(url.indexOf("?") + 1) : "";
  const want = canonical(dataset, search);
  useEffect(() => {
    if (want !== search) {
      route(`${path}?${want}`, true);
    }
  }, [path, search, want, route]);
  return canonicalize(parse(dataset, search));
}
