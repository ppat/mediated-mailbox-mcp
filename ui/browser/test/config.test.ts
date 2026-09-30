// The configuration the entry document carries for the browser (docs/UI.md section 18.1).
import { afterEach, expect, test } from "bun:test";
import { configDefaults, metaReader, readConfig } from "../src/app/config.ts";

afterEach(() => {
  for (const meta of document.head.querySelectorAll('meta[name^="mediated-mailbox."]')) {
    meta.remove();
  }
});

function carry(values: Record<string, string>): void {
  for (const [key, content] of Object.entries(values)) {
    const meta = document.createElement("meta");
    meta.name = `mediated-mailbox.${key}`;
    meta.content = content;
    document.head.append(meta);
  }
}

test("each key is read from its meta tag, durations in whole milliseconds", () => {
  carry({ default_theme: "dark", stream_reconnect_max: "45000", stream_poll_interval: "2500" });
  expect(readConfig(metaReader(document))).toEqual({
    defaultTheme: "dark",
    reconnectMax: 45_000,
    pollInterval: 2_500,
  });
});

test("an absent tag takes the record's default", () => {
  expect(readConfig(metaReader(document))).toEqual({
    defaultTheme: "system",
    reconnectMax: 30_000,
    pollInterval: 5_000,
  });
  expect(configDefaults).toEqual({
    defaultTheme: "system",
    reconnectMax: 30_000,
    pollInterval: 5_000,
  });
});

test("a tag the browser cannot read takes the record's default", () => {
  for (const [theme, duration] of [
    ["dim", "0"],
    ["", "1.5"],
    ["DARK", "-3"],
    ["light ", "30s"],
  ] as const) {
    const config = readConfig((key) => (key === "default_theme" ? theme : duration));
    expect(config).toEqual({ defaultTheme: "system", reconnectMax: 30_000, pollInterval: 5_000 });
  }
});
