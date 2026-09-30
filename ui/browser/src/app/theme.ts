// The theme override of docs/UI.md section 14.2. The OS preference selects the palette unless an
// override is in force, and an override chosen in the settings menu is kept per browser and wins over
// the deployment's default_theme (section 18.1). The stylesheet reads the override from the root
// element's data-theme attribute, which system leaves unset.
export const themes = ["system", "dark", "light"] as const;
export type Theme = (typeof themes)[number];

const key = "mediated-mailbox.theme";

function isTheme(value: string | null): value is Theme {
  return themes.some((t) => t === value);
}

// storedTheme reads the override kept in this browser, and the deployment's default when there is none.
export function storedTheme(storage: Storage | undefined, fallback: Theme): Theme {
  try {
    const value = storage?.getItem(key) ?? null;
    return isTheme(value) ? value : fallback;
  } catch {
    return fallback;
  }
}

// applyTheme sets a theme on the root element.
export function applyTheme(root: HTMLElement, theme: Theme): void {
  if (theme === "system") {
    delete root.dataset["theme"];
  } else {
    root.dataset["theme"] = theme;
  }
}

// keepTheme keeps the operator's choice in this browser.
export function keepTheme(storage: Storage | undefined, theme: Theme): void {
  try {
    storage?.setItem(key, theme);
  } catch {
    // A browser refusing storage keeps the choice for this page load only.
  }
}
