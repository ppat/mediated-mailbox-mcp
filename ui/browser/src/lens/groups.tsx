// Levels 1 and 2 of the zoom ladder (docs/UI.md sections 4 and 7.3). The bars of one dimension's groups
// and the same groups as a table beside them, every group, 50 to a page. The endpoint answers every
// group at once, so the table pages them here under the view's page=. A click on a group's bar or row
// applies it as a filter and descends.
import type { Groups } from "../app/api.ts";
import { count, share } from "../app/format.ts";
import { dimensionWording } from "../app/wording.ts";
import { descend, descriptor, filterWord, href, withPage, type View } from "../app/url.ts";
import { Bars, type Bar } from "./bars.tsx";
import { RowsTable, type Column } from "./table.tsx";

// perPage is the group table's page size, the one page size of the read API (docs/UI.md section 5).
const perPage = 50;

// groupBars are an answer's groups as bars, each worded by its dimension and leading where a click on
// it descends. The null group is worded by the dimension's null wording.
export function groupBars(
  answer: Groups,
  account: string,
  view: View,
  screen: string | undefined,
  stay: boolean,
): Bar[] {
  const dim = descriptor(view.dataset).dimensions.find((d) => d.name === answer.group);
  return answer.rows.map((row) => {
    const value = Object.values(row.key)[0] ?? null;
    const raw = value === null ? null : String(value);
    const word = filterWord(raw, holdsEmpty(dim));
    const worded =
      raw === null
        ? { text: nullWording(dim), known: true }
        : raw === ""
          ? { text: "empty", known: true }
          : dimensionWording(view.dataset, answer.group, raw);
    return {
      key: raw ?? "\u0000none",
      label: worded.text,
      known: worded.known,
      count: row.count,
      restricted: "restricted" in row ? row.restricted : undefined,
      // A group no filter can name leads nowhere (docs/UI.md section 7.3).
      href: word === undefined ? undefined : href(account, descend(view, word, stay), screen),
    };
  });
}

// holdsEmpty reports whether a dimension declares it can hold a value stored empty.
function holdsEmpty(dim: object | undefined): boolean {
  return dim !== undefined && Reflect.get(dim, "empty") === true;
}

// nullWording is a dimension's wording for its null group, which only some dimensions declare.
function nullWording(dim: object | undefined): string {
  const w: unknown = dim === undefined ? undefined : Reflect.get(dim, "null_wording");
  return typeof w === "string" ? w : "none";
}

// groupColumns are the group table's columns, the group linking where a click descends, its count,
// its share of the total and, for a message-derived dataset, its restricted count.
function groupColumns(header: string, total: number, sensitive: boolean): Column<Bar>[] {
  const columns: Column<Bar>[] = [
    {
      key: "group",
      header,
      width: 240,
      cell: (bar) => {
        const label = (
          <>
            {bar.label}
            {bar.known ? null : <span class="badge">unknown</span>}
          </>
        );
        return bar.href === undefined ? label : <a href={bar.href}>{label}</a>;
      },
      title: (bar) => bar.label,
    },
    { key: "count", header: "count", width: 96, numeric: true, cell: (bar) => count(bar.count) },
    {
      key: "share",
      header: "share",
      width: 80,
      numeric: true,
      cell: (bar) => share(bar.count, total),
    },
  ];
  if (sensitive) {
    columns.push({
      key: "restricted",
      header: "restricted",
      width: 96,
      numeric: true,
      cell: (bar) => count(bar.restricted ?? 0),
    });
  }
  return columns;
}

type GroupsProps = {
  account: string;
  view: View;
  answer: Groups;
  name: string;
  // screen is the path the view's links lead under, the dataset's name when left out.
  screen?: string;
  // stay keeps a click at level 2, for a screen whose rows sit below its groups.
  stay?: boolean;
  // paged false shows every group on one page, for a screen whose page= pages its rows instead.
  paged?: boolean;
  // table false shows the bars alone, for a small breakdown beside another.
  table?: boolean;
  // keys false leaves the row cursor to another table on the screen.
  keys?: boolean;
};

export function GroupsView(props: GroupsProps) {
  const { answer, view } = props;
  const bars = groupBars(answer, props.account, view, props.screen, props.stay === true);
  const size = props.paged === false ? Math.max(1, bars.length) : perPage;
  const page = props.paged === false ? 1 : Math.max(1, Number(view.page ?? "1") || 1);
  const pages = Math.max(1, Math.ceil(bars.length / size));
  const header =
    descriptor(view.dataset).dimensions.find((d) => d.name === answer.group)?.wording ??
    answer.group;
  const columns = groupColumns(header, answer.total.count, "restricted" in answer.total);
  return (
    <div class="groups">
      <Bars name={`${props.name} by ${header}`} bars={bars} />
      {props.table === false ? null : (
        <RowsTable
          caption={`${props.name}, ${count(bars.length)} groups`}
          rowsName="groups"
          columns={columns}
          rows={bars.slice((page - 1) * size, page * size)}
          open={(bar) => bar.href}
          rowKey={(bar) => bar.key}
          keys={props.keys}
          page={page}
          pages={pages}
          pageHref={(p) => href(props.account, withPage(view, p), props.screen)}
        />
      )}
    </div>
  );
}
