# 0102. Every change to the policy is appended to a history no runtime role can rewrite

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [O2](../../../USE_CASES.md#o2--observable), [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

The policy lives as rows in `policy_rules`, and every change to it is made through the UI
([ADR-0004](../classification/0004-sender-list-decides.md),
[ADR-0084](./0084-ui-writes-decisions-and-account-setup.md)). Each row records who created it and
when, and nothing else of its past. Once the UI adds, edits and removes rules, an edit overwrites
its row and a removal deletes it.

A removal, or an edit that drops a domain, lifts a restriction. The sender's messages go back to
pending scan through the delisting transition
([ADR-0037](../redaction/0037-delisting-transition.md)), and once scanned, a body with no code or
link in it can be released to the agent. The audit log records each body served
([ADR-0016](../data/0016-schema.md)), but not why its sender became readable. Without a record of
policy changes, "when did this sender stop being restricted, and who did it" has no answer, which
is the question [O2](../../../USE_CASES.md#o2--observable) requires the system to answer for any
past window.

## Decision

- **Every write to `policy_rules` appends one row to `policy_changes` in the same transaction.**
  The row carries the time, the identity that made the change by
  [ADR-0084](./0084-ui-writes-decisions-and-account-setup.md)'s rule, the action, the rule's
  identifier, the rule's account or none for a base rule, and the rule's domain suffixes before and
  after. The actions are a rule added, a rule edited, a rule lifted, which is removed, and a rule
  inserted by confirming a candidate. A change that fails leaves neither row.
- **No runtime role may update or delete a row of `policy_changes`**, as on the audit log. Evidence
  of who lifted a restriction survives a compromise of the UI that made the change.
- **The UI shows the history** as a dataset on the policy screen, the base policy's changes and the
  account's own, and each rule's changes on its own screen ([docs/UI.md](../../UI.md#87-policy)).

## Alternatives considered

These were put to the operator with the cost of each, and the operator chose the full history.

- **Each rule records only its last edit, its time and identity.** For it, two columns on
  `policy_rules` and no new table. Against it, a removed rule leaves no trace, and the removal is
  the change that lifts a restriction.
- **Nothing beyond the created time and creator.** For it, today's schema. Against it, edits and
  removals leave no trace, so the question above has no answer for any window.

## Consequences

- The UI's grant gains insert on `policy_changes`, and only insert
  ([ADR-0084](./0084-ui-writes-decisions-and-account-setup.md)).
- Confirming a candidate inserts its rule and its history row in the one transaction that sets the
  candidate's status.
- Importing a policy file writes one history row for each rule the import adds, edits or lifts, in
  the import's transaction, however the import is designed.
- The history grows with every change, and like the audit log nothing in the running system trims
  it.
- Assumptions about other components. Only the UI writes `policy_rules`, so only the UI writes the
  history. A change written to `policy_rules` outside the UI, directly in the database, leaves no
  history row, and nothing detects it. Row-level security confines the reads of an account's
  history rows to its account, with the base policy's rows readable by every account. Today's
  row-level security on `policy_rules` lets no role write a base rule, so how the UI writes a base
  rule and its history row is an open decision of the unit that builds policy management
  ([ROADMAP.md](../../../ROADMAP.md#open-decisions)).
- The history records changes and drives nothing. Removed rules still reach the delisting
  transition by their effect, as [ADR-0037](../redaction/0037-delisting-transition.md) decides, and
  never by reading this table.
- The same-transaction rule and the append-only grant are controls. Their violation injections are
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
