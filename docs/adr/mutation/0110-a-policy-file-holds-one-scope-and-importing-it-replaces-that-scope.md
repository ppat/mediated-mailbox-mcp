# 0110. A policy file holds one scope's rules and imports into any scope, making that scope's stored rules equal to the file after a preview and one confirmation of every restriction it lifts, and a rule's identifier is unique within its scope

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C4](../../../USE_CASES.md#c4--the-sensitive-sender-list-keeps-pace), [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

The policy lives in the database, and a file form exists for import and export
([ADR-0004](../classification/0004-sender-list-decides.md)). It has two scopes, the base rules
every account inherits and each account's own overlay rules
([ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md)). Every view in
the UI is per account and nothing aggregates across accounts
([ADR-0056](../operability/0056-ui-organized-around-the-operators-work.md)). Removing a rule lifts a
restriction, which can release a body at once, and a released body cannot be recalled
([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md),
[ADR-0037](../redaction/0037-delisting-transition.md)). Every write to the rules appends its row
to the policy history in the same transaction
([ADR-0102](./0102-policy-changes-recorded-in-an-append-only-history.md)). A rule's identifier was
unique across every rule of every scope ([ADR-0016](../data/0016-schema.md)).

The operator ruled on 2026-10-02 on what one file covers and what an import does to stored rules
the file leaves out, and then that an exported file is generic, importable as the base policy or
into any account's policy.

## Decision

- **One scope's rules are exported to a file, and a file imports into any scope.** The base
  policy's export holds the base rules, and an account's export holds that account's overlay
  rules. The file is ADR-0004's form and names no scope, so a file exported from one scope imports
  into the base policy or into any account. A rule carries its identifier, its domain suffixes and
  its class, and nothing else.
- **A rule's identifier is unique within its scope**, the base policy or one account's overlay.
  The base policy and an account may each hold a rule of one identifier, and so may two accounts,
  so a file's identifiers never collide with another scope's rules. On an account's screens an
  identifier names the account's own rule first and the base rule otherwise, and the base rule of
  an identifier the account also holds has an address of its own.
- **An import makes the scope's stored rules equal to the file.** A rule in the file and not stored
  is added, a rule stored and not in the file is lifted, and a rule in both whose suffixes differ is
  edited to the file's.
- **The operator sees every change before it is made, and confirms every lift once.** The UI shows
  the difference first, lifts first. An import that only adds restriction applies with no
  confirmation. An import that lifts anything needs one confirmation covering every lift, with
  {k} the count it lifts. For the base scope the operator types `lift {k}`, as a base lift is
  typed. The server checks the confirmation's count, not only the screen.
- **An import is one transaction**, each change with its history row, so an import that fails
  leaves neither rules nor history. An import whose scope's stored rules changed since its preview
  was computed is refused.
- **A file is checked whole before anything is shown.** A file not of this form, a rule that fails
  a write's checks, or two rules with one identifier refuses the whole file.

## Alternatives considered

- **An import that only adds.** For it, a file can never lift a restriction, and lifting stays on
  the rule screens alone. Against it, a file cannot prune the policy, and the operator ruled that
  the file replaces the scope's rules.
- **A preview where each removal is opt-in.** For it, nothing is lifted unless the operator ticks
  it, each through the ordinary lift dialog. Against it, the operator chose replacement with one
  confirmation of every lift.
- **One file holding the base rules and every account's overlay.** For it, one file backs up or
  moves a whole installation's policy. Against it, it would be the first place the UI handles
  several accounts' data together, which ADR-0056's per-account rule refuses, and the operator chose
  one file per scope.
- **Import and export for the base policy alone.** For it, the smallest surface, and enough to load
  the operator's policy before the first account connects. Against it, an account's overlay could be
  neither backed up nor copied to another account, and the operator chose one file per scope.
- **A file bound to the scope it was exported from.** For it, a file imported into the wrong scope
  is caught. Against it, the operator ruled that an exported file is generic, importable as the
  base policy or into any account.
- **Identifiers unique across every rule, rewritten on import when another scope holds them.** For
  it, an identifier names one rule everywhere and the schema keeps its key. Against it, a rewritten
  rule no longer matches its file, so importing the same file again lifts the rewritten rule and
  adds the file's, and copying an account's file to the base policy while the account keeps its
  rules would rewrite every one of them.

## Consequences

- The operator's policy can be imported into the base scope before any account exists, from the
  base policy's installation screen ([docs/UI.md](../../UI.md#814-base-policy)), and an account's
  rules can be copied to another account or promoted to the base policy by exporting and importing.
- `policy_rules` is keyed on its scope and identifier ([ADR-0016](../data/0016-schema.md)), and the
  policy snapshot's validation refuses a repeated identifier within one scope only, where it
  refused one across every rule ([ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md)).
  Both land together, so no stored policy ever holds an identifier in two scopes while a process
  still refuses that.
- A message's `class_rule_id` names an identifier and not its scope. When an account and the base
  policy both hold the identifier, with suffixes that may differ, the message records which
  identifier matched but not which scope's rule, and the UI lists both rather than guess.
- A stale or partial file lifts every rule it leaves out once confirmed. The typed count on a base
  import, the account's numbers on an account's import, and Put it back after the import are what
  stand against that.
- A file round trip loses each rule's source, created time and history, which stay in the database.
- A rule an import lifts reaches the delisting transition as any lift does (ADR-0037).
- Assumptions about other components. The identity on each history row follows
  [ADR-0084](./0084-ui-writes-decisions-and-account-setup.md)'s rule, as for every policy write. The
  base scope is read and written with no account named, in a transaction of the base policy's own
  ([ADR-0112](../data/0112-the-base-policy-is-written-and-read-in-a-transaction-of-its-own.md)).
