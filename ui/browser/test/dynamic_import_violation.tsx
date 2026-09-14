// Violation file. A test written as .tsx loads the runner's module by a dynamic import, which the browser
// lint bans (ADR-0064, ADR-0072). It is not named as a test, so bun test never runs it.
const loaded = import("bun:test"); // want ast-grep "runner-dynamic-import-tsx"

export const view = <p hidden={loaded === undefined} />;
