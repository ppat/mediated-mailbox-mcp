// The system screen of docs/UI.md section 8.8, rendered by the router over recorded answers.
import { afterEach, expect, test } from "bun:test";
import { act } from "preact/test-utils";
import { accountsPath, systemPath, type System } from "../src/app/api.ts";
import { App } from "../src/app/router.tsx";
import { filled, ProgressBar } from "../src/lens/progress.tsx";
import { backoff, systemValues, Values } from "../src/screens/system.tsx";
import { testDeps } from "./app.ts";
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

const answers = {
  [accountsPath()]: ok("accounts.json"),
  [systemPath("personal")]: ok("system.json"),
  [systemPath("other")]: ok("system-other.json"),
};

async function open(path: string, given: Readonly<Record<string, Answer>> = answers) {
  server = recorded(given);
  at(path);
  mounted = mount(<App deps={testDeps(server)} />);
  await settle();
  return mounted.root;
}

// rows reads the screen's rows as label, value text and note.
function rows(root: HTMLElement): (string | null)[][] {
  const list = root.querySelector('dl[aria-label="Operational state"]');
  return [...(list?.querySelectorAll("dd") ?? [])].map((dd) => [
    dd.previousElementSibling?.textContent ?? null,
    dd.querySelector(".value")?.textContent ?? null,
    dd.querySelector(".muted")?.textContent ?? null,
  ]);
}

async function recording(file: string): Promise<System> {
  return JSON.parse(await Bun.file(new URL(`fixtures/${file}`, import.meta.url)).text());
}

const marker = "<script>mmfieldmarker-authoutcome</script>";

test("the screen shows each operational value, and links to no screen that does not exist yet", async () => {
  const root = await open("/personal/system");
  expect(root.querySelector("main h1")?.textContent).toBe("System");
  expect(root.querySelector("main .as-of")?.textContent).toBe("as of 2026-09-10 10:16Z");
  expect(rows(root)).toEqual([
    ["Account", "personal", " · gmail"],
    ["Backfill pass 1", "complete", null],
    ["Backfill pass 2", "running, page 3,065 of 3,368 (91.0%)", null],
    ["Sync cursor age", "4m", null],
    ["Last successful tick", "2026-09-10 10:15Z", null],
    ["Scan backlog", "1 message pending scan", null],
    ["Rate", "3.1 of 5.0 units/s, cap 8.0 units/s", null],
    ["Backoff", "not in backoff", null],
    ["Last throttle", "2026-09-10 08:16Z", null],
    ["Last authentication", marker, " · 2026-09-10 04:16Z"],
  ]);
  expect(root.querySelector('dl[aria-label="Operational state"] a')).toBeNull();
  const bars = [...root.querySelectorAll("main svg.progress")];
  expect(bars.map((b) => b.getAttribute("aria-label"))).toEqual(["Backfill pass 2 progress"]);
  const fill = bars[0]?.querySelector(".progress-fill");
  expect(fill?.getAttribute("data-state")).toBe("running");
  expect(fill?.getAttribute("width")).toBe(String((3065 / 3368) * 160));
  const cursor = [...root.querySelectorAll("dd .value")][3];
  expect(cursor?.getAttribute("title")).toBe("2026-09-10 10:12Z");
});

test("an account with no state row reads not connected, with nothing recorded", async () => {
  const root = await open("/other/system");
  expect(rows(root)).toEqual([
    ["Account", "other", " · gmail, not connected"],
    ["Backfill pass 1", "running", null],
    ["Backfill pass 2", "not complete", null],
    ["Sync cursor age", "no cursor yet", null],
    ["Last successful tick", "none yet", null],
    ["Scan backlog", "1 message pending scan", null],
    ["Rate", "no rate state yet", null],
    ["Backoff", "not in backoff", null],
    ["Last throttle", "never", null],
    ["Last authentication", "none recorded", null],
  ]);
  expect(root.querySelector("main svg.progress")).toBeNull();
});

test("a backoff reads until its time while that time is ahead, and not in backoff after", () => {
  const now = Date.parse("2026-09-10T10:16:04Z");
  expect(backoff("2026-09-10T10:20:00Z", now)).toBe("in backoff until 2026-09-10 10:20Z");
  expect(backoff("2026-09-10T10:16:04Z", now)).toBe("not in backoff");
  expect(backoff(null, now)).toBe("not in backoff");
});

test("the screen and the partial-index banner share one read of the system endpoint", async () => {
  const root = await open("/personal/system");
  expect(root.querySelector(".banner")).not.toBeNull();
  expect(server?.calls.filter((c) => c === systemPath("personal"))).toEqual([
    systemPath("personal"),
  ]);
});

test("a failed system read shows the region's error card, and its retry reads again", async () => {
  const root = await open("/personal/system", {
    ...answers,
    [systemPath("personal")]: { file: "error-unknown-account.json", status: 400 },
  });
  const card = root.querySelector("main [role=alert]");
  expect(card?.querySelector("h2")?.textContent).toBe("This request was refused");
  expect(root.querySelector('dl[aria-label="Operational state"]')).toBeNull();
  const before = server?.calls.length ?? 0;
  await act(() => card?.querySelector("button")?.click());
  await settle();
  expect(server?.calls.slice(before)).toEqual([systemPath("personal")]);
});

test("an account the listing does not name reads no system", async () => {
  const root = await open("/nobody/system");
  expect(root.querySelector("main [role=alert]")?.textContent).toBe(
    "This request was refusedNo account nobody is served here.",
  );
  expect(server?.calls).not.toContain(systemPath("nobody"));
});

test("the navigation marks System, and g then s goes to it", async () => {
  const root = await open("/personal");
  const links = () =>
    [...root.querySelectorAll('nav[aria-label="Screens"] a')].map((a) => [
      a.textContent,
      a.getAttribute("href"),
      a.getAttribute("aria-current"),
    ]);
  expect(links()).toEqual([
    ["Home", "/personal", "page"],
    ["System", "/personal/system", null],
  ]);
  for (const key of ["g", "s"]) {
    await act(() => {
      document.dispatchEvent(new KeyboardEvent("keydown", { key, bubbles: true }));
    });
  }
  await settle();
  expect(location.pathname).toBe("/personal/system");
  expect(links()).toEqual([
    ["Home", "/personal", null],
    ["System", "/personal/system", "page"],
  ]);
});

test("an authentication outcome carrying markup arrives as text, and nothing is built from it", async () => {
  const root = await open("/personal/system");
  const list = root.querySelector('dl[aria-label="Operational state"]');
  const auth = [...(list?.querySelectorAll("dd .value") ?? [])].at(-1);
  expect(auth?.textContent).toBe(marker);
  expect(auth?.childElementCount).toBe(0);
  expect(root.querySelectorAll("script").length).toBe(0);
  // Ten labels and ten values, a value element in each, the account's and the authentication's
  // notes, and pass 2's progress bar, an svg with its track and its fill.
  expect(list?.querySelectorAll("*").length).toBe(10 + 10 + 10 + 2 + 3);

  // The same text in every row's value and note, so rendering that stays inert for all but some rows
  // cannot pass.
  const answer = await recording("system.json");
  const values = systemValues(answer, Date.parse(answer.as_of)).map((v) => ({
    ...v,
    text: marker,
    note: marker,
  }));
  const list2 = mount(<Values label="Every row" values={values} />);
  try {
    const texts = [...list2.root.querySelectorAll("dd .value")];
    expect(texts.map((t) => t.textContent)).toEqual(values.map(() => marker));
    expect(texts.every((t) => t.childElementCount === 0)).toBe(true);
    const notes = [...list2.root.querySelectorAll("dd .muted")];
    expect(notes.map((n) => n.textContent)).toEqual(values.map(() => ` · ${marker}`));
    expect(notes.every((n) => n.childElementCount === 0)).toBe(true);
    expect(list2.root.querySelectorAll("script").length).toBe(0);
    expect(list2.root.querySelector("dl")?.querySelectorAll("*").length).toBe(
      10 + 10 + 10 + 10 + 3,
    );
  } finally {
    list2.unmount();
  }
});

test("the progress bar fills its share of the track, clamped, in the color of its state", () => {
  expect(filled(1, 4)).toBe(40);
  expect(filled(5, 4)).toBe(160);
  expect(filled(-1, 4)).toBe(0);
  expect(filled(3, 0)).toBe(0);
  const bar = mount(<ProgressBar part={1} whole={2} state="failed" label="Pass progress" />);
  try {
    const svg = bar.root.querySelector("svg");
    expect(svg?.getAttribute("role")).toBe("img");
    expect(svg?.getAttribute("aria-label")).toBe("Pass progress");
    expect(svg?.querySelector(".progress-track")?.getAttribute("width")).toBe("160");
    const fill = svg?.querySelector(".progress-fill");
    expect([fill?.getAttribute("width"), fill?.getAttribute("data-state")]).toEqual([
      "80",
      "failed",
    ]);
  } finally {
    bar.unmount();
  }
});
