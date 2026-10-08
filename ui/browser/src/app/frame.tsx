// The global chrome of docs/UI.md section 6, the one frame every screen shares. Top to bottom it is the
// top bar, the address line, the refused-credential banner and the partial-index banner of section 12,
// and the screen's body. A detail panel, when the route opens one, sits beside the frame, and
// everything behind it is inert until it closes (section 4). An installation screen has the same frame
// with no account in view (InstallationFrame).
import type { ComponentChildren } from "preact";
import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import { accountsPath, systemPath, type Account, type Run, type System } from "./api.ts";
import type { State } from "./cache.ts";
import { useDeps } from "./deps.ts";
import { count, share, utc } from "./format.ts";
import { bindings, ignoresKeys } from "./keys.ts";
import { Menu } from "./menu.tsx";
import { keepLastAccount, switchAccount } from "./routes.ts";
import { applyTheme, keepTheme, storedTheme, themes, type Theme } from "./theme.ts";

type FrameProps = {
  account: string;
  children: ComponentChildren;
  // live is the live indicator, given by a live surface.
  live?: ComponentChildren;
  // panel is the detail panel a row-detail route opens over its list.
  panel?: ComponentChildren;
};

// The primary navigation, which lists only the screens that exist. Each screen adds its entry when it
// lands, in the order section 6 gives, with the key that follows g to go to it (section 13).
const screens = [
  { name: "Home", path: "", key: "h" },
  { name: "Jobs", path: "jobs", key: "j" },
  { name: "Policy", path: "policy", key: "o" },
  { name: "System", path: "system", key: "s" },
] as const;

// Account settings is a screen of the account outside the primary navigation, reached from the account
// selector's menu, its own g key and the last authentication rows (sections 6, 8.13 and 13).
const settings = { name: "Account settings", path: "account", key: "a" } as const;

// existingScreen is the name of the screen an app path under an account opens, when that screen exists,
// and undefined for one that does not exist yet, which the UI does not link to (docs/UI.md section 8.8).
export function existingScreen(href: string): string | undefined {
  const segment =
    href
      .split("?")[0]
      ?.split("/")
      .filter((s) => s !== "")[1] ?? "";
  return [...screens, settings].find((s) => s.path === segment)?.name;
}

// settingsPath is an account's settings screen.
export function settingsPath(account: string): string {
  return screenPath(account, settings.path);
}

export function Frame(props: FrameProps) {
  const { storage } = useDeps();
  const [showMap, setShowMap] = useState(false);
  const openMap = useCallback(() => setShowMap(true), []);
  useGlobalKeys(props.account, openMap);
  useEffect(() => keepLastAccount(storage, props.account), [storage, props.account]);
  return (
    <>
      <div class="frame" inert={props.panel !== undefined}>
        <header class="topbar">
          <AccountSelector account={props.account} />
          <Navigation account={props.account} />
          <div class="topbar-end">
            <SettingsMenu onShowMap={openMap} />
            <ListedAccount account={props.account}>{props.live}</ListedAccount>
          </div>
        </header>
        <AddressLine />
        <ListedAccount account={props.account}>
          <RefusedCredentialBanner account={props.account} />
          <PartialIndexBanner account={props.account} />
        </ListedAccount>
        <main class="body">
          <KnownAccount account={props.account}>{props.children}</KnownAccount>
        </main>
      </div>
      <ListedAccount account={props.account}>{props.panel}</ListedAccount>
      {showMap ? <KeyboardMap close={() => setShowMap(false)} /> : null}
    </>
  );
}

// useGlobalKeys follows the bindings that work on every screen, ? for the map and g then a screen's key
// for that screen.
// On an installation screen, which has no account, the g keys do nothing (section 13).
function useGlobalKeys(account: string | undefined, showMap: () => void) {
  const { route } = useLocation();
  const { timers } = useDeps();
  useEffect(() => {
    let pendingGo = false;
    let expiry: number | undefined;
    const onKey = (event: KeyboardEvent) => {
      if (ignoresKeys(event)) {
        return;
      }
      if (event.key === "?") {
        showMap();
      } else if (event.key === "g") {
        pendingGo = true;
        timers.clear(expiry);
        expiry = timers.set(() => (pendingGo = false), 1_000);
        return;
      } else if (pendingGo && account !== undefined) {
        const screen = [...screens, settings].find((s) => s.key === event.key);
        if (screen !== undefined) {
          route(screenPath(account, screen.path));
        }
      }
      pendingGo = false;
    };
    document.addEventListener("keydown", onKey);
    return () => {
      timers.clear(expiry);
      document.removeEventListener("keydown", onKey);
    };
  }, [account, route, showMap, timers]);
}

// AccountSelector shows the account in view, or Installation on an installation screen, and its menu
// lists every account with its provider, then Account settings for the account in view, Connect an
// account and Installation (section 6). Switching from an installation screen goes to the account's Home.
function AccountSelector(props: { account: string | undefined }) {
  const { accounts } = useDeps();
  const { path, url } = useLocation();
  const state = accounts.read(accountsPath()).value;
  const listed: readonly Account[] = state.status === "ok" ? state.answer.accounts : [];
  const provider = listed.find((a) => a.account_id === props.account)?.provider;
  const search = url.includes("?") ? url.slice(url.indexOf("?")) : "";
  const { account } = props;
  return (
    <Menu
      label="Account"
      button={
        account === undefined ? (
          "Installation"
        ) : (
          <>
            <span class="mono">{account}</span>
            {provider === undefined ? null : <span class="muted"> · {provider}</span>}
          </>
        )
      }
    >
      {(close) => [
        ...listed.map((a) => (
          <li key={a.account_id} role="none">
            <a
              role="menuitem"
              href={
                account === undefined
                  ? `/${encodeURIComponent(a.account_id)}`
                  : switchAccount(path, search, a.account_id)
              }
              aria-current={a.account_id === account ? "true" : undefined}
              onClick={close}
            >
              <span class="mono">{a.account_id}</span>
              <span class="muted">{a.provider}</span>
            </a>
          </li>
        )),
        account === undefined ? null : (
          <li key="settings" role="none">
            <a role="menuitem" href={settingsPath(account)} onClick={close}>
              Account settings
            </a>
          </li>
        ),
        <li key="connect" role="none">
          <a role="menuitem" href="/setup/connect" onClick={close}>
            Connect an account
          </a>
        </li>,
        <li key="installation" role="none">
          <a role="menuitem" href="/setup" onClick={close}>
            Installation
          </a>
        </li>,
      ]}
    </Menu>
  );
}

// The installation's own navigation, Setup and Base policy (section 6). Base policy is current on every
// base policy screen, its rules, history and import included.
const installationScreens = [
  { name: "Setup", path: "/setup", here: (path: string) => path === "/setup" },
  {
    name: "Base policy",
    path: "/setup/policy",
    here: (path: string) => path === "/setup/policy" || path.startsWith("/setup/policy/"),
  },
] as const;

// InstallationFrame is the frame of an installation screen, which belongs to no account. The account
// selector reads Installation, the primary navigation is the installation's own, and search, the
// range control and the group-by control are absent (section 6). A panel a base policy route opens
// over its list sits beside the frame, as an account's does.
export function InstallationFrame(props: {
  children: ComponentChildren;
  panel?: ComponentChildren;
}) {
  const [showMap, setShowMap] = useState(false);
  const openMap = useCallback(() => setShowMap(true), []);
  const { path } = useLocation();
  useGlobalKeys(undefined, openMap);
  return (
    <>
      <div class="frame" inert={props.panel !== undefined}>
        <header class="topbar">
          <AccountSelector account={undefined} />
          <nav aria-label="Screens">
            <ul class="nav">
              {installationScreens.map((s) => (
                <li key={s.path}>
                  <a href={s.path} aria-current={s.here(path) ? "page" : undefined}>
                    {s.name}
                  </a>
                </li>
              ))}
            </ul>
          </nav>
          <div class="topbar-end">
            <SettingsMenu onShowMap={openMap} />
          </div>
        </header>
        <AddressLine />
        <main class="body">{props.children}</main>
      </div>
      {props.panel}
      {showMap ? <KeyboardMap close={() => setShowMap(false)} /> : null}
    </>
  );
}

function Navigation(props: { account: string }) {
  const { path } = useLocation();
  return (
    <nav aria-label="Screens">
      <ul class="nav">
        {screens.map((s) => {
          const target = screenPath(props.account, s.path);
          const here = path === target || (s.path !== "" && path.startsWith(`${target}/`));
          return (
            <li key={s.name}>
              <a href={target} aria-current={here ? "page" : undefined}>
                {s.name}
                {s.path === "jobs" ? (
                  <ListedAccount account={props.account}>
                    <RunningMark account={props.account} />
                  </ListedAccount>
                ) : null}
              </a>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}

// screenPath is a screen's path under an account.
export function screenPath(account: string, path: string): string {
  return [`/${encodeURIComponent(account)}`, path].filter((part) => part !== "").join("/");
}

// RunningMark is the Jobs item's mark while any workload runs, read from the system endpoint's
// decisions block (section 6). It is a labeled mark, so it is not told by color alone.
function RunningMark(props: { account: string }) {
  const { system } = useDeps();
  const state = system.read(systemPath(props.account)).value;
  if (state.status !== "ok" || state.answer.decisions.workloads_running === 0) {
    return null;
  }
  return <span class="running-mark"> running</span>;
}

function SettingsMenu(props: { onShowMap: () => void }) {
  const { storage, config } = useDeps();
  const [theme, setTheme] = useState<Theme>(() => storedTheme(storage, config.defaultTheme));
  const choose = (t: Theme) => {
    applyTheme(document.documentElement, t);
    keepTheme(storage, t);
    setTheme(t);
  };
  return (
    <Menu label="Settings" button="Settings" end>
      {(close) => [
        ...themes.map((t) => (
          <li key={t} role="none">
            <button
              type="button"
              role="menuitemradio"
              aria-checked={t === theme}
              onClick={() => {
                choose(t);
                close();
              }}
            >
              Theme {t}
            </button>
          </li>
        )),
        <li key="map" role="none">
          <button
            type="button"
            role="menuitem"
            onClick={() => {
              close();
              props.onShowMap();
            }}
          >
            Keyboard map
          </button>
        </li>,
      ]}
    </Menu>
  );
}

// AddressLine shows the current view's URL as selectable text, the handle a view is shared by.
function AddressLine() {
  const { url } = useLocation();
  return (
    <input
      class="address"
      type="text"
      readOnly
      aria-label="Address of this view"
      value={`${location.origin}${url}`}
      onFocus={(event) => event.currentTarget.select()}
    />
  );
}

// refusedCredential is the refused-credential banner's text while the account's latest authentication
// reads refused (ADR-0097), and undefined otherwise. A failed attempt raises none, since workloads retry
// it (section 12).
export function refusedCredential(
  system: System,
  provider: string | undefined,
): string | undefined {
  const op = system.operational;
  if (op.last_auth_outcome !== "refused") {
    return undefined;
  }
  const who = provider === undefined ? "The provider" : providerName(provider);
  const when = op.last_auth_at === null ? "" : ` at ${utc(op.last_auth_at)}`;
  return `${who} refused ${system.account}'s credential${when}. Workloads that call ${who} fail for this account until it is re-authorized.`;
}

// providerName is how a provider is named in a sentence.
export function providerName(provider: string): string {
  return provider === "gmail" ? "Gmail" : provider;
}

// RefusedCredentialBanner reads the system endpoint's operational block the partial-index banner
// reads, through the same cached read, so it follows the stream and the polls as that banner does, and
// sits above it (section 12). Its Re-authorize link goes straight to the account's re-authorization.
function RefusedCredentialBanner(props: { account: string }) {
  const { system, accounts } = useDeps();
  const state = system.read(systemPath(props.account)).value;
  const listed = accounts.read(accountsPath()).value;
  const provider =
    listed.status === "ok"
      ? listed.answer.accounts.find((a) => a.account_id === props.account)?.provider
      : undefined;
  const text = state.status === "ok" ? refusedCredential(state.answer, provider) : undefined;
  return text === undefined ? null : (
    <p class="banner banner-refused" role="alert">
      {text} <a href={`${settingsPath(props.account)}/reauthorize`}>Re-authorize {props.account}</a>
    </p>
  );
}

// partialIndex is the banner's text while a backfill pass runs, naming each running pass, or undefined
// when neither runs. A pass 1 running after an earlier run of it succeeded was re-opened by a change of
// scanner, to fetch again the subjects an earlier scanner masked, once the run's start has masked the
// others again from the index (ADR-0120). The index already holds the whole mailbox then, so the
// banner says so and no count is a count so far (docs/UI.md section 12).
export function partialIndex(system: System): string | undefined {
  const op = system.operational;
  const sentences: string[] = [];
  if (!op.backfill_pass1_complete && op.backfill_pass1_run?.state === "running") {
    const words = backfillProgress(op.backfill_pass1_run);
    const progress = words === undefined ? "" : `, ${words}`;
    const completed = op.backfill_pass1_succeeded_at;
    sentences.push(
      completed === null
        ? `Backfill pass 1 is running${progress}, so every count here is a count so far.`
        : `Backfill pass 1 is running again${progress}, to fetch again the subjects an earlier scanner masked and mask them under the scanner now in force. ` +
            `It last completed at ${utc(completed)}, so the index holds the whole mailbox and no count here is a count so far. ` +
            "The other subjects were masked again from the index when the run started, and a subject it has not fetched yet keeps its earlier masks.",
    );
  }
  if (!op.backfill_pass2_complete && op.backfill_pass2_run?.state === "running") {
    sentences.push(
      `Backfill pass 2 is running with ${count(op.pending_scan)} ${op.pending_scan === 1 ? "message" : "messages"} pending scan, and a pending message denies its body until it is scanned.`,
    );
  }
  return sentences.length === 0 ? undefined : sentences.join(" ");
}

// indexing reports whether backfill pass 1 runs with no earlier run of it succeeded, when every count
// on a lens is a count so far. A re-opened pass 1 runs over an index that already holds the whole
// mailbox, so it is not indexing. A caller without an answer from the system read counts the index as
// partial.
export function indexing(system: System): boolean {
  const op = system.operational;
  return (
    !op.backfill_pass1_complete &&
    op.backfill_pass1_run?.state === "running" &&
    op.backfill_pass1_succeeded_at === null
  );
}

// refetchOf is a backfill pass 1 run's fetch of stale subjects again, once its enumeration has ended,
// as the subjects it fetched again of those plus the ones still to fetch, or undefined for a run that
// fetched none and has none to fetch (docs/UI.md section 8.1, ADR-0120).
export function refetchOf(
  run: Pick<Run, "checkpoint" | "counters">,
): { fetched: number; of: number } | undefined {
  const fetched = numberIn(run.counters, "refetched") ?? 0;
  const stale = numberIn(run.checkpoint, "stale") ?? 0;
  return fetched + stale === 0 ? undefined : { fetched, of: fetched + stale };
}

// backfillProgress is a backfill run's progress in words, for a pass 1 run fetching stale subjects
// again the subjects it fetched again of all it fetches, else the page of pages its checkpoint records,
// with the share of each, or undefined while its checkpoint records neither (docs/UI.md sections 8.1
// and 12).
export function backfillProgress(run: Pick<Run, "checkpoint" | "counters">): string | undefined {
  const refetch = refetchOf(run);
  if (refetch !== undefined) {
    return `${count(refetch.fetched)} of ${count(refetch.of)} subjects fetched again (${share(refetch.fetched, refetch.of)})`;
  }
  const page = numberIn(run.checkpoint, "page");
  const of = numberIn(run.checkpoint, "of");
  return page === undefined || of === undefined
    ? undefined
    : `page ${count(page)} of ${count(of)} (${share(page, of)})`;
}

// numberIn reads a number from a free-form JSON object the contract leaves untyped, such as a run's
// checkpoint, and nothing else from it.
export function numberIn(value: unknown, key: string): number | undefined {
  if (typeof value !== "object" || value === null) {
    return undefined;
  }
  const v: unknown = Reflect.get(value, key);
  return typeof v === "number" ? v : undefined;
}

// unknownIndex is the banner's text when the system read failed. The banner fails toward a partial
// index, so a failed read never lets counts read as final.
export const unknownIndex =
  "The index's backfill state is unknown, because the system read failed, so every count here may be a count so far.";

// PartialIndexBanner reads the system endpoint's operational block. While it shows, it follows the
// account's event stream, through the tab's one connection for the account, and reads the block again on every backfill run event and on every poll of
// the stream's fallback, so its figures move with the pass. It is not a live surface and shows no live
// indicator (docs/UI.md section 9).
function PartialIndexBanner(props: { account: string }) {
  const { system, streams } = useDeps();
  const { account } = props;
  const path = systemPath(account);
  const text = bannerText(system.read(path).value);
  const showing = text !== undefined;
  useEffect(() => {
    if (!showing) {
      return undefined;
    }
    const refresh = () => void system.refresh(path);
    const following = streams.follow(account, {
      onRun: (run) => {
        if (run.workload === "backfill") {
          refresh();
        }
      },
      refetch: refresh,
    });
    return following.stop;
  }, [showing, account, path, system, streams]);
  return text === undefined ? null : (
    <p class="banner" role="status">
      {text} <a href={screenPath(account, "jobs")}>See Jobs</a>
    </p>
  );
}

// bannerText is the partial-index banner's text for the system read's state, the running passes on an
// answer, the unknown state on a failure, and nothing while the read loads or when no pass runs.
export function bannerText(state: State<System>): string | undefined {
  if (state.status === "ok") {
    return partialIndex(state.answer);
  }
  return state.status === "error" ? unknownIndex : undefined;
}

// ListedAccount shows its children only once the accounts endpoint lists the account, so nothing in
// the chrome reads or follows the system of an account the UI does not serve.
export function ListedAccount(props: { account: string; children: ComponentChildren }) {
  const { accounts } = useDeps();
  const state = accounts.read(accountsPath()).value;
  const listed =
    state.status === "ok" && state.answer.accounts.some((a) => a.account_id === props.account);
  return listed ? <>{props.children}</> : null;
}

// KnownAccount shows the screen only for an account the accounts endpoint lists. Every read would refuse
// another anyway, and this names the mistake once instead of once per region.
function KnownAccount(props: { account: string; children: ComponentChildren }) {
  const { accounts } = useDeps();
  const state = accounts.read(accountsPath()).value;
  if (state.status === "ok" && !state.answer.accounts.some((a) => a.account_id === props.account)) {
    return (
      <section class="error-card" role="alert">
        <h2>This request was refused</h2>
        <p>
          No account <span class="mono">{props.account}</span> is served here.
        </p>
      </section>
    );
  }
  return <>{props.children}</>;
}

function KeyboardMap(props: { close: () => void }) {
  const dialog = useRef<HTMLDivElement>(null);
  useEffect(() => {
    const opener = document.activeElement;
    dialog.current?.focus();
    return () => {
      if (opener instanceof HTMLElement) {
        opener.focus();
      }
    };
  }, []);
  return (
    <div
      class="dialog"
      role="dialog"
      aria-modal="true"
      aria-labelledby="keyboard-map-title"
      tabIndex={-1}
      ref={dialog}
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          event.stopPropagation();
          props.close();
        }
      }}
    >
      <h2 id="keyboard-map-title">Keyboard map</h2>
      <table>
        <tbody>
          {bindings.map((b) => (
            <tr key={b.keys}>
              <td class="mono">{b.keys}</td>
              <td>{b.action}</td>
            </tr>
          ))}
        </tbody>
      </table>
      <button type="button" onClick={props.close}>
        Close
      </button>
    </div>
  );
}
