// The rows table, level 3 of the zoom ladder (docs/UI.md sections 4 and 7). A plain table over one
// server page of 50 rows, with the page count, a row cursor moved by j and k, Enter opening the row
// under the cursor, and [ and ] for the previous and next page. Every cell stays on one line and
// truncates, and the full value shows on hover. Below the columns' total width the table scrolls inside
// its own container, never the page.
import type { ComponentChild } from "preact";
import { useCallback, useEffect, useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import { ignoresKeys, typing } from "../app/keys.ts";

// Column is one column of a row type. cell renders the value, and title is its full text for the hover.
export type Column<Row> = {
  key: string;
  header: string;
  width: number;
  numeric?: boolean;
  cell: (row: Row) => ComponentChild;
  title?: (row: Row) => string;
};

// Span replaces a row's first columns with one cell when cell returns one, as the page row replaces
// the message row's fields (docs/UI.md section 7.2).
export type Span<Row> = {
  columns: number;
  cell: (row: Row) => ComponentChild | undefined;
};

// Selection makes a table one that selects, the sender picker's (docs/UI.md section 8.7). The row cursor
// and the selection are separate. A box at each row's left shows and toggles its selection, x and Enter
// toggle the row under the cursor, Shift with j or k extends the selection as the cursor moves, and Ctrl
// or Cmd with a selects every row the table's search matches, across its pages, while no field has the
// keys (docs/UI.md section 13).
export type Selection<Row> = {
  selected: (row: Row) => boolean;
  // refused is why a row cannot be selected, or undefined when it can.
  refused: (row: Row) => string | undefined;
  // label names a row's box, "Select {domain}".
  label: (row: Row) => string;
  // set selects the rows, or with on false clears them.
  set: (rows: readonly Row[], on: boolean) => void;
  // all selects every row the search matches.
  all: () => void;
};

type TableProps<Row> = {
  caption: string;
  // rowsName is the plural the empty table names, "No {rowsName} match".
  rowsName: string;
  columns: readonly Column<Row>[];
  rows: readonly Row[];
  // open is where a row leads, its detail or its object, if it leads anywhere. A row it gives nothing
  // for leads nowhere.
  open?: (row: Row) => string | undefined;
  page: number;
  pages: number;
  // pageHref is the view's URL at another page.
  pageHref: (page: number) => string;
  span?: Span<Row>;
  // rowKey is a row's identity, which keeps a row's cells with its row when another row takes its
  // place. A table without one keys its rows by position.
  rowKey?: (row: Row) => string | number;
  // keys false leaves the row cursor's keys to another table on the screen, since one table holds them
  // (docs/UI.md section 13).
  keys?: boolean;
  selection?: Selection<Row>;
};

export function RowsTable<Row>(props: TableProps<Row>) {
  const { route } = useLocation();
  const { rows, open, page, pages, pageHref, rowKey, selection } = props;
  const keys = props.keys !== false;
  // The cursor holds the identity of the row under it, its key where the table keys its rows, so it
  // stays on the operator's row when the rows move (docs/UI.md section 8.3). Without a key it is the
  // row's position.
  const [selected, setSelected] = useState<string | number | undefined>(undefined);
  const identity = useCallback(
    (row: Row, i: number): string | number => (rowKey === undefined ? i : rowKey(row)),
    [rowKey],
  );
  const cursor = rows.findIndex((row, i) => identity(row, i) === selected);

  // A new page starts with no row under the cursor, so Enter never opens a row the operator has not
  // moved to.
  useEffect(() => setSelected(undefined), [page]);

  useEffect(() => {
    if (!keys) {
      return undefined;
    }
    const onKey = (event: KeyboardEvent) => {
      if (
        selection !== undefined &&
        (event.ctrlKey || event.metaKey) &&
        event.key === "a" &&
        !typing(event)
      ) {
        event.preventDefault();
        selection.all();
        return;
      }
      if (ignoresKeys(event)) {
        return;
      }
      const select = (i: number) => {
        const row = rows[i];
        if (row !== undefined) {
          setSelected(identity(row, i));
        }
      };
      const pickable = (row: Row | undefined): row is Row =>
        row !== undefined && selection?.refused(row) === undefined;
      if (selection !== undefined && (event.key === "x" || event.key === "Enter")) {
        const row = rows[cursor];
        if (pickable(row)) {
          selection.set([row], !selection.selected(row));
        }
      } else if (selection !== undefined && (event.key === "J" || event.key === "K")) {
        const next =
          event.key === "J" ? Math.min(rows.length - 1, cursor + 1) : Math.max(0, cursor - 1);
        select(next);
        selection.set([rows[cursor], rows[next]].filter(pickable), true);
      } else if (event.key === "j") {
        select(Math.min(rows.length - 1, cursor + 1));
      } else if (event.key === "k") {
        select(Math.max(0, cursor - 1));
      } else if (event.key === "Enter" && open !== undefined && cursor >= 0) {
        const row = rows[cursor];
        const target = row === undefined ? undefined : open(row);
        if (target !== undefined) {
          route(target);
        }
      } else if (event.key === "[" && page > 1) {
        route(pageHref(page - 1));
      } else if (event.key === "]" && page < pages) {
        route(pageHref(page + 1));
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [keys, rows, open, cursor, identity, page, pages, pageHref, route, selection]);

  // With a fixed table layout the table is never narrower than its columns' total, so the container
  // scrolls when the region is narrower than that.
  return (
    <div class="table-scroll">
      <table class="rows">
        <caption class="label">{props.caption}</caption>
        <colgroup>
          {selection === undefined ? null : <col width={40} />}
          {props.columns.map((c) => (
            <col key={c.key} width={c.width} />
          ))}
        </colgroup>
        <thead>
          <tr>
            {selection === undefined ? null : (
              <th scope="col">
                <span class="visually-hidden">selected</span>
              </th>
            )}
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
              <td colSpan={props.columns.length + (selection === undefined ? 0 : 1)}>
                No {props.rowsName} match
              </td>
            </tr>
          ) : (
            rows.map((row, i) => (
              <tr
                key={props.rowKey === undefined ? i : props.rowKey(row)}
                aria-selected={i === cursor}
                onClick={() => {
                  setSelected(identity(row, i));
                }}
              >
                {selection === undefined ? null : (
                  <SelectBox row={row} index={i} selection={selection} />
                )}
                <Cells row={row} columns={props.columns} span={props.span} />
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

// SelectBox is a row's selection box, a checkbox labelled by its row. A row the selection refuses has
// its box disabled and described by why (docs/UI.md section 8.7).
function SelectBox<Row>(props: { row: Row; index: number; selection: Selection<Row> }) {
  const { row, selection } = props;
  const refused = selection.refused(row);
  const id = `select-why-${props.index}`;
  return (
    <td class="select-box">
      <input
        type="checkbox"
        checked={selection.selected(row)}
        disabled={refused !== undefined}
        aria-label={selection.label(row)}
        aria-describedby={refused === undefined ? undefined : id}
        onChange={(event) => selection.set([row], event.currentTarget.checked)}
      />
      {refused === undefined ? null : (
        <span id={id} class="visually-hidden">
          {refused}
        </span>
      )}
    </td>
  );
}

// Cells are one row's cells, its first columns spanned by one cell when the span gives one.
function Cells<Row>(props: { row: Row; columns: readonly Column<Row>[]; span?: Span<Row> }) {
  const { row, columns, span } = props;
  const spanned = span?.cell(row);
  const shown = spanned === undefined || span === undefined ? columns : columns.slice(span.columns);
  return (
    <>
      {spanned === undefined || span === undefined ? null : (
        <td key="span" colSpan={span.columns}>
          {spanned}
        </td>
      )}
      {shown.map((c) => (
        <td key={c.key} class={c.numeric === true ? "numeric" : undefined} title={c.title?.(row)}>
          {c.cell(row)}
        </td>
      ))}
    </>
  );
}
