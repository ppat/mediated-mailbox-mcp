// The global chrome and the entry route, rendered by the router over recorded answers.
import { afterEach, expect, test } from "bun:test";
import { act } from "preact/test-utils";
import { accountsPath, systemPath, type System } from "../src/app/api.ts";
import { bannerText, partialIndex, unknownIndex } from "../src/app/frame.tsx";
import { App } from "../src/app/router.tsx";
import { homeAnswers, testDeps, type Connection } from "./app.ts";
import { ok, recorded, type Recorded } from "./fixtures/fetch.ts";
import { at, mount, settle, type Mounted } from "./render.ts";

let mounted: Mounted | undefined;
let server: Recorded | undefined;
afterEach(() => {
  mounted?.unmount();
  mounted = undefined;
  expect(server?.missing ?? []).toEqual([]);
  server = undefined;
  delete document.documentElement.dataset["theme"];
});

function serve(): Recorded {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [systemPath("other")]: ok("system-other.json"),
    ...homeAnswers("personal"),
    ...homeAnswers("other"),
  });
  return server;
}

let connections: Connection[] = [];

async function open(path: string): Promise<HTMLElement> {
  const s = serve();
  at(path);
  connections = [];
  mounted = mount(<App deps={testDeps(s, connections)} />);
  await settle();
  return mounted.root;
}

function press(key: string): Promise<void> {
  return act(() => {
    document.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

test("the entry route goes to the first account by identifier", async () => {
  await open("/");
  expect(location.pathname).toBe("/other");
});

test("the entry route goes to the account last used here", async () => {
  await open("/personal");
  mounted?.unmount();
  const s = serve();
  at("/");
  const deps = testDeps(s);
  deps.storage?.setItem("mediated-mailbox.account", "personal");
  mounted = mount(<App deps={deps} />);
  await settle();
  expect(location.pathname).toBe("/personal");
});

test("the top bar shows the account with its provider, and its menu lists every account", async () => {
  const root = await open("/personal");
  const selector = root.querySelector<HTMLButtonElement>('button[aria-label="Account"]');
  expect(selector?.textContent).toBe("personal · gmail");
  expect(selector?.getAttribute("aria-expanded")).toBe("false");
  await act(() => selector?.click());
  expect(selector?.getAttribute("aria-expanded")).toBe("true");
  const items = [...root.querySelectorAll('[role="menu"] [role="menuitem"]')];
  expect(items.map((i) => [i.textContent, i.getAttribute("href")])).toEqual([
    ["othergmail", "/other"],
    ["personalgmail", "/personal"],
    ["Account settings", "/personal/account"],
    ["Connect an account", "/setup/connect"],
    ["Installation", "/setup"],
  ]);
  expect(document.activeElement).toBe(items[0] ?? null);
  await act(() => {
    items[0]?.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  });
  await settle();
  expect(root.querySelector('[role="menu"]')).toBeNull();
  expect(document.activeElement).toBe(selector);
});

test("the navigation lists only the screens that exist and marks the current one", async () => {
  const root = await open("/personal");
  const links = [...root.querySelectorAll('nav[aria-label="Screens"] a')];
  expect(
    links.map((a) => [a.textContent, a.getAttribute("href"), a.getAttribute("aria-current")]),
  ).toEqual([
    ["Home", "/personal", "page"],
    ["Jobs running", "/personal/jobs", null],
    ["System", "/personal/system", null],
  ]);
});

test("Jobs carries a labeled running mark while the decisions block counts a running workload", async () => {
  const root = await open("/personal");
  const mark = root.querySelector('nav[aria-label="Screens"] .running-mark');
  expect(mark?.textContent).toBe(" running");
});

test("the address line shows the view's whole URL", async () => {
  const root = await open("/personal?x=1");
  const address = root.querySelector<HTMLInputElement>('input[aria-label="Address of this view"]');
  expect(address?.value).toBe("https://ui.mediated-mailbox.test/personal?x=1");
  expect(address?.readOnly).toBe(true);
});

test("the theme override is set on the root element and kept in this browser", async () => {
  const root = await open("/personal");
  await act(() => root.querySelector<HTMLButtonElement>('button[aria-label="Settings"]')?.click());
  const radios = [...root.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]')];
  expect(radios.map((r) => [r.textContent, r.getAttribute("aria-checked")])).toEqual([
    ["Theme system", "true"],
    ["Theme dark", "false"],
    ["Theme light", "false"],
  ]);
  await act(() => radios[2]?.click());
  expect(document.documentElement.dataset["theme"]).toBe("light");
  expect(localStorage.getItem("mediated-mailbox.theme")).toBe("light");
});

test("the partial-index banner names backfill pass 2 and its pending count", async () => {
  const root = await open("/personal");
  const banner = root.querySelector('[role="status"].banner');
  expect(banner?.textContent).toBe(
    "Backfill pass 2 is running with 1 message pending scan, and a pending message denies its body until it is scanned. See Jobs",
  );
  expect(banner?.querySelector("a")?.getAttribute("href")).toBe("/personal/jobs");
});

test("the banner names pass 1 with its page and share, and both passes in one banner", async () => {
  const answer: System = JSON.parse(
    await Bun.file(new URL("fixtures/system.json", import.meta.url)).text(),
  );
  const pass2 = answer.operational.backfill_pass2_run;
  if (pass2 === null) {
    throw new Error("the recording holds no pass 2 run");
  }
  // A run's checkpoint is free-form JSON in the contract, so it is written as JSON.
  const checkpoint = JSON.parse('{"page": 1684, "of": 3368}');
  const both: System = {
    ...answer,
    operational: {
      ...answer.operational,
      backfill_pass1_complete: false,
      backfill_pass1_run: { ...pass2, pass: "pass1", checkpoint },
      backfill_pass1_succeeded_at: null,
    },
  };
  expect(partialIndex(both)).toBe(
    "Backfill pass 1 is running, page 1,684 of 3,368 (50.0%), so every count here is a count so far. " +
      "Backfill pass 2 is running with 1 message pending scan, and a pending message denies its body until it is scanned.",
  );
  const neither: System = {
    ...answer,
    operational: { ...answer.operational, backfill_pass2_complete: true },
  };
  expect(partialIndex(neither)).toBeUndefined();
});

test("while a re-opened backfill pass 1 runs, the banner says the index is whole", async () => {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system-reopened.json"),
    [systemPath("other")]: ok("system-other.json"),
    ...homeAnswers("personal"),
    ...homeAnswers("other"),
  });
  at("/personal");
  mounted = mount(<App deps={testDeps(server)} />);
  await settle();
  const banner = mounted.root.querySelector('[role="status"].banner');
  expect(banner?.textContent).toBe(
    "Backfill pass 1 is running again, page 842 of 3,368 (25.0%), to mask every subject again under the scanner now in force. " +
      "It last completed at 2026-09-02 10:16Z, so the index holds the whole mailbox and no count here is a count so far. " +
      "A subject it has not reached yet keeps its earlier masks. See Jobs",
  );
  expect(banner?.querySelector("a")?.getAttribute("href")).toBe("/personal/jobs");
});

test("a re-opened pass 1 without a checkpoint page leaves out the page clause", async () => {
  const reopened: System = await recording("system-reopened.json");
  const run = reopened.operational.backfill_pass1_run;
  if (run === null) {
    throw new Error("the recording holds no pass 1 run");
  }
  const started: System = {
    ...reopened,
    operational: { ...reopened.operational, backfill_pass1_run: { ...run, checkpoint: null } },
  };
  expect(partialIndex(started)).toBe(
    "Backfill pass 1 is running again, to mask every subject again under the scanner now in force. " +
      "It last completed at 2026-09-02 10:16Z, so the index holds the whole mailbox and no count here is a count so far. " +
      "A subject it has not reached yet keeps its earlier masks.",
  );
});

test("an account the server does not list is named once in the body", async () => {
  const root = await open("/nobody");
  expect(root.querySelector("main [role=alert]")?.textContent).toBe(
    "This request was refusedNo account nobody is served here.",
  );
});

test("? shows the keyboard map, Escape closes it, and g then h goes home", async () => {
  const root = await open("/personal/nothing");
  expect(root.textContent).toContain("No screen is at this address.");
  await press("?");
  const dialog = root.querySelector('[role="dialog"]');
  expect(dialog?.querySelector("h2")?.textContent).toBe("Keyboard map");
  await act(() => {
    dialog?.dispatchEvent(new KeyboardEvent("keydown", { key: "Escape", bubbles: true }));
  });
  expect(root.querySelector('[role="dialog"]')).toBeNull();
  await press("g");
  await press("h");
  await settle();
  expect(location.pathname).toBe("/personal");
});

// recording reads a recorded answer, which the contract types.
async function recording(file: string) {
  return JSON.parse(await Bun.file(new URL(`fixtures/${file}`, import.meta.url)).text());
}

test("the banner reads the system block again on each backfill run event and each poll", async () => {
  await open("/personal");
  const systemReads = () => server?.calls.filter((c) => c === systemPath("personal")).length;
  expect(connections.map((c) => [c.account, c.open])).toEqual([["personal", true]]);
  const connection = connections[0];
  if (connection === undefined) {
    throw new Error("the banner opened no stream");
  }
  const before = systemReads() ?? 0;
  const system: System = await recording("system.json");
  const jobs = await recording("jobs.json");
  await act(() => {
    connection.objects.handle(
      "run",
      JSON.stringify({ ...jobs.sync.last_tick, account: "personal" }),
      1,
    );
  });
  await settle();
  expect(systemReads()).toBe(before);
  await act(() => {
    const run = { ...system.operational.backfill_pass2_run, account: "personal" };
    connection.objects.handle("run", JSON.stringify(run), 2);
  });
  await settle();
  expect(systemReads()).toBe(before + 1);
  await act(() => connection.refetch());
  await settle();
  expect(systemReads()).toBe(before + 2);
  mounted?.unmount();
  mounted = undefined;
  expect(connection.open).toBe(false);
});

test("a failed system read keeps the banner, saying the backfill state is unknown", async () => {
  const refused = await recording("error-unknown-account.json");
  const failure = { ...refused.error, status: 400 };
  expect(bannerText({ status: "error", failure })).toBe(unknownIndex);
  expect(bannerText({ status: "loading", attempt: 0 })).toBeUndefined();
  const system: System = await recording("system.json");
  expect(bannerText({ status: "ok", answer: system, at: 0 })).toBe(partialIndex(system));
});

test("an account the listing does not name gets no banner and opens no stream", async () => {
  const root = await open("/nobody");
  expect(root.querySelector(".banner")).toBeNull();
  expect(connections).toEqual([]);
  expect(server?.calls).not.toContain(systemPath("nobody"));
});

test("the account in view is kept as the one last used in this browser", async () => {
  await open("/personal");
  expect(localStorage.getItem("mediated-mailbox.account")).toBe("personal");
});

test("choosing system clears the override from the root element", async () => {
  const root = await open("/personal");
  const choose = async (index: number) => {
    await act(() =>
      root.querySelector<HTMLButtonElement>('button[aria-label="Settings"]')?.click(),
    );
    await act(() =>
      root.querySelectorAll<HTMLButtonElement>('[role="menuitemradio"]')[index]?.click(),
    );
  };
  await choose(1);
  expect(document.documentElement.dataset["theme"]).toBe("dark");
  await choose(0);
  expect(document.documentElement.hasAttribute("data-theme")).toBe(false);
  expect(localStorage.getItem("mediated-mailbox.theme")).toBe("system");
});

test("a press outside an open menu closes it", async () => {
  const root = await open("/personal");
  await act(() => root.querySelector<HTMLButtonElement>('button[aria-label="Account"]')?.click());
  expect(root.querySelector('[role="menu"]')).not.toBeNull();
  await act(() => {
    document.body.dispatchEvent(new MouseEvent("mousedown", { bubbles: true }));
  });
  expect(root.querySelector('[role="menu"]')).toBeNull();
});
