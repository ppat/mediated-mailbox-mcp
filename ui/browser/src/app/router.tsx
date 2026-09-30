// The routes of docs/UI.md section 5 that exist, and the one effect at the route boundary that keeps a
// dataset view's URL canonical (ADR-0063). Fixed screens are matched before anything else, and each
// screen adds its route when it lands. A route receives only its path's parameters as props, and its
// dependencies through the context of deps.ts.
import { useEffect } from "preact/hooks";
import { LocationProvider, Route, Router, useLocation } from "preact-iso";
import { accountsPath } from "./api.ts";
import { DepsContext, useDeps, type Deps } from "./deps.ts";
import { SystemScreen } from "../screens/system.tsx";
import { Frame } from "./frame.tsx";
import { Region } from "./region.tsx";
import { entryAccount, readLastAccount } from "./routes.ts";
import { canonical, canonicalize, parse, type DatasetName, type View } from "./url.ts";

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
      <Route path="/:account" component={AccountHome} />
      <Route path="/:account/system" component={System} />
      <Route default component={NotFound} />
    </Router>
  );
}

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
