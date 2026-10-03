// The installation screen of docs/UI.md section 8.10, what belongs to no account. It reads the
// installation endpoint alone, which carries each OAuth client's name, provider, identifier and
// project ID and each account's identifier, provider and client, and nothing of any account's state
// (ADR-0056). Of the base policy it reads only the count of base rules, which belongs to no account.
import { useState } from "preact/hooks";
import {
  installationPath,
  send,
  type Client,
  type Failure,
  type Installation,
  type RemovedClient,
} from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { count } from "../app/format.ts";
import { providerName } from "../app/frame.tsx";
import { consoleTab } from "../app/guide.ts";
import { Region } from "../app/region.tsx";
import { clientsConsole } from "./steps.ts";

export function InstallationScreen() {
  const { installation } = useDeps();
  const path = installationPath();
  return (
    <section aria-labelledby="installation-title" class="setup">
      <h1 id="installation-title">Installation</h1>
      <p>
        What belongs to no account lives here, the OAuth clients accounts connect through and the
        accounts themselves.
      </p>
      <Region
        name="installation"
        state={installation.read(path)}
        shape="block"
        retry={() => void installation.retry(path)}
      >
        {(answer) => <Regions answer={answer} />}
      </Region>
    </section>
  );
}

function Regions(props: { answer: Installation }) {
  const { answer } = props;
  const starting = answer.accounts.length === 0;
  return (
    <>
      {starting ? <GettingStarted answer={answer} /> : null}
      <Clients answer={answer} />
      <Accounts answer={answer} starting={starting} />
    </>
  );
}

// hasClient reports whether any provider an account can use has a client.
function hasClient(answer: Installation): boolean {
  return answer.clients.some((c) => answer.providers.includes(c.provider));
}

// baseRulesStatus is the base policy item's status, its count of base rules, optional either way.
export function baseRulesStatus(n: number): string {
  if (n === 0) {
    return "No base rules yet · optional";
  }
  return `${count(n)} ${n === 1 ? "base rule" : "base rules"} · optional`;
}

// GettingStarted is the first run, shown until an account exists. Connecting cannot start until a
// client is set up, and the current item carries its action. Reviewing the base policy is optional and
// never blocks the next item, so it is never the current one, and it links to the base policy, since a
// rule in place before the first account connects classifies that account's senders from its first
// backfill (section 8.10).
function GettingStarted(props: { answer: Installation }) {
  const ready = hasClient(props.answer);
  return (
    <section aria-labelledby="getting-started" class="getting-started">
      <h2 id="getting-started">Getting started</h2>
      <div class="before-you-start">
        <h3>Before you start</h3>
        <p>
          The whole setup takes about 15 minutes. It needs a Google account allowed to create Cloud
          projects, which need not be the mailbox, and the Gmail address to connect. Chrome or Edge
          keeps the guide on top of Google&apos;s console. Google warns that the app is unverified,
          which is expected.
        </p>
      </div>
      <ol class="checklist">
        <li>
          <Status state={ready ? "done" : "current"} />{" "}
          <a href="/setup/gmail/new">Set up a Gmail OAuth client</a>
          {ready ? null : (
            <a class="primary" href="/setup/gmail/new">
              Start
            </a>
          )}
        </li>
        <li>
          <span class="step-status" data-state="optional">
            <span aria-hidden="true">○</span> {baseRulesStatus(props.answer.base_rules)}
          </span>{" "}
          <a href="/setup/policy">Review the base policy</a>
        </li>
        <li>
          <Status state={ready ? "current" : "blocked"} />{" "}
          {ready ? (
            <>
              <a href="/setup/connect">Connect an account</a>
              <a class="primary" href="/setup/connect">
                Connect
              </a>
            </>
          ) : (
            <span class="faint">Connect an account</span>
          )}
        </li>
      </ol>
    </section>
  );
}

// Status is a setup step's status as text and a glyph, never told by color alone (section 14.2).
export function Status(props: { state: "done" | "current" | "blocked" }) {
  const words = { done: "done", current: "not started", blocked: "cannot start yet" } as const;
  const glyphs = { done: "●", current: "○", blocked: "◌" } as const;
  return (
    <span class="step-status" data-state={props.state}>
      <span aria-hidden="true">{glyphs[props.state]}</span> {words[props.state]}
    </span>
  );
}

// shortened is a client identifier cut for its column, its full value shown on hover.
function shortened(id: string): string {
  return id.length > 24 ? `${id.slice(0, 24)}…` : id;
}

function Clients(props: { answer: Installation }) {
  const { answer } = props;
  return (
    <section aria-labelledby="clients-title">
      <h2 id="clients-title">OAuth clients</h2>
      <p>
        Accounts can share a client, or each use their own. A client set up in an
        organization&apos;s own Cloud project may choose the Internal audience, and then serves only
        that organization&apos;s mailboxes.
      </p>
      {answer.providers.map((provider) => {
        const clients = answer.clients.filter((c) => c.provider === provider);
        return (
          <div key={provider} class="provider-clients">
            <h3>{providerName(provider)}</h3>
            {clients.length === 0 ? (
              <p class="muted">No {providerName(provider)} client yet</p>
            ) : (
              <table class="rows" aria-label={`${providerName(provider)} clients`}>
                <thead>
                  <tr>
                    <th scope="col">Name</th>
                    <th scope="col">Client</th>
                    <th scope="col">Project</th>
                    <th scope="col">Accounts</th>
                    <th scope="col">Actions</th>
                  </tr>
                </thead>
                <tbody>
                  {clients.map((c) => (
                    <ClientRow key={c.name} client={c} />
                  ))}
                </tbody>
              </table>
            )}
            <a href={`/setup/${encodeURIComponent(provider)}/new`}>
              Add a {providerName(provider)} client
            </a>
          </div>
        );
      })}
    </section>
  );
}

// inUse is why a client an account connects through cannot be removed (section 8.10).
export function inUse(n: number): string {
  return n === 1
    ? "1 account connects through this client. Move it to another client from its settings first."
    : `${n} accounts connect through this client. Move them to another client from their settings first.`;
}

function ClientRow(props: { client: Client }) {
  const { client } = props;
  const { post, installation } = useDeps();
  const [confirming, setConfirming] = useState(false);
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const remove = async () => {
    const result = await send<RemovedClient>(
      post,
      `/api/setup/${encodeURIComponent(client.provider)}/clients/${encodeURIComponent(client.name)}/remove`,
      {},
    );
    setConfirming(false);
    if (result.ok) {
      void installation.refresh(installationPath());
    } else {
      setFailure(result.failure);
    }
  };
  const used = client.accounts.length;
  return (
    <tr>
      <td class="mono">{client.name}</td>
      <td class="mono" title={client.client_id}>
        {shortened(client.client_id)}
      </td>
      <td class="mono">{client.project_id ?? ""}</td>
      <td>
        {used === 0 ? (
          <span class="muted">no account yet</span>
        ) : (
          client.accounts.map((a) => (
            <span key={a} class="mono account-name">
              {a}
            </span>
          ))
        )}
      </td>
      <td class="actions">
        {client.project_id === null ? null : (
          <a href={clientsConsole(client.project_id)} target={consoleTab}>
            Open in Google Cloud console
          </a>
        )}
        <a
          href={`/setup/${encodeURIComponent(client.provider)}/${encodeURIComponent(client.name)}`}
        >
          Replace
        </a>
        <button
          type="button"
          disabled={used > 0}
          title={used > 0 ? inUse(used) : undefined}
          onClick={() => setConfirming(true)}
        >
          Remove
        </button>
        {used > 0 ? <span class="muted">{inUse(used)}</span> : null}
        {confirming ? (
          <div class="dialog" role="dialog" aria-modal="true" aria-label={`Remove ${client.name}`}>
            <p>
              Remove the client <span class="mono">{client.name}</span>? Its sealed secret is
              deleted here. The client itself stays in Google Cloud console, where you can delete
              it.
            </p>
            <button type="button" class="primary" onClick={() => void remove()}>
              Remove {client.name}
            </button>
            <button type="button" onClick={() => setConfirming(false)}>
              Keep it
            </button>
          </div>
        ) : null}
        {failure === undefined ? null : (
          <p class="refusal" role="alert">
            {failure.message}
          </p>
        )}
      </td>
    </tr>
  );
}

function Accounts(props: { answer: Installation; starting: boolean }) {
  const { answer } = props;
  const ready = hasClient(answer);
  return (
    <section aria-labelledby="accounts-title">
      <h2 id="accounts-title">Accounts</h2>
      {answer.accounts.length === 0 ? (
        <p class="muted">No account is connected yet.</p>
      ) : (
        <table class="rows" aria-label="Accounts">
          <thead>
            <tr>
              <th scope="col">Account</th>
              <th scope="col">Provider</th>
              <th scope="col">Client</th>
              <th scope="col">Settings</th>
            </tr>
          </thead>
          <tbody>
            {answer.accounts.map((a) => (
              <tr key={a.account_id}>
                <td class="mono">
                  <a href={`/${encodeURIComponent(a.account_id)}`}>{a.account_id}</a>
                </td>
                <td>{a.provider}</td>
                <td class="mono">{a.oauth_client ?? ""}</td>
                <td>
                  <a href={`/${encodeURIComponent(a.account_id)}/account`}>settings</a>
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      )}
      {props.starting ? null : ready ? (
        <a class="primary" href="/setup/connect">
          Connect an account
        </a>
      ) : (
        <p>
          <button type="button" disabled>
            Connect an account
          </button>{" "}
          <span class="muted">Set up an OAuth client first.</span>
        </p>
      )}
    </section>
  );
}
