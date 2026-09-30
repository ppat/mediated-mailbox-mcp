import { describe, expect, test } from "bun:test";
import { allow } from "../gate.ts";

describe("the gate", () => {
  test("refuses a flagged item", () => {
    expect(allow(true)).toBe(false);
  });
});

test("allows a clean item", () => {
  expect(allow(false)).toBe(true);
});
