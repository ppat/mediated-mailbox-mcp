// The browser app's entry point and composition root, the one module scripts/build.ts bundles.
// Everything the bundle holds is reached from here, so a module nothing here imports never reaches dist
// or the Go binary. It reads the configuration the entry document carries, binds the app's dependencies
// to the browser's own fetch, clock, storage and event stream, and renders the app into the entry
// document's body.
import { h, render } from "preact";
import "./theme.css";
import { eventsPath } from "./api.ts";
import { metaReader, readConfig } from "./config.ts";
import { makeDeps, regionTiming } from "./deps.ts";
import { App } from "./router.tsx";
import { applyTheme, storedTheme } from "./theme.ts";
import { subscribe } from "./transport.ts";

function browserStorage(): Storage | undefined {
  try {
    return window.localStorage;
  } catch {
    return undefined;
  }
}

const config = readConfig(metaReader(document));
const storage = browserStorage();
const now = () => Date.now();
applyTheme(document.documentElement, storedTheme(storage, config.defaultTheme));
const deps = makeDeps(
  (path, signal) =>
    fetch(path, { headers: { Accept: "application/json" }, credentials: "same-origin", signal }),
  now,
  storage,
  regionTiming,
  config,
  (account, objects, refetch) => subscribe(eventsPath(account), objects, refetch, now, config),
);
render(h(App, { deps }), document.body);
