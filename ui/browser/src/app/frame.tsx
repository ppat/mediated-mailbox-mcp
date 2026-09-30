// The global chrome of docs/UI.md section 6, the one frame every screen shares. Top to bottom it is the
// top bar, the address line, the partial-index banner of section 12 while a backfill pass runs, and the
// screen's body. A detail panel, when the route opens one, sits beside the frame, and everything behind
// it is inert until it closes (section 4).
import { effect } from "@preact/signals";
import type { ComponentChildren } from "preact";
import { useCallback, useEffect, useRef, useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import { accountsPath, systemPath, type Account, type System } from "./api.ts";
import type { State } from "./cache.ts";
import { useDeps } from "./deps.ts";
import { count, share } from "./format.ts";
import { bindings, ignoresKeys } from "./keys.ts";
import { Menu } from "./menu.tsx";
import { keepLastAccount, switchAccount } from "./routes.ts";
import { LiveObjects } from "./stream.ts";
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
  { name: "System", path: "system", key: "s" },
] as const;

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
            {props.live}
          </div>
        </header>
        <AddressLine />
        <ListedAccount account={props.account}>
          <PartialIndexBanner account={props.account} />
        </ListedAccount>
        <main class="body">
          <KnownAccount account={props.account}>{props.children}</KnownAccount>
        </main>
      </div>
      {props.panel}
      {showMap ? <KeyboardMap close={() => setShowMap(false)} /> : null}
    </>
  );
}

// useGlobalKeys follows the bindings that work on every screen, ? for the map and g then a screen's key
// for that screen.
function useGlobalKeys(account: string, showMap: () => void) {
  const { route } = useLocation();
  useEffect(() => {
    let pendingGo = false;
    let expiry: ReturnType<typeof setTimeout> | undefined;
    const onKey = (event: KeyboardEvent) => {
      if (ignoresKeys(event)) {
        return;
      }
      if (event.key === "?") {
        showMap();
      } else if (event.key === "g") {
        pendingGo = true;
        clearTimeout(expiry);
        expiry = setTimeout(() => (pendingGo = false), 1_000);
        return;
      } else if (pendingGo) {
        const screen = screens.find((s) => s.key === event.key);
        if (screen !== undefined) {
          route(screenPath(account, screen.path));
        }
      }
      pendingGo = false;
    };
    document.addEventListener("keydown", onKey);
    return () => {
      clearTimeout(expiry);
      document.removeEventListener("keydown", onKey);
    };
  }, [account, route, showMap]);
}

function AccountSelector(props: { account: string }) {
  const { accounts } = useDeps();
  const { path, url } = useLocation();
  const state = accounts.read(accountsPath()).value;
  const listed: readonly Account[] = state.status === "ok" ? state.answer.accounts : [];
  const provider = listed.find((a) => a.account_id === props.account)?.provider;
  const search = url.includes("?") ? url.slice(url.indexOf("?")) : "";
  return (
    <Menu
      label="Account"
      button={
        <>
          <span class="mono">{props.account}</span>
          {provider === undefined ? null : <span class="muted"> · {provider}</span>}
        </>
      }
    >
      {(close) =>
        listed.map((a) => (
          <li key={a.account_id} role="none">
            <a
              role="menuitem"
              href={switchAccount(path, search, a.account_id)}
              aria-current={a.account_id === props.account ? "true" : undefined}
              onClick={close}
            >
              <span class="mono">{a.account_id}</span>
              <span class="muted">{a.provider}</span>
            </a>
          </li>
        ))
      }
    </Menu>
  );
}

function Navigation(props: { account: string }) {
  const { path } = useLocation();
  return (
    <nav aria-label="Screens">
      <ul class="nav">
        {screens.map((s) => {
          const target = screenPath(props.account, s.path);
          return (
            <li key={s.name}>
              <a href={target} aria-current={path === target ? "page" : undefined}>
                {s.name}
              </a>
            </li>
          );
        })}
      </ul>
    </nav>
  );
}

// screenPath is a screen's path under an account.
function screenPath(account: string, path: string): string {
  return [`/${encodeURIComponent(account)}`, path].filter((part) => part !== "").join("/");
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

// partialIndex is the banner's text while a backfill pass runs, naming each running pass, or undefined
// when neither runs.
export function partialIndex(system: System): string | undefined {
  const op = system.operational;
  const sentences: string[] = [];
  if (!op.backfill_pass1_complete && op.backfill_pass1_run?.state === "running") {
    const page = numberIn(op.backfill_pass1_run.checkpoint, "page");
    const of = numberIn(op.backfill_pass1_run.checkpoint, "of");
    sentences.push(
      page === undefined || of === undefined
        ? "Backfill pass 1 is running, so every count here is a count so far."
        : `Backfill pass 1 is running, page ${count(page)} of ${count(of)} (${share(page, of)}), so every count here is a count so far.`,
    );
  }
  if (!op.backfill_pass2_complete && op.backfill_pass2_run?.state === "running") {
    sentences.push(
      `Backfill pass 2 is running with ${count(op.pending_scan)} ${op.pending_scan === 1 ? "message" : "messages"} pending scan, and a pending message denies its body until it is scanned.`,
    );
  }
  return sentences.length === 0 ? undefined : sentences.join(" ");
}

// indexing reports whether backfill pass 1 is running, when every count on a lens is a count so far.
// A caller without an answer from the system read counts the index as partial.
export function indexing(system: System): boolean {
  const op = system.operational;
  return !op.backfill_pass1_complete && op.backfill_pass1_run?.state === "running";
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
// account's event stream and reads the block again on every backfill run event and on every poll of
// the stream's fallback, so its figures move with the pass. It is not a live surface and shows no live
// indicator (docs/UI.md section 9).
function PartialIndexBanner(props: { account: string }) {
  const { system, connect } = useDeps();
  const { account } = props;
  const path = systemPath(account);
  const text = bannerText(system.read(path).value);
  const showing = text !== undefined;
  useEffect(() => {
    if (!showing) {
      return undefined;
    }
    const objects = new LiveObjects(account);
    const refresh = () => void system.refresh(path);
    const subscription = connect(account, objects, refresh);
    const stop = effect(() => {
      if (objects.lastRun.value?.workload === "backfill") {
        refresh();
      }
    });
    return () => {
      stop();
      subscription.stop();
    };
  }, [showing, account, path, system, connect]);
  return text === undefined ? null : (
    <p class="banner" role="status">
      {text}
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
