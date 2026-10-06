// The base policy's installation screens of docs/UI.md section 8.14, and the installation's base policy
// item of section 8.10, rendered by the router over answers recorded from the real server
// (ui/internal/api/policy_fixtures_integration_test.go). They show no account's state, only each
// account's identifier, so no count of senders or messages appears on them.
import { afterEach, expect, test } from "bun:test";
import { act } from "preact/test-utils";
import { accountsPath, baseMatchPath, installationPath } from "../src/app/api.ts";
import { pause } from "../src/screens/addrule.tsx";
import { App } from "../src/app/router.tsx";
import { ManualTimers, testDeps } from "./app.ts";
import { ok, recorded, type Answer, type Recorded } from "./fixtures/fetch.ts";
import { at, mount, settle, type Mounted } from "./render.ts";

let mounted: Mounted | undefined;
let server: Recorded | undefined;
afterEach(() => {
  mounted?.unmount();
  mounted = undefined;
  expect(server?.missing ?? []).toEqual([]);
  server = undefined;
});

const base: Record<string, Answer | readonly Answer[]> = {
  [accountsPath()]: ok("accounts.json"),
  "/api/setup/policy": ok("base-policy.json"),
};

async function open(
  path: string,
  answers: Readonly<Record<string, Answer | readonly Answer[]>>,
): Promise<HTMLElement> {
  server = recorded(answers);
  at(path);
  mounted = mount(<App deps={testDeps(server, [], undefined, new ManualTimers())} />);
  await settle();
  return mounted.root;
}

function text(root: Element, selector: string): string[] {
  return [...root.querySelectorAll(selector)].map((e) => e.textContent?.trim() ?? "");
}

function control(root: ParentNode, label: string): HTMLElement | undefined {
  return [...root.querySelectorAll<HTMLElement>("button, a")].find(
    (b) => b.textContent?.trim() === label,
  );
}

async function click(element: Element | null | undefined): Promise<void> {
  if (!(element instanceof HTMLElement)) {
    throw new Error("no such control");
  }
  await act(() => element.click());
  await settle();
}

async function type(input: Element | null | undefined, value: string): Promise<void> {
  if (!(input instanceof HTMLInputElement)) {
    throw new Error("no such field");
  }
  await act(() => {
    input.value = value;
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await settle();
}

function dialog(): HTMLElement | null {
  return document.querySelector('[role="alertdialog"]');
}

test("the base policy lists its rules and the accounts today, with no account's counts, under the installation's navigation", async () => {
  const root = await open("/setup/policy", base);
  expect(root.querySelector("main h1")?.textContent).toBe("Base policy");
  expect(root.querySelector(".topbar button")?.textContent).toBe("Installation");
  expect(
    [...root.querySelectorAll('nav[aria-label="Screens"] a')].map((a) => [
      a.textContent,
      a.getAttribute("href"),
      a.getAttribute("aria-current"),
    ]),
  ).toEqual([
    ["Setup", "/setup", null],
    ["Base policy", "/setup/policy", "page"],
  ]);
  expect(root.querySelector("main p")?.textContent).toBe(
    "These rules restrict senders in every account, those connected later included. The accounts today: other, personal.",
  );
  expect([...root.querySelectorAll("main p a")].map((a) => a.getAttribute("href"))).toEqual([
    "/other/policy",
    "/personal/policy",
  ]);
  expect(text(root, ".strip .figure")).toEqual(["1base rules", "2026-09-09 22:16Zlatest change"]);
  expect(
    [...root.querySelectorAll('nav[aria-label="Base policy actions"] a')].map((a) => [
      a.textContent,
      a.getAttribute("href"),
    ]),
  ).toEqual([
    ["Add a base rule", "/setup/policy/new"],
    ["History", "/setup/policy/history"],
    ["Import", "/setup/policy/import"],
    ["Export", "/api/setup/policy/export"],
  ]);
  expect(
    [...root.querySelectorAll("tbody tr")].map((tr) =>
      [...tr.querySelectorAll("td")].map((td) => td.textContent),
    ),
  ).toEqual([["base.bank", "bank.example", "operator", "2026-09-07 10:16Z · operator"]]);
  expect(root.querySelector("tbody a")?.getAttribute("href")).toBe("/setup/policy/base.bank");
  // No count of any account's senders or messages appears.
  expect(root.textContent).not.toMatch(/\d (sender|message)/);
});

test("the base policy's search narrows the rules through its endpoint", async () => {
  const root = await open("/setup/policy", {
    ...base,
    "/api/setup/policy?search=bank": ok("base-policy-search.json"),
  });
  await type(root.querySelector('form[role="search"] input'), "bank");
  await act(() => root.querySelector<HTMLFormElement>('form[role="search"]')?.requestSubmit());
  await settle();
  expect(location.pathname + location.search).toBe("/setup/policy?search=bank");
  expect(text(root, "tbody td:first-child")).toEqual(["base.bank"]);
});

test("a base rule's lift names every account that loses it, needs its identifier typed, and offers Put it back", async () => {
  const root = await open("/setup/policy/base.bank", {
    ...base,
    "/api/setup/policy": [ok("base-policy.json"), ok("base-policy-lifted.json")],
    "/api/setup/policy/history?range=all&rule=base.bank": ok("base-history-bank.json"),
    "POST /api/setup/policy/rules/base.bank/lift": ok("base-lifted.json"),
  });
  const panel = document.querySelector('aside[role="dialog"]');
  expect(panel?.textContent).toContain(
    "Only for one account? Open that account's policy and use Change where this applies…",
  );
  expect(text(panel ?? root, ".rule-actions button")).toEqual(["Edit domains", "Lift restriction"]);
  expect(text(panel ?? root, "tbody td:nth-child(3)")).toEqual(["added"]);
  await click(control(panel ?? root, "Lift restriction"));
  const d = dialog();
  expect(d?.textContent).toContain("Every account loses this restriction: other, personal.");
  expect(d?.textContent).toContain(
    "From each process's next policy reload, the senders it alone restricted no longer have their bodies denied.",
  );
  expect(document.activeElement?.textContent).toBe("Cancel");
  const lift = () => control(dialog() ?? root, "Lift restriction on base.bank");
  expect(lift()?.hasAttribute("disabled")).toBe(true);
  await type(d?.querySelector("input"), "base.bank");
  await click(lift());
  expect(server?.posted).toEqual([
    {
      path: "/api/setup/policy/rules/base.bank/lift",
      body: { suffixes_before: ["bank.example"], confirmation: "base.bank" },
    },
  ]);
  expect(location.pathname).toBe("/setup/policy");
  expect(root.querySelector('.policy [role="status"]')?.textContent).toBe(
    "Lifted base.bank. Put it back",
  );
});

test("with no account connected, a base lift says nothing is released now", async () => {
  await open("/setup/policy/base.bank", {
    [accountsPath()]: ok("accounts-empty.json"),
    "/api/setup/policy": ok("base-policy-no-accounts.json"),
    "/api/setup/policy/history?range=all&rule=base.bank": ok("base-history-bank.json"),
  });
  await click(control(document, "Lift restriction"));
  const says = document.getElementById(dialog()?.getAttribute("aria-describedby") ?? "");
  expect(says?.textContent).toStartWith("No account is connected, so nothing is released now.");
});

test("Add a base rule checks each suffix's shape as it is typed, and adds to the base policy alone", async () => {
  const timers = new ManualTimers();
  server = recorded({
    ...base,
    "/api/setup/policy": [ok("base-policy.json"), ok("base-policy-added.json")],
    "/api/setup/policy/history?range=all&rule=base.newsletter": ok("base-history-newsletter.json"),
    "POST /api/setup/policy/rules": ok("base-added.json"),
    [baseMatchPath(["bad suffix"])]: ok("base-match-invalid.json"),
    [baseMatchPath(["newsletter.example"])]: ok("base-match-newsletter.json"),
  });
  at("/setup/policy/new");
  mounted = mount(<App deps={testDeps(server, [], undefined, timers)} />);
  await settle();
  const panel = document.querySelector('aside[role="dialog"]');
  expect(panel?.getAttribute("aria-label")).toBe("Add a base rule");
  expect(panel?.querySelector('input[type="radio"]')).toBeNull();
  const id = panel?.querySelector<HTMLInputElement>('label input[type="text"].mono');
  expect(document.activeElement).toBe(id ?? null);
  expect(panel?.textContent).toContain(
    "Match counts are per account. Each account's policy screen shows what this would restrict there.",
  );
  expect(panel?.textContent).toContain(
    "Restricts these domains in every account. From each process's next policy reload their bodies are denied.",
  );
  const line = () => panel?.querySelector<HTMLInputElement>(".suffix-line input");
  await type(line(), "bad suffix");
  const verdict = () =>
    document.getElementById(line()?.getAttribute("aria-describedby") ?? "")?.textContent;
  await act(() => timers.advance(pause));
  await settle();
  expect(verdict()).toBe("Not shaped like a domain name. ");
  expect(control(panel ?? document, "Add rule")?.hasAttribute("disabled")).toBe(true);
  await type(line(), "newsletter.example");
  await act(() => timers.advance(pause));
  await settle();
  expect(verdict()).toBe("");
  expect(id?.value).toBe("operator.newsletter.example");
  await type(id, "base.newsletter");
  await click(control(panel ?? document, "Add rule"));
  expect(server.posted).toEqual([
    {
      path: "/api/setup/policy/rules",
      body: { rule_id: "base.newsletter", suffixes: ["newsletter.example"] },
    },
  ]);
  expect(location.pathname).toBe("/setup/policy/base.newsletter");
});

test("Add a base rule warns of a public suffix and names the base rule already matching a suffix, refusing neither", async () => {
  const timers = new ManualTimers();
  server = recorded({
    ...base,
    [baseMatchPath(["com"])]: ok("base-match-public.json"),
    [baseMatchPath(["bank.example"])]: ok("base-match-bank.json"),
  });
  at("/setup/policy/new");
  mounted = mount(<App deps={testDeps(server, [], undefined, timers)} />);
  await settle();
  const panel = document.querySelector('aside[role="dialog"]');
  const line = () => panel?.querySelector<HTMLInputElement>(".suffix-line input");
  const verdict = () =>
    document.getElementById(line()?.getAttribute("aria-describedby") ?? "")?.textContent;
  await type(line(), "com");
  await act(() => timers.advance(pause));
  await settle();
  expect(verdict()).toBe("This is a public suffix. It restricts every sender under it.");
  expect(control(panel ?? document, "Add rule")?.hasAttribute("disabled")).toBe(false);
  await type(line(), "bank.example");
  await act(() => timers.advance(pause));
  await settle();
  expect(verdict()).toBe("Already matched by the base rule base.bank. ");
  expect(control(panel ?? document, "Add rule")?.hasAttribute("disabled")).toBe(false);
});

test("a base import lifting a rule enables only once lift {k} is typed, and sends it", async () => {
  const root = await open("/setup/policy/import", {
    ...base,
    "/api/setup/policy": [ok("base-policy.json"), ok("base-policy-imported.json")],
    "POST /api/setup/policy/import/preview": ok("base-preview-lifting.json"),
    "POST /api/setup/policy/import": ok("base-imported.json"),
  });
  expect(root.querySelector("main h1")?.textContent).toBe("Import the base policy");
  expect(root.textContent).toContain(
    "The file replaces the base rules, which every account inherits.",
  );
  const input = root.querySelector<HTMLInputElement>('input[type="file"]');
  const file =
    "rules:\n  - id: base.new\n    domain_suffix: [new.example]\n    class: restricted\n";
  Object.defineProperty(input, "files", {
    configurable: true,
    value: [new File([file], "policy-base.yaml")],
  });
  await act(async () => {
    input?.dispatchEvent(new Event("change", { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  await settle();
  // The base policy's preview carries no account's numbers.
  expect(root.querySelector(".preview-group.lifts li")?.textContent).toBe(
    "base.bank, bank.example",
  );
  await click(control(root, "Import, lifting 1 restriction"));
  const d = dialog();
  expect(d?.textContent).toContain("Every account loses these restrictions: other, personal.");
  const field = d?.querySelector("input");
  expect(field?.closest("label")?.textContent).toBe("Type lift 1 to confirm ");
  const apply = () => control(dialog() ?? root, "Import and lift 1");
  expect(apply()?.hasAttribute("disabled")).toBe(true);
  expect(d?.textContent).toContain("Type lift 1 to enable");
  await type(field, "lift 2");
  expect(apply()?.hasAttribute("disabled")).toBe(true);
  await type(field, "lift 1");
  await click(apply());
  expect(server?.posted[1]).toEqual({
    path: "/api/setup/policy/import",
    body: {
      file,
      computed_against: '[{"id":"base.bank","class":"restricted","suffixes":["bank.example"]}]',
      confirmation: "lift 1",
    },
  });
  expect(location.pathname).toBe("/setup/policy");
  expect(root.querySelector('.policy [role="status"]')?.textContent).toBe(
    "Imported: 1 rule added, 0 rules edited, 1 restriction lifted. Put back the 1 lifted restriction",
  );
});

test("the base policy's history lists its changes alone, a lifted rule's Restore opening Add a base rule", async () => {
  const root = await open("/setup/policy/history", {
    ...base,
    "/api/setup/policy/history?range=30d": ok("base-history.json"),
  });
  const rows = [...root.querySelectorAll("tbody tr")];
  expect(rows.map((tr) => [...tr.querySelectorAll("td")].map((td) => td.textContent))).toEqual([
    ["2026-09-09 22:16Z", "operator", "lifted", "base.old", "base", "−old.example", "Restore"],
    ["2026-09-07 10:16Z", "operator", "added", "base.bank", "base", "bank.example", ""],
  ]);
  expect(control(rows[0] ?? root, "Restore")?.getAttribute("href")).toBe(
    "/setup/policy/new?suffix=old.example&id=base.old",
  );
  expect(rows[1]?.querySelector("td:nth-child(4) a")?.getAttribute("href")).toBe(
    "/setup/policy/base.bank",
  );
  expect(
    [...root.querySelectorAll(".range a")].map((a) => [
      a.getAttribute("href"),
      a.getAttribute("aria-current"),
    ]),
  ).toContainEqual(["/setup/policy/history?range=30d", "true"]);
});

test("the installation's base policy item counts the base rules and is optional", async () => {
  const root = await open("/setup", {
    [accountsPath()]: ok("accounts-empty.json"),
    [installationPath()]: ok("setup-base-rules.json"),
  });
  const item = [...root.querySelectorAll(".checklist li")][1];
  expect(item?.textContent).toBe("○ 1 base rule · optional Review the base policy");
  expect(item?.querySelector("a")?.getAttribute("href")).toBe("/setup/policy");
  expect(item?.querySelector(".step-status")?.getAttribute("data-state")).toBe("optional");
});
