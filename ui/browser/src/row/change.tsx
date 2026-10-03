// The policy change row of docs/UI.md section 7.2, one row of the policy history (ADR-0102), as the
// columns a rows table draws. The rule identifier links to the rule while it exists, the scope is a
// badge, and the change is shown as suffix chips, each marked by its sign as well as its color. A lift,
// and an edit that removed suffixes, end with Restore.
import type { ChangeRow } from "../app/api.ts";
import { local, utc } from "../app/format.ts";
import { changeAction } from "../app/wording.ts";
import type { Column } from "../lens/table.tsx";
import { addScreen, ruleScreen, suffixChange } from "../screens/policywrites.tsx";
import { Badge, Worded } from "./badge.tsx";

// removedBy are the suffixes a change removed, every suffix of a lift and those an edit took away.
export function removedBy(row: ChangeRow): string[] {
  if (row.action === "lifted") {
    return row.suffixes_before;
  }
  return row.action === "edited"
    ? suffixChange(row.suffixes_before, row.suffixes_after).removed
    : [];
}

// restoreHref is where a row's Restore leads. With the rule gone it opens Add a rule filled with the
// removed suffixes, the row's scope and its identifier. With the rule still held it opens Edit domains
// with the suffixes added (docs/UI.md section 8.7). account is the account in view, undefined on the
// base policy's installation screens.
export function restoreHref(row: ChangeRow, account: string | undefined): string | undefined {
  const removed = removedBy(row);
  if (removed.length === 0) {
    return undefined;
  }
  const scope = row.account === null ? "base" : "account";
  return row.rule_exists
    ? ruleScreen(account, scope, row.rule_id, removed)
    : addScreen(account, { suffixes: removed, scope, id: row.rule_id });
}

// SuffixChips are a change's suffixes, +domain for one an edit added, −domain for one an edit or a lift
// removed, and the rule's suffixes unmarked for a change that created the rule.
export function SuffixChips(props: { row: ChangeRow }) {
  const { row } = props;
  if (row.action === "added" || row.action === "confirmed") {
    return (
      <>
        {row.suffixes_after.map((s, i) => (
          <span key={i} class="suffix-chip mono">
            {s}
          </span>
        ))}
      </>
    );
  }
  const { added, removed } = suffixChange(row.suffixes_before, row.suffixes_after);
  return (
    <>
      {added.map((s, i) => (
        <span key={`+${i}`} class="suffix-chip mono" data-change="added">
          +{s}
        </span>
      ))}
      {removed.map((s, i) => (
        <span key={`-${i}`} class="suffix-chip mono" data-change="removed">
          −{s}
        </span>
      ))}
    </>
  );
}

// changeColumns are the policy change row's columns, for the account in view or for the base policy's
// installation screens when account is undefined.
export function changeColumns(account: string | undefined): Column<ChangeRow>[] {
  return [
    {
      key: "ts",
      header: "time",
      width: 136,
      cell: (row) => utc(row.ts),
      title: (row) => local(row.ts),
    },
    {
      key: "actor",
      header: "identity",
      width: 120,
      cell: (row) => <span class="mono">{row.actor}</span>,
      title: (row) => row.actor,
    },
    {
      key: "action",
      header: "action",
      width: 160,
      cell: (row) => <Worded table={changeAction} stored={row.action} />,
    },
    {
      key: "rule_id",
      header: "rule",
      width: 220,
      cell: (row) =>
        row.rule_exists ? (
          <a
            class="mono"
            href={ruleScreen(account, row.account === null ? "base" : "account", row.rule_id)}
          >
            {row.rule_id}
          </a>
        ) : (
          <span class="mono">{row.rule_id}</span>
        ),
      title: (row) => row.rule_id,
    },
    {
      key: "scope",
      header: "scope",
      width: 120,
      cell: (row) => <Badge tone="muted" text={row.account ?? "base"} />,
    },
    {
      key: "change",
      header: "change",
      width: 260,
      cell: (row) => <SuffixChips row={row} />,
      title: (row) => [...row.suffixes_before, "→", ...row.suffixes_after].join(" "),
    },
    {
      key: "restore",
      header: "",
      width: 88,
      cell: (row) => {
        const target = restoreHref(row, account);
        return target === undefined ? null : <a href={target}>Restore</a>;
      },
    },
  ];
}
