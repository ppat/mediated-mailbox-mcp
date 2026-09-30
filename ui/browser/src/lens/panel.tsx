// The row-detail panel, level 4 of the zoom ladder (docs/UI.md section 4). L4 is a route, and the panel
// sits over the level 3 list it came from, 480 pixels wide at the right edge and scrolling inside itself.
// The frame makes the page behind it inert, and Escape navigates back to the list's URL.
import type { ComponentChildren } from "preact";
import { useEffect, useRef } from "preact/hooks";
import { useLocation } from "preact-iso";

export function DetailPanel(props: { title: string; back: string; children: ComponentChildren }) {
  const { route } = useLocation();
  const panel = useRef<HTMLElement>(null);
  useEffect(() => {
    panel.current?.focus();
    const onKey = (event: KeyboardEvent) => {
      if (event.key === "Escape") {
        route(props.back);
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [props.back, route]);
  return (
    <aside
      class="panel"
      role="dialog"
      aria-modal="true"
      aria-label={props.title}
      tabIndex={-1}
      ref={panel}
    >
      <a href={props.back}>Close</a>
      <h2>{props.title}</h2>
      {props.children}
    </aside>
  );
}
