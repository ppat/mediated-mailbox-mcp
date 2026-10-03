// Connecting an account and re-authorizing one, docs/UI.md section 8.12, one page of three steps. The
// first names the account and starts the session's consent attempt, the second sends the operator to
// Google's consent page, and the third takes the address Google redirected the browser to. Nothing is
// stored until the third succeeds, and the attempt is the server's, so a reload restores it.
import { h, render } from "preact";
import { useCallback, useEffect, useMemo, useRef, useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import {
  accountPath,
  connectPath,
  installationPath,
  reauthorizePath,
  send,
  type AccountSettings,
  type AttemptAnswer,
  type Client,
  type ConsentAttempt,
  type Connected,
  type Failure,
  type Installation,
} from "../app/api.ts";
import { DepsContext, useDeps } from "../app/deps.ts";
import { Frame, InstallationFrame, providerName } from "../app/frame.tsx";
import { count, duration } from "../app/format.ts";
import {
  guideWindow,
  linkStylesheet,
  pipSize,
  readPasted,
  redirectHost,
  sideFeatures,
  type PasteChecks,
} from "../app/guide.ts";
import { Region } from "../app/region.tsx";
import { setupPath } from "./clientsetup.tsx";

// What the page does, connect a new account or re-authorize the account in the path.
type Mode = { kind: "connect" } | { kind: "reauthorize"; account: string };

// ConnectRoute is the installation's connect page, or with guide=paste the compact layout of the paste
// box's side window, which holds only the paste field and its checks (section 8.12).
export function ConnectRoute() {
  const { query } = useLocation();
  if (query["guide"] === "paste") {
    return (
      <main class="guide">
        <SidePasteBox />
      </main>
    );
  }
  return (
    <InstallationFrame>
      <ConnectPage mode={{ kind: "connect" }} />
    </InstallationFrame>
  );
}

// ReauthorizeRoute is the account's re-authorization, the same page with step 1 a summary.
export function ReauthorizeRoute(props: { account: string }) {
  return (
    <Frame account={props.account}>
      <ConnectPage mode={{ kind: "reauthorize", account: props.account }} />
    </Frame>
  );
}

function ConnectPage(props: { mode: Mode }) {
  const { installation, settings } = useDeps();
  const path = installationPath();
  const { mode } = props;
  return (
    <section aria-labelledby="connect-title" class="connect">
      <Region
        name="installation"
        state={installation.read(path)}
        shape="block"
        retry={() => void installation.retry(path)}
      >
        {(answer) =>
          mode.kind === "connect" ? (
            <Attempted mode={mode} answer={answer} account={undefined} />
          ) : (
            <Region
              name="account"
              state={settings.read(accountPath(mode.account))}
              shape="block"
              retry={() => void settings.retry(accountPath(mode.account))}
            >
              {(account) => <Attempted mode={mode} answer={answer} account={account} />}
            </Region>
          )
        }
      </Region>
    </section>
  );
}

function attemptPathOf(mode: Mode): string {
  return mode.kind === "connect" ? connectPath() : reauthorizePath(mode.account);
}

// Attempted reads the session's attempt, which a reload restores, then shows the steps.
function Attempted(props: {
  mode: Mode;
  answer: Installation;
  account: AccountSettings | undefined;
}) {
  const { attempts } = useDeps();
  const path = attemptPathOf(props.mode);
  return (
    <Region
      name="attempt"
      state={attempts.read(path)}
      shape="block"
      retry={() => void attempts.retry(path)}
    >
      {(held) => <Steps {...props} restored={held.attempt} />}
    </Region>
  );
}

type StepsProps = {
  mode: Mode;
  answer: Installation;
  account: AccountSettings | undefined;
  restored: ConsentAttempt | null;
};

// Outcome is what finishing stored.
type Outcome = { connected: Connected };

function Steps(props: StepsProps) {
  const { mode, answer, account } = props;
  const deps = useDeps();
  const { query } = useLocation();
  const [attempt, setAttempt] = useState<ConsentAttempt | null>(props.restored);
  const [replaced, setReplaced] = useState(false);
  const [outcome, setOutcome] = useState<Outcome | undefined>(undefined);
  const path = attemptPathOf(mode);
  // A tab whose attempt a newer one replaced shows so when it next has focus (section 8.12).
  useEffect(() => {
    if (attempt === null) {
      return undefined;
    }
    const check = () => {
      void deps.attempts.refresh(path).then(() => {
        const now = deps.attempts.read(path).peek();
        if (
          now.status === "ok" &&
          now.answer.attempt?.attempt !== attempt.attempt &&
          outcome === undefined
        ) {
          setReplaced(true);
        }
      });
    };
    window.addEventListener("focus", check);
    return () => window.removeEventListener("focus", check);
  }, [attempt, deps, path, outcome]);
  if (outcome !== undefined) {
    return <Success mode={mode} connected={outcome.connected} answer={answer} />;
  }
  const title = mode.kind === "connect" ? "Connect an account" : `Re-authorize ${mode.account}`;
  return (
    <>
      <h1 id="connect-title">{title}</h1>
      {replaced ? (
        <p class="refusal" role="alert">
          A newer connection in this browser replaced this one.{" "}
          <button
            type="button"
            onClick={() => {
              setAttempt(null);
              setReplaced(false);
            }}
          >
            Start again
          </button>
        </p>
      ) : null}
      <NameStep
        mode={mode}
        answer={answer}
        account={account}
        chosen={query["client"]}
        attempt={attempt}
        onStarted={(a) => {
          setAttempt(a);
          setReplaced(false);
          void deps.attempts.refresh(path);
        }}
      />
      {attempt === null ? null : (
        <>
          <GrantStep attempt={attempt} answer={answer} />
          <PasteStep
            mode={mode}
            attempt={attempt}
            answer={answer}
            onConnected={(connected) => {
              setOutcome({ connected });
              void deps.installation.refresh(installationPath());
              void deps.accounts.refresh("/api/accounts");
              if (mode.kind === "reauthorize") {
                void deps.settings.refresh(accountPath(mode.account));
              }
            }}
          />
        </>
      )}
    </>
  );
}

// proposeIdentifier proposes an account identifier from a mailbox's local part, lowercased, with each
// run of characters outside letters, digits and hyphens replaced by a hyphen, and a suffix added when
// an account already holds it (section 8.12). The operator may edit what it proposes.
export function proposeIdentifier(mailbox: string, taken: readonly string[]): string {
  const at = mailbox.lastIndexOf("@");
  const local = (at < 0 ? mailbox : mailbox.slice(0, at)).toLowerCase();
  const base = local.replace(/[^a-z0-9-]+/g, "-").replace(/^-+|-+$/g, "") || "account";
  if (!taken.includes(base)) {
    return base;
  }
  for (let n = 2; ; n++) {
    const candidate = `${base}-${n}`;
    if (!taken.includes(candidate)) {
      return candidate;
    }
  }
}

// clientsFor are the clients an account can connect through, of the providers whose accounts connect
// through one.
function clientsFor(answer: Installation): Client[] {
  return answer.clients.filter((c) => answer.providers.includes(c.provider));
}

type NameProps = {
  mode: Mode;
  answer: Installation;
  account: AccountSettings | undefined;
  chosen: string | undefined;
  attempt: ConsentAttempt | null;
  onStarted: (attempt: ConsentAttempt) => void;
};

// NameStep is step 1. Connecting names the mailbox, the identifier proposed from it and the client the
// consent is issued to, and may lower the account's rate target. Re-authorizing shows the account as a
// summary, asks for the mailbox only when the account remembers none, and offers another client of
// its provider (section 8.12). Confirming starts the attempt, so a refusal shows here, before anything
// happens at Google.
function NameStep(props: NameProps) {
  const { mode, answer, account } = props;
  const deps = useDeps();
  const [mailbox, setMailbox] = useState("");
  const [identifier, setIdentifier] = useState<string | undefined>(undefined);
  const [client, setClient] = useState<string | undefined>(props.chosen);
  const [percent, setPercent] = useState("");
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const [moving, setMoving] = useState(props.chosen !== undefined);
  const taken = answer.accounts.map((a) => a.account_id);
  const proposed = identifier ?? proposeIdentifier(mailbox, taken);
  const clients = clientsFor(answer);
  const remembered = account?.mailbox ?? null;
  const start = async () => {
    setFailure(undefined);
    const result =
      mode.kind === "connect"
        ? await send<AttemptAnswer>(deps.post, connectPath(), {
            account: proposed,
            client: client ?? (clients.length === 1 ? clients[0]?.name : undefined) ?? "",
            mailbox,
            lowered_target: percent === "" ? null : Number(percent) / 100,
          })
        : await send<AttemptAnswer>(deps.post, reauthorizePath(mode.account), {
            mailbox: remembered === null ? mailbox : null,
            client: client ?? null,
          });
    if (!result.ok) {
      setFailure(result.failure);
      return;
    }
    if (result.value.attempt !== null) {
      props.onStarted(result.value.attempt);
    }
  };
  if (mode.kind === "reauthorize" && account !== undefined) {
    const others = account.other_clients;
    const through = client ?? account.oauth_client ?? "";
    return (
      <section class="connect-step" aria-label="Step 1, the account">
        <h2>1. The account</h2>
        <dl class="values">
          <dt>Account</dt>
          <dd class="mono">{account.account_id}</dd>
          <dt>Provider</dt>
          <dd>{providerName(account.provider)}</dd>
          <dt>Client</dt>
          <dd class="mono">{through}</dd>
          <dt>Mailbox</dt>
          <dd>{remembered ?? "not remembered yet"}</dd>
        </dl>
        {remembered === null ? (
          <label>
            Mailbox address{" "}
            <input
              type="email"
              value={mailbox}
              onInput={(e) => setMailbox(e.currentTarget.value.trim())}
            />
            <span class="muted"> Asked once, and remembered from then on.</span>
          </label>
        ) : null}
        {others.length > 0 && !moving ? (
          <button type="button" onClick={() => setMoving(true)}>
            Connect through another client
          </button>
        ) : null}
        {moving ? (
          <ClientList
            clients={clients.filter((c) => others.includes(c.name))}
            chosen={client}
            choose={setClient}
          />
        ) : null}
        <button type="button" class="primary" onClick={() => void start()}>
          {props.attempt === null ? "Continue" : "Start again"}
        </button>
        <Refusal failure={failure} identifier={account.account_id} client={through} />
      </section>
    );
  }
  return (
    <section class="connect-step" aria-label="Step 1, name the account">
      <h2>1. Name the account</h2>
      <label>
        Mailbox address{" "}
        <input
          type="email"
          value={mailbox}
          onInput={(e) => setMailbox(e.currentTarget.value.trim())}
        />
      </label>
      <label>
        Identifier{" "}
        <input
          type="text"
          class="mono"
          value={proposed}
          onInput={(e) => setIdentifier(e.currentTarget.value)}
        />
      </label>
      <p class="muted">
        The identifier cannot change once connected, since every record of the account is keyed on
        it, and the mailbox is remembered.
      </p>
      <div>
        <span class="label">Connect through</span>{" "}
        {clients.length === 1 && clients[0] !== undefined ? (
          <>
            {providerName(clients[0].provider)}, through <span class="mono">{clients[0].name}</span>{" "}
            <a href={setupPath(clients[0].provider, "new", 1)}>Set up another client</a>
          </>
        ) : (
          <ClientList clients={clients} chosen={client} choose={setClient} />
        )}
      </div>
      <details>
        <summary>Lower this account&apos;s rate target</summary>
        <label>
          Rate target, percent of the declared ceiling{" "}
          <input
            type="number"
            min="6"
            max="50"
            value={percent}
            onInput={(e) => setPercent(e.currentTarget.value)}
          />
        </label>
      </details>
      <button
        type="button"
        class="primary"
        disabled={mailbox === "" || (clients.length > 1 && client === undefined)}
        onClick={() => void start()}
      >
        {props.attempt === null ? "Continue" : "Start again"}
      </button>
      <Refusal failure={failure} identifier={proposed} client={client ?? ""} />
    </section>
  );
}

// ClientList lists the clients by provider, each with its name, its project ID and the accounts on it,
// nothing preselected unless the address chose one, and closes with setting up another (section 8.12).
function ClientList(props: {
  clients: Client[];
  chosen: string | undefined;
  choose: (name: string) => void;
}) {
  return (
    <fieldset class="client-list">
      <legend>Connect through</legend>
      {props.clients.map((c) => (
        <label key={c.name}>
          <input
            type="radio"
            name="client"
            checked={props.chosen === c.name}
            onChange={() => props.choose(c.name)}
          />{" "}
          {providerName(c.provider)} <span class="mono">{c.name}</span>{" "}
          <span class="muted">
            {c.project_id ?? ""}{" "}
            {c.accounts.length === 0 ? "no account yet" : c.accounts.join(", ")}
          </span>
        </label>
      ))}
      <a href={setupPath("gmail", "new", 1)}>Set up another client</a>
    </fieldset>
  );
}

// GrantStep is step 2, what Google asks next, the consent page's link, the attempt's age and time left,
// and the errors Google shows on its own page (section 8.12).
function GrantStep(props: { attempt: ConsentAttempt; answer: Installation }) {
  const { attempt } = props;
  const { now, timers, config } = useDeps();
  const [, tick] = useState(0);
  useEffect(() => {
    const id = timers.set(() => tick((n) => n + 1), 1_000);
    return () => timers.clear(id);
  });
  const left = Date.parse(attempt.expires_at) - now();
  const client = props.answer.clients.find((c) => c.name === attempt.client);
  return (
    <section class="connect-step" aria-label="Step 2, grant access at Google">
      <h2>2. Grant access at Google</h2>
      <p>
        Google asks you to pick the account, past its unverified-app warning by Advanced and the
        link to continue, which is expected, and to allow Gmail access, leaving the Gmail permission
        ticked. If Google&apos;s warning says the app is being tested, stop: the client is still in
        Testing. Publish it (setup step 5) first, or this account is refused in 7 days. Google may
        email a security alert that mediated mailbox was granted access. That is this connection.
      </p>
      <div class="address-picture" aria-label="What the browser shows at the end">
        <span class="mono">
          {config.consentRedirect === "" ? "" : `${config.consentRedirect}?state=…&code=…`}
        </span>
        <p>
          Your browser cannot open this page. That is expected. If a page loads instead, something
          on your own computer answered; the address bar still works.
        </p>
      </div>
      <p>
        <a class="primary" href={attempt.consent_address} target="_blank" rel="noopener noreferrer">
          Open Google&apos;s consent page
        </a>
      </p>
      <p class="muted">
        {left > 0 ? `${duration(left)} left of this attempt's 15 minutes` : "This attempt expired."}
      </p>
      <details>
        <summary>Google showed an error instead?</summary>
        <table class="rows">
          <tbody>
            <tr>
              <td class="mono">redirect_uri_mismatch</td>
              <td>
                The client is a Web application. Create a Desktop app client at OAuth client setup
                step 6, then Replace the account&apos;s client.
              </td>
            </tr>
            <tr>
              <td class="mono">deleted_client, invalid_client</td>
              <td>
                The client was deleted, or its secret disabled. Restore it within 30 days from the
                console&apos;s deleted credentials, or set the client up again.
              </td>
            </tr>
            <tr>
              <td class="mono">access_denied</td>
              <td>
                Naming testing or verification, the app is in Testing and this mailbox is not a test
                user. Publish the app at OAuth client setup step 5.
              </td>
            </tr>
            <tr>
              <td class="mono">admin_policy_enforced</td>
              <td>
                An organization&apos;s administrator blocks the app. Ask the administrator to trust
                the client identifier <span class="mono">{client?.client_id ?? ""}</span>, or
                connect this mailbox through a client set up in the organization&apos;s own Cloud
                project, through Set up another client.
              </td>
            </tr>
            <tr>
              <td>anything else</td>
              <td>
                Open the consent page again, and if Google shows the same page, check each step of
                OAuth client setup.
              </td>
            </tr>
          </tbody>
        </table>
      </details>
    </section>
  );
}

type PasteProps = {
  mode: Mode;
  attempt: ConsentAttempt;
  answer: Installation;
  onConnected: (connected: Connected) => void;
};

// PasteStep is step 3. One field takes the whole address and names what it found before anything is
// sent, and the button finishes the attempt (section 8.12).
function PasteStep(props: PasteProps) {
  const { mode, attempt } = props;
  const deps = useDeps();
  const [address, setAddress] = useState("");
  const [failure, setFailure] = useState<Failure | undefined>(undefined);
  const [pip, setPip] = useState<Window | undefined>(undefined);
  const [blocked, setBlocked] = useState(false);
  const field = useRef<HTMLInputElement>(null);
  const redirect = deps.config.consentRedirect;
  const checks = useMemo(
    () => readPasted(address, attempt.attempt, redirect),
    [address, attempt.attempt, redirect],
  );
  // When the tab becomes visible again after the consent page was opened, focus moves to the field.
  useEffect(() => {
    const visible = () => {
      if (document.visibilityState === "visible") {
        field.current?.focus();
      }
    };
    document.addEventListener("visibilitychange", visible);
    return () => document.removeEventListener("visibilitychange", visible);
  }, []);
  // The paste box's side window sends its address, and is told the checks back.
  useEffect(
    () =>
      deps.guides.channel.listen((message) => {
        if (message.kind === "address") {
          setAddress(message.address);
        }
      }),
    [deps.guides],
  );
  useEffect(() => {
    deps.guides.channel.post({ kind: "checks", checks });
  }, [deps.guides, checks]);
  const finish = async () => {
    setFailure(undefined);
    const target =
      mode.kind === "connect"
        ? `${connectPath()}/finish`
        : `${reauthorizePath(mode.account)}/finish`;
    const result = await send<Connected>(deps.post, target, { address });
    if (result.ok) {
      props.onConnected(result.value);
    } else {
      setFailure(result.failure);
    }
  };
  const button =
    attempt.kind === "connect"
      ? `Connect ${attempt.account}`
      : attempt.kind === "move"
        ? `Move ${attempt.account} to ${attempt.client}`
        : `Re-authorize ${attempt.account}`;
  return (
    <section class="connect-step" aria-label="Step 3, paste the address">
      <h2>3. Paste the address</h2>
      <p>
        Copy the address from the tab that says it can&apos;t reach the site, and paste it here.
      </p>
      <label>
        Address{" "}
        <input
          ref={field}
          type="text"
          class="mono paste"
          value={address}
          onInput={(e) => setAddress(e.currentTarget.value)}
          onKeyDown={(e) => {
            if (e.key === "Enter") {
              void finish();
            }
          }}
        />
      </label>
      <button
        type="button"
        onClick={() => {
          void deps.guides.paste().then(setAddress, () => undefined);
        }}
      >
        Paste from clipboard
      </button>
      <Checks checks={checks} />
      <button
        type="button"
        class="primary"
        disabled={address.trim() === ""}
        onClick={() => void finish()}
      >
        {button}
      </button>
      {deps.guides.pip !== undefined ? (
        <button
          type="button"
          onClick={() => {
            void deps.guides
              .pip?.(pipSize.width, pipSize.height)
              .then(setPip, () => setBlocked(true));
          }}
        >
          Keep the paste box on top
        </button>
      ) : (
        <button
          type="button"
          onClick={() => {
            const opened = deps.guides.open(
              "/setup/connect?guide=paste",
              guideWindow,
              sideFeatures(screen.availWidth),
            );
            setBlocked(opened === null);
          }}
        >
          Keep the paste box on top
        </button>
      )}
      {blocked ? (
        <p class="warning" role="alert">
          Your browser blocked the window. Allow pop-ups for this site, or keep using this tab.
        </p>
      ) : null}
      <PipPasteBox
        target={pip}
        checks={checks}
        onAddress={setAddress}
        close={() => setPip(undefined)}
      />
      <div role="status" class="status-region">
        <Refusal
          failure={failure}
          identifier={attempt.account}
          client={attempt.client}
          mode={mode}
          project={props.answer.clients.find((c) => c.name === attempt.client)?.project_id ?? null}
        />
      </div>
    </section>
  );
}

// checkLine is one check, with its glyph, so it is not told by color alone.
function checkLine(ok: boolean, yes: string, no: string) {
  return (
    <li data-ok={ok ? "true" : "false"}>
      <span aria-hidden="true">{ok ? "✓ " : "✗ "}</span>
      {ok ? yes : no}
    </li>
  );
}

// Checks names what the pasted address holds, before anything is sent.
function Checks(props: { checks: PasteChecks | null }) {
  const { checks } = props;
  const host = redirectHost(useDeps().config.consentRedirect);
  if (checks === null) {
    return null;
  }
  return (
    <ul class="paste-checks" aria-label="What the address holds">
      {checkLine(checks.address, `The address is ${host}.`, `The address is not ${host}.`)}
      {checkLine(
        checks.state,
        "The state matches this attempt.",
        "The state is not this attempt's.",
      )}
      {checkLine(checks.code, "A code is present.", "No code is present.")}
    </ul>
  );
}

// PasteBox is the paste box's own window, the paste field and its three checks alone, and nothing about
// any account, the identifier and the mailbox included (section 8.12).
export function PasteBox(props: {
  checks: PasteChecks | null;
  onAddress: (address: string) => void;
}) {
  const [address, setAddress] = useState("");
  return (
    <section aria-label="Paste the address">
      <label>
        Paste the address{" "}
        <input
          type="text"
          class="mono paste"
          value={address}
          onInput={(e) => {
            setAddress(e.currentTarget.value);
            props.onAddress(e.currentTarget.value);
          }}
        />
      </label>
      <Checks checks={props.checks} />
    </section>
  );
}

// SidePasteBox is the paste box in the side window, a page of its own, which tells the connect page the
// address over the guide's channel and is told the checks back.
function SidePasteBox() {
  const { guides } = useDeps();
  const [checks, setChecks] = useState<PasteChecks | null>(null);
  useEffect(
    () =>
      guides.channel.listen((message) => {
        if (message.kind === "checks") {
          setChecks(message.checks);
        }
      }),
    [guides],
  );
  const onAddress = useCallback(
    (address: string) => guides.channel.post({ kind: "address", address }),
    [guides],
  );
  return <PasteBox checks={checks} onAddress={onAddress} />;
}

// PipPasteBox renders the paste box into the small window while it is open, with the bundle's
// stylesheet linked.
function PipPasteBox(props: {
  target: Window | undefined;
  checks: PasteChecks | null;
  onAddress: (address: string) => void;
  close: () => void;
}) {
  const deps = useDeps();
  const { target } = props;
  useEffect(() => {
    if (target === undefined) {
      return undefined;
    }
    linkStylesheet(target.document);
    target.addEventListener("pagehide", props.close);
    return () => {
      target.removeEventListener("pagehide", props.close);
      render(null, target.document.body);
    };
  }, [target, props.close]);
  useEffect(() => {
    if (target !== undefined) {
      render(
        h(
          DepsContext.Provider,
          { value: deps },
          h(
            "div",
            { class: "guide" },
            h(PasteBox, { checks: props.checks, onAddress: props.onAddress }),
          ),
        ),
        target.document.body,
      );
    }
  });
  return null;
}

// Refusal names a refused request's cause and its fix (section 8.12). Nothing was stored.
function Refusal(props: {
  failure: Failure | undefined;
  identifier: string;
  client: string;
  mode?: Mode;
  project?: string | null;
}) {
  const { failure } = props;
  if (failure === undefined) {
    return null;
  }
  return (
    <p class="refusal" role="alert">
      {refusalText(failure, props.identifier, props.client, props.mode, props.project ?? null)}
    </p>
  );
}

// refusalText is a refusal's wording, by its code.
export function refusalText(
  f: Failure,
  identifier: string,
  client: string,
  mode?: Mode,
  project: string | null = null,
): string {
  switch (f.code) {
    case "consent_declined":
      return "You declined at Google. Open the consent page again.";
    case "no_code":
      return "This address carries no code. Copy the address of the page Google sent you to.";
    case "wrong_address":
      return "This is not the address Google sent you to. Copy it from the tab that says it can't reach the site.";
    case "wrong_attempt":
      return "This address belongs to another attempt. Use the latest tab, or start again.";
    case "no_attempt":
      return "No connection is in progress in this browser session. The browser was closed or the UI restarted. Start again from step 1.";
    case "attempt_expired":
      return "This attempt expired. Open the consent page again.";
    case "code_refused":
      return "Google refused the code. It may already have been used. Open the consent page again.";
    case "scope_missing":
      return "Google granted no access to Gmail. Tick the Gmail permission on Google's page.";
    case "scope_refused":
      return "Google granted more than the Gmail permission this system asks for, so nothing was saved. Open the consent page again.";
    case "api_disabled":
      return `The Gmail API is not enabled in ${project === null ? "the client's project" : `project ${project}`}. Enable it (OAuth client setup step 2), wait a minute, then open the consent page again.`;
    case "wrong_mailbox": {
      const named =
        mode?.kind === "reauthorize" ? "the remembered mailbox" : "the mailbox you named";
      const more =
        mode?.kind === "reauthorize"
          ? " If this mailbox's address changed, the account cannot be re-authorized. Connect it again under a new identifier."
          : "";
      return `${f.message} Sign in to ${named} at Google.${more}`;
    }
    case "identifier_taken":
      return `An account named ${identifier} already exists, or was connected while this one was in progress. Nothing was saved. Start again from step 1 with another identifier.`;
    case "identifier_refused":
      return `${identifier} cannot be an account's identifier. It may not be ., .. or /, nor a word the UI's own paths use.`;
    case "client_changed":
      return `The client ${client} changed while this was in progress. Nothing was saved. Start again from step 1.`;
    case "unknown_client":
      return "No client of this name is set up.";
    case "mailbox_refused":
      return "The mailbox is an address with a local part and a domain.";
    case "target_refused":
      return "A lowered target is above 5% and at most 50% of the provider's declared ceiling.";
    case "provider_unreachable":
      return "Could not reach Google. Nothing was saved. Press Connect again. If Google then refuses the code, open the consent page again.";
    case "stale_page":
      return "This page is older than the UI server's last restart. Reload, then try again.";
    default:
      return f.message;
  }
}

// Success says what finishing stored and what comes next (section 8.12).
function Success(props: { mode: Mode; connected: Connected; answer: Installation }) {
  const { connected } = props;
  const home = `/${encodeURIComponent(connected.account)}`;
  if (connected.kind === "connect") {
    return (
      <>
        <h1 id="connect-title">
          Connected {connected.account} ({connected.mailbox}).
        </h1>
        <p>
          Every workload is told of {connected.account} and serves it, and backfill starts indexing
          it. Nothing more needs doing.
        </p>
        <p>
          The base policy&apos;s {count(props.answer.base_rules)}{" "}
          {props.answer.base_rules === 1 ? "rule applies" : "rules apply"} to {connected.account}{" "}
          from the start. Rules made for one account do not.{" "}
          <a href={`${home}/policy`}>Review {connected.account}&apos;s policy.</a>
        </p>
        <a class="primary" href={home}>
          Go to {connected.account}
        </a>
      </>
    );
  }
  return (
    <>
      <h1 id="connect-title">
        {connected.kind === "move"
          ? `Moved ${connected.account} to ${connected.client}.`
          : `Re-authorized ${connected.account}.`}
      </h1>
      <p>Every workload is told of the new credential and uses it from its next call.</p>
      <a class="primary" href={home}>
        Go to {connected.account}
      </a>
    </>
  );
}
