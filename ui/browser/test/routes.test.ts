// The rules about screen paths of docs/UI.md sections 5 and 6.
import { expect, test } from "bun:test";
import { toChildArray, type ComponentChildren, type VNode } from "preact";
import { entryAccount, switchAccount } from "../src/app/routes.ts";
import { isScreen, Routes } from "../src/app/router.tsx";

const accounts = [
  { account_id: "personal", provider: "gmail" },
  { account_id: "other", provider: "gmail" },
];

test("the entry route takes the account last used while it is listed, else the first by identifier", () => {
  expect(entryAccount(accounts, "personal")).toBe("personal");
  expect(entryAccount(accounts, "gone")).toBe("other");
  expect(entryAccount(accounts, undefined)).toBe("other");
  expect(entryAccount([], "personal")).toBeUndefined();
});

test("switching account keeps a screen and its level and drops its filters and objects", () => {
  expect(switchAccount("/personal", "", "other")).toBe("/other");
  expect(switchAccount("/personal/plans", "?level=3&status=DRAFT&page=2", "other")).toBe(
    "/other/plans?level=3",
  );
  expect(switchAccount("/personal/plans/7f3a9c00", "?section=flows", "other")).toBe("/other/plans");
  expect(switchAccount("/personal/jobs/r-0913", "?level=1", "other")).toBe("/other/jobs?level=1");
  expect(switchAccount("/personal/messages", "", "a/b")).toBe("/a%2Fb/messages");
});

// paths is every path the route table matches, the not-found route aside. The router reads its routes
// from its children through toChildArray, as this does, so no route reaches the router that is not here.
function paths(): string[] {
  const table: VNode<{ children?: ComponentChildren }> = Routes();
  return toChildArray(table.props.children)
    .filter((r): r is VNode<{ path?: string }> => typeof r === "object")
    .flatMap((r) => (r.props.path === undefined ? [] : [r.props.path]));
}

// param stands for every parameter segment, whatever its name and flag, so a route reads the same
// however its path names its parameters.
const param = ":";

// segments splits a path as the router's matcher does, ignoring empty segments, so a missing or doubled
// slash changes nothing, and reads every parameter segment as param. A literal keeps its flag, since a
// literal with a flag matches more than its word.
function segments(path: string): string[] {
  return path
    .split("/")
    .filter(Boolean)
    .map((s) => (s.startsWith(":") ? param : s));
}

// wordsAt is every literal word a route path holds right after the segments of prefix, where an account
// or a rule identifier also sits in another route. A literal of the prefix matches with or without a
// flag, as the router's matcher compares a literal without its flag. A route putting a parameter where
// the prefix has a literal, such as /:account/:section/export, is not read here and is left to review
// (docs/UI.md section 5).
function wordsAt(prefix: string[]): string[] {
  const words = paths()
    .map(segments)
    .filter(
      (s) => s.length > prefix.length && prefix.every((p, i) => s[i]?.replace(/[+*?]$/, "") === p),
    )
    .map((s) => s[prefix.length] ?? param)
    .filter((w) => w !== param);
  return [...new Set(words)].toSorted((a, b) => a.localeCompare(b));
}

// The router's words where an account or a rule identifier also sits are written out, so a route the
// router adds there, read as wordsAt reads it, is a change to these lists, which review checks against the accounts and rules
// already stored (docs/UI.md section 5). The server's tests pin the same words, the top-level ones in
// ui/internal/api and each policy scope's in ui/internal/core/rules, which account setup and the policy
// writes refuse as identifiers.
test("the router's words where an account or a rule identifier also sits are a closed list", () => {
  expect(wordsAt([])).toEqual(["setup"]);
  expect(wordsAt([param, "policy"])).toEqual(["base", "history", "import", "new", "pick"]);
  expect(wordsAt(["setup", "policy"])).toEqual(["history", "import", "new"]);
  // A base rule seen from an account sits after policy/base, where no word does.
  expect(wordsAt([param, "policy", "base"])).toEqual([]);
});

// Every route mounts its screen by its route identity, apart from the entry route, which holds no
// account, and the not-found route, which holds no state of its own (docs/UI.md section 6).
test("every route but the entry and not-found routes mounts its screen by its route identity", () => {
  const table: VNode<{ children?: ComponentChildren }> = Routes();
  const routes = toChildArray(table.props.children).filter(
    (r): r is VNode<{ path?: string; default?: boolean; component?: unknown }> =>
      typeof r === "object",
  );
  const exempt = routes.filter((r) => r.props.path === "/" || r.props.default === true);
  expect(exempt.map((r) => r.props.path ?? "default")).toEqual(["/", "default"]);
  const unkeyed = routes
    .filter((r) => !exempt.includes(r) && !isScreen(r.props.component))
    .map((r) => r.props.path);
  expect(unkeyed).toEqual([]);
  expect(routes.length).toBeGreaterThan(exempt.length);
});
