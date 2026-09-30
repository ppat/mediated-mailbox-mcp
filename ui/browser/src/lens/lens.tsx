// The lens shell (docs/UI.md sections 3 and 4). It reads one dataset view through the dataset endpoint
// and shows the summary strip above every level, the breadcrumb of applied filters with the range
// control beside it, and at level 3 the page of rows. A screen gives the lens its name and the columns
// of its row type.
import { useEffect } from "preact/hooks";
import { useLocation } from "preact-iso";
import { isFigures, lensPath, systemPath, type RowsPage, type System } from "../app/api.ts";
import type { State } from "../app/cache.ts";
import { useDeps } from "../app/deps.ts";
import { count } from "../app/format.ts";
import { indexing } from "../app/frame.tsx";
import { ignoresKeys } from "../app/keys.ts";
import { Region } from "../app/region.tsx";
import { apiQuery, chips, href, removeChip, withPage, type View } from "../app/url.ts";
import { Breadcrumb } from "./breadcrumb.tsx";
import { RangeControl } from "./range.tsx";
import { Strip } from "./strip.tsx";
import { RowsTable, type Column } from "./table.tsx";

type LensProps<Row> = {
  account: string;
  name: string;
  view: View;
  rowsName: string;
  columns: readonly Column<Row>[];
  // rows picks the lens's rows out of a page of the dataset's rows, or undefined when the page is not
  // the lens's dataset.
  rows: (page: RowsPage) => readonly Row[] | undefined;
  open?: (row: Row) => string;
  // panelOpen says a row's detail is open, which takes Escape from the breadcrumb.
  panelOpen: boolean;
};

// countsSoFar reports whether the strip's figures are counts so far, while backfill pass 1 runs.
// Without an answer from the system read the index counts as partial, so no count reads as final while
// the read is missing or failed.
export function countsSoFar(system: State<System>): boolean {
  return system.status !== "ok" || indexing(system.answer);
}

// summaryView is the level 0 read of a view, the same filters and range with no page or group.
export function summaryView(view: View): View {
  return { ...view, level: "0", group: undefined, page: undefined };
}

export function Lens<Row>(props: LensProps<Row>) {
  const { lens, system } = useDeps();
  const { route } = useLocation();
  const { account, view } = props;
  const summaryPath = lensPath(account, apiQuery(summaryView(view)));
  const rowsPath = lensPath(account, apiQuery(view));
  const soFar = countsSoFar(system.read(systemPath(account)).value);

  // Escape removes the last chip and ascends, when no panel, menu or dialog has taken it.
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const applied = chips(view);
      const last = applied[applied.length - 1];
      if (event.key !== "Escape" || props.panelOpen || ignoresKeys(event) || last === undefined) {
        return;
      }
      route(href(account, removeChip(view, last.index)));
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [account, view, props.panelOpen, route]);

  return (
    <section aria-label={props.name}>
      <Region
        name="summary"
        state={lens.read(summaryPath)}
        shape="block"
        retry={() => void lens.retry(summaryPath)}
      >
        {(answer) => (isFigures(answer) ? <Strip summary={answer} soFar={soFar} /> : null)}
      </Region>
      <div class="crumbs">
        <Breadcrumb account={account} name={props.name} view={view} />
        <RangeControl account={account} view={view} />
      </div>
      {view.level === "3" ? (
        <Region
          name={props.rowsName}
          state={lens.read(rowsPath)}
          shape="table"
          retry={() => void lens.retry(rowsPath)}
        >
          {(answer) => {
            const rows = isFigures(answer) ? undefined : props.rows(answer);
            if (rows === undefined || isFigures(answer)) {
              return null;
            }
            return (
              <RowsTable
                caption={`${props.name}, ${count(answer.total.count)} in all`}
                rowsName={props.rowsName}
                columns={props.columns}
                rows={rows}
                open={props.open}
                page={answer.page}
                pages={answer.pages}
                pageHref={(page) => href(account, withPage(view, page))}
              />
            );
          }}
        </Region>
      ) : null}
    </section>
  );
}
