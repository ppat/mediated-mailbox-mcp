// Violation file. Each line below builds markup from a string in a way the browser lint bans, so the
// ban is proven by its want annotation (ADR-0063, ADR-0072). Nothing imports this module.
export function hatches(html: string, element: HTMLElement, range: Range, frame: HTMLIFrameElement): string {
  element.innerHTML = html; // want oxlint "no-restricted-properties.*'innerHTML'"
  element["innerHTML"] = html; // want oxlint "no-restricted-properties.*'innerHTML'"
  const { outerHTML } = element; // want oxlint "no-restricted-properties.*'outerHTML'"
  element.insertAdjacentHTML("beforeend", html); // want oxlint "no-restricted-properties.*'insertAdjacentHTML'"
  range.createContextualFragment(html); // want oxlint "no-restricted-properties.*'createContextualFragment'"
  element.setHTMLUnsafe(html); // want oxlint "no-restricted-properties.*'setHTMLUnsafe'"
  Document.parseHTMLUnsafe(html); // want oxlint "no-restricted-properties.*'parseHTMLUnsafe'"
  new DOMParser().parseFromString(html, "text/html"); // want oxlint "no-restricted-properties.*'parseFromString'"
  frame.srcdoc = html; // want oxlint "no-restricted-properties.*'srcdoc'"
  document.write(html); // want oxlint "no-restricted-properties.*'document.write'"
  document.writeln(html); // want oxlint "no-restricted-properties.*'document.writeln'"
  return outerHTML;
}

export function Row(props: { html: string }) {
  return <div dangerouslySetInnerHTML={{ __html: props.html }} />; // want oxlint "no-danger"
}

export function Spread(props: { html: string }) {
  const markup = { dangerouslySetInnerHTML: { __html: props.html } }; // want ast-grep "markup-prop-tsx"
  return <div {...markup} />;
}

export function InlineSpread(props: { html: string }) {
  return <div {...{ "dangerouslySetInnerHTML": { __html: props.html } }} />; // want ast-grep "markup-prop-tsx"
}

export function Assigned(props: { html: string }) {
  const markup: Record<string, unknown> = {};
  markup.dangerouslySetInnerHTML = { __html: props.html }; // want oxlint "no-restricted-properties.*'dangerouslySetInnerHTML'"
  return <div {...markup} title={props.html} />;
}
