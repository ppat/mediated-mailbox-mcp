// The jobs screen of docs/UI.md section 8.3, rendered by the router over answers recorded from the real
// server, and its live refresh through the stream's handler. The applying plan's description carries
// markup marker text, which the reorg apply card and the runs table show as its title, and the
// inert-rendering test here checks both in the form ADR-0064 requires. Its mutation demonstrations are
// under ui/browser/testdata/mutations.
import { afterEach, expect, test } from "bun:test";
import { options, type VNode } from "preact";
import { act } from "preact/test-utils";
import {
  accountsPath,
  jobsPath,
  lensPath,
  runPath,
  systemPath,
  type Jobs,
} from "../src/app/api.ts";
import { LiveProgress, LiveText } from "../src/app/live.tsx";
import { App } from "../src/app/router.tsx";
import { RowsTable } from "../src/lens/table.tsx";
import { JobsScreen, progressOf, runMeasure } from "../src/screens/jobs.tsx";
import { testDeps, type Connection } from "./app.ts";
import { ok, recorded, type Answer, type Recorded } from "./fixtures/fetch.ts";
import { at, mount, settle, type Mounted } from "./render.ts";

let mounted: Mounted | undefined;
let server: Recorded | undefined;
let connections: Connection[] = [];
afterEach(() => {
  mounted?.unmount();
  mounted = undefined;
  expect(server?.missing ?? []).toEqual([]);
  server = undefined;
});

const runs = "dataset=runs";
const canonical = "level=3&range=7d&sort=started_at,desc&page=1&pass=!tick";

async function open(path: string): Promise<HTMLElement> {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [jobsPath("personal")]: ok("jobs.json"),
    [lensPath("personal", `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick`)]:
      ok("runs-summary.json"),
    [lensPath("personal", `${runs}&${canonical}`)]: ok("runs-rows.json"),
  });
  at(path);
  connections = [];
  mounted = mount(<App deps={testDeps(server, connections)} />);
  await settle();
  return mounted.root;
}

async function recording<T>(file: string): Promise<T> {
  return JSON.parse(await Bun.file(new URL(`fixtures/${file}`, import.meta.url)).text());
}

function press(key: string): Promise<void> {
  return act(() => {
    document.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

// fields are a card's fields, name and text.
function fields(root: HTMLElement, card: string): [string, string][] {
  const dl = root.querySelector(`section.card[aria-label="${card}"] dl`);
  const dts = [...(dl?.querySelectorAll("dt") ?? [])];
  return dts.map((dt) => [dt.textContent ?? "", dt.nextElementSibling?.textContent ?? ""]);
}

// deliver hands an event to every open connection, as the account's one stream would deliver it to
// every reader in the tab.
async function deliver(name: string, data: object): Promise<void> {
  await act(() => {
    for (const connection of connections.filter((c) => c.open)) {
      connection.objects.handle(name, JSON.stringify(data), 1);
    }
  });
  await settle();
}

test("a URL missing parameters is replaced by the runs view's canonical form", async () => {
  await open("/personal/jobs");
  expect(location.pathname + location.search).toBe(`/personal/jobs?${canonical}`);
});

test("the cards show each workload's state and fields, with the decisions block's counts", async () => {
  const root = await open(`/personal/jobs?${canonical}`);
  expect([...root.querySelectorAll("section.card h2")].map((h) => h.textContent)).toEqual([
    "Backfill running",
    "Delta sync idle",
    "Reorg apply running",
    "Heuristics idle",
    "Rate budget",
  ]);
  expect(fields(root, "Backfill")).toEqual([
    [
      "pass 1",
      "complete · succeeded, 84,212 messages, 3,368 pages, 1d, completed 2026-09-02 10:16Z",
    ],
    ["pass 2", "r-0913 running"],
    ["checkpoint", "page 3,065 of 3,368"],
    ["decided", "76,610 of 84,212, 7,602 pending, 70,100 scanned this run, 6,510 skipped"],
    ["started", "2026-09-10 09:36Z"],
    ["heartbeat", "1m ago"],
    ["estimated time left", "50m 30s"],
  ]);
  expect(fields(root, "Delta sync")).toEqual([
    ["cadence", "every 5m"],
    ["last tick", "2026-09-10 10:11Z, succeeded, 4m, 3 added, 1 modified, 0 removed"],
    ["cursor age", "4m"],
    [
      "gap recoveries, 7 days",
      "1, last 2026-09-08 00:00Z to 2026-09-08 06:00Z, 41 reconciled at 2026-09-08 10:16Z",
    ],
  ]);
  expect(fields(root, "Reorg apply")).toEqual([
    ["applying", "<script>mmfieldmarker-applyingplan</script>, 1 of 2 operations"],
    [
      "last run",
      "<script>mmfieldmarker-appliedplan</script>, succeeded, 2 operations, 0 failures, 0s, 2026-08-31 10:16Z, rollback available over 2 logged operations",
    ],
  ]);
  expect(fields(root, "Heuristics")).toEqual([
    ["cadence", "every 1d"],
    ["last run", "2026-09-09 14:16Z, 0s, 2 candidates emitted"],
    ["awaiting review", "6 candidates"],
    ["next run", "2026-09-10 14:16Z"],
  ]);
  expect(root.querySelector('section[aria-label="Rate budget"] p')?.textContent).toBe(
    "3.1 of 5.0 units/s, cap 8.0 units/s, not in backoff, last throttle 2026-09-10 08:16Z",
  );
  expect([...root.querySelectorAll(".classes li")].map((li) => li.textContent)).toEqual([
    "interactive 0.4 of 1.5 units/s",
    "sync 0.8 of 1.0 units/s",
    "batch 1.9 of 2.5 units/s",
  ]);
});

test("the recent runs open without delta-sync ticks, each linking to its run", async () => {
  const root = await open(`/personal/jobs?${canonical}`);
  expect(root.querySelector(".breadcrumb .chip")?.textContent).toBe("pass not tick ×");
  const table = root.querySelector("table.rows");
  expect(table?.querySelector("caption")?.textContent).toBe("Recent runs, 5 in all");
  const rows = [...(table?.querySelectorAll("tbody tr") ?? [])].map((tr) =>
    [...tr.querySelectorAll("td")].map((td) => td.textContent),
  );
  expect(rows).toEqual([
    [
      "r-0913",
      "backfill · pass2",
      "2026-09-10 09:36Z",
      "40m so far",
      "running",
      "page 3,065 of 3,368",
      "0",
    ],
    [
      "r-0915",
      "apply · <script>mmfieldmarker-applyingplan</script>",
      "2026-09-10 07:16Z",
      "3h so far",
      "running",
      "1 of 2 operations",
      "0",
    ],
    ["r-0909", "heuristics", "2026-09-09 14:16Z", "0s", "succeeded", "", "0"],
    [
      "r-0911",
      "sync · gap_recovery",
      "2026-09-08 10:16Z",
      "0s",
      "succeeded",
      "2026-09-08 00:00Z to 2026-09-08 06:00Z, 41 reconciled",
      "0",
    ],
    ["r-0912", "backfill · pass2", "2026-09-08 08:16Z", "1h", "failed", "page 14 of 3,368", "5"],
  ]);
  const failures = table?.querySelector('a[href="/personal/jobs/r-0912"]:not(.mono)');
  expect(failures?.textContent).toBe("5");
  expect([...root.querySelectorAll(".strip .figure")].map((f) => f.textContent)).toEqual([
    "5runs",
    "2running",
    "1failed",
    "2026-09-08 09:16Zlast failure",
  ]);
});

test("the plans' titles carrying markup arrive as text in their cards and their row", async () => {
  const root = await open(`/personal/jobs?${canonical}`);
  const text = "<script>mmfieldmarker-applyingplan</script>";
  const [card, last] = [...root.querySelectorAll('section.card[aria-label="Reorg apply"] dd')];
  expect(card?.textContent).toBe(`${text}, 1 of 2 operations`);
  expect(card?.childElementCount).toBe(0);
  // The last run's plan title is the applied plan's, which carries its own marker.
  expect(last?.textContent).toStartWith("<script>mmfieldmarker-appliedplan</script>, succeeded");
  expect(last?.childElementCount).toBe(0);
  const cell = [...root.querySelectorAll("tbody td")].find((td) => td.textContent?.includes(text));
  expect(cell?.textContent).toBe(`apply · ${text}`);
  expect(cell?.getAttribute("title")).toBe(text);
  expect(cell?.childElementCount).toBe(0);
  expect(root.querySelectorAll("script").length).toBe(0);
});

// classFills are the rate budget's class bars' drawn widths, interactive, sync and batch.
function classFills(root: HTMLElement): (string | null | undefined)[] {
  return ["interactive", "sync", "batch"].map((name) => fill(root, `${name} used of reserved`));
}

// fill is a progress bar's drawn width, found by its label.
function fill(root: HTMLElement, label: string): string | null | undefined {
  return root
    .querySelector(`svg.progress[aria-label="${label}"] .progress-fill`)
    ?.getAttribute("width");
}

test("an event redraws a card's run and its row, the rate redraws the budget, and nothing re-runs", async () => {
  let screens = 0;
  let tables = 0;
  let texts = 0;
  let bars = 0;
  const previous: ((vnode: VNode) => void) | undefined = Object.getOwnPropertyDescriptor(
    options,
    "diffed",
  )?.value;
  options.diffed = (vnode: VNode) => {
    if (vnode.type === JobsScreen) {
      screens += 1;
    }
    if (vnode.type === RowsTable) {
      tables += 1;
    }
    if (vnode.type === LiveText) {
      texts += 1;
    }
    if (vnode.type === LiveProgress) {
      bars += 1;
    }
    previous?.(vnode);
  };
  try {
    const root = await open(`/personal/jobs?${canonical}`);
    expect(root.querySelector(".live")?.textContent).toBe("live · connecting");
    const [s, t, x, b] = [screens, tables, texts, bars];
    expect(x).toBeGreaterThan(10);
    expect(b).toBe(4);
    expect(fill(root, "Backfill pass 2 progress")).toBe(String((3065 / 3368) * 160));
    expect(classFills(root)).toEqual([
      String((0.4 / 1.5) * 160),
      String((0.8 / 1.0) * 160),
      String((1.9 / 2.5) * 160),
    ]);
    const jobs: Jobs = await recording("jobs.json");
    const run = jobs.backfill.pass2.run;
    if (run === null || jobs.rate === null) {
      throw new Error("the recording holds no pass 2 run or rate");
    }
    // A run's checkpoint is free-form JSON in the contract, so it is written as JSON.
    await deliver("run", {
      ...run,
      account: "personal",
      checkpoint: JSON.parse('{"page": 3100, "of": 3368}'),
    });
    expect(fields(root, "Backfill")[2]).toEqual(["checkpoint", "page 3,100 of 3,368"]);
    expect(fill(root, "Backfill pass 2 progress")).toBe(String((3100 / 3368) * 160));
    const row = root.querySelector("tbody tr");
    expect(row?.querySelectorAll("td")[5]?.textContent).toBe("page 3,100 of 3,368");
    await deliver("rate", {
      ...jobs.rate,
      account: "personal",
      current: 4.2,
      classes: {
        interactive: { used: 0.9, reserved: 1.5 },
        sync: { used: 0.3, reserved: 1.0 },
        batch: { used: 0.5, reserved: 2.5 },
      },
    });
    expect(classFills(root)).toEqual([
      String((0.9 / 1.5) * 160),
      String((0.3 / 1.0) * 160),
      String((0.5 / 2.5) * 160),
    ]);
    expect(root.querySelector('section[aria-label="Rate budget"] p')?.textContent).toStartWith(
      "4.2 of 5.0 units/s",
    );
    expect([screens, tables, texts, bars]).toEqual([s, t, x, b]);
  } finally {
    options.diffed = previous;
  }
});

test("a pass 2 that has just started shows its bar on its first page event", async () => {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [jobsPath("personal")]: ok("jobs-pass2-started.json"),
    [lensPath("personal", `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick`)]:
      ok("runs-summary.json"),
    [lensPath("personal", `${runs}&${canonical}`)]: ok("runs-rows.json"),
  });
  at(`/personal/jobs?${canonical}`);
  connections = [];
  mounted = mount(<App deps={testDeps(server, connections)} />);
  await settle();
  const root = mounted.root;
  const jobs: Jobs = await recording("jobs-pass2-started.json");
  const run = jobs.backfill.pass2.run;
  if (run === null) {
    throw new Error("the recording holds no pass 2 run");
  }
  expect(fill(root, "Backfill pass 2 progress")).toBe("0");
  await deliver("run", {
    ...run,
    account: "personal",
    checkpoint: JSON.parse('{"page": 1, "of": 3368}'),
  });
  expect(fields(root, "Backfill")[2]).toEqual(["checkpoint", "page 1 of 3,368"]);
  expect(fill(root, "Backfill pass 2 progress")).toBe(String((1 / 3368) * 160));
});

test("a run event for a run the screen does not show reads the cards and the table again", async () => {
  await open(`/personal/jobs?${canonical}`);
  const reads = () =>
    server?.calls.filter((c) => c === jobsPath("personal") || c.includes("dataset=runs")).length ??
    0;
  const before = reads();
  const jobs: Jobs = await recording("jobs.json");
  const tick = jobs.sync.last_tick;
  if (tick === null) {
    throw new Error("the recording holds no tick");
  }
  await deliver("run", { ...tick, account: "personal", run_id: "r-0916", state: "running" });
  expect(reads()).toBe(before + 3);
});

test("the runs at level 1 are bars and a table, and a click on a group descends to level 2", async () => {
  const l1 = "level=1&group=workload&range=7d&sort=started_at,desc&pass=!tick";
  const l2 = "level=2&group=state&range=7d&sort=started_at,desc&pass=!tick&workload=backfill";
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [jobsPath("personal")]: ok("jobs.json"),
    [lensPath("personal", `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick`)]:
      ok("runs-summary.json"),
    [lensPath("personal", `${runs}&${l1}`)]: ok("runs-by-workload.json"),
    [lensPath(
      "personal",
      `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick&workload=backfill`,
    )]: ok("runs-summary-backfill.json"),
    [lensPath("personal", `${runs}&${l2}`)]: ok("runs-backfill-by-state.json"),
  });
  at(`/personal/jobs?${l1}`);
  connections = [];
  mounted = mount(<App deps={testDeps(server, connections)} />);
  await settle();
  const root = mounted.root;
  const bars = () =>
    [...root.querySelectorAll("figure.bars .bar")].map((b) => [
      b.querySelector(".bar-label")?.textContent,
      b.querySelector(".bar-count")?.textContent,
      b.getAttribute("href"),
    ]);
  expect(bars()).toEqual([
    ["backfill", "2", `/personal/jobs?${l2}`],
    [
      "apply",
      "1",
      "/personal/jobs?level=2&group=state&range=7d&sort=started_at,desc&pass=!tick&workload=apply",
    ],
    [
      "heuristics",
      "1",
      "/personal/jobs?level=2&group=state&range=7d&sort=started_at,desc&pass=!tick&workload=heuristics",
    ],
    [
      "sync",
      "1",
      "/personal/jobs?level=2&group=state&range=7d&sort=started_at,desc&pass=!tick&workload=sync",
    ],
  ]);
  expect(root.querySelectorAll("figure.bars .bar-restricted").length).toBe(0);
  const groupTable = [...root.querySelectorAll("table.rows")].find((t) =>
    t.querySelector("caption")?.textContent?.includes("groups"),
  );
  expect([...(groupTable?.querySelectorAll("tbody tr") ?? [])].map((tr) => tr.textContent)).toEqual(
    ["backfill240.0%", "apply120.0%", "heuristics120.0%", "sync120.0%"],
  );
  expect(
    [...root.querySelectorAll(".groupby a")].map((a) => [
      a.textContent,
      a.getAttribute("aria-current"),
    ]),
  ).toEqual([
    ["workload", "true"],
    ["state", null],
    ["day", null],
  ]);
  await act(() => root.querySelector<HTMLAnchorElement>("figure.bars .bar")?.click());
  await settle();
  expect(location.search).toBe(`?${l2}`);
  expect(bars().map((b) => [b[0], b[1]])).toEqual([
    ["failed", "1"],
    ["running", "1"],
  ]);
  expect([...root.querySelectorAll(".breadcrumb .chip")].map((c) => c.textContent)).toEqual([
    "pass not tick ×",
    "workload backfill ×",
  ]);
});

// openAdvancing opens the jobs screen on answers that move on after the first read, as the recorded
// state does when a tick finishes, pass 2 moves and a heuristics run starts.
async function openAdvancing(extra: Record<string, Answer> = {}): Promise<HTMLElement> {
  server = recorded({
    ...extra,
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [jobsPath("personal")]: [ok("jobs.json"), ok("jobs-later.json")],
    [lensPath("personal", `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick`)]: [
      ok("runs-summary.json"),
      ok("runs-summary-later.json"),
    ],
    [lensPath("personal", `${runs}&${canonical}`)]: [
      ok("runs-rows.json"),
      ok("runs-rows-later.json"),
    ],
  });
  at(`/personal/jobs?${canonical}`);
  connections = [];
  mounted = mount(<App deps={testDeps(server, connections)} />);
  await settle();
  return mounted.root;
}

// rowsById are the runs table's rows by their run id, each row's cells.
function rowsById(root: HTMLElement): Map<string, (string | null)[]> {
  const rows = [...root.querySelectorAll("table.rows tbody tr")].map((tr) =>
    [...tr.querySelectorAll("td")].map((td) => td.textContent),
  );
  return new Map(rows.map((cells) => [cells[0] ?? "", cells]));
}

test("a new tick replaces the card's run, and its next event reaches the card", async () => {
  const root = await openAdvancing();
  const later: Jobs = await recording("jobs-later.json");
  const tick = later.sync.last_tick;
  if (tick === null) {
    throw new Error("the later recording holds no tick");
  }
  await deliver("run", { ...tick, account: "personal" });
  expect(fields(root, "Delta sync")[1]).toEqual([
    "last tick",
    "2026-09-10 10:14Z, succeeded, 30s, 5 added, 0 modified, 2 removed",
  ]);
  // A run's counters are free-form JSON in the contract, so they are written as JSON.
  await deliver("run", {
    ...tick,
    account: "personal",
    counters: JSON.parse('{"added": 7, "modified": 1, "removed": 2}'),
  });
  expect(fields(root, "Delta sync")[1]).toEqual([
    "last tick",
    "2026-09-10 10:14Z, succeeded, 30s, 7 added, 1 modified, 2 removed",
  ]);
});

test("a poll's re-read reaches the cards and the rows", async () => {
  const root = await openAdvancing();
  await act(() => {
    for (const connection of connections.filter((c) => c.open)) {
      connection.refetch();
    }
  });
  await settle();
  expect(fields(root, "Backfill")[2]).toEqual(["checkpoint", "page 3,300 of 3,368"]);
  expect(rowsById(root).get("r-0913")?.[5]).toBe("page 3,300 of 3,368");
});

test("a reconnect's re-read puts a new run first, and every row keeps its own run's cells", async () => {
  const root = await openAdvancing();
  const before = rowsById(root);
  await act(() => {
    for (const connection of connections.filter((c) => c.open)) {
      connection.refetch();
    }
  });
  await settle();
  const after = rowsById(root);
  expect([...after.keys()]).toEqual(["r-0917", "r-0913", "r-0915", "r-0909", "r-0911", "r-0912"]);
  expect(after.get("r-0917")?.slice(1, 7)).toEqual([
    "heuristics",
    "2026-09-10 10:15Z",
    "30s so far",
    "running",
    "",
    "0",
  ]);
  for (const id of ["r-0915", "r-0909", "r-0911", "r-0912"]) {
    expect(after.get(id)).toEqual(before.get(id));
  }
  expect(after.get("r-0913")?.slice(4, 6)).toEqual(["running", "page 3,300 of 3,368"]);
  expect(root.querySelector('section.card[aria-label="Heuristics"] h2')?.textContent).toBe(
    "Heuristics running",
  );
});

test("a shown run's change of state reads the cards again", async () => {
  await open(`/personal/jobs?${canonical}`);
  const reads = () => server?.calls.filter((c) => c === jobsPath("personal")).length ?? 0;
  const before = reads();
  const jobs: Jobs = await recording("jobs.json");
  const run = jobs.backfill.pass2.run;
  if (run === null) {
    throw new Error("the recording holds no pass 2 run");
  }
  await deliver("run", { ...run, account: "personal" });
  expect(reads()).toBe(before);
  await deliver("run", { ...run, account: "personal", state: "succeeded" });
  expect(reads()).toBe(before + 1);
});

test("times judged against now read the live clock, not the time the page was read", async () => {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [jobsPath("personal")]: ok("jobs.json"),
    [lensPath("personal", `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick`)]:
      ok("runs-summary.json"),
    [lensPath("personal", `${runs}&${canonical}`)]: ok("runs-rows.json"),
  });
  at(`/personal/jobs?${canonical}`);
  connections = [];
  // The page is read at 10:16:04Z, and the browser's clock reads an hour later.
  const later = Date.parse("2026-09-10T11:16:04Z");
  mounted = mount(<App deps={testDeps(server, connections, () => later)} />);
  await settle();
  const root = mounted.root;
  expect(rowsById(root).get("r-0913")?.[3]).toBe("1h 40m so far");
  const jobs: Jobs = await recording("jobs.json");
  if (jobs.rate === null) {
    throw new Error("the recording holds no rate");
  }
  await deliver("rate", {
    ...jobs.rate,
    account: "personal",
    backoff_until: "2026-09-10T10:46:04Z",
  });
  expect(root.querySelector('section[aria-label="Rate budget"] p')?.textContent).toContain(
    "not in backoff",
  );
});

test("the row cursor stays on the chosen run when a re-read puts a new run above it", async () => {
  const root = await openAdvancing({ [runPath("personal", "r-0913")]: ok("run-r-0913.json") });
  await press("j");
  const selected = () =>
    [...root.querySelectorAll('table.rows tbody tr[aria-selected="true"]')].map(
      (tr) => tr.querySelector("td")?.textContent,
    );
  expect(selected()).toEqual(["r-0913"]);
  await act(() => {
    for (const connection of connections.filter((c) => c.open)) {
      connection.refetch();
    }
  });
  await settle();
  expect(selected()).toEqual(["r-0913"]);
  await press("Enter");
  await settle();
  expect(location.pathname).toBe("/personal/jobs/r-0913");
});

test("keyboard focus stays on the chosen run's link when a re-read puts a new run above it", async () => {
  const root = await openAdvancing();
  const link = root.querySelector<HTMLAnchorElement>(
    'table.rows tbody tr a[href="/personal/jobs/r-0913"].mono',
  );
  link?.focus();
  expect(document.activeElement?.textContent).toBe("r-0913");
  await act(() => {
    for (const connection of connections.filter((c) => c.open)) {
      connection.refetch();
    }
  });
  await settle();
  expect(root.querySelector("table.rows tbody tr td")?.textContent).toBe("r-0917");
  expect(document.activeElement?.textContent).toBe("r-0913");
});

test("an account's first rate event reads the jobs endpoint again and brings up the rate budget", async () => {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("other")]: ok("system-other.json"),
    [jobsPath("other")]: [ok("jobs-other.json"), ok("jobs-other-spent.json")],
    [lensPath("other", `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick`)]:
      ok("runs-summary-other.json"),
    [lensPath("other", `${runs}&${canonical}`)]: ok("runs-rows-other.json"),
  });
  at(`/other/jobs?${canonical}`);
  connections = [];
  mounted = mount(<App deps={testDeps(server, connections)} />);
  await settle();
  const root = mounted.root;
  const budget = () => root.querySelector('section[aria-label="Rate budget"] p')?.textContent;
  expect(budget()).toBe("No rate state yet. The account has not spent from its budget.");
  const spent: Jobs = await recording("jobs-other-spent.json");
  if (spent.rate === null) {
    throw new Error("the recording holds no rate");
  }
  await deliver("rate", { ...spent.rate, account: "other" });
  expect(server.calls.filter((c) => c === jobsPath("other")).length).toBe(2);
  expect(budget()).toBe("0.6 of 5.0 units/s, cap 8.0 units/s, not in backoff, last throttle never");
});

test("the live indicator follows an account switch and shows none of the last account's stream", async () => {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [systemPath("other")]: ok("system-other.json"),
    [jobsPath("personal")]: ok("jobs.json"),
    [jobsPath("other")]: ok("jobs-other.json"),
    [lensPath("personal", `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick`)]:
      ok("runs-summary.json"),
    [lensPath("personal", `${runs}&${canonical}`)]: ok("runs-rows.json"),
    [lensPath("other", `${runs}&level=0&range=7d&sort=started_at,desc&pass=!tick`)]:
      ok("runs-summary-other.json"),
    [lensPath("other", `${runs}&${canonical}`)]: ok("runs-rows-other.json"),
  });
  at(`/personal/jobs?${canonical}`);
  connections = [];
  mounted = mount(<App deps={testDeps(server, connections)} />);
  await settle();
  const root = mounted.root;
  const jobs: Jobs = await recording("jobs.json");
  if (jobs.rate === null) {
    throw new Error("the recording holds no rate");
  }
  const rate = jobs.rate;
  await act(() => {
    for (const connection of connections.filter((c) => c.open && c.account === "personal")) {
      connection.status.value = "live";
      connection.objects.handle(
        "rate",
        JSON.stringify({ ...rate, account: "personal" }),
        Date.parse("2026-09-10T10:16:04Z"),
      );
    }
  });
  await settle();
  expect(root.querySelector(".live")?.textContent).toBe("live · updated 0s ago");
  await act(() => root.querySelector<HTMLButtonElement>('button[aria-label="Account"]')?.click());
  await act(() =>
    root.querySelector<HTMLAnchorElement>('[role="menu"] a[href^="/other/jobs"]')?.click(),
  );
  await settle();
  expect(location.pathname).toBe("/other/jobs");
  const other = connections.filter((c) => c.open && c.account === "other");
  expect(other.length).toBeGreaterThan(0);
  // The new account's stream is live and has sent nothing yet, so the indicator shows no update,
  // and in particular not the last account's.
  await act(() => {
    for (const connection of other) {
      connection.status.value = "live";
    }
  });
  await settle();
  expect(root.querySelector(".live")?.textContent).toBe("live · connecting");
});

test("a pass 1 run fetching stale subjects again shows the subjects it fetched again in the runs table, and draws them on its bar", async () => {
  const jobs: Jobs = JSON.parse(
    await Bun.file(new URL("fixtures/jobs-reopened.json", import.meta.url)).text(),
  );
  const run = jobs.backfill.pass1.run;
  if (run === null) {
    throw new Error("the recording holds no pass 1 run");
  }
  expect(progressOf(run)).toBe("12 of 42 subjects fetched again");
  expect(runMeasure(run)).toEqual({ part: 12, whole: 42, state: "running" });
  const ended = { ...run, checkpoint: JSON.parse('{"page": 3368, "of": 3368}'), counters: {} };
  expect(progressOf(ended)).toBe("page 3,368 of 3,368");
  expect(runMeasure(ended)).toEqual({ part: 3368, whole: 3368, state: "running" });
});
