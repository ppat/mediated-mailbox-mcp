# 0056. The UI is organized around the operator's work

**Status:** Accepted ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer),
[G3](../../../USE_CASES.md#g3--reorganization)

## Context

[ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md) fixes what the UI is
(read-mostly, two decision verbs, a rollback request, OAuth client setup, account setup and policy
management, a separate deployment, never a body) and lists its views, but not how those views are organized or
how the UI grows past them. The operator asked for a UI structured to take on views not yet described, with drill-down,
zoom in and out, and aggregate views over plans and any other element as if analyzing a data set or
a proposed change. Three directions were sketched and compared on one canvas, differing only in
what the UI is organized around. The shared foundation under all three is stated in
[docs/UI.md](../../UI.md). Every view is a lens over an account-scoped dataset, viewed on one zoom
ladder.

## Decision

- **The home is organized around the operator's work, not the data sources.** What awaits a
  decision first (plans in DRAFT, candidates pending), then what is worth a look (attention items
  derived from the measurements the design records), then the account's system state, with a live
  strip of running work.
- **The plan reviewer is shaped like code review.** A rail of sections working like a diff's file
  list, the zoom ladder inside the plan (flows, then a flow by sender, then the operations), and
  an approval footer that restates in one sentence with the numbers what approving does.
- **Every analytical view is the zoom ladder over its dataset**, with ADR-0084's views as the
  default groupings. Nothing analytical is a bespoke page.
- **Batch work is visible and inspectable.** A live view of every workload of
  [ADR-0022](../operability/0022-four-workloads.md), and a failed run drilled as a dataset whose
  rows are its failures.
- **Every view is per account.** The account is chosen explicitly, is always visible, rides in
  the URL, and nothing aggregates across accounts.
- **What belongs to no account has installation screens of its own.** The installation's OAuth
  clients, the base policy, the list of accounts with connecting one, and the first run, before
  any account exists, sit on screens whose URL names no account. Of what is stored, an installation
  screen shows only each OAuth client's name, provider, client identifier and project ID, each
  account's identifier, provider and the client it connects through, and the base policy's rules
  and their history, the three kinds of record that belong to no account's own rows
  ([ADR-0016](../data/0016-schema.md), [ADR-0091](../data/0091-accounts-listed-apart-from-their-state.md),
  [ADR-0106](../provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md),
  [ADR-0004](../classification/0004-sender-list-decides.md),
  [ADR-0102](../mutation/0102-policy-changes-recorded-in-an-append-only-history.md)).
  It reads and shows no stored account's state, counts or rows, so the per-account rule above holds
  for every view of an account's data and nothing aggregates across accounts. The one account it
  handles values of is the one being connected, whose rows it writes alone.
- **Message-derived text renders as text only.** Subjects, display names, addresses, labels, and
  reasons are attacker-written text. No rendering path interprets them as markup.
- ADR-0084's constraints are unchanged. Two decision verbs, a rollback request, OAuth client setup,
  account setup and policy management by database grant, code that never opens a stored credential, never a body. The shape adds
  legibility, not surface. The UI stays boring in ADR-0084's sense.

## Alternatives considered

- **A console organized around the data sources**, one page per ADR-0084 view. Its case was
  familiar, literal, nothing clever to get wrong, the cheapest first version. Not chosen because
  drill-down is built per page and drifts, growth is linear in pages, and the plan page becomes a
  form with a table rather than a review.
- **An explorer organized around one query model**, dataset by grouping by level by filters, with
  the six views as saved presets and verbs attached to row types. Its case was one mechanism for
  every view, present and future. Calendar, a second account, and audit by actor all arrive as
  registry entries, and analysis is first-class everywhere. Not chosen as the organizing shape
  because generic surfaces go bland and strain on the one screen that matters (a plan's label
  operations have no home in a ladder), and the read API becomes a small query engine that must
  stay bounded by a registry. Its engine is kept underneath the
  chosen shape ([ADR-0057](./0057-one-dataset-endpoint-behind-a-registry.md)).

The operator chose the third direction on seeing all three, on the grounds that it serves the use
case best.

## Consequences

- The plan reviewer groups a plan's operations by flow, sender, and sensitivity. That is a query
  over rows, which the plan JSON of [ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)
  does not directly afford on its own. The engine writes the operations as rows with their flows
  when a plan is saved ([ADR-0016](../data/0016-schema.md), [ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)).
- The jobs and failed-run screens read only recorded state, the bound
  [ADR-0034](./0034-system-status-operation.md) sets for the client surface. Each workload
  records its runs, timeline events, and per-item failures ([ADR-0022](./0022-four-workloads.md)).
- The attention rules behind "worth a look" are the UI's own design, defined in
  [docs/UI.md section 8.1](../../UI.md#81-home). They are derived at read time from thresholds in
  the UI's configuration and carry no state and no verb.
- Assumptions about other components: analysis views read through the dataset endpoint of
  [ADR-0057](./0057-one-dataset-endpoint-behind-a-registry.md). The plan's status set includes
  the rejected and refused states the reviewer displays ([ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)).
  The decision verbs, OAuth client setup, account setup and policy management remain the only
  writes ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)).
- The rendering rule and the per-account rule are controls. Their violation injections are
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
