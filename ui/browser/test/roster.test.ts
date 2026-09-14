// The dependency roster check (ADR-0063). Every direct dependency declared by the browser layer's
// manifests has a roster line, and every roster line names a dependency its manifest declares.
import { expect, test } from "bun:test";

type Manifest = { dependencies?: Record<string, string>; devDependencies?: Record<string, string> };

// rosterEntries returns "<manifest> <package>" for every roster line, skipping comments and blank lines. A line
// must carry a reason after the package.
function rosterEntries(text: string): string[] {
  const out: string[] = [];
  for (const [index, line] of text.split("\n").entries()) {
    if (line.trim() === "" || line.startsWith("#")) {
      continue;
    }
    const [manifest, name, ...reason] = line.trim().split(/\s+/);
    if (manifest === undefined || name === undefined || reason.length === 0) {
      throw new Error(`roster.txt:${index + 1}: a line names a manifest, a package and a reason`);
    }
    out.push(`${manifest} ${name}`);
  }
  return out;
}

function manifestEntries(path: string, manifest: Manifest): string[] {
  return [
    ...Object.keys(manifest.dependencies ?? {}),
    ...Object.keys(manifest.devDependencies ?? {}),
  ].map((name) => `${path} ${name}`);
}

// disagreements lists what one side has and the other lacks, in both directions.
function disagreements(roster: string[], declared: string[]): string[] {
  const inRoster = new Set(roster);
  const inManifests = new Set(declared);
  return [
    ...roster
      .filter((entry) => !inManifests.has(entry))
      .map((entry) => `roster line with no dependency: ${entry}`),
    ...declared
      .filter((entry) => !inRoster.has(entry))
      .map((entry) => `dependency with no roster line: ${entry}`),
  ];
}

test("the roster and the manifests agree", async () => {
  const roster = rosterEntries(await Bun.file("roster.txt").text());
  const declared = [
    ...manifestEntries("package.json", await Bun.file("package.json").json()),
    ...manifestEntries("codegen/package.json", await Bun.file("codegen/package.json").json()),
  ];
  expect(disagreements(roster, declared)).toEqual([]);
});

test("a disagreement is reported in either direction", () => {
  expect(disagreements(["package.json a"], ["package.json b"])).toEqual([
    "roster line with no dependency: package.json a",
    "dependency with no roster line: package.json b",
  ]);
});
