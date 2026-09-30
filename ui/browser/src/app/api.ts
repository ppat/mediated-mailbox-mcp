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
export type RateEvent = Schemas["RateEvent"];
export type PlanEvent = Schemas["PlanEvent"];
export type LensAnswer = operations["getLens"]["responses"][200]["content"]["application/json"];
export type RowsPage = Exclude<LensAnswer, LensFigures>;

// Fetch is the one way a read reaches the server, a path in and a response out. The signal aborts a
// request nobody waits for any more.
export type Fetch = (path: string, signal: AbortSignal) => Promise<Response>;

// Failure is a read that did not answer, in the error contract's shape (docs/UI.md section 17.3). A
// server that could not be reached, or answered with something other than the contract's error, is
// reported with the UI server as its origin, with the HTTP status 0 when there was no answer at all.
export type Failure = Schemas["ErrorDetail"] & { status: number };

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
      if (origin === "client" || origin === "ui" || origin === "database") {
        return {
          origin,
          code: field(detail, "code") ?? "",
          message: field(detail, "message") ?? "",
          request_id: field(detail, "request_id") ?? "",
          status,
        };
      }
    }
  }
  return unanswered(status, `the UI server answered ${status} outside the error contract`);
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

export function eventsPath(account: string): string {
  return `/api/${encodeURIComponent(account)}/events`;
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

export function readLens(
  fetch: Fetch,
  path: string,
  signal: AbortSignal,
): Promise<Result<LensAnswer>> {
  return get<LensAnswer>(fetch, path, signal);
}

// isFigures tells the dataset endpoint's summary answer from a page of rows.
export function isFigures(answer: LensAnswer): answer is LensFigures {
  return "figures" in answer;
}
