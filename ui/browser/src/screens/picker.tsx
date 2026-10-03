// Restrict senders from the index, the sender picker of docs/UI.md section 8.7. The senders dataset at
// level 3 with its search, the sender row's fields, a selection box at each row's left, and for a
// restricted sender the rule restricting it. The selection rides in the address as pick= with each
// selected domain, or pick=all for every sender the search matches, so Back and a reload keep it, and
// the dataset's read never carries it. Restrict as one rule hands the selection to Add a rule by its
// address, as the suffixes it adds, or as the search when the selection is every match.
import { useEffect } from "preact/hooks";
import { useLocation } from "preact-iso";
import { isRowsPage, lensPath, type RowsPage, type SenderRow } from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { count, counted } from "../app/format.ts";
import { typing } from "../app/keys.ts";
import { apiQuery, serialize, type View } from "../app/url.ts";
import { Lens } from "../lens/lens.tsx";
import type { Selection } from "../lens/table.tsx";
import { restricts, senderColumns } from "../row/sender.tsx";
import { searchOf, SearchBox, withSearch } from "./policy.tsx";
import { addScreen, openAdd, policyHome } from "./policywrites.tsx";

// Picked is the selection the address holds, every match or the domains named.
export type Picked = { all: true } | { all: false; domains: readonly string[] };

// pickedOf reads the selection from a view's pick filters, which the picker keeps out of the read.
export function pickedOf(view: View): Picked {
  const picks = view.filters.filter((f) => f.dimension === "pick").map((f) => f.raw);
  return picks.includes("all")
    ? { all: true }
    : { all: false, domains: picks.filter((p) => p !== "") };
}

// withoutPicks is the view the dataset is read and paged with, the selection left out, so another page
// or another search starts with nothing selected.
export function withoutPicks(view: View): View {
  return { ...view, filters: view.filters.filter((f) => f.dimension !== "pick") };
}

// pickerHref is the picker's address for a view and a selection.
export function pickerHref(account: string, view: View, picked: Picked): string {
  const params = new URLSearchParams(serialize(withoutPicks(view)));
  if (picked.all) {
    params.append("pick", "all");
  } else {
    for (const d of picked.domains) {
      params.append("pick", d);
    }
  }
  return `${policyHome(account)}/pick?${params.toString().replaceAll("%2C", ",").replaceAll("%21", "!")}`;
}

// restrictHref is Add a rule's address for a selection, the selected domains as its suffixes, or the
// search when the selection is every match.
export function restrictHref(account: string, view: View, picked: Picked): string {
  return picked.all
    ? addScreen(account, { search: searchOf(view) })
    : addScreen(account, { suffixes: picked.domains });
}

export function PickerScreen(props: { account: string; view: View }) {
  const { account } = props;
  const { lens } = useDeps();
  const { route } = useLocation();
  const view = withoutPicks(props.view);
  const picked = pickedOf(props.view);
  const page = lens.read(lensPath(account, apiQuery(view))).value;
  const rows: readonly SenderRow[] =
    page.status === "ok" && isRowsPage(page.answer) && page.answer.dataset === "senders"
      ? page.answer.rows
      : [];
  const total =
    page.status === "ok" && isRowsPage(page.answer) ? page.answer.total.count : undefined;
  const selecting = picked.all || picked.domains.length > 0;
  const go = (next: Picked) => route(pickerHref(account, view, next), true);
  const restrict = () => openAdd(restrictHref(account, view, picked), "pick");

  // Escape clears the selection before it closes anything, and r restricts the selection as one rule
  // (docs/UI.md section 13).
  useEffect(() => {
    const onKey = (event: KeyboardEvent) => {
      if (!selecting || typing(event) || event.ctrlKey || event.metaKey || event.altKey) {
        return;
      }
      if (event.key === "Escape") {
        route(pickerHref(account, view, { all: false, domains: [] }), true);
      } else if (event.key === "r") {
        openAdd(restrictHref(account, view, picked), "pick");
      }
    };
    document.addEventListener("keydown", onKey);
    return () => document.removeEventListener("keydown", onKey);
  });

  const selection: Selection<SenderRow> = {
    selected: (row) => !restricts(row) && (picked.all || picked.domains.includes(row.domain)),
    refused: (row) =>
      restricts(row)
        ? `Already restricted by ${row.restricted_by ?? "its stored class"}`
        : undefined,
    label: (row) => `Select ${row.domain}`,
    set: (changed, on) => {
      const shown = picked.all
        ? rows.filter((r) => !restricts(r)).map((r) => r.domain)
        : picked.domains;
      const names = changed.map((r) => r.domain);
      const domains = on
        ? [...shown, ...names.filter((d) => !shown.includes(d))]
        : shown.filter((d) => !names.includes(d));
      go({ all: false, domains });
    },
    all: () => go({ all: true }),
  };

  const selectedRows = rows.filter((r) => selection.selected(r));
  const messages = selectedRows.reduce((sum, r) => sum + r.message_count, 0);
  const search = searchOf(view);
  return (
    <section aria-labelledby="picker-title" class="picker">
      <h1 id="picker-title">Restrict senders from the index</h1>
      <SearchBox
        key={search}
        label="Search senders by domain"
        value={search}
        target={(text) => pickerHref(account, withSearch(view, text), { all: false, domains: [] })}
      />
      <p class="visually-hidden" role="status">
        {total === undefined || search === ""
          ? null
          : total === 0
            ? `No senders match '${search}'`
            : `${counted(total, "sender")} match '${search}'`}
      </p>
      <Lens
        account={account}
        name="Senders"
        view={view}
        screen="policy/pick"
        rowsName="senders"
        columns={senderColumns(account)}
        rows={(p: RowsPage) => (p.dataset === "senders" ? p.rows : undefined)}
        rowKey={(row: SenderRow) => row.domain}
        panelOpen={selecting}
        selection={selection}
      />
      {total === 0 && search !== "" ? <p>No senders match &apos;{search}&apos;</p> : null}
      {selecting ? (
        <div class="selection-bar">
          <p role="status" aria-live="polite">
            <span aria-hidden="true">
              {picked.all
                ? `all ${count(total ?? 0)} matching senders`
                : `${counted(selectedRows.length, "sender")} · ${counted(messages, "message")} selected`}
            </span>
            <span class="visually-hidden">
              {picked.all
                ? `Selected all ${count(total ?? 0)} matching senders`
                : `${counted(selectedRows.length, "sender")}, ${counted(messages, "message")} selected`}
            </span>
          </p>
          <button type="button" class="primary" onClick={restrict}>
            Restrict as one rule
          </button>
        </div>
      ) : null}
      <p>
        <a href={policyHome(account)}>Back to the policy</a>
      </p>
    </section>
  );
}
