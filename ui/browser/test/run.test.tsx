// The run screen of docs/UI.md section 8.4, rendered by the router over answers recorded from the real
// server. Run r-0912 failed with five item failures and was resumed by r-0913, which is running and
// has none. The recorded subjects, error summary and event detail carry markup marker text, and the
// inert-rendering tests here check each surface that shows them in the form ADR-0064 requires. Their
// mutation demonstrations are under ui/browser/testdata/mutations.
import { afterEach, expect, test } from "bun:test";
import { options, type VNode } from "preact";
import { act } from "preact/test-utils";
import { LocationProvider } from "preact-iso";
import {
  accountsPath,
  failurePath,
  lensPath,
  runPath,
  systemPath,
  type FailureDetail,
  type FailuresPage,
  type FailureRow,
  type RunSummary,
} from "../src/app/api.ts";
import { LiveText } from "../src/app/live.tsx";
import { App } from "../src/app/router.tsx";
import type { components } from "../src/generated/contract.ts";
import { canonicalize, parse } from "../src/app/url.ts";
import { GroupsView } from "../src/lens/groups.tsx";
import { RowsTable } from "../src/lens/table.tsx";
import { failureColumns, pageRow } from "../src/row/failure.tsx";
import { FailureBody, meaning, RunScreen } from "../src/screens/run.tsx";
import { testDeps, type Connection } from "./app.ts";
import { ok, recorded, type Answer, type Recorded } from "./fixtures/fetch.ts";
import { at, mount, settle, type Mounted } from "./render.ts";

type BySender = components["schemas"]["FailuresBySender"];

let mounted: Mounted | undefined;
let server: Recorded | undefined;
let connections: Connection[] = [];
afterEach(() => {
  mounted?.unmount();
  mounted = undefined;
  expect(server?.missing ?? []).toEqual([]);
  server = undefined;
});

const failures = "dataset=failures&run=r-0912";
const canonical = "level=1&group=error_class&sort=last_at,desc";

const r0912: Record<string, Answer> = {
  [runPath("personal", "r-0912")]: ok("run-r-0912.json"),
  [lensPath("personal", `${failures}&${canonical}`)]: ok("failures-by-error-class.json"),
  [lensPath("personal", `${failures}&level=1&group=disposition&sort=last_at,desc`)]: ok(
    "failures-by-disposition.json",
  ),
  [lensPath("personal", `${failures}&level=3&sort=last_at,desc&page=1`)]: ok("failures-rows.json"),
};

async function open(path: string, answers: Record<string, Answer>): Promise<HTMLElement> {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [systemPath("personal")]: ok("system.json"),
    ...answers,
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

// barsOf are a figure's bars, each label, count and whether it draws a restricted share.
function barsOf(f: Element | undefined) {
  return [...(f?.querySelectorAll(".bar") ?? [])].map((b) => [
    b.querySelector(".bar-label")?.textContent,
    b.querySelector(".bar-count")?.textContent,
    b.querySelectorAll(".bar-restricted").length,
  ]);
}

// failureDetail is a failure's panel over r-0912's items.
function failureDetail(row: FailureRow): string {
  return `/personal/jobs/r-0912/failures/${row.seq}`;
}

test("a URL missing parameters is replaced by the failures view's canonical form", async () => {
  await open("/personal/jobs/r-0912", r0912);
  expect(location.pathname + location.search).toBe(`/personal/jobs/r-0912?${canonical}`);
});

test("the strip shows the run, where it failed, its failures and its resumer", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, r0912);
  const head = root.querySelector('[aria-label="Run"]');
  expect([...(head?.querySelectorAll(".figure") ?? [])].map((f) => f.textContent)).toEqual([
    "backfill pass2r-0912",
    "failedstate",
    "2026-09-08 08:16Zstarted",
    "2026-09-08 09:16Zfinished",
    "1hduration",
    "page 14 of 3,368checkpoint",
  ]);
  const counts = root.querySelector('[aria-label="Failures of the run"]');
  expect([...(counts?.querySelectorAll(".figure") ?? [])].map((f) => f.textContent)).toEqual([
    "page 14 of 3,368, 1 retrywhere it failed",
    "5item failures",
    "1recovered by r-0913 (1)",
    "1not found at the provider",
    "r-0913 runningresumed by",
  ]);
  expect(root.querySelector('[aria-label="Failures of the run"] a')?.getAttribute("href")).toBe(
    "/personal/jobs/r-0913",
  );
});

test("the timeline draws a shaped mark per event, a legend of the six kinds, and no mark for progress", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, r0912);
  const marks = [...root.querySelectorAll(".timeline-mark")];
  expect(marks.map((m) => m.getAttribute("data-kind"))).toEqual([
    "start",
    "failure",
    "backoff",
    "retry",
    "finish",
  ]);
  expect(root.querySelectorAll(".timeline-progress").length).toBe(1);
  expect([...root.querySelectorAll(".legend-item")].map((l) => l.textContent)).toEqual([
    "start",
    "backoff",
    "retry",
    "failure",
    "resume",
    "finish",
  ]);
});

test("an event's detail carrying markup arrives in its mark's hover as text", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, r0912);
  const failure = root.querySelector('.timeline-mark[data-kind="failure"] title');
  expect(failure?.textContent).toBe(
    "failure at 2026-09-08 08:36Z, page 12: <script>mmfieldmarker-eventdetail</script>",
  );
  expect(failure?.childElementCount).toBe(0);
  expect(root.querySelectorAll("script").length).toBe(0);
});

test("the breakdown draws bars by error class with the restricted share, and one by disposition", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, r0912);
  const [byClass, byDisposition] = [...root.querySelectorAll("figure.bars")];
  expect(barsOf(byClass)).toEqual([
    ["provider error", "2", 0],
    ["not found at provider", "1", 0],
    ["scanner timeout", "1", 1],
    ["provider throttled", "1", 0],
  ]);
  expect(barsOf(byDisposition)).toEqual([
    ["pending", "2", 0],
    ["abandoned", "1", 0],
    ["not found", "1", 0],
    ["recovered", "1", 1],
  ]);
  expect(byClass?.querySelector(".bar")?.getAttribute("href")).toBe(
    "/personal/jobs/r-0912?level=2&group=sender&sort=last_at,desc&error_class=provider_error",
  );
  expect(
    [...root.querySelectorAll(".groupby a")].map((a) => [
      a.textContent,
      a.getAttribute("aria-current"),
    ]),
  ).toEqual([
    ["error class", "true"],
    ["sender", null],
    ["page", null],
    ["disposition", null],
  ]);
});

test("the items render message rows with the failure's columns, and a page item as a page row", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, r0912);
  const table = [...root.querySelectorAll("table.rows")].find((t) =>
    t.querySelector("caption")?.textContent?.startsWith("Failed items"),
  );
  expect(table?.querySelector("caption")?.textContent).toBe("Failed items, 5 in all");
  const rows = [...(table?.querySelectorAll("tbody tr") ?? [])];
  const page: FailuresPage = await recording("failures-rows.json");
  const pageIndex = page.rows.findIndex((r) => r.item_kind === "page");
  const pageCells = rows[pageIndex]?.querySelectorAll("td") ?? [];
  expect(pageCells.length).toBe(6);
  expect(pageCells[0]?.getAttribute("colspan")).toBe("7");
  expect(pageCells[0]?.textContent).toBe("page 13");
  const goneIndex = page.rows.findIndex((r) => r.item_id === "m-gone");
  const gone = [...(rows[goneIndex]?.querySelectorAll("td") ?? [])].map((td) => td.textContent);
  expect(gone.slice(0, 2)).toEqual(["m-gone", "not in the index"]);
  const bankIndex = page.rows.findIndex((r) => r.item_id === "m-bank");
  const bank = [...(rows[bankIndex]?.querySelectorAll("td") ?? [])].map((td) => td.textContent);
  expect(bank).toEqual([
    "mmfieldmarker-bankaddress@bank.example",
    "mmfieldmarker-banksubject",
    "2026-09-08",
    "",
    "restricted",
    "",
    "restricted",
    "12",
    "scanner timeout",
    "2",
    "2026-09-08 13:16Z",
    "recovered by r-0913",
  ]);
});

test("a subject carrying markup arrives as text in its message row, and nothing is built from it", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, r0912);
  const cells = [...root.querySelectorAll("tbody td")].filter((td) =>
    td.textContent?.includes("mmfieldmarker-newslettersubject"),
  );
  expect(cells.length).toBe(2);
  for (const td of cells) {
    expect(td.textContent).toBe("<script>mmfieldmarker-newslettersubject</script>");
    expect(td.getAttribute("title")).toBe("<script>mmfieldmarker-newslettersubject</script>");
    // The cell holds the link to the row's detail, and the link holds the text alone.
    expect(td.childElementCount).toBe(1);
    expect(td.firstElementChild?.childElementCount).toBe(0);
  }
  expect(root.querySelectorAll("script").length).toBe(0);
});

// marked is a failure with the markup marker text in every field the message row and the page row
// render as text, so rendering that stays inert for all but some fields cannot pass.
function marked(row: FailureRow, text: string): FailureRow {
  return {
    ...row,
    item_id: text,
    message_id: text,
    from_email: row.from_email === null ? null : text,
    subject: row.from_email === null ? null : text,
    labels: row.from_email === null ? null : [text, text, text, text, text],
    sender_class: row.from_email === null ? null : text,
    content_flags: row.from_email === null ? null : [text],
    scan_state: row.from_email === null ? null : text,
    error_class: text,
    disposition: text,
    recovered_by: text,
  };
}

test("markup in every field of every message row and page row arrives as text", async () => {
  const page: FailuresPage = await recording("failures-rows.json");
  const text = "<script>mmfieldmarker-newslettersubject</script>";
  const rows = page.rows.map((r) => marked(r, text));
  const table = mount(
    <LocationProvider>
      <RowsTable
        caption="Failed items"
        rowsName="failed items"
        columns={failureColumns(failureDetail, (id) => `/personal/jobs/${id}`)}
        span={pageRow(failureDetail)}
        rows={rows}
        page={1}
        pages={1}
        pageHref={(p) => `/personal/jobs/r-0912?page=${p}`}
      />
    </LocationProvider>,
  );
  try {
    const body = table.root.querySelector("tbody");
    expect(table.root.querySelectorAll("script").length).toBe(0);
    // Every text node the rows hold is the marker whole, or the fixed words the rows add around it.
    const words = new Set(["", " ", "+", "unknown", " by ", "page ", "not in the index"]);
    const walker = document.createTreeWalker(body ?? document.body, NodeFilter.SHOW_TEXT);
    const texts: string[] = [];
    for (let n = walker.nextNode(); n !== null; n = walker.nextNode()) {
      texts.push(n.textContent ?? "");
    }
    expect(
      texts.filter((t) => t !== text && !words.has(t) && !/^[\d-]+$/.test(t) && !t.includes("Z")),
    ).toEqual([]);
    expect(texts.filter((t) => t === text).length).toBeGreaterThan(rows.length * 5);
    for (const tr of body?.querySelectorAll("tr") ?? []) {
      for (const el of tr.querySelectorAll("*")) {
        expect(["TD", "A", "SPAN"]).toContain(el.tagName);
      }
    }
  } finally {
    table.unmount();
  }
});

test("a click on a bar applies its group and regroups at level 2, and the items follow the filter", async () => {
  const filtered = `${failures}&level=2&group=sender&sort=last_at,desc&error_class=provider_error`;
  const root = await open(`/personal/jobs/r-0912?${canonical}`, {
    ...r0912,
    [lensPath("personal", filtered)]: ok("failures-provider-error-by-sender.json"),
    [lensPath(
      "personal",
      `${failures}&level=1&group=disposition&sort=last_at,desc&error_class=provider_error`,
    )]: ok("failures-provider-error-by-disposition.json"),
    [lensPath(
      "personal",
      `${failures}&level=3&sort=last_at,desc&page=1&error_class=provider_error`,
    )]: ok("failures-provider-error-rows.json"),
  });
  await act(() => root.querySelector<HTMLAnchorElement>("figure.bars .bar")?.click());
  await settle();
  expect(location.search).toBe(
    "?level=2&group=sender&sort=last_at,desc&error_class=provider_error",
  );
  expect(root.querySelector(".breadcrumb .chip")?.textContent).toBe("error class provider error ×");
  expect(
    [...root.querySelectorAll("figure.bars")][0]?.querySelectorAll(".bar-label")[0]?.textContent,
  ).toBe("newsletter.example");
  const items = [...root.querySelectorAll("table.rows caption")].map((c) => c.textContent);
  expect(items).toContain("Failed items, 2 in all");
});

test("a failure opens as a panel with what happened, what it means, its provenance and audit rows", async () => {
  const path = `/personal/jobs/r-0912/failures/1?${canonical}`;
  const root = await open(path, {
    ...r0912,
    [failurePath("personal", "r-0912", "1")]: ok("failure-1.json"),
  });
  const panel = document.querySelector('aside[role="dialog"]');
  expect(panel?.getAttribute("aria-label")).toBe("Failure 1 of run r-0912");
  expect(root.querySelector(".frame")?.hasAttribute("inert")).toBe(true);
  expect(panel?.textContent).toContain(
    "The run recorded: <script>mmfieldmarker-errorsummary</script>",
  );
  expect(panel?.textContent).toContain(
    "scanner timeout on this item. Run r-0913 retried it successfully. Its scan state is not scanned, restricted sender.",
  );
  expect([...(panel?.querySelectorAll(".audit li") ?? [])].map((li) => li.textContent)).toEqual([
    "2026-09-10 08:16Z agent body denied",
  ]);
  expect(panel?.querySelector(".recorded")?.childElementCount).toBe(0);
  expect(document.querySelectorAll("script").length).toBe(0);
  await press("Escape");
  await settle();
  expect(location.pathname + location.search).toBe(`/personal/jobs/r-0912?${canonical}`);
});

test("a run with no failures says so, and reads neither the breakdown nor the items", async () => {
  const root = await open("/personal/jobs/r-0913?level=1&group=error_class&sort=last_at,desc", {
    [runPath("personal", "r-0913")]: ok("run-r-0913.json"),
  });
  expect(root.textContent).toContain("no failures");
  expect(server?.calls.some((c) => c.includes("/lens?"))).toBe(false);
});

test("a run event redraws the strip and the indicator shows, and the screen never re-runs", async () => {
  let screens = 0;
  let texts = 0;
  const previous: ((vnode: VNode) => void) | undefined = Object.getOwnPropertyDescriptor(
    options,
    "diffed",
  )?.value;
  options.diffed = (vnode: VNode) => {
    if (vnode.type === RunScreen) {
      screens += 1;
    }
    if (vnode.type === LiveText) {
      texts += 1;
    }
    previous?.(vnode);
  };
  try {
    const root = await open("/personal/jobs/r-0913?level=1&group=error_class&sort=last_at,desc", {
      [runPath("personal", "r-0913")]: ok("run-r-0913.json"),
    });
    expect(root.querySelector(".live")?.textContent).toBe("live · connecting");
    const checkpoint = () =>
      [...root.querySelectorAll('[aria-label="Run"] .figure')].at(-1)?.textContent;
    expect(checkpoint()).toBe("page 3,065 of 3,368checkpoint");
    const head = root.querySelector('[aria-label="Run"]');
    const before = [screens, texts];
    expect(texts).toBeGreaterThan(5);
    const summary: RunSummary = await recording("run-r-0913.json");
    // The partial-index banner follows the stream too, so the event reaches every open connection,
    // as the one stream of the account would deliver it.
    const live = connections.filter((c) => c.open);
    expect(live.length).toBeGreaterThan(0);
    // A run's checkpoint is free-form JSON in the contract, so it is written as JSON.
    const moved = {
      ...summary.run,
      account: "personal",
      checkpoint: JSON.parse('{"page": 3100, "of": 3368}'),
    };
    await act(() => {
      for (const c of live) {
        c.objects.handle("run", JSON.stringify(moved), 1);
      }
    });
    await settle();
    expect(checkpoint()).toBe("page 3,100 of 3,368checkpoint");
    expect(root.querySelector('[aria-label="Run"]')).toBe(head);
    expect([screens, texts]).toEqual(before);
    expect(server?.calls.filter((c) => c === runPath("personal", "r-0913")).length).toBe(2);
  } finally {
    options.diffed = previous;
  }
});

test("a failed run whose resumer runs is live, and follows the stream", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, r0912);
  expect(connections.some((c) => c.open && c.account === "personal")).toBe(true);
  expect(root.querySelector(".live")?.textContent).toBe("live · connecting");
});

test("a group's value carrying markup arrives as text in its bar and its row", async () => {
  const answer: BySender = await recording("failures-provider-error-by-sender.json");
  const text = "<script>mmfieldmarker-newslettersubject</script>";
  const markedGroups: BySender = {
    ...answer,
    rows: answer.rows.map((r) => ({ ...r, key: { sender: text } })),
  };
  const view = canonicalize(parse("failures", "level=2&group=sender&error_class=provider_error"));
  const groups = mount(
    <LocationProvider>
      <GroupsView
        account="personal"
        view={view}
        answer={markedGroups}
        name="Failures"
        screen="jobs/r-0912"
        stay
      />
    </LocationProvider>,
  );
  try {
    const labels = [...groups.root.querySelectorAll(".bar-label, tbody td a")];
    expect(labels.length).toBe(answer.rows.length * 2);
    for (const label of labels) {
      expect(label.textContent).toBe(text);
      expect(label.childElementCount).toBe(0);
    }
    expect(groups.root.querySelectorAll("script").length).toBe(0);
  } finally {
    groups.unmount();
  }
});

test("a gone item says the message was not found at the provider, and what the index holds of it", async () => {
  await open(`/personal/jobs/r-0912/failures/4?${canonical}`, {
    ...r0912,
    [failurePath("personal", "r-0912", "4")]: ok("failure-4.json"),
  });
  expect(document.querySelector('aside[role="dialog"]')?.textContent).toContain(
    "not found at provider. The index no longer holds it.",
  );
  const provenance = document.querySelector('aside[role="dialog"] .provenance');
  const shown = [...(provenance?.querySelectorAll("dt") ?? [])].find(
    (dt) => dt.textContent === "disposition",
  )?.nextElementSibling?.textContent;
  expect(shown).toBe("not found");
  // A gone item the first pass records keeps its message in the index, with its scan state as it was.
  const detail: FailureDetail = await recording("failure-4.json");
  expect(meaning({ ...detail.row, scan_state: "skipped_restricted" })).toBe(
    "not found at provider. Its scan state is not scanned, restricted sender.",
  );
});

test("a failure's panel shows its message's rule ids, scan time and scanner version", async () => {
  await open(`/personal/jobs/r-0912/failures/5?${canonical}`, {
    ...r0912,
    [failurePath("personal", "r-0912", "5")]: ok("failure-5.json"),
  });
  const provenance = document.querySelector('aside[role="dialog"] .provenance');
  const pairs = [...(provenance?.querySelectorAll("dt") ?? [])].map((dt) => [
    dt.textContent,
    dt.nextElementSibling?.textContent,
  ]);
  // No rule set the newsletter's normal class, and both content rules the scan recorded are listed.
  expect(pairs).toContainEqual(["rule that set the class", "none"]);
  expect(pairs).toContainEqual([
    "rule ids that fired",
    "content.mfa.subject_numeric_6, content.mfa.trigger_window",
  ]);
  expect(pairs).toContainEqual(["scanned", "2026-09-09 05:16Z, scanner version 3"]);
  // The newsletter's subject carries markup marker text, which arrives as text.
  const subject = [...(provenance?.querySelectorAll("dt") ?? [])].find(
    (dt) => dt.textContent === "subject",
  )?.nextElementSibling;
  expect(subject?.textContent).toBe("<script>mmfieldmarker-newslettersubject</script>");
  expect(subject?.childElementCount).toBe(0);
  expect(document.querySelectorAll("script").length).toBe(0);
});

test("a failure's panel shows the rule that set its message's class apart from the rule ids that fired", async () => {
  await open(`/personal/jobs/r-0912/failures/1?${canonical}`, {
    ...r0912,
    [failurePath("personal", "r-0912", "1")]: ok("failure-1.json"),
  });
  const provenance = document.querySelector('aside[role="dialog"] .provenance');
  const pairs = [...(provenance?.querySelectorAll("dt") ?? [])].map((dt) => [
    dt.textContent,
    dt.nextElementSibling?.textContent,
  ]);
  // The bank's rule restricted the sender, and no content rule fired on a message never scanned.
  expect(pairs).toContainEqual(["sender class", "restricted"]);
  expect(pairs).toContainEqual(["rule that set the class", "rule.bank"]);
  expect(pairs).toContainEqual(["rule ids that fired", "none"]);
  // The rule links to the account's policy searched for its identifier, which lists that identifier's
  // rule in each scope that holds one, since the index records the identifier and not its scope.
  const rule = [...(provenance?.querySelectorAll("dt") ?? [])].find(
    (dt) => dt.textContent === "rule that set the class",
  )?.nextElementSibling;
  expect(rule?.querySelector("a")?.getAttribute("href")).toBe("/personal/policy?search=rule.bank");
  // A restricted sender carries no Restrict control.
  expect(provenance?.textContent).not.toContain("Restrict");
});

test("a failure's panel offers Restrict {domain}… for a sender whose class reads normal", async () => {
  await open(`/personal/jobs/r-0912/failures/5?${canonical}`, {
    ...r0912,
    [failurePath("personal", "r-0912", "5")]: ok("failure-5.json"),
  });
  const provenance = document.querySelector('aside[role="dialog"] .provenance');
  const restrict = [...(provenance?.querySelectorAll("a") ?? [])].find((a) =>
    a.textContent?.startsWith("Restrict "),
  );
  expect(restrict?.textContent).toBe("Restrict newsletter.example…");
  expect(restrict?.getAttribute("href")).toBe("/personal/policy/new?suffix=newsletter.example");
});

test("markup in every message-derived field of a failure's panel arrives as text", async () => {
  const detail: FailureDetail = await recording("failure-5.json");
  const text = "<script>mmfieldmarker-newslettersubject</script>";
  const markedDetail: FailureDetail = {
    ...detail,
    error_summary: text,
    class_rule_id: text,
    rule_ids: [text],
    row: {
      ...detail.row,
      item_id: text,
      subject: text,
      from_email: text,
      sender_class: text,
      content_flags: [text],
      scan_state: text,
      error_class: text,
      disposition: text,
      recovered_by: text,
    },
  };
  const panel = mount(<FailureBody account="personal" detail={markedDetail} />);
  try {
    expect(panel.root.querySelectorAll("script").length).toBe(0);
    const values = [...panel.root.querySelectorAll(".provenance dd")];
    for (const label of [
      "item",
      "subject",
      "sender",
      "rule that set the class",
      "rule ids that fired",
      "error summary",
    ]) {
      const dd = values.find((v) => v.previousElementSibling?.textContent === label);
      expect(dd?.textContent).toContain(text);
      // The rule that set the class links to the policy searched for it, and the link holds the text.
      expect(
        [...(dd?.querySelectorAll("*") ?? [])].every(
          (el) => el.tagName === "SPAN" || (el.tagName === "A" && el.childElementCount === 0),
        ),
      ).toBe(true);
    }
    expect(panel.root.querySelector(".recorded")?.textContent).toBe(text);
    expect(panel.root.querySelector(".recorded")?.childElementCount).toBe(0);
  } finally {
    panel.unmount();
  }
});

// sender is the filter a link applies, read from the link's query string.
function sender(a: Element | null): string | null {
  return a === null
    ? null
    : new URLSearchParams(a.getAttribute("href")?.split("?")[1]).get("sender");
}

test("the empty group links with the word empty, and a stored value the grammar cannot name as itself links nowhere", () => {
  const view = canonicalize(parse("failures", "level=1&group=sender"));
  const answer: BySender = {
    account: "personal",
    dataset: "failures",
    level: 1,
    as_of: "2026-09-10T10:16:04Z",
    group: "sender",
    filters: {},
    total: { count: 9, restricted: 0, flagged: 0 },
    rows: [
      { key: { sender: "" }, count: 2, restricted: 0, flagged: 0 },
      { key: { sender: "example.test" }, count: 2, restricted: 0, flagged: 0 },
      { key: { sender: null }, count: 1, restricted: 0, flagged: 0 },
      { key: { sender: "none" }, count: 1, restricted: 0, flagged: 0 },
      { key: { sender: "empty" }, count: 1, restricted: 0, flagged: 0 },
      { key: { sender: "a,b" }, count: 1, restricted: 0, flagged: 0 },
      { key: { sender: "!x" }, count: 1, restricted: 0, flagged: 0 },
    ],
  };
  const groups = mount(
    <LocationProvider>
      <GroupsView
        account="personal"
        view={view}
        answer={answer}
        name="Failures"
        screen="jobs/r-0901"
        stay
      />
    </LocationProvider>,
  );
  try {
    const bars = [...groups.root.querySelectorAll("figure.bars .bar")];
    expect(
      bars.map((b) => [
        b.querySelector(".bar-label")?.textContent,
        sender(b.tagName === "A" ? b : null),
      ]),
    ).toEqual([
      ["empty", "empty"],
      ["example.test", "example.test"],
      ["no message", "none"],
      ["none", null],
      ["empty", null],
      ["a,b", null],
      ["!x", null],
    ]);
    const cells = [...groups.root.querySelectorAll("tbody tr td:first-child")];
    expect(cells.map((td) => [td.textContent, sender(td.querySelector("a"))])).toEqual([
      ["empty", "empty"],
      ["example.test", "example.test"],
      ["no message", "none"],
      ["none", null],
      ["empty", null],
      ["a,b", null],
      ["!x", null],
    ]);
  } finally {
    groups.unmount();
  }
});

test("j moves one row cursor, the failed items', and Enter opens one failure", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, {
    ...r0912,
    [failurePath("personal", "r-0912", "1")]: ok("failure-1.json"),
  });
  const entries = history.length;
  await press("j");
  const selected = [...root.querySelectorAll('tbody tr[aria-selected="true"]')];
  expect(selected.length).toBe(1);
  expect(selected[0]?.closest("table")?.querySelector("caption")?.textContent).toBe(
    "Failed items, 5 in all",
  );
  await press("Enter");
  await settle();
  expect(history.length).toBe(entries + 1);
  expect(location.pathname).toBe("/personal/jobs/r-0912/failures/1");
});

test("an unlisted account's failure path opens no panel and reads no failure", async () => {
  // The run screen's body reads its run before the accounts endpoint answers, and is refused.
  await open(`/nobody/jobs/r-0912/failures/1?${canonical}`, {
    [runPath("nobody", "r-0912")]: { file: "error-unknown-account-run.json", status: 400 },
  });
  expect(document.querySelector('aside[role="dialog"]')).toBeNull();
  expect(server?.calls.filter((c) => c.includes("/failures/"))).toEqual([]);
  expect(document.querySelector(".live")).toBeNull();
});

// runLinks are the three kinds of link from r-0912 to r-0913 its screen draws, the resumer and the
// recovering run in the strip, and a failed item's recovering run.
const runLinks: [string, string][] = [
  [
    "the resumer",
    '[aria-label="Failures of the run"] .figure:last-child a[href="/personal/jobs/r-0913"]',
  ],
  [
    "recovered by",
    '[aria-label="Failures of the run"] .figure:nth-child(3) a[href="/personal/jobs/r-0913"]',
  ],
  ["a failed item's recovering run", 'table.rows tbody a.mono[href="/personal/jobs/r-0913"]'],
];

for (const [kind, selector] of runLinks) {
  test(`following ${kind} to another run shows that run's strip and follows its events`, async () => {
    const root = await open(`/personal/jobs/r-0912?${canonical}`, {
      ...r0912,
      [runPath("personal", "r-0913")]: ok("run-r-0913.json"),
    });
    const link = root.querySelector<HTMLAnchorElement>(selector);
    expect(link).not.toBeNull();
    await act(() => link?.click());
    await settle();
    expect(location.pathname).toBe("/personal/jobs/r-0913");
    const head = () =>
      [...root.querySelectorAll('[aria-label="Run"] .figure')].map((f) => f.textContent);
    expect(head().slice(0, 2)).toEqual(["backfill pass2r-0913", "runningstate"]);
    expect(head().at(-1)).toBe("page 3,065 of 3,368checkpoint");
    expect(root.querySelector(".live")?.textContent).toBe("live · connecting");
    const summary: RunSummary = await recording("run-r-0913.json");
    await act(() => {
      for (const connection of connections.filter((c) => c.open)) {
        connection.objects.handle(
          "run",
          JSON.stringify({
            ...summary.run,
            account: "personal",
            checkpoint: JSON.parse('{"page": 3100, "of": 3368}'),
          }),
          1,
        );
      }
    });
    await settle();
    expect(head().at(-1)).toBe("page 3,100 of 3,368checkpoint");
  });
}

test("switching account on the run screen shows the other account's run and none of the last one's", async () => {
  const root = await open(`/personal/jobs/r-0912?${canonical}`, {
    ...r0912,
    [systemPath("other")]: ok("system-other.json"),
    [runPath("other", "r-0912")]: { file: "error-unknown-run-other.json", status: 404 },
  });
  const head = () =>
    [...root.querySelectorAll('[aria-label="Run"] .figure .figure-value')].map(
      (f) => f.textContent,
    );
  expect(head()[0]).toBe("backfill pass2");
  // A link to the same run under the other account, as a pasted address or a bookmark reaches it.
  const link = document.createElement("a");
  link.href = `/other/jobs/r-0912?${canonical}`;
  root.append(link);
  await act(() => link.click());
  await settle();
  expect(location.pathname).toBe("/other/jobs/r-0912");
  expect(head().every((text) => text === "")).toBe(true);
  expect(root.querySelector('[role="alert"] h2')?.textContent).toBe("This request was refused");
  expect(root.textContent).not.toContain("page 14 of 3,368");
});
