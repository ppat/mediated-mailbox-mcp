// Bundles the browser app into dist, which the UI's Go binary embeds (docs/UI.md section 18).
//
// Every build passes the production define (ADR-0063). Bun's browser target otherwise substitutes the
// literal "development", so a dependency carrying a development build would ship it. test/build.test.ts
// builds a probe module with these options and fails if the define is missing.
//
// Run it from ui/browser: bun scripts/build.ts
import { mkdir, readdir, rm } from "node:fs/promises";
import { join } from "node:path";

export const options = {
  entrypoints: ["src/app/main.ts"],
  target: "browser",
  minify: true,
  define: { "process.env.NODE_ENV": JSON.stringify("production") },
} satisfies Bun.BuildConfig;

// The placeholder Go's embed needs on a fresh clone, kept when the directory is emptied.
const placeholder = ".gitkeep";

async function build(outdir: string): Promise<void> {
  // Files left by an earlier build would otherwise be embedded beside the new ones.
  await mkdir(outdir, { recursive: true });
  for (const name of await readdir(outdir)) {
    if (name !== placeholder) {
      await rm(join(outdir, name), { recursive: true });
    }
  }
  const result = await Bun.build({ ...options, outdir });
  for (const log of result.logs) {
    console.error(log);
  }
  if (!result.success) {
    process.exit(1);
  }
  for (const output of result.outputs) {
    console.log(output.path);
  }
}

if (import.meta.main) {
  await build("dist");
}
