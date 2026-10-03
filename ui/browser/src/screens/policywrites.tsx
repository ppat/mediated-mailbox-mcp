// What every policy screen of docs/UI.md sections 8.7 and 8.14 shares: the addresses of the policy
// screens, the writes and where they go, the checks a write's identifier and suffixes take before they
// are sent, the lift dialog and the import dialog that follows its contract, a refusal and the status
// region that announces each write's outcome, with Put it back after a lift.
//
// One account's policy screens write either scope through the account's endpoints, and the base
// policy's installation screens write the base policy through its own, so a write names the account it
// is made from, or none on an installation screen.
import type { ComponentChildren } from "preact";
import { useEffect, useId, useRef, useState } from "preact/hooks";
import {
  installationPath,
  policyApi,
  send,
  type Failure,
  type Post,
  type Problem,
  type RuleAnswer,
} from "../app/api.ts";
import { useDeps, type Deps } from "../app/deps.ts";
import { counted } from "../app/format.ts";
import { outcomeKey, type Outcome, type PutBack, type Scope } from "../app/outcomes.ts";
import { problemKind } from "../app/wording.ts";

// --- Addresses -----------------------------------------------------------------------------------

function segment(value: string): string {
  return encodeURIComponent(value);
}

// policyHome is a scope's policy screen, the account's, or the base policy's installation screen when
// no account is in view.
export function policyHome(account: string | undefined): string {
  return account === undefined ? "/setup/policy" : `/${segment(account)}/policy`;
}

// ruleScreen is one rule's address. From an account, a base rule is always at /policy/base/{id}, so a
// base rule and the account's rule of one identifier are each reachable (docs/UI.md section 8.7). added
// opens Edit domains with those suffixes added.
export function ruleScreen(
  account: string | undefined,
  scope: Scope,
  rule: string,
  added: readonly string[] = [],
): string {
  const path =
    account === undefined
      ? `/setup/policy/${segment(rule)}`
      : `${policyHome(account)}/${scope === "base" ? "base/" : ""}${segment(rule)}`;
  return added.length === 0
    ? path
    : `${path}?${new URLSearchParams(added.map((s) => ["suffix", s])).toString()}`;
}

// AddFill is what Add a rule opens filled with, the suffixes or the picker's search handed to it, and
// the scope and identifier a restore or a change of scope fills in.
export type AddFill = {
  suffixes?: readonly string[];
  search?: string;
  scope?: Scope;
  id?: string;
};

// addScreen is Add a rule's address over a scope's policy screen. The installation's panel adds to the
// base policy alone, so its address carries no scope.
export function addScreen(account: string | undefined, fill: AddFill = {}): string {
  const params = new URLSearchParams((fill.suffixes ?? []).map((s) => ["suffix", s]));
  if (fill.search !== undefined) {
    params.append("search", fill.search);
  }
  if (account !== undefined && fill.scope !== undefined) {
    params.append("scope", fill.scope);
  }
  if (fill.id !== undefined) {
    params.append("id", fill.id);
  }
  const query = params.toString();
  return `${policyHome(account)}/new${query === "" ? "" : `?${query}`}`;
}

// Opener is what opened Add a rule when it was not opened by its own address: the sender picker, whose
// search and selection Cancel returns to, or a change of where a rule applies, which continues to the
// old rule's lift once the add succeeds.
export type Opener = "pick" | "move";

// openAdd opens Add a rule recording what opened it in the browser's history entry, so the panel reads
// it back after a reload, and Back returns to the picker as the operator left it.
export function openAdd(url: string, opener: Opener): void {
  history.pushState({ opener }, "", url);
  dispatchEvent(new PopStateEvent("popstate"));
}

// opener reads what opened the panel in view from the browser's history entry.
export function openerOf(state: unknown): Opener | undefined {
  if (typeof state !== "object" || state === null) {
    return undefined;
  }
  const o: unknown = Reflect.get(state, "opener");
  return o === "pick" || o === "move" ? o : undefined;
}

// --- Checks --------------------------------------------------------------------------------------

// The words each scope's screens use in their addresses after the policy path, which no identifier
// of that scope may be (docs/UI.md sections 5, 8.7 and 8.14).
const routeWords: Record<Scope, readonly string[]> = {
  account: ["new", "pick", "history", "import", "base"],
  base: ["new", "history", "import"],
};

// identifierProblem is why an identifier cannot be a rule's in a scope, the checks of a write the UI
// makes before it sends one, or undefined when it can. Whether the scope already holds it is the
// server's to say.
export function identifierProblem(id: string, scope: Scope): string | undefined {
  if (id.trim() === "") {
    return "A rule needs an identifier.";
  }
  if (id !== id.trim()) {
    return "An identifier carries no space before or after it.";
  }
  if (routeWords[scope].includes(id) || id === "." || id === ".." || id.includes("/")) {
    return `${id} cannot be an identifier, since the policy screens' addresses use it. It may not be ., .. or hold a /.`;
  }
  return undefined;
}

// shaped reports whether a suffix has the shape of a domain name, dot-separated labels that are not
// empty and hold only letters of any script, digits, combining marks and hyphens, as the policy's
// validation checks it (ADR-0041).
export function shaped(suffix: string): boolean {
  return /^[\p{L}\p{Nd}\p{M}-]+(?:\.[\p{L}\p{Nd}\p{M}-]+)*$/u.test(suffix);
}

// SuffixChange is what an edit of a rule's suffixes adds and removes.
export type SuffixChange = { added: string[]; removed: string[] };

export function suffixChange(before: readonly string[], after: readonly string[]): SuffixChange {
  return {
    added: after.filter((s) => !before.includes(s)),
    removed: before.filter((s) => !after.includes(s)),
  };
}

// lines are a list of suffixes typed one per line, each trimmed, the empty ones and repeats left out.
export function lines(typed: readonly string[]): string[] {
  const out: string[] = [];
  for (const line of typed) {
    const s = line.trim();
    if (s !== "" && !out.includes(s)) {
      out.push(s);
    }
  }
  return out;
}

// --- Writes --------------------------------------------------------------------------------------

// Write is one policy write, an add, an edit of a rule's suffixes or a lift (docs/UI.md section 17.4).
export type Write =
  | { kind: "add"; rule: string; suffixes: readonly string[] }
  | {
      kind: "edit";
      rule: string;
      before: readonly string[];
      suffixes: readonly string[];
      confirmation: string | null;
    }
  | { kind: "lift"; rule: string; before: readonly string[]; confirmation: string | null };

// sendWrite sends one write to the scope's endpoint. An account's write names its scope, and the
// installation's names none, since its scope is always the base policy.
export function sendWrite(
  post: Post,
  account: string | undefined,
  scope: Scope,
  write: Write,
): Promise<{ ok: true; value: RuleAnswer } | { ok: false; failure: Failure }> {
  const named = account === undefined ? {} : { scope };
  const rules = `${policyApi(account)}/rules`;
  if (write.kind === "add") {
    return send<RuleAnswer>(post, rules, {
      ...named,
      rule_id: write.rule,
      suffixes: write.suffixes,
    });
  }
  if (write.kind === "edit") {
    return send<RuleAnswer>(post, `${rules}/${segment(write.rule)}`, {
      ...named,
      suffixes_before: write.before,
      suffixes: write.suffixes,
      confirmation: write.confirmation,
    });
  }
  return send<RuleAnswer>(post, `${rules}/${segment(write.rule)}/lift`, {
    ...named,
    suffixes_before: write.before,
    confirmation: write.confirmation,
  });
}

// putBackWrite is the write that adds one lifted restriction back, the rule again when it was lifted
// whole, or its removed suffixes back onto the suffixes it holds now. It only adds restriction, so it
// carries no confirmation (docs/UI.md section 8.7).
export function putBackWrite(p: PutBack): Write {
  return p.onto === undefined
    ? { kind: "add", rule: p.rule, suffixes: p.suffixes }
    : {
        kind: "edit",
        rule: p.rule,
        before: p.onto,
        suffixes: [...p.onto, ...p.suffixes.filter((s) => !p.onto?.includes(s))],
        confirmation: null,
      };
}

// refreshPolicy reads again everything a policy write can change that this tab has read: the rules,
// the history and the senders of every account's lens, every rule's detail and release count, every
// suffix's match, the base policy's reads and the installation's count of base rules.
export function refreshPolicy(deps: Deps): void {
  void deps.lens.refreshWhere((p) => /[?&]dataset=(rules|policy_changes|senders)(&|$)/.test(p));
  void deps.rules.refreshWhere(() => true);
  void deps.releases.refreshWhere(() => true);
  void deps.matches.refreshWhere(() => true);
  void deps.baseMatches.refreshWhere(() => true);
  void deps.basePolicy.refreshWhere(() => true);
  void deps.baseHistory.refreshWhere(() => true);
  void deps.installation.refreshWhere((p) => p === installationPath());
}

// lifted names what a lift removed, a rule or its suffixes, in the words of the outcome and the dialog.
export function lifted(names: readonly string[]): string {
  return names.join(", ");
}

// liftOutcome is the outcome of a lift, "Lifted {suffix or rule}." with Put it back.
export function liftOutcome(names: readonly string[], putBack: readonly PutBack[]): Outcome {
  return { text: `Lifted ${lifted(names)}.`, putBack, putBackLabel: "Put it back" };
}

// --- Refusals ------------------------------------------------------------------------------------

// refusalText is a refused write's cause in the screen's words. scopeName is the account in view for a
// write to its own rules, and undefined for a write to the base policy.
export function refusalText(failure: Failure, rule: string, scopeName: string | undefined): string {
  switch (failure.code) {
    case "identifier_taken":
      return scopeName === undefined
        ? `The base policy already has a rule ${rule}.`
        : `${scopeName} already has a rule ${rule}.`;
    case "rule_refused":
      return "The rule fails the policy's checks:";
    case "file_refused":
      return "The file is refused whole:";
    case "stale_rule":
      return "The rule changed since this page read it. It now shows as stored. Make the change again.";
    case "unknown_rule":
      return "The policy no longer holds this rule.";
    case "stale_preview":
      return "The policy changed since this preview. Preview again.";
    case "identity_missing":
      return "No identity was forwarded, so the change was not recorded.";
    case "stale_page":
      return "This page is older than the UI server's last restart. Reload, then try again.";
    default:
      return failure.message;
  }
}

// problemText is one problem a refused write or file names, with its line and its rule where the server
// names them.
export function problemText(p: Problem): string {
  const where = [
    p.line === null ? "" : `Line ${p.line}`,
    p.rule_id === "" ? "" : `rule ${p.rule_id}`,
  ]
    .filter((w) => w !== "")
    .join(", ");
  const what =
    p.kind === "invalid_suffix" && p.suffix !== null
      ? `the suffix ${p.suffix} is not shaped like a domain name`
      : problemKind[p.kind];
  return where === "" ? `${what[0]?.toUpperCase() ?? ""}${what.slice(1)}.` : `${where}: ${what}.`;
}

// Refusal shows a refused write, its cause and each problem the server named, and takes focus when it
// appears, so the operator hears why (docs/UI.md section 8.7).
export function Refusal(props: { failure: Failure; rule: string; scopeName: string | undefined }) {
  const ref = useRef<HTMLDivElement>(null);
  useEffect(() => ref.current?.focus(), [props.failure]);
  const problems = props.failure.problems ?? [];
  return (
    <div class="refusal" role="alert" tabIndex={-1} ref={ref}>
      <p>{refusalText(props.failure, props.rule, props.scopeName)}</p>
      {problems.length === 0 ? null : (
        <ul>
          {problems.map((p, i) => (
            <li key={i}>{problemText(p)}</li>
          ))}
        </ul>
      )}
    </div>
  );
}

// --- The lift dialog -----------------------------------------------------------------------------

// releaseSentences are the lift dialog's sentences on what a lift releases and what cannot be recalled
// (ADR-0002, ADR-0037). The base policy's own screen speaks of the senders the rule alone restricted,
// since it shows no account's counts.
export const releaseSentences =
  "From each process's next policy reload, their bodies are no longer denied for their sender. " +
  "A message scanned before the rule existed, or skipped by the scan gate, can then be released to the agent at once. " +
  "A message held as restricted goes back to pending scan when a scanning workload next compares the index with the policy, " +
  "and is released once a scan finds no code or link in it. A body already released cannot be recalled.";

export const baseReleaseSentences =
  "From each process's next policy reload, the senders it alone restricted no longer have their bodies denied. " +
  "A message scanned before the rule existed, or skipped by the scan gate, can then be released to the agent at once. " +
  "A message held as restricted goes back to pending scan when a scanning workload next compares the index with the policy, " +
  "and is released once a scan finds no code or link in it. A body already released cannot be recalled.";

// Typed is a dialog's typed confirmation, the value expected, the field's label stating it, and what
// the disabled button says until it is typed.
export type Typed = { expected: string; label: string; until: string };

// confirmed reports whether a typed confirmation is the expected value, compared after trimming
// surrounding space, so a pasted value with a trailing space confirms (docs/UI.md section 8.7).
export function confirmed(typed: string, expected: string): boolean {
  return typed.trim() === expected;
}

// LiftDialog is the dialog every lift goes through, an alertdialog labelled by its title and described
// by its sentences. Focus starts on Cancel, stays inside it while it is open, and returns to the control
// that opened it. With typed, the button enables only once the expected value is typed. The button names
// the effect and is outlined in the restricted color (docs/UI.md sections 8.7 and 14.2).
export function LiftDialog(props: {
  title: string;
  children: ComponentChildren;
  typed?: Typed;
  action: string;
  refusal?: ComponentChildren;
  onConfirm: (typed: string | null) => void;
  onCancel: () => void;
}) {
  const id = useId();
  const dialog = useRef<HTMLDivElement>(null);
  const cancel = useRef<HTMLButtonElement>(null);
  const [typed, setTyped] = useState("");
  useEffect(() => {
    const opener = document.activeElement;
    cancel.current?.focus();
    return () => {
      if (opener instanceof HTMLElement && opener.isConnected) {
        opener.focus();
      }
    };
  }, []);
  const ready = props.typed === undefined || confirmed(typed, props.typed.expected);
  return (
    <div
      class="dialog lift-dialog"
      role="alertdialog"
      aria-modal="true"
      aria-labelledby={`${id}-title`}
      aria-describedby={`${id}-says`}
      ref={dialog}
      onKeyDown={(event) => {
        if (event.key === "Escape") {
          event.stopPropagation();
          props.onCancel();
        } else if (event.key === "Tab") {
          trap(event, dialog.current);
        }
        // Every other key stays with the dialog, so no key of the screen behind it acts.
        event.stopPropagation();
      }}
    >
      <h2 id={`${id}-title`}>{props.title}</h2>
      <div id={`${id}-says`}>{props.children}</div>
      {props.typed === undefined ? null : (
        <label>
          {props.typed.label}{" "}
          <input
            type="text"
            class="mono"
            value={typed}
            autocomplete="off"
            onInput={(event) => setTyped(event.currentTarget.value)}
          />
        </label>
      )}
      {props.refusal}
      <div class="dialog-actions">
        <button type="button" ref={cancel} onClick={props.onCancel}>
          Cancel
        </button>
        <button
          type="button"
          class="lift"
          disabled={!ready}
          aria-describedby={ready ? undefined : `${id}-until`}
          onClick={() => props.onConfirm(props.typed === undefined ? null : typed.trim())}
        >
          {props.action}
        </button>
        {ready || props.typed === undefined ? null : (
          <span id={`${id}-until`} class="muted">
            {props.typed.until}
          </span>
        )}
      </div>
    </div>
  );
}

// trap keeps Tab inside the dialog, moving from its last control to its first and back.
function trap(event: KeyboardEvent, dialog: HTMLElement | null): void {
  if (dialog === null) {
    return;
  }
  const focusable = [
    ...dialog.querySelectorAll<HTMLElement>("a[href], button:not([disabled]), input, textarea"),
  ];
  const first = focusable[0];
  const last = focusable[focusable.length - 1];
  if (first === undefined || last === undefined) {
    return;
  }
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault();
    last.focus();
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault();
    first.focus();
  }
}

// --- Outcomes ------------------------------------------------------------------------------------

// OutcomeRegion is a policy screen's status region, which announces the outcome of the last write made
// from it, and after a lift carries Put it back, which adds back exactly what was lifted with no
// confirmation, since it only adds restriction (docs/UI.md section 8.7).
export function OutcomeRegion(props: { account: string | undefined }) {
  const deps = useDeps();
  const key = outcomeKey(props.account);
  const outcome = deps.outcomes.all.value.get(key);
  const [failure, setFailure] = useState<{ failure: Failure; rule: string } | undefined>();
  const putBack = async (items: readonly PutBack[]) => {
    setFailure(undefined);
    for (const p of items) {
      const result = await sendWrite(deps.post, props.account, p.scope, putBackWrite(p));
      if (!result.ok) {
        setFailure({ failure: result.failure, rule: p.rule });
        refreshPolicy(deps);
        return;
      }
    }
    deps.outcomes.set(key, {
      text: `Put back ${lifted(items.map((p) => (p.onto === undefined ? p.rule : lifted(p.suffixes))))}.`,
      putBack: [],
      putBackLabel: "",
    });
    refreshPolicy(deps);
  };
  return (
    <div class="status-region" role="status">
      {outcome === undefined ? null : (
        <p>
          {outcome.text}
          {outcome.putBack.length === 0 ? null : (
            <>
              {" "}
              <button type="button" class="primary" onClick={() => void putBack(outcome.putBack)}>
                {outcome.putBackLabel}
              </button>
            </>
          )}
        </p>
      )}
      {failure === undefined ? null : (
        <Refusal
          failure={failure.failure}
          rule={failure.rule}
          scopeName={
            outcome?.putBack.find((p) => p.rule === failure.rule)?.scope === "account"
              ? props.account
              : undefined
          }
        />
      )}
    </div>
  );
}

// countLine writes senders and messages as a lift states them, "1 sender and 4 stored messages".
export function countLine(senders: number, messages: number): string {
  return `${counted(senders, "sender")} and ${counted(messages, "stored message")}`;
}
