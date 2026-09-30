// The app's dependencies for a test, bound to recorded answers, the shim's storage, short region
// timings and the configuration's defaults. The event stream's transport runs only in a browser
// (ADR-0063, ADR-0064), so connect hands the test each connection's objects and refetch, through which
// the test delivers what the transport would, a handled event or a poll.
import { signal } from "@preact/signals";
import { configDefaults } from "../src/app/config.ts";
import { makeDeps, type Deps } from "../src/app/deps.ts";
import type { LiveObjects } from "../src/app/stream.ts";
import type { Recorded } from "./fixtures/fetch.ts";

export type Connection = {
  account: string;
  objects: LiveObjects;
  refetch: () => void;
  open: boolean;
};

export function testDeps(
  server: Recorded,
  connections: Connection[] = [],
  now = () => Date.parse("2026-09-10T10:16:04Z"),
): Deps {
  localStorage.clear();
  return makeDeps(
    server.fetch,
    now,
    localStorage,
    { slow: 20, timeout: 60 },
    configDefaults,
    (account, objects, refetch) => {
      const connection: Connection = { account, objects, refetch, open: true };
      connections.push(connection);
      return {
        status: signal("connecting"),
        stop: () => {
          connection.open = false;
        },
      };
    },
  );
}
