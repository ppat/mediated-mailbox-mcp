// The rows table, level 3 of the zoom ladder (docs/UI.md sections 4 and 7). A plain table over one
// server page of 50 rows, with the page count, a row cursor moved by j and k, Enter opening the row
// under the cursor, and [ and ] for the previous and next page. Every cell stays on one line and
// truncates, and the full value shows on hover. Below the columns' total width the table scrolls inside
// its own container, never the page.
import type { ComponentChild } from "preact";
import { useEffect, useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import { ignoresKeys } from "../app/keys.ts";

// Column is one column of a row type. cell renders the value, and title is its full text for the hover.
export type Column<Row> = {
  key: string;
  header: string;
  width: number;
  numeric?: boolean;
  cell: (row: Row) => ComponentChild;
  title?: (row: Row) => string;
};

type TableProps<Row> = {
  caption: string;
  // rowsName is the plural the empty table names, "No {rowsName} match".
  rowsName: string;
  columns: readonly Column<Row>[];
  rows: readonly Row[];
  // open is where a row leads, its detail or its object, if it leads anywhere.
  open?: (row: Row) => string;
  page: number;
  pages: number;
  // pageHref is the view's URL at another page.
  pageHref: (page: number) => string;
};

export function RowsTable<Row>(props: TableProps<Row>) {
  const { route } = useLocation();
  const [cursor, setCursor] = useState(-1);
  const { rows, open, page, pages, pageHref } = props;

  // A new page starts with no row under the cursor, so Enter never opens a row the operator has not
  // moved to.
  useEffect(() => setCursor(-1), [page]);

  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (ignoresKeys(event)) {
        return;
      }
      if (event.key === "j") {
        setCursor((c) => Math.min(rows.length - 1, c + 1));
      } else if (event.key === "k") {
        setCursor((c) => Math.max(0, c - 1));
      } else if (event.key === "Enter" && open !== undefined && cursor >= 0) {
        const row = rows[cursor];
        if (row !== undefined) {
          route(open(row));
        }
      } else if (event.key === "[" && page > 1) {
        route(pageHref(page - 1));
      } else if (event.key === "]" && page < pages) {
        route(pageHref(page + 1));
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [rows, open, cursor, page, pages, pageHref, route]);

  // With a fixed table layout the table is never narrower than its columns' total, so the container
  // scrolls when the region is narrower than that.
  return (
    <div class="table-scroll">
      <table class="rows">
        <caption class="label">{props.caption}</caption>
        <colgroup>
          {props.columns.map((c) => (
            <col key={c.key} width={c.width} />
          ))}
        </colgroup>
        <thead>
          <tr>
            {props.columns.map((c) => (
              <th key={c.key} scope="col" class={c.numeric === true ? "numeric" : undefined}>
                {c.header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.length === 0 ? (
            <tr>
              <td colSpan={props.columns.length}>No {props.rowsName} match</td>
            </tr>
          ) : (
            rows.map((row, i) => (
              <tr
                key={i}
                aria-selected={i === cursor}
                onClick={() => {
                  setCursor(i);
                }}
              >
                {props.columns.map((c) => (
                  <td
                    key={c.key}
                    class={c.numeric === true ? "numeric" : undefined}
                    title={c.title?.(row)}
                  >
                    {c.cell(row)}
                  </td>
                ))}
              </tr>
            ))
          )}
        </tbody>
      </table>
      <nav class="pager" aria-label="Pages">
        {page > 1 ? <a href={pageHref(page - 1)}>Previous page</a> : null}
        <span class="muted">
          Page {page} of {Math.max(1, pages)}
        </span>
        {page < pages ? <a href={pageHref(page + 1)}>Next page</a> : null}
      </nav>
    </div>
  );
}
