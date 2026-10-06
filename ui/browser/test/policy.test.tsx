// An account's policy screens of docs/UI.md section 8.7, rendered by the router over answers recorded
// from the real server (ui/internal/api/policy_fixtures_integration_test.go). The base policy holds
// base.bank, restricting bank.example, and personal holds operator.lender.example. The sender picker's
// senders include one whose domain carries the markup marker, and the inert-rendering tests check every
// surface a domain, a rule identifier or a suffix reaches in the form ADR-0064 requires. Their mutation
// demonstrations are under ui/browser/testdata/mutations.
import { afterEach, expect, test } from "bun:test";
import { act } from "preact/test-utils";
import { LocationProvider } from "preact-iso";
import {
  accountsPath,
  lensPath,
  matchPath,
  releasePath,
  rulePath,
  systemPath,
  type ChangeRow,
  type ImportPreview,
  type RuleDetail,
  type RuleRow,
  type SenderRow,
} from "../src/app/api.ts";
import { App } from "../src/app/router.tsx";
import { RowsTable } from "../src/lens/table.tsx";
import { changeColumns } from "../src/row/change.tsx";
import { senderColumns } from "../src/row/sender.tsx";
import { pause } from "../src/screens/addrule.tsx";
import { ruleColumns } from "../src/screens/policy.tsx";
import { ManualTimers, testDeps } from "./app.ts";
import { ok, recorded, type Answer, type Recorded } from "./fixtures/fetch.ts";
import { at, mount, settle, type Mounted } from "./render.ts";

let mounted: Mounted | undefined;
let server: Recorded | undefined;
let timers = new ManualTimers();
afterEach(() => {
  mounted?.unmount();
  mounted = undefined;
  expect(server?.missing ?? []).toEqual([]);
  server = undefined;
  timers = new ManualTimers();
});

const marker = (tag: string) => `<script>mmfieldmarker-${tag}</script>`;
const refusedAnswer = (file: string, status = 400): Answer => ({ file, status });

const rulesQuery = "dataset=rules&level=3&range=all&sort=rule_id,asc&page=1";
const figuresQuery = "dataset=rules&level=0&range=all&sort=rule_id,asc";
const rules = lensPath("personal", rulesQuery);
const figures = lensPath("personal", figuresQuery);
const lender = rulePath("personal", "account", "operator.lender.example");
const bank = rulePath("personal", "base", "base.bank");

// list are the reads of the account's policy list as seeded, which every policy screen of the account
// shows behind a panel.
const list: Record<string, Answer | readonly Answer[]> = {
  [accountsPath()]: ok("accounts.json"),
  [systemPath("personal")]: ok("system.json"),
  [figures]: ok("rules-summary.json"),
  [rules]: ok("rules-rows.json"),
};

async function open(
  path: string,
  answers: Readonly<Record<string, Answer | readonly Answer[]>>,
  state?: unknown,
): Promise<HTMLElement> {
  server = recorded(answers);
  at(path);
  if (state !== undefined) {
    history.replaceState(state, "", path);
  }
  mounted = mount(<App deps={testDeps(server, [], undefined, timers)} />);
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
  if (!(input instanceof HTMLInputElement || input instanceof HTMLTextAreaElement)) {
    throw new Error("no such field");
  }
  await act(() => {
    input.value = value;
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
  await settle();
}

async function press(key: string, init: KeyboardEventInit = {}, target: EventTarget = document) {
  await act(() => {
    target.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true, ...init }));
  });
  await settle();
}

async function wait(ms: number): Promise<void> {
  await act(() => timers.advance(ms));
  await settle();
}

// area is the Edit domains text area, and panel the panel open over the policy screen.
function area(): Element | null {
  return document.querySelector(".edit-domains textarea");
}

function openPanel(): Element | null {
  return document.querySelector('aside[role="dialog"]');
}

function dialog(): HTMLElement | null {
  return document.querySelector('[role="alertdialog"]');
}

// --- The list ------------------------------------------------------------------------------------

test("the policy lists the base rules marked base and then the account's own, each linking to its scope", async () => {
  const root = await open("/personal/policy", list);
  expect(location.search).toBe(`?${rulesQuery.replace("dataset=rules&", "")}`);
  expect(root.querySelector("main h1")?.textContent).toBe("Policy");
  const nav = [...root.querySelectorAll('nav[aria-label="Screens"] a')];
  expect(nav.find((a) => a.textContent === "Policy")?.getAttribute("aria-current")).toBe("page");
  const rows = [...root.querySelectorAll("tbody tr")].map((tr) =>
    [...tr.querySelectorAll("td")].map((td) => td.textContent),
  );
  expect(rows).toEqual([
    [
      "base.bank",
      "base",
      "bank.example",
      "operator",
      "2026-09-07 10:16Z · operator",
      "1",
      "index updated, 1 of 1",
    ],
    [
      "operator.lender.example",
      "personal",
      "lender.example",
      "operator",
      "2026-09-07 10:16Z · operator",
      "1",
      "index updated, 0 of 1",
    ],
  ]);
  expect(
    [...root.querySelectorAll("tbody td:first-child a")].map((a) => a.getAttribute("href")),
  ).toEqual(["/personal/policy/base/base.bank", "/personal/policy/operator.lender.example"]);
  expect(root.querySelector("tbody td:last-child")?.getAttribute("title")).toBe(
    "Bodies are denied from each process's next policy reload. This counts the senders whose stored class reads restricted.",
  );
  expect(text(root, ".strip .label")).toEqual([
    "base rules",
    "rules for this account",
    "senders restricted",
    "latest change",
  ]);
  const actions = root.querySelector('nav[aria-label="Policy actions"]');
  expect(
    [...(actions?.querySelectorAll("a") ?? [])].map((a) => [a.textContent, a.getAttribute("href")]),
  ).toEqual([
    ["Add a rule", "/personal/policy/new"],
    ["Restrict senders from the index", "/personal/policy/pick"],
    ["History", "/personal/policy/history"],
    ["Import", "/personal/policy/import"],
    ["Export", "/api/personal/policy/export"],
  ]);
  expect(control(root, "Export")?.hasAttribute("download")).toBe(true);
  // The account holds a rule of its own, so its part of the table has no empty line.
  expect(root.textContent).not.toContain("No rules for personal alone.");
});

test("the search answers which rule holds a domain, through the rules dataset's search filter", async () => {
  const root = await open("/personal/policy", {
    ...list,
    [lensPath("personal", `${figuresQuery}&search=bank`)]: ok("rules-summary-search.json"),
    [lensPath("personal", `${rulesQuery}&search=bank`)]: ok("rules-rows-search.json"),
  });
  await type(root.querySelector('form[role="search"] input'), " bank ");
  await act(() => root.querySelector<HTMLFormElement>('form[role="search"]')?.requestSubmit());
  await settle();
  expect(location.search).toBe("?level=3&range=all&sort=rule_id,asc&page=1&search=bank");
  expect(text(root, "tbody td:first-child")).toEqual(["base.bank"]);
});

test("with no rule of its own the account's part of the table says the base rules apply to every account", async () => {
  const root = await open("/personal/policy", {
    ...list,
    [figures]: ok("rules-summary-lifted.json"),
    [rules]: ok("rules-rows-lifted.json"),
  });
  expect(text(root, "tbody td:first-child")).toEqual(["base.bank"]);
  expect(root.textContent).toContain(
    "No rules for personal alone. The base rules above apply to every account.",
  );
});

// --- A rule --------------------------------------------------------------------------------------

test("a rule shows each suffix's match in the account, the senders it matches and its history", async () => {
  const root = await open("/personal/policy/operator.lender.example", {
    ...list,
    [lender]: ok("rule-lender.json"),
  });
  const panel = document.querySelector('aside[role="dialog"]');
  expect(panel?.getAttribute("aria-label")).toBe("Rule operator.lender.example");
  expect(root.querySelector(".frame")?.hasAttribute("inert")).toBe(true);
  expect(text(panel ?? root, ".suffix-details li")).toEqual([
    "lender.example matches 1 sender · 4 messages in personal",
  ]);
  expect(text(panel ?? root, ".matched li")).toEqual(["lender.example normal 4 messages"]);
  const history = [...(panel?.querySelectorAll("tbody tr") ?? [])];
  expect(history.map((tr) => tr.querySelector("td:nth-child(3)")?.textContent)).toEqual([
    "edited",
    "added",
  ]);
  // The edit that removed old.lender.example ends with Restore, which opens Edit domains with it added,
  // since the rule still exists.
  expect(control(history[0] ?? root, "Restore")?.getAttribute("href")).toBe(
    "/personal/policy/operator.lender.example?suffix=old.lender.example",
  );
  expect(history[0]?.querySelector('[data-change="removed"]')?.textContent).toBe(
    "−old.lender.example",
  );
  expect(control(history[1] ?? root, "Restore")).toBeUndefined();
  expect(text(panel ?? root, ".rule-actions button")).toEqual([
    "Edit domains",
    "Change where this applies…",
    "Lift restriction",
  ]);
});

test("a rule's address with no scope names the account's rule, or the base rule when the account holds none", async () => {
  await open("/personal/policy/base.bank", {
    ...list,
    [rulePath("personal", "account", "base.bank")]: refusedAnswer(
      "error-unknown-rule-row.json",
      404,
    ),
    [bank]: ok("rule-base-bank.json"),
  });
  const panel = document.querySelector('aside[role="dialog"]');
  expect(panel?.querySelector(".provenance dd")?.textContent).toBe("base.bank");
  expect(panel?.querySelector(".provenance .badge")?.textContent).toBe("base");
  expect(
    control(panel ?? document, "Open it on the base policy screen")?.getAttribute("href"),
  ).toBe("/setup/policy/base.bank");
});

test("lifting the account's rule states the account's numbers, asks no typing, and offers Put it back", async () => {
  const root = await open("/personal/policy/operator.lender.example", {
    ...list,
    [lender]: ok("rule-lender.json"),
    [figures]: [
      ok("rules-summary.json"),
      ok("rules-summary-lifted.json"),
      ok("rules-summary-put-back.json"),
    ],
    [rules]: [ok("rules-rows.json"), ok("rules-rows-lifted.json"), ok("rules-rows-put-back.json")],
    "POST /api/personal/policy/rules/operator.lender.example/lift": ok("rule-lifted.json"),
    "POST /api/personal/policy/rules": ok("rule-put-back.json"),
  });
  const opener = control(document, "Lift restriction");
  await click(opener);
  const d = dialog();
  expect(d?.getAttribute("aria-modal")).toBe("true");
  expect(document.getElementById(d?.getAttribute("aria-labelledby") ?? "")?.textContent).toBe(
    "Lift the restriction on operator.lender.example",
  );
  expect(
    document.getElementById(d?.getAttribute("aria-describedby") ?? "")?.textContent,
  ).toStartWith(
    "In personal, 1 sender and 4 stored messages are restricted by this rule and by no other rule.From each process's next policy reload, their bodies are no longer denied",
  );
  expect(document.activeElement?.textContent).toBe("Cancel");
  expect(d?.querySelector("input")).toBeNull();
  const lift = control(d ?? document, "Lift restriction on operator.lender.example");
  expect(lift?.className).toBe("lift");
  expect(lift?.hasAttribute("disabled")).toBe(false);
  await click(lift);
  expect(server?.posted).toEqual([
    {
      path: "/api/personal/policy/rules/operator.lender.example/lift",
      body: { scope: "account", suffixes_before: ["lender.example"], confirmation: null },
    },
  ]);
  expect(location.pathname).toBe("/personal/policy");
  expect(text(root, "tbody td:first-child")).toEqual(["base.bank"]);
  const status = root.querySelector('.policy > [role="status"]');
  expect(status?.textContent).toBe("Lifted operator.lender.example. Put it back");
  await click(control(status ?? root, "Put it back"));
  expect(server?.posted[1]).toEqual({
    path: "/api/personal/policy/rules",
    body: { scope: "account", rule_id: "operator.lender.example", suffixes: ["lender.example"] },
  });
  expect(status?.textContent).toBe("Put back operator.lender.example.");
  expect(text(root, "tbody td:first-child")).toEqual(["base.bank", "operator.lender.example"]);
});

test("a base rule's lift reads the base sentences and enables only once its identifier is typed", async () => {
  await open("/personal/policy/base/base.bank", {
    ...list,
    [bank]: ok("rule-base-bank.json"),
    [figures]: [ok("rules-summary.json"), ok("rules-summary-base-lifted.json")],
    [rules]: [ok("rules-rows.json"), ok("rules-rows-base-lifted.json")],
    "POST /api/personal/policy/rules/base.bank/lift": ok("base-lifted-from-account.json"),
  });
  const opener = control(document, "Lift restriction");
  opener?.focus();
  await click(opener);
  const d = dialog();
  expect(d?.textContent).toContain(
    "This is a base rule. These counts are personal's. Each account's policy screen shows its own.",
  );
  expect(d?.textContent).toContain("Every account loses it: other, personal.");
  expect([...(d?.querySelectorAll("p a") ?? [])].map((a) => a.getAttribute("href"))).toEqual([
    "/other/policy",
    "/personal/policy",
  ]);
  const field = d?.querySelector("input");
  expect(field?.closest("label")?.textContent).toBe("Type base.bank to confirm ");
  const lift = () => control(dialog() ?? document, "Lift restriction on base.bank");
  expect(lift()?.hasAttribute("disabled")).toBe(true);
  expect(d?.textContent).toContain("Type the identifier to enable");
  for (const wrong of ["base", "BASE.BANK", "base.bank2"]) {
    await type(field, wrong);
    expect(lift()?.hasAttribute("disabled")).toBe(true);
  }
  // Escape closes the dialog and leaves the panel open, focus back on the control that opened it.
  await press("Escape", {}, field ?? document);
  expect(dialog()).toBeNull();
  expect(document.querySelector('aside[role="dialog"]')).not.toBeNull();
  expect(document.activeElement).toBe(opener ?? null);
  await click(control(document, "Lift restriction"));
  // A pasted identifier with surrounding space confirms, compared after trimming.
  await type(dialog()?.querySelector("input"), "  base.bank ");
  expect(lift()?.hasAttribute("disabled")).toBe(false);
  await click(control(dialog() ?? document, "Lift restriction on base.bank"));
  expect(server?.posted).toEqual([
    {
      path: "/api/personal/policy/rules/base.bank/lift",
      body: { scope: "base", suffixes_before: ["bank.example"], confirmation: "base.bank" },
    },
  ]);
  expect(location.pathname).toBe("/personal/policy");
});

test("removing a suffix from a base rule asks for the rule's identifier typed too", async () => {
  await open("/personal/policy/base/base.bank", {
    ...list,
    [bank]: ok("rule-base-bank.json"),
    [releasePath("personal", "base", "base.bank", [])]: ok("release-bank.json"),
  });
  await click(control(document, "Edit domains"));
  await type(area(), "bank.example.net");
  await click(control(document, "Save"));
  const d = dialog();
  expect(document.getElementById(d?.getAttribute("aria-labelledby") ?? "")?.textContent).toBe(
    "Lift the restriction on bank.example",
  );
  expect(d?.querySelector("input")?.closest("label")?.textContent).toBe(
    "Type base.bank to confirm ",
  );
  const lift = () => control(dialog() ?? document, "Lift restriction on bank.example");
  expect(lift()?.hasAttribute("disabled")).toBe(true);
  await type(dialog()?.querySelector("input"), "base.bank");
  expect(lift()?.hasAttribute("disabled")).toBe(false);
  expect(server?.posted).toEqual([]);
});

test("Tab stays inside the lift dialog", async () => {
  await open("/personal/policy/base/base.bank", { ...list, [bank]: ok("rule-base-bank.json") });
  await click(control(document, "Lift restriction"));
  const d = dialog();
  const focusable = [
    ...(d?.querySelectorAll<HTMLElement>("a[href], button:not([disabled]), input") ?? []),
  ];
  focusable[focusable.length - 1]?.focus();
  const last = document.activeElement;
  await press("Tab", {}, last ?? document);
  expect(document.activeElement).toBe(focusable[0] ?? null);
  await press("Tab", { shiftKey: true }, document.activeElement ?? document);
  expect(document.activeElement).toBe(last);
});

test("Edit domains that only adds saves at once, and one that removes a suffix goes through the lift dialog", async () => {
  const root = await open("/personal/policy/operator.lender.example", {
    ...list,
    [lender]: [ok("rule-lender.json"), ok("rule-lender-two.json"), ok("rule-lender-removed.json")],
    [releasePath("personal", "account", "operator.lender.example", ["lender.example"])]:
      ok("release-lendermail.json"),
    "POST /api/personal/policy/rules/operator.lender.example": [
      ok("rule-edited.json"),
      ok("rule-suffix-removed.json"),
      // Put it back sends the first edit's request again from the state the first edit started from.
      ok("rule-edited.json"),
    ],
  });
  await click(control(document, "Edit domains"));
  await type(area(), "lender.example\nlendermail.example");
  expect(text(document.body, ".edit-domains .diff li")).toEqual(["+lendermail.example"]);
  await click(control(document, "Save"));
  expect(dialog()).toBeNull();
  expect(server?.posted).toEqual([
    {
      path: "/api/personal/policy/rules/operator.lender.example",
      body: {
        scope: "account",
        suffixes_before: ["lender.example"],
        suffixes: ["lender.example", "lendermail.example"],
        confirmation: null,
      },
    },
  ]);
  expect(openPanel()?.querySelector('[role="status"]')?.textContent).toBe(
    "Added lendermail.example to operator.lender.example.",
  );
  expect(text(openPanel() ?? root, ".suffix-details li")).toEqual([
    "lender.example matches 1 sender · 4 messages in personal",
    "lendermail.example matches 0 senders · 0 messages in personal",
  ]);
  await click(control(document, "Edit domains"));
  await type(area(), "lender.example");
  expect(text(document.body, ".edit-domains .diff li")).toEqual(["−lendermail.example"]);
  await click(control(document, "Save"));
  expect(server?.posted.length).toBe(1);
  const d = dialog();
  expect(document.getElementById(d?.getAttribute("aria-labelledby") ?? "")?.textContent).toBe(
    "Lift the restriction on lendermail.example",
  );
  expect(d?.textContent).toContain(
    "In personal, 0 senders and 0 stored messages are restricted by this suffix and by no other rule.",
  );
  await click(control(d ?? document, "Lift restriction on lendermail.example"));
  expect(server?.posted[1]).toEqual({
    path: "/api/personal/policy/rules/operator.lender.example",
    body: {
      scope: "account",
      suffixes_before: ["lender.example", "lendermail.example"],
      suffixes: ["lender.example"],
      confirmation: null,
    },
  });
  const status = openPanel()?.querySelector('[role="status"]');
  expect(status?.textContent).toBe("Lifted lendermail.example. Put it back");
  await click(control(status ?? root, "Put it back"));
  expect(server?.posted[2]).toEqual({
    path: "/api/personal/policy/rules/operator.lender.example",
    body: {
      scope: "account",
      suffixes_before: ["lender.example"],
      suffixes: ["lender.example", "lendermail.example"],
      confirmation: null,
    },
  });
});

test("an edit removing several suffixes states what the set releases together, in one sentence", async () => {
  await open("/personal/policy/operator.lender.example", {
    ...list,
    [lender]: ok("rule-lender-three.json"),
    [releasePath("personal", "account", "operator.lender.example", ["lender.example"])]: ok(
      "release-newsletters.json",
    ),
  });
  await click(control(document, "Edit domains"));
  await type(area(), "lender.example");
  await click(control(document, "Save"));
  const says = document.getElementById(dialog()?.getAttribute("aria-describedby") ?? "");
  // Each suffix alone releases 1 sender and 3 messages, and nothing; together they release the
  // sender only both cover as well.
  expect(says?.querySelector("p")?.textContent).toBe(
    "In personal, 2 senders and 4 stored messages are restricted by these suffixes and by no other rule.",
  );
  expect(says?.textContent?.match(/In personal,/g)?.length).toBe(1);
  expect(server?.posted).toEqual([]);
});

test("an edit removing every suffix is not an edit, and offers Lift restriction instead", async () => {
  await open("/personal/policy/operator.lender.example", {
    ...list,
    [lender]: ok("rule-lender.json"),
  });
  await click(control(document, "Edit domains"));
  await type(document.querySelector(".edit-domains textarea"), "");
  expect(control(document.querySelector(".edit-domains") ?? document, "Save")).toBeUndefined();
  expect(document.querySelector(".edit-domains")?.textContent).toContain(
    "A rule needs a domain suffix, so removing every one is not an edit.",
  );
  await click(control(document.querySelector(".edit-domains") ?? document, "Lift restriction"));
  expect(
    document.getElementById(dialog()?.getAttribute("aria-labelledby") ?? "")?.textContent,
  ).toBe("Lift the restriction on operator.lender.example");
  expect(server?.posted).toEqual([]);
});

test("a history row's Restore with the rule still held opens Edit domains with the removed suffix added", async () => {
  await open("/personal/policy/operator.lender.example?suffix=old.lender.example", {
    ...list,
    [lender]: ok("rule-lender.json"),
  });
  expect(document.querySelector<HTMLTextAreaElement>(".edit-domains textarea")?.value).toBe(
    "lender.example\nold.lender.example",
  );
  expect(text(document.body, ".edit-domains .diff li")).toEqual(["+old.lender.example"]);
});

// --- Add a rule ----------------------------------------------------------------------------------

test("Add a rule opened with a suffix checks it, names what it matches, and adds the rule", async () => {
  const root = await open("/personal/policy/new?suffix=newsletter.example", {
    ...list,
    [matchPath("personal", ["newsletter.example"])]: ok("match-newsletter.json"),
    "POST /api/personal/policy/rules": ok("rule-added.json"),
    [rulePath("personal", "account", "operator.newsletter.example")]: ok("rule-newsletter.json"),
    [figures]: [ok("rules-summary.json"), ok("rules-summary-added.json")],
    [rules]: [ok("rules-rows.json"), ok("rules-rows-added.json")],
  });
  const panel = document.querySelector('aside[role="dialog"]');
  expect(panel?.getAttribute("aria-label")).toBe("Add a rule");
  // Opened with suffixes handed to it, focus starts on the Add button.
  expect(document.activeElement?.textContent).toBe("Add rule");
  const id = panel?.querySelector<HTMLInputElement>('label input[type="text"].mono');
  expect(id?.value).toBe("operator.newsletter.example");
  const line = panel?.querySelector(".suffix-line input");
  expect(document.getElementById(line?.getAttribute("aria-describedby") ?? "")?.textContent).toBe(
    "matches 2 senders · 4 messages in personal This matches 40.0% of personal's senders. It is likely broader than one institution.",
  );
  expect(panel?.textContent).toContain(
    "Restricts 2 senders and 4 stored messages in personal. From each process's next policy reload their bodies are denied.",
  );
  expect(panel?.textContent).not.toContain("Applies to every account.");
  await click(control(panel ?? root, "Add rule"));
  expect(server?.posted).toEqual([
    {
      path: "/api/personal/policy/rules",
      body: {
        scope: "account",
        rule_id: "operator.newsletter.example",
        suffixes: ["newsletter.example"],
      },
    },
  ]);
  expect(location.pathname).toBe("/personal/policy/operator.newsletter.example");
  expect(document.querySelector('aside[role="dialog"] [role="status"]')?.textContent).toBe(
    "Added operator.newsletter.example.",
  );
});

test("each typed line is checked once typing pauses, and a refused line keeps Add rule disabled", async () => {
  await open("/personal/policy/new?suffix=newsletter.example", {
    ...list,
    [matchPath("personal", ["newsletter.example"])]: ok("match-newsletter.json"),
    [matchPath("personal", ["newsletter.example", "bad suffix"])]: ok("match-invalid.json"),
    [matchPath("personal", ["newsletter.example", "com"])]: ok("match-public.json"),
  });
  const panel = document.querySelector('aside[role="dialog"]');
  const lines = () => [...(panel?.querySelectorAll<HTMLInputElement>(".suffix-line input") ?? [])];
  const verdict = (i: number) =>
    document.getElementById(lines()[i]?.getAttribute("aria-describedby") ?? "")?.textContent;
  await type(lines()[1], "bad suffix");
  // Typing has not paused yet, so nothing more was read and the line has no verdict.
  await wait(pause - 1);
  expect(server?.calls.filter((c) => c.includes("/policy/match")).length).toBe(1);
  expect(verdict(1)).toBe("");
  await wait(1);
  expect(verdict(1)).toBe("Not shaped like a domain name. ");
  expect(lines()[1]?.getAttribute("aria-invalid")).toBe("true");
  expect(control(panel ?? document, "Add rule")?.hasAttribute("disabled")).toBe(true);
  await type(lines()[1], "com");
  await wait(pause);
  expect(verdict(1)).toBe(
    "matches 0 senders · 0 messages in personal This matches 0.0% of personal's senders. It is likely broader than one institution.",
  );
  expect(control(panel ?? document, "Add rule")?.hasAttribute("disabled")).toBe(false);
});

test("Add a rule names the rule already matching a suffix, and the base scope says whose counts it shows", async () => {
  await open("/personal/policy/new?suffix=lender.example", {
    ...list,
    [matchPath("personal", ["lender.example"])]: ok("match-lender.json"),
  });
  const panel = document.querySelector('aside[role="dialog"]');
  const line = panel?.querySelector(".suffix-line input");
  expect(
    document.getElementById(line?.getAttribute("aria-describedby") ?? "")?.textContent,
  ).toContain("Already matched by personal's rule operator.lender.example.");
  const [own, base] = [...(panel?.querySelectorAll<HTMLInputElement>('input[type="radio"]') ?? [])];
  expect(own?.closest("label")?.textContent).toBe(" This account, personal");
  expect(own?.checked).toBe(true);
  await click(base);
  expect(panel?.textContent).toContain(
    "Applies to every account. The counts shown are personal's.",
  );
});

test("an identifier the screens' addresses use is refused before anything is sent, and a refusal names its cause", async () => {
  const root = await open("/personal/policy/new?suffix=lender.example", {
    ...list,
    [matchPath("personal", ["lender.example"])]: ok("match-lender.json"),
    "POST /api/personal/policy/rules": refusedAnswer("error-identifier-taken.json", 409),
  });
  const panel = document.querySelector('aside[role="dialog"]') ?? root;
  const id = panel.querySelector<HTMLInputElement>('label input[type="text"].mono');
  for (const word of ["new", "pick", "history", "import", "base", ".", "..", "a/b", " x"]) {
    await type(id, word);
    expect(control(panel, "Add rule")?.hasAttribute("disabled")).toBe(true);
  }
  await type(id, "operator.lender.example");
  await click(control(panel, "Add rule"));
  const refusal = panel.querySelector(".refusal[role='alert']");
  expect(refusal?.textContent).toBe("personal already has a rule operator.lender.example.");
  expect(document.activeElement).toBe(refusal);
});

test("a write the policy's checks refuse names each problem", async () => {
  await open("/personal/policy/new?suffix=lender.example&id=x", {
    ...list,
    [matchPath("personal", ["lender.example"])]: ok("match-lender.json"),
    "POST /api/personal/policy/rules": refusedAnswer("error-rule-refused.json"),
  });
  await click(control(document, "Add rule"));
  expect(text(document.body, ".refusal[role='alert'] li")).toEqual([
    "rule new: the suffix bad suffix is not shaped like a domain name.",
    "rule new: its identifier is a word the policy screens' addresses use, exactly . or .., or holds a /.",
  ]);
});

// --- Change where this applies -------------------------------------------------------------------

test("Change where this applies adds under the other scope first, then lifts the old rule with nothing lost", async () => {
  await open("/personal/policy/operator.lender.example", {
    ...list,
    [lender]: [ok("rule-lender.json"), ok("rule-lender-covered.json")],
    [matchPath("personal", ["lender.example"])]: ok("match-lender.json"),
    "POST /api/personal/policy/rules": ok("rule-moved-add.json"),
    "POST /api/personal/policy/rules/operator.lender.example/lift": ok("rule-moved-lift.json"),
    [rulePath("personal", "base", "operator.lender.example")]: ok("rule-lender-base.json"),
  });
  await click(control(document, "Change where this applies…"));
  await click(control(document, "Every account"));
  expect(location.pathname + location.search).toBe(
    "/personal/policy/new?suffix=lender.example&scope=base&id=operator.lender.example",
  );
  const panel = document.querySelector('aside[role="dialog"]');
  expect(
    panel?.querySelector<HTMLInputElement>('input[type="radio"]:checked')?.closest("label")
      ?.textContent,
  ).toBe(" Every account (the base policy)");
  await click(control(panel ?? document, "Add rule"));
  expect(server?.posted).toEqual([
    {
      path: "/api/personal/policy/rules",
      body: { scope: "base", rule_id: "operator.lender.example", suffixes: ["lender.example"] },
    },
  ]);
  const d = dialog();
  expect(d?.textContent).toContain(
    "Nothing in personal loses its restriction, since operator.lender.example covers every suffix.",
  );
  await click(control(d ?? document, "Lift restriction on operator.lender.example"));
  expect(server?.posted[1]).toEqual({
    path: "/api/personal/policy/rules/operator.lender.example/lift",
    body: { scope: "account", suffixes_before: ["lender.example"], confirmation: null },
  });
  expect(location.pathname).toBe("/personal/policy/base/operator.lender.example");
});

test("a change of where a rule applies whose add fails never sends the lift", async () => {
  await open(
    "/personal/policy/new?suffix=lender.example&scope=base&id=operator.lender.example",
    {
      ...list,
      [matchPath("personal", ["lender.example"])]: ok("match-lender.json"),
      "POST /api/personal/policy/rules": refusedAnswer("error-identifier-taken-base.json", 409),
    },
    { opener: "move" },
  );
  await click(control(document, "Add rule"));
  expect(server?.posted.map((p) => p.path)).toEqual(["/api/personal/policy/rules"]);
  expect(dialog()).toBeNull();
  expect(document.querySelector(".refusal[role='alert']")?.textContent).toBe(
    "The base policy already has a rule operator.lender.example.",
  );
});

// --- The sender picker ---------------------------------------------------------------------------

const sendersQuery = "dataset=senders&level=3&sort=message_count,desc&page=1";
const picker: Record<string, Answer | readonly Answer[]> = {
  [accountsPath()]: ok("accounts.json"),
  [systemPath("personal")]: ok("system.json"),
  [lensPath("personal", "dataset=senders&level=0&sort=message_count,desc")]:
    ok("senders-summary.json"),
  [lensPath("personal", sendersQuery)]: ok("senders-rows.json"),
};

test("the picker selects senders by box and by key, keeps the selection in the address, and refuses a restricted sender", async () => {
  const root = await open("/personal/policy/pick", picker);
  expect(location.search).toBe("?level=3&sort=message_count,desc&page=1");
  const boxes = () => [...root.querySelectorAll<HTMLInputElement>(".select-box input")];
  expect(boxes().map((b) => [b.getAttribute("aria-label"), b.disabled])).toEqual([
    ["Select lender.example", true],
    ["Select newsletter.example", false],
    [`Select ${marker("senderdomain")}`, false],
    ["Select bank.example", true],
    ["Select mail.newsletter.example", false],
  ]);
  expect(
    document.getElementById(boxes()[3]?.getAttribute("aria-describedby") ?? "")?.textContent,
  ).toBe("Already restricted by base.bank");
  expect(root.querySelector(".selection-bar")).toBeNull();
  // j moves the cursor to the first row, a restricted one, which x cannot select.
  await press("j");
  await press("x");
  expect(location.search).not.toContain("pick=");
  await press("j");
  await press("x");
  expect(location.search).toBe("?level=3&sort=message_count,desc&page=1&pick=newsletter.example");
  expect(root.querySelector(".selection-bar [aria-hidden]")?.textContent).toBe(
    "1 sender · 3 messages selected",
  );
  expect(root.querySelector(".selection-bar .visually-hidden")?.textContent).toBe(
    "1 sender, 3 messages selected",
  );
  // Shift with j extends the selection as the cursor moves.
  await press("J", { shiftKey: true });
  expect(new URLSearchParams(location.search).getAll("pick")).toEqual([
    "newsletter.example",
    marker("senderdomain"),
  ]);
  // Enter toggles the row under the cursor here, and never leaves the picker.
  await press("Enter");
  expect(location.pathname).toBe("/personal/policy/pick");
  expect(new URLSearchParams(location.search).getAll("pick")).toEqual(["newsletter.example"]);
  await click(boxes()[4]);
  expect(new URLSearchParams(location.search).getAll("pick")).toEqual([
    "newsletter.example",
    "mail.newsletter.example",
  ]);
  // Escape clears the selection before it closes anything.
  await press("Escape");
  expect(location.search).toBe("?level=3&sort=message_count,desc&page=1");
  expect(location.pathname).toBe("/personal/policy/pick");
});

test("Ctrl or Cmd with a selects every match across pages, and outside a field only", async () => {
  const root = await open("/personal/policy/pick", picker);
  const search = root.querySelector<HTMLInputElement>('form[role="search"] input');
  await press("a", { ctrlKey: true }, search ?? document);
  expect(location.search).not.toContain("pick=");
  await press("a", { metaKey: true });
  expect(location.search).toBe("?level=3&sort=message_count,desc&page=1&pick=all");
  expect(root.querySelector(".selection-bar [aria-hidden]")?.textContent).toBe(
    "all 5 matching senders",
  );
  expect(root.querySelector(".selection-bar .visually-hidden")?.textContent).toBe(
    "Selected all 5 matching senders",
  );
  expect(
    [...root.querySelectorAll<HTMLInputElement>(".select-box input")].map((b) => b.checked),
  ).toEqual([false, true, true, false, true]);
});

test("Restrict as one rule hands the selection to Add a rule, whose Cancel returns to the picker as it was", async () => {
  const root = await open(
    "/personal/policy/pick?level=3&sort=message_count,desc&page=1&pick=newsletter.example",
    {
      ...picker,
      ...list,
      [matchPath("personal", ["newsletter.example"])]: ok("match-newsletter.json"),
    },
  );
  const pickerAt = location.pathname + location.search;
  await press("r");
  expect(location.pathname + location.search).toBe(
    "/personal/policy/new?suffix=newsletter.example",
  );
  expect(document.querySelector('aside[role="dialog"]')?.getAttribute("aria-label")).toBe(
    "Add a rule",
  );
  await act(async () => {
    const back = new Promise((resolve) => addEventListener("popstate", resolve, { once: true }));
    control(document.querySelector('aside[role="dialog"]') ?? root, "Close")?.click();
    await back;
  });
  await settle();
  expect(location.pathname + location.search).toBe(pickerAt);
  expect(root.querySelector(".selection-bar [aria-hidden]")?.textContent).toBe(
    "1 sender · 3 messages selected",
  );
});

test("a search narrows the senders, announces its count, and every match is handed over as the search", async () => {
  const root = await open("/personal/policy/pick", {
    ...picker,
    ...list,
    [lensPath("personal", "dataset=senders&level=0&sort=message_count,desc&search=newsletter")]: ok(
      "senders-summary-search.json",
    ),
    [lensPath("personal", `${sendersQuery}&search=newsletter`)]: ok("senders-rows-search.json"),
    [matchPath("personal", ["newsletter.example", "mail.newsletter.example"])]: ok(
      "match-newsletter-both.json",
    ),
  });
  await type(root.querySelector('form[role="search"] input'), "newsletter");
  await act(() => root.querySelector<HTMLFormElement>('form[role="search"]')?.requestSubmit());
  await settle();
  expect(location.search).toBe("?level=3&sort=message_count,desc&page=1&search=newsletter");
  expect(root.querySelector('.picker > p[role="status"]')?.textContent).toBe(
    "2 senders match 'newsletter'",
  );
  await press("a", { ctrlKey: true });
  await click(control(root, "Restrict as one rule"));
  expect(location.pathname + location.search).toBe("/personal/policy/new?search=newsletter");
  const panel = document.querySelector('aside[role="dialog"]');
  expect(
    [...(panel?.querySelectorAll<HTMLInputElement>(".suffix-line input") ?? [])].map(
      (i) => i.value,
    ),
  ).toEqual(["newsletter.example", "mail.newsletter.example", ""]);
  // The two suffixes are counted together, the sender both match once.
  expect(panel?.textContent).toContain(
    "Restricts 2 senders and 4 stored messages in personal. From each process's next policy reload their bodies are denied.",
  );
});

// --- History -------------------------------------------------------------------------------------

test("the history lists the base policy's changes and the account's, a lifted rule's row unlinked and carrying Restore", async () => {
  const root = await open("/personal/policy/history", {
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [lensPath("personal", "dataset=policy_changes&level=0&range=30d&sort=ts,desc")]:
      ok("changes-summary.json"),
    [lensPath("personal", "dataset=policy_changes&level=3&range=30d&sort=ts,desc&page=1")]:
      ok("changes-rows.json"),
  });
  expect(location.search).toBe("?level=3&range=30d&sort=ts,desc&page=1");
  const rows = [...root.querySelectorAll("tbody tr")];
  expect(rows.map((tr) => [...tr.querySelectorAll("td")].map((td) => td.textContent))).toEqual([
    ["2026-09-09 22:16Z", "operator", "lifted", "base.old", "base", "−old.example", "Restore"],
    [
      "2026-09-09 10:16Z",
      "operator",
      "edited",
      "operator.lender.example",
      "personal",
      "−old.lender.example",
      "Restore",
    ],
    [
      "2026-09-08 10:16Z",
      "operator",
      "lifted",
      "operator.gone.example",
      "personal",
      "−gone.example",
      "Restore",
    ],
    ["2026-09-07 10:16Z", "operator", "added", "base.bank", "base", "bank.example", ""],
    [
      "2026-09-07 10:16Z",
      "operator",
      "added",
      "operator.lender.example",
      "personal",
      "lender.example",
      "",
    ],
  ]);
  expect(
    rows.map((tr) => tr.querySelector("td:nth-child(4) a")?.getAttribute("href") ?? null),
  ).toEqual([
    null,
    "/personal/policy/operator.lender.example",
    null,
    "/personal/policy/base/base.bank",
    "/personal/policy/operator.lender.example",
  ]);
  expect(rows.map((tr) => control(tr, "Restore")?.getAttribute("href") ?? null)).toEqual([
    "/personal/policy/new?suffix=old.example&scope=base&id=base.old",
    "/personal/policy/operator.lender.example?suffix=old.lender.example",
    "/personal/policy/new?suffix=gone.example&scope=account&id=operator.gone.example",
    null,
    null,
  ]);
});

test("Restore of a lifted rule opens Add a rule filled with its suffixes, its scope and its identifier", async () => {
  await open("/personal/policy/new?suffix=old.example&scope=base&id=base.old", {
    ...list,
    [matchPath("personal", ["old.example"])]: ok("match-old.json"),
  });
  const panel = document.querySelector('aside[role="dialog"]');
  expect(panel?.querySelector<HTMLInputElement>('label input[type="text"].mono')?.value).toBe(
    "base.old",
  );
  expect(
    panel?.querySelector<HTMLInputElement>('input[type="radio"]:checked')?.closest("label")
      ?.textContent,
  ).toBe(" Every account (the base policy)");
  expect(panel?.querySelector<HTMLInputElement>(".suffix-line input")?.value).toBe("old.example");
});

// --- Import --------------------------------------------------------------------------------------

const importing: Record<string, Answer | readonly Answer[]> = {
  ...list,
};

async function choose(root: Element, file: string): Promise<void> {
  const input = root.querySelector<HTMLInputElement>('input[type="file"]');
  if (input === null) {
    throw new Error("no file input");
  }
  Object.defineProperty(input, "files", {
    configurable: true,
    value: [new File([file], "policy.yaml", { type: "application/yaml" })],
  });
  await act(async () => {
    input.dispatchEvent(new Event("change", { bubbles: true }));
    await new Promise((resolve) => setTimeout(resolve, 0));
  });
  await settle();
}

test("an import that lifts asks one dialog naming every lift, and Put back adds them back", async () => {
  const root = await open("/personal/policy/import", {
    ...importing,
    "POST /api/personal/policy/import/preview": ok("preview-lifting.json"),
    "POST /api/personal/policy/import": ok("imported.json"),
    "POST /api/personal/policy/rules": ok("imported-put-back.json"),
    [figures]: [ok("rules-summary.json"), ok("rules-summary-imported.json")],
    [rules]: [ok("rules-rows.json"), ok("rules-rows-imported.json")],
  });
  expect(root.querySelector("main h1")?.textContent).toBe("Import personal's rules");
  expect(root.textContent).toContain(
    "The file replaces personal's own rules. The base rules are not touched.",
  );
  expect(root.querySelector('input[type="file"]')?.closest("label")?.textContent).toBe(
    "Choose a policy file ",
  );
  const file =
    "rules:\n  - id: operator.new.example\n    domain_suffix: [new.example]\n    class: restricted\n";
  await choose(root, file);
  expect(server?.posted).toEqual([{ path: "/api/personal/policy/import/preview", body: { file } }]);
  // The lifted group comes first, with the account's numbers.
  expect(text(root, ".preview h3")).toEqual(["Rules lifted (1)", "Rules added (1)"]);
  expect(root.querySelector(".preview-group.lifts li")?.textContent).toBe(
    "operator.lender.example, lender.exampleIn personal, 1 sender and 4 stored messages are restricted by this rule and by no other rule.",
  );
  await click(control(root, "Import, lifting 1 restriction"));
  const d = dialog();
  expect(document.getElementById(d?.getAttribute("aria-labelledby") ?? "")?.textContent).toBe(
    "This import lifts 1 restriction",
  );
  expect(d?.textContent).toContain(
    "In personal, 1 sender and 4 stored messages are restricted by what this import lifts and by no other rule.",
  );
  expect(document.activeElement?.textContent).toBe("Cancel");
  expect(d?.querySelector("input")).toBeNull();
  await click(control(d ?? root, "Import and lift 1"));
  expect(server?.posted[1]).toEqual({
    path: "/api/personal/policy/import",
    body: {
      file,
      computed_against:
        '[{"id":"operator.lender.example","class":"restricted","suffixes":["lender.example"]}]',
      confirmation: "lift 1",
    },
  });
  expect(location.pathname).toBe("/personal/policy");
  const status = root.querySelector('.policy > [role="status"]');
  expect(status?.textContent).toBe(
    "Imported: 1 rule added, 0 rules edited, 1 restriction lifted. Put back the 1 lifted restriction",
  );
  await click(control(status ?? root, "Put back the 1 lifted restriction"));
  expect(server?.posted[2]).toEqual({
    path: "/api/personal/policy/rules",
    body: { scope: "account", rule_id: "operator.lender.example", suffixes: ["lender.example"] },
  });
});

test("an import that only adds applies at once, a file equal to the stored rules has nothing to import, and a refused file names its problem", async () => {
  const root = await open("/personal/policy/import", {
    ...importing,
    "POST /api/personal/policy/import/preview": [
      refusedAnswer("preview-refused.json"),
      ok("preview-equal.json"),
      ok("preview-adding.json"),
    ],
    "POST /api/personal/policy/import": ok("imported-adding.json"),
  });
  await choose(root, "rules: [not");
  expect(root.querySelector(".refusal[role='alert']")?.textContent).toBe(
    "The file is refused whole:Line 1: the file is not the policy file's form.",
  );
  await choose(root, "the stored rules");
  expect(root.textContent).toContain("The file matches the stored rules. Nothing to import.");
  expect(root.querySelector(".preview button")).toBeNull();
  await choose(root, "a file that adds");
  await click(control(root, "Import 1 change"));
  expect(dialog()).toBeNull();
  expect(server?.posted[3]).toEqual({
    path: "/api/personal/policy/import",
    body: {
      file: "a file that adds",
      computed_against:
        '[{"id":"operator.lender.example","class":"restricted","suffixes":["lender.example"]}]',
      confirmation: null,
    },
  });
  expect(root.querySelector('.policy > [role="status"]')?.textContent).toBe(
    "Imported: 1 rule added, 0 rules edited, 0 restrictions lifted.",
  );
});

test("an import against a policy changed since its preview is refused with the words to preview again", async () => {
  const root = await open("/personal/policy/import", {
    ...importing,
    "POST /api/personal/policy/import/preview": ok("preview-lifting.json"),
    "POST /api/personal/policy/import": refusedAnswer("error-stale-preview.json", 409),
  });
  await choose(root, "a file");
  await click(control(root, "Import, lifting 1 restriction"));
  await click(control(dialog() ?? root, "Import and lift 1"));
  expect(dialog()?.querySelector(".refusal[role='alert']")?.textContent).toBe(
    "The policy changed since this preview. Preview again.",
  );
  expect(location.pathname).toBe("/personal/policy/import");
});

// --- Inert rendering -----------------------------------------------------------------------------

// inert requires the marker's full text, angle brackets included, in every text the surface shows,
// no script element built from it, and each element of the surface one the component itself makes.
function inert(root: Element, mark: string, tags: readonly string[]): void {
  expect(root.querySelectorAll("script").length).toBe(0);
  expect(root.textContent).toContain(mark);
  for (const el of root.querySelectorAll("*")) {
    expect(tags).toContain(el.tagName);
  }
}

function inTable<Row>(
  rows: readonly Row[],
  columns: Parameters<typeof RowsTable<Row>>[0]["columns"],
) {
  return mount(
    <LocationProvider>
      <RowsTable
        caption="rows"
        rowsName="rows"
        columns={columns}
        rows={rows}
        page={1}
        pages={1}
        pageHref={() => "/"}
        keys={false}
      />
    </LocationProvider>,
  );
}

async function recording<T>(file: string): Promise<T> {
  const value: T = await Bun.file(new URL(`./fixtures/${file}`, import.meta.url)).json();
  return value;
}

test("a sender domain carrying markup arrives as text in the picker's row, its box's label and its Restrict control", async () => {
  const root = await open("/personal/policy/pick", picker);
  const mark = marker("senderdomain");
  const row = [...root.querySelectorAll("tbody tr")].find((tr) => tr.textContent?.includes(mark));
  if (row === undefined) {
    throw new Error("no marked row");
  }
  inert(row, mark, ["TD", "INPUT", "SPAN", "A"]);
  const cells = [...row.querySelectorAll("td")];
  expect(cells[1]?.textContent).toBe(mark);
  expect(cells[1]?.getAttribute("title")).toBe(mark);
  expect(cells[1]?.firstElementChild?.childElementCount).toBe(0);
  expect(row.querySelector("input")?.getAttribute("aria-label")).toBe(`Select ${mark}`);
  const restrict = control(row, `Restrict ${mark}…`);
  expect(restrict?.childElementCount).toBe(0);
  expect(new URLSearchParams(restrict?.getAttribute("href")?.split("?")[1]).get("suffix")).toBe(
    mark,
  );
});

test("markup in a rule's identifier and suffixes arrives as text in its row, and nothing is built from it", async () => {
  const mark = marker("ruleid");
  const page: { rows: RuleRow[] } = await recording("rules-rows.json");
  const rows = page.rows.map((r) => ({
    ...r,
    rule_id: mark,
    suffixes: [mark, mark],
    source: mark,
    created_by: mark,
  }));
  const table = inTable(rows, ruleColumns("personal"));
  try {
    for (const tr of table.root.querySelectorAll("tbody tr")) {
      inert(tr, mark, ["TD", "A", "SPAN"]);
      expect(tr.querySelector("td a")?.textContent).toBe(mark);
      expect(tr.querySelector("td a")?.childElementCount).toBe(0);
      expect([...tr.querySelectorAll(".suffix-chip")].map((s) => s.textContent)).toEqual([
        mark,
        mark,
      ]);
    }
  } finally {
    table.unmount();
  }
});

test("markup in a change's identifier, actor and suffixes arrives as text in its history row", async () => {
  const mark = marker("changeid");
  const page: { rows: ChangeRow[] } = await recording("changes-rows.json");
  const rows = page.rows.map((r) => ({
    ...r,
    rule_id: mark,
    actor: mark,
    suffixes_before: r.suffixes_before.map(() => mark),
    suffixes_after: r.suffixes_after.length === 0 ? [] : [mark, `${mark}2`],
  }));
  const table = inTable(rows, changeColumns("personal"));
  try {
    for (const tr of table.root.querySelectorAll("tbody tr")) {
      inert(tr, mark, ["TD", "A", "SPAN"]);
      expect(tr.querySelector("td:nth-child(4)")?.textContent).toBe(mark);
      expect(tr.querySelector("td:nth-child(4)")?.firstElementChild?.childElementCount).toBe(0);
      expect(tr.querySelector("td:nth-child(2)")?.textContent).toBe(mark);
    }
  } finally {
    table.unmount();
  }
});

test("markup in a sender's domain arrives as text in the sender row", async () => {
  const mark = marker("senderrow");
  const page: { rows: SenderRow[] } = await recording("senders-rows.json");
  const rows = page.rows.map((r) => ({
    ...r,
    domain: mark,
    restricted_by: r.restricted_by === null ? null : mark,
  }));
  const table = inTable(rows, senderColumns("personal"));
  try {
    for (const tr of table.root.querySelectorAll("tbody tr")) {
      inert(tr, mark, ["TD", "A", "SPAN"]);
      expect(tr.querySelector("td")?.textContent).toBe(mark);
    }
  } finally {
    table.unmount();
  }
});

// withValues answers the paths it names with the values given, which a test derives from a recording by
// putting marker text in its message-derived fields, and every other path from the recordings.
function withValues(base: Recorded, values: Readonly<Record<string, unknown>>): Recorded {
  const answer = (key: string) =>
    new Response(JSON.stringify(values[key]), {
      status: 200,
      headers: { "Content-Type": "application/json" },
    });
  return {
    ...base,
    fetch: async (path, signal) => (path in values ? answer(path) : base.fetch(path, signal)),
    post: async (path, body) => {
      const key = `POST ${path}`;
      if (key in values) {
        base.posted.push({ path, body });
        return answer(key);
      }
      return base.post(path, body);
    },
  };
}

async function openWith(
  path: string,
  answers: Readonly<Record<string, Answer | readonly Answer[]>>,
  values: Readonly<Record<string, unknown>>,
): Promise<HTMLElement> {
  server = recorded(answers);
  at(path);
  mounted = mount(<App deps={testDeps(withValues(server, values), [], undefined, timers)} />);
  await settle();
  return mounted.root;
}

test("markup in a rule's detail arrives as text in its panel and its lift dialog", async () => {
  const mark = marker("detail");
  const detail: RuleDetail = await recording("rule-lender-two.json");
  const marked: RuleDetail = {
    ...detail,
    rule_id: mark,
    source: mark,
    created_by: mark,
    suffixes: [mark, "lendermail.example"],
    suffix_details: detail.suffix_details.map((d, i) => (i === 0 ? { ...d, suffix: mark } : d)),
    matched: detail.matched.map((m) => ({ ...m, domain: mark })),
    history: detail.history.map((h) => ({ ...h, rule_id: mark, actor: mark })),
  };
  await openWith("/personal/policy/operator.lender.example", list, { [lender]: marked });
  const panel = document.querySelector('aside[role="dialog"]');
  if (panel === null) {
    throw new Error("no panel");
  }
  expect(panel.querySelectorAll("script").length).toBe(0);
  const dd = panel.querySelector(".provenance dd");
  expect(dd?.textContent).toBe(mark);
  expect(dd?.childElementCount).toBe(0);
  for (const selector of [
    ".suffix-details li .mono",
    ".matched li .mono",
    "tbody td:nth-child(4) a",
    "tbody td:nth-child(2) span",
  ]) {
    const el = panel.querySelector(selector);
    expect(el?.textContent).toBe(mark);
    expect(el?.childElementCount).toBe(0);
  }
  await click(control(panel, "Lift restriction"));
  const d = dialog();
  const title = document.getElementById(d?.getAttribute("aria-labelledby") ?? "");
  expect(title?.textContent).toBe(`Lift the restriction on ${mark}`);
  expect(title?.childElementCount).toBe(0);
  expect(control(d ?? panel, `Lift restriction on ${mark}`)?.childElementCount).toBe(0);
  expect(document.querySelectorAll("script").length).toBe(0);
});

test("markup in a file's identifiers and suffixes arrives as text in the preview and the import dialog", async () => {
  const mark = marker("fileid");
  const preview: ImportPreview = await recording("preview-lifting.json");
  const marked: ImportPreview = {
    ...preview,
    added: preview.added.map((r) => ({ ...r, rule_id: mark, suffixes: [mark] })),
    lifted: preview.lifted.map((r) => ({ ...r, rule_id: mark, suffixes: [mark] })),
    lifts: 2,
    edited: [
      {
        rule_id: mark,
        before: [mark],
        after: [mark],
        added: [mark],
        removed: [{ suffix: mark, released: { senders: 0, messages: 0 } }],
      },
    ],
  };
  const root = await openWith("/personal/policy/import", list, {
    "POST /api/personal/policy/import/preview": marked,
  });
  await choose(root, "a file");
  const section = root.querySelector(".preview");
  if (section === null) {
    throw new Error("no preview");
  }
  inert(section, mark, ["SECTION", "H3", "UL", "LI", "SPAN", "P", "BUTTON"]);
  const names = [...section.querySelectorAll("li .mono")];
  expect(names.length).toBe(6);
  for (const name of names) {
    expect(name.textContent).toBe(mark);
    expect(name.childElementCount).toBe(0);
  }
  await click(control(section, "Import, lifting 2 restrictions"));
  const d = dialog();
  const listed = [...(d?.querySelectorAll("li .mono") ?? [])];
  expect(listed.length).toBe(3);
  for (const name of listed) {
    expect(name.textContent).toBe(mark);
    expect(name.childElementCount).toBe(0);
  }
  expect(document.querySelectorAll("script").length).toBe(0);
});

test("g then o goes to the account's policy", async () => {
  await open("/personal/policy/pick", { ...picker, ...list });
  await press("g");
  await press("o");
  expect(location.pathname).toBe("/personal/policy");
  expect(document.querySelector("main h1")?.textContent).toBe("Policy");
});
