// The row-detail panel, level 4 of the zoom ladder (docs/UI.md section 4). L4 is a route, and the panel
// sits over the level 3 list it came from, 480 pixels wide at the right edge and scrolling inside itself.
// The frame makes the page behind it inert, and Escape navigates back to the list's URL. A panel that
// closes somewhere other than its back address, as Add a rule opened from the sender picker returns to
// the picker (docs/UI.md section 8.7), gives close, which Escape and its Close link call instead.
import type { ComponentChildren } from "preact";
import { useEffect, useRef } from "preact/hooks";
import { useLocation } from "preact-iso";

export function DetailPanel(props: {
  title: string;
  back: string;
  close?: () => void;
  children: ComponentChildren;
}) {
  const { route } = useLocation();
  const panel = useRef<HTMLElement>(null);
  const { back, close } = props;
  useEffect(() => {
    // A panel whose content already placed focus inside it, as Add a rule does, keeps it there.
    if (!(panel.current?.contains(document.activeElement) ?? false)) {
      panel.current?.focus();
    }
  }, []);
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        if (close === undefined) {
          route(back);
        } else {
          close();
        }
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [back, close, route]);
  return (
    <aside
      class="panel"
      role="dialog"
      aria-modal="true"
      aria-label={props.title}
      tabIndex={-1}
      ref={panel}
    >
      {close === undefined ? (
        <a href={back}>Close</a>
      ) : (
        <button type="button" onClick={close}>
          Close
        </button>
      )}
      <h2>{props.title}</h2>
      {props.children}
    </aside>
  );
}
