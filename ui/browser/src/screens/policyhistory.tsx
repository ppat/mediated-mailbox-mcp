// The policy history (docs/UI.md sections 8.7 and 8.14). From an account it is the policy_changes
// dataset, the base policy's changes and the account's own, newest first, at level 3 by default,
// groupable by action, scope and identity. On the base policy's installation screen it is the base
// policy's changes alone, read from its own endpoint over the same ranges. A lifted rule's identifier
// links nowhere, and its row carries Restore.
import { useLocation } from "preact-iso";
import { baseHistoryPath, type ChangeRow, type RowsPage } from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { Region } from "../app/region.tsx";
import type { View } from "../app/url.ts";
import { Lens } from "../lens/lens.tsx";
import { customRange, presets } from "../lens/range.tsx";
import { RowsTable } from "../lens/table.tsx";
import { changeColumns } from "../row/change.tsx";
import { policyHome } from "./policywrites.tsx";

export function PolicyHistoryScreen(props: { account: string; view: View }) {
  const { account } = props;
  return (
    <section aria-labelledby="history-title" class="policy-history">
      <h1 id="history-title">Policy history</h1>
      <p>
        <a href={policyHome(account)}>Back to the policy</a>
      </p>
      <Lens
        account={account}
        name="Policy history"
        view={props.view}
        screen="policy/history"
        rowsName="changes"
        columns={changeColumns(account)}
        rows={(page: RowsPage) => (page.dataset === "policy_changes" ? page.rows : undefined)}
        rowKey={(row: ChangeRow) => row.id}
        panelOpen={false}
      />
    </section>
  );
}

// defaultBaseRange is the base policy's history's range when its address names none, the
// policy_changes dataset's default (section 5).
export const defaultBaseRange = "30d";

// BaseHistoryScreen is the base policy's history on its installation screen, with the ranges of
// section 5 as links, since no dataset view holds its state.
export function BaseHistoryScreen() {
  const { baseHistory } = useDeps();
  const { query } = useLocation();
  const range = query.range ?? defaultBaseRange;
  const path = baseHistoryPath(range);
  const custom = customRange(range);
  return (
    <section aria-labelledby="history-title" class="policy-history">
      <h1 id="history-title">Base policy history</h1>
      <p>
        <a href={policyHome(undefined)}>Back to the base policy</a>
      </p>
      <div class="range" role="group" aria-label="Time range, UTC">
        {presets.map((p) => (
          <a
            key={p.range}
            class="preset"
            href={`/setup/policy/history?range=${p.range}`}
            aria-current={range === p.range ? "true" : undefined}
          >
            {p.wording}
          </a>
        ))}
        {custom === undefined ? null : (
          <span class="preset" aria-current="true">
            {custom.from} to {custom.to}
          </span>
        )}
      </div>
      <Region
        name="history"
        state={baseHistory.read(path)}
        shape="table"
        retry={() => void baseHistory.retry(path)}
      >
        {(answer) => (
          <RowsTable
            caption="The base policy's changes, newest first"
            rowsName="changes"
            columns={changeColumns(undefined)}
            rows={answer.rows}
            rowKey={(row) => row.id}
            page={1}
            pages={1}
            pageHref={() => ""}
          />
        )}
      </Region>
    </section>
  );
}
