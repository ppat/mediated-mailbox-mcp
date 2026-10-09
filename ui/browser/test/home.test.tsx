// The home screen of docs/UI.md section 8.1, rendered by the router over answers recorded from the real
// server, with its running-work strip refreshed through the stream's handler. The candidates' domains and
// signal evidence, the masking card's sender and rule, the applying plan's title and the last
// authentication outcome carry markup marker text, and the inert-rendering test here checks each in the
// form ADR-0064 requires. Its mutation demonstrations are under ui/browser/testdata/mutations.
import { afterEach, expect, test } from "bun:test";
import { options, type VNode } from "preact";
import { act } from "preact/test-utils";
import {
  accountsPath,
  attentionPath,
  jobsPath,
  systemPath,
  type Jobs,
  type System,
} from "../src/app/api.ts";
import { LiveProgress, LiveText } from "../src/app/live.tsx";
import { App } from "../src/app/router.tsx";
import {
  corpusValues,
  HomeScreen,
  oldestCandidates,
  pendingCandidates,
  signalText,
  strongest,
} from "../src/screens/home.tsx";
import { Card } from "../src/screens/jobs.tsx";
import { homeAnswers, testDeps, type Connection } from "./app.ts";
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

const chrome = {
  [accountsPath()]: ok("accounts.json"),
  [systemPath("personal")]: ok("system.json"),
  [systemPath("other")]: ok("system-other.json"),
};

async function open(
  path = "/personal",
  answers: Readonly<Record<string, Answer | readonly Answer[]>> = {
    ...chrome,
    ...homeAnswers("personal"),
  },
): Promise<HTMLElement> {
  server = recorded(answers);
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

// cells are the strip's cells, each its heading and its fields as name and text.
function cells(root: HTMLElement): [string, [string, string][]][] {
  return [...root.querySelectorAll(".running-now section.card")].map((card) => [
    card.querySelector("h2")?.textContent ?? "",
    [...card.querySelectorAll("dt")].map((dt) => [
      dt.textContent ?? "",
      dt.nextElementSibling?.textContent ?? "",
    ]),
  ]);
}

function field(root: HTMLElement, card: string, name: string): Element | undefined {
  const dts = [...root.querySelectorAll(`.running-now section.card[aria-label="${card}"] dt`)];
  return dts.find((dt) => dt.textContent === name)?.nextElementSibling ?? undefined;
}

// fill is a progress bar's drawn width, found by its label.
function fill(root: HTMLElement, label: string): string | null | undefined {
  return root
    .querySelector(`svg.progress[aria-label="${label}"] .progress-fill`)
    ?.getAttribute("width");
}

// deliver hands an event to every open connection.
async function deliver(name: string, data: object): Promise<void> {
  await act(() => {
    for (const connection of connections.filter((c) => c.open)) {
      connection.objects.handle(name, JSON.stringify(data), 1);
    }
  });
  await settle();
}

// items are the inbox's items, each its cells' text.
function items(root: HTMLElement): string[][] {
  return [...root.querySelectorAll(".inbox-item")].map((li) =>
    [...li.children].map((c) => c.textContent ?? ""),
  );
}

// values are a value list's rows as label, value, note and link.
function values(root: HTMLElement, label: string): (string | null)[][] {
  const list = root.querySelector(`.home-system dl[aria-label="${label}"]`);
  return [...(list?.querySelectorAll("dd") ?? [])].map((dd) => [
    dd.previousElementSibling?.textContent ?? null,
    dd.querySelector(".value")?.textContent ?? null,
    dd.querySelector(".muted")?.textContent ?? null,
    dd.querySelector("a")?.getAttribute("href") ?? null,
  ]);
}

const markup = (tag: string) => `<script>mmfieldmarker-${tag}</script>`;

test("the strip shows each workload's cell, with the pass running and the batch class", async () => {
  const root = await open();
  expect(root.querySelector(".running-now h2 a")?.getAttribute("href")).toBe("/personal/jobs");
  expect(cells(root)).toEqual([
    [
      "Backfill running",
      [
        ["pass 2", "r-0913 running"],
        ["started", "2026-09-10 09:36Z"],
        ["heartbeat", "1m ago"],
        ["checkpoint", "page 3,065 of 3,368 (91.0%)"],
        ["estimated time left", "50m 30s"],
        ["batch class", "1.9 of 2.5 units/s"],
      ],
    ],
    [
      "Delta sync idle",
      [
        ["last tick", "2026-09-10 10:11Z, succeeded"],
        ["last change set", "3 added, 1 modified, 0 removed"],
        ["cursor age", "4m"],
        ["cadence", "every 5m"],
      ],
    ],
    ["Reorg apply running", [["applying", `${markup("applyingplan")}, 1 of 2 operations`]]],
    ["Heuristics idle", [["last run", "2026-09-09 14:16Z, 0s, 2 candidates emitted"]]],
  ]);
  expect(fill(root, "Backfill progress")).toBe(String((3065 / 3368) * 160));
  expect(fill(root, "batch used of reserved")).toBe(String((1.9 / 2.5) * 160));
});

test("an event redraws the strip's run and the batch class, and nothing re-runs", async () => {
  let screens = 0;
  let cards = 0;
  let texts = 0;
  let bars = 0;
  const previous: ((vnode: VNode) => void) | undefined = Object.getOwnPropertyDescriptor(
    options,
    "diffed",
  )?.value;
  options.diffed = (vnode: VNode) => {
    if (vnode.type === HomeScreen) {
      screens += 1;
    }
    if (vnode.type === Card) {
      cards += 1;
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
    const root = await open();
    expect(root.querySelector(".live")?.textContent).toBe("live · connecting");
    const counted = [screens, cards, texts, bars];
    expect(counted.every((n) => n > 0)).toBe(true);
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
    expect(field(root, "Backfill", "checkpoint")?.textContent).toBe("page 3,100 of 3,368 (92.0%)");
    expect(fill(root, "Backfill progress")).toBe(String((3100 / 3368) * 160));
    await deliver("rate", {
      ...jobs.rate,
      account: "personal",
      classes: { ...jobs.rate.classes, batch: { used: 0.5, reserved: 2.5 } },
    });
    expect(field(root, "Backfill", "batch class")?.textContent).toBe("0.5 of 2.5 units/s");
    expect(fill(root, "batch used of reserved")).toBe(String((0.5 / 2.5) * 160));
    expect([screens, cards, texts, bars]).toEqual(counted);
  } finally {
    options.diffed = previous;
  }
});

test("a run event for a run the strip does not show reads the jobs endpoint again, and a poll every read", async () => {
  await open();
  const reads = (path: string) => server?.calls.filter((c) => c === path).length ?? 0;
  const jobs: Jobs = await recording("jobs.json");
  const tick = jobs.sync.last_tick;
  if (tick === null) {
    throw new Error("the recording holds no tick");
  }
  const before = reads(jobsPath("personal"));
  await deliver("run", { ...tick, account: "personal", run_id: "r-0999", state: "running" });
  expect(reads(jobsPath("personal"))).toBe(before + 1);
  const paths = [
    jobsPath("personal"),
    systemPath("personal"),
    attentionPath("personal"),
    pendingCandidates("personal"),
    oldestCandidates("personal"),
  ];
  const counts = paths.map(reads);
  await act(() => connections.find((c) => c.open)?.refetch());
  await settle();
  expect(paths.map(reads)).toEqual(counts.map((n) => n + 1));
});

test("the inbox lists the pending candidates by score with their strongest signals, unlinked", async () => {
  const root = await open();
  expect(root.querySelector(".inbox h2")?.textContent).toBe(
    "Awaiting your decision · 6 items, the oldest waiting 40d",
  );
  expect(items(root)).toEqual([
    [
      "Candidate",
      "lender.example",
      `display name ${markup("displayname")} matches listed bank.example`,
      "0.90",
      "4 messages",
      "first seen 2026-06",
      "1d 2h in queue",
    ],
    [
      "Candidate",
      "cluster.example",
      `registrable-domain clustering with listed ${markup("clusterdomain")}`,
      "0.85",
      "none",
      "none",
      "40d in queue",
    ],
    [
      "Candidate",
      "credit.example",
      `institution keyword ${markup("keyword")} in domain`,
      "0.80",
      "none",
      "none",
      "2h in queue",
    ],
    [
      "Candidate",
      markup("candidatedomain"),
      "transactional pattern (noreply, no List-Id, never labeled)",
      "0.65",
      "none",
      "none",
      "2h in queue",
    ],
    [
      "Candidate",
      "similar.example",
      `similar to confirmed ${markup("similardomain")} (0.87)`,
      "0.60",
      "none",
      "none",
      "2h in queue",
    ],
    [
      "Candidate",
      "strange.example",
      `${markup("heuristic")} unknown`,
      "0.40",
      "none",
      "none",
      "2h in queue",
    ],
  ]);
  // The candidate screen does not exist yet, so no item links anywhere.
  expect(root.querySelectorAll(".inbox a").length).toBe(0);
  const unknown = root.querySelectorAll(".inbox-item")[5]?.querySelector(".inbox-signal");
  expect(unknown?.querySelector(".badge")?.textContent).toBe("unknown");
});

test("the strongest signal comes first in the heuristics' order, and a signal without its evidence is not worded", () => {
  const none = { name: null, domain: null, keyword: null, score: null };
  const signals = [
    { heuristic: "a_new_one", evidence: none },
    { heuristic: "embedding_similarity", evidence: { ...none, domain: "chase.com", score: 0.5 } },
    { heuristic: "domain_clustering", evidence: { ...none, domain: "chase.com" } },
  ];
  expect(strongest(signals)?.heuristic).toBe("domain_clustering");
  expect(strongest([signals[0] ?? signals[1]].filter((s) => s !== undefined))?.heuristic).toBe(
    "a_new_one",
  );
  expect(strongest([])).toBeUndefined();
  expect(
    signalText({ heuristic: "display_name", evidence: { ...none, name: "Chase" } }),
  ).toBeUndefined();
  expect(signalText({ heuristic: "a_new_one", evidence: none })).toBeUndefined();
});

test("worth a look shows each card newest first, linking only into a screen that exists", async () => {
  const root = await open();
  const cards = [...root.querySelectorAll(".worth-a-look article")];
  expect(
    cards.map((c) => [
      c.querySelector("h3")?.textContent,
      c.querySelector(".attention-figure")?.textContent,
      c.querySelector("a")?.textContent ?? null,
      c.querySelector("a")?.getAttribute("href") ?? null,
    ]),
  ).toEqual([
    ["Scan backlog", "1 now", null, null],
    ["Body serves", "1 since 2026-09-10 08:16Z", null, null],
    [
      "Sync gap",
      "1 since 2026-09-08 10:16Z",
      "See Jobs",
      "/personal/jobs?range=7d&pass=gap_recovery",
    ],
    ["Masking", "21 since 2026-09-04 10:16Z", null, null],
  ]);
  expect(cards[2]?.querySelector(".attention-sentence")?.textContent).toBe(
    "Delta sync recovered from a cursor gap on 2026-09-08 10:16Z, re-enumerating a 6h window and reconciling 41 messages. A repeated gap means the cadence or the cursor lifetime needs attention.",
  );
});

test("the System column shows the operational and corpus blocks, linking where a screen exists", async () => {
  const root = await open();
  expect(root.querySelector(".home-system .as-of")?.textContent).toBe("as of 2026-09-10 10:16Z");
  expect(values(root, "Operational")).toEqual([
    ["Backfill pass 1", "complete", null, "/personal/jobs"],
    ["Backfill pass 2", "91.0%, 1 pending", null, null],
    ["Sync cursor age", "4m", " · last successful tick 2026-09-10 10:15Z", "/personal/jobs"],
    ["Rate", "3.1 of 5.0 units/s, cap 8.0 units/s, not in backoff", null, "/personal/jobs"],
    ["Last authentication", markup("authoutcome"), " · 2026-09-10 04:16Z", "/personal/account"],
  ]);
  expect(values(root, "Corpus")).toEqual([
    ["body served, 24 hours", "1", null, null],
    ["body denied, 24 hours", "1", null, null],
    ["mutation applied, 24 hours", "0", null, null],
    ["mutation refused, 24 hours", "0", null, null],
    ["Messages", "5", null, null],
    ["Threads", "4", null, null],
    ["Unfiled", "3 (60.0%)", null, null],
    ["Restricted messages", "1", null, null],
    ["Masking events, 7 days", "22", null, null],
    ["Gate skips, 7 days", "1", null, null],
  ]);
});

test("while backfill pass 1 runs, the corpus counts are counts so far", async () => {
  const system: System = await recording("system.json");
  const pass2 = system.operational.backfill_pass2_run;
  if (pass2 === null) {
    throw new Error("the recording holds no pass 2 run");
  }
  const indexing: System = {
    ...system,
    operational: {
      ...system.operational,
      backfill_pass1_complete: false,
      backfill_pass1_run: { ...pass2, pass: "pass1" },
      backfill_pass1_succeeded_at: null,
    },
  };
  expect(corpusValues(indexing).map((v) => v.text)).toEqual([
    "1 so far",
    "1 so far",
    "0 so far",
    "0 so far",
    "5 so far",
    "4 so far",
    "3 so far (60.0%)",
    "1 so far",
    "22 so far",
    "1 so far",
  ]);
});

test("while a re-opened backfill pass 1 runs, the corpus counts are not counts so far", async () => {
  const reopened: System = await recording("system-reopened.json");
  expect(corpusValues(reopened).map((v) => v.text)).toEqual([
    "1",
    "1",
    "0",
    "0",
    "5",
    "4",
    "3 (60.0%)",
    "1",
    "22",
    "1",
  ]);
});

test("an account with nothing indexed, nothing to decide and nothing worth a look says so", async () => {
  const root = await open("/other", {
    [accountsPath()]: ok("accounts.json"),
    [systemPath("other")]: ok("system-other-empty.json"),
    [jobsPath("other")]: ok("jobs-other-empty.json"),
    [attentionPath("other")]: ok("attention-other-empty.json"),
    [pendingCandidates("other")]: ok("candidates-rows-pending-other-empty.json"),
    [oldestCandidates("other")]: ok("candidates-rows-pending-oldest-other-empty.json"),
  });
  expect(cells(root)[0]).toEqual([
    "Backfill not started",
    [
      ["pass", "not started"],
      ["batch class", "no rate state yet"],
    ],
  ]);
  expect(root.querySelector(".inbox p")?.textContent).toBe("Nothing awaits your decision");
  expect(root.querySelector(".worth-a-look p")?.textContent).toBe(
    "Nothing is worth a look right now.",
  );
  expect(root.querySelector(".home-system dl[aria-label='Corpus']")).toBeNull();
  expect(
    root.querySelector(".home-system > div > p.muted, .home-system p.muted:not(.as-of)")
      ?.textContent,
  ).toBe("The index is empty. Backfill pass 1 has not started.");
  expect(root.querySelector(".banner")).toBeNull();
});

test("message-derived and attacker-written text arrives inert on every surface of the screen", async () => {
  const root = await open();
  expect(root.querySelectorAll("script").length).toBe(0);
  const inbox = [...root.querySelectorAll(".inbox-item")];
  // A candidate's domain.
  const domain = inbox[3]?.querySelector(".inbox-domain");
  expect(domain?.textContent).toBe(markup("candidatedomain"));
  expect(domain?.getAttribute("title")).toBe(markup("candidatedomain"));
  expect(domain?.childElementCount).toBe(0);
  // The signal evidence each template words, a display name, a clustered domain, a keyword and a
  // similar domain.
  for (const [i, text] of [
    [0, `display name ${markup("displayname")} matches listed bank.example`],
    [1, `registrable-domain clustering with listed ${markup("clusterdomain")}`],
    [2, `institution keyword ${markup("keyword")} in domain`],
    [4, `similar to confirmed ${markup("similardomain")} (0.87)`],
  ] as const) {
    const cell = inbox[i]?.querySelector(".inbox-signal");
    expect(cell?.textContent).toBe(text);
    expect(cell?.childElementCount).toBe(0);
  }
  // A stored identifier no template knows, shown as itself beside its badge.
  const unknown = inbox[5]?.querySelector(".inbox-signal");
  expect(unknown?.childElementCount).toBe(2);
  expect(unknown?.querySelector(".muted")?.textContent).toBe(markup("heuristic"));
  expect(unknown?.querySelector(".muted")?.childElementCount).toBe(0);
  // The masking card's sentence, which carries the sender's domain and the rule.
  const masking = root.querySelector(
    '.worth-a-look article[aria-label="Masking"] .attention-sentence',
  );
  expect(masking?.textContent).toBe(
    `Masking fired 21 times on ${markup("masksender")} this week, all under ${markup("maskrule")}. A sender masked this often under one rule is worth checking for an over-mask.`,
  );
  expect(masking?.childElementCount).toBe(0);
  for (const sentence of root.querySelectorAll(".attention-sentence")) {
    expect(sentence.childElementCount).toBe(0);
  }
  // The applying plan's title in the strip.
  const applying = field(root, "Reorg apply", "applying");
  expect(applying?.textContent).toBe(`${markup("applyingplan")}, 1 of 2 operations`);
  expect(applying?.childElementCount).toBe(0);
  // The last authentication outcome, provider text, in the System column.
  const auth = [...root.querySelectorAll('.home-system dl[aria-label="Operational"] dd')][4];
  const value = auth?.querySelector(".value");
  expect(value?.textContent).toBe(markup("authoutcome"));
  expect(value?.childElementCount).toBe(0);
  expect(auth?.querySelectorAll("*").length).toBe(3);
});

test("the strip and the banner share the account's one connection, which closes with the screen", async () => {
  const root = await open();
  // The banner shows, since pass 2 runs, so two surfaces follow the account on one connection.
  expect(root.querySelector(".banner")).not.toBeNull();
  expect(connections.map((c) => [c.account, c.open])).toEqual([["personal", true]]);
  const reads = (path: string) => server?.calls.filter((c) => c === path).length ?? 0;
  const system: System = await recording("system.json");
  const pass2 = system.operational.backfill_pass2_run;
  if (pass2 === null) {
    throw new Error("the recording holds no pass 2 run");
  }
  const before = reads(systemPath("personal"));
  // One backfill event reaches both, the banner reading the system endpoint again and the strip
  // redrawing its checkpoint.
  await deliver("run", {
    ...pass2,
    account: "personal",
    checkpoint: JSON.parse('{"page": 3200, "of": 3368}'),
  });
  expect(reads(systemPath("personal"))).toBe(before + 1);
  expect(field(root, "Backfill", "checkpoint")?.textContent).toBe("page 3,200 of 3,368 (95.0%)");

  // On the system screen only the banner follows. Home's connection has closed, and a poll on the
  // banner's reads the system endpoint and none of the strip's or the inbox's reads.
  await press("g");
  await press("s");
  await settle();
  expect(location.pathname).toBe("/personal/system");
  expect(connections.filter((c) => c.open).map((c) => c.account)).toEqual(["personal"]);
  expect(connections[0]?.open).toBe(false);
  const homeReads = [
    jobsPath("personal"),
    attentionPath("personal"),
    pendingCandidates("personal"),
  ];
  const counts = homeReads.map(reads);
  const systemBefore = reads(systemPath("personal"));
  await act(() => connections.find((c) => c.open)?.refetch());
  await settle();
  expect(homeReads.map(reads)).toEqual(counts);
  expect(reads(systemPath("personal"))).toBe(systemBefore + 1);
  mounted?.unmount();
  mounted = undefined;
  expect(connections.filter((c) => c.open)).toEqual([]);
});

test("switching accounts closes the last account's connection and follows the new one's", async () => {
  const root = await open("/personal", {
    ...chrome,
    ...homeAnswers("personal"),
    ...homeAnswers("other"),
  });
  await act(() => root.querySelector<HTMLButtonElement>('button[aria-label="Account"]')?.click());
  await act(() => root.querySelector<HTMLAnchorElement>('[role="menu"] a[href="/other"]')?.click());
  await settle();
  expect(location.pathname).toBe("/other");
  expect(connections.filter((c) => c.open).map((c) => c.account)).toEqual(["other"]);
  expect(connections.filter((c) => c.account === "personal").map((c) => c.open)).toEqual([false]);
});

test("an account's first rate event reads the jobs endpoint again and brings up the strip's batch class", async () => {
  const root = await open("/other", {
    ...chrome,
    ...homeAnswers("other"),
    [jobsPath("other")]: [ok("jobs-other.json"), ok("jobs-other-spent.json")],
  });
  expect(field(root, "Backfill", "batch class")?.textContent).toBe("no rate state yet");
  const spent: Jobs = await recording("jobs-other-spent.json");
  if (spent.rate === null) {
    throw new Error("the recording holds no rate");
  }
  await deliver("rate", { ...spent.rate, account: "other" });
  expect(server?.calls.filter((c) => c === jobsPath("other")).length).toBe(2);
  expect(field(root, "Backfill", "batch class")?.textContent).toBe("0.0 of 2.5 units/s");
});

test("a pass that starts while Home is open draws its checkpoint and bar on its first page event, and nothing re-runs", async () => {
  let cards = 0;
  let texts = 0;
  let bars = 0;
  const previous: ((vnode: VNode) => void) | undefined = Object.getOwnPropertyDescriptor(
    options,
    "diffed",
  )?.value;
  options.diffed = (vnode: VNode) => {
    if (vnode.type === Card) {
      cards += 1;
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
    const root = await open("/personal", {
      ...chrome,
      ...homeAnswers("personal"),
      [jobsPath("personal")]: ok("jobs-pass2-started.json"),
    });
    const jobs: Jobs = await recording("jobs-pass2-started.json");
    const run = jobs.backfill.pass2.run;
    if (run === null) {
      throw new Error("the recording holds no pass 2 run");
    }
    expect(field(root, "Backfill", "pass 2")?.textContent).toBe(`${run.run_id} running`);
    expect(field(root, "Backfill", "checkpoint")?.textContent).toBe("no page yet");
    expect(fill(root, "Backfill progress")).toBe("0");
    const counted = [cards, texts, bars];
    // A run's checkpoint is free-form JSON in the contract, so it is written as JSON.
    await deliver("run", {
      ...run,
      account: "personal",
      checkpoint: JSON.parse('{"page": 1, "of": 3368}'),
    });
    expect(field(root, "Backfill", "checkpoint")?.textContent).toBe("page 1 of 3,368 (0.0%)");
    expect(fill(root, "Backfill progress")).toBe(String((1 / 3368) * 160));
    expect([cards, texts, bars]).toEqual(counted);
  } finally {
    options.diffed = previous;
  }
});

test("a shown run's change of state reads the jobs and system endpoints again, and its cell follows", async () => {
  const root = await open();
  const reads = (path: string) => server?.calls.filter((c) => c === path).length ?? 0;
  const jobs: Jobs = await recording("jobs.json");
  // The last tick, which no banner follows, so every read here is the strip's.
  const tick = jobs.sync.last_tick;
  if (tick === null) {
    throw new Error("the recording holds no tick");
  }
  const counts = () => [reads(jobsPath("personal")), reads(systemPath("personal"))];
  const before = counts();
  // The same state again reads nothing.
  await deliver("run", { ...tick, account: "personal" });
  expect(counts()).toEqual(before);
  await deliver("run", { ...tick, account: "personal", state: "failed" });
  expect(counts()).toEqual(before.map((n) => n + 1));
  expect(field(root, "Delta sync", "last tick")?.textContent).toBe("2026-09-10 10:11Z, failed");
});

test("a re-opened pass 1 shows the subjects it fetched again of all it fetches, and follows its run's events without re-running", async () => {
  let texts = 0;
  let bars = 0;
  const previous: ((vnode: VNode) => void) | undefined = Object.getOwnPropertyDescriptor(
    options,
    "diffed",
  )?.value;
  options.diffed = (vnode: VNode) => {
    if (vnode.type === LiveText) {
      texts += 1;
    }
    if (vnode.type === LiveProgress) {
      bars += 1;
    }
    previous?.(vnode);
  };
  try {
    const root = await open("/personal", {
      ...chrome,
      [systemPath("personal")]: ok("system-reopened.json"),
      ...homeAnswers("personal"),
      [jobsPath("personal")]: ok("jobs-reopened.json"),
    });
    const jobs: Jobs = await recording("jobs-reopened.json");
    const run = jobs.backfill.pass1.run;
    if (run === null) {
      throw new Error("the recording holds no pass 1 run");
    }
    expect(field(root, "Backfill", "checkpoint")?.textContent).toBe(
      "12 of 42 subjects fetched again (28.6%)",
    );
    expect(fill(root, "Backfill progress")).toBe(String((12 / 42) * 160));
    const counted = [texts, bars];
    // A run's checkpoint and counters are free-form JSON in the contract, so they are written as JSON.
    await deliver("run", {
      ...run,
      account: "personal",
      checkpoint: { ...JSON.parse(JSON.stringify(run.checkpoint)), stale: 27 },
      counters: { ...JSON.parse(JSON.stringify(run.counters)), refetched: 15 },
    });
    expect(field(root, "Backfill", "checkpoint")?.textContent).toBe(
      "15 of 42 subjects fetched again (35.7%)",
    );
    expect(fill(root, "Backfill progress")).toBe(String((15 / 42) * 160));
    expect([texts, bars]).toEqual(counted);
  } finally {
    options.diffed = previous;
  }
});
