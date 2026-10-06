// The run screen of docs/UI.md section 8.4. The run's fields and its resumer's state are bound to the
// objects the stream replaces, so an event redraws their text and the screen never re-runs. The failure
// counts and the timeline read the run summary endpoint, and the breakdown and the failed items read the
// failures dataset with this run as parent, and each run event of the run or its resumer reads them
// again, since the stream carries neither (section 9). One failure's detail is a panel over the items.
import { useComputed, type ReadonlySignal } from "@preact/signals";
import {
  failurePath,
  isGroups,
  isRowsPage,
  lensPath,
  runPath,
  systemPath,
  type FailureDetail,
  type FailureRow,
  type Run,
  type RunEvent,
  type RunSummary,
} from "../app/api.ts";
import { useDeps, type Deps } from "../app/deps.ts";
import { count, utc } from "../app/format.ts";
import { Live, LiveText, type Stream } from "../app/live.tsx";
import { Region } from "../app/region.tsx";
import { apiQuery, href, serialize, withPage, type Parent, type View } from "../app/url.ts";
import {
  auditAction,
  contentFlag,
  disposition,
  errorClass,
  scanStateLong,
  senderClass,
  word,
} from "../app/wording.ts";
import { Breadcrumb } from "../lens/breadcrumb.tsx";
import { GroupBy } from "../lens/groupby.tsx";
import { GroupsView } from "../lens/groups.tsx";
import { useChipEscape } from "../lens/lens.tsx";
import { DetailPanel } from "../lens/panel.tsx";
import { RowsTable } from "../lens/table.tsx";
import { Timeline } from "../lens/timeline.tsx";
import { failureColumns, pageRow } from "../row/failure.tsx";
import { elapsed, progressOf, RunState, runHref } from "./jobs.tsx";
import { addScreen, policyHome } from "./policywrites.tsx";

// runScreen is the screen's path below the account, where the failures view's links lead.
export function runScreen(run: string): string {
  return `jobs/${encodeURIComponent(run)}`;
}

// itemsView is the failed items' read, the view's filters, sort and page at level 3.
export function itemsView(view: View): View {
  return { ...view, level: "3", group: undefined, page: view.page ?? "1" };
}

// dispositionView is the small breakdown's read, the view's filters grouped by disposition.
export function dispositionView(view: View): View {
  return { ...view, level: "1", group: "disposition", page: undefined };
}

// failureHref is one failure's panel over the items, keeping the view.
export function failureHref(account: string, run: string, seq: number, view: View): string {
  return `/${encodeURIComponent(account)}/${runScreen(run)}/failures/${seq}?${serialize(view)}`;
}

// runReads are the reads a run event of the run or its resumer reads again.
export function runReads(account: string, run: string, view: View): string[] {
  const parent: Parent = { name: "run", value: run };
  return [
    runPath(account, run),
    lensPath(account, apiQuery(view, parent)),
    lensPath(account, apiQuery(dispositionView(view), parent)),
    lensPath(account, apiQuery(itemsView(view), parent)),
  ];
}

// onRunEvent is what a run event does to the run screen. An event of the run, of its resumer, or of a
// run that resumes it reads the summary, the breakdown and the items again. A state that differs from
// the one last seen or shown also reads the system endpoint again, for the chrome's running mark
// (section 6).
export function onRunEvent(
  deps: Deps,
  account: string,
  run: string,
  view: View,
  seen: Map<string, string>,
): (event: RunEvent) => void {
  return (event) => {
    const summary = deps.runs.read(runPath(account, run)).peek();
    const resumer = summary.status === "ok" ? summary.answer.resumer?.run_id : undefined;
    if (event.run_id === run || event.run_id === resumer || event.resumed_from === run) {
      const [summaryPath, ...lensPaths] = runReads(account, run, view);
      if (summaryPath !== undefined) {
        void deps.runs.refresh(summaryPath);
      }
      // The breakdown and the items are read only once the summary counts a failure, so a run with
      // none has nothing of theirs to read again until the refreshed summary shows some.
      if (summary.status === "ok" && summary.answer.failures.count > 0) {
        for (const p of lensPaths) {
          void deps.lens.refresh(p);
        }
      }
    }
    const shownState =
      summary.status !== "ok"
        ? undefined
        : [summary.answer.run, summary.answer.resumer].find((r) => r?.run_id === event.run_id)
            ?.state;
    if ((seen.get(event.run_id) ?? shownState) !== event.state) {
      seen.set(event.run_id, event.state);
      void deps.system.refresh(systemPath(account));
    }
  };
}

type RunProps = { account: string; run: string; view: View; live: Stream; panelOpen: boolean };

export function RunScreen(props: RunProps) {
  const { account, run, view, live } = props;
  const { runs } = useDeps();
  const path = runPath(account, run);
  const summary = runs.read(path);
  return (
    <div class="run">
      <RunHead summary={summary} live={live} />
      <Region name="run" state={summary} shape="block" retry={() => void runs.retry(path)}>
        {(answer) => (
          <>
            <RunCounts account={account} summary={answer} live={live} />
            <Timeline
              events={answer.events}
              startedAt={answer.run.started_at}
              endedAt={answer.run.finished_at ?? answer.as_of}
            />
            {answer.failures.count === 0 ? (
              <p class="muted">no failures</p>
            ) : (
              <Failures account={account} run={run} view={view} panelOpen={props.panelOpen} />
            )}
          </>
        )}
      </Region>
    </div>
  );
}

// RunHead is the L0 strip's run fields, each a computed text of the run's latest event, or of the
// summary's run before any event. It reads no signal while rendering, so it renders once.
function RunHead(props: {
  summary: ReadonlySignal<ReturnType<Deps["runs"]["read"]>["value"]>;
  live: Stream;
}) {
  const { summary, live } = props;
  const shown = useComputed<Run | undefined>(() => {
    const state = summary.value;
    if (state.status !== "ok") {
      return undefined;
    }
    return live.objects.run(state.answer.run.run_id).value ?? state.answer.run;
  });
  const text = (f: (r: Run, now: number) => string) => (
    <LiveText
      value={shown}
      fallback={undefined}
      text={(r, now) => (r === undefined ? "" : f(r, now))}
      clock={live.clock}
    />
  );
  return (
    <section class="strip" aria-label="Run">
      <span class="figure">
        <span class="figure-value">
          {text((r) => `${r.workload}${r.pass === null ? "" : ` ${r.pass}`}`)}
        </span>
        <span class="label mono">{text((r) => r.run_id)}</span>
      </span>
      <span class="figure">
        <span class="figure-value">{text((r) => r.state)}</span>
        <span class="label">state</span>
      </span>
      <span class="figure">
        <span class="figure-value">{text((r) => utc(r.started_at))}</span>
        <span class="label">started</span>
      </span>
      <span class="figure">
        <span class="figure-value">
          {text((r) => (r.finished_at === null ? "not yet" : utc(r.finished_at)))}
        </span>
        <span class="label">finished</span>
      </span>
      <span class="figure">
        <span class="figure-value">{text((r, now) => elapsed(r, now))}</span>
        <span class="label">duration</span>
      </span>
      <span class="figure">
        <span class="figure-value">{text((r) => progressOf(r) || "none")}</span>
        <span class="label">checkpoint</span>
      </span>
    </section>
  );
}

// RunCounts is the rest of the L0 strip, from the run summary. Where it failed is the checkpoint the
// run stopped at and how many retries it recorded, item failures, how many recovered and by which run,
// how many were not found at the provider, and the resuming run with its state.
function RunCounts(props: { account: string; summary: RunSummary; live: Stream }) {
  const { account, summary, live } = props;
  const retries = summary.events.filter((e) => e.kind === "retry").length;
  const gone = summary.failures.dispositions["gone"] ?? 0;
  const recovered = summary.failures.dispositions["recovered"] ?? 0;
  return (
    <section class="strip" aria-label="Failures of the run">
      {summary.run.state === "failed" ? (
        <span class="figure">
          <span class="figure-value">
            {progressOf(summary.run) || "no checkpoint"}, {count(retries)}{" "}
            {retries === 1 ? "retry" : "retries"}
          </span>
          <span class="label">where it failed</span>
        </span>
      ) : null}
      <span class="figure">
        <span class="figure-value">{count(summary.failures.count)}</span>
        <span class="label">item failures</span>
      </span>
      <span class="figure">
        <span class="figure-value">{count(recovered)}</span>
        <span class="label">
          recovered
          {summary.recovered_by.map((r) => (
            <span key={r.run_id}>
              {" by "}
              <a class="mono" href={runHref(account, r.run_id)}>
                {r.run_id}
              </a>{" "}
              ({count(r.recovered)})
            </span>
          ))}
        </span>
      </span>
      <span class="figure">
        <span class="figure-value">{count(gone)}</span>
        <span class="label">not found at the provider</span>
      </span>
      <span class="figure">
        <span class="figure-value">
          {summary.resumer === null ? (
            "none"
          ) : (
            <>
              <a class="mono" href={runHref(account, summary.resumer.run_id)}>
                {summary.resumer.run_id}
              </a>{" "}
              <RunState live={live} run={summary.resumer} />
            </>
          )}
        </span>
        <span class="label">resumed by</span>
      </span>
    </section>
  );
}

// Failures is the breakdown and the failed items, sharing the view's filters (section 8.4).
function Failures(props: { account: string; run: string; view: View; panelOpen: boolean }) {
  const { account, run, view } = props;
  useChipEscape(account, view, runScreen(run), props.panelOpen);
  const { lens } = useDeps();
  const parent: Parent = { name: "run", value: run };
  const screen = runScreen(run);
  const groupsPath = lensPath(account, apiQuery(view, parent));
  const dispositionPath = lensPath(account, apiQuery(dispositionView(view), parent));
  const items = itemsView(view);
  const itemsPath = lensPath(account, apiQuery(items, parent));
  const detail = (row: FailureRow) => failureHref(account, run, row.seq, view);
  return (
    <>
      <div class="crumbs">
        <Breadcrumb account={account} name={`run ${run}`} view={view} screen={screen} />
        <GroupBy account={account} view={view} screen={screen} />
      </div>
      <div class="breakdown">
        <Region
          name="breakdown"
          state={lens.read(groupsPath)}
          shape="chart"
          retry={() => void lens.retry(groupsPath)}
        >
          {(answer) =>
            isGroups(answer) ? (
              <GroupsView
                account={account}
                view={view}
                answer={answer}
                name="Failures"
                screen={screen}
                stay
                paged={false}
                keys={false}
              />
            ) : null
          }
        </Region>
        {view.group === "disposition" ? null : (
          <Region
            name="dispositions"
            state={lens.read(dispositionPath)}
            shape="chart"
            retry={() => void lens.retry(dispositionPath)}
          >
            {(answer) =>
              isGroups(answer) ? (
                <GroupsView
                  account={account}
                  view={dispositionView(view)}
                  answer={answer}
                  name="Failures by disposition"
                  screen={screen}
                  stay
                  paged={false}
                  table={false}
                />
              ) : null
            }
          </Region>
        )}
      </div>
      <Region
        name="failures"
        state={lens.read(itemsPath)}
        shape="table"
        retry={() => void lens.retry(itemsPath)}
      >
        {(answer) =>
          isRowsPage(answer) && answer.dataset === "failures" ? (
            <RowsTable
              caption={`Failed items, ${count(answer.total.count)} in all`}
              rowsName="failed items"
              columns={failureColumns(detail, (id) => runHref(account, id))}
              span={pageRow(detail)}
              rowKey={(row) => row.seq}
              rows={answer.rows}
              open={detail}
              page={answer.page}
              pages={answer.pages}
              pageHref={(p) => href(account, withPage(view, p), screen)}
            />
          ) : null
        }
      </Region>
    </>
  );
}

// meaning is the sentence for a failure's disposition (docs/UI.md section 8.4).
export function meaning(row: FailureRow): string {
  const cls = word(errorClass, row.error_class).text;
  switch (row.disposition) {
    case "recovered":
      return `${cls} on this item. Run ${row.recovered_by ?? "unknown"} retried it successfully. Its scan state is ${row.scan_state === null ? "unknown" : word(scanStateLong, row.scan_state).text}.`;
    case "pending":
      return `${cls} on this item. It has not been retried yet. Its scan state stays pending, so its body is denied until a run reaches it.`;
    case "gone":
      return `${cls}. ${row.scan_state === null ? "The index no longer holds it." : `Its scan state is ${word(scanStateLong, row.scan_state).text}.`}`;
    case "abandoned":
      return `${cls} on this item after ${row.attempts} attempts. The workload gave up. Its scan state stays pending, so its body is denied until a later run reaches it.`;
    default:
      return `${cls} on this item, with the disposition ${row.disposition}, which this screen does not know.`;
  }
}

export function FailurePanel(props: { account: string; run: string; seq: string; view: View }) {
  const { account, run, seq, view } = props;
  const { failures } = useDeps();
  const path = failurePath(account, run, seq);
  const back = `/${encodeURIComponent(account)}/${runScreen(run)}?${serialize(view)}`;
  return (
    <DetailPanel title={`Failure ${seq} of run ${run}`} back={back}>
      <Region
        name="failure"
        state={failures.read(path)}
        shape="block"
        retry={() => void failures.retry(path)}
      >
        {(answer) => <FailureBody account={account} detail={answer} />}
      </Region>
    </DetailPanel>
  );
}

// senderDomain is the domain of a sender's address, what follows its last @, lowercased, which a
// restriction of the sender names. The failure's detail carries the address and not the stored domain.
export function senderDomain(address: string | null): string | undefined {
  if (address === null || !address.includes("@")) {
    return undefined;
  }
  const domain = address.slice(address.lastIndexOf("@") + 1).toLowerCase();
  return domain === "" ? undefined : domain;
}

// FailureBody is a failure's detail. The rule that set the message's class links to the account's
// policy searched for its identifier, which lists that identifier's rule in each scope that holds one,
// and a sender whose class reads normal carries Restrict {domain}… (docs/UI.md section 7.1).
export function FailureBody(props: { account: string; detail: FailureDetail }) {
  const { account, detail } = props;
  const row = detail.row;
  const domain = senderDomain(row.from_email);
  return (
    <>
      <h3>What happened</h3>
      <p>
        The run recorded: <span class="recorded">{detail.error_summary ?? "no summary"}</span>
      </p>
      <h3>What it means</h3>
      <p>{meaning(row)}</p>
      <h3>Provenance</h3>
      <dl class="provenance">
        <dt>run</dt>
        <dd class="mono">{detail.run}</dd>
        <dt>item</dt>
        <dd>
          {row.item_kind} <span class="mono">{row.item_id}</span>
        </dd>
        <dt>subject</dt>
        <dd>{row.subject ?? "not in the index"}</dd>
        <dt>sender</dt>
        <dd class="mono">{row.from_email ?? ""}</dd>
        <dt>page</dt>
        <dd>{row.page ?? "none"}</dd>
        <dt>attempts</dt>
        <dd>{row.attempts}</dd>
        <dt>first error</dt>
        <dd>{utc(row.first_at)}</dd>
        <dt>last error</dt>
        <dd>{utc(row.last_at)}</dd>
        <dt>sender class</dt>
        <dd>
          {row.sender_class === null ? "" : word(senderClass, row.sender_class).text}
          {row.sender_class === "normal" && domain !== undefined ? (
            <>
              {" "}
              <a href={addScreen(account, { suffixes: [domain] })}>Restrict {domain}…</a>
            </>
          ) : null}
        </dd>
        <dt>rule that set the class</dt>
        <dd class="mono">
          {detail.class_rule_id === null ? (
            "none"
          ) : (
            <a
              href={`${policyHome(account)}?${new URLSearchParams({ search: detail.class_rule_id }).toString()}`}
            >
              {detail.class_rule_id}
            </a>
          )}
        </dd>
        <dt>content flags</dt>
        <dd>{(row.content_flags ?? []).map((f) => word(contentFlag, f).text).join(", ")}</dd>
        <dt>rule ids that fired</dt>
        <dd class="mono">{(detail.rule_ids ?? []).join(", ") || "none"}</dd>
        <dt>scanned</dt>
        <dd>
          {detail.scanned_at === null ? "never" : utc(detail.scanned_at)}
          {detail.scanner_version === null ? null : `, scanner version ${detail.scanner_version}`}
        </dd>
        <dt>disposition</dt>
        <dd>{word(disposition, row.disposition).text}</dd>
        <dt>error summary</dt>
        <dd>{detail.error_summary ?? ""}</dd>
      </dl>
      <h3>Audit rows of the message</h3>
      {detail.audit.length === 0 ? (
        <p class="muted">No audit row.</p>
      ) : (
        <>
          <ul class="audit">
            {detail.audit.map((a) => (
              <li key={a.id}>
                {utc(a.at)} <span class="mono">{a.actor}</span> {word(auditAction, a.action).text}
              </li>
            ))}
          </ul>
          <p class="muted">
            {count(detail.audit.length)} of {count(detail.audit_count)} shown, newest first.
          </p>
        </>
      )}
    </>
  );
}

// RunLive is the run screen's live indicator, shown while the run or its resumer runs, by their latest
// events or, before any, by the run summary. The screen follows the stream whenever it is open, so a run
// that resumes this one is seen when it starts.
export function RunLive(props: { account: string; run: string; live: Stream }) {
  const { runs } = useDeps();
  const summary = runs.read(runPath(props.account, props.run));
  const running = useComputed(() => {
    const state = summary.value;
    if (state.status !== "ok") {
      return false;
    }
    const { run, resumer } = state.answer;
    const latest = (r: Run) => props.live.objects.run(r.run_id).value ?? r;
    return (
      latest(run).state === "running" || (resumer !== null && latest(resumer).state === "running")
    );
  });
  return <Shown when={running} stream={props.live} />;
}

// Shown is the live indicator while when holds. It re-renders only when when changes.
function Shown(props: { when: ReadonlySignal<boolean>; stream: Stream }) {
  return props.when.value ? <Live stream={props.stream} /> : null;
}
