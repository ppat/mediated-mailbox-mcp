// Violation file. Each directive below names only ordinary rules and gives a reason, so the search for
// suppression directives allows it (ADR-0072). oxlint honours each, so no debugger statement is reported,
// and a directive naming an ordinary rule on a ban's line leaves the ban reported. Nothing imports this
// module.
export function allowed(element: HTMLElement, html: string): void {
  // oxlint-disable-next-line no-debugger -- a statement the directive must silence
  debugger;
  debugger; // eslint-disable-line eslint/no-debugger -- a statement the directive must silence
  element.innerHTML = html; /* want oxlint "no-restricted-properties.*'innerHTML'" */ // oxlint-disable-line no-debugger -- an ordinary rule
}

/* eslint-disable no-debugger -- the rest of the file holds statements the directive must silence */
export function fileLevel(): void {
  debugger;
}
