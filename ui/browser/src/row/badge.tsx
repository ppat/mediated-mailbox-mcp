// Badges and worded values (docs/UI.md sections 7.1 and 14.2). A badge is outlined in its semantic
// color with text of the same color and always carries its label, so no status is told by color alone.
// A stored value the wording does not know is shown as itself in muted text with an unknown badge.
import { word } from "../app/wording.ts";

export type Tone = "restricted" | "flagged" | "ok" | "info" | "muted";

export function Badge(props: { tone: Tone; text: string }) {
  return (
    <span class="badge" data-tone={props.tone}>
      {props.text}
    </span>
  );
}

// Worded shows a stored value in its vocabulary's wording, or the value marked unknown.
export function Worded(props: { table: Readonly<Record<string, string>>; stored: string }) {
  const w = word(props.table, props.stored);
  if (w.known) {
    return <>{w.text}</>;
  }
  return (
    <>
      <span class="muted">{w.text}</span> <Badge tone="muted" text="unknown" />
    </>
  );
}
