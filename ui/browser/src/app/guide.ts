// The setup guide's windows and what passes between them and the page (docs/UI.md sections 8.11 and
// 8.12). The guide sits beside Google's tab in a small window that stays on top where the browser
// offers Document Picture-in-Picture, as Chrome and Edge do, and in a narrow window of the same page
// anywhere else. Either way the guide and the page are documents of one origin, and they tell each
// other what changed over one channel, so a step marked done in either window opens the next in both.
// Nothing here is ambient. The composition root binds each part to the browser, and a test to its own.

// GuideMessage is one thing the page or its guide tells the other. A setup step names its client's
// marks key and the step now in view. A pasted address goes from the paste guide to the page, and the
// checks the page made of it go back, so the guide holds no attempt and nothing about any account.
export type GuideMessage =
  | { kind: "step"; key: string; step: number }
  | { kind: "address"; address: string }
  | { kind: "checks"; checks: PasteChecks | null };

// PasteChecks are the three checks of a pasted address the page names before anything is sent
// (section 8.12).
export type PasteChecks = { address: boolean; state: boolean; code: boolean };

export type Channel = {
  post: (message: GuideMessage) => void;
  // listen calls on with every message another document posts, and returns what stops it.
  listen: (on: (message: GuideMessage) => void) => () => void;
};

export type Guides = {
  channel: Channel;
  // pip opens a window of the given size that stays above every other, when the browser supports one.
  pip: ((width: number, height: number) => Promise<Window>) | undefined;
  // open opens a named window with the given features, or returns null when the browser blocked it.
  open: (url: string, name: string, features: string) => Window | null;
  // copy puts text on the clipboard, and paste reads it.
  copy: (text: string) => Promise<void>;
  paste: () => Promise<string>;
  // console opens a Google Cloud console page in the one named console tab every step reuses.
  console: (url: string) => void;
};

// consoleTab is the name of the one tab every console link opens in.
export const consoleTab = "mediated-mailbox-console";

// guideWindow is the name of the side window, so opening it again reuses it.
export const guideWindow = "mediated-mailbox-guide";

// sideFeatures sizes the side window and places it at the right edge of the screen, where the browser
// allows (section 8.11).
export function sideFeatures(screenWidth: number): string {
  const width = 360;
  return `popup,width=${width},height=640,left=${Math.max(0, screenWidth - width)},top=0`;
}

// The small window's size, about 340 by 520 pixels (section 8.11).
export const pipSize = { width: 340, height: 520 } as const;

// linkStylesheet links the bundle's stylesheet into a guide window's document once, since the content
// security policy refuses inline styles (docs/UI.md section 15).
export function linkStylesheet(doc: Document): void {
  if (doc.head.querySelector('link[href="/main.css"]') !== null) {
    return;
  }
  const link = doc.createElement("link");
  link.rel = "stylesheet";
  link.href = "/main.css";
  doc.head.append(link);
}

// redirectHost is the host and port of the redirect address, as an address bar shows them, or empty
// for an address the browser cannot read.
export function redirectHost(redirect: string): string {
  try {
    return new URL(redirect).host;
  } catch {
    return "";
  }
}

// readPasted reads a pasted address for the three checks, against the configured redirect address
// and the attempt's state. It names only what holds, and tells nothing else about the attempt.
export function readPasted(text: string, state: string, redirect: string): PasteChecks | null {
  const trimmed = text.trim();
  if (trimmed === "") {
    return null;
  }
  let url: URL;
  try {
    // An address pasted without its scheme is read as plain HTTP, as the address bar may leave it out.
    url = new URL(trimmed.includes("://") ? trimmed : "http://" + trimmed);
  } catch {
    return { address: false, state: false, code: false };
  }
  return {
    address:
      url.protocol === "http:" &&
      redirectHost(redirect) !== "" &&
      url.host === redirectHost(redirect),
    state: url.searchParams.get("state") === state,
    code: (url.searchParams.get("code") ?? "") !== "",
  };
}
