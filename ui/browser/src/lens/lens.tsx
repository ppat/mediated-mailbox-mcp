// The lens shell (docs/UI.md sections 3 and 4). It reads one dataset view through the dataset endpoint
// and shows the summary strip above every level, the breadcrumb of applied filters with the range
// control beside it, and at level 3 the page of rows. A screen gives the lens its name and the columns
// of its row type.
import { useEffect } from "preact/hooks";
import { useLocation } from "preact-iso";
import {
  isFigures,
  isGroups,
  isRowsPage,
  lensPath,
  systemPath,
  type LensFigures,
  type RowsPage,
  type System,
} from "../app/api.ts";
import type { State } from "../app/cache.ts";
import { useDeps } from "../app/deps.ts";
import { count } from "../app/format.ts";
import { indexing } from "../app/frame.tsx";
import { ignoresKeys } from "../app/keys.ts";
import { Region } from "../app/region.tsx";
import { apiQuery, chips, href, removeChip, withPage, type View } from "../app/url.ts";
import { Breadcrumb } from "./breadcrumb.tsx";
import { GroupBy } from "./groupby.tsx";
import { GroupsView } from "./groups.tsx";
import { RangeControl } from "./range.tsx";
import { Strip } from "./strip.tsx";
import { RowsTable, type Column } from "./table.tsx";

type LensProps<Row> = {
  account: string;
  name: string;
  view: View;
  // screen is the path below the account the lens's links lead to, the dataset's name when left out.
  screen?: string;
  rowsName: string;
  columns: readonly Column<Row>[];
  // rows picks the lens's rows out of a page of the dataset's rows, or undefined when the page is not
  // the lens's dataset.
  rows: (page: RowsPage) => readonly Row[] | undefined;
  open?: (row: Row) => string;
  // rowKey is a row's identity, which keeps each row's cells with its row as the page changes.
  rowKey?: (row: Row) => string | number;
  // panelOpen says a row's detail is open, which takes Escape from the breadcrumb.
  panelOpen: boolean;
};

// countsSoFar reports whether the strip's figures are counts so far, while a first backfill pass 1
// runs, and not while a re-opened one does. Without an answer from the system read the index counts
// as partial, so no count reads as final while the read is missing or failed.
export function countsSoFar(system: State<System>): boolean {
  return system.status !== "ok" || indexing(system.answer);
}

// SummaryStrip is the strip with its figures counted so far while a first backfill pass 1 runs. It
// reads the system endpoint itself, so a new answer there redraws the strip alone and not the lens
// around it.
function SummaryStrip(props: { account: string; summary: LensFigures }) {
  const { system } = useDeps();
  const soFar = countsSoFar(system.read(systemPath(props.account)).value);
  return <Strip summary={props.summary} soFar={soFar} />;
}

// summaryView is the level 0 read of a view, the same filters and range with no page or group.
export function summaryView(view: View): View {
  return { ...view, level: "0", group: undefined, page: undefined };
}

// useChipEscape makes Escape remove the last chip and ascend, when no panel, menu or dialog has taken
// it (docs/UI.md section 13).
export function useChipEscape(
  account: string,
  view: View,
  screen: string | undefined,
  panelOpen: boolean,
): void {
  const { route } = useLocation();
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      const applied = chips(view);
      const last = applied[applied.length - 1];
      if (event.key !== "Escape" || panelOpen || ignoresKeys(event) || last === undefined) {
        return;
      }
      route(href(account, removeChip(view, last.index), screen));
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  }, [account, view, screen, panelOpen, route]);
}

export function Lens<Row>(props: LensProps<Row>) {
  const { lens } = useDeps();
  const { account, view } = props;
  const { screen } = props;
  const summaryPath = lensPath(account, apiQuery(summaryView(view)));
  const rowsPath = lensPath(account, apiQuery(view));
  const grouped = view.level === "1" || view.level === "2";

  useChipEscape(account, view, screen, props.panelOpen);

  return (
    <section aria-label={props.name}>
      <Region
        name="summary"
        state={lens.read(summaryPath)}
        shape="block"
        retry={() => void lens.retry(summaryPath)}
      >
        {(answer) =>
          isFigures(answer) ? <SummaryStrip account={account} summary={answer} /> : null
        }
      </Region>
      <div class="crumbs">
        <Breadcrumb account={account} name={props.name} view={view} screen={screen} />
        {/* The custom range's fields start from the view's range, so another range starts them afresh. */}
        <RangeControl key={view.range ?? ""} account={account} view={view} screen={screen} />
        <GroupBy account={account} view={view} screen={screen} />
      </div>
      {grouped ? (
        <Region
          name={props.rowsName}
          state={lens.read(rowsPath)}
          shape="chart"
          retry={() => void lens.retry(rowsPath)}
        >
          {(answer) =>
            isGroups(answer) ? (
              <GroupsView
                account={account}
                view={view}
                answer={answer}
                name={props.name}
                screen={screen}
              />
            ) : null
          }
        </Region>
      ) : null}
      {view.level === "3" ? (
        <Region
          name={props.rowsName}
          state={lens.read(rowsPath)}
          shape="table"
          retry={() => void lens.retry(rowsPath)}
        >
          {(answer) => {
            const rows = isRowsPage(answer) ? props.rows(answer) : undefined;
            if (rows === undefined || !isRowsPage(answer)) {
              return null;
            }
            return (
              <RowsTable
                caption={`${props.name}, ${count(answer.total.count)} in all`}
                rowsName={props.rowsName}
                columns={props.columns}
                rows={rows}
                open={props.open}
                rowKey={props.rowKey}
                page={answer.page}
                pages={answer.pages}
                pageHref={(page) => href(account, withPage(view, page), screen)}
              />
            );
          }}
        </Region>
      ) : null}
    </section>
  );
}
