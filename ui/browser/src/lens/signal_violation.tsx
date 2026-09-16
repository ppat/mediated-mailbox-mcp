// Violation file. Lines carrying a want read a signal's value inside a rendering position, which the
// ast-grep rule bans (ADR-0063, ADR-0072). Lines without one are reads the rule must leave alone, so a
// rule that over-reports turns the proof red too. Nothing imports this module.
import { signal } from "@preact/signals";

const count = signal(0);

export function Lens() {
  return (
    <section
      title={String(count.value)} // want ast-grep "signal-value-in-render"
      onClick={() => count.value++}
      onInput={function () {
        count.value = 0;
      }}
    >
      {count.value /* want ast-grep "signal-value-in-render" */}
      {[1, 2].map((step) => step + count.value) /* want ast-grep "signal-value-in-render" */}
      {count}
    </section>
  );
}
