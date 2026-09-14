// Proves the build passes the production define, by building a probe module with the build's own options.
import { expect, test } from "bun:test";
import { options } from "../scripts/build.ts";

test("the build replaces process.env.NODE_ENV with production", async () => {
  const result = await Bun.build({ ...options, entrypoints: ["test/probes/define.ts"], minify: false });
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
