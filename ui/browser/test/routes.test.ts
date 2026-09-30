// The rules about screen paths of docs/UI.md sections 5 and 6.
import { expect, test } from "bun:test";
import { entryAccount, switchAccount } from "../src/app/routes.ts";

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
