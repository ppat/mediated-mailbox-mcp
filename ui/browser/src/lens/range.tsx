// The range control (docs/UI.md section 6), on every dataset with a range. It offers the presets and a
// custom range of two UTC dates, each a whole day with the end included, and writes range=.
import { useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import { descriptor, href, withRange, type View } from "../app/url.ts";

export const presets = [
  { range: "24h", wording: "24 hours" },
  { range: "7d", wording: "7 days" },
  { range: "30d", wording: "30 days" },
  { range: "90d", wording: "90 days" },
  { range: "all", wording: "all time" },
] as const;

// customRange reads the range= value of two dates, or undefined when the range is a preset.
export function customRange(range: string | undefined): { from: string; to: string } | undefined {
  const [from, to, ...rest] = (range ?? "").split(",");
  return from !== undefined && to !== undefined && rest.length === 0 ? { from, to } : undefined;
}

export function RangeControl(props: { account: string; view: View }) {
  const { route } = useLocation();
  const custom = customRange(props.view.range);
  const [from, setFrom] = useState(custom?.from ?? "");
  const [to, setTo] = useState(custom?.to ?? "");
  if (!descriptor(props.view.dataset).ranged) {
    return null;
  }
  return (
    <div class="range" role="group" aria-label="Time range, UTC">
      {presets.map((p) => (
        <a
          key={p.range}
          class="preset"
          href={href(props.account, withRange(props.view, p.range))}
          aria-current={props.view.range === p.range ? "true" : undefined}
        >
          {p.wording}
        </a>
      ))}
      <form
        class="range"
        onSubmit={(event) => {
          event.preventDefault();
          route(href(props.account, withRange(props.view, `${from},${to}`)));
        }}
      >
        <input
          type="date"
          aria-label="From, UTC"
          value={from}
          onInput={(event) => setFrom(event.currentTarget.value)}
          required
        />
        <input
          type="date"
          aria-label="To, UTC, included"
          value={to}
          onInput={(event) => setTo(event.currentTarget.value)}
          required
        />
        <button type="submit">Apply</button>
      </form>
    </div>
  );
}
