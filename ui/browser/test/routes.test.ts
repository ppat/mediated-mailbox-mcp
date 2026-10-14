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
  expect(switchAccount("/ui/accounts/personal", "", "other")).toBe("/ui/accounts/other");
  expect(
    switchAccount("/ui/accounts/personal/plans", "?level=3&status=DRAFT&page=2", "other"),
  ).toBe("/ui/accounts/other/plans?level=3");
  expect(switchAccount("/ui/accounts/personal/plans/7f3a9c00", "?section=flows", "other")).toBe(
    "/ui/accounts/other/plans",
  );
  expect(switchAccount("/ui/accounts/personal/jobs/r-0913", "?level=1", "other")).toBe(
    "/ui/accounts/other/jobs?level=1",
  );
  expect(switchAccount("/ui/accounts/personal/messages", "", "a/b")).toBe(
    "/ui/accounts/a%2Fb/messages",
  );
});

// paths is every path the route table matches, the not-found route aside. The router reads its routes
// from its children through toChildArray, as this does, so no route reaches the router that is not here.
function paths(): string[] {
  const table: VNode<{ children?: ComponentChildren }> = Routes();
  return toChildArray(table.props.children)
    .filter((r): r is VNode<{ path?: string }> => typeof r === "object")
    .flatMap((r) => (r.props.path === undefined ? [] : [r.props.path]));
}

// root is the UI's root, written out here rather than read from the code under test, under which every
// route sits (ADR-0137, docs/UI.md section 5).
const root = "/ui/";

// Every route the router matches sits under the UI's root, whatever its parameters, so the product's
// top-level words are its surfaces, and an account's identifier follows accounts and a rule's follows
// rules or base, never sitting where a route word does. The server's tests require the same of its
// routes (docs/UI.md section 5).
test("every route sits under the UI's root", () => {
  const all = paths();
  expect(all.length).toBeGreaterThan(0);
  expect(all.filter((p) => !p.startsWith(root) && p !== root)).toEqual([]);
  expect(
    all.filter((p) => p.includes("/:account") && !p.startsWith(`${root}accounts/:account`)),
  ).toEqual([]);
  expect(all.filter((p) => /\/:rule\b/.test(p) && !/\/(rules|base)\/:rule$/.test(p))).toEqual([]);
});

// Every route mounts its screen by its route identity, apart from the entry route, which holds no
// account, and the not-found route, which holds no state of its own (docs/UI.md section 6).
test("every route but the entry and not-found routes mounts its screen by its route identity", () => {
  const table: VNode<{ children?: ComponentChildren }> = Routes();
  const routes = toChildArray(table.props.children).filter(
    (r): r is VNode<{ path?: string; default?: boolean; component?: unknown }> =>
      typeof r === "object",
  );
  const exempt = routes.filter((r) => r.props.path === root || r.props.default === true);
  expect(exempt.map((r) => r.props.path ?? "default")).toEqual([root, "default"]);
  const unkeyed = routes
    .filter((r) => !exempt.includes(r) && !isScreen(r.props.component))
    .map((r) => r.props.path);
  expect(unkeyed).toEqual([]);
  expect(routes.length).toBeGreaterThan(exempt.length);
});
