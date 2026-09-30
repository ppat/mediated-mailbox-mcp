// The fetch function the browser tests pass the app, answering each path from a response recorded from
// the real server (ADR-0064). It substitutes no behavior of the app's own, and a path no recording
// answers is kept in missing, which each test requires to be empty, so a test never passes on an
// answer it did not name.
import type { Fetch } from "../../src/app/api.ts";

// Answer is the recording that answers a path, a file in this directory, with the status the server
// sent it with.
export type Answer = { file: string; status: number };

export type Recorded = { fetch: Fetch; calls: string[]; missing: string[] };

// A path's answers are one recording, or recordings of the same path taken as the recorded state moved
// on, which answer its reads in order, the last answering every read after it.
export function recorded(answers: Readonly<Record<string, Answer | readonly Answer[]>>): Recorded {
  const calls: string[] = [];
  const missing: string[] = [];
  const reads = new Map<string, number>();
  const fetch: Fetch = async (path) => {
    calls.push(path);
    const given = answers[path];
    const n = reads.get(path) ?? 0;
    reads.set(path, n + 1);
    const answer = pick(given, n);
    if (answer === undefined) {
      missing.push(path);
      return new Response("", { status: 599 });
    }
    const body = await Bun.file(new URL(answer.file, import.meta.url)).text();
    return new Response(body, {
      status: answer.status,
      headers: { "Content-Type": "application/json" },
    });
  };
  return { fetch, calls, missing };
}

// pick is the answer to a path's nth read.
function pick(given: Answer | readonly Answer[] | undefined, n: number): Answer | undefined {
  if (given === undefined || "file" in given) {
    return given;
  }
  return given[Math.min(n, given.length - 1)];
}

export const ok = (file: string): Answer => ({ file, status: 200 });
