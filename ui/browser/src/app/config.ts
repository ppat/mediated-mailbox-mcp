// The configuration the browser reads from the entry document, which the Go handler renders as meta
// tags (docs/UI.md section 18.1). main.ts is the one reader. A tag that is absent or cannot be read
// takes the record's default, which is the value the server defaults to as well.
import { themes, type Theme } from "./theme.ts";

export type BrowserConfig = {
  defaultTheme: Theme;
  // reconnectMax is the stream's reconnection backoff ceiling, and pollInterval the polling fallback's
  // interval, both in milliseconds (ADR-0058).
  reconnectMax: number;
  pollInterval: number;
  // consentRedirect is the loopback address a consent redirects to, which the connect page pictures
  // and checks a pasted address against (docs/UI.md section 8.12). The browser holds no copy of it, so
  // a page without the tag pictures no address and finds no paste to be on it.
  consentRedirect: string;
  // requestToken is the session's request token, which every state-changing request carries
  // (ADR-0061). An entry document without one sends requests the server refuses as a stale page.
  requestToken: string;
};

export const configDefaults: BrowserConfig = {
  defaultTheme: "system",
  reconnectMax: 30_000,
  pollInterval: 5_000,
  consentRedirect: "",
  requestToken: "",
};

// readConfig reads each key through read, which returns a meta tag's content by key.
export function readConfig(read: (key: string) => string | undefined): BrowserConfig {
  const theme = read("default_theme");
  return {
    defaultTheme: themes.find((t) => t === theme) ?? configDefaults.defaultTheme,
    reconnectMax: milliseconds(read("stream_reconnect_max")) ?? configDefaults.reconnectMax,
    pollInterval: milliseconds(read("stream_poll_interval")) ?? configDefaults.pollInterval,
    consentRedirect: read("consent_redirect") ?? configDefaults.consentRedirect,
    requestToken: read("request_token") ?? configDefaults.requestToken,
  };
}

function milliseconds(value: string | undefined): number | undefined {
  if (value === undefined || !/^[1-9][0-9]*$/.test(value)) {
    return undefined;
  }
  return Number(value);
}

// metaReader reads a key's meta tag from a document.
export function metaReader(document: Document): (key: string) => string | undefined {
  return (key) =>
    document.querySelector<HTMLMetaElement>(`meta[name="mediated-mailbox.${key}"]`)?.content;
}
