// The policy screen of docs/UI.md section 8.7, the rules dataset over the account's policy, the base
// rules first, marked base, then the account's own, each with what it matches among the account's
// senders. Above the table sit the actions and the search over identifiers and suffixes, and below it,
// with no rule of the account's own, the line saying the base rules apply to every account. Every count
// is the account in view's alone. Policy is the one screen whose rows are not all the account's, as
// section 5 states.
import { useState } from "preact/hooks";
import { useLocation } from "preact-iso";
import { isFigures, lensPath, policyApi, type RowsPage, type RuleRow } from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { count, local, utc } from "../app/format.ts";
import { apiQuery, href, type View } from "../app/url.ts";
import { Lens, summaryView } from "../lens/lens.tsx";
import type { Column } from "../lens/table.tsx";
import { Badge } from "../row/badge.tsx";
import { addScreen, OutcomeRegion, policyHome, ruleScreen } from "./policywrites.tsx";

// indexUpdatedHover is what "index updated" means, said on its hover (section 8.7).
export const indexUpdatedHover =
  "Bodies are denied from each process's next policy reload. This counts the senders whose stored class reads restricted.";

// ruleColumns are a rule row's columns. The identifier links to the rule in its scope.
export function ruleColumns(account: string): Column<RuleRow>[] {
  return [
    {
      key: "rule_id",
      header: "rule",
      width: 220,
      cell: (row) => (
        <a class="mono" href={ruleScreen(account, row.scope, row.rule_id)}>
          {row.rule_id}
        </a>
      ),
      title: (row) => row.rule_id,
    },
    {
      key: "scope",
      header: "scope",
      width: 120,
      cell: (row) => <Badge tone="muted" text={row.scope === "base" ? "base" : account} />,
    },
    {
      key: "suffixes",
      header: "domain suffixes",
      width: 240,
      cell: (row) => <Suffixes suffixes={row.suffixes} />,
      title: (row) => row.suffixes.join(", "),
    },
    {
      key: "source",
      header: "source",
      width: 140,
      cell: (row) => row.source,
      title: (row) => row.source,
    },
    {
      key: "created_at",
      header: "created",
      width: 200,
      cell: (row) => (
        <>
          {utc(row.created_at)} · <span class="mono">{row.created_by}</span>
        </>
      ),
      title: (row) => `${local(row.created_at)} · ${row.created_by}`,
    },
    {
      key: "senders",
      header: "senders",
      width: 80,
      numeric: true,
      cell: (row) => count(row.senders),
    },
    {
      key: "restricted",
      header: "index",
      width: 160,
      cell: (row) => `index updated, ${count(row.restricted)} of ${count(row.senders)}`,
      title: () => indexUpdatedHover,
    },
  ];
}

// Suffixes are a rule's domain suffixes as chips, each a text child.
export function Suffixes(props: { suffixes: readonly string[] }) {
  return (
    <>
      {props.suffixes.map((s, i) => (
        <span key={i} class="suffix-chip mono">
          {s}
        </span>
      ))}
    </>
  );
}

// searchOf is the view's search filter, empty when it has none.
export function searchOf(view: View): string {
  return view.filters.find((f) => f.dimension === "search")?.raw ?? "";
}

// withSearch is the view searched for text, on its first page, with no search when text is empty.
export function withSearch(view: View, text: string): View {
  const kept = view.filters.filter((f) => f.dimension !== "search");
  return {
    ...view,
    page: view.page === undefined ? undefined : "1",
    filters: text === "" ? kept : [...kept, { dimension: "search", raw: text }],
  };
}

// SearchBox is a search over a list, which submits the address its search builds.
export function SearchBox(props: {
  label: string;
  value: string;
  target: (text: string) => string;
}) {
  const { route } = useLocation();
  const [text, setText] = useState(props.value);
  return (
    <form
      class="search"
      role="search"
      onSubmit={(event) => {
        event.preventDefault();
        route(props.target(text.trim()));
      }}
    >
      <label>
        {props.label}{" "}
        <input type="search" value={text} onInput={(event) => setText(event.currentTarget.value)} />
      </label>
      <button type="submit">Search</button>
    </form>
  );
}

// PolicyActions are the actions above the table, Add a rule, Restrict senders from the index and
// History, then Import and Export, which act on the account's own rules. Export is a download of the
// file the server names, which the router leaves to the browser.
function PolicyActions(props: { account: string }) {
  const home = policyHome(props.account);
  return (
    <nav class="policy-actions" aria-label="Policy actions">
      <a class="primary" href={addScreen(props.account)}>
        Add a rule
      </a>
      <a href={`${home}/pick`}>Restrict senders from the index</a>
      <a href={`${home}/history`}>History</a>
      <span class="policy-file">
        <a href={`${home}/import`}>Import</a>
        <a href={`${policyApi(props.account)}/export`} download="">
          Export
        </a>
      </span>
    </nav>
  );
}

export function PolicyScreen(props: { account: string; view: View; panelOpen: boolean }) {
  const { account, view } = props;
  return (
    <section aria-labelledby="policy-title" class="policy">
      <h1 id="policy-title">Policy</h1>
      <OutcomeRegion account={account} />
      <PolicyActions account={account} />
      <SearchBox
        key={searchOf(view)}
        label="Search rules by identifier or domain"
        value={searchOf(view)}
        target={(text) => href(account, withSearch(view, text), "policy")}
      />
      <Lens
        account={account}
        name="Policy"
        view={view}
        screen="policy"
        rowsName="rules"
        columns={ruleColumns(account)}
        rows={(page: RowsPage) => (page.dataset === "rules" ? page.rows : undefined)}
        open={(row: RuleRow) => ruleScreen(account, row.scope, row.rule_id)}
        rowKey={(row: RuleRow) => `${row.scope}:${row.rule_id}`}
        panelOpen={props.panelOpen}
      />
      <NoOwnRules account={account} view={view} />
    </section>
  );
}

// NoOwnRules is the account's part of the table when it holds no rule of its own (section 8.7). It
// reads the summary the strip shows, whose own figure counts the account's rules.
function NoOwnRules(props: { account: string; view: View }) {
  const { lens } = useDeps();
  const state = lens.read(lensPath(props.account, apiQuery(summaryView(props.view)))).value;
  if (state.status !== "ok" || !isFigures(state.answer)) {
    return null;
  }
  const own = state.answer.figures.find((f) => f.key === "own");
  if (own === undefined || own.value !== 0) {
    return null;
  }
  return (
    <p class="muted">
      No rules for {props.account} alone. The base rules above apply to every account.
    </p>
  );
}
