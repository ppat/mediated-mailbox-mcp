// The fetch function the browser tests pass the app, answering each path from a response recorded from
// the real server (ADR-0064). It substitutes no behavior of the app's own, and a path no recording
// answers is kept in missing, which each test requires to be empty, so a test never passes on an
// answer it did not name.
import { readFileSync } from "node:fs";
import type { Fetch, Post } from "../../src/app/api.ts";

// Answer is the recording that answers a path, a file in this directory, with the status the server
// sent it with.
export type Answer = { file: string; status: number };

// A state-changing request's answers are keyed by "POST " and the path, and each request it sent is
// kept in posted with its body, so a test asserts what the app sent.
export type Recorded = {
  fetch: Fetch;
  post: Post;
  calls: string[];
  posted: { path: string; body: unknown }[];
  missing: string[];
};

// Loaded is an answer with its recording read.
type Loaded = { body: string; status: number };

// A path's answers are one recording, or recordings of the same path taken as the recorded state moved
// on, which answer its reads in order, the last answering every read after it.
//
// Every recording is read from disk here, when the answers are given, so a read answers from memory
// within the microtasks that follow it and never waits on the disk. settle (test/render.ts) relies on
// that, since it turns the task queue once per round and a read still on the disk after that turn
// would land after the test's assertions, which is the race a loaded machine loses. A recording that
// does not exist fails the test where its answers are given.
export function recorded(answers: Readonly<Record<string, Answer | readonly Answer[]>>): Recorded {
  const loaded = new Map<string, readonly Loaded[]>();
  for (const [path, given] of Object.entries(answers)) {
    loaded.set(
      path,
      ("file" in given ? [given] : given).map((answer) => ({
        body: readFileSync(new URL(answer.file, import.meta.url), "utf8"),
        status: answer.status,
      })),
    );
  }
  const calls: string[] = [];
  const missing: string[] = [];
  const posted: { path: string; body: unknown }[] = [];
  const reads = new Map<string, number>();
  const answer = (key: string): Response => {
    const n = reads.get(key) ?? 0;
    reads.set(key, n + 1);
    const given = loaded.get(key);
    const recording = given?.[Math.min(n, given.length - 1)];
    if (recording === undefined) {
      missing.push(key);
      return new Response("", { status: 599 });
    }
    return new Response(recording.body, {
      status: recording.status,
      headers: { "Content-Type": "application/json" },
    });
  };
  const fetch: Fetch = async (path) => {
    calls.push(path);
    return answer(path);
  };
  const post: Post = async (path, body) => {
    calls.push(`POST ${path}`);
    posted.push({ path, body });
    return answer(`POST ${path}`);
  };
  return { fetch, post, calls, posted, missing };
}

export const ok = (file: string): Answer => ({ file, status: 200 });
