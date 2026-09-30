// Bundles the browser app into dist, which the UI's Go binary embeds (docs/UI.md section 18).
//
// Every build passes the production define (ADR-0063). Bun's browser target otherwise substitutes the
// literal "development", so a dependency carrying a development build would ship it. test/build.test.ts
// builds a probe module with these options and fails if the define is missing.
//
// The stylesheet src/app/main.ts imports is written beside the entry as main.css, which the entry document
// links. Bun inlines every font a stylesheet's url() reaches as a data: URI, which the policy's
// font-src 'self' blocks, so the font URLs stay external as /fonts/ paths and the build copies the font
// files there (docs/UI.md section 14.3). test/build.test.ts proves the built stylesheet names only files
// the build copied.
//
// Run it from ui/browser: bun scripts/build.ts
import { copyFile, mkdir, readdir, rm } from "node:fs/promises";
import { join } from "node:path";

export const options = {
  entrypoints: ["src/app/main.ts"],
  target: "browser",
  minify: true,
  define: { "process.env.NODE_ENV": JSON.stringify("production") },
  external: ["/fonts/*"],
} satisfies Bun.BuildConfig;

// fonts is where the vendored font files sit, and fontsOut the bundle directory the stylesheet names them in.
export const fonts = "src/fonts";
const fontsOut = "fonts";

// The placeholder Go's embed needs on a fresh clone, kept when the directory is emptied.
const placeholder = ".gitkeep";

export async function build(outdir: string): Promise<void> {
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
    throw new Error("the bundle did not build");
  }
  for (const output of result.outputs) {
    console.log(output.path);
  }
  await mkdir(join(outdir, fontsOut));
  for (const name of await readdir(fonts)) {
    if (name.endsWith(".woff2")) {
      await copyFile(join(fonts, name), join(outdir, fontsOut, name));
      console.log(join(outdir, fontsOut, name));
    }
  }
}

if (import.meta.main) {
  await build("dist");
}
