// Violation file. Each directive below can silence one of the browser's bans, and the search for
// suppression directives refuses it (ADR-0072). oxlint honours every one except the upper-case spelling, so
// the ban's finding goes unreported on the other lines and each line wants only the search's finding.
// Nothing imports this module.
export function refused(element: HTMLElement, html: string): void {
  /* want suppression "names no rule" */ // oxlint-disable-next-line
  element.innerHTML = html;
  /* want suppression `names "no-restricted-properties"` */ // eslint-disable-next-line no-debugger, no-restricted-properties -- reason
  element.innerHTML = html;
  /* want suppression `names "@typescript-eslint/no-restricted-properties"` */ // oxlint-disable-next-line @typescript-eslint/no-restricted-properties -- reason
  element.innerHTML = html;
  element.innerHTML = html; /* want suppression `names "eslint/no-restricted-properties"` */ // eslint-disable-line eslint/no-restricted-properties -- reason
  /* want suppression "gives no reason" */ // oxlint-disable-next-line no-debugger
  debugger;
  /* want suppression "does not honour" */ // OXLINT-DISABLE-NEXT-LINE no-restricted-properties -- reason
  element.innerHTML = html; // want oxlint "no-restricted-properties.*'innerHTML'"
}

export const suppressed = { dangerouslySetInnerHTML: { __html: "" } }; /* want suppression "ast-grep-ignore" */ // ast-grep-ignore: markup-prop-ts

/* want suppression `names "no-restricted-properties"` */ /* eslint-disable no-restricted-properties -- reason */
export function fileLevel(element: HTMLElement, html: string): void {
  element.innerHTML = html;
}
