// The hand-written fetch over the read API, typed by the types generated from the contract (ADR-0065).
// Nothing here is ambient. Every read takes the fetch function it goes through as a parameter, which
// the composition root in main.ts binds to the browser's fetch and the tests to recorded fixtures
// (ADR-0040, ADR-0064).
import type { components, operations } from "../generated/contract.ts";

type Schemas = components["schemas"];

export type Accounts = Schemas["Accounts"];
export type Account = Schemas["Account"];
export type System = Schemas["System"];
export type Run = Schemas["Run"];
export type LensFigures = Schemas["LensFigures"];
export type Figure = Schemas["Figure"];
export type PlansPage = Schemas["PlansPage"];
export type CandidatesPage = Schemas["CandidatesPage"];
export type RunEvent = Schemas["RunEvent"];
export type Jobs = Schemas["Jobs"];
export type RunSummary = Schemas["RunSummary"];
export type RunsPage = Schemas["RunsPage"];
export type RunRow = Schemas["RunRow"];
export type FailuresPage = Schemas["FailuresPage"];
export type FailureRow = Schemas["FailureRow"];
export type FailureDetail = Schemas["FailureDetail"];
export type TimelineEvent = Schemas["TimelineEvent"];
export type RateEvent = Schemas["RateEvent"];
export type PlanEvent = Schemas["PlanEvent"];
export type Attention = Schemas["Attention"];
export type AttentionCard = Schemas["AttentionCard"];
export type CandidateRow = Schemas["CandidateRow"];
export type Signal = Schemas["Signal"];
export type Installation = Schemas["Installation"];
export type Client = Schemas["Client"];
export type InstallationAccount = Schemas["InstallationAccount"];
export type AttemptAnswer = Schemas["AttemptAnswer"];
export type ConsentAttempt = Schemas["ConsentAttempt"];
export type Connected = Schemas["Connected"];
export type ClientAnswer = Schemas["ClientAnswer"];
export type RemovedClient = Schemas["RemovedClient"];
export type AccountSettings = Schemas["AccountSettings"];
export type TargetAnswer = Schemas["TargetAnswer"];
export type AddClient = Schemas["AddClient"];
export type ReplaceClient = Schemas["ReplaceClient"];
export type ConnectRequest = Schemas["Connect"];
export type ReauthorizeRequest = Schemas["Reauthorize"];
export type RuleRow = Schemas["RuleRow"];
export type RuleDetail = Schemas["RuleDetail"];
export type ChangeRow = Schemas["ChangeRow"];
export type SenderRow = Schemas["SenderRow"];
export type MatchAnswer = Schemas["MatchAnswer"];
export type SuffixMatch = Schemas["SuffixMatch"];
export type RuleAnswer = Schemas["RuleAnswer"];
export type Problem = Schemas["Problem"];
export type BasePolicy = Schemas["BasePolicy"];
export type BaseRule = Schemas["BaseRule"];
export type BaseHistory = Schemas["BaseHistory"];
export type ImportPreview = Schemas["ImportPreview"];
export type Imported = Schemas["Imported"];
export type Counts = Schemas["Counts"];
export type BaseMatchAnswer = Schemas["BaseMatchAnswer"];
export type BaseSuffixMatch = Schemas["BaseSuffixMatch"];
export type LensAnswer = operations["getLens"]["responses"][200]["content"]["application/json"];
// Groups is a level 1 or 2 answer, every group of one dimension.
export type Groups = Extract<LensAnswer, { group: string }>;
export type RowsPage = Exclude<LensAnswer, LensFigures | Groups>;

// Fetch is the one way a read reaches the server, a path in and a response out. The signal aborts a
// request nobody waits for any more.
export type Fetch = (path: string, signal: AbortSignal) => Promise<Response>;

// Post is the one way a state-changing request reaches the server, a path and a JSON body in and a
// response out. The composition root binds it to the browser's fetch with the page's request token in
// its header (docs/UI.md section 15).
export type Post = (path: string, body: unknown) => Promise<Response>;

// Failure is a read that did not answer, in the error contract's shape (docs/UI.md section 17.3). A
// server that could not be reached, or answered with something other than the contract's error, is
// reported with the UI server as its origin, with the HTTP status 0 when there was no answer at all. A
// refused policy write or file carries each problem the server named beside the envelope.
export type Failure = Schemas["ErrorDetail"] & { status: number; problems?: Problem[] };

export type Result<T> = { ok: true; value: T } | { ok: false; failure: Failure };

// get reads one path and returns its body typed as the caller's read declares it. The body of a
// successful answer is trusted to be the contract's type, which the recorded fixtures' check against
// the contract document backs on the server side (ADR-0064).
async function get<T>(fetch: Fetch, path: string, signal: AbortSignal): Promise<Result<T>> {
  let response: Response;
  try {
    response = await fetch(path, signal);
  } catch {
    if (signal.aborted) {
      return {
        ok: false,
        failure: { ...unanswered(0, "the request was abandoned"), code: "aborted" },
      };
    }
    return { ok: false, failure: unanswered(0, "the UI server did not answer") };
  }
  return answered<T>(response);
}

// send sends one state-changing request and returns its answer typed as the caller declares it.
export async function send<T>(post: Post, path: string, body: unknown): Promise<Result<T>> {
  let response: Response;
  try {
    response = await post(path, body);
  } catch {
    return { ok: false, failure: unanswered(0, "the UI server did not answer") };
  }
  return answered<T>(response);
}

// answered reads a response's JSON body as the caller's type, or the error contract's failure.
async function answered<T>(response: Response): Promise<Result<T>> {
  // json() answers any, since JSON carries no type of its own. The generated types describe what a 200
  // carries.
  let body;
  try {
    body = await response.json();
  } catch {
    return {
      ok: false,
      failure: unanswered(
        response.status,
        `the UI server answered ${response.status} without JSON`,
      ),
    };
  }
  if (!response.ok) {
    return { ok: false, failure: failureOf(response.status, body) };
  }
  const value: T = body;
  return { ok: true, value };
}

function unanswered(status: number, message: string): Failure {
  return { origin: "ui", code: "no_answer", message, request_id: "", status };
}

// failureOf reads the error contract's envelope, and falls back to the UI server as the origin when
// the body is not one.
function failureOf(status: number, body: unknown): Failure {
  if (typeof body === "object" && body !== null && "error" in body) {
    const detail: unknown = body.error;
    if (typeof detail === "object" && detail !== null) {
      const origin = field(detail, "origin");
      if (
        origin === "client" ||
        origin === "ui" ||
        origin === "database" ||
        origin === "provider"
      ) {
        return {
          origin,
          code: field(detail, "code") ?? "",
          message: field(detail, "message") ?? "",
          request_id: field(detail, "request_id") ?? "",
          status,
          ...problemsOf(body),
        };
      }
    }
  }
  return unanswered(status, `the UI server answered ${status} outside the error contract`);
}

// problemsOf reads the problems a refused policy write or file names beside the error envelope
// (docs/UI.md section 17.4). Only the fields the contract declares are read from each.
function problemsOf(body: object): { problems?: Problem[] } {
  const listed: unknown = Reflect.get(body, "problems");
  if (!Array.isArray(listed)) {
    return {};
  }
  const problems: Problem[] = [];
  for (const p of listed) {
    if (typeof p !== "object" || p === null) {
      continue;
    }
    const kind = problemKind(field(p, "kind"));
    if (kind === undefined) {
      continue;
    }
    const line: unknown = Reflect.get(p, "line");
    problems.push({
      kind,
      rule_id: field(p, "rule_id") ?? "",
      line: typeof line === "number" ? line : null,
      suffix: field(p, "suffix") ?? null,
    });
  }
  return { problems };
}

const problemKinds = [
  "blank_identifier",
  "repeated_identifier",
  "no_suffix",
  "invalid_suffix",
  "other_class",
  "reserved_identifier",
  "not_the_form",
  "too_large",
] as const satisfies readonly Problem["kind"][];

function problemKind(kind: string | undefined): Problem["kind"] | undefined {
  return problemKinds.find((k) => k === kind);
}

function field(value: object, key: string): string | undefined {
  const v: unknown = Reflect.get(value, key);
  return typeof v === "string" ? v : undefined;
}

// The paths of the reads the app makes. The account is a path segment on every scoped read.
export function accountsPath(): string {
  return "/api/accounts";
}

export function systemPath(account: string): string {
  return `/api/${encodeURIComponent(account)}/system`;
}

export function attentionPath(account: string): string {
  return `/api/${encodeURIComponent(account)}/attention`;
}

export function eventsPath(account: string): string {
  return `/api/${encodeURIComponent(account)}/events`;
}

export function jobsPath(account: string): string {
  return `/api/${encodeURIComponent(account)}/jobs`;
}

export function runPath(account: string, run: string): string {
  return `/api/${encodeURIComponent(account)}/jobs/${encodeURIComponent(run)}`;
}

// failurePath is the failures dataset's row detail, one failure of a run.
export function failurePath(account: string, run: string, seq: string): string {
  return `/api/${encodeURIComponent(account)}/failures/${encodeURIComponent(seq)}?${new URLSearchParams({ run }).toString()}`;
}

// installationPath is the installation endpoint, which reads no account's state (section 17.4).
export function installationPath(): string {
  return "/api/setup";
}

// connectPath is the session's attempt to connect an account.
export function connectPath(): string {
  return "/api/setup/connect";
}

export function accountPath(account: string): string {
  return `/api/${encodeURIComponent(account)}/account`;
}

// reauthorizePath is the session's attempt to re-authorize or move an account.
export function reauthorizePath(account: string): string {
  return `/api/${encodeURIComponent(account)}/account/reauthorize`;
}

export function clientsPath(provider: string): string {
  return `/api/setup/${encodeURIComponent(provider)}/clients`;
}

export function clientPath(provider: string, client: string): string {
  return `${clientsPath(provider)}/${encodeURIComponent(client)}`;
}

// rulePath is one rule's row detail in an account's policy, the rules dataset's row named by the
// rule's scope and identifier (docs/UI.md section 8.7).
export function rulePath(account: string, scope: "base" | "account", rule: string): string {
  return `/api/${encodeURIComponent(account)}/rules/${encodeURIComponent(`${scope}:${rule}`)}`;
}

// matchPath is what each suffix typed in Add a rule matches in the account.
export function matchPath(account: string, suffixes: readonly string[]): string {
  return `/api/${encodeURIComponent(account)}/policy/match?${new URLSearchParams(suffixes.map((s) => ["suffix", s])).toString()}`;
}

// baseMatchPath is what each suffix typed in Add a base rule is, read with no account: its shape, whether
// it is a public suffix, and the base rules already matching it (docs/UI.md section 8.14).
export function baseMatchPath(suffixes: readonly string[]): string {
  return `/api/setup/policy/match?${new URLSearchParams(suffixes.map((s) => ["suffix", s])).toString()}`;
}

// releasePath is what keeping only keep of a rule's suffixes, or none for a whole lift, would release in
// the account, the senders the rule restricts there and no other rule does (docs/UI.md section 8.7).
export function releasePath(
  account: string,
  scope: "base" | "account",
  rule: string,
  keep: readonly string[],
): string {
  const params = new URLSearchParams([
    ["scope", scope],
    ["rule", rule],
    ...keep.map((k): [string, string] => ["keep", k]),
  ]);
  return `/api/${encodeURIComponent(account)}/policy/release?${params.toString()}`;
}

// policyApi is where a scope's policy writes go, an account's policy endpoints, which write either
// scope, or the installation's base policy endpoints when no account is in view.
export function policyApi(account: string | undefined): string {
  return account === undefined ? "/api/setup/policy" : `/api/${encodeURIComponent(account)}/policy`;
}

// basePolicyPath is the base policy screen's read, the base rules its search keeps.
export function basePolicyPath(search: string): string {
  return search === ""
    ? "/api/setup/policy"
    : `/api/setup/policy?${new URLSearchParams({ search }).toString()}`;
}

// baseHistoryPath is the base policy's history over a range, narrowed to one rule when rule is given.
export function baseHistoryPath(range: string, rule?: string): string {
  const params = new URLSearchParams({ range });
  if (rule !== undefined) {
    params.append("rule", rule);
  }
  return `/api/setup/policy/history?${params.toString().replaceAll("%2C", ",")}`;
}

// lensPath is the dataset endpoint with the query the URL grammar builds (src/app/url.ts).
export function lensPath(account: string, query: string): string {
  return `/api/${encodeURIComponent(account)}/lens?${query}`;
}

export function readAccounts(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<Accounts>> {
  return get<Accounts>(fetch, path, signal);
}

export function readSystem(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<System>> {
  return get<System>(fetch, path, signal);
}

export function readAttention(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<Attention>> {
  return get<Attention>(fetch, path, signal);
}

export function readJobs(fetch: Fetch, path: string, signal: AbortSignal): Promise<Result<Jobs>> {
  return get<Jobs>(fetch, path, signal);
}

export function readRun(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<RunSummary>> {
  return get<RunSummary>(fetch, path, signal);
}

export function readFailure(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<FailureDetail>> {
  return get<FailureDetail>(fetch, path, signal);
}

export function readLens(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<LensAnswer>> {
  return get<LensAnswer>(fetch, path, signal);
}

export function readInstallation(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<Installation>> {
  return get<Installation>(fetch, path, signal);
}

export function readAttempt(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<AttemptAnswer>> {
  return get<AttemptAnswer>(fetch, path, signal);
}

export function readAccount(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<AccountSettings>> {
  return get<AccountSettings>(fetch, path, signal);
}

export function readRule(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<RuleDetail>> {
  return get<RuleDetail>(fetch, path, signal);
}

export function readMatch(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<MatchAnswer>> {
  return get<MatchAnswer>(fetch, path, signal);
}

export function readBaseMatch(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<BaseMatchAnswer>> {
  return get<BaseMatchAnswer>(fetch, path, signal);
}

export function readRelease(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<Counts>> {
  return get<Counts>(fetch, path, signal);
}

export function readBasePolicy(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<BasePolicy>> {
  return get<BasePolicy>(fetch, path, signal);
}

export function readBaseHistory(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<BaseHistory>> {
  return get<BaseHistory>(fetch, path, signal);
}

// isFigures tells the dataset endpoint's summary answer from a page of rows or a set of groups.
export function isFigures(answer: LensAnswer): answer is LensFigures {
  return "figures" in answer;
}

// isGroups tells a set of groups from the summary and from a page of rows.
export function isGroups(answer: LensAnswer): answer is Groups {
  return "group" in answer;
}

// isRowsPage tells a page of rows from the summary and from a set of groups.
export function isRowsPage(answer: LensAnswer): answer is RowsPage {
  return "page" in answer;
}
