// One rule's panel over its scope's policy screen (docs/UI.md sections 8.7 and 8.14). From an account it
// shows the rule's header, each domain suffix with what it matches in the account, the senders it
// matches and its history, and carries Edit domains, Change where this applies… and, set apart on the
// right, Lift restriction. On the base policy's installation screen it shows no account's counts and
// has no Change where this applies…, since moving a rule to one account's own rules names an account.
import type { ComponentChildren } from "preact";
import { useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import {
  basePolicyPath,
  releasePath,
  baseHistoryPath,
  rulePath,
  type BaseRule,
  type ChangeRow,
  type Counts,
  type Failure,
  type RuleDetail,
} from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { count, counted, local, utc } from "../app/format.ts";
import { outcomeKey, type Scope } from "../app/outcomes.ts";
import { Region } from "../app/region.tsx";
import { DetailPanel } from "../lens/panel.tsx";
import { RowsTable } from "../lens/table.tsx";
import { Badge } from "../row/badge.tsx";
import { changeColumns } from "../row/change.tsx";
import { ClassBadge } from "../row/sender.tsx";
import {
  addScreen,
  baseReleaseSentences,
  countLine,
  lifted,
  LiftDialog,
  liftOutcome,
  lines,
  openAdd,
  OutcomeRegion,
  policyHome,
  refreshPolicy,
  Refusal,
  releaseSentences,
  ruleScreen,
  sendWrite,
  shaped,
  suffixChange,
  type Typed,
} from "./policywrites.tsx";

// typedIdentifier is a base rule's lift confirmation, the rule's identifier typed (section 8.7).
export function typedIdentifier(rule: string): Typed {
  return {
    expected: rule,
    label: `Type ${rule} to confirm`,
    until: "Type the identifier to enable",
  };
}

// AccountsLosing lists the accounts a base rule's lift reaches, each linking to its own policy screen,
// where its numbers are.
function AccountsLosing(props: { accounts: readonly string[]; lead: string }) {
  return (
    <p>
      {props.lead}{" "}
      {props.accounts.map((a, i) => (
        <span key={a}>
          {i === 0 ? null : ", "}
          <a class="mono" href={policyHome(a)}>
            {a}
          </a>
        </span>
      ))}
      .
    </p>
  );
}

// accountSays are the lift dialog's sentences on an account's screen, the account's numbers for what
// the lift releases there and what cannot be recalled, and for a base rule the base sentences, the
// accounts that lose it. A change of where a rule applies lifts the old rule once the new one covers
// every suffix, so nothing in the account loses its restriction then (section 8.7).
export function accountSays(props: {
  account: string;
  scope: Scope;
  released: Counts;
  what: "rule" | "suffix" | "suffixes";
  accounts: readonly string[];
  movedTo?: string;
}): ComponentChildren {
  return (
    <>
      {props.movedTo !== undefined && props.released.senders === 0 ? (
        <p>
          Nothing in {props.account} loses its restriction, since {props.movedTo} covers every
          suffix.
        </p>
      ) : (
        <p>
          In {props.account}, {countLine(props.released.senders, props.released.messages)} are
          restricted by {props.what === "suffixes" ? "these suffixes" : `this ${props.what}`} and by
          no other rule.
        </p>
      )}
      <p>{releaseSentences}</p>
      {props.scope === "base" ? (
        <>
          <p>
            This is a base rule. These counts are {props.account}&apos;s. Each account&apos;s policy
            screen shows its own.
          </p>
          <AccountsLosing accounts={props.accounts} lead="Every account loses it:" />
        </>
      ) : null}
    </>
  );
}

// EditSays are the lift dialog's sentences for an edit removing suffixes from a rule on an account's
// screen. The numbers are read for the removed suffixes together, what keeping only the rule's other
// suffixes would release in the account, so a sender that only the removed set covers is counted once
// and counted at all (section 8.7).
function EditSays(props: { account: string; detail: RuleDetail; removed: readonly string[] }) {
  const { releases } = useDeps();
  const { account, detail, removed } = props;
  const keep = detail.suffixes.filter((s) => !removed.includes(s));
  const path = releasePath(account, detail.scope, detail.rule_id, keep);
  return (
    <Region
      name="release"
      state={releases.read(path)}
      shape="block"
      retry={() => void releases.retry(path)}
    >
      {(released) =>
        accountSays({
          account,
          scope: detail.scope,
          released,
          what: removed.length === 1 ? "suffix" : "suffixes",
          accounts: detail.accounts,
        })
      }
    </Region>
  );
}

// baseSays are the lift dialog's sentences on the base policy's installation screen, which shows no
// account's counts (section 8.14).
export function baseSays(accounts: readonly string[]): ComponentChildren {
  return (
    <>
      {accounts.length === 0 ? (
        <p>No account is connected, so nothing is released now.</p>
      ) : (
        <AccountsLosing accounts={accounts} lead="Every account loses this restriction:" />
      )}
      <p>{baseReleaseSentences}</p>
    </>
  );
}

// LiftRule is the lift of a whole rule through the lift dialog. Once lifted the screen reads "Lifted
// {rule}." with Put it back, and the browser goes to after.
export function LiftRule(props: {
  account: string | undefined;
  scope: Scope;
  rule: string;
  suffixes: readonly string[];
  says: ComponentChildren;
  after: string;
  onCancel: () => void;
}) {
  const deps = useDeps();
  const { route } = useLocation();
  const [failure, setFailure] = useState<Failure | undefined>();
  const typed = props.scope === "base" ? typedIdentifier(props.rule) : undefined;
  const lift = async (confirmation: string | null) => {
    setFailure(undefined);
    const result = await sendWrite(deps.post, props.account, props.scope, {
      kind: "lift",
      rule: props.rule,
      before: props.suffixes,
      confirmation,
    });
    if (!result.ok) {
      setFailure(result.failure);
      return;
    }
    deps.outcomes.set(
      outcomeKey(props.account),
      liftOutcome(
        [props.rule],
        [{ scope: props.scope, rule: props.rule, suffixes: props.suffixes, onto: undefined }],
      ),
    );
    refreshPolicy(deps);
    route(props.after);
  };
  return (
    <LiftDialog
      title={`Lift the restriction on ${props.rule}`}
      typed={typed}
      action={`Lift restriction on ${props.rule}`}
      refusal={
        failure === undefined ? undefined : (
          <Refusal
            failure={failure}
            rule={props.rule}
            scopeName={props.scope === "account" ? props.account : undefined}
          />
        )
      }
      onConfirm={(t) => void lift(t)}
      onCancel={props.onCancel}
    >
      {props.says}
    </LiftDialog>
  );
}

// EditDomains edits a rule's suffixes, one per line. An edit that only adds saves at once. One that
// removes any suffix shows the change as a diff and saves through the lift dialog for the removed ones.
// An edit that would remove every suffix is not an edit, since a rule needs one, and Lift restriction is
// offered instead (section 8.7).
export function EditDomains(props: {
  account: string | undefined;
  scope: Scope;
  rule: string;
  suffixes: readonly string[];
  added: readonly string[];
  says: (removed: readonly string[]) => ComponentChildren;
  onLiftWhole: () => void;
  onClose: () => void;
}) {
  const deps = useDeps();
  const { route } = useLocation();
  const [text, setText] = useState(
    [...props.suffixes, ...props.added.filter((s) => !props.suffixes.includes(s))].join("\n"),
  );
  const [confirming, setConfirming] = useState(false);
  const [failure, setFailure] = useState<Failure | undefined>();
  const after = lines(text.split("\n"));
  const change = suffixChange(props.suffixes, after);
  const misshaped = after.filter((s) => !shaped(s));
  const unchanged = change.added.length === 0 && change.removed.length === 0;
  const scopeName = props.scope === "account" ? props.account : undefined;
  const save = async (confirmation: string | null) => {
    setFailure(undefined);
    const result = await sendWrite(deps.post, props.account, props.scope, {
      kind: "edit",
      rule: props.rule,
      before: props.suffixes,
      suffixes: after,
      confirmation,
    });
    if (!result.ok) {
      setFailure(result.failure);
      if (result.failure.code === "stale_rule") {
        refreshPolicy(deps);
      }
      return;
    }
    deps.outcomes.set(
      outcomeKey(props.account),
      change.removed.length === 0
        ? { text: `Added ${lifted(change.added)} to ${props.rule}.`, putBack: [], putBackLabel: "" }
        : liftOutcome(change.removed, [
            { scope: props.scope, rule: props.rule, suffixes: change.removed, onto: after },
          ]),
    );
    refreshPolicy(deps);
    setConfirming(false);
    props.onClose();
    route(ruleScreen(props.account, props.scope, props.rule), true);
  };
  const refusal =
    failure === undefined ? undefined : (
      <Refusal failure={failure} rule={props.rule} scopeName={scopeName} />
    );
  return (
    <section class="edit-domains" aria-label="Edit domains">
      <label>
        Domain suffixes, one per line
        <textarea
          rows={Math.max(3, after.length + 1)}
          value={text}
          onInput={(event) => setText(event.currentTarget.value)}
        />
      </label>
      {unchanged ? null : (
        <ul class="diff" aria-label="The change">
          {change.added.map((s) => (
            <li key={`+${s}`} class="suffix-chip mono" data-change="added">
              +{s}
            </li>
          ))}
          {change.removed.map((s) => (
            <li key={`-${s}`} class="suffix-chip mono" data-change="removed">
              −{s}
            </li>
          ))}
        </ul>
      )}
      {misshaped.map((s) => (
        <p key={s} class="refusal">
          <span class="mono">{s}</span> is not shaped like a domain name.
        </p>
      ))}
      {after.length === 0 ? (
        <p>
          A rule needs a domain suffix, so removing every one is not an edit.{" "}
          <button type="button" class="lift" onClick={props.onLiftWhole}>
            Lift restriction
          </button>
        </p>
      ) : (
        <button
          type="button"
          class="primary"
          disabled={unchanged || misshaped.length > 0}
          onClick={() => (change.removed.length === 0 ? void save(null) : setConfirming(true))}
        >
          Save
        </button>
      )}
      <button type="button" onClick={props.onClose}>
        Cancel
      </button>
      {confirming ? null : refusal}
      {confirming ? (
        <LiftDialog
          title={`Lift the restriction on ${lifted(change.removed)}`}
          typed={props.scope === "base" ? typedIdentifier(props.rule) : undefined}
          action={`Lift restriction on ${lifted(change.removed)}`}
          refusal={refusal}
          onConfirm={(t) => void save(t)}
          onCancel={() => setConfirming(false)}
        >
          {props.says(change.removed)}
        </LiftDialog>
      ) : null}
    </section>
  );
}

// RuleHeader is a rule's identifier, scope, class, source and created time and identity.
function RuleHeader(props: {
  scope: string;
  rule: { rule_id: string; class: string; source: string; created_at: string; created_by: string };
}) {
  const { rule } = props;
  return (
    <dl class="provenance">
      <dt>identifier</dt>
      <dd class="mono">{rule.rule_id}</dd>
      <dt>scope</dt>
      <dd>
        <Badge tone="muted" text={props.scope} />
      </dd>
      <dt>class</dt>
      <dd>{rule.class}</dd>
      <dt>source</dt>
      <dd>{rule.source}</dd>
      <dt>created</dt>
      <dd title={local(rule.created_at)}>
        {utc(rule.created_at)} · <span class="mono">{rule.created_by}</span>
      </dd>
    </dl>
  );
}

// History is a rule's history as policy change rows, newest first.
function History(props: { account: string | undefined; rows: readonly ChangeRow[] }) {
  return (
    <RowsTable
      caption="History"
      rowsName="changes"
      columns={changeColumns(props.account)}
      rows={props.rows}
      rowKey={(row) => row.id}
      page={1}
      pages={1}
      pageHref={() => ""}
      keys={false}
    />
  );
}

// addedOf reads the suffixes the address adds, which open Edit domains with them added.
function addedOf(url: string): string[] {
  const query = url.includes("?") ? url.slice(url.indexOf("?") + 1) : "";
  return new URLSearchParams(query).getAll("suffix");
}

// RulePanel is one rule of the account's policy. With no scope named, the address names the account's
// own rule of the identifier, or the base rule when the account holds none (section 8.7), so the
// account's rule is read first and the base rule on its refusal as no such row.
export function RulePanel(props: { account: string; scope: Scope | undefined; rule: string }) {
  const { rules } = useDeps();
  const { url } = useLocation();
  const { account, rule } = props;
  const ownPath = rulePath(account, props.scope ?? "account", rule);
  const own = rules.read(ownPath);
  const ownState = own.value;
  const fallback =
    props.scope === undefined &&
    ownState.status === "error" &&
    ownState.failure.code === "unknown_row";
  const path = fallback ? rulePath(account, "base", rule) : ownPath;
  const state = fallback ? rules.read(path) : own;
  return (
    <DetailPanel title={`Rule ${rule}`} back={policyHome(account)}>
      <Region name="rule" state={state} shape="block" retry={() => void rules.retry(path)}>
        {(detail) => <RuleBody key={url} account={account} detail={detail} added={addedOf(url)} />}
      </Region>
    </DetailPanel>
  );
}

function RuleBody(props: { account: string; detail: RuleDetail; added: readonly string[] }) {
  const { account, detail } = props;
  const [editing, setEditing] = useState(props.added.length > 0);
  const [lifting, setLifting] = useState(false);
  const [moving, setMoving] = useState(false);
  const other: Scope = detail.scope === "base" ? "account" : "base";
  const says = (removed: readonly string[]) => (
    <EditSays account={account} detail={detail} removed={removed} />
  );
  return (
    <>
      <OutcomeRegion account={account} />
      <RuleHeader scope={detail.scope === "base" ? "base" : account} rule={detail} />
      {detail.scope === "base" ? (
        <p>
          Every account inherits this rule.{" "}
          <a href={ruleScreen(undefined, "base", detail.rule_id)}>
            Open it on the base policy screen
          </a>
        </p>
      ) : null}
      {detail.elsewhere === null ? null : (
        <p>
          {detail.elsewhere.scope === "base"
            ? `The base policy also has a rule ${detail.rule_id}.`
            : `${account} also has a rule ${detail.rule_id} of its own.`}{" "}
          <a href={ruleScreen(account, detail.elsewhere.scope, detail.rule_id)}>Open it</a>
        </p>
      )}
      <div class="rule-actions">
        <button type="button" onClick={() => setEditing(true)}>
          Edit domains
        </button>
        <button type="button" aria-expanded={moving} onClick={() => setMoving(!moving)}>
          Change where this applies…
        </button>
        <button type="button" class="lift" onClick={() => setLifting(true)}>
          Lift restriction
        </button>
      </div>
      {moving ? (
        <p class="move-choice">
          <button
            type="button"
            onClick={() =>
              openAdd(
                addScreen(account, { suffixes: detail.suffixes, scope: other, id: detail.rule_id }),
                "move",
              )
            }
          >
            {other === "account" ? `Only ${account}` : "Every account"}
          </button>
        </p>
      ) : null}
      {editing ? (
        <EditDomains
          account={account}
          scope={detail.scope}
          rule={detail.rule_id}
          suffixes={detail.suffixes}
          added={props.added}
          says={says}
          onLiftWhole={() => setLifting(true)}
          onClose={() => setEditing(false)}
        />
      ) : null}
      {lifting ? (
        <LiftRule
          account={account}
          scope={detail.scope}
          rule={detail.rule_id}
          suffixes={detail.suffixes}
          says={accountSays({
            account,
            scope: detail.scope,
            released: detail.released,
            what: "rule",
            accounts: detail.accounts,
          })}
          after={policyHome(account)}
          onCancel={() => setLifting(false)}
        />
      ) : null}
      <h3>Domain suffixes</h3>
      <ul class="suffix-details">
        {detail.suffix_details.map((d) => (
          <li key={d.suffix}>
            <span class="mono">{d.suffix}</span> matches {counted(d.senders, "sender")} ·{" "}
            {counted(d.messages, "message")} in {account}
          </li>
        ))}
      </ul>
      <h3>Senders it matches</h3>
      {detail.matched.length === 0 ? (
        <p class="muted">It matches no sender in {account}.</p>
      ) : (
        <>
          <ul class="matched">
            {detail.matched.map((s) => (
              <li key={s.domain}>
                <span class="mono">{s.domain}</span> <ClassBadge stored={s.sender_class} />{" "}
                {counted(s.messages, "message")}
              </li>
            ))}
          </ul>
          {detail.matched_total > detail.matched.length ? (
            <p class="muted">
              {count(detail.matched.length)} of {count(detail.matched_total)} shown, most messages
              first.
            </p>
          ) : null}
        </>
      )}
      <h3>History</h3>
      <History account={account} rows={detail.history} />
    </>
  );
}

// BaseRulePanel is one base rule on the base policy's installation screen, read from the base policy's
// list and its history, with no count of any account's (section 8.14).
export function BaseRulePanel(props: { rule: string }) {
  const { basePolicy } = useDeps();
  const { url } = useLocation();
  const path = basePolicyPath("");
  return (
    <DetailPanel title={`Rule ${props.rule}`} back={policyHome(undefined)}>
      <Region
        name="base policy"
        state={basePolicy.read(path)}
        shape="block"
        retry={() => void basePolicy.retry(path)}
      >
        {(answer) => {
          const rule = answer.rules.find((r) => r.rule_id === props.rule);
          return rule === undefined ? (
            <p>The base policy holds no rule {props.rule}.</p>
          ) : (
            <BaseRuleBody key={url} rule={rule} accounts={answer.accounts} added={addedOf(url)} />
          );
        }}
      </Region>
    </DetailPanel>
  );
}

function BaseRuleBody(props: {
  rule: BaseRule;
  accounts: readonly string[];
  added: readonly string[];
}) {
  const { rule } = props;
  const { baseHistory } = useDeps();
  const [editing, setEditing] = useState(props.added.length > 0);
  const [lifting, setLifting] = useState(false);
  const history = baseHistoryPath("all", rule.rule_id);
  return (
    <>
      <OutcomeRegion account={undefined} />
      <RuleHeader scope="base" rule={rule} />
      <p>
        Only for one account? Open that account&apos;s policy and use Change where this applies…
      </p>
      <div class="rule-actions">
        <button type="button" onClick={() => setEditing(true)}>
          Edit domains
        </button>
        <button type="button" class="lift" onClick={() => setLifting(true)}>
          Lift restriction
        </button>
      </div>
      {editing ? (
        <EditDomains
          account={undefined}
          scope="base"
          rule={rule.rule_id}
          suffixes={rule.suffixes}
          added={props.added}
          says={() => baseSays(props.accounts)}
          onLiftWhole={() => setLifting(true)}
          onClose={() => setEditing(false)}
        />
      ) : null}
      {lifting ? (
        <LiftRule
          account={undefined}
          scope="base"
          rule={rule.rule_id}
          suffixes={rule.suffixes}
          says={baseSays(props.accounts)}
          after={policyHome(undefined)}
          onCancel={() => setLifting(false)}
        />
      ) : null}
      <h3>Domain suffixes</h3>
      <ul class="suffix-details">
        {rule.suffixes.map((s) => (
          <li key={s} class="mono">
            {s}
          </li>
        ))}
      </ul>
      <h3>History</h3>
      <Region
        name="history"
        state={baseHistory.read(history)}
        shape="table"
        retry={() => void baseHistory.retry(history)}
      >
        {(answer) => <History account={undefined} rows={answer.rows} />}
      </Region>
    </>
  );
}
