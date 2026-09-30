// The URL grammar of docs/UI.md section 5, in one module (ADR-0063). A dataset view's whole state rides
// in its query string, and these pure functions of that string and the generated descriptor table read
// it, fill the dataset's defaults, and write it back. Every link to a view is built by serialize, and
// the route effect in router.tsx replaces a URL whose canonical form differs, by comparing strings.
//
// Nothing here checks a value against the registry. The dataset endpoint refuses what the registry does
// not declare, and a present parameter is written back as it was read, so a wrong value reaches the
// server and its refusal is shown rather than silently corrected.
import { descriptors } from "../generated/descriptors.ts";

export type DatasetName = keyof typeof descriptors;
type Descriptor = (typeof descriptors)[DatasetName];

// Defaults is a descriptor's defaults with every optional part named, which each dataset's literal type
// leaves out where the dataset has none.
type Defaults = {
  level: number;
  group?: string;
  range?: string;
  sort: string;
  filters?: Readonly<Record<string, readonly string[]>>;
};

function defaults(dataset: DatasetName): Defaults {
  return descriptors[dataset].default;
}

// Filter is one dimension filter as the URL writes it. An empty value is a default filter the operator
// removed, which the router keeps so canonicalization does not restore it, and which no request sends.
export type Filter = { dimension: string; raw: string };

// View is one dataset view. Level, group, range, sort and page are the strings the URL carries, and
// filters keep the order they were applied in, which is the breadcrumb's order.
export type View = {
  dataset: DatasetName;
  level: string | undefined;
  group: string | undefined;
  range: string | undefined;
  sort: string | undefined;
  page: string | undefined;
  filters: Filter[];
};

const common = ["level", "group", "range", "sort", "page"] as const;

export function isDataset(name: string): name is DatasetName {
  return Object.hasOwn(descriptors, name);
}

export function descriptor(dataset: DatasetName): Descriptor {
  return descriptors[dataset];
}

// parse reads a query string into a view, taking every parameter it does not know as a filter.
export function parse(dataset: DatasetName, search: string): View {
  const view: View = {
    dataset,
    level: undefined,
    group: undefined,
    range: undefined,
    sort: undefined,
    page: undefined,
    filters: [],
  };
  for (const [name, value] of new URLSearchParams(search)) {
    switch (name) {
      case "level":
      case "group":
      case "range":
      case "sort":
      case "page":
        view[name] ??= value;
        break;
      default:
        view.filters.push({ dimension: name, raw: value });
    }
  }
  return view;
}

// canonicalize fills every parameter the view leaves out from the dataset's defaults, and changes
// nothing the view carries. The default filters come first, since they apply before any the operator
// adds. Page is written at level 3, the one level that pages.
export function canonicalize(view: View): View {
  const d = defaults(view.dataset);
  const level = view.level ?? String(d.level);
  const filled: Filter[] = [];
  for (const [dimension, values] of Object.entries(d.filters ?? {})) {
    if (!view.filters.some((f) => f.dimension === dimension)) {
      filled.push({ dimension, raw: values.join(",") });
    }
  }
  return {
    dataset: view.dataset,
    level,
    group: view.group ?? d.group,
    range: view.range ?? d.range,
    sort: view.sort ?? d.sort,
    page: view.page ?? (level === "3" ? "1" : undefined),
    filters: [...filled, ...view.filters],
  };
}

// serialize writes a view as the URL's query string, the common parameters first in a fixed order, then
// the filters in the order they were applied.
export function serialize(view: View): string {
  const params = new URLSearchParams();
  for (const name of common) {
    const value = view[name];
    if (value !== undefined) {
      params.append(name, value);
    }
  }
  for (const f of view.filters) {
    params.append(f.dimension, f.raw);
  }
  return restore(params.toString());
}

// canonical returns the canonical query string for a dataset's query string.
export function canonical(dataset: DatasetName, search: string): string {
  return serialize(canonicalize(parse(dataset, search)));
}

// apiQuery is the dataset endpoint's query for a view. A removed default filter is left out, because an
// absent filter is no filter to the endpoint.
export function apiQuery(view: View): string {
  const params = new URLSearchParams({ dataset: view.dataset });
  for (const name of common) {
    const value = view[name];
    if (value !== undefined) {
      params.append(name, value);
    }
  }
  for (const f of view.filters) {
    if (f.raw !== "") {
      params.append(f.dimension, f.raw);
    }
  }
  return restore(params.toString());
}

// restore puts back the grammar's comma and exclamation mark, which the platform percent-encodes, so a
// URL reads as the grammar writes it and two canonical forms compare equal as strings.
function restore(query: string): string {
  return query.replaceAll("%2C", ",").replaceAll("%21", "!");
}

// chips are the filters the breadcrumb shows, the applied ones in order, with each one's index in the
// view's filters.
export function chips(view: View): { filter: Filter; index: number }[] {
  return view.filters.flatMap((filter, index) => (filter.raw === "" ? [] : [{ filter, index }]));
}

// removeChip removes the filter at index and every filter applied after it, since the breadcrumb is a
// stack (docs/UI.md section 4). A default filter removed is written empty, so it stays removed. The
// view returns to its first page, and a level 2 view left with no filter returns to level 1, since
// level 2 needs one.
export function removeChip(view: View, index: number): View {
  const removable = Object.keys(defaults(view.dataset).filters ?? {});
  const kept = view.filters.slice(0, index);
  for (const f of view.filters.slice(index)) {
    if (f.raw === "" || removable.includes(f.dimension)) {
      kept.push({ dimension: f.dimension, raw: "" });
    }
  }
  const applied = kept.filter((f) => f.raw !== "").length;
  return {
    ...view,
    level: view.level === "2" && applied === 0 ? "1" : view.level,
    page: view.page === undefined ? undefined : "1",
    filters: kept,
  };
}

// withRange sets the view's range and returns it to its first page.
export function withRange(view: View, range: string): View {
  return { ...view, range, page: view.page === undefined ? undefined : "1" };
}

// withPage sets the view's page.
export function withPage(view: View, page: number): View {
  return { ...view, page: String(page) };
}

// href is the screen path of a dataset view under an account.
export function href(account: string, view: View): string {
  return `/${encodeURIComponent(account)}/${view.dataset}?${serialize(view)}`;
}
