// The jobs screen of docs/UI.md section 8.3, a live surface. The workload cards and the rate budget
// read the jobs endpoint, the counts of plans in DRAFT and of candidates awaiting review read the
// system endpoint's decisions block, and the recent runs are the runs dataset at level 3. Each card's
// runs, the rate budget and each row's state, duration and checkpoint are bound to the objects the
// stream replaces, so an event redraws their text and no component re-runs (section 9).
import { computed, effect, untracked } from "@preact/signals";
import { useEffect, useMemo } from "preact/hooks";
import type { ComponentChildren } from "preact";
import {
  jobsPath,
  lensPath,
  systemPath,
  type Jobs,
  type Run,
  type RateEvent,
  type RunEvent,
  type RunRow,
  type RowsPage,
  type System,
} from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { age, count, duration, rate, utc } from "../app/format.ts";
import { numberIn } from "../app/frame.tsx";
import { LiveProgress, LiveText, type Measure, type Stream } from "../app/live.tsx";
import { Region } from "../app/region.tsx";
import { useCanonicalView } from "../app/router.tsx";
import { apiQuery, type View } from "../app/url.ts";
import { runState, word, workloadState } from "../app/wording.ts";
import { Lens, summaryView } from "../lens/lens.tsx";
import type { Column } from "../lens/table.tsx";
import { Badge, type Tone } from "../row/badge.tsx";
import type { ProgressState } from "../lens/progress.tsx";

// runHref is a run's screen.
export function runHref(account: string, run: string): string {
  return `/${encodeURIComponent(account)}/jobs/${encodeURIComponent(run)}`;
}

// planTitle is a plan's title, the first line of its description cut at 80 characters (section 8.1).
export function planTitle(description: string | null): string {
  const first = (description ?? "").split("\n")[0] ?? "";
  return first.length > 80 ? `${first.slice(0, 80)}…` : first;
}

// elapsed is a run's duration, to its finish or, while it runs, to now with "so far".
export function elapsed(run: Run, now: number): string {
  if (run.finished_at !== null) {
    return duration(Date.parse(run.finished_at) - Date.parse(run.started_at));
  }
  return `${duration(now - Date.parse(run.started_at))} so far`;
}

// progressOf is a run's checkpoint or operations as the runs table shows them, page of pages,
// operations of operations, or a gap recovery's window and reconciled count.
export function progressOf(run: Run): string {
  const page = numberIn(run.checkpoint, "page");
  const of = numberIn(run.checkpoint, "of");
  if (page !== undefined && of !== undefined) {
    return `page ${count(page)} of ${count(of)}`;
  }
  const done = numberIn(run.counters, "ops_done");
  const total = numberIn(run.counters, "ops_total");
  if (done !== undefined && total !== undefined) {
    return `${count(done)} of ${count(total)} operations`;
  }
  const reconciled = numberIn(run.counters, "reconciled");
  const start = textIn(run.counters, "window_start");
  const end = textIn(run.counters, "window_end");
  if (reconciled !== undefined && start !== undefined && end !== undefined) {
    return `${utc(start)} to ${utc(end)}, ${count(reconciled)} reconciled`;
  }
  return "";
}

// textIn reads a string from a free-form JSON object the contract leaves untyped.
function textIn(value: unknown, key: string): string | undefined {
  if (typeof value !== "object" || value === null) {
    return undefined;
  }
  const v: unknown = Reflect.get(value, key);
  return typeof v === "string" ? v : undefined;
}

// stateTone is a run state's badge color. The badge carries its label, so the color is not alone.
function stateTone(state: string): Tone {
  switch (state) {
    case "running":
      return "info";
    case "succeeded":
      return "ok";
    case "failed":
      return "restricted";
    default:
      return "muted";
  }
}

// RunState is a run's state as a labeled mark, its text and its color bound to its latest event.
export function RunState(props: { live: Stream; run: Run }) {
  const { live, run } = props;
  const latest = live.objects.run(run.run_id);
  // The tone is rebuilt for another run or another answer, as LiveText's text is.
  const tone = useMemo(() => computed(() => stateTone((latest.value ?? run).state)), [latest, run]);
  return (
    <span class="badge" data-tone={tone}>
      <LiveText value={latest} fallback={props.run} text={(r) => word(runState, r.state).text} />
    </span>
  );
}

// A card's line binds its text to one run's latest event.
function RunLine(props: {
  live: Stream;
  run: Run;
  text: (run: Run, now: number) => string;
  clock?: boolean;
}) {
  return (
    <LiveText
      value={props.live.objects.run(props.run.run_id)}
      fallback={props.run}
      text={props.text}
      clock={props.clock === true ? props.live.clock : undefined}
    />
  );
}

// jobsRuns are the runs the jobs endpoint's cards show.
export function jobsRuns(jobs: Jobs): Run[] {
  return [
    jobs.backfill.pass1.run,
    jobs.backfill.pass2.run,
    jobs.sync.last_tick,
    jobs.sync.last_gap_recovery,
    jobs.apply.running,
    jobs.apply.last,
    jobs.heuristics.last,
  ].filter((r): r is Run => r !== null);
}

export function JobsScreen(props: { account: string; live: Stream }) {
  const { account, live } = props;
  const { jobs } = useDeps();
  const view = useCanonicalView("runs");
  const path = jobsPath(account);
  return (
    <div class="jobs">
      <Region
        name="workloads"
        state={jobs.read(path)}
        shape="block"
        retry={() => void jobs.retry(path)}
      >
        {(answer) => (
          <>
            <Cards account={account} jobs={answer} live={live} />
            <RateBudget account={account} jobs={answer} live={live} />
          </>
        )}
      </Region>
      <Lens
        account={account}
        name="Recent runs"
        view={view}
        screen="jobs"
        rowsName="runs"
        columns={runColumns(account, live)}
        rows={(page: RowsPage) => (page.dataset === "runs" ? page.rows : undefined)}
        open={(row: RunRow) => runHref(account, row.run_id)}
        rowKey={(row: RunRow) => row.run_id}
        panelOpen={false}
      />
    </div>
  );
}

// onJobsRun is what a run event does to the jobs screen. A run neither the cards nor the table's page
// show started since the page loaded, so the jobs endpoint and the table's reads are read again. A run
// whose state differs from the one last seen or shown reads the system endpoint again, for the
// chrome's running mark (section 6), and the jobs endpoint again when a card or a row shows it.
export function onJobsRun(
  deps: ReturnType<typeof useDeps>,
  account: string,
  view: View,
  seen: Map<string, string>,
): (run: RunEvent) => void {
  return (run) => {
    const jobsState = deps.jobs.read(jobsPath(account)).peek();
    const rowsPath = lensPath(account, apiQuery(view));
    const rowsState = deps.lens.read(rowsPath).peek();
    const shown: Run[] = [
      ...(jobsState.status === "ok" ? jobsRuns(jobsState.answer) : []),
      ...(rowsState.status === "ok" &&
      "page" in rowsState.answer &&
      rowsState.answer.dataset === "runs"
        ? rowsState.answer.rows
        : []),
    ];
    const known = new Map(shown.map((r) => [r.run_id, r.state]));
    if (!known.has(run.run_id)) {
      void deps.jobs.refresh(jobsPath(account));
      void deps.lens.refresh(lensPath(account, apiQuery(summaryView(view))));
      void deps.lens.refresh(rowsPath);
    }
    if ((seen.get(run.run_id) ?? known.get(run.run_id)) !== run.state) {
      seen.set(run.run_id, run.state);
      void deps.system.refresh(systemPath(account));
      // A shown run's new state can change its card's workload state and fields, which the jobs
      // endpoint's blocks carry.
      if (known.has(run.run_id)) {
        void deps.jobs.refresh(jobsPath(account));
      }
    }
  };
}

function runColumns(account: string, live: Stream): Column<RunRow>[] {
  return [
    {
      key: "run",
      header: "run",
      width: 120,
      cell: (row) => (
        <a class="mono" href={runHref(account, row.run_id)}>
          {row.run_id}
        </a>
      ),
    },
    {
      key: "workload",
      header: "workload",
      width: 220,
      cell: (row) =>
        `${row.workload}${row.plan_id !== null ? ` · ${planTitle(row.plan_description)}` : row.pass === null ? "" : ` · ${row.pass}`}`,
      title: (row) => (row.plan_id !== null ? (row.plan_description ?? "") : (row.pass ?? "")),
    },
    {
      key: "started",
      header: "started",
      width: 128,
      cell: (row) => utc(row.started_at),
      title: (row) => row.started_at,
    },
    {
      key: "duration",
      header: "duration",
      width: 120,
      cell: (row) => <RunLine live={live} run={row} text={elapsed} clock />,
    },
    {
      key: "state",
      header: "state",
      width: 104,
      cell: (row) => <RunState live={live} run={row} />,
    },
    {
      key: "progress",
      header: "checkpoint or operations",
      width: 220,
      cell: (row) => <RunLine live={live} run={row} text={(r) => progressOf(r)} />,
    },
    {
      key: "failures",
      header: "failures",
      width: 88,
      numeric: true,
      cell: (row) =>
        row.failures > 0 ? <a href={runHref(account, row.run_id)}>{count(row.failures)}</a> : "0",
    },
  ];
}

function Card(props: { name: string; state: string; children: ComponentChildren }) {
  return (
    <section class="card" aria-label={props.name}>
      <h2>
        {props.name}{" "}
        <Badge
          tone={props.state === "running" ? "info" : "muted"}
          text={word(workloadState, props.state).text}
        />
      </h2>
      <dl>{props.children}</dl>
    </section>
  );
}

function Field(props: { name: string; children: ComponentChildren }) {
  return (
    <>
      <dt>{props.name}</dt>
      <dd>{props.children}</dd>
    </>
  );
}

// Decision is one count of the system endpoint's decisions block, which the cards show beside the
// jobs endpoint's fields (section 8.3). It alone re-renders when the system read answers.
function Decision(props: { account: string; count: (d: System["decisions"]) => number }) {
  const { system } = useDeps();
  const state = system.read(systemPath(props.account)).value;
  return <>{state.status === "ok" ? count(props.count(state.answer.decisions)) : "…"}</>;
}

function Cards(props: { account: string; jobs: Jobs; live: Stream }) {
  const { account, jobs, live } = props;
  const { pass1, pass2 } = jobs.backfill;
  const sync = jobs.sync;
  const apply = jobs.apply;
  const heuristics = jobs.heuristics;
  return (
    <div class="cards">
      <Card name="Backfill" state={jobs.backfill.state}>
        <Field name="pass 1">
          {pass1.complete ? "complete" : "not complete"}
          {pass1.run === null ? null : (
            <>
              {" · "}
              <RunLine
                live={live}
                run={pass1.run}
                clock
                text={(r, now) =>
                  `${word(runState, r.state).text}, ${count(numberIn(r.counters, "messages") ?? 0)} messages, ${count(numberIn(r.counters, "pages") ?? 0)} pages, ${elapsed(r, now)}${r.finished_at === null ? "" : `, completed ${utc(r.finished_at)}`}`
                }
              />
            </>
          )}
        </Field>
        {pass2.run === null ? (
          <Field name="pass 2">{pass2.complete ? "complete" : "not started"}</Field>
        ) : (
          <>
            <Field name="pass 2">
              <a class="mono" href={runHref(account, pass2.run.run_id)}>
                {pass2.run.run_id}
              </a>{" "}
              <RunState live={live} run={pass2.run} />
            </Field>
            <Field name="checkpoint">
              <RunLine live={live} run={pass2.run} text={(r) => progressOf(r)} />
              <LiveProgress
                value={live.objects.run(pass2.run.run_id)}
                fallback={pass2.run}
                measure={runMeasure}
                label="Backfill pass 2 progress"
              />
            </Field>
            <Field name="decided">
              <RunLine
                live={live}
                run={pass2.run}
                text={(r) => {
                  const decided = numberIn(r.counters, "decided") ?? 0;
                  const pending = numberIn(r.counters, "pending") ?? 0;
                  return `${count(decided)} of ${count(decided + pending)}, ${count(pending)} pending, ${count(numberIn(r.counters, "scanned") ?? 0)} scanned this run, ${count(numberIn(r.counters, "skipped") ?? 0)} skipped`;
                }}
              />
            </Field>
            <Field name="started">{utc(pass2.run.started_at)}</Field>
            <Field name="heartbeat">
              <RunLine
                live={live}
                run={pass2.run}
                clock
                text={(r, now) =>
                  r.heartbeat_at === null ? "none yet" : `${age(r.heartbeat_at, now)} ago`
                }
              />
            </Field>
            {pass2.eta_seconds === null ? null : (
              <Field name="estimated time left">{duration(pass2.eta_seconds * 1_000)}</Field>
            )}
          </>
        )}
      </Card>
      <Card name="Delta sync" state={sync.state}>
        <Field name="cadence">every {duration(sync.cadence_seconds * 1_000)}</Field>
        {sync.last_tick === null ? (
          <Field name="last tick">none yet</Field>
        ) : (
          <Field name="last tick">
            <RunLine
              live={live}
              run={sync.last_tick}
              clock
              text={(r, now) =>
                `${utc(r.started_at)}, ${word(runState, r.state).text}, ${elapsed(r, now)}, ${count(numberIn(r.counters, "added") ?? 0)} added, ${count(numberIn(r.counters, "modified") ?? 0)} modified, ${count(numberIn(r.counters, "removed") ?? 0)} removed`
              }
            />
          </Field>
        )}
        <Field name="cursor age">
          {sync.cursor_at === null ? (
            "no cursor yet"
          ) : (
            <LiveText
              value={live.clock}
              fallback={0}
              text={(now) => age(sync.cursor_at ?? "", now)}
            />
          )}
        </Field>
        <Field name="gap recoveries, 7 days">
          {count(sync.gap_recoveries_7d)}
          {sync.last_gap_recovery === null
            ? null
            : `, last ${progressOf(sync.last_gap_recovery)} at ${utc(sync.last_gap_recovery.started_at)}`}
        </Field>
      </Card>
      <Card name="Reorg apply" state={apply.state}>
        {apply.running === null ? (
          <Field name="now">
            idle, <Decision account={account} count={(d) => d.plans_draft} /> plans in DRAFT
          </Field>
        ) : (
          <Field name="applying">
            {planTitle(apply.running.plan_description)}
            {", "}
            <RunLine live={live} run={apply.running} text={(r) => progressOf(r)} />
          </Field>
        )}
        {apply.last === null ? (
          <Field name="last run">none yet</Field>
        ) : (
          <Field name="last run">
            {planTitle(apply.last.plan_description)}
            {`, ${word(runState, apply.last.state).text}, ${count(numberIn(apply.last.counters, "ops_done") ?? 0)} operations, ${count(numberIn(apply.last.counters, "failures") ?? 0)} failures, ${elapsed(apply.last, Date.parse(jobs.as_of))}, ${utc(apply.last.started_at)}`}
            {apply.rollback_available
              ? `, rollback available over ${count(apply.op_log_rows ?? 0)} logged operations`
              : ", no rollback available"}
          </Field>
        )}
      </Card>
      <Card name="Heuristics" state={heuristics.state}>
        <Field name="cadence">every {duration(heuristics.cadence_seconds * 1_000)}</Field>
        {heuristics.last === null ? (
          <Field name="last run">none yet</Field>
        ) : (
          <Field name="last run">
            <RunLine
              live={live}
              run={heuristics.last}
              clock
              text={(r, now) =>
                `${utc(r.started_at)}, ${elapsed(r, now)}, ${count(numberIn(r.counters, "candidates") ?? 0)} candidates emitted`
              }
            />
          </Field>
        )}
        <Field name="awaiting review">
          <Decision account={account} count={(d) => d.candidates_pending} /> candidates
        </Field>
        <Field name="next run">
          {heuristics.next_run_at === null ? "after the first run" : utc(heuristics.next_run_at)}
        </Field>
      </Card>
    </div>
  );
}

// backoffWording is the backoff state's wording (section 11). A time already past reads not in backoff.
export function backoffWording(until: string | null, now: number): string {
  return until === null || Date.parse(until) <= now
    ? "not in backoff"
    : `in backoff until ${utc(until)}`;
}

// pagesOf is a run's checkpoint page of pages, undefined when its checkpoint records none.
function pagesOf(run: Run): { page: number; of: number } | undefined {
  const page = numberIn(run.checkpoint, "page");
  const of = numberIn(run.checkpoint, "of");
  return page === undefined || of === undefined ? undefined : { page, of };
}

// progressState is the progress bar's state for a run state, complete once it succeeded.
function progressState(state: Run["state"]): ProgressState {
  return state === "succeeded" ? "complete" : state === "failed" ? "failed" : "running";
}

// runMeasure is a paged run's checkpoint as its progress bar draws it, an empty track until the
// checkpoint records a page, so the bar fills on the run's first page event (docs/UI.md section 8.3).
function runMeasure(run: Run): Measure {
  const pages = pagesOf(run);
  return { part: pages?.page ?? 0, whole: pages?.of ?? 0, state: progressState(run.state) };
}

// classes are the rate budget's priority classes of ADR-0025, in the order the budget lists them.
const classes = ["interactive", "sync", "batch"] as const;

// classMeasures draw each class's used of reserved, in info, since a class's spend is a rate that
// neither completes nor fails (docs/UI.md section 8.3). Each is made once, so a bar's fill is not
// rebuilt on every render.
const classMeasures: Record<
  (typeof classes)[number],
  (rate: Pick<RateEvent, "classes">) => Measure
> = {
  interactive: classMeasure("interactive"),
  sync: classMeasure("sync"),
  batch: classMeasure("batch"),
};

function classMeasure(name: string): (rate: Pick<RateEvent, "classes">) => Measure {
  return (r) => {
    const c = r.classes?.[name];
    return { part: c?.used ?? 0, whole: c?.reserved ?? 0, state: "running" };
  };
}

// RateBudget is the rate budget of docs/UI.md section 8.3. While the answer shown has no rate state, a
// rate event reads the jobs endpoint again, as a run the screen does not show does, so an account's
// first spend brings up the budget. The event is read in an effect, so the component never re-runs on
// it.
function RateBudget(props: { account: string; jobs: Jobs; live: Stream }) {
  const { account, jobs, live } = props;
  const deps = useDeps();
  const spent = jobs.rate !== null;
  useEffect(() => {
    if (spent) {
      return undefined;
    }
    return effect(() => {
      if (live.objects.rate.value !== undefined) {
        untracked(() => void deps.jobs.refresh(jobsPath(account)));
      }
    });
  }, [spent, live, deps, account]);
  if (jobs.rate === null) {
    return (
      <section class="card" aria-label="Rate budget">
        <h2>Rate budget</h2>
        <p class="muted">No rate state yet. The account has not spent from its budget.</p>
      </section>
    );
  }
  const rateState = jobs.rate;
  return (
    <section class="card" aria-label="Rate budget">
      <h2>Rate budget</h2>
      <p>
        <LiveText
          value={live.objects.rate}
          fallback={rateState}
          clock={live.clock}
          text={(r, now) =>
            `${rate(r.current, r.target)}, cap ${r.cap.toFixed(1)} units/s, ${backoffWording(r.backoff_until, now)}, last throttle ${r.last_throttle_at === null ? "never" : utc(r.last_throttle_at)}`
          }
        />
      </p>
      <ul class="classes">
        {classes.map((name) => (
          <li key={name}>
            {name}{" "}
            <LiveText
              value={live.objects.rate}
              fallback={rateState}
              text={(r) => {
                const c = r.classes?.[name];
                return c === undefined
                  ? "no reservation recorded"
                  : `${c.used.toFixed(1)} of ${c.reserved.toFixed(1)} units/s`;
              }}
            />
            <LiveProgress
              value={live.objects.rate}
              fallback={rateState}
              measure={classMeasures[name]}
              label={`${name} used of reserved`}
            />
          </li>
        ))}
      </ul>
      <p class="muted">Interactive keeps its reservation, and batch absorbs any reduction first.</p>
    </section>
  );
}
