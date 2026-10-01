// The system screen of docs/UI.md section 8.8, read-only. It shows the account's identifier and
// provider and the system endpoint's operational block, one row per value. A row links where Home's
// operational row links once the screen it links to exists, and until then renders unlinked, so the
// rows UI.md sends to Jobs link there and the two it sends to the corpus lens do not yet. It is not a
// live surface. It reads the system endpoint through the same cached
// read as the chrome's partial-index banner, so a page load sends one request, and while the banner
// follows the stream the values here move with it.
import type { ComponentChildren } from "preact";
import { systemPath, type Run, type System } from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { ListedAccount, numberIn } from "../app/frame.tsx";
import { age, count, local, rate, share, utc } from "../app/format.ts";
import { Region } from "../app/region.tsx";
import { ProgressBar } from "../lens/progress.tsx";

// Value is one row of the screen, its label and its value as text. The text is rendered as text and
// never as markup, since the authentication outcome is text the provider adapter records (section 11).
export type Value = {
  key: string;
  label: string;
  text: string;
  mono?: boolean;
  // note follows the text in muted type.
  note?: string;
  // title is the hover, the UTC time of an age or the local time of a UTC time.
  title?: string;
  progress?: { part: number; whole: number };
  // href is the screen the value links to, where one exists.
  href?: string;
};

// jobsHref is the jobs screen of an account, where the rows of the backfill run, delta sync and the
// rate link (docs/UI.md section 8.8).
export function jobsHref(account: string): string {
  return `/${encodeURIComponent(account)}/jobs`;
}

// pass is a backfill pass's value, complete, or running with its page of pages while its latest run
// runs, else not complete.
export function pass(key: string, label: string, complete: boolean, run: Run | null): Value {
  if (complete) {
    return { key, label, text: "complete" };
  }
  if (run?.state !== "running") {
    return { key, label, text: "not complete" };
  }
  const page = numberIn(run.checkpoint, "page");
  const of = numberIn(run.checkpoint, "of");
  if (page === undefined || of === undefined) {
    return { key, label, text: "running" };
  }
  return {
    key,
    label,
    text: `running, page ${count(page)} of ${count(of)} (${share(page, of)})`,
    progress: { part: page, whole: of },
  };
}

// backoff is the backoff wording of section 11, derived from backoff_until. A time already past is
// not a backoff.
export function backoff(until: string | null | undefined, now: number): string {
  return until === null || until === undefined || Date.parse(until) <= now
    ? "not in backoff"
    : `in backoff until ${utc(until)}`;
}

// systemValues are the screen's rows, in the order section 8.8 gives, for one system answer.
export function systemValues(system: System, now: number): Value[] {
  const op = system.operational;
  const r = op.rate;
  const jobs = jobsHref(system.account);
  const toJobs = (v: Value): Value => ({ ...v, href: jobs });
  return [
    {
      key: "account",
      label: "Account",
      text: system.account,
      mono: true,
      note: op.connected ? system.provider : `${system.provider}, not connected`,
    },
    toJobs(pass("pass1", "Backfill pass 1", op.backfill_pass1_complete, op.backfill_pass1_run)),
    pass("pass2", "Backfill pass 2", op.backfill_pass2_complete, op.backfill_pass2_run),
    toJobs(
      op.sync_cursor_at === null
        ? { key: "cursor", label: "Sync cursor age", text: "no cursor yet" }
        : {
            key: "cursor",
            label: "Sync cursor age",
            text: age(op.sync_cursor_at, now),
            title: utc(op.sync_cursor_at),
          },
    ),
    toJobs(
      op.last_successful_tick_at === null
        ? { key: "tick", label: "Last successful tick", text: "none yet" }
        : {
            key: "tick",
            label: "Last successful tick",
            text: utc(op.last_successful_tick_at),
            title: local(op.last_successful_tick_at),
          },
    ),
    {
      key: "backlog",
      label: "Scan backlog",
      text: `${count(op.pending_scan)} ${op.pending_scan === 1 ? "message" : "messages"} pending scan`,
    },
    toJobs({
      key: "rate",
      label: "Rate",
      text:
        r === null
          ? "no rate state yet"
          : `${rate(r.current, r.target)}, cap ${r.cap.toFixed(1)} units/s`,
    }),
    toJobs({ key: "backoff", label: "Backoff", text: backoff(r?.backoff_until, now) }),
    toJobs(
      r === null || r.last_throttle_at === null
        ? { key: "throttle", label: "Last throttle", text: "never" }
        : {
            key: "throttle",
            label: "Last throttle",
            text: utc(r.last_throttle_at),
            title: local(r.last_throttle_at),
          },
    ),
    authentication(op.last_auth_outcome, op.last_auth_at),
  ];
}

// authentication is the last provider authentication, its outcome as recorded then its time. It never
// links, because Home's row links to this screen.
function authentication(outcome: string | null, at: string | null): Value {
  const value: Value = {
    key: "auth",
    label: "Last authentication",
    text: outcome ?? "none recorded",
  };
  return at === null ? value : { ...value, note: utc(at), title: local(at) };
}

// Values is a list of rows, each a label and its value. The value's text sits in its own element,
// followed by its note and its progress bar.
export function Values(props: { label: string; values: readonly Value[] }) {
  return (
    <dl class="values" aria-label={props.label}>
      {props.values.map((v) => [
        <dt key={`${v.key}-label`}>{v.label}</dt>,
        <dd key={`${v.key}-value`}>
          <Linked href={v.href}>
            <span class={v.mono === true ? "value mono" : "value"} title={v.title}>
              {v.text}
            </span>
          </Linked>
          {v.note === undefined ? null : <span class="muted"> · {v.note}</span>}
          {v.progress === undefined ? null : (
            <ProgressBar
              part={v.progress.part}
              whole={v.progress.whole}
              state="running"
              label={`${v.label} progress`}
            />
          )}
        </dd>,
      ])}
    </dl>
  );
}

// Linked wraps a value in its link where it has one.
function Linked(props: { href: string | undefined; children: ComponentChildren }) {
  return props.href === undefined ? (
    <>{props.children}</>
  ) : (
    <a href={props.href}>{props.children}</a>
  );
}

export function SystemScreen(props: { account: string }) {
  return (
    <section aria-labelledby="system-title">
      <h1 id="system-title">System</h1>
      <ListedAccount account={props.account}>
        <SystemValues account={props.account} />
      </ListedAccount>
    </section>
  );
}

// SystemValues reads the system endpoint through the cache the partial-index banner reads, so both
// share one request and one answer.
function SystemValues(props: { account: string }) {
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
          <Values label="Operational state" values={systemValues(answer, now())} />
        </>
      )}
    </Region>
  );
}
