// A menu button in the ARIA menu pattern, for the account selector and the settings menu (docs/UI.md
// sections 6 and 16). The button says whether the menu is open, opening it moves focus to the first
// item, the arrow keys move between items, and Escape or choosing an item closes it and returns focus to
// the button. A press outside the menu closes it and leaves focus where the press put it.
import type { ComponentChildren } from "preact";
import { useEffect, useRef, useState } from "preact/hooks";

type MenuProps = {
  label: string;
  button: ComponentChildren;
  end?: boolean;
  // children renders the items, each an element with role menuitem or menuitemradio, and takes the
  // function that closes the menu.
  children: (close: () => void) => ComponentChildren;
};

// Phase is whether the menu is open, and when closed whether focus goes back to its button.
type Phase = "closed" | "open" | "returning";

export function Menu(props: MenuProps) {
  const [phase, setPhase] = useState<Phase>("closed");
  const button = useRef<HTMLButtonElement>(null);
  const list = useRef<HTMLUListElement>(null);

  useEffect(() => {
    if (phase === "returning") {
      button.current?.focus();
      setPhase("closed");
      return undefined;
    }
    if (phase !== "open") {
      return undefined;
    }
    menuItems(list.current)[0]?.focus();
    const outside = (event: MouseEvent) => {
      const within = [button.current, list.current].some(
        (el) => el !== null && event.target instanceof Node && el.contains(event.target),
      );
      if (!within) {
        setPhase("closed");
      }
    };
    document.addEventListener("mousedown", outside);
    return () => document.removeEventListener("mousedown", outside);
  }, [phase]);

  const close = () => setPhase("returning");

  const onKeyDown = (event: KeyboardEvent) => {
    const all = menuItems(event.currentTarget instanceof HTMLElement ? event.currentTarget : null);
    const at = all.findIndex((item) => item === document.activeElement);
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      close();
    } else if (event.key === "ArrowDown" || event.key === "ArrowUp") {
      event.preventDefault();
      const step = event.key === "ArrowDown" ? 1 : -1;
      all[(at + step + all.length) % all.length]?.focus();
    }
  };

  const open = phase === "open";
  return (
    <div class={props.end === true ? "menu menu-end" : "menu"}>
      <button
        type="button"
        ref={button}
        aria-haspopup="menu"
        aria-expanded={open}
        aria-label={props.label}
        onClick={() => setPhase(open ? "closed" : "open")}
      >
        {props.button}
      </button>
      {open ? (
        <ul class="menu-list" role="menu" aria-label={props.label} ref={list} onKeyDown={onKeyDown}>
          {props.children(close)}
        </ul>
      ) : null}
    </div>
  );
}

function menuItems(list: HTMLElement | null): HTMLElement[] {
  return Array.from(list?.querySelectorAll<HTMLElement>('[role^="menuitem"]') ?? []);
}
