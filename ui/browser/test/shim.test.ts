// Proves the runner preloads the DOM shim. This module does not import the preload, so the globals it
// reads exist only if bunfig.toml registered them before the module loaded.
import { expect, test } from "bun:test";

const originAtLoad = globalThis.location?.origin;

test("the shim is registered at its origin before test modules load", () => {
  expect(originAtLoad).toBe("https://ui.mediated-mailbox.test");
});

test("text set on a node stays text", () => {
  const node = document.createElement("p");
  node.textContent = "<b>marker</b>";
  expect(node.childElementCount).toBe(0);
  expect(node.textContent).toBe("<b>marker</b>");
});
