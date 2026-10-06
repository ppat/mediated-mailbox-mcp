// The installation screens and account settings of docs/UI.md sections 8.10 to 8.13, rendered by the
// router over answers recorded from the real server.
import { afterEach, expect, test } from "bun:test";
import { act } from "preact/test-utils";
import {
  accountPath,
  accountsPath,
  connectPath,
  installationPath,
  reauthorizePath,
  systemPath,
  type System,
} from "../src/app/api.ts";
import { refusedCredential } from "../src/app/frame.tsx";
import { readPasted } from "../src/app/guide.ts";
import { App } from "../src/app/router.tsx";
import { proposeIdentifier } from "../src/screens/connect.tsx";
import { projectProblem, readClientFile } from "../src/screens/steps.ts";
import { homeAnswers, testDeps, TestGuides, type Connection } from "./app.ts";
import { ok, recorded, type Answer, type Recorded } from "./fixtures/fetch.ts";
import { at, mount, settle, type Mounted } from "./render.ts";

let mounted: Mounted | undefined;
let server: Recorded | undefined;
afterEach(() => {
  mounted?.unmount();
  mounted = undefined;
  expect(server?.missing ?? []).toEqual([]);
  server = undefined;
});

const refusedAnswer = (file: string, status = 400): Answer => ({ file, status });

// The marked account the connect page's recordings name, which no guide window may show.
const markedAccount = "mmfieldmarker-account";
const markedMailbox = "mmfieldmarker-mailbox@gmail.com";

async function open(
  path: string,
  answers: Readonly<Record<string, Answer | readonly Answer[]>>,
  guides: TestGuides = new TestGuides(),
): Promise<HTMLElement> {
  server = recorded(answers);
  at(path);
  const connections: Connection[] = [];
  mounted = mount(
    <App deps={testDeps(server, connections, undefined, undefined, guides.guides())} />,
  );
  await settle();
  return mounted.root;
}

// everything is an element's text and every attribute value of it and its descendants, the whole of
// what a document shows or carries.
function everything(root: Element | undefined): string {
  if (root === undefined) {
    return "";
  }
  const values = [root, ...root.querySelectorAll("*")].flatMap((e) =>
    [...e.attributes].map((a) => a.value),
  );
  return [root.textContent ?? "", ...values].join("\n");
}

function text(root: Element, selector: string): string[] {
  return [...root.querySelectorAll(selector)].map((e) => e.textContent?.trim() ?? "");
}

async function type(input: Element | null, value: string): Promise<void> {
  if (!(input instanceof HTMLInputElement)) {
    throw new Error("no such field");
  }
  await act(() => {
    input.value = value;
    input.dispatchEvent(new Event("input", { bubbles: true }));
  });
}

async function click(element: Element | null | undefined): Promise<void> {
  if (!(element instanceof HTMLElement)) {
    throw new Error("no such control");
  }
  await act(() => element.click());
  await settle();
}

function button(root: Element, label: string): Element | undefined {
  return [...root.querySelectorAll("button, a")].find((b) => b.textContent?.trim() === label);
}

test("with no account the entry route goes to the installation, whose first run lists its items", async () => {
  const root = await open("/", {
    [accountsPath()]: ok("accounts-empty.json"),
    [installationPath()]: ok("setup-first-run.json"),
  });
  expect(location.pathname).toBe("/setup");
  expect(root.querySelector("main h1")?.textContent).toBe("Installation");
  expect(root.querySelector(".topbar button")?.textContent).toBe("Installation");
  expect(text(root, ".checklist li")).toEqual([
    "○ not started Set up a Gmail OAuth clientStart",
    "○ No base rules yet · optional Review the base policy",
    "◌ cannot start yet Connect an account",
  ]);
  expect(text(root, ".provider-clients p")).toEqual(["No Gmail client yet"]);
  expect(root.querySelector('a[href="/setup/connect"]')).toBeNull();
  expect(text(root, "section[aria-labelledby='accounts-title'] p")).toEqual([
    "No account is connected yet.",
  ]);
});

test("the installation lists each client with its accounts, and a client in use cannot be removed", async () => {
  const root = await open("/setup", {
    [accountsPath()]: ok("accounts.json"),
    [installationPath()]: ok("setup.json"),
  });
  expect(root.querySelector(".getting-started")).toBeNull();
  const rows = [...root.querySelectorAll('table[aria-label="Gmail clients"] tbody tr')];
  expect(rows.map((r) => r.querySelector("td")?.textContent)).toEqual([
    "household-mail",
    "spare-client",
  ]);
  const [used, spare] = rows.map((r) => button(r, "Remove"));
  expect(used?.hasAttribute("disabled")).toBe(true);
  expect(rows[0]?.textContent).toContain(
    "2 accounts connect through this client. Move them to another client from their settings first.",
  );
  expect(spare?.hasAttribute("disabled")).toBe(false);
  expect(rows[1]?.textContent).toContain("no account yet");
  expect(rows[0]?.querySelector('a[target="mediated-mailbox-console"]')?.getAttribute("href")).toBe(
    "https://console.cloud.google.com/auth/clients?project=household-mail",
  );
  const accounts = [...root.querySelectorAll('table[aria-label="Accounts"] tbody tr')];
  expect(
    accounts.map((r) => [...r.querySelectorAll("a")].map((a) => a.getAttribute("href"))),
  ).toEqual([
    ["/other", "/other/account"],
    ["/personal", "/personal/account"],
  ]);
  expect(button(root, "Connect an account")?.getAttribute("href")).toBe("/setup/connect");
});

test("client setup opens on the first step not done, and Done marks it and opens the next step's page", async () => {
  const guides = new TestGuides();
  const root = await open(
    "/setup/gmail/new",
    {
      [accountsPath()]: ok("accounts-empty.json"),
      [installationPath()]: ok("setup-first-run.json"),
    },
    guides,
  );
  expect(root.querySelector(".step h2")?.textContent).toBe("1. Create the project");
  const field = root.querySelector(".step .field input");
  await type(field, "123456789012");
  expect(root.querySelector(".step .field .refusal")?.textContent).toBe(
    "That is the project number",
  );
  await type(field, "household-mail");
  await act(() => {
    field?.dispatchEvent(new Event("blur"));
  });
  await click(button(root, "Done, open step 2"));
  expect(new URLSearchParams(location.search).get("step")).toBe("2");
  expect(root.querySelector(".step h2")?.textContent).toBe("2. Enable the Gmail API");
  expect(guides.consoles).toEqual([
    "https://console.cloud.google.com/apis/library/gmail.googleapis.com?project=household-mail",
  ]);
  expect(text(root, ".rail button span")).toEqual([
    "done",
    "not done",
    "not done",
    "not done",
    "not done",
    "not done",
    "",
  ]);
  expect(JSON.parse(localStorage.getItem("mediated-mailbox.setup.gmail.new") ?? "{}")).toEqual({
    done: [true, false, false, false, false, false],
    project: "household-mail",
  });
});

test("step 1 tells a project number and a project name from a project ID", () => {
  expect(projectProblem("household-mail")).toBeUndefined();
  expect(projectProblem("123456789012")).toBe("That is the project number");
  expect(projectProblem("Household Mail")).toBe("That is the project name");
  expect(projectProblem("1abcdef")).toBe(
    "A project ID is 6 to 30 lowercase letters, digits and hyphens, starting with a letter",
  );
});

test("step 7 reads a desktop client's file, and refuses a web application's and any other", () => {
  expect(
    readClientFile(
      '{"installed": {"client_id": "id", "client_secret": "secret", "project_id": "household-mail"}}',
    ),
  ).toEqual({ ok: true, clientID: "id", secret: "secret", project: "household-mail" });
  expect(readClientFile('{"web": {"client_id": "id", "client_secret": "secret"}}')).toEqual({
    ok: false,
    problem:
      "This file holds a Web application client. Create a Desktop app client at step 6 instead.",
  });
  expect(readClientFile('{"type": "service_account"}')).toEqual({
    ok: false,
    problem: "This file holds type, not a Desktop app client.",
  });
  expect(readClientFile("not json").ok).toBe(false);
});

test("step 7 checks the client with Google before saving it, and says what each answer means", async () => {
  const root = await open("/setup/gmail/new?step=7", {
    [accountsPath()]: ok("accounts-empty.json"),
    [installationPath()]: ok("setup-first-run.json"),
    "POST /api/setup/gmail/clients": [
      refusedAnswer("error-client-refused.json"),
      ok("client-added.json"),
    ],
  });
  const fields = [...root.querySelectorAll(".bring-back input:not([type=file])")];
  await type(fields[0] ?? null, "household-id.apps.example");
  await type(fields[1] ?? null, "household-secret");
  await type(fields[2] ?? null, "household-mail");
  expect(fields[1]?.getAttribute("type")).toBe("password");
  await click(button(root, "Check with Google and save"));
  expect(server?.posted).toEqual([
    {
      path: "/api/setup/gmail/clients",
      body: {
        name: "household-mail",
        client_id: "household-id.apps.example",
        client_secret: "household-secret",
        project_id: null,
      },
    },
  ]);
  expect(root.querySelector(".status-region")?.textContent).toContain(
    "Google does not recognise this client identifier and secret. Nothing was saved.",
  );
  await click(button(root, "Check with Google and save"));
  const status = root.querySelector(".status-region");
  expect(status?.textContent).toContain(
    "Google accepted this client. Saved as household-mail, its secret sealed.",
  );
  expect(button(status ?? root, "Connect your first account")?.getAttribute("href")).toBe(
    "/setup/connect?client=household-mail",
  );
});

// redirect is the consent redirect the test app's entry document carries.
const redirect = "http://127.0.0.1:47823/";

test("a pasted address names what it holds before anything is sent", () => {
  expect(readPasted("http://127.0.0.1:47823/?state=s&code=c", "s", redirect)).toEqual({
    address: true,
    state: true,
    code: true,
  });
  expect(readPasted("127.0.0.1:47823/?state=other", "s", redirect)).toEqual({
    address: true,
    state: false,
    code: false,
  });
  expect(readPasted("https://example.com/?state=s&code=c", "s", redirect)).toEqual({
    address: false,
    state: true,
    code: true,
  });
  expect(readPasted("http://127.0.0.1:47824/?state=s&code=c", "s", redirect)?.address).toBe(false);
  expect(readPasted("http://[::1]:5000/?state=s&code=c", "s", "http://[::1]:5000/")?.address).toBe(
    true,
  );
  expect(readPasted("http://127.0.0.1:47823/?state=s&code=c", "s", "")?.address).toBe(false);
  expect(readPasted("  ", "s", redirect)).toBeNull();
});

test("an identifier is proposed from the mailbox's local part, with a suffix when it is taken", () => {
  expect(proposeIdentifier("Jo.Smith+mail@gmail.com", [])).toBe("jo-smith-mail");
  expect(proposeIdentifier("jo@gmail.com", ["jo", "jo-2"])).toBe("jo-3");
  expect(proposeIdentifier("@gmail.com", [])).toBe("account");
});

const connectAnswers = {
  [accountsPath()]: ok("accounts-empty.json"),
  [installationPath()]: ok("setup-client.json"),
};

test("step 1 starts the attempt, and a refused identifier says why before anything happens at Google", async () => {
  const root = await open("/setup/connect", {
    ...connectAnswers,
    [connectPath()]: [ok("connect-none.json"), ok("connect-attempt.json")],
    "POST /api/setup/connect": [
      refusedAnswer("error-identifier-refused.json"),
      ok("connect-started.json"),
    ],
  });
  expect(root.textContent).toContain("Gmail, through household-mail");
  await type(root.querySelector('input[type="email"]'), "jo.smith@gmail.com");
  const identifier = root.querySelector(".connect-step input.mono");
  expect(identifier instanceof HTMLInputElement ? identifier.value : undefined).toBe("jo-smith");
  await type(identifier, "setup");
  await click(button(root, "Continue"));
  expect(root.querySelector(".refusal")?.textContent).toBe(
    "setup cannot be an account's identifier. It may not be ., .. or /, nor a word the UI's own paths use.",
  );
  expect(root.querySelector('section[aria-label="Step 2, grant access at Google"]')).toBeNull();
  await click(button(root, "Continue"));
  expect(server?.posted.map((p) => p.body)).toEqual([
    {
      account: "setup",
      client: "household-mail",
      mailbox: "jo.smith@gmail.com",
      lowered_target: null,
    },
    {
      account: "setup",
      client: "household-mail",
      mailbox: "jo.smith@gmail.com",
      lowered_target: null,
    },
  ]);
  const consent = button(root, "Open Google's consent page");
  expect(consent?.getAttribute("target")).toBe("_blank");
  expect(consent?.getAttribute("href")).toStartWith(
    "https://accounts.google.com/o/oauth2/v2/auth?",
  );
});

test("a restored attempt finishes from the pasted address, names a refusal's cause, and says what it stored", async () => {
  const root = await open("/setup/connect", {
    ...connectAnswers,
    [connectPath()]: ok("connect-attempt.json"),
    "POST /api/setup/connect/finish": [
      refusedAnswer("error-wrong-mailbox.json"),
      ok("connected.json"),
    ],
  });
  const paste = root.querySelector("input.paste");
  await type(paste, "http://127.0.0.1:47823/?state=recorded-attempt&code=4/a-code");
  expect(text(root, ".paste-checks li")).toEqual([
    "✓ The address is 127.0.0.1:47823.",
    "✓ The state matches this attempt.",
    "✓ A code is present.",
  ]);
  await click(button(root, `Connect ${markedAccount}`));
  expect(server?.posted).toEqual([
    {
      path: "/api/setup/connect/finish",
      body: { address: "http://127.0.0.1:47823/?state=recorded-attempt&code=4/a-code" },
    },
  ]);
  expect(root.querySelector(".status-region .refusal")?.textContent).toBe(
    `Google granted access to someone.else@gmail.com, not ${markedMailbox}. Sign in to the mailbox you named at Google.`,
  );
  await click(button(root, `Connect ${markedAccount}`));
  expect(root.querySelector("main h1")?.textContent).toBe(
    `Connected ${markedAccount} (${markedMailbox}).`,
  );
  expect(button(root, `Go to ${markedAccount}`)?.getAttribute("href")).toBe(`/${markedAccount}`);
  // The base policy's count belongs to no account, and its rules apply from the start.
  expect(text(root, "main p").find((p) => p.startsWith("The base policy's"))).toBe(
    `The base policy's 0 rules apply to ${markedAccount} from the start. Rules made for one account do not. Review ${markedAccount}'s policy.`,
  );
  expect(button(root, `Review ${markedAccount}'s policy.`)?.getAttribute("href")).toBe(
    `/${markedAccount}/policy`,
  );
});

test("the paste box's own window and its side window hold nothing about any account", async () => {
  const guides = new TestGuides(undefined, true);
  const root = await open(
    "/setup/connect",
    { ...connectAnswers, [connectPath()]: ok("connect-attempt.json") },
    guides,
  );
  expect(root.textContent).toContain(markedAccount);
  await click(button(root, "Keep the paste box on top"));
  const small = guides.windows[0];
  expect(small?.body.querySelector("input.paste")).not.toBeNull();
  for (const marked of [markedAccount, markedMailbox, "mmfieldmarker"]) {
    expect(everything(small?.documentElement)).not.toContain(marked);
  }
  mounted?.unmount();
  mounted = undefined;
  const side = await open("/setup/connect?guide=paste", {});
  expect(side.querySelector("input.paste")).not.toBeNull();
  expect(everything(side)).not.toContain("mmfieldmarker");
  expect(server?.calls).toEqual([]);
});

test("the setup guide's small window shows the step in view and nothing else", async () => {
  const guides = new TestGuides(undefined, true);
  const root = await open(
    "/setup/gmail/new?step=2",
    {
      [accountsPath()]: ok("accounts-empty.json"),
      [installationPath()]: ok("setup-first-run.json"),
    },
    guides,
  );
  await click(button(root, "Keep the steps on top"));
  const small = guides.windows[0];
  expect(small?.body.querySelector("h2")?.textContent).toBe("2. Enable the Gmail API");
  expect(small?.head.querySelector('link[rel="stylesheet"]')?.getAttribute("href")).toBe(
    "/main.css",
  );
  expect(small?.body.querySelector(".rail")).toBeNull();
});

const settingsAnswers = {
  [accountsPath()]: ok("accounts.json"),
  [systemPath("personal")]: ok("system.json"),
  [systemPath("other")]: ok("system-other.json"),
};

test("account settings show the client, the credential's health, the target and the rows, never the credential", async () => {
  const root = await open("/personal/account", {
    ...settingsAnswers,
    [accountPath("personal")]: ok("account-personal.json"),
    "POST /api/personal/account/target": ok("target-set.json"),
  });
  expect(root.querySelector("main h1")?.textContent).toBe("personal · Gmail");
  expect(root.textContent).toContain("personal@gmail.com");
  expect(button(root, "household-mail")?.getAttribute("href")).toBe(
    "/setup/gmail/household-mail?step=7",
  );
  expect(button(root, "Move to another client")?.getAttribute("href")).toBe(
    "/personal/account/reauthorize",
  );
  // The recorded outcome carries markup, which the wording does not know, so it is shown as itself, inert.
  const health = root.querySelector('section[aria-labelledby="credential-title"] p');
  expect(health?.querySelector(".badge")?.textContent).toBe("unknown");
  expect(health?.textContent).toContain("<script>mmfieldmarker-authoutcome</script>");
  expect(health?.querySelectorAll("script").length).toBe(0);
  expect(root.textContent).toContain(
    "Lowered to 30% of Gmail's declared ceiling · now 5.0 units/s",
  );
  await type(root.querySelector('section[aria-labelledby="target-title"] input'), "25");
  await click(button(root, "Save"));
  expect(server?.posted).toEqual([
    { path: "/api/personal/account/target", body: { lowered_target: 0.25 } },
  ]);
  expect(text(root, 'dl[aria-label="The account\'s rows"] dt')).toEqual([
    "Identifier",
    "Provider",
    "Client",
    "Mailbox",
    "Connected",
    "Last authentication",
    "Rate target",
    "Backfill pass 1",
    "Backfill pass 2",
    "Sync cursor written",
  ]);
});

test("an account with no state row reads not connected, and offers Connect and no target field", async () => {
  const root = await open("/other/account", {
    ...settingsAnswers,
    [accountPath("other")]: ok("account-other.json"),
    ...homeAnswers("other"),
  });
  const health = root.querySelector('section[aria-labelledby="credential-title"]');
  expect(health?.querySelector(".badge")?.textContent).toBe("not connected");
  expect(button(health ?? root, "Connect")?.getAttribute("href")).toBe(
    "/other/account/reauthorize",
  );
  expect(root.querySelector('section[aria-labelledby="target-title"] input')).toBeNull();
});

test("the refused-credential banner reads the latest authentication, and a failed one raises none", async () => {
  const system: System = JSON.parse(
    await Bun.file(new URL("fixtures/system.json", import.meta.url)).text(),
  );
  const refused = {
    ...system,
    operational: { ...system.operational, last_auth_outcome: "refused" },
  };
  expect(refusedCredential(refused, "gmail")).toBe(
    "Gmail refused personal's credential at 2026-09-10 04:16Z. Workloads that call Gmail fail for this account until it is re-authorized.",
  );
  expect(
    refusedCredential(
      { ...system, operational: { ...system.operational, last_auth_outcome: "failed" } },
      "gmail",
    ),
  ).toBeUndefined();
  expect(refusedCredential(system, "gmail")).toBeUndefined();
});

test("re-authorizing shows the account as a summary and asks for no mailbox it remembers", async () => {
  const root = await open("/personal/account/reauthorize", {
    ...settingsAnswers,
    [installationPath()]: ok("setup.json"),
    [accountPath("personal")]: ok("account-personal.json"),
    [reauthorizePath("personal")]: ok("reauthorize-none.json"),
  });
  expect(root.querySelector("main h1")?.textContent).toBe("Re-authorize personal");
  expect(root.querySelector('input[type="email"]')).toBeNull();
  expect(text(root, ".connect-step dd")).toEqual([
    "personal",
    "Gmail",
    "household-mail",
    "personal@gmail.com",
  ]);
  await click(button(root, "Connect through another client"));
  expect(text(root, ".client-list label").map((t) => t.replace(/\s+/g, " "))).toEqual([
    "Gmail spare-client spare-client no account yet",
  ]);
});

test("the connect page pictures and checks the configured redirect address", async () => {
  server = recorded({ ...connectAnswers, [connectPath()]: ok("connect-attempt.json") });
  at("/setup/connect");
  const deps = testDeps(server);
  deps.config = { ...deps.config, consentRedirect: "http://[::1]:5000/" };
  mounted = mount(<App deps={deps} />);
  await settle();
  const root = mounted.root;
  expect(root.querySelector(".address-picture .mono")?.textContent).toBe(
    "http://[::1]:5000/?state=…&code=…",
  );
  const paste = root.querySelector("input.paste");
  await type(paste, "http://127.0.0.1:47823/?state=recorded-attempt&code=c");
  expect(text(root, ".paste-checks li")[0]).toBe("✗ The address is not [::1]:5000.");
  await type(paste, "http://[::1]:5000/?state=recorded-attempt&code=c");
  expect(text(root, ".paste-checks li")[0]).toBe("✓ The address is [::1]:5000.");
});
