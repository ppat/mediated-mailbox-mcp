// The message row of docs/UI.md section 7.1, the one component every message-derived row renders
// through, as the columns a rows table draws. The fields keep their order and widths, every cell stays
// on one line and shows its full value on hover, and every message-derived value is a text child, so
// markup in it arrives as text (section 11). A lens adds its own columns after these.
import { utc, utcDate } from "../app/format.ts";
import { contentFlag, scanState, scanStateLong, senderClass, word } from "../app/wording.ts";
import type { Column } from "../lens/table.tsx";
import { Badge, Worded } from "./badge.tsx";

// MessageFields are the message row's wire fields. A dataset whose rows can lack a message, such as a
// run's failures, carries them as null there.
export type MessageFields = {
  message_id: string | null;
  from_email: string | null;
  subject: string | null;
  sent_at: string | null;
  labels: readonly string[] | null;
  sender_class: string | null;
  content_flags: readonly string[] | null;
  scan_state: string | null;
};

// shownLabels is how many labels a cell shows as chips before the +N.
const shownLabels = 4;

// messageColumns are the message row's seven columns. detail is where the subject leads, the row's
// detail, and is left out where no detail exists. A row with no message fields shows its message's
// identifier, which identity gives where the row carries it apart from the message fields, and "not in
// the index", since its message is gone from the index.
export function messageColumns<Row extends MessageFields>(
  detail?: (row: Row) => string,
  identity: (row: Row) => string | null = (row) => row.message_id,
): Column<Row>[] {
  return [
    {
      key: "from_email",
      header: "sender",
      width: 220,
      cell: (row) => <span class="mono">{row.from_email ?? identity(row)}</span>,
      title: (row) => row.from_email ?? identity(row) ?? "",
    },
    {
      key: "subject",
      header: "subject",
      width: 240,
      cell: (row) => {
        if (row.from_email === null) {
          return <span class="muted">not in the index</span>;
        }
        const target = detail?.(row);
        return target === undefined ? row.subject : <a href={target}>{row.subject}</a>;
      },
      title: (row) => row.subject ?? "",
    },
    {
      key: "sent_at",
      header: "date",
      width: 88,
      cell: (row) => (row.sent_at === null ? null : utcDate(row.sent_at)),
      title: (row) => (row.sent_at === null ? "" : utc(row.sent_at)),
    },
    {
      key: "labels",
      header: "labels",
      width: 200,
      cell: (row) => <Labels labels={row.labels ?? []} />,
      title: (row) => (row.labels ?? []).join(", "),
    },
    {
      key: "sender_class",
      header: "class",
      width: 72,
      cell: (row) => (row.sender_class === null ? null : <SenderClass stored={row.sender_class} />),
    },
    {
      key: "content_flags",
      header: "flags",
      width: 144,
      cell: (row) =>
        (row.content_flags ?? []).map((f) => {
          const w = word(contentFlag, f);
          return w.known ? (
            <Badge key={f} tone="flagged" text={w.text} />
          ) : (
            <Worded key={f} table={contentFlag} stored={f} />
          );
        }),
    },
    {
      key: "scan_state",
      header: "scan",
      width: 96,
      cell: (row) =>
        row.scan_state === null ? null : (
          <span class="muted">
            <Worded table={scanState} stored={row.scan_state} />
          </span>
        ),
      title: (row) => (row.scan_state === null ? "" : word(scanStateLong, row.scan_state).text),
    },
  ];
}

// Labels shows the first four labels as chips and a count of the rest. Below 1360 pixels the stylesheet
// shows the narrow form instead, one chip and the count of the rest.
function Labels(props: { labels: readonly string[] }) {
  const { labels } = props;
  const rest = (n: number) =>
    labels.length > n ? <span class="muted">+{labels.length - n}</span> : null;
  return (
    <>
      <span class="labels-wide">
        {labels.slice(0, shownLabels).map((l, i) => (
          <span key={i} class="label-chip">
            {l}
          </span>
        ))}
        {rest(shownLabels)}
      </span>
      <span class="labels-narrow">
        {labels.slice(0, 1).map((l, i) => (
          <span key={i} class="label-chip">
            {l}
          </span>
        ))}
        {rest(1)}
      </span>
    </>
  );
}

// SenderClass is restricted in the restricted color, normal in muted text, and any other value marked
// unknown.
function SenderClass(props: { stored: string }) {
  const w = word(senderClass, props.stored);
  if (!w.known) {
    return <Worded table={senderClass} stored={props.stored} />;
  }
  return props.stored === "restricted" ? (
    <Badge tone="restricted" text={w.text} />
  ) : (
    <span class="muted">{w.text}</span>
  );
}
