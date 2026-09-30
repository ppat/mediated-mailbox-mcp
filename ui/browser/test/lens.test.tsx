// The lens shell over the plans and candidates datasets, the two the server serves, at levels 0 and 3,
// on answers recorded from the real server. A plan's description is client-written text carrying markup
// marker text, and the inert-rendering test here checks that the rows table renders it as text in the
// form ADR-0064 requires. Its mutation demonstrations are under ui/browser/testdata/mutations.
import { afterEach, expect, test } from "bun:test";
import { render } from "preact";
import { act } from "preact/test-utils";
import { LocationProvider, Route, Router } from "preact-iso";
import {
  accountsPath,
  jobsPath,
  lensPath,
  systemPath,
  type Fetch,
  type LensFigures,
  type PlansPage,
  type System,
  type RowsPage,
} from "../src/app/api.ts";
import { DepsContext } from "../src/app/deps.ts";
import { planId } from "../src/app/format.ts";
import { useCanonicalView } from "../src/app/router.tsx";
import { planStatus } from "../src/app/wording.ts";
import { canonicalize, parse } from "../src/app/url.ts";
import { Breadcrumb } from "../src/lens/breadcrumb.tsx";
import { countsSoFar, Lens } from "../src/lens/lens.tsx";
import { figureText, Strip } from "../src/lens/strip.tsx";
import { RowsTable, type Column } from "../src/lens/table.tsx";
import { ManualTimers, testDeps } from "./app.ts";
import { ok, recorded, type Answer, type Recorded } from "./fixtures/fetch.ts";
import { at, mount, settle, type Mounted } from "./render.ts";
import { signal as makeSignal, type Signal } from "@preact/signals";
import type { State } from "../src/app/cache.ts";
import { Region } from "../src/app/region.tsx";
import { App } from "../src/app/router.tsx";

type PlanRow = PlansPage["rows"][number];

let mounted: Mounted | undefined;
let server: Recorded | undefined;
afterEach(() => {
  panelOpen = false;
  mounted?.unmount();
  mounted = undefined;
  expect(server?.missing ?? []).toEqual([]);
  server = undefined;
});

function refused(file: string): Answer {
  return { file, status: 400 };
}

function press(key: string): Promise<void> {
  return act(() => {
    document.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
  });
}

const plansDefault = "level=3&range=all&sort=created_at,desc&page=1";
const candidatesDefault = "level=3&range=all&sort=score,desc&page=1&status=pending";

const columns: readonly Column<PlanRow>[] = [
  {
    key: "plan",
    header: "plan",
    width: 80,
    cell: (row) => <span class="mono">{planId(row.plan_id)}</span>,
  },
  {
    key: "description",
    header: "description",
    width: 240,
    cell: (row) => row.description,
    title: (row) => row.description ?? "",
  },
  { key: "status", header: "status", width: 160, cell: (row) => planStatus[row.status] },
  { key: "messages", header: "messages", width: 88, numeric: true, cell: (row) => row.messages },
];

// described is a column showing a plan's description, for the test that puts it in every position.
function described(key: string): Column<PlanRow> {
  return { key, header: key, width: 120, cell: (row) => row.description };
}

function plansOf(page: RowsPage): readonly PlanRow[] | undefined {
  return page.dataset === "plans" ? page.rows : undefined;
}

function Plans(props: { account: string }) {
  const view = useCanonicalView("plans");
  return (
    <Lens
      account={props.account}
      name="Plans"
      view={view}
      rowsName="plans"
      columns={columns}
      rows={plansOf}
      open={(row) => `/${props.account}/plans/${row.plan_id}`}
      panelOpen={false}
    />
  );
}

// panelOpen is what the candidates route tells its lens about a detail panel, set by a test.
let panelOpen = false;

function Candidates(props: { account: string }) {
  const view = useCanonicalView("candidates");
  return (
    <Lens
      account={props.account}
      name="Review queue"
      view={view}
      rowsName="candidates"
      columns={[]}
      rows={(page) => (page.dataset === "candidates" ? page.rows : undefined)}
      panelOpen={panelOpen}
    />
  );
}

async function open(path: string, answers: Record<string, Answer>): Promise<HTMLElement> {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    ...answers,
  });
  at(path);
  mounted = mount(
    <DepsContext.Provider value={testDeps(server)}>
      <LocationProvider>
        <Router>
          <Route path="/:account/plans" component={Plans} />
          <Route path="/:account/candidates" component={Candidates} />
        </Router>
      </LocationProvider>
    </DepsContext.Provider>,
  );
  await settle();
  return mounted.root;
}

const plansAnswers = {
  [lensPath("personal", "dataset=plans&level=0&range=all&sort=created_at,desc")]:
    ok("plans-summary.json"),
  [lensPath("personal", `dataset=plans&${plansDefault}`)]: ok("plans-rows.json"),
};

test("a URL missing parameters is replaced by its canonical form", async () => {
  await open("/personal/plans", plansAnswers);
  expect(location.pathname + location.search).toBe(`/personal/plans?${plansDefault}`);
});

test("the summary strip shows every figure and the time the read began", async () => {
  const root = await open(`/personal/plans?${plansDefault}`, plansAnswers);
  const figures = [...root.querySelectorAll(".strip .figure")];
  expect(figures.map((f) => [f.textContent, f.getAttribute("href")])).toEqual([
    ["1Awaiting approval", "/personal/plans?status=DRAFT"],
    ["0Approved, waiting to apply", "/personal/plans?status=APPROVED"],
    ["1Applying", "/personal/plans?status=APPLYING"],
    ["1Applied", "/personal/plans?status=APPLIED"],
    ["0Rolled back", "/personal/plans?status=ROLLED_BACK"],
    ["0Rejected", "/personal/plans?status=REJECTED"],
    ["0Apply refused", "/personal/plans?status=APPLY_REFUSED"],
  ]);
  expect(root.querySelector(".strip .as-of")?.textContent).toBe("as of 2026-09-10 10:16Z");
});

test("level 3 is one page of rows under the strip", async () => {
  const root = await open(`/personal/plans?${plansDefault}`, plansAnswers);
  const table = root.querySelector("table.rows");
  expect(table?.querySelector("caption")?.textContent).toBe("Plans, 3 in all");
  expect([...(table?.querySelectorAll("th") ?? [])].map((th) => th.getAttribute("scope"))).toEqual([
    "col",
    "col",
    "col",
    "col",
  ]);
  const rows = [...(table?.querySelectorAll("tbody tr") ?? [])];
  expect(rows.map((r) => [...r.querySelectorAll("td")].map((td) => td.textContent))).toEqual([
    ["7f3a9c…", "<script>mmfieldmarker-applyingplan</script>", "Applying", "2"],
    [
      "7f3a9c…",
      "<script>mmfieldmarker-plandescription</script>\nSecond line",
      "Awaiting approval",
      "0",
    ],
    ["7f3a9c…", "<script>mmfieldmarker-appliedplan</script>", "Applied", "2"],
  ]);
  expect(root.querySelector(".pager")?.textContent).toBe("Page 1 of 1");
});

test("a description carrying markup arrives as text, and nothing is built from it", async () => {
  const root = await open(`/personal/plans?${plansDefault}`, plansAnswers);
  const cell = root.querySelectorAll("tbody tr")[1]?.querySelectorAll("td")[1];
  expect(cell?.textContent).toBe("<script>mmfieldmarker-plandescription</script>\nSecond line");
  expect(cell?.getAttribute("title")).toBe(
    "<script>mmfieldmarker-plandescription</script>\nSecond line",
  );
  expect(cell?.childElementCount).toBe(0);
  expect(root.querySelectorAll("script").length).toBe(0);
  expect(root.querySelector("tbody")?.querySelectorAll("*").length).toBe(3 + 3 * 4 + 3);

  // The same text in every column position and every row, so rendering that stays inert for all but
  // some columns or rows cannot pass.
  const answer: PlansPage = JSON.parse(
    await Bun.file(new URL("fixtures/plans-rows.json", import.meta.url)).text(),
  );
  const table = mount(
    <LocationProvider>
      <RowsTable
        caption="Plans"
        rowsName="plans"
        columns={["a", "b", "c", "d"].map(described)}
        rows={answer.rows}
        page={1}
        pages={1}
        pageHref={(p) => `/personal/plans?page=${p}`}
      />
    </LocationProvider>,
  );
  try {
    const cells = [...table.root.querySelectorAll("tbody td")];
    expect(cells.map((td) => td.textContent)).toEqual(
      answer.rows.flatMap((row) => Array.from({ length: 4 }, () => row.description ?? "")),
    );
    expect(cells.every((td) => td.childElementCount === 0)).toBe(true);
    expect(table.root.querySelectorAll("script").length).toBe(0);
    expect(table.root.querySelector("tbody")?.querySelectorAll("*").length).toBe(3 + 3 * 4);
  } finally {
    table.unmount();
  }
});

test("j and k move the row cursor and Enter opens the row under it", async () => {
  const root = await open(`/personal/plans?${plansDefault}`, plansAnswers);
  const selected = () =>
    [...root.querySelectorAll("tbody tr")].map((r) => r.getAttribute("aria-selected"));
  await press("j");
  await press("j");
  expect(selected()).toEqual(["false", "true", "false"]);
  await press("k");
  expect(selected()).toEqual(["true", "false", "false"]);
  await press("Enter");
  expect(location.pathname).toBe("/personal/plans/7f3a9c00-0000-4000-8000-000000000002");
});

const candidatesAnswers = {
  [lensPath("personal", "dataset=candidates&level=0&range=all&sort=score,desc&status=pending")]: ok(
    "candidates-summary-pending.json",
  ),
  [lensPath("personal", `dataset=candidates&${candidatesDefault}`)]: ok(
    "candidates-rows-pending.json",
  ),
  [lensPath("personal", "dataset=candidates&level=0&range=all&sort=score,desc")]: ok(
    "candidates-summary-all.json",
  ),
  [lensPath("personal", "dataset=candidates&level=3&range=all&sort=score,desc&page=1")]: ok(
    "candidates-rows-all.json",
  ),
};

test("a default filter is a chip, and removing it writes it empty so it stays removed", async () => {
  const root = await open("/personal/candidates", candidatesAnswers);
  const chip = root.querySelector<HTMLAnchorElement>(".breadcrumb .chip");
  expect(root.querySelector(".breadcrumb")?.firstChild?.textContent).toBe("Review queue");
  expect(chip?.textContent).toBe("status Awaiting review ×");
  expect(chip?.getAttribute("aria-label")).toBe("Remove the filter status Awaiting review");
  expect(chip?.getAttribute("href")).toBe(
    "/personal/candidates?level=3&range=all&sort=score,desc&page=1&status=",
  );
  await act(() => chip?.click());
  await settle();
  expect(location.search).toBe("?level=3&range=all&sort=score,desc&page=1&status=");
  expect(root.querySelector(".breadcrumb .chip")).toBeNull();
});

test("Escape removes the last chip", async () => {
  await open("/personal/candidates", candidatesAnswers);
  await press("Escape");
  await settle();
  expect(location.search).toBe("?level=3&range=all&sort=score,desc&page=1&status=");
});

test("the range control offers the presets and writes range=", async () => {
  const root = await open(`/personal/plans?${plansDefault}`, plansAnswers);
  const presets = [...root.querySelectorAll(".range a")];
  expect(presets.map((a) => [a.textContent, a.getAttribute("aria-current")])).toEqual([
    ["24 hours", null],
    ["7 days", null],
    ["30 days", null],
    ["90 days", null],
    ["all time", "true"],
  ]);
  expect(presets[1]?.getAttribute("href")).toBe(
    "/personal/plans?level=3&range=7d&sort=created_at,desc&page=1",
  );
});

test("a refused read shows its error card with the origin and request id, and Retry reads again", async () => {
  const search = "level=3&range=all&sort=bogus,desc&page=1";
  const root = await open(`/personal/plans?${search}`, {
    [lensPath("personal", "dataset=plans&level=0&range=all&sort=bogus,desc")]: refused(
      "error-unknown-sort-summary.json",
    ),
    [lensPath("personal", `dataset=plans&${search}`)]: refused("error-unknown-sort-rows.json"),
  });
  const cards = [...root.querySelectorAll('[role="alert"]')];
  expect(cards.map((card) => card.querySelector("h2")?.textContent)).toEqual([
    "This request was refused",
    "This request was refused",
  ]);
  expect(cards[1]?.textContent).toContain('declares no sortable column "bogus"');
  expect(cards[1]?.textContent).toContain("Request recorded");
  const reads = server?.calls.length ?? 0;
  await act(() => cards[1]?.querySelector("button")?.click());
  await settle();
  expect(server?.calls.length).toBe(reads + 1);
});

// A read that answers late is a condition of the transport, which no recording can hold. This fetch
// holds each lens read until the test releases it, then answers with the recording made at that path,
// and rejects a held read the app aborts, as the browser's fetch does.
type Held = { path: string; release: () => Promise<void>; aborted: () => boolean };

function holding(recording: Recorded, held: Held[]): Fetch {
  return (path, signal) => {
    if (!path.includes("/lens?")) {
      return recording.fetch(path, signal);
    }
    return new Promise<Response>((resolve, reject) => {
      signal.addEventListener("abort", () => reject(new DOMException("aborted", "AbortError")));
      held.push({
        path,
        release: async () => resolve(await recording.fetch(path, signal)),
        aborted: () => signal.aborted,
      });
    });
  };
}

test("a region past its timeout shows its card, a late answer replaces it, and Retry abandons the read", async () => {
  server = recorded({ [systemPath("personal")]: ok("system.json"), ...plansAnswers });
  const held: Held[] = [];
  const timers = new ManualTimers();
  const deps = testDeps({ ...server, fetch: holding(server, held) }, [], undefined, timers);
  at(`/personal/plans?${plansDefault}`);
  mounted = mount(
    <DepsContext.Provider value={deps}>
      <LocationProvider>
        <Router>
          <Route path="/:account/plans" component={Plans} />
          <Route path="/:account/candidates" component={Candidates} />
        </Router>
      </LocationProvider>
    </DepsContext.Provider>,
  );
  await settle();
  const root = mounted.root;
  expect(root.querySelectorAll('[aria-busy="true"]').length).toBe(2);
  expect(root.textContent).not.toContain("still loading");
  await act(() => timers.advance(999));
  expect(root.textContent).not.toContain("still loading");
  await act(() => timers.advance(1));
  expect(root.textContent).toContain("still loading");
  await act(() => timers.advance(8_999));
  expect(root.querySelector('[role="alert"]')).toBeNull();
  await act(() => timers.advance(1));
  expect([...root.querySelectorAll('[role="alert"] h2')].map((h) => h.textContent)).toEqual([
    "No answer came in time",
    "No answer came in time",
  ]);
  expect(root.textContent).toContain("no answer within 10 seconds, and the request is still open");
  const [summary, rows] = held;
  if (summary === undefined || rows === undefined || held.length !== 2) {
    throw new Error(`the lens held ${held.length} reads, want its summary and its rows`);
  }

  // The summary's answer arrives after its card showed, and replaces the card.
  await act(() => summary.release());
  await settle();
  expect(root.querySelector(".strip")).not.toBeNull();
  expect(root.querySelectorAll('[role="alert"]').length).toBe(1);

  // Retry abandons the rows read still open and sends a new one, whose answer renders the table.
  await act(() => root.querySelector<HTMLButtonElement>('[role="alert"] button')?.click());
  await settle();
  expect(rows.aborted()).toBe(true);
  expect(held.map((h) => h.path)).toEqual([summary.path, rows.path, rows.path]);
  expect(root.querySelectorAll('[aria-busy="true"]').length).toBe(1);
  await act(() => held[2]?.release());
  await settle();
  expect(root.querySelector("table.rows caption")?.textContent).toBe("Plans, 3 in all");
});

test("a lens with no rows keeps its strip and chips and says no rows match", async () => {
  const search = `${plansDefault}&status=REJECTED`;
  const root = await open(`/personal/plans?${search}`, {
    [lensPath("personal", "dataset=plans&level=0&range=all&sort=created_at,desc&status=REJECTED")]:
      ok("plans-summary-rejected.json"),
    [lensPath("personal", `dataset=plans&${search}`)]: ok("plans-rows-empty.json"),
  });
  expect([...root.querySelectorAll(".strip .figure-value")].map((v) => v.textContent)).toEqual([
    "0",
    "0",
    "0",
    "0",
    "0",
    "0",
    "0",
  ]);
  expect(root.querySelector(".breadcrumb .chip")?.textContent).toBe("status Rejected ×");
  expect(root.querySelector("tbody")?.textContent).toBe("No plans match");
  expect(root.querySelector("caption")?.textContent).toBe("Plans, 0 in all");
});

test("Escape leaves the chips alone while a detail panel is open", async () => {
  panelOpen = true;
  await open("/personal/candidates", candidatesAnswers);
  await press("Escape");
  await settle();
  expect(location.search).toBe(`?${candidatesDefault}`);
});

test("while backfill pass 1 runs, every figure on the strip is a count so far", async () => {
  const summary = JSON.parse(
    await Bun.file(new URL("fixtures/plans-summary.json", import.meta.url)).text(),
  );
  mounted = mount(<Strip summary={summary} soFar />);
  expect([...mounted.root.querySelectorAll(".figure .label")].map((l) => l.textContent)).toEqual([
    "Awaiting approval so far",
    "Approved, waiting to apply so far",
    "Applying so far",
    "Applied so far",
    "Rolled back so far",
    "Rejected so far",
    "Apply refused so far",
  ]);
});

test("a chip for a value the vocabulary does not hold shows the value marked unknown", () => {
  const view = canonicalize(parse("candidates", "status=pending,bogus"));
  mounted = mount(<Breadcrumb account="personal" name="Review queue" view={view} />);
  const chip = mounted.root.querySelector(".chip");
  expect(chip?.textContent).toBe("status Awaiting review or bogus unknown×");
  expect(chip?.querySelector(".badge")?.textContent).toBe("unknown");
  expect(chip?.getAttribute("aria-label")).toBe(
    "Remove the filter status Awaiting review or bogus, unknown",
  );
});

test("a new page starts with no row under the cursor", async () => {
  const page: PlansPage = JSON.parse(
    await Bun.file(new URL("fixtures/plans-rows.json", import.meta.url)).text(),
  );
  const table = (n: number) => (
    <LocationProvider>
      <RowsTable
        caption="Plans"
        rowsName="plans"
        columns={columns}
        rows={page.rows}
        page={n}
        pages={2}
        pageHref={(p) => `/personal/plans?page=${p}`}
      />
    </LocationProvider>
  );
  mounted = mount(table(1));
  await settle();
  await press("j");
  const selected = () =>
    [...(mounted?.root.querySelectorAll("tbody tr") ?? [])].map((r) =>
      r.getAttribute("aria-selected"),
    );
  expect(selected()).toEqual(["true", "false", "false"]);
  const root = mounted.root;
  await act(() => render(table(2), root));
  await settle();
  expect(selected()).toEqual(["false", "false", "false"]);
});

test("the strip counts so far while pass 1 runs or while the system read has no answer", async () => {
  const system: System = JSON.parse(
    await Bun.file(new URL("fixtures/system.json", import.meta.url)).text(),
  );
  expect(countsSoFar({ status: "ok", answer: system, at: 0 })).toBe(false);
  const failure = {
    origin: "client",
    code: "unknown_account",
    message: "",
    request_id: "",
    status: 400,
  } as const;
  expect(countsSoFar({ status: "error", failure })).toBe(true);
  expect(countsSoFar({ status: "loading", attempt: 0 })).toBe(true);
  const pass2 = system.operational.backfill_pass2_run;
  if (pass2 === null) {
    throw new Error("the recording holds no pass 2 run");
  }
  const running: System = {
    ...system,
    operational: {
      ...system.operational,
      backfill_pass1_complete: false,
      backfill_pass1_run: { ...pass2, pass: "pass1" },
    },
  };
  expect(countsSoFar({ status: "ok", answer: running, at: 0 })).toBe(true);
});

test("a figure is a count, a time in UTC, or none for a time with nothing to show", async () => {
  const summary: LensFigures = JSON.parse(
    await Bun.file(new URL("fixtures/plans-summary.json", import.meta.url)).text(),
  );
  const [first] = summary.figures;
  if (first === undefined) {
    throw new Error("the recording holds no figure");
  }
  expect(figureText({ ...first, value: 12480 })).toBe("12,480");
  expect(figureText({ ...first, value: null, at: "2026-09-08T09:16:04Z" })).toBe(
    "2026-09-08 09:16Z",
  );
  expect(figureText({ ...first, value: null, at: null })).toBe("none");
});

test("a region that moves to another slow read starts its timers again", async () => {
  const timers = new ManualTimers();
  const deps = testDeps(recorded({}), [], undefined, timers);
  const region = (state: Signal<State<string>>) => (
    <DepsContext.Provider value={deps}>
      <Region name="rows" state={state} shape="table" retry={() => undefined}>
        {(v) => v}
      </Region>
    </DepsContext.Provider>
  );
  mounted = mount(region(makeSignal<State<string>>({ status: "loading", attempt: 0 })));
  await settle();
  await act(() => timers.advance(1_000));
  expect(mounted.root.textContent).toContain("still loading");
  const root = mounted.root;
  await act(() =>
    render(region(makeSignal<State<string>>({ status: "loading", attempt: 0 })), root),
  );
  await settle();
  expect(root.textContent).not.toContain("still loading");
  // The new read's timers count from its start, a whole second.
  await act(() => timers.advance(999));
  expect(root.textContent).not.toContain("still loading");
  await act(() => timers.advance(1));
  expect(root.textContent).toContain("still loading");
});

test("the custom range's fields start again from a new range", async () => {
  const custom = "level=3&range=2026-09-01,2026-09-10&sort=started_at,desc&page=1&pass=!tick";
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    [jobsPath("personal")]: ok("jobs.json"),
    [lensPath(
      "personal",
      "dataset=runs&level=0&range=2026-09-01,2026-09-10&sort=started_at,desc&pass=!tick",
    )]: ok("runs-summary-custom.json"),
    [lensPath("personal", `dataset=runs&${custom}`)]: ok("runs-rows-custom.json"),
    [lensPath("personal", "dataset=runs&level=0&range=7d&sort=started_at,desc&pass=!tick")]:
      ok("runs-summary.json"),
    [lensPath("personal", "dataset=runs&level=3&range=7d&sort=started_at,desc&page=1&pass=!tick")]:
      ok("runs-rows.json"),
  });
  at(`/personal/jobs?${custom}`);
  const app = mount(<App deps={testDeps(server)} />);
  mounted = app;
  await settle();
  const fields = () =>
    [...app.root.querySelectorAll<HTMLInputElement>('.range input[type="date"]')].map(
      (i) => i.value,
    );
  expect(fields()).toEqual(["2026-09-01", "2026-09-10"]);
  await act(() =>
    [...app.root.querySelectorAll<HTMLAnchorElement>(".range a")]
      .find((a) => a.textContent === "7 days")
      ?.click(),
  );
  await settle();
  expect(location.search).toContain("range=7d");
  expect(fields()).toEqual(["", ""]);
});
