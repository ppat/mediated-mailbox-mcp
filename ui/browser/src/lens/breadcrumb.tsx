// The breadcrumb of applied filter chips (docs/UI.md sections 4 and 6), preceded by the lens's name. The
// chips are in the order applied, and removing one removes it and every chip applied after it.
import { dimensionWording } from "../app/wording.ts";
import { chips, descriptor, href, removeChip, type View } from "../app/url.ts";

// chipText is a chip's words, the dimension's wording and its values' wording, with an exclusion
// written as not and an any-of as or. known is false when a value is one the dimension's vocabulary
// does not hold, which the chip shows as the value itself marked unknown (docs/UI.md section 7.1).
export function chipText(
  view: View,
  dimension: string,
  value: string,
): { text: string; known: boolean } {
  const dim = descriptor(view.dataset).dimensions.find((d) => d.name === dimension);
  const name = dim?.wording ?? dimension;
  const exclude = value.startsWith("!");
  const values = (exclude ? value.slice(1) : value)
    .split(",")
    .map((v) => dimensionWording(view.dataset, dimension, v));
  return {
    text: `${name} ${exclude ? "not " : ""}${values.map((v) => v.text).join(" or ")}`,
    known: values.every((v) => v.known),
  };
}

export function Breadcrumb(props: { account: string; name: string; view: View; screen?: string }) {
  return (
    <nav class="breadcrumb" aria-label="Applied filters">
      <span>{props.name}</span>
      {chips(props.view).map(({ filter, index }) => {
        const { text, known } = chipText(props.view, filter.dimension, filter.raw);
        return (
          <a
            key={`${index}-${filter.dimension}`}
            class="chip"
            href={href(props.account, removeChip(props.view, index), props.screen)}
            aria-label={`Remove the filter ${text}${known ? "" : ", unknown"}`}
          >
            {text} {known ? null : <span class="badge">unknown</span>}
            <span aria-hidden="true">×</span>
          </a>
        );
      })}
    </nav>
  );
}
