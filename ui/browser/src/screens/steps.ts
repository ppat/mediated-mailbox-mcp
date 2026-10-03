// The OAuth client setup's steps as data, and the pure rules around them (docs/UI.md section 8.11).
// The labels and pages are taken from Google's documentation, and the wording is checked step by step
// against the live console before a real client is set up (ROADMAP.md, M7). The console's deep links
// are not documented as stable, so each step also carries the menu path to the same page.

// Part is one piece of an action's text, plain, a label Google prints in bold, or a value to enter in
// a mono chip with a copy control.
export type Part = string | { bold: string } | { enter: string };

export type Step = {
  title: string;
  // why is the one sentence of why, where the step needs one.
  why?: string;
  // page is the console page's address, for the project given.
  page: (project: string) => string;
  menu: string;
  // matters is the one value that matters on the step, for "Looks different?".
  matters: string;
  actions: Part[][];
  done: string;
  warning?: string;
};

// Each console page is written out whole, so the bundle's scan for another origin finds each address
// it carries as one string and nothing pieced together (ui/internal/api/scan.go). They are navigation
// targets the operator opens, never fetched.
const clientsPage = "https://console.cloud.google.com/auth/clients";

// inProject is a console page for the project, which the page's own project picker may still override.
const inProject = (page: string) => (project: string) =>
  `${page}?${new URLSearchParams({ project }).toString()}`;

// clientsConsole is the console's clients page for a project, which the installation screen links a
// client to.
export function clientsConsole(project: string): string {
  return inProject(clientsPage)(project);
}

const picker: Part[] = ["Check the project picker at the top shows this project."];

export const steps: readonly Step[] = [
  {
    title: "Create the project",
    page: () => "https://console.cloud.google.com/projectcreate",
    menu: "Menu › IAM & Admin › Create a project",
    matters: "the Project ID shown under the name",
    actions: [
      ["Use any Google account you own. It does not need to be the mailbox you will connect."],
      ["Enter ", { bold: "Project name" }, " ", { enter: "mediated-mailbox" }, "."],
      [
        "Leave ",
        { bold: "Location" },
        " as ",
        { bold: "No organization" },
        " for a personal account.",
      ],
      ["Press ", { bold: "Create" }, "."],
      [
        "Copy the ",
        { bold: "Project ID" },
        " shown under the name, and paste it into the field below.",
      ],
      [
        "If the console stays in another project, choose the new one in the project picker at the top.",
      ],
    ],
    done: "the new project's name in the project picker, and its ID pasted here",
  },
  {
    title: "Enable the Gmail API",
    page: inProject("https://console.cloud.google.com/apis/library/gmail.googleapis.com"),
    menu: 'Menu › APIs & Services › Library, search "Gmail API"',
    matters: "the Gmail API",
    actions: [picker, ["Press ", { bold: "Enable" }, "."]],
    done: "the Gmail API's page no longer offering Enable",
    warning: "Google lists the Gmail scope in step 4 only once the API is enabled.",
  },
  {
    title: "Set up Google Auth Platform",
    page: inProject("https://console.cloud.google.com/auth/overview"),
    menu: "Menu › Google Auth Platform › Overview",
    matters: "the audience, External",
    actions: [
      picker,
      ["Press ", { bold: "Get started" }, "."],
      [
        "Under ",
        { bold: "App Information" },
        " enter ",
        { bold: "App name" },
        " ",
        { enter: "mediated mailbox" },
        " and choose your address as ",
        { bold: "User support email" },
        ", then ",
        { bold: "Next" },
        ".",
      ],
      [
        "Under ",
        { bold: "Audience" },
        " choose ",
        { bold: "External" },
        ", then ",
        { bold: "Next" },
        ".",
      ],
      [
        "Under ",
        { bold: "Contact Information" },
        " enter your address, then ",
        { bold: "Next" },
        ".",
      ],
      [
        "Under ",
        { bold: "Finish" },
        " tick the agreement to Google's user data policy, then ",
        { bold: "Continue" },
        " and ",
        { bold: "Create" },
        ".",
      ],
    ],
    done: "Branding, Audience, Data Access and Clients reachable in Google Auth Platform's menu",
    warning:
      "Internal is offered only to an organization, and a personal account chooses External. Internal works only if every mailbox that connects through this client is in this organization. External always works.",
  },
  {
    title: "Add the Gmail scope",
    page: inProject("https://console.cloud.google.com/auth/scopes"),
    menu: "Menu › Google Auth Platform › Data Access",
    matters: "the scope https://www.googleapis.com/auth/gmail.modify",
    actions: [
      picker,
      ["Press ", { bold: "Add or remove scopes" }, "."],
      [
        "Filter by ",
        { enter: "gmail.modify" },
        " and tick ",
        { enter: "https://www.googleapis.com/auth/gmail.modify" },
        ", or, only once step 2 is done, paste it into the box for adding scopes by hand.",
      ],
      ["Press ", { bold: "Update" }, ", and save the page if it offers to."],
      ["Add no other scope."],
    ],
    done: "gmail.modify listed among the project's scopes",
    warning:
      "The scope lets the system label and move mail and cannot permanently delete anything. Google marks it restricted, which is expected.",
  },
  {
    title: "Publish to production",
    page: inProject("https://console.cloud.google.com/auth/audience"),
    menu: "Menu › Google Auth Platform › Audience",
    matters: "the publishing status, In production",
    actions: [
      picker,
      [
        "Under ",
        { bold: "Publishing status" },
        ", press ",
        { bold: "Publish app" },
        ", and confirm.",
      ],
      [
        "Google then marks the app as needing verification, which this client does not need, so leave it.",
      ],
    ],
    done: "Publishing status reading In production",
    warning:
      "A client left in Testing has every grant expire after 7 days, so each account is refused a week after it connects, and nothing here can see the setting. Google shows each consent an unverified-app warning, which is expected for a client only its owner uses, and an unverified app may have at most 100 users, far more than one installation connects. Google may email you asking to verify the app. Nothing needs doing.",
  },
  {
    title: "Create the desktop client",
    page: inProject(clientsPage),
    menu: "Menu › Google Auth Platform › Clients",
    matters: "the application type, Desktop app",
    actions: [
      picker,
      ["Press ", { bold: "Create client" }, "."],
      ["Choose ", { bold: "Application type" }, " ", { bold: "Desktop app" }, "."],
      ["Enter ", { bold: "Name" }, " ", { enter: "mediated mailbox" }, "."],
      ["Press ", { bold: "Create" }, "."],
      [
        "In the dialog, press ",
        { bold: "Download JSON" },
        ", or copy the ",
        { bold: "Client ID" },
        " and ",
        { bold: "Client secret" },
        " before closing it.",
      ],
    ],
    done: 'the dialog "OAuth client created"',
    warning:
      "Choose Desktop app, never Web application. Google shows the secret only in that dialog, and the downloaded file keeps it. A secret lost before it is brought back is replaced from the client's page by Add secret. Google deletes a client unused for 6 months, after emailing the project's contacts 30 days before.",
  },
  {
    title: "Bring the client back",
    page: () => "",
    menu: "",
    matters: "",
    actions: [
      ["Drop or choose the downloaded file, or paste the client ID and the client secret."],
    ],
    done: "Google's answer below",
  },
];

// projectShaped reports whether a value has the shape of a project ID, 6 to 30 lowercase letters,
// digits and hyphens starting with a letter (section 8.11).
export function projectShaped(value: string): boolean {
  return /^[a-z][a-z0-9-]{5,29}$/.test(value);
}

// projectProblem is what step 1's field says of a value that is not a project ID, a project number
// or a project name, and undefined for one that is.
export function projectProblem(value: string): string | undefined {
  if (value === "" || projectShaped(value)) {
    return undefined;
  }
  if (/^[0-9]+$/.test(value)) {
    return "That is the project number";
  }
  if (/[ A-Z]/.test(value)) {
    return "That is the project name";
  }
  return "A project ID is 6 to 30 lowercase letters, digits and hyphens, starting with a letter";
}

// Marks are a client's setup marks, kept per browser and per client, a new client's under new until
// it is saved (section 8.11). Steps 1 to 6 carry a mark the operator sets, and the project ID is kept
// until the client is saved.
export type Marks = { done: boolean[]; project: string };

export function marksKey(provider: string, client: string): string {
  return `mediated-mailbox.setup.${provider}.${client}`;
}

export function readMarks(storage: Storage | undefined, key: string): Marks {
  const none: Marks = { done: [false, false, false, false, false, false], project: "" };
  try {
    const raw = storage?.getItem(key);
    if (raw === null || raw === undefined) {
      return none;
    }
    const parsed: unknown = JSON.parse(raw);
    if (typeof parsed !== "object" || parsed === null) {
      return none;
    }
    const done: unknown = Reflect.get(parsed, "done");
    const project: unknown = Reflect.get(parsed, "project");
    return {
      done: none.done.map((_, i) => Array.isArray(done) && done[i] === true),
      project: typeof project === "string" ? project : "",
    };
  } catch {
    return none;
  }
}

export function keepMarks(storage: Storage | undefined, key: string, marks: Marks): void {
  try {
    storage?.setItem(key, JSON.stringify(marks));
  } catch {
    // A browser refusing storage keeps no marks, and every step reads not done.
  }
}

// forgetMarks removes a key's marks, a new client's once it is saved under its name.
export function forgetMarks(storage: Storage | undefined, key: string): void {
  try {
    storage?.removeItem(key);
  } catch {
    // Nothing was kept.
  }
}

// firstOpen is the step the page opens on, the first not marked done, step 7 once 1 to 6 are.
export function firstOpen(marks: Marks): number {
  const i = marks.done.findIndex((d) => !d);
  return i < 0 ? 7 : i + 1;
}

// ClientFile is what a downloaded client file holds for the setup.
export type ClientFile =
  | { ok: true; clientID: string; secret: string; project: string | undefined }
  | { ok: false; problem: string };

// readClientFile reads the file Google offers at step 6, the installed entry of a desktop client's
// file. A file of any other kind is refused with a line naming what it holds, so a web application's
// client is caught before it is sent (section 8.11).
export function readClientFile(text: string): ClientFile {
  let parsed: unknown;
  try {
    parsed = JSON.parse(text);
  } catch {
    return {
      ok: false,
      problem: "This file is not a client file Google downloads. It is not JSON.",
    };
  }
  if (typeof parsed !== "object" || parsed === null) {
    return { ok: false, problem: "This file is not a client file Google downloads." };
  }
  const installed: unknown = Reflect.get(parsed, "installed");
  if (typeof installed !== "object" || installed === null) {
    if (Reflect.has(parsed, "web")) {
      return {
        ok: false,
        problem:
          "This file holds a Web application client. Create a Desktop app client at step 6 instead.",
      };
    }
    const kinds = Object.keys(parsed).join(", ");
    return {
      ok: false,
      problem: `This file holds ${kinds === "" ? "nothing" : kinds}, not a Desktop app client.`,
    };
  }
  const clientID: unknown = Reflect.get(installed, "client_id");
  const secret: unknown = Reflect.get(installed, "client_secret");
  const project: unknown = Reflect.get(installed, "project_id");
  if (
    typeof clientID !== "string" ||
    typeof secret !== "string" ||
    clientID === "" ||
    secret === ""
  ) {
    return {
      ok: false,
      problem: "This Desktop app client's file holds no client ID or no secret.",
    };
  }
  return { ok: true, clientID, secret, project: typeof project === "string" ? project : undefined };
}
