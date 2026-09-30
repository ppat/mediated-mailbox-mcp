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
