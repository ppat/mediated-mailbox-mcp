// Violation file. Each line carrying a want hands an element the markup property through an object rather
// than a JSX attribute, which the browser lint bans (ADR-0063, ADR-0072). The line without one passes text
// through the same kind of object and must not be reported. Nothing imports this module.
import { h } from "preact";

export function viaCall(html: string) {
  return h("div", { dangerouslySetInnerHTML: { __html: html } }); // want ast-grep "markup-prop-ts"
}

export function viaShorthand(html: string) {
  const dangerouslySetInnerHTML = { __html: html };
  return h("div", { dangerouslySetInnerHTML }); // want ast-grep "markup-prop-ts"
}

export function asText(html: string) {
  return h("div", { title: html }, html);
}
