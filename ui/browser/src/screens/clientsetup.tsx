// The OAuth client setup of docs/UI.md section 8.11, for a new client and for the client named in the
// path. Everything is done in Google Cloud console, and the page carries the instructions for every
// step, one step expanded at a time, with a guide that sits beside or over Google's tab. Steps 1 to 6
// carry the operator's own marks, kept per browser and per client, and step 7 is Google's answer.
import { h, render, type ComponentChildren } from "preact";
import { useCallback, useEffect, useMemo, useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import {
  clientPath,
  clientsPath,
  installationPath,
  send,
  type ClientAnswer,
  type Failure,
  type Installation,
} from "../app/api.ts";
import { DepsContext, useDeps } from "../app/deps.ts";
import { InstallationFrame, providerName } from "../app/frame.tsx";
import { consoleTab, guideWindow, linkStylesheet, pipSize, sideFeatures } from "../app/guide.ts";
import { Region } from "../app/region.tsx";
import {
  firstOpen,
  forgetMarks,
  keepMarks,
  marksKey,
  projectProblem,
  projectShaped,
  readClientFile,
  readMarks,
  steps,
  type Marks,
  type Part,
} from "./steps.ts";

// newClient is the path segment of a client not yet saved, under which its marks are kept.
const newClient = "new";

// setupPath is a client's setup page with a step in view.
export function setupPath(provider: string, client: string, step: number, side = false): string {
  const q = new URLSearchParams({ step: String(step) });
  if (side) {
    q.set("guide", "side");
  }
  return `/setup/${encodeURIComponent(provider)}/${encodeURIComponent(client)}?${q.toString()}`;
}

// ClientSetupRoute is the page, or with guide=side the compact layout of its side window, which holds
// only the step, its values and its controls (section 8.11).
export function ClientSetupRoute(props: { provider: string; client: string }) {
  const { query } = useLocation();
  if (query["guide"] === "side") {
    return (
      <main class="guide">
        <ClientSetup provider={props.provider} client={props.client} compact />
      </main>
    );
  }
  return (
    <InstallationFrame>
      <ClientSetup provider={props.provider} client={props.client} compact={false} />
    </InstallationFrame>
  );
}

function ClientSetup(props: { provider: string; client: string; compact: boolean }) {
  const { installation } = useDeps();
  const path = installationPath();
  return (
    <Region
      name="installation"
      state={installation.read(path)}
      shape="block"
      retry={() => void installation.retry(path)}
    >
      {(answer) => <Setup answer={answer} {...props} />}
    </Region>
  );
}

// useSetupState is the page's marks and the step in view, which follow the address, the other window
// of the guide, and the marks the operator sets.
function useSetupState(provider: string, client: string) {
  const deps = useDeps();
  const { storage, guides } = deps;
  const { query, path, route } = useLocation();
  const key = marksKey(provider, client);
  const [marks, setMarks] = useState<Marks>(() => readMarks(storage, key));
  const asked = Number(query["step"]);
  const step = Number.isInteger(asked) && asked >= 1 && asked <= 7 ? asked : firstOpen(marks);
  const side = query["guide"] === "side";
  const show = useCallback(
    (n: number) => route(`${path}?${setupQuery(n, side)}`, true),
    [path, route, side],
  );
  // The other window of the guide tells this one which step it opened and that marks changed.
  useEffect(
    () =>
      guides.channel.listen((message) => {
        if (message.kind === "step" && message.key === key) {
          setMarks(readMarks(storage, key));
          show(message.step);
        }
      }),
    [guides, storage, key, show],
  );
  const open = useCallback(
    (n: number) => {
      show(n);
      guides.channel.post({ kind: "step", key, step: n });
    },
    [show, guides, key],
  );
  const update = useCallback(
    (next: Marks, then: number) => {
      keepMarks(storage, key, next);
      setMarks(next);
      open(then);
    },
    [storage, key, open],
  );
  return { marks, step, open, update, storeKey: key };
}

function setupQuery(step: number, side: boolean): string {
  const q = new URLSearchParams({ step: String(step) });
  if (side) {
    q.set("guide", "side");
  }
  return q.toString();
}

type SetupProps = { answer: Installation; provider: string; client: string; compact: boolean };

function Setup(props: SetupProps) {
  const { answer, provider, client, compact } = props;
  const state = useSetupState(provider, client);
  const existing = answer.clients.find((c) => c.name === client && c.provider === provider);
  const project = state.marks.project !== "" ? state.marks.project : (existing?.project_id ?? "");
  const [pipWindow, setPipWindow] = useState<Window | undefined>(undefined);
  const pip = useMemo(() => ({ window: pipWindow, set: setPipWindow }), [pipWindow]);
  if (!answer.providers.includes(provider)) {
    return <p>No provider {provider} authenticates through an OAuth client.</p>;
  }
  if (client !== newClient && existing === undefined) {
    return (
      <p>
        No {providerName(provider)} client is named <span class="mono">{client}</span>.{" "}
        <a href={setupPath(provider, newClient, 1)}>Set up a new client</a>
      </p>
    );
  }
  const stepProps = { ...props, ...state, project, existing, pip };
  if (compact) {
    return <ExpandedStep {...stepProps} n={state.step} compact />;
  }
  return (
    <section aria-labelledby="client-setup-title" class="client-setup">
      <h1 id="client-setup-title">
        {existing === undefined
          ? `Set up a ${providerName(provider)} client`
          : `${existing.name} · client ${existing.client_id}`}
      </h1>
      {existing === undefined ? null : (
        <p>
          {existing.accounts.length === 0
            ? "No account connects through this client."
            : `Connected through it: ${existing.accounts.join(", ")}`}
        </p>
      )}
      <GuideOffer {...stepProps} n={state.step} compact={false} />
      <PipGuide {...stepProps} n={state.step} compact />
      <div class="setup-columns">
        <ol class="rail" aria-label="Steps">
          {steps.map((s, i) => (
            <li key={s.title}>
              <button
                type="button"
                aria-current={state.step === i + 1 ? "step" : undefined}
                onClick={() => state.open(i + 1)}
              >
                {i + 1}. {s.title} <span class="muted">{markWord(state.marks, i)}</span>
              </button>
            </li>
          ))}
        </ol>
        <div class="steps">
          {steps.map((s, i) =>
            state.step === i + 1 ? (
              <ExpandedStep key={s.title} {...stepProps} n={i + 1} compact={false} />
            ) : (
              <button
                key={s.title}
                type="button"
                class="step-row"
                onClick={() => state.open(i + 1)}
              >
                {i + 1}. {s.title} <span class="muted">{markWord(state.marks, i)}</span>
              </button>
            ),
          )}
        </div>
      </div>
    </section>
  );
}

// markWord is a step's status as the rail shows it. Step 7's is Google's answer, shown on the step.
function markWord(marks: Marks, i: number): string {
  if (i === 6) {
    return "";
  }
  return marks.done[i] === true ? "done" : "not done";
}

type StepProps = SetupProps & {
  n: number;
  marks: Marks;
  storeKey: string;
  open: (n: number) => void;
  update: (next: Marks, then: number) => void;
  project: string;
  existing: Installation["clients"][number] | undefined;
  pip: Pip;
};

// ExpandedStep is one step in full, in the page, in the side window and in the small window alike.
function ExpandedStep(props: StepProps) {
  const { n, marks, project } = props;
  const deps = useDeps();
  const step = steps[n - 1];
  if (step === undefined) {
    return null;
  }
  const page =
    n >= 2 && n <= 6 && project !== "" ? step.page(project) : n === 1 ? step.page("") : "";
  const mark = (then: number, openConsole: boolean) => {
    const done = marks.done.map((d, i) => (i === n - 1 ? true : d));
    props.update({ ...marks, done }, then);
    const next = steps[then - 1];
    if (openConsole && next !== undefined && then <= 6 && project !== "") {
      deps.guides.console(next.page(project));
    }
  };
  return (
    <section class="step" aria-label={`Step ${n}, ${step.title}`}>
      <h2>
        {n}. {step.title}
      </h2>
      {step.why === undefined ? null : <p>{step.why}</p>}
      {page === "" ? null : (
        <p>
          <a class="primary" href={page} target={consoleTab}>
            Open in Google Cloud console
          </a>
        </p>
      )}
      {n >= 2 && n <= 6 && project === "" ? (
        <p class="muted">Enter the project ID at step 1 first, which every later link names.</p>
      ) : null}
      <ol class="actions-list">
        {step.actions.map((action, i) => (
          <li key={i}>
            {action.map((part, j) => (
              <PartView key={j} part={part} />
            ))}
          </li>
        ))}
      </ol>
      {n === 1 ? <ProjectField {...props} /> : null}
      {n === 7 ? <BringBack {...props} /> : null}
      {step.warning === undefined ? null : (
        <p class="warning" role="note">
          <span aria-hidden="true">⚠ </span>
          {step.warning}
        </p>
      )}
      <p>
        <span class="label">Done when you see</span> {step.done}
      </p>
      {n === 7 ? null : (
        <details>
          <summary>Looks different?</summary>
          <p>
            In the console, {step.menu}. What matters here is {step.matters}.
          </p>
        </details>
      )}
      {n === 7 ? null : (
        <p class="step-controls">
          <button type="button" class="primary" onClick={() => mark(n + 1, true)}>
            Done, open step {n + 1}
          </button>
          <button type="button" onClick={() => mark(n + 1, false)}>
            Mark done
          </button>
          {n > 1 ? (
            <button type="button" onClick={() => props.open(n - 1)}>
              Back
            </button>
          ) : null}
          {props.compact ? null : <GuideOffer {...props} />}
        </p>
      )}
    </section>
  );
}

function PartView(props: { part: Part }) {
  const { part } = props;
  if (typeof part === "string") {
    return <>{part}</>;
  }
  if ("bold" in part) {
    return <strong>{part.bold}</strong>;
  }
  return <Value text={part.enter} />;
}

// Value is a value to enter, in a mono chip with a copy control.
export function Value(props: { text: string }) {
  const { guides } = useDeps();
  const [copied, setCopied] = useState(false);
  return (
    <span class="value-chip">
      <code class="mono">{props.text}</code>
      <button
        type="button"
        aria-label={`Copy ${props.text}`}
        onClick={() => {
          void guides.copy(props.text).then(() => setCopied(true));
        }}
      >
        {copied ? "Copied" : "Copy"}
      </button>
    </span>
  );
}

// ProjectField is step 1's field for the project ID, with the offer to reuse the project of a client
// already set up, which marks steps 1 to 5 done since they belong to the project (section 8.11).
function ProjectField(props: StepProps) {
  const { marks, answer, existing } = props;
  const [value, setValue] = useState(props.project);
  const problem = projectProblem(value);
  const others = answer.clients.filter(
    (c) => c.provider === props.provider && c.project_id !== null && c.name !== existing?.name,
  );
  const keep = (project: string, done: boolean[], then: number) =>
    props.update({ done, project }, then);
  return (
    <div class="field">
      {marks.done.every((d) => !d) && existing === undefined ? (
        <p class="muted">
          Started before, in another browser? Paste the project ID, then check each step&apos;s
          &quot;Done when you see&quot; in the console.
        </p>
      ) : null}
      <label>
        Project ID{" "}
        <input
          type="text"
          class="mono"
          value={value}
          onInput={(e) => setValue(e.currentTarget.value.trim())}
          onBlur={() => {
            if (projectShaped(value)) {
              keep(value, marks.done, 1);
            }
          }}
        />
      </label>
      {problem === undefined ? null : (
        <p class="refusal" role="alert">
          {problem}
        </p>
      )}
      {others.map((c) => (
        <button
          key={c.name}
          type="button"
          onClick={() => {
            const project = c.project_id ?? "";
            setValue(project);
            keep(project, [true, true, true, true, true, marks.done[5] === true], 6);
          }}
        >
          Use the project of {c.name}
        </button>
      ))}
    </div>
  );
}

// Answer is step 7's answer from Google, as the status region shows it.
type Answer =
  | { kind: "accepted"; name: string; replaced: boolean; changedID: boolean }
  | { kind: "refused" }
  | { kind: "unreachable" }
  | { kind: "failed"; failure: Failure };

// BringBack is step 7. It takes the file Google offers at step 6, or a pasted identifier and secret,
// names a new client, and checks the client with Google before it is saved (section 8.11).
function BringBack(props: StepProps) {
  const { provider, existing, project, answer } = props;
  const deps = useDeps();
  const [clientID, setClientID] = useState("");
  const [secret, setSecret] = useState("");
  const [showSecret, setShowSecret] = useState(false);
  const [fileProject, setFileProject] = useState<string | undefined>(undefined);
  const [fileProblem, setFileProblem] = useState<string | undefined>(undefined);
  const [name, setName] = useState<string | undefined>(undefined);
  const [result, setResult] = useState<Answer | undefined>(undefined);
  const [confirming, setConfirming] = useState(false);
  const proposed = name ?? (project !== "" ? project : (fileProject ?? ""));
  const projectID = project !== "" ? project : (fileProject ?? null);
  const sameID = existing !== undefined && clientID === existing.client_id;
  const nameTaken =
    existing === undefined && answer.clients.some((c) => c.name === proposed)
      ? proposed
      : undefined;
  const readFile = async (file: File) => {
    const read = readClientFile(await file.text());
    if (!read.ok) {
      setFileProblem(read.problem);
      return;
    }
    setFileProblem(undefined);
    setClientID(read.clientID);
    setSecret(read.secret);
    setFileProject(read.project);
  };
  const save = async () => {
    setConfirming(false);
    const body = { client_id: clientID, client_secret: secret, project_id: projectID };
    const sent =
      existing === undefined
        ? await send<ClientAnswer>(deps.post, clientsPath(provider), { name: proposed, ...body })
        : await send<ClientAnswer>(deps.post, clientPath(provider, existing.name), body);
    if (sent.ok) {
      if (existing === undefined) {
        keepMarks(deps.storage, marksKey(provider, sent.value.client.name), {
          done: [true, true, true, true, true, true],
          project: sent.value.client.project_id ?? "",
        });
        forgetMarks(deps.storage, marksKey(provider, newClient));
      }
      setResult({
        kind: "accepted",
        name: sent.value.client.name,
        replaced: existing !== undefined,
        changedID: existing !== undefined && !sameID,
      });
      void deps.installation.refresh(installationPath());
      return;
    }
    const f = sent.failure;
    setResult(
      f.code === "client_refused"
        ? { kind: "refused" }
        : f.origin === "provider"
          ? { kind: "unreachable" }
          : { kind: "failed", failure: f },
    );
  };
  const onSave = () => {
    if (existing !== undefined && !sameID) {
      setConfirming(true);
      return;
    }
    void save();
  };
  const accounts = existing?.accounts ?? [];
  const others = answer.accounts.filter((a) => a.oauth_client !== existing?.name);
  const ready = clientID !== "" && secret !== "" && (existing !== undefined || proposed !== "");
  return (
    <div class="bring-back">
      {existing === undefined ? null : (
        <>
          <h3>Replace {existing.name}</h3>
          <p>Lost the secret? Add a new one to the same client from its page, by Add secret.</p>
        </>
      )}
      <DropZone onFile={(f) => void readFile(f)} />
      {fileProblem === undefined ? null : (
        <p class="refusal" role="alert">
          {fileProblem}
        </p>
      )}
      <label>
        Client ID{" "}
        <input
          type="text"
          class="mono"
          value={clientID}
          onInput={(e) => setClientID(e.currentTarget.value.trim())}
        />
      </label>
      <label>
        Client secret{" "}
        <input
          type={showSecret ? "text" : "password"}
          class="mono"
          autocomplete="off"
          value={secret}
          onInput={(e) => setSecret(e.currentTarget.value.trim())}
        />
      </label>
      <button type="button" onClick={() => setShowSecret(!showSecret)}>
        {showSecret ? "Hide" : "Show"}
      </button>
      {fileProject !== undefined && project !== "" && fileProject !== project ? (
        <p class="warning" role="note">
          This client belongs to project {fileProject}, but steps 2 to 5 were done in {project}.
          Check the Gmail API and the publishing status in {fileProject}.
        </p>
      ) : null}
      {existing === undefined ? (
        <label>
          Name{" "}
          <input
            type="text"
            class="mono"
            value={proposed}
            onInput={(e) => setName(e.currentTarget.value.trim())}
          />
          <span class="muted">
            {" "}
            How every screen and account refers to the client. It cannot change once saved.
          </span>
        </label>
      ) : null}
      {nameTaken === undefined ? null : (
        <p class="refusal" role="alert">
          Another client is named {nameTaken}.
        </p>
      )}
      <p>
        <button type="button" class="primary" disabled={!ready} onClick={onSave}>
          {existing === undefined
            ? "Check with Google and save"
            : sameID
              ? "Update the secret"
              : `Replace ${existing.name}`}
        </button>
      </p>
      {existing !== undefined && sameID ? (
        <p class="muted">
          Every account on {existing.name} keeps its grant. Disable the old secret in the console
          once this is saved.
        </p>
      ) : null}
      {confirming && existing !== undefined ? (
        <div class="dialog" role="dialog" aria-modal="true" aria-label={`Replace ${existing.name}`}>
          <p>
            {accounts.length === 0
              ? `No account connects through ${existing.name}.`
              : `The grants of the accounts on ${existing.name} were issued to its current client. After replacing it, each of these ${accounts.length} accounts is refused at its next token refresh and needs re-authorizing: ${accounts.join(", ")}.`}
          </p>
          {others.length > 0 ? <p>Accounts on other clients are untouched.</p> : null}
          <button type="button" class="primary" onClick={() => void save()}>
            Replace {existing.name}
          </button>
          <button type="button" onClick={() => setConfirming(false)}>
            Keep it
          </button>
        </div>
      ) : null}
      <div role="status" class="status-region">
        <AnswerView result={result} answer={answer} accounts={accounts} provider={provider} />
      </div>
    </div>
  );
}

function AnswerView(props: {
  result: Answer | undefined;
  answer: Installation;
  accounts: string[];
  provider: string;
}) {
  const { result } = props;
  if (result === undefined) {
    return null;
  }
  switch (result.kind) {
    case "accepted": {
      const connect = `/setup/connect?${new URLSearchParams({ client: result.name }).toString()}`;
      return (
        <>
          <p>Google accepted this client. Saved as {result.name}, its secret sealed.</p>
          {result.changedID && props.accounts.length > 0 ? (
            <ul>
              {props.accounts.map((a) => (
                <li key={a}>
                  <span class="mono">{a}</span>{" "}
                  <a href={`/${encodeURIComponent(a)}/account/reauthorize`}>Re-authorize {a}</a>
                </li>
              ))}
            </ul>
          ) : null}
          <a class="primary" href={connect}>
            {props.answer.accounts.length === 0
              ? "Connect your first account"
              : `Connect an account through ${result.name}`}
          </a>
        </>
      );
    }
    case "refused":
      return (
        <>
          <p>Google does not recognise this client identifier and secret. Nothing was saved.</p>
          <ul>
            <li>
              A client created in the last few minutes may not be active yet. Wait five minutes and
              press Check again.
            </li>
            <li>A space copied with the secret.</li>
            <li>A client of another type than Desktop app.</li>
          </ul>
        </>
      );
    case "unreachable":
      return <p>Could not reach Google. Nothing was saved. Try again.</p>;
    default:
      return <p>{refusalOf(result.failure, props.answer, props.provider)}</p>;
  }
}

// refusalOf words a refused save other than Google's answer (sections 8.11 and 17.3).
function refusalOf(f: Failure, answer: Installation, provider: string): ComponentChildren {
  switch (f.code) {
    case "name_taken":
      return "Another client holds this name. Nothing was saved.";
    case "name_refused":
      return "A name is 6 to 30 lowercase letters, digits and hyphens starting with a letter, and never new. Nothing was saved.";
    case "client_exists": {
      const holder = answer.clients.find(
        (c) => c.provider === provider && f.message.endsWith(c.name),
      );
      return holder === undefined ? (
        "This client is already set up under another name."
      ) : (
        <>
          This client is already set up as{" "}
          <a href={setupPath(provider, holder.name, 7)}>{holder.name}</a>
        </>
      );
    }
    case "stale_page":
      return "This page is older than the UI server's last restart. Reload, then try again.";
    default:
      return f.message;
  }
}

// DropZone takes the downloaded file dropped on it or chosen with its control.
function DropZone(props: { onFile: (file: File) => void }) {
  return (
    <div
      class="drop-zone"
      onDragOver={(e) => e.preventDefault()}
      onDrop={(e) => {
        e.preventDefault();
        const file = e.dataTransfer?.files[0];
        if (file !== undefined) {
          props.onFile(file);
        }
      }}
    >
      <label>
        Drop the downloaded file here, or choose it{" "}
        <input
          type="file"
          accept="application/json,.json"
          onChange={(e) => {
            const file = e.currentTarget.files?.[0];
            if (file !== undefined) {
              props.onFile(file);
            }
          }}
        />
      </label>
    </div>
  );
}

// Pip is the small window, while it is open.
type Pip = { window: Window | undefined; set: (w: Window | undefined) => void };

// GuideOffer puts the steps where the operator works, in a small window that stays on top where the
// browser supports one, and in a side window of the same page anywhere else (section 8.11).
function GuideOffer(props: StepProps) {
  const deps = useDeps();
  const { guides } = deps;
  const [blocked, setBlocked] = useState(false);
  if (guides.pip !== undefined) {
    return (
      <>
        <button
          type="button"
          onClick={() => {
            void guides.pip?.(pipSize.width, pipSize.height).then(
              (w) => props.pip.set(w),
              () => setBlocked(true),
            );
          }}
        >
          Keep the steps on top
        </button>
        {blocked ? <Blocked /> : null}
      </>
    );
  }
  return (
    <>
      <button
        type="button"
        onClick={() => {
          const opened = guides.open(
            setupPath(props.provider, props.client, props.n, true),
            guideWindow,
            sideFeatures(screen.availWidth),
          );
          setBlocked(opened === null);
        }}
      >
        Open the steps in a side window
      </button>
      {blocked ? <Blocked /> : null}
    </>
  );
}

function Blocked() {
  return (
    <p class="warning" role="alert">
      Your browser blocked the window. Allow pop-ups for this site, or keep using this tab.
    </p>
  );
}

// PipGuide renders the step in view into the small window while it is open, with the bundle's
// stylesheet linked, since the content security policy refuses inline styles. Closing the window
// leaves the page on the step the guide was at.
function PipGuide(props: StepProps) {
  const deps = useDeps();
  const target = props.pip.window;
  useEffect(() => {
    if (target === undefined) {
      return undefined;
    }
    linkStylesheet(target.document);
    const close = () => props.pip.set(undefined);
    target.addEventListener("pagehide", close);
    return () => target.removeEventListener("pagehide", close);
  }, [target, props.pip]);
  useEffect(() => {
    if (target === undefined) {
      return undefined;
    }
    render(
      h(
        DepsContext.Provider,
        { value: deps },
        h("div", { class: "guide" }, h(ExpandedStep, { ...props, compact: true })),
      ),
      target.document.body,
    );
    return undefined;
  });
  useEffect(
    () => () => {
      if (target !== undefined) {
        render(null, target.document.body);
      }
    },
    [target],
  );
  return null;
}
