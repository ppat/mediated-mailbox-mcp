// Account settings of docs/UI.md section 8.13. The account's identifier, provider, mailbox and client,
// then its credential's health, its rate target and what the UI reads of its two rows. It never shows
// the credential, which the UI cannot read (ADR-0084).
import { useState } from "preact/hooks";
import {
  accountPath,
  send,
  type AccountSettings,
  type Failure,
  type TargetAnswer,
} from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { ListedAccount, providerName, settingsPath } from "../app/frame.tsx";
import { utc } from "../app/format.ts";
import { Region } from "../app/region.tsx";
import { Values, type Value } from "./system.tsx";
import { setupPath } from "./clientsetup.tsx";

export function AccountScreen(props: { account: string }) {
  const { settings } = useDeps();
  const path = accountPath(props.account);
  return (
    <section aria-labelledby="account-title" class="account-settings">
      <ListedAccount account={props.account}>
        <Region
          name="account"
          state={settings.read(path)}
          shape="block"
          retry={() => void settings.retry(path)}
        >
          {(answer) => <Settings answer={answer} />}
        </Region>
      </ListedAccount>
    </section>
  );
}

function Settings(props: { answer: AccountSettings }) {
  const { answer } = props;
  const reauthorize = `${settingsPath(answer.account_id)}/reauthorize`;
  return (
    <>
      <h1 id="account-title">
        <span class="mono">{answer.account_id}</span> · {providerName(answer.provider)}
      </h1>
      <p>{answer.mailbox ?? <span class="muted">no mailbox remembered</span>}</p>
      {answer.oauth_client === null ? null : (
        <p>
          through{" "}
          <a class="mono" href={setupPath(answer.provider, answer.oauth_client, 7)}>
            {answer.oauth_client}
          </a>
          {answer.other_clients.length > 0 ? (
            <>
              {" · "}
              <a href={reauthorize}>Move to another client</a>
            </>
          ) : null}
        </p>
      )}
      <Credential answer={answer} reauthorize={reauthorize} />
      <Target answer={answer} />
      <section aria-labelledby="rows-title">
        <h2 id="rows-title">What this account&apos;s rows hold</h2>
        <Values label="The account's rows" values={rows(answer)} />
        <a href={`/${encodeURIComponent(answer.account_id)}/system`}>
          See System for the live view
        </a>
      </section>
    </>
  );
}

// Health is the credential's health as the latest recorded authentication reads it (section 8.13).
type Health = {
  badge: string;
  tone: "ok" | "restricted" | "muted";
  sentence: string;
  primary: boolean;
};

export function health(answer: AccountSettings): Health {
  const provider = providerName(answer.provider);
  if (!answer.connected) {
    return {
      badge: "not connected",
      tone: "muted",
      sentence: "This account is not connected.",
      primary: true,
    };
  }
  const at = answer.last_auth_at === null ? "" : utc(answer.last_auth_at);
  switch (answer.last_auth_outcome) {
    case "succeeded":
      return {
        badge: "accepted",
        tone: "ok",
        sentence: `${provider} accepted the credential at ${at}.`,
        primary: false,
      };
    case "refused":
      return {
        badge: "refused",
        tone: "restricted",
        sentence: `${provider} refused the credential. Every workload that calls ${provider} fails for this account until it is re-authorized. The usual causes are access revoked in the Google account, a password change, a client left in Testing, and the account's client, ${answer.oauth_client ?? "its client"}, deleted or replaced.`,
        primary: true,
      };
    case "failed":
      return {
        badge: "no answer",
        tone: "muted",
        sentence: `The last attempt got no answer ${provider} could read. Workloads retry on their own, so nothing is needed unless this persists. If this lasts past an hour, check that the deployment reaches Google's token endpoint.`,
        primary: false,
      };
    case null:
      return {
        badge: "not used yet",
        tone: "muted",
        sentence: "No workload has authenticated yet.",
        primary: false,
      };
    default:
      // A stored value the wording does not know is shown as itself, never hidden (section 7.1). It
      // is text a deployable recorded from its provider adapter's report, so it renders inert.
      return {
        badge: "unknown",
        tone: "muted",
        sentence: answer.last_auth_outcome,
        primary: false,
      };
  }
}

function Credential(props: { answer: AccountSettings; reauthorize: string }) {
  const h = health(props.answer);
  return (
    <section aria-labelledby="credential-title">
      <h2 id="credential-title">Credential</h2>
      <p>
        <span class="badge" data-tone={h.tone}>
          {h.badge}
        </span>{" "}
        {h.sentence}
      </p>
      <a class={h.primary ? "primary" : "outlined"} href={props.reauthorize}>
        {props.answer.connected ? "Re-authorize" : "Connect"}
      </a>
    </section>
  );
}

// targetText is the rate target's sentence, the default or the lowered percent (section 8.13).
export function targetText(answer: AccountSettings): string {
  const provider = providerName(answer.provider);
  return answer.lowered_target === null
    ? `Default, half of ${provider}'s declared ceiling`
    : `Lowered to ${Math.round(answer.lowered_target * 100)}% of ${provider}'s declared ceiling`;
}

// Target shows the rate target and its field. The target in units per second shows only as the rate
// state reports it, since the UI does not compute ADR-0024's fractions, and lowering asks no
// confirmation, since a lower target only slows the account's work (section 8.13).
function Target(props: { answer: AccountSettings }) {
  const { answer } = props;
  const deps = useDeps();
  const [percent, setPercent] = useState(
    answer.lowered_target === null ? "" : String(Math.round(answer.lowered_target * 100)),
  );
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const save = async (fraction: number | null) => {
    setFailure(undefined);
    const result = await send<TargetAnswer>(deps.post, `${accountPath(answer.account_id)}/target`, {
      lowered_target: fraction,
    });
    if (result.ok) {
      void deps.settings.refresh(accountPath(answer.account_id));
      if (fraction === null) {
        setPercent("");
      }
    } else {
      setFailure(result.failure);
    }
  };
  return (
    <section aria-labelledby="target-title">
      <h2 id="target-title">Rate target</h2>
      <p>
        {targetText(answer)}
        {answer.current_target === null ? null : (
          <span class="muted"> · now {answer.current_target.toFixed(1)} units/s</span>
        )}
      </p>
      {answer.connected ? (
        <>
          <label>
            Percent of the declared ceiling{" "}
            <input
              type="number"
              min="6"
              max="50"
              value={percent}
              onInput={(e) => setPercent(e.currentTarget.value)}
            />
            %
          </label>
          <button
            type="button"
            class="primary"
            disabled={percent === ""}
            onClick={() => void save(Number(percent) / 100)}
          >
            Save
          </button>
          <button type="button" onClick={() => void save(null)}>
            Reset to default
          </button>
          {failure === undefined ? null : (
            <p class="refusal" role="alert">
              {failure.code === "target_refused"
                ? "A lowered target is above 5% and at most 50% of the declared ceiling."
                : failure.code === "stale_page"
                  ? "This page is older than the UI server's last restart. Reload, then try again."
                  : failure.message}
            </p>
          )}
        </>
      ) : null}
    </section>
  );
}

function yesNo(b: boolean): string {
  return b ? "complete" : "not complete";
}

// rows is what the UI reads of the account's two rows, never the credential (section 8.13).
export function rows(answer: AccountSettings): Value[] {
  return [
    { key: "id", label: "Identifier", text: answer.account_id, mono: true },
    { key: "provider", label: "Provider", text: answer.provider },
    { key: "client", label: "Client", text: answer.oauth_client ?? "none", mono: true },
    { key: "mailbox", label: "Mailbox", text: answer.mailbox ?? "none remembered" },
    { key: "connected", label: "Connected", text: answer.connected ? "yes" : "no" },
    {
      key: "auth",
      label: "Last authentication",
      text: answer.last_auth_outcome ?? "none recorded",
      note: answer.last_auth_at === null ? undefined : utc(answer.last_auth_at),
    },
    { key: "target", label: "Rate target", text: targetText(answer) },
    { key: "pass1", label: "Backfill pass 1", text: yesNo(answer.backfill_pass1_complete) },
    { key: "pass2", label: "Backfill pass 2", text: yesNo(answer.backfill_pass2_complete) },
    {
      key: "cursor",
      label: "Sync cursor written",
      text: answer.sync_cursor_at === null ? "never" : utc(answer.sync_cursor_at),
    },
  ];
}
