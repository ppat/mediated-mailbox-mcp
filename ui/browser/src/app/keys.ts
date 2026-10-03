// The keyboard map of docs/UI.md section 13, as data, so the map the settings menu and ? show is the
// same list the handlers follow. A binding is listed here with the screen it needs, and lands with it.
export type Binding = { keys: string; action: string };

export const bindings: readonly Binding[] = [
  { keys: "j / k", action: "move the row cursor down / up in the table that holds it" },
  { keys: "Enter", action: "open the row under the cursor" },
  {
    keys: "Escape",
    action: "close the detail panel, a menu or this map, or remove the last filter chip",
  },
  { keys: "g then h", action: "go to Home" },
  { keys: "g then j", action: "go to Jobs" },
  { keys: "g then s", action: "go to System" },
  { keys: "g then a", action: "go to Account settings" },
  { keys: "[ / ]", action: "previous / next page" },
  { keys: "?", action: "show this map" },
];

// ignoresKeys reports whether a key press belongs to what it was typed into, a text field or a
// modified chord, rather than to the keyboard map.
export function ignoresKeys(event: KeyboardEvent): boolean {
  if (event.ctrlKey || event.metaKey || event.altKey) {
    return true;
  }
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  return (
    target.isContentEditable ||
    target instanceof HTMLInputElement ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement
  );
}
