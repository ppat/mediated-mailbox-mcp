// Pure rules about screen paths, from docs/UI.md sections 5 and 6.
import type { Account } from "./api.ts";

const lastAccountKey = "mediated-mailbox.account";

// entryAccount is where the entry route sends the browser, the account last used in this browser when
// it is still listed, else the first account by identifier. It is undefined when no account exists.
export function entryAccount(
  accounts: readonly Account[],
  last: string | undefined,
): string | undefined {
  if (last !== undefined && accounts.some((a) => a.account_id === last)) {
    return last;
  }
  return accounts.map((a) => a.account_id).toSorted()[0];
}

export function readLastAccount(storage: Storage | undefined): string | undefined {
  try {
    return storage?.getItem(lastAccountKey) ?? undefined;
  } catch {
    return undefined;
  }
}

export function keepLastAccount(storage: Storage | undefined, account: string): void {
  try {
    storage?.setItem(lastAccountKey, account);
  } catch {
    // A browser refusing storage sends the entry route to the first account.
  }
}

// switchAccount is the path the account selector sends a screen to under another account. The screen
// keeps its first segment below the account, so an object's screen goes to its list and a row detail
// to its lens, and keeps only the level of its query, so a lens keeps its dataset and level and drops
// its filters (section 6).
export function switchAccount(pathname: string, search: string, account: string): string {
  const screen = pathname.split("/").filter((s) => s !== "")[1];
  const target = `/${encodeURIComponent(account)}${screen === undefined ? "" : `/${screen}`}`;
  const level = new URLSearchParams(search).get("level");
  return level === null || screen === undefined
    ? target
    : `${target}?${new URLSearchParams({ level }).toString()}`;
}
