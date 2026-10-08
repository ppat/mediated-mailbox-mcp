// The home screen of docs/UI.md section 8.1. Under the chrome, the running-work strip, a live surface,
// then two columns. Left, what awaits a decision and what is worth a look. Right, the system. The strip
// reads the jobs endpoint and binds each cell's run and the batch class to the objects the stream
// replaces, so an event redraws their text and no component re-runs (section 9). The rest of Home is
// not live. Nothing here links to a screen that does not exist yet.
import {
  attentionPath,
  isRowsPage,
  jobsPath,
  lensPath,
  systemPath,
  type AttentionCard,
  type CandidateRow,
  type Jobs,
  type RateEvent,
  type Run,
  type RunEvent,
  type Signal,
  type System,
} from "../app/api.ts";
import type { Deps } from "../app/deps.ts";
import { useDeps } from "../app/deps.ts";
import { age, count, duration, local, rate, share, utc } from "../app/format.ts";
import {
  backfillProgress,
  existingScreen,
  indexing,
  ListedAccount,
  numberIn,
  settingsPath,
} from "../app/frame.tsx";
import { LiveProgress, LiveText, type Stream } from "../app/live.tsx";
import { Region } from "../app/region.tsx";
import { apiQuery, canonicalize, parse } from "../app/url.ts";
import { auditAction, runState, word } from "../app/wording.ts";
import { Badge } from "../row/badge.tsx";
import {
  Card,
  classMeasures,
  Decision,
  elapsed,
  Field,
  jobsRuns,
  pagesOf,
  planTitle,
  progressOf,
  RunLine,
  RunState,
  runHref,
  runMeasure,
  useFirstSpend,
} from "./jobs.tsx";
import { backoff, jobsHref, pass, Values, type Value } from "./system.tsx";

// pendingCandidates is the candidates dataset filtered to pending at its default sort, by score, which
// the inbox lists, and oldestCandidates the same filter sorted by created time ascending, whose first
// row is the oldest candidate (section 8.1).
export function pendingCandidates(account: string): string {
  return lensPath(account, apiQuery(canonicalize(parse("candidates", ""))));
}

export function oldestCandidates(account: string): string {
  return lensPath(account, apiQuery(canonicalize(parse("candidates", "sort=created_at,asc"))));
}

export function HomeScreen(props: { account: string; live: Stream }) {
  const { account, live } = props;
  // Nothing reads before the accounts endpoint lists the account, so an account the UI does not serve
  // is named once by the frame and read nowhere.
  return (
    <ListedAccount account={account}>
      <div class="home">
        <RunningNow account={account} live={live} />
        <div class="home-columns">
          <div class="home-main">
            <Inbox account={account} />
            <WorthALook account={account} />
          </div>
          <SystemColumn account={account} />
        </div>
      </div>
    </ListedAccount>
  );
}

// onHomeRun is what a run event does to Home. A run no cell shows, or a shown run whose state differs
// from the one last seen or shown, reads the jobs endpoint again, and a change of state also reads the
// system endpoint again, for the chrome's running mark and the reorg apply cell's count of plans in
// DRAFT (section 8.1).
export function onHomeRun(
  deps: Deps,
  account: string,
  seen: Map<string, string>,
): (run: RunEvent) => void {
  return (run) => {
    const state = deps.jobs.read(jobsPath(account)).peek();
    const known = new Map(
      (state.status === "ok" ? jobsRuns(state.answer) : []).map((r) => [r.run_id, r.state]),
    );
    if (!known.has(run.run_id)) {
      void deps.jobs.refresh(jobsPath(account));
    }
    if ((seen.get(run.run_id) ?? known.get(run.run_id)) !== run.state) {
      seen.set(run.run_id, run.state);
      void deps.system.refresh(systemPath(account));
      if (known.has(run.run_id)) {
        void deps.jobs.refresh(jobsPath(account));
      }
    }
  };
}

// homeReads are every read Home makes, which a reconnect and every poll of the fallback read again.
export function homeReads(account: string): string[] {
  return [
    jobsPath(account),
    systemPath(account),
    attentionPath(account),
    pendingCandidates(account),
    oldestCandidates(account),
  ];
}

// stripPass is the backfill pass the strip shows, pass 1 while its latest run runs, else pass 2 while
// its latest run runs, else the pass whose latest run finished last (section 8.1).
export function stripPass(jobs: Jobs): { label: string; run: Run; eta: number | null } | undefined {
  const passes = [
    { label: "pass 1", run: jobs.backfill.pass1.run, eta: jobs.backfill.pass1.eta_seconds },
    { label: "pass 2", run: jobs.backfill.pass2.run, eta: jobs.backfill.pass2.eta_seconds },
  ].flatMap((p) => (p.run === null ? [] : [{ ...p, run: p.run }]));
  const running = passes.find((p) => p.run.state === "running");
  if (running !== undefined) {
    return running;
  }
  return passes
    .filter((p) => p.run.finished_at !== null)
    .toSorted(
      (a, b) => Date.parse(b.run.finished_at ?? "") - Date.parse(a.run.finished_at ?? ""),
    )[0];
}

function RunningNow(props: { account: string; live: Stream }) {
  const { account, live } = props;
  const { jobs } = useDeps();
  const path = jobsPath(account);
  return (
    <section class="running-now" aria-labelledby="running-now-title">
      <h2 id="running-now-title">
        <a href={jobsHref(account)}>Running now</a>
      </h2>
      <Region
        name="running now"
        state={jobs.read(path)}
        shape="block"
        retry={() => void jobs.retry(path)}
      >
        {(answer) => <Cells account={account} jobs={answer} live={live} />}
      </Region>
    </section>
  );
}

function Cells(props: { account: string; jobs: Jobs; live: Stream }) {
  const { account, jobs, live } = props;
  useFirstSpend(account, jobs, live);
  const backfill = stripPass(jobs);
  const tick = jobs.sync.last_tick;
  const apply = jobs.apply;
  const heuristics = jobs.heuristics.last;
  return (
    <div class="cards">
      <Card name="Backfill" state={jobs.backfill.state}>
        {backfill === undefined ? (
          <Field name="pass">not started</Field>
        ) : (
          <>
            <Field name={backfill.label}>
              <a class="mono" href={runHref(account, backfill.run.run_id)}>
                {backfill.run.run_id}
              </a>{" "}
              <RunState live={live} run={backfill.run} />
            </Field>
            <Field name="started">{utc(backfill.run.started_at)}</Field>
            <Field name="heartbeat">
              <RunLine
                live={live}
                run={backfill.run}
                clock
                text={(r, now) =>
                  r.heartbeat_at === null ? "none yet" : `${age(r.heartbeat_at, now)} ago`
                }
              />
            </Field>
            <Field name="checkpoint">
              <RunLine live={live} run={backfill.run} text={checkpointText} />
              <LiveProgress
                value={live.objects.run(backfill.run.run_id)}
                fallback={backfill.run}
                measure={runMeasure}
                label="Backfill progress"
              />
            </Field>
            {backfill.eta === null ? null : (
              <Field name="estimated time left">{duration(backfill.eta * 1_000)}</Field>
            )}
          </>
        )}
        <Field name="batch class">
          {jobs.rate === null ? (
            "no rate state yet"
          ) : (
            <>
              <LiveText value={live.objects.rate} fallback={jobs.rate} text={batchText} />
              <LiveProgress
                value={live.objects.rate}
                fallback={jobs.rate}
                measure={classMeasures.batch}
                label="batch used of reserved"
              />
            </>
          )}
        </Field>
      </Card>
      <Card name="Delta sync" state={jobs.sync.state}>
        {tick === null ? (
          <Field name="last tick">none yet</Field>
        ) : (
          <>
            <Field name="last tick">
              <RunLine
                live={live}
                run={tick}
                text={(r) => `${utc(r.started_at)}, ${word(runState, r.state).text}`}
              />
            </Field>
            <Field name="last change set">
              <RunLine live={live} run={tick} text={changeSet} />
            </Field>
          </>
        )}
        <Field name="cursor age">
          {jobs.sync.cursor_at === null ? (
            "no cursor yet"
          ) : (
            <LiveText
              value={live.clock}
              fallback={0}
              text={(now) => age(jobs.sync.cursor_at ?? "", now)}
            />
          )}
        </Field>
        <Field name="cadence">every {duration(jobs.sync.cadence_seconds * 1_000)}</Field>
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
      </Card>
      <Card name="Heuristics" state={jobs.heuristics.state}>
        {heuristics === null ? (
          <Field name="last run">none yet</Field>
        ) : (
          <Field name="last run">
            <RunLine
              live={live}
              run={heuristics}
              clock
              text={(r, now) =>
                `${utc(r.started_at)}, ${elapsed(r, now)}, ${count(numberIn(r.counters, "candidates") ?? 0)} candidates emitted`
              }
            />
          </Field>
        )}
      </Card>
    </div>
  );
}

// checkpointText is a backfill run's progress with its percent, the subjects a pass 1 run fetched again
// of all it fetches while it fetches stale subjects again, else its checkpoint's page of pages, or "no
// page yet" until the checkpoint records either, beside a bar that is an empty track until then
// (docs/UI.md section 8.1).
function checkpointText(run: Run): string {
  return backfillProgress(run) ?? "no page yet";
}

function changeSet(run: Run): string {
  return `${count(numberIn(run.counters, "added") ?? 0)} added, ${count(numberIn(run.counters, "modified") ?? 0)} modified, ${count(numberIn(run.counters, "removed") ?? 0)} removed`;
}

// batchText is the batch class's used of reserved, the one priority class backfill spends from.
function batchText(r: Pick<RateEvent, "classes">): string {
  const c = r.classes?.batch;
  return c === undefined
    ? "no reservation recorded"
    : `${c.used.toFixed(1)} of ${c.reserved.toFixed(1)} units/s`;
}

// The heuristics of ADR-0004's table, in its order, which orders a candidate's signals (section 8.6).
const heuristics = [
  "display_name",
  "domain_clustering",
  "institution_keyword",
  "transactional_pattern",
  "embedding_similarity",
] as const;

// strongest is the first of a candidate's signals in the table's order, a signal whose identifier the
// table does not name coming after every one it does.
export function strongest(signals: readonly Signal[]): Signal | undefined {
  const rank = (s: Signal) => {
    const i = heuristics.findIndex((h) => h === s.heuristic);
    return i < 0 ? heuristics.length : i;
  };
  return signals.toSorted((a, b) => rank(a) - rank(b))[0];
}

// signalText is a signal worded by its heuristic's template, or undefined when its identifier is not one
// the table names or its template lacks an evidence key (section 8.6).
export function signalText(s: Signal): string | undefined {
  const e = s.evidence;
  switch (s.heuristic) {
    case "display_name":
      return e.name === null || e.domain === null
        ? undefined
        : `display name ${e.name} matches listed ${e.domain}`;
    case "domain_clustering":
      return e.domain === null
        ? undefined
        : `registrable-domain clustering with listed ${e.domain}`;
    case "institution_keyword":
      return e.keyword === null ? undefined : `institution keyword ${e.keyword} in domain`;
    case "transactional_pattern":
      return "transactional pattern (noreply, no List-Id, never labeled)";
    case "embedding_similarity":
      return e.domain === null || e.score === null
        ? undefined
        : `similar to confirmed ${e.domain} (${e.score.toFixed(2)})`;
    default:
      return undefined;
  }
}

// SignalWords is a candidate's strongest signal in words, or the signal's identifier in muted text with
// an unknown badge, never dropped (sections 7.1 and 8.6).
export function SignalWords(props: { signals: readonly Signal[] }) {
  const s = strongest(props.signals);
  if (s === undefined) {
    return <span class="muted">no signal recorded</span>;
  }
  const text = signalText(s);
  if (text !== undefined) {
    return <>{text}</>;
  }
  return (
    <>
      <span class="muted">{s.heuristic === "" ? "unreadable signal" : s.heuristic}</span>{" "}
      <Badge tone="muted" text="unknown" />
    </>
  );
}

function Inbox(props: { account: string }) {
  const { lens, now } = useDeps();
  const path = pendingCandidates(props.account);
  return (
    <section class="inbox" aria-labelledby="inbox-title">
      <h2 id="inbox-title">
        Awaiting your decision
        <InboxCount account={props.account} />
      </h2>
      <Region
        name="awaiting your decision"
        state={lens.read(path)}
        shape="table"
        retry={() => void lens.retry(path)}
      >
        {(answer) => {
          const rows: readonly CandidateRow[] =
            isRowsPage(answer) && answer.dataset === "candidates" ? answer.rows : [];
          const total = isRowsPage(answer) ? answer.total.count : 0;
          if (total === 0) {
            return <p class="muted">Nothing awaits your decision</p>;
          }
          return (
            <>
              <ul class="inbox-items">
                {rows.map((row) => (
                  <li key={row.domain} class="inbox-item">
                    <span class="label">Candidate</span>
                    <span class="inbox-domain" title={row.domain}>
                      {row.domain}
                    </span>
                    <span class="inbox-signal">
                      <SignalWords signals={row.signals} />
                    </span>
                    <span class="numeric">{row.score.toFixed(2)}</span>
                    <span class="numeric">
                      {row.message_count === null ? "none" : `${count(row.message_count)} messages`}
                    </span>
                    <span>
                      {row.first_seen === null
                        ? "none"
                        : `first seen ${row.first_seen.slice(0, 7)}`}
                    </span>
                    <span title={utc(row.created_at)}>{age(row.created_at, now())} in queue</span>
                  </li>
                ))}
              </ul>
            </>
          );
        }}
      </Region>
    </section>
  );
}

// InboxCount is the heading's item count and the oldest item's age, once the inbox's read has answered
// with items (section 8.1).
function InboxCount(props: { account: string }) {
  const { lens } = useDeps();
  const state = lens.read(pendingCandidates(props.account)).value;
  if (state.status !== "ok" || !isRowsPage(state.answer) || state.answer.total.count === 0) {
    return null;
  }
  const total = state.answer.total.count;
  return (
    <span class="inbox-count muted">
      {" · "}
      {count(total)} {total === 1 ? "item" : "items"}, the oldest waiting{" "}
      <OldestAge account={props.account} />
    </span>
  );
}

// OldestAge is how long the oldest pending candidate has waited, from the read sorted by created time.
function OldestAge(props: { account: string }) {
  const { lens, now } = useDeps();
  const state = lens.read(oldestCandidates(props.account)).value;
  if (state.status !== "ok") {
    return <>{state.status === "error" ? "an unknown time" : "…"}</>;
  }
  const answer = state.answer;
  const first = isRowsPage(answer) && answer.dataset === "candidates" ? answer.rows[0] : undefined;
  return <>{first === undefined ? "an unknown time" : age(first.created_at, now())}</>;
}

function WorthALook(props: { account: string }) {
  const { attention } = useDeps();
  const path = attentionPath(props.account);
  return (
    <section class="worth-a-look" aria-labelledby="worth-a-look-title">
      <h2 id="worth-a-look-title">Worth a look</h2>
      <Region
        name="worth a look"
        state={attention.read(path)}
        shape="block"
        retry={() => void attention.retry(path)}
      >
        {(answer) =>
          answer.cards.length === 0 ? (
            <p class="muted">Nothing is worth a look right now.</p>
          ) : (
            answer.cards.map((c, i) => <AttentionItem key={`${c.rule}-${i}`} card={c} />)
          )
        }
      </Region>
    </section>
  );
}

// AttentionItem is one card, its what, number, since, sentence and link. The sentence is the server's,
// and carries a sender's domain an adversary chose, so it renders as text (section 8.1).
function AttentionItem(props: { card: AttentionCard }) {
  const { card } = props;
  const screen = existingScreen(card.link);
  return (
    <article class="attention card" aria-label={card.what}>
      <h3>{card.what}</h3>
      <p class="attention-figure">
        <span class="figure-value">{count(card.number)}</span>{" "}
        {card.since === null ? (
          <span class="muted">now</span>
        ) : (
          <span class="muted" title={local(card.since)}>
            since {utc(card.since)}
          </span>
        )}
      </p>
      <p class="attention-sentence">{card.sentence}</p>
      {screen === undefined ? null : <a href={card.link}>See {screen}</a>}
    </article>
  );
}

function SystemColumn(props: { account: string }) {
  return (
    <section class="home-system" aria-labelledby="home-system-title">
      <h2 id="home-system-title">System</h2>
      <SystemFigures account={props.account} />
    </section>
  );
}

// SystemFigures reads the system endpoint through the cache the partial-index banner reads, so both
// share one request and one answer.
function SystemFigures(props: { account: string }) {
  const { system, now } = useDeps();
  const path = systemPath(props.account);
  return (
    <Region
      name="system"
      state={system.read(path)}
      shape="block"
      retry={() => void system.retry(path)}
    >
      {(answer) => (
        <>
          <p class="as-of muted" title={local(answer.as_of)}>
            as of {utc(answer.as_of)}
          </p>
          <Values label="Operational" values={operationalValues(answer, now())} />
          <Corpus system={answer} />
        </>
      )}
    </Region>
  );
}

// operationalValues are the System column's operational rows (section 8.1). A row links where its
// screen exists, Jobs and System, and the pass 2 row, which sends to the corpus lens, does not yet.
export function operationalValues(system: System, now: number): Value[] {
  const op = system.operational;
  const jobs = jobsHref(system.account);
  const toJobs = (v: Value): Value => ({ ...v, href: jobs });
  const pass2 = op.backfill_pass2_run;
  const pages = pass2 === null ? undefined : pagesOf(pass2);
  const pending = `${count(op.pending_scan)} pending`;
  const r = op.rate;
  return [
    toJobs(pass("pass1", "Backfill pass 1", op.backfill_pass1_complete, op.backfill_pass1_run)),
    {
      key: "pass2",
      label: "Backfill pass 2",
      text: op.backfill_pass2_complete
        ? "complete"
        : pass2?.state === "running" && pages !== undefined
          ? `${share(pages.page, pages.of)}, ${pending}`
          : `not complete, ${pending}`,
    },
    toJobs({
      key: "cursor",
      label: "Sync cursor age",
      text: op.sync_cursor_at === null ? "no cursor yet" : age(op.sync_cursor_at, now),
      title: op.sync_cursor_at === null ? undefined : utc(op.sync_cursor_at),
      note:
        op.last_successful_tick_at === null
          ? "no successful tick yet"
          : `last successful tick ${utc(op.last_successful_tick_at)}`,
    }),
    toJobs({
      key: "rate",
      label: "Rate",
      text:
        r === null
          ? "no rate state yet"
          : `${rate(r.current, r.target)}, cap ${r.cap.toFixed(1)} units/s, ${backoff(r.backoff_until, now)}`,
    }),
    {
      key: "auth",
      label: "Last authentication",
      text: op.last_auth_outcome ?? "none recorded",
      note: op.last_auth_at === null ? undefined : utc(op.last_auth_at),
      href: settingsPath(system.account),
    },
  ];
}

// corpusValues are the System column's corpus rows, each a count so far while a first backfill pass 1
// runs (section 12). None links yet, since the lenses they link to do not exist.
export function corpusValues(system: System): Value[] {
  const c = system.corpus;
  const sofar = indexing(system) ? " so far" : "";
  const n = (v: number) => `${count(v)}${sofar}`;
  return [
    ...Object.keys(auditAction).map((action) => ({
      key: `audit-${action}`,
      label: `${word(auditAction, action).text}, 24 hours`,
      text: n(c.audit_24h[action] ?? 0),
    })),
    { key: "messages", label: "Messages", text: n(c.messages) },
    { key: "threads", label: "Threads", text: n(c.threads) },
    { key: "unfiled", label: "Unfiled", text: `${n(c.unfiled)} (${share(c.unfiled, c.messages)})` },
    { key: "restricted", label: "Restricted messages", text: n(c.restricted) },
    { key: "masking", label: "Masking events, 7 days", text: n(c.masking_events_7d) },
    { key: "gate", label: "Gate skips, 7 days", text: n(c.gate_skips_7d) },
  ];
}

// Corpus is the corpus rows, or with backfill pass 1 not started, neither complete nor with a run, the
// sentence that the index is empty (section 8.1).
function Corpus(props: { system: System }) {
  const op = props.system.operational;
  if (!op.backfill_pass1_complete && op.backfill_pass1_run === null) {
    return <p class="muted">The index is empty. Backfill pass 1 has not started.</p>;
  }
  return <Values label="Corpus" values={corpusValues(props.system)} />;
}
