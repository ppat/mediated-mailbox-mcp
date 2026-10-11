// Proves the build passes the production define, by building a probe module with the build's own options,
// that the bundle's stylesheet loads its fonts as files from the UI's own origin (docs/UI.md section
// 14.3), and that the bundle's top level holds only the pinned words (docs/UI.md section 5).
import { expect, test } from "bun:test";
import { mkdtemp, readdir, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { build, options } from "../scripts/build.ts";

test("the build replaces process.env.NODE_ENV with production", async () => {
  const result = await Bun.build({
    ...options,
    entrypoints: ["test/probes/define.ts"],
    minify: false,
  });
  expect(result.success).toBe(true);
  const [output] = result.outputs;
  if (output === undefined) {
    throw new Error("the build produced no output");
  }
  const text = await output.text();
  expect(text).toContain('"production"');
  expect(text).not.toContain("development");
  expect(text).not.toContain("process.env");
});

function byName(a: string, b: string): number {
  return a.localeCompare(b);
}

test("the built stylesheet takes each font from a file the build copies, and inlines none", async () => {
  const outdir = await mkdtemp(join(tmpdir(), "mediated-mailbox-ui-build-"));
  try {
    await build(outdir);
    const css = await Bun.file(join(outdir, "main.css")).text();
    const named = [...css.matchAll(/url\(([^)]*)\)/g)].map((m) => m[1] ?? "");
    const copied = (await readdir(join(outdir, "fonts"))).map((name) => `/fonts/${name}`);
    expect(named.toSorted(byName)).toEqual(copied.toSorted(byName));
    expect(copied.toSorted(byName)).toEqual([
      "/fonts/IBMPlexMono-Medium-Latin1.woff2",
      "/fonts/IBMPlexMono-Regular-Latin1.woff2",
      "/fonts/IBMPlexSans-Medium-Latin1.woff2",
      "/fonts/IBMPlexSans-Regular-Latin1.woff2",
      "/fonts/IBMPlexSans-SemiBold-Latin1.woff2",
    ]);
    expect(css).not.toContain("data:");
  } finally {
    await rm(outdir, { recursive: true });
  }
});

// Every name at the bundle's top level is a top-level path the UI serves, which account setup refuses as
// an account identifier, so the names are written out and a file the build adds there is a change to
// this list, which review checks against the accounts already stored (docs/UI.md section 5). The
// server's tests in ui/internal/api pin the same names among the words account setup refuses.
test("the bundle's top level is the entry module, its stylesheet and the fonts directory alone", async () => {
  const outdir = await mkdtemp(join(tmpdir(), "mediated-mailbox-ui-build-"));
  try {
    await build(outdir);
    const names = (await readdir(outdir)).filter((name) => !name.startsWith("."));
    expect(names.toSorted(byName)).toEqual(["fonts", "main.css", "main.js"]);
  } finally {
    await rm(outdir, { recursive: true });
  }
});
