// The base policy's installation screen of docs/UI.md section 8.14, the rules every account inherits,
// readable and writable before any account exists. It shows nothing of any account but its identifier,
// so it carries no count of senders or messages. Its reads are the base policy endpoints, which read in
// a base-policy transaction (ADR-0112).
import { useLocation } from "preact-iso";
import { basePolicyPath, policyApi, type BaseRule } from "../app/api.ts";
import { useDeps } from "../app/deps.ts";
import { count, local, utc } from "../app/format.ts";
import { Region } from "../app/region.tsx";
import { RowsTable, type Column } from "../lens/table.tsx";
import { SearchBox, Suffixes } from "./policy.tsx";
import { addScreen, OutcomeRegion, policyHome, ruleScreen } from "./policywrites.tsx";

// baseColumns are a base rule's columns, each row opening its rule.
function baseColumns(): Column<BaseRule>[] {
  return [
    {
      key: "rule_id",
      header: "rule",
      width: 220,
      cell: (row) => (
        <a class="mono" href={ruleScreen(undefined, "base", row.rule_id)}>
          {row.rule_id}
        </a>
      ),
      title: (row) => row.rule_id,
    },
    {
      key: "suffixes",
      header: "domain suffixes",
      width: 280,
      cell: (row) => <Suffixes suffixes={row.suffixes} />,
      title: (row) => row.suffixes.join(", "),
    },
    { key: "source", header: "source", width: 140, cell: (row) => row.source },
    {
      key: "created_at",
      header: "created",
      width: 220,
      cell: (row) => (
        <>
          {utc(row.created_at)} · <span class="mono">{row.created_by}</span>
        </>
      ),
      title: (row) => `${local(row.created_at)} · ${row.created_by}`,
    },
  ];
}

export function BasePolicyScreen() {
  const { basePolicy } = useDeps();
  const { query } = useLocation();
  const search = query.search ?? "";
  const path = basePolicyPath(search);
  const home = policyHome(undefined);
  return (
    <section aria-labelledby="base-title" class="policy">
      <h1 id="base-title">Base policy</h1>
      <Region
        name="base policy"
        state={basePolicy.read(path)}
        shape="table"
        retry={() => void basePolicy.retry(path)}
      >
        {(answer) => (
          <>
            <p>
              These rules restrict senders in every account, those connected later included.{" "}
              {answer.accounts.length === 0 ? (
                "No account is connected yet."
              ) : (
                <>
                  The accounts today:{" "}
                  {answer.accounts.map((a, i) => (
                    <span key={a}>
                      {i === 0 ? null : ", "}
                      <a class="mono" href={policyHome(a)}>
                        {a}
                      </a>
                    </span>
                  ))}
                  .
                </>
              )}
            </p>
            <section class="strip" aria-label="Summary">
              <span class="figure">
                <span class="figure-value">{count(answer.total)}</span>
                <span class="label">base rules</span>
              </span>
              <a class="figure" href={`${home}/history`}>
                <span class="figure-value">
                  {answer.latest === null ? "none" : utc(answer.latest)}
                </span>
                <span class="label">latest change</span>
              </a>
            </section>
            <OutcomeRegion account={undefined} />
            <nav class="policy-actions" aria-label="Base policy actions">
              <a class="primary" href={addScreen(undefined)}>
                Add a base rule
              </a>
              <a href={`${home}/history`}>History</a>
              <span class="policy-file">
                <a href={`${home}/import`}>Import</a>
                <a href={`${policyApi(undefined)}/export`} download="">
                  Export
                </a>
              </span>
            </nav>
            <SearchBox
              key={search}
              label="Search base rules by identifier or domain"
              value={search}
              target={(text) =>
                text === "" ? home : `${home}?${new URLSearchParams({ search: text }).toString()}`
              }
            />
            {answer.total === 0 ? (
              <p>
                No base rules yet. A base rule names one institution&apos;s mail domains and
                restricts its senders in every account.{" "}
                <a href={addScreen(undefined)}>Add a base rule</a>
              </p>
            ) : (
              <RowsTable
                caption={`Base rules, ${count(answer.rules.length)} of ${count(answer.total)}`}
                rowsName="base rules"
                columns={baseColumns()}
                rows={answer.rules}
                rowKey={(row) => row.rule_id}
                open={(row) => ruleScreen(undefined, "base", row.rule_id)}
                page={1}
                pages={1}
                pageHref={() => home}
              />
            )}
          </>
        )}
      </Region>
    </section>
  );
}
