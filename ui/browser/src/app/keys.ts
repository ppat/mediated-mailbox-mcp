// The keyboard map of docs/UI.md section 13, as data, so the map the settings menu and ? show is the
// same list the handlers follow. A binding is listed here with the screen it needs, and lands with it.
export type Binding = { keys: string; action: string };

export const bindings: readonly Binding[] = [
  { keys: "j / k", action: "move the row cursor down / up in the table that holds it" },
  {
    keys: "Enter",
    action: "open the row under the cursor, or in the sender picker toggle its selection",
  },
  {
    keys: "Escape",
    action:
      "clear a selection, else close the detail panel, a dialog, a menu or this map, else remove the last filter chip",
  },
  { keys: "g then h", action: "go to Home" },
  { keys: "g then j", action: "go to Jobs" },
  { keys: "g then o", action: "go to Policy" },
  { keys: "g then s", action: "go to System" },
  { keys: "g then a", action: "go to Account settings" },
  { keys: "x", action: "in the sender picker, toggle the selection of the row under the cursor" },
  { keys: "Shift+j / Shift+k", action: "in the sender picker, extend the selection down / up" },
  {
    keys: "Ctrl+a or Cmd+a",
    action: "in the sender picker, select every sender the search matches, across its pages",
  },
  { keys: "r", action: "in the sender picker, while a selection exists, restrict as one rule" },
  { keys: "[ / ]", action: "previous / next page" },
  { keys: "?", action: "show this map" },
];

// ignoresKeys reports whether a key press belongs to what it was typed into, a text field or a
// modified chord, rather than to the keyboard map.
export function ignoresKeys(event: KeyboardEvent): boolean {
  if (event.ctrlKey || event.metaKey || event.altKey) {
    return true;
  }
  return typing(event);
}

// typing reports whether a key press was typed into a field, where every key belongs to the field, so
// Ctrl or Cmd with a selects the field's text there (docs/UI.md section 13). A selection box or a
// radio takes no text, so a key pressed on one belongs to the keyboard map, and the sender picker's
// keys work after a click on a row's box.
export function typing(event: KeyboardEvent): boolean {
  const target = event.target;
  if (!(target instanceof HTMLElement)) {
    return false;
  }
  return (
    target.isContentEditable ||
    (target instanceof HTMLInputElement && !boxes.has(target.type)) ||
    target instanceof HTMLTextAreaElement ||
    target instanceof HTMLSelectElement
  );
}

const boxes = new Set(["checkbox", "radio"]);
