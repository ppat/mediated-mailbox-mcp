// The browser app's entry point and composition root, the one module scripts/build.ts bundles.
// Everything the bundle holds is reached from here, so a module nothing here imports never reaches dist
// or the Go binary. It reads the configuration the entry document carries, binds the app's dependencies
// to the browser's own fetch, clock, storage and event stream, and renders the app into the entry
// document's body.
import { h, render } from "preact";
import "./theme.css";
import { eventsPath } from "./api.ts";
import { metaReader, readConfig } from "./config.ts";
import { browserTimers, makeDeps, regionTiming } from "./deps.ts";
import { consoleTab, type GuideMessage, type Guides } from "./guide.ts";
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

// The guide's channel is one BroadcastChannel of the origin, which reaches the page and every guide
// window, and never the document that posted.
const broadcast = new BroadcastChannel("mediated-mailbox-guide");

// pictureInPicture is the browser's Document Picture-in-Picture, where it has one.
function pictureInPicture(): Guides["pip"] {
  const api: unknown = Reflect.get(window, "documentPictureInPicture");
  if (typeof api !== "object" || api === null) {
    return undefined;
  }
  const request: unknown = Reflect.get(api, "requestWindow");
  if (typeof request !== "function") {
    return undefined;
  }
  return async (width, height) => {
    const opened: unknown = await Reflect.apply(request, api, [{ width, height }]);
    if (!(opened instanceof Window)) {
      throw new Error("the browser opened no window");
    }
    return opened;
  };
}

const guides: Guides = {
  channel: {
    // oxlint-disable-next-line unicorn/require-post-message-target-origin -- a BroadcastChannel's postMessage takes no target origin
    post: (message) => broadcast.postMessage(message),
    listen: (on) => {
      const handle = (event: MessageEvent<GuideMessage>) => on(event.data);
      broadcast.addEventListener("message", handle);
      return () => broadcast.removeEventListener("message", handle);
    },
  },
  pip: pictureInPicture(),
  open: (url, name, features) => window.open(url, name, features),
  copy: (text) => navigator.clipboard.writeText(text),
  paste: () => navigator.clipboard.readText(),
  console: (url) => {
    window.open(url, consoleTab);
  },
};
applyTheme(document.documentElement, storedTheme(storage, config.defaultTheme));
const deps = makeDeps(
  (path, signal) =>
    fetch(path, { headers: { Accept: "application/json" }, credentials: "same-origin", signal }),
  (path, body) =>
    fetch(path, {
      method: "POST",
      headers: {
        Accept: "application/json",
        "Content-Type": "application/json",
        "X-Request-Token": config.requestToken,
      },
      credentials: "same-origin",
      body: JSON.stringify(body),
    }),
  guides,
  now,
  storage,
  regionTiming,
  browserTimers,
  config,
  (account, objects, refetch) => subscribe(eventsPath(account), objects, refetch, now, config),
);
render(h(App, { deps }), document.body);
