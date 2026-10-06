// The sender row of docs/UI.md section 7.2, one sender domain of the account's index, as the columns a
// rows table draws. A domain is message-derived text, so it renders as a text child everywhere it shows.
// The corpus lens the domain would link to does not exist yet, so it links nowhere, as the UI links only
// to screens that exist (section 7.1).
import type { SenderRow } from "../app/api.ts";
import { count, local, utc, utcDate } from "../app/format.ts";
import { senderClass } from "../app/wording.ts";
import type { Column } from "../lens/table.tsx";
import { addScreen } from "../screens/policywrites.tsx";
import { Badge, Worded } from "./badge.tsx";

// restricts reports whether a sender is already restricted, by its stored class or by a rule of the
// account's policy its stored class has not caught up with.
export function restricts(row: SenderRow): boolean {
  return row.sender_class === "restricted" || row.restricted_by !== null;
}

// ClassBadge is a stored sender class, restricted in the restricted color and normal in muted text.
export function ClassBadge(props: { stored: string }) {
  if (props.stored === "restricted") {
    return <Badge tone="restricted" text="restricted" />;
  }
  return props.stored === "normal" ? (
    <span class="muted">normal</span>
  ) : (
    <Worded table={senderClass} stored={props.stored} />
  );
}

function day(iso: string | null): string {
  return iso === null ? "" : utcDate(iso);
}

// senderColumns are the sender row's columns. The last names the rule restricting a restricted sender,
// linking to the account's policy searched for it, or for a sender no rule restricts carries Restrict
// {domain}…, which opens Add a rule with the domain as its suffix (section 7.1).
export function senderColumns(account: string): Column<SenderRow>[] {
  return [
    {
      key: "domain",
      header: "domain",
      width: 240,
      cell: (row) => <span class="mono">{row.domain}</span>,
      title: (row) => row.domain,
    },
    {
      key: "sender_class",
      header: "class",
      width: 88,
      cell: (row) => <ClassBadge stored={row.sender_class} />,
    },
    {
      key: "message_count",
      header: "messages",
      width: 88,
      numeric: true,
      cell: (row) => count(row.message_count),
    },
    {
      key: "first_seen",
      header: "first seen",
      width: 96,
      cell: (row) => day(row.first_seen),
      title: (row) =>
        row.first_seen === null ? "" : `${utc(row.first_seen)} · ${local(row.first_seen)}`,
    },
    {
      key: "last_seen",
      header: "last seen",
      width: 96,
      cell: (row) => day(row.last_seen),
      title: (row) =>
        row.last_seen === null ? "" : `${utc(row.last_seen)} · ${local(row.last_seen)}`,
    },
    {
      key: "list_id_ratio",
      header: "list-id",
      width: 72,
      numeric: true,
      cell: (row) => (row.list_id_ratio === null ? "" : `${(row.list_id_ratio * 100).toFixed(1)}%`),
    },
    {
      key: "scan_hits",
      header: "scan hits",
      width: 80,
      numeric: true,
      cell: (row) => count(row.scan_hits),
    },
    {
      key: "rule",
      header: "rule",
      width: 260,
      cell: (row) =>
        row.restricted_by !== null ? (
          <a
            class="mono"
            href={`/${encodeURIComponent(account)}/policy?${new URLSearchParams({ search: row.restricted_by }).toString()}`}
          >
            {row.restricted_by}
          </a>
        ) : row.sender_class === "normal" ? (
          <a href={addScreen(account, { suffixes: [row.domain] })}>Restrict {row.domain}…</a>
        ) : null,
      title: (row) => row.restricted_by ?? `Restrict ${row.domain}…`,
    },
  ];
}
