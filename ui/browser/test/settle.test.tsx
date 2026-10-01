// How a browser test settles. Nothing settle waits on is real I/O or a real frame: the recorded fetch
// answers from memory, mount runs the first render's effects, and settle turns the task queue until a
// round renders nothing.
import { afterEach, expect, test } from "bun:test";
import { useEffect, useState } from "preact/hooks";
import { accountsPath } from "../src/app/api.ts";
import { ok, recorded } from "./fixtures/fetch.ts";
import { mount, settle, type Mounted } from "./render.ts";

let mounted: Mounted | undefined;
afterEach(() => {
  mounted?.unmount();
  mounted = undefined;
});

// A read answered from memory lands within a few microtasks. A thousand leaves room for any change in
// how many the response's body takes, and is far short of a task's turn, which a read from the disk
// waits for.
test("the recorded fetch answers a read within microtasks, without waiting for a task", async () => {
  const server = recorded({ [accountsPath()]: ok("accounts.json") });
  let answered = false;
  void server
    .fetch(accountsPath(), new AbortController().signal)
    .then((response) => response.json())
    .then(() => {
      answered = true;
    });
  for (let i = 0; i < 1_000; i++) {
    if (answered) {
      break;
    }
    await Promise.resolve();
  }
  expect(answered).toBe(true);
  expect(server.missing).toEqual([]);
});

test("mount runs the effects of the first render before it returns", () => {
  let ran = false;
  function Effect() {
    useEffect(() => {
      ran = true;
    }, []);
    return null;
  }
  mounted = mount(<Effect />);
  expect(ran).toBe(true);
});

// Chain answers each step after its render's effect runs, as a read an effect starts answers, so each
// step takes one round of settle.
function Chain(props: { steps: number }) {
  const [step, setStep] = useState(0);
  useEffect(() => {
    if (step < props.steps) {
      void Promise.resolve().then(() => setStep(step + 1));
    }
  }, [step, props.steps]);
  return <p>{step}</p>;
}

test("settle waits out a chain of reads however many rounds it takes", async () => {
  mounted = mount(<Chain steps={20} />);
  await settle();
  expect(mounted.root.textContent).toBe("20");
});

test("settle fails on a tree that never stops rendering", async () => {
  mounted = mount(<Chain steps={Number.POSITIVE_INFINITY} />);
  const failure = await settle().then(
    () => "settled",
    (error: unknown) => (error instanceof Error ? error.message : String(error)),
  );
  expect(failure).toBe("settle: the tree still renders after 50 rounds");
});
