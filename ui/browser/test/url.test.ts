// The URL grammar of docs/UI.md section 5, against the descriptor table generated from the contract.
// Every expected string is written out here, not built with the module under test.
import { describe, expect, test } from "bun:test";
import {
  apiQuery,
  canonical,
  canonicalize,
  chips,
  href,
  parse,
  removeChip,
  serialize,
  withPage,
  withRange,
} from "../src/app/url.ts";

describe("canonicalization", () => {
  test("a bare URL takes every default the descriptor table gives", () => {
    expect(canonical("plans", "")).toBe("level=3&range=all&sort=created_at,desc&page=1");
    expect(canonical("candidates", "")).toBe(
      "level=3&range=all&sort=score,desc&page=1&status=pending",
    );
  });

  test("a parameter the URL carries is kept as written, even one the registry would refuse", () => {
    expect(canonical("candidates", "sort=created_at,asc&level=0&status=dismissed")).toBe(
      "level=0&range=all&sort=created_at,asc&status=dismissed",
    );
    expect(canonical("plans", "level=9&bogus=1")).toBe(
      "level=9&range=all&sort=created_at,desc&bogus=1",
    );
  });

  test("the canonical form is canonical", () => {
    for (const search of ["", "status=DRAFT,APPLIED&page=2", "status=!DRAFT", "status="]) {
      const once = canonical("plans", search);
      expect(canonical("plans", once)).toBe(once);
    }
  });

  test("the comma and the exclamation mark stay literal", () => {
    expect(canonical("plans", "status=%21DRAFT")).toBe(
      "level=3&range=all&sort=created_at,desc&page=1&status=!DRAFT",
    );
    expect(serialize(parse("plans", "status=DRAFT%2CAPPLIED"))).toBe("status=DRAFT,APPLIED");
  });

  test("a removed default filter stays removed and is never sent", () => {
    const view = canonicalize(parse("candidates", "status="));
    expect(serialize(view)).toBe("level=3&range=all&sort=score,desc&page=1&status=");
    expect(apiQuery(view)).toBe("dataset=candidates&level=3&range=all&sort=score,desc&page=1");
  });
});

describe("the breadcrumb", () => {
  test("chips keep the order the filters were applied in", () => {
    const view = canonicalize(parse("candidates", "status=pending,confirmed&created=x"));
    expect(chips(view).map((c) => c.filter.dimension)).toEqual(["status", "created"]);
  });

  test("removing a chip removes it and every chip after it, and a removed default is written empty", () => {
    const view = canonicalize(parse("candidates", "page=3&status=pending&a=1&b=2"));
    expect(serialize(removeChip(view, 2))).toBe(
      "level=3&range=all&sort=score,desc&page=1&status=pending&a=1",
    );
    expect(serialize(removeChip(view, 0))).toBe("level=3&range=all&sort=score,desc&page=1&status=");
  });

  test("a level 2 view left with no filter goes back to level 1", () => {
    const view = parse("plans", "level=2&group=g&status=DRAFT");
    expect(serialize(removeChip(view, 0))).toBe("level=1&group=g");
  });
});

describe("links", () => {
  test("a range or a page change is written through the serializer", () => {
    const view = canonicalize(parse("plans", "page=4"));
    expect(serialize(withRange(view, "2026-09-01,2026-09-10"))).toBe(
      "level=3&range=2026-09-01,2026-09-10&sort=created_at,desc&page=1",
    );
    expect(href("personal", withPage(view, 5))).toBe(
      "/personal/plans?level=3&range=all&sort=created_at,desc&page=5",
    );
  });

  test("the account is one encoded path segment", () => {
    expect(href("a/b", canonicalize(parse("plans", "")))).toStartWith("/a%2Fb/plans?");
  });
});
