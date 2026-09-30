// A run's failed items (docs/UI.md sections 7.2 and 8.4). A message or operation item renders as the
// message row with the failure's columns after it, and a page item as the page row, whose page number
// spans the message row's fields so the failure columns stay aligned in one table.
import type { FailureRow } from "../app/api.ts";
import { utc } from "../app/format.ts";
import { disposition, errorClass } from "../app/wording.ts";
import type { Column, Span } from "../lens/table.tsx";
import { Worded } from "./badge.tsx";
import { messageColumns } from "./message.tsx";

// failureColumns are the message row's columns, then the failure's own. detail is a failure's row
// detail, and run a run's screen, where a recovering run links.
export function failureColumns(
  detail: (row: FailureRow) => string,
  run: (id: string) => string,
): Column<FailureRow>[] {
  return [
    ...messageColumns<FailureRow>(detail, (row) => row.item_id),
    {
      key: "page",
      header: "page",
      width: 72,
      numeric: true,
      cell: (row) => row.page,
    },
    {
      key: "error_class",
      header: "error class",
      width: 140,
      cell: (row) => <Worded table={errorClass} stored={row.error_class} />,
    },
    { key: "attempts", header: "attempts", width: 80, numeric: true, cell: (row) => row.attempts },
    {
      key: "last_at",
      header: "last error",
      width: 128,
      cell: (row) => utc(row.last_at),
    },
    {
      key: "disposition",
      header: "disposition",
      width: 180,
      cell: (row) => (
        <>
          <Worded table={disposition} stored={row.disposition} />
          {row.recovered_by === null ? null : (
            <>
              {" by "}
              <a class="mono" href={run(row.recovered_by)}>
                {row.recovered_by}
              </a>
            </>
          )}
        </>
      ),
    },
  ];
}

// pageRow is the page row's spanning cell for a page item, and nothing for any other item, which
// renders as the message row.
export function pageRow(detail: (row: FailureRow) => string): Span<FailureRow> {
  return {
    columns: messageColumns<FailureRow>().length,
    cell: (row) =>
      row.item_kind === "page" ? (
        <a href={detail(row)}>
          page <span class="mono">{row.page ?? row.item_id}</span>
        </a>
      ) : undefined,
  };
}
