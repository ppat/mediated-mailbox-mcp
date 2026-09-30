// The formatting rules of docs/UI.md section 11, each against the section's own examples.
import { expect, test } from "bun:test";
import {
  age,
  ageOf,
  count,
  duration,
  labelChange,
  planId,
  rate,
  share,
  utc,
  utcDate,
  utcTime,
} from "../src/app/format.ts";

test("counts carry a thousands separator and no abbreviation", () => {
  expect(count(12480)).toBe("12,480");
  expect(count(84212)).toBe("84,212");
  expect(count(0)).toBe("0");
});

test("shares carry one decimal and a percent sign", () => {
  expect(share(148, 1000)).toBe("14.8%");
  expect(share(3065, 3368)).toBe("91.0%");
  expect(share(1, 0)).toBe("0.0%");
});

test("rates carry one decimal and the unit", () => {
  expect(rate(3.1, 5)).toBe("3.1 of 5.0 units/s");
});

test("durations and ages take the largest two units", () => {
  const h = 3_600_000;
  const m = 60_000;
  expect(duration(h + 10 * m)).toBe("1h 10m");
  expect(duration(6 * h + 41 * m + 5_000)).toBe("6h 41m");
  expect(duration(12_000)).toBe("12s");
  expect(duration(14 * 24 * h)).toBe("14d");
  expect(duration(28 * h)).toBe("1d 4h");
  expect(duration(400)).toBe("0s");
  const now = Date.parse("2026-09-10T10:16:04Z");
  expect(age("2026-09-09T06:16:04Z", now)).toBe("1d 4h");
  expect(ageOf("2026-09-09T06:16:04Z", 14 * 24 * h, now)).toBe("1d 4h of 14d");
});

test("absolute times are UTC with the Z suffix", () => {
  expect(utc("2026-09-10T10:12:44Z")).toBe("2026-09-10 10:12Z");
  expect(utc("2026-09-10T12:12:44+02:00")).toBe("2026-09-10 10:12Z");
  expect(utcTime("2026-09-10T10:12:44Z")).toBe("10:12Z");
  expect(utcDate("2026-09-03T23:59:00Z")).toBe("2026-09-03");
});

test("a plan identifier shows its first six characters", () => {
  expect(planId("7f3a9c00-0000-4000-8000-000000000001")).toBe("7f3a9c…");
});

test("a label change is two bracketed sets joined by an arrow", () => {
  expect(labelChange(["INBOX"], ["INBOX", "Finance/Statements"])).toBe(
    "[INBOX] → [INBOX, Finance/Statements]",
  );
});
