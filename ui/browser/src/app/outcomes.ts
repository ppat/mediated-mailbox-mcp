// The outcomes of the policy writes (docs/UI.md section 8.7). Each policy screen shows the outcome of the
// last write made from it in its status region, and after a lift, "Put it back" with exactly what the
// lift removed. A write often ends on another screen than the one it was made from, a rule lifted from
// its panel ending on the policy list, or an import on the scope's policy screen, so an outcome is held
// here, keyed by the policy screen it belongs to, rather than by the component that made the write.
import { signal, type ReadonlySignal } from "@preact/signals";

// Scope is a rule's scope, the base policy or the account's own rules.
export type Scope = "base" | "account";

// PutBack is one restriction a lift removed. Putting it back adds a lifted rule again with its
// suffixes, or, when onto names the suffixes the rule holds now, adds the removed suffixes back onto it.
export type PutBack = {
  scope: Scope;
  rule: string;
  suffixes: readonly string[];
  onto: readonly string[] | undefined;
};

// Outcome is what a write's status region says, with the restrictions Put it back adds back and the
// words of its control.
export type Outcome = {
  text: string;
  putBack: readonly PutBack[];
  putBackLabel: string;
};

export class Outcomes {
  readonly #all = signal<ReadonlyMap<string, Outcome>>(new Map());

  // all is every screen's outcome, keyed by outcomeKey.
  get all(): ReadonlySignal<ReadonlyMap<string, Outcome>> {
    return this.#all;
  }

  set(key: string, outcome: Outcome | undefined): void {
    const next = new Map(this.#all.peek());
    if (outcome === undefined) {
      next.delete(key);
    } else {
      next.set(key, outcome);
    }
    this.#all.value = next;
  }
}

// outcomeKey is the policy screen an outcome belongs to, an account's policy screen, or the base
// policy's installation screen when no account is in view.
export function outcomeKey(account: string | undefined): string {
  return account === undefined ? "setup" : `account:${account}`;
}
