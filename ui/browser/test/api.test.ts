// The hand-written fetch and the data cache, over responses recorded from the real server.
import { afterEach, expect, test } from "bun:test";
import { accountsPath, lensPath, readAccounts, readLens } from "../src/app/api.ts";
import { Cache, freshness } from "../src/app/cache.ts";
import { ok, recorded, type Recorded } from "./fixtures/fetch.ts";

const signal = new AbortController().signal;

let server: Recorded | undefined;
afterEach(() => {
  expect(server?.missing ?? []).toEqual([]);
});

const unknownDataset = lensPath("personal", "dataset=messages");

function serve(): Recorded {
  server = recorded({
    [accountsPath()]: ok("accounts.json"),
    [unknownDataset]: { file: "error-unknown-dataset.json", status: 400 },
  });
  return server;
}

test("an answer is typed by the contract", async () => {
  const result = await readAccounts(serve().fetch, accountsPath(), signal);
  expect(result).toEqual({
    ok: true,
    value: {
      accounts: [
        { account_id: "other", provider: "gmail" },
        { account_id: "personal", provider: "gmail" },
      ],
    },
  });
});

test("a refusal carries the error contract's origin, code, message and request id", async () => {
  const result = await readLens(serve().fetch, unknownDataset, signal);
  expect(result).toEqual({
    ok: false,
    failure: {
      origin: "client",
      code: "unknown_dataset",
      message: 'the registry declares no dataset "messages"',
      request_id: "recorded",
      status: 400,
    },
  });
});

test("a server that does not answer is the UI server's failure", async () => {
  const result = await readAccounts(
    () => Promise.reject(new Error("refused")),
    accountsPath(),
    signal,
  );
  expect(result.ok).toBe(false);
  expect(result.ok ? undefined : result.failure.origin).toBe("ui");
});

test("the cache reuses a fresh answer, joins a request in flight and refetches a stale one", async () => {
  const s = serve();
  let now = 1_000;
  const cache = new Cache(
    (path, abort) => readAccounts(s.fetch, path, abort),
    () => now,
  );
  const first = cache.read(accountsPath());
  expect(cache.read(accountsPath())).toBe(first);
  expect(first.value.status).toBe("loading");
  await cache.refresh(accountsPath());
  expect(s.calls).toEqual([accountsPath()]);
  expect(first.value.status).toBe("ok");

  now += freshness - 1;
  cache.read(accountsPath());
  expect(s.calls.length).toBe(1);

  now += 1;
  cache.read(accountsPath());
  await cache.refresh(accountsPath());
  expect(s.calls.length).toBe(2);
  expect(first.value.status).toBe("ok");
});

test("a failed read keeps its failure until it is retried", async () => {
  const s = serve();
  const cache = new Cache(
    (path, abort) => readLens(s.fetch, path, abort),
    () => 0,
  );
  const state = cache.read(unknownDataset);
  await cache.refresh(unknownDataset);
  expect(state.value.status).toBe("error");
  cache.read(unknownDataset);
  expect(s.calls.length).toBe(1);
  const retry = cache.refresh(unknownDataset);
  expect(state.value.status).toBe("loading");
  await retry;
  expect(s.calls.length).toBe(2);
});
