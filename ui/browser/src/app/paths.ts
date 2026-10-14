// Where the UI serves everything, under its one root, beside the client surface's /api and /mcp
// (ADR-0137, docs/UI.md sections 5 and 18). Every page address, every route and every read of the read
// API is built from this module, so no component writes a root-absolute address. The one other place
// the root is written is the stylesheet's font URLs, under /ui/fonts/, which the build keeps external
// through this module's root and test/build.test.ts compares with the font files the build copies.

// uiRoot is the UI's root.
export const uiRoot = "/ui";

// readRoot is the read API's root (docs/UI.md section 17).
const readRoot = `${uiRoot}/api`;

// accountsRead is the read API's accounts list, the one unscoped read of an account.
export function accountsRead(): string {
  return `${readRoot}/accounts`;
}

// accountRead is a read API path of an account, path the rest below the account with its segments
// already escaped.
export function accountRead(account: string, path: string): string {
  return `${readRoot}/accounts/${encodeURIComponent(account)}/${path}`;
}

// setupRead is a read API path of the installation's, which belong to no account, path the rest below
// setup with its segments already escaped, and empty for the installation endpoint itself.
export function setupRead(path = ""): string {
  return path === "" ? `${readRoot}/setup` : `${readRoot}/setup/${path}`;
}

// accountPage is the page address of a screen of an account, screen the path below the account with its
// segments already escaped, and empty for the account's home.
export function accountPage(account: string, screen = ""): string {
  const home = `${uiRoot}/accounts/${encodeURIComponent(account)}`;
  return screen === "" ? home : `${home}/${screen}`;
}

// setupPage is the page address of an installation screen, path the rest below setup with its segments
// already escaped, and empty for the installation screen itself.
export function setupPage(path = ""): string {
  return path === "" ? `${uiRoot}/setup` : `${uiRoot}/setup/${path}`;
}

// stylesheetPath is the bundle's stylesheet, which the entry document links and a guide window's
// document links again (docs/UI.md section 15).
export function stylesheetPath(): string {
  return `${uiRoot}/main.css`;
}

// entryPage is the entry route, which sends the browser on to an account or to setup.
export function entryPage(): string {
  return `${uiRoot}/`;
}

// The route patterns the router matches, the account's pages under accountRoute and the installation's
// under setupRoute.
export const accountRoute = `${uiRoot}/accounts/:account`;
export const setupRoute = `${uiRoot}/setup`;

// belowAccount is the path segments of a page address below its account, still escaped, and the account
// decoded, or undefined for an address that is no account's page.
export function belowAccount(pathname: string): { account: string; below: string[] } | undefined {
  const segments = pathname.split("/").filter((s) => s !== "");
  const root = uiRoot.split("/").filter((s) => s !== "");
  const account = segments[root.length + 1];
  if (
    account === undefined ||
    segments[root.length] !== "accounts" ||
    root.some((s, i) => segments[i] !== s)
  ) {
    return undefined;
  }
  return { account: decodeURIComponent(account), below: segments.slice(root.length + 2) };
}
