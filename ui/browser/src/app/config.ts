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
};

export const configDefaults: BrowserConfig = {
  defaultTheme: "system",
  reconnectMax: 30_000,
  pollInterval: 5_000,
};

// readConfig reads each key through read, which returns a meta tag's content by key.
export function readConfig(read: (key: string) => string | undefined): BrowserConfig {
  const theme = read("default_theme");
  return {
    defaultTheme: themes.find((t) => t === theme) ?? configDefaults.defaultTheme,
    reconnectMax: milliseconds(read("stream_reconnect_max")) ?? configDefaults.reconnectMax,
    pollInterval: milliseconds(read("stream_poll_interval")) ?? configDefaults.pollInterval,
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
