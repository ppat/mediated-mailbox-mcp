// Add a rule, the panel over a scope's policy screen (docs/UI.md sections 8.7 and 8.14). From an account
// it chooses the scope, takes the identifier, prefilled operator.{first suffix} until the operator edits
// it, and the domain suffixes one per line, each checked once typing pauses and shown with what it
// matches in the account. On the base policy's installation screen it adds to the base policy alone and
// checks each suffix's shape, since no account's counts belong there. Adding only adds restriction, so
// it asks no confirmation. A change of where a rule applies continues, once its add succeeds, to the
// old rule's lift dialog, so the add always comes first.
import { useEffect, useRef, useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import {
  isRowsPage,
  lensPath,
  baseMatchPath,
  matchPath,
  rulePath,
  type Failure,
  type BaseSuffixMatch,
  type MatchAnswer,
  type RuleDetail,
  type SuffixMatch,
} from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { counted, share } from "../app/format.ts";
import { outcomeKey, type Scope } from "../app/outcomes.ts";
import { apiQuery, canonicalize, parse } from "../app/url.ts";
import { DetailPanel } from "../lens/panel.tsx";
import { restricts } from "../row/sender.tsx";
import { accountSays, LiftRule } from "./rule.tsx";
import {
  identifierProblem,
  lines,
  openerOf,
  policyHome,
  refreshPolicy,
  Refusal,
  ruleScreen,
  sendWrite,
  type AddFill,
  type Opener,
} from "./policywrites.tsx";

// broadShare is the share of the account's senders above which a suffix carries the warning that it is
// likely broader than one institution (docs/UI.md section 8.7).
export const broadShare = 0.1;

// pause is how long typing must stop before the lines typed are checked, so a check is not read on
// every key.
export const pause = 400;

// fillOf reads what the panel's address fills it with.
export function fillOf(url: string): AddFill {
  const query = new URLSearchParams(url.includes("?") ? url.slice(url.indexOf("?") + 1) : "");
  const scope = query.get("scope");
  return {
    suffixes: query.getAll("suffix"),
    search: query.get("search") ?? undefined,
    scope: scope === "base" || scope === "account" ? scope : undefined,
    id: query.get("id") ?? undefined,
  };
}

// pickedSearch is the senders dataset's rows read for a search, as the sender picker reads them.
export function pickedSearch(account: string, search: string, page: number): string {
  const view = canonicalize(parse("senders", new URLSearchParams({ search }).toString()));
  return lensPath(account, apiQuery({ ...view, page: String(page) }));
}

// Verdict is one line's check, refused when the line cannot be a suffix, with the words shown under it.
type Verdict = { refused: boolean; says: string[]; warning: string | undefined };

// verdict is a line's check from an account, from what the server answered for it.
export function verdict(account: string, m: SuffixMatch, total: number): Verdict {
  if (!m.valid) {
    return { refused: true, says: ["Not shaped like a domain name."], warning: undefined };
  }
  const says = [
    `matches ${counted(m.senders, "sender")} · ${counted(m.messages, "message")} in ${account}`,
    ...m.rules.map(
      (r) =>
        `Already matched by ${r.scope === "base" ? "the base rule" : `${account}'s rule`} ${r.rule_id}.`,
    ),
  ];
  const broad = m.public_suffix || (total > 0 && m.senders / total > broadShare);
  return {
    refused: false,
    says,
    warning: broad
      ? `This matches ${share(m.senders, total)} of ${account}'s senders. It is likely broader than one institution.`
      : undefined,
  };
}

// baseVerdict is a line's check on the base policy's own screen, from the base policy's match read,
// which names the base rules already matching it and whether it is a public suffix, and counts nothing,
// since no account's numbers belong there (section 8.14).
export function baseVerdict(m: BaseSuffixMatch): Verdict {
  if (!m.valid) {
    return { refused: true, says: ["Not shaped like a domain name."], warning: undefined };
  }
  return {
    refused: false,
    says: m.rules.map((r) => `Already matched by the base rule ${r.rule_id}.`),
    warning: m.public_suffix
      ? "This is a public suffix. It restricts every sender under it."
      : undefined,
  };
}

// AddRulePanel is the panel at a scope's new address. opener is what opened it, read from the browser's
// history entry.
export function AddRulePanel(props: { account: string | undefined }) {
  const { url } = useLocation();
  const fill = fillOf(url);
  const opener = openerOf(history.state);
  return (
    <DetailPanel
      title={props.account === undefined ? "Add a base rule" : "Add a rule"}
      back={policyHome(props.account)}
      close={opener === "pick" ? backToPicker : undefined}
    >
      {fill.search !== undefined && props.account !== undefined ? (
        <SearchedDomains account={props.account} search={fill.search} fill={fill} opener={opener} />
      ) : (
        <AddForm
          key={url}
          account={props.account}
          fill={fill}
          opener={opener}
          handedOff={(fill.suffixes ?? []).length > 0}
        />
      )}
    </DetailPanel>
  );
}

// backToPicker returns to the sender picker the panel was opened from, with its search and selection,
// which its history entry holds (section 8.7).
function backToPicker(): void {
  history.back();
}

// SearchedDomains reads every sender a picker's search matches, across its pages, and fills the form
// with the domains no rule restricts yet, since a restricted sender cannot be selected (section 8.7).
function SearchedDomains(props: {
  account: string;
  search: string;
  fill: AddFill;
  opener: Opener | undefined;
}) {
  const { lens } = useDeps();
  const first = lens.read(pickedSearch(props.account, props.search, 1)).value;
  if (first.status === "error") {
    return (
      <p class="refusal" role="alert">
        The senders matching {props.search} could not be read: {first.failure.message}
      </p>
    );
  }
  if (first.status !== "ok" || !isRowsPage(first.answer) || first.answer.dataset !== "senders") {
    return <p class="muted">Reading the senders that match {props.search}…</p>;
  }
  const domains: string[] = first.answer.rows.filter((r) => !restricts(r)).map((r) => r.domain);
  for (let page = 2; page <= first.answer.pages; page++) {
    const state = lens.read(pickedSearch(props.account, props.search, page)).value;
    if (state.status !== "ok" || !isRowsPage(state.answer) || state.answer.dataset !== "senders") {
      return <p class="muted">Reading the senders that match {props.search}…</p>;
    }
    domains.push(...state.answer.rows.filter((r) => !restricts(r)).map((r) => r.domain));
  }
  return (
    <>
      <p>
        The {counted(domains.length, "sender")} matching <span class="mono">{props.search}</span>{" "}
        that no rule restricts yet.
      </p>
      <AddForm
        account={props.account}
        fill={{ ...props.fill, suffixes: domains }}
        opener={props.opener}
        handedOff
      />
    </>
  );
}

// Moved is a change of where a rule applies whose add succeeded, waiting on the old rule's lift.
type Moved = { scope: Scope; detail: RuleDetail };

function AddForm(props: {
  account: string | undefined;
  fill: AddFill;
  opener: Opener | undefined;
  handedOff: boolean;
}) {
  const deps = useDeps();
  const { route } = useLocation();
  const { account, fill } = props;
  const [scope, setScope] = useState<Scope>(
    account === undefined ? "base" : (fill.scope ?? "account"),
  );
  const [typed, setTyped] = useState<string[]>([...(fill.suffixes ?? []), ""]);
  const [checked, setChecked] = useState<string[]>(lines(fill.suffixes ?? []));
  const [id, setId] = useState(fill.id ?? "");
  const [idEdited, setIdEdited] = useState(fill.id !== undefined);
  const [failure, setFailure] = useState<Failure | undefined>();
  const [moved, setMoved] = useState<Moved | undefined>();
  const first = useRef<HTMLInputElement>(null);
  const add = useRef<HTMLButtonElement>(null);
  const { handedOff } = props;
  useEffect(() => {
    if (handedOff) {
      add.current?.focus();
    } else {
      first.current?.focus();
    }
  }, [handedOff]);
  // The lines typed are checked once typing pauses, not on every key (section 8.7).
  const { timers } = deps;
  useEffect(() => {
    const waiting = timers.set(() => setChecked(lines(typed)), pause);
    return () => timers.clear(waiting);
  }, [typed, timers]);
  const suffixes = lines(typed);
  const identifier = idEdited ? id : `operator.${suffixes[0] ?? ""}`;
  const idProblem = identifierProblem(identifier, scope);
  const match =
    account === undefined || checked.length === 0
      ? undefined
      : deps.matches.read(matchPath(account, checked)).value;
  const answer: MatchAnswer | undefined = match?.status === "ok" ? match.answer : undefined;
  const baseMatch =
    account !== undefined || checked.length === 0
      ? undefined
      : deps.baseMatches.read(baseMatchPath(checked)).value;
  const baseAnswer = baseMatch?.status === "ok" ? baseMatch.answer : undefined;
  const verdictOf = (line: string): Verdict | undefined => {
    if (!checked.includes(line)) {
      return undefined;
    }
    if (account === undefined) {
      const b = baseAnswer?.suffixes.find((s) => s.suffix === line);
      return b === undefined ? undefined : baseVerdict(b);
    }
    const m = answer?.suffixes.find((s) => s.suffix === line);
    return m === undefined || answer === undefined
      ? undefined
      : verdict(account, m, answer.senders_total);
  };
  const refused = suffixes.some((s) => verdictOf(s)?.refused === true);
  const type = (i: number, value: string) => {
    const parts = value.split(/\r?\n/);
    const next = [...typed.slice(0, i), ...parts, ...typed.slice(i + 1)];
    if (next[next.length - 1] !== "") {
      next.push("");
    }
    setTyped(next);
  };
  // The suffixes counted together, so a sender two of them match counts once.
  const counts = answer?.together ?? { senders: 0, messages: 0 };
  const submit = async () => {
    setFailure(undefined);
    const result = await sendWrite(deps.post, account, scope, {
      kind: "add",
      rule: identifier,
      suffixes,
    });
    if (!result.ok) {
      setFailure(result.failure);
      return;
    }
    deps.outcomes.set(outcomeKey(account), {
      text: `Added ${identifier}.`,
      putBack: [],
      putBackLabel: "",
    });
    refreshPolicy(deps);
    if (props.opener === "move" && account !== undefined) {
      // The old rule is the one of the same identifier in the other scope. Its lift dialog states the
      // account's numbers read after the add, which are then nothing the new rule does not cover.
      const old: Scope = scope === "base" ? "account" : "base";
      const path = rulePath(account, old, identifier);
      await deps.rules.refresh(path);
      const state = deps.rules.read(path).peek();
      if (state.status === "ok") {
        setMoved({ scope: old, detail: state.answer });
        return;
      }
    }
    route(ruleScreen(account, scope, identifier));
  };
  if (moved !== undefined && account !== undefined) {
    const target = ruleScreen(account, scope, identifier);
    return (
      <LiftRule
        account={account}
        scope={moved.scope}
        rule={moved.detail.rule_id}
        suffixes={moved.detail.suffixes}
        says={accountSays({
          account,
          scope: moved.scope,
          released: moved.detail.released,
          what: "rule",
          accounts: moved.detail.accounts,
          movedTo: identifier,
        })}
        after={target}
        onCancel={() => route(target)}
      />
    );
  }
  return (
    <form
      class="add-rule"
      onSubmit={(event) => {
        event.preventDefault();
        void submit();
      }}
    >
      {account === undefined ? null : (
        <fieldset>
          <legend>Applies to</legend>
          <label>
            <input
              type="radio"
              name="scope"
              ref={first}
              checked={scope === "account"}
              onChange={() => setScope("account")}
            />{" "}
            This account, {account}
          </label>
          <label>
            <input
              type="radio"
              name="scope"
              checked={scope === "base"}
              onChange={() => setScope("base")}
            />{" "}
            Every account (the base policy)
          </label>
        </fieldset>
      )}
      <label>
        Rule identifier{" "}
        <input
          type="text"
          class="mono"
          ref={account === undefined ? first : undefined}
          value={identifier}
          onInput={(event) => {
            setIdEdited(true);
            setId(event.currentTarget.value);
          }}
        />
      </label>
      {idProblem === undefined || (!idEdited && suffixes.length === 0) ? null : (
        <p class="refusal">{idProblem}</p>
      )}
      {account === undefined ? (
        <p class="muted">
          Match counts are per account. Each account&apos;s policy screen shows what this would
          restrict there.
        </p>
      ) : null}
      <fieldset class="suffix-lines">
        <legend>Domain suffixes, one per line</legend>
        {typed.map((line, i) => {
          const v = line.trim() === "" ? undefined : verdictOf(line.trim());
          const described = `suffix-${i}-verdict`;
          return (
            <div key={i} class="suffix-line">
              <input
                type="text"
                class="mono"
                aria-label={`Domain suffix ${i + 1}`}
                aria-describedby={described}
                aria-invalid={v?.refused === true ? "true" : undefined}
                value={line}
                onInput={(event) => type(i, event.currentTarget.value)}
              />
              <span id={described} class="verdict" aria-live="polite">
                {v === undefined ? null : (
                  <>
                    {v.says.map((s, j) => (
                      <span key={j} class={v.refused ? "refusal" : "muted"}>
                        {s}{" "}
                      </span>
                    ))}
                    {v.warning === undefined ? null : <span class="warning">{v.warning}</span>}
                  </>
                )}
              </span>
            </div>
          );
        })}
      </fieldset>
      <p>Class: restricted, the one class.</p>
      {account === undefined ? (
        <p>
          Restricts these domains in every account. From each process&apos;s next policy reload
          their bodies are denied.
        </p>
      ) : (
        <>
          {scope === "base" ? (
            <p>Applies to every account. The counts shown are {account}&apos;s.</p>
          ) : null}
          <p>
            Restricts {counted(counts.senders, "sender")} and{" "}
            {counted(counts.messages, "stored message")} in {account}. From each process&apos;s next
            policy reload their bodies are denied.
          </p>
        </>
      )}
      <button
        type="submit"
        class="primary"
        ref={add}
        disabled={suffixes.length === 0 || idProblem !== undefined || refused}
      >
        Add rule
      </button>
      {failure === undefined ? null : (
        <Refusal
          failure={failure}
          rule={identifier}
          scopeName={scope === "account" ? account : undefined}
        />
      )}
    </form>
  );
}
