# Mediated Mailbox MCP — UI design

This file holds the design of the UI (directory `ui/`). It states what the UI *is* in enough depth to
build it. How it is organized, what every screen contains and does, how it renders, what it looks
like, what its read API is shaped like, what the browser framework must be able to do, and how the
code is laid out, configured, and tested. This document and the decision records it cites are the
whole reading list for the session that builds the UI. Nothing here depends on opening a mockup.
The mockup sources exist at [ui/design/](../ui/design/README.md) for a design session, and that
directory carries its own notes.

The decisions behind this design that had alternatives live as decision records, indexed at
[docs/adr/README.md](./adr/README.md), and are cited here by number. The outcome the UI serves is
[O4](../USE_CASES.md#o4--the-operator-can-see-and-steer). The rules that bound it are ADR-0084's,
via the [decision-record index](./adr/README.md). The UI carries two decisions, OAuth client setup,
account setup and policy management as its only writes, is read-mostly, and never displays a
message body. Build
state lives in [ROADMAP.md](../ROADMAP.md), never here. Vocabulary is defined in
[DESIGN.md's Glossary](../DESIGN.md#glossary), never here.

The split with [DESIGN.md](../DESIGN.md) is one of altitude. The design document holds the system.
This document holds one component's design at the same rate of change and by the same split test,
holding only what would still be true if any single reversible decision had gone the other way.
Every fact about the system this document relies on, the schema included, is stated by a record
and cited here. What is still open is listed in [section 20](#20-what-remains-open).

Two readers use this document. The session that builds the UI reads it top to bottom, in the order
of [section 2](#2-reading-order-for-the-builder). The operator reads sections 1 to 10 to check
that the screens are the ones wanted.

## 1. What the UI is for

The operator's standing duties live here. Plan review, candidate review, masking review, and
keeping accounts connected are the duties ADR-0084 names, and the operator added two more on
2026-09-10. Watching batch work as it runs
(backfill, delta sync, reorg apply, the heuristics job) and inspecting a run, failed or not, down to
its individual failures. The loop the UI is organized around is the operator's. Open it, see what
needs a decision, see what is worth a look, decide, and leave.

Every view is per account. The account is chosen explicitly, is always visible, and nothing is ever
aggregated across accounts. This is the operator's ruling, not a consequence of
[P3](../USE_CASES.md#p3--multi-account), whose isolation binds clients and credentials.
What belongs to no account, the installation's OAuth clients, the base policy, the list of
accounts and the first run before any account exists, has installation screens of its own, which
show no account's data ([section 8.10](#810-installation), ADR-0056).

The UI has no authentication of its own in the first version. A deployment may place it behind
an ingress that forwards to an authenticating proxy and passes the identity in a declared header,
and the UI's own authentication may come later (ADR-0084). TLS holds
([section 15](#15-security-of-the-ui-itself)).

The UI is desktop-first. It is designed at 1440 pixels wide with a floor of about 1280, and the
first version has no mobile layout.

## 2. Reading order for the builder

The reading list is this document plus every decision record it cites. The order below is where
to start. Any record cited in a section and not listed here is read when that section is built.

1. [USE_CASES.md](../USE_CASES.md), the fixed point and
   [O4](../USE_CASES.md#o4--the-operator-can-see-and-steer). Ten minutes. It says what the UI is
   judged on and what it must never do.
2. ADR-0084, ADR-0016, ADR-0042, ADR-0047, and ADR-0051, via the [index](./adr/README.md). The
   UI's shape and grants, the schema it reads, the languages it is written in, how queries are
   written, the environment contract.
3. This document, in order. Sections 3 to 7 are the model every screen instantiates. Section 8 is
   the screens. Sections 14 to 19 are what the code must be. Section 20 is what remains open.
4. ADR-0056, ADR-0057, ADR-0058, ADR-0059, ADR-0060, ADR-0061, ADR-0062, ADR-0063, ADR-0064, and
   ADR-0065, via the [index](./adr/README.md), for why the shape, the read API, the live surfaces,
   the palettes, the no-code-in-the-database rule, the request token, the content security policy,
   the browser framework, the browser's tests, and the contract pipeline are what they are, and
   what was rejected.
5. ADR-0020, ADR-0032, ADR-0019, ADR-0004, ADR-0093, ADR-0005, ADR-0003, ADR-0002, ADR-0034,
   ADR-0022, ADR-0025, ADR-0018, for the mechanisms the screens display, and ADR-0080, ADR-0081,
   ADR-0106, ADR-0107, ADR-0091, ADR-0097, ADR-0024, ADR-0037, ADR-0041, ADR-0102, ADR-0110, ADR-0112,
   ADR-0113 and ADR-0114 for the setups, the account settings and the policy writes. Read each when building the screen that shows it.
6. [TESTING.md](../TESTING.md), ADR-0043, ADR-0044, and the UI's rows in
   [docs/VERIFICATIONS.md](./VERIFICATIONS.md), before writing a test.
7. [ROADMAP.md's open decisions](../ROADMAP.md#open-decisions) and
   [section 20](#20-what-remains-open), before touching anything either lists.

## 3. The lens model

Every view ADR-0084 names, the two setups aside, is the same shape. An account-scoped dataset,
viewed at some aggregation level, sliced by a few dimensions, drilled to individual rows, and for
two of them a decision attached to the row in view. That shape is a **lens**. The unit of design
is the lens, not the page.

| ADR-0084 view | Dataset | Natural dimensions | Row | Decision |
| --- | --- | --- | --- | --- |
| Corpus overview | messages, senders | sender, label, sender class, scan state, time | message | none |
| Reorg plans | a plan's message operations | flow, sender, sensitivity, reason | one operation, before → after | approve or reject |
| Review queue | policy candidates | score, heuristic that fired, status | candidate with its sender statistics | confirm or dismiss |
| Masking events | masking events | rule, tier, sender, time | event | none |
| Scan gate decisions | gate decisions | decision, reason, sender, time | decision | none |
| Audit log | audit rows | action, actor, sender class, rule, time | audit row | none |

The consequence is how the UI grows. Calendar events, a second account, a policy-rule view, an
agent-activity view, job runs and their failures all have this shape. A new view is a registered
dataset plus a row renderer, not a page built from scratch. The mechanism is ADR-0057, the same move
ADR-0053 makes for the client surface.

## 4. The zoom ladder

Every lens is viewed at one of five levels, the **zoom ladder**.

| Level | Name | What it shows | On a reorg plan |
| --- | --- | --- | --- |
| L0 | Summary | the lens's named figures and a status | messages, threads, share of corpus, label operations, restricted messages inside, validation, age |
| L1 | Distribution | one dimension, chart plus table, each group clickable | operations by flow (from label → to label) |
| L2 | Cohort | one filter applied, a second dimension opened | inside one flow, by sender domain |
| L3 | Rows | a paged list of individual rows | operations with sender, masked subject, date, before → after labels, reason |
| L4 | Row detail | one row with its provenance, as a panel over the L3 list it came from | one message with both sensitivity axes and the rules that set them, scan state, audit trail, and what the plan does to it |

The navigation rules that make this analysis rather than page-hopping:

- Clicking any aggregate (a bar, a count, a group row) applies it as a filter and descends one
  level. The applied filters are a breadcrumb of chips under the top bar, in the order applied.
  The breadcrumb is a stack. Removing a chip removes it and every chip applied after it, and the
  view returns to the level that chip was applied at. Descending from L1 opens L2 grouped by the
  first groupable dimension, in the registry's order, that neither the group nor an applied filter
  names. Descending from L2, or from L1 when no such dimension is left, opens L3. A click on the
  null group applies `none`. Asking which dimension to open before descending was the alternative,
  and it would add a step to every click when the group-by control already changes it in one.
- Every state is a URL. Account, dataset, filters, level, sort, and page all ride in it, so the
  back button, a bookmark, and a link pasted into a ticket reproduce a view exactly. Nothing
  about a view lives only in memory. A URL missing a parameter is canonicalized by the router,
  which fills the dataset's defaults and replaces the URL.
- Charts are navigation devices and tables are the workhorse. A bar exists to be clicked into a
  filter. Every chart has the same data as a table beside or beneath it.
- One message-row component renders in every message-derived lens
  ([section 7](#7-the-rows-and-the-charts)), so the operator learns it once.
- L0 is always visible above the level in view, as a strip, at every level including L4 and on
  every screen that shows a dataset at L3 (the plans list, the review queue, the runs table), so
  the whole is never lost while drilling. The strip carries the as-of time, the UTC time the read
  transaction began, so a stale tab on a non-live screen is visibly stale.
- L4 is a route, rendered as a side panel over the L3 list it came from. The panel is 480 px wide,
  fixed to the right edge, scrolling internally, and the page behind it is inert until it closes.
  `Escape` navigates back to the L3 URL.

## 5. Information architecture and the URL

**Scopes**, carried in every URL:

| Scope | Rule |
| --- | --- |
| account | mandatory, the first path segment, never implicit, never `all`, on every route but the installation screens' of [section 8.10](#810-installation), which belong to no account |
| time range | `range=` with the grammar below. UTC on the wire and in the display, with the `Z` suffix, local time on hover |
| search | a filter named `search`, declared in the registry as a filter-only entry of kind text, legal at every level, on three datasets. Its value is the text searched for, taken whole, so a comma or a leading `!` in it is part of the text and not the filter grammar's. On `messages` it is a case-insensitive substring match over the masked subject through the index's trigram index, and the search box submits `/{account}/messages?level=3&search=…`. On `senders` it is a case-insensitive substring match over the domain, the sender picker's search of [section 8.7](#87-policy). On `rules` it is a case-insensitive substring match over the rule identifiers and the domain suffixes, the search box above the policy screen's table. Sender and label search on messages are the ordinary `sender` and `label` filters |

**The route table.** The entry route first, then the fixed screens, the object screens, and the
lens routes, which exist only for the five analysis datasets. The URL is the view state. The
common parameters `sort=` and `page=` apply to every route that shows rows, and `level=`,
`group=`, and the dimension filters to every lens route and to `ops` and `failures`, and `range=` to
every dataset the default-range table gives a range.

| Route | Screen |
| --- | --- |
| `/` | redirects to `/{account}` for the account last used in this browser, else the first account by identifier, else to `/setup` when no account exists |
| `/setup` | Installation ([8.10](#810-installation)) |
| `/setup/{provider}/new?step=N` and `/setup/{provider}/{client}?step=N` | OAuth client setup ([8.11](#811-oauth-client-setup)), for a new client and for the client named `{client}`, `{provider}` one that authenticates through an OAuth client, `gmail` today, `step` the step in view, and `&guide=side` the compact layout of its side window |
| `/setup/connect?client=…` | Connect an account ([8.12](#812-connect-an-account-and-re-authorize)), `client=` the client chosen to connect through |
| `/setup/policy` and `/setup/policy/{rule-id}` | Base policy and one base rule ([8.14](#814-base-policy)) |
| `/setup/policy/new?suffix=…&id=…` | Add a base rule, the panel over the base policy screen, `id=` the identifier a restore fills in ([8.14](#814-base-policy)) |
| `/setup/policy/history?…` | The base policy's history, the `policy_changes` dataset's base rows ([8.14](#814-base-policy)) |
| `/setup/policy/import` | Import the base policy from a file ([8.7](#87-policy), [8.14](#814-base-policy)) |
| `/{account}` | Home ([8.1](#81-home)) |
| `/{account}/account` | Account settings ([8.13](#813-account-settings)) |
| `/{account}/account/reauthorize?client=…` | Re-authorize the account ([8.12](#812-connect-an-account-and-re-authorize)), `client=` another client to move the account to |
| `/{account}/plans` | Plans ([8.9](#89-plans)) |
| `/{account}/plans/{plan-id}?section=…` | Plan reviewer ([8.2](#82-plan-reviewer)) |
| `/{account}/plans/{plan-id}/ops?…` | the plan's operations ladder, the `ops` dataset with `plan` as its parent, shown inside the reviewer's Flows section |
| `/{account}/plans/{plan-id}/ops/{message-id}` | one operation, the panel over the plan's L3 |
| `/{account}/plans/{plan-id}/sample?seed=S` | the sample ([8.2](#82-plan-reviewer)) |
| `/{account}/candidates?status=…` | Review queue ([8.6](#86-review-queue)) |
| `/{account}/candidates/{domain}` | one candidate |
| `/{account}/jobs` | Jobs ([8.3](#83-jobs)) |
| `/{account}/jobs/{run-id}?…` | Run ([8.4](#84-run)), whose ladder is the `failures` dataset with `run` as its parent |
| `/{account}/jobs/{run-id}/failures/{seq}` | one failure, the panel over the run's L3 |
| `/{account}/policy` and `/{account}/policy/{rule-id}?suffix=…` | Policy and one rule, the account's own rule of that identifier or else the base rule, `suffix=` opening Edit domains with those suffixes added ([8.7](#87-policy)) |
| `/{account}/policy/base/{rule-id}?suffix=…` | One base rule seen from the account, for an identifier the account also holds ([8.7](#87-policy)) |
| `/{account}/policy/new?suffix=…&scope=…&id=…` | Add a rule, the panel over the policy screen, `suffix=` or `search=` carrying the senders picked into it, and `scope=` and `id=` the scope and identifier a restore or a change of scope fills in ([8.7](#87-policy)) |
| `/{account}/policy/pick?search=…&pick=…` | Restrict senders from the index, the `senders` dataset at L3, `pick=` the selected domains, or `pick=all` for every sender the search matches ([8.7](#87-policy)) |
| `/{account}/policy/history?…` | Policy history, the `policy_changes` dataset ([8.7](#87-policy)) |
| `/{account}/policy/import` | Import the account's own rules from a file ([8.7](#87-policy)) |
| `/{account}/system` | System ([8.8](#88-system)) |
| `/{account}/{dataset}?group=a&level=N&range=…&{dimension}={value}…` | an analysis lens ([8.5](#85-the-analysis-lenses)), `{dataset}` one of `messages`, `senders`, `masking`, `gate`, `audit` |
| `/{account}/{dataset}/{row-id}` | one row, the panel over the lens's L3. Not for `senders`, which has no row detail; its rows link out to `messages?sender=` |

The installation routes are matched before every account route, and the fixed and object routes
before the lens route, so `setup` is never read as an account and `plans`, `candidates`, `jobs`,
`policy`, `account`, and `system` are never read as dataset names. Account setup refuses an account
identifier equal to a top-level path segment the UI serves, OAuth client setup a client name equal
to `new` or not shaped like a project ID, and the policy screens a rule identifier equal to one of
their own route words, exactly `.` or `..`, or holding a `/`, so no account, client or rule is
unreachable ([sections 8.12](#812-connect-an-account-and-re-authorize),
[8.11](#811-oauth-client-setup) and [8.7](#87-policy)). Switching account from an object route
goes to that object's list under the new account (a plan to the plans list, a candidate to the
queue, a run to Jobs, a row detail to its lens).

**Query parameters.** `level` is explicit, 0 to 3 on a lens route. L0 ignores `group`. L1 requires
exactly one group. L2 requires exactly one group and at least one dimension filter. L3 ignores
`group`. A mismatch is a client error. `group=a` names one groupable dimension of the dataset.
`sort=column,asc` or `sort=column,desc` names one of the dataset's sortable columns, and each
dataset has a default. `page=P` is a 1-based page number over a fixed 50 rows. `range=` is `24h`,
`7d`, `30d`, `90d`, `all`, or `from,to` as two UTC dates, each a whole day, `to` inclusive. Every
other parameter is a dimension filter named as the registry declares it.

**Filter grammar.** `dim=value` is equality. `dim=a,b` is any of. `dim=!value` is exclusion. The
unfiled group of the label dimension is `label=none` in the URL and a `null` key in the API. An
audit row with no message falls into the `none` group of every message-derived dimension. A value
stored empty, such as a sender's empty domain, is `sender=empty` in the URL and an empty key in the
API, on a dimension that can hold one ([section 17.1](#171-the-dataset-endpoint)). A default
filter the operator removed is written `dim=` with no value in the browser's URL, so the router's
canonicalization does not put the default back. The router leaves it out of the request it sends,
because an absent filter is no filter to the endpoint. Filling default filters only into a URL
that carries no filter at all was the alternative, and it would leave a hand-written URL ambiguous.

**Default range per dataset:**

| Dataset | Default range |
| --- | --- |
| messages, senders, candidates, plans, rules | all time |
| policy_changes | last 30 days |
| masking, gate, runs | last 7 days |
| audit | last 24 hours |
| ops, failures | bounded by their parent, no range |

**The registry is closed.** These are the datasets the dataset endpoint serves, and nothing else
is a dataset.

| Dataset | Rows | Parent | Sensitivity counts |
| --- | --- | --- | --- |
| `messages` | messages, with `label` as a grouping (the label distribution is a grouping, not a dataset) | none | yes |
| `senders` | one per sender domain, each row carrying the sender's class. No row detail; identity `domain` | none | no |
| `masking` | masking events | none | yes |
| `gate` | gate decisions | none | yes |
| `audit` | audit rows | none | yes |
| `candidates` | policy candidates | none | no |
| `plans` | reorg plans | none | no |
| `ops` | a plan's message operations | `plan`, required | yes |
| `runs` | job runs | none | no |
| `failures` | a run's item failures | `run`, required | yes |
| `rules` | policy rules, the base rules and the account's own. Identity the scope and the rule identifier together, since both scopes may hold one identifier, written `base:{id}` or `account:{id}` in the row-detail path, the scope word ending at the first colon | none | no |
| `policy_changes` | the policy history, one row per change to a rule (ADR-0102) | none | no |

The op log, the accounts table, and the rate state are read by the bespoke endpoints of
[section 17.4](#174-the-bespoke-endpoints), never as datasets. Row detail exists for `messages`,
`masking`, `gate`, `audit`, `ops`, `failures`, and `rules`. `senders` and `policy_changes` have none. `plans`,
`candidates` and `runs` have none as datasets, because their row paths belong to the bespoke
screens.

A run's detail is the Run screen of [section 8.4](#84-run), and a row of the `runs` dataset opens
it. A panel of the `runs` dataset over the Jobs table was the alternative. That panel would sit at
`/{account}/jobs/{run-id}`, which is the Run screen's route, and the Run screen holds a ladder of
its own over the run's failures, with its own row detail, which a 480 px panel cannot hold.

**Areas**, the way the datasets and the bespoke reads group for the operator:

| Area | Datasets | Bespoke reads |
| --- | --- | --- |
| Corpus | messages, senders | |
| Sensitivity | masking, gate | |
| Policy | candidates, rules, policy_changes | the match counts behind adding a rule |
| Change | plans, ops | the op log, through the plan endpoint |
| Audit | audit | |
| Jobs | runs, failures | the jobs summary, the run summary |
| System | | accounts, rate state, the system summary, the account |
| Installation | | the installation summary |

**Policy is the one exception to per-account rows.** Policy lives in the database, as ADR-0004's
base policy shared by every account and ADR-0085's per-account overlays, so it is not one
account's data. `/{account}/policy` shows the base rules, marked "base", and the account's overlay
rules (ADR-0004), and base rules are written from it as well as the account's own
([section 8.7](#87-policy)). The base policy alone also has an installation screen, which needs no
account to exist ([section 8.14](#814-base-policy)). No row of another account's data is shown, so
the per-account rule of [section 1](#1-what-the-ui-is-for) is not breached. Nothing else on an account's screens shows a row
outside the account in the path.

**Decisions.** Two, each with two outcomes. A plan is approved or rejected, a candidate is
confirmed or dismissed. Four requests carry them, all by database grant (ADR-0084), which calls
them its two decision verbs. No request exists for retrying a job, triggering a rollback, or
editing policy other than through the policy management of [section 8.7](#87-policy).

**OAuth client setup and account setup.** Two separate flows over two separate stored records
are the UI's other writes (ADR-0080, ADR-0084, ADR-0106, ADR-0107). OAuth client setup runs once
per client, and an installation holds any number of clients for each provider that authenticates
through one. Account setup connects an account through one of its provider's clients, shared with
other accounts or its own, sets what the account's rows hold, and re-authorizes an account whose
credential stopped working, through the same client or another of the provider's. The UI seals
each client's secret and each credential it receives, and its code never opens a stored
credential. One isolated part of it opens a client's secret, for a consent's code exchange
(ADR-0081).
Their screens are [sections 8.10](#810-installation) to [8.13](#813-account-settings).

**Growth slots the shape already fits**, each arriving as a registered dataset. Calendar events
(restriction on any participant, per ADR-0027), the second account (the scope selector earns its
keep), the Fastmail backend (nothing changes above the port and nothing in the UI), rollback runs
(already `runs`), agent activity (audit rows by actor and session), and dashboards (either lenses
here or links to the metrics stack). A feedback decision on masking and gate events (marking a
true or false positive) would be a third decision and needs its own record first. The row-detail
layout leaves room for it.

## 6. Global chrome

Every screen shares one frame, top to bottom.

| Region | Content, in order | Notes |
| --- | --- | --- |
| Top bar | the account selector, then the primary navigation (Home · Plans · Jobs · Corpus · Masking · Gate · Audit · Review queue · Policy · System), then search, then the settings menu, then the live indicator on live surfaces | The account selector shows the account identifier and its provider. Its menu lists every account from the accounts endpoint, each with its provider and no other value of it, then three links, Account settings for the account in view ([section 8.13](#813-account-settings)), Connect an account and Installation ([section 8.10](#810-installation)). Switching accounts keeps a lens's dataset and level and drops its filters, and follows [section 5](#5-information-architecture-and-the-url)'s rule from an object route. The settings menu holds the theme override (system, dark, light) and the keyboard map |
| Address line | the URL of the current view, as text, selectable | It is the share handle. It updates on every navigation |
| Breadcrumb | the applied filter chips in the order applied, each removable, preceded by the lens or object name | Absent on Home. On a plan or a run it starts with the object |
| Range control | on every dataset with a range, the presets (last 24 hours, 7 days, 30 days, 90 days, all time) and a custom UTC range with a start and an end, writing `range=` | Sits at the right of the breadcrumb line |
| Group-by control | the dataset's groupable dimensions, in the registry's order, writing `group=` | Sits beside the range control, on levels 1 and 2 |
| Body | the screen | |

An installation screen ([section 8.10](#810-installation)) has the same frame with no account in
view. The account selector reads "Installation". The primary navigation is the installation's
own, Setup · Base policy, and search, the range control and the group-by control are absent, since
each of them belongs to an account. The breadcrumb starts with "Installation". Base policy reads as
the current screen on every base policy screen, a rule, the history and import included, and a
panel a base policy route opens makes the frame behind it inert, as an account's does.

The primary navigation marks the current screen. Review queue shows the count of pending
candidates and Jobs shows a running mark when any workload is running, both read from the system
endpoint's decisions block ([section 17.4](#174-the-bespoke-endpoints)) on every page load and
updated from decision responses and the stream. The stream updates them on the screens that follow
it, the live surfaces and the partial-index banner of [section 9](#9-live-surfaces), where a run
event whose state differs from the one last seen or shown reads the system endpoint again, since
only a change of state can change the running mark. Holding a stream open on every screen for the
marks alone was the alternative, and section 9 keeps the stream to those screens. Nothing else in
the chrome carries a number.

The router mounts each screen afresh for each route identity, which is the account and the object
the path names, such as a run. Switching accounts, or following a link from one run to another,
therefore starts the screen, its reads, its live state and everything it holds anew, for every screen
and for any screen added later. A test over the route table requires every route to mount its screen
this way, apart from two. The entry route holds no account and only sends the browser to one. The
not-found route holds no state of its own, and the frame it shows follows the account its address
names through the frame's own effects. The row a detail panel opens over a list is not part of the
identity, since the list behind the panel stays the same screen. Keying each component by the inputs
it reads was the alternative, and a component that missed one input would show one account's or one
run's data under another's address.

## 7. The rows and the charts

### 7.1 The message row

One component renders every message-derived row, in the corpus lens, the plan's operations, the
masking and gate lenses, the audit lens where a message is present, the review queue's sender
messages, and a run's message and operation failures. Its fields, in order, one line each, with
the wire name the row carries in the API. Every cell truncates to its column width with an
ellipsis and shows its full value on hover.

| Field | Wire name | Width | Rendering | Links to |
| --- | --- | --- | --- | --- |
| sender address | `from_email` | 220 px | mono, the full address | the corpus lens at L3 with `from=` this address |
| masked subject | `subject` | flexible, 240 px minimum | text, the stored subject exactly (already masked at rest per ADR-0003) | the row's detail (L4) |
| date | `sent_at` | 88 px | `2026-09-03` in the row, the full UTC time on hover | nothing |
| labels | `labels` | 200 px | the first four labels in stored order as chips, then `+N`; below 1360 px the column collapses to one chip with the count | the corpus lens filtered by that label |
| sender class | `sender_class` | 72 px | a badge, `restricted` in the restricted color or `normal` in muted text | nothing |
| content flags | `content_flags` | 72 px per badge | one badge per flag, `mfa` or `link`, in the flagged color | nothing |
| scan state | `scan_state` | 96 px | muted text, the short form of [section 11](#11-rendering-and-formatting-rules), the long wording on hover | nothing |

A link to a screen that does not exist yet is left out and the cell shows its value, since the UI
links only to screens that exist. A row whose message the index no longer holds, such as a failure
whose message has since left the index, has no message fields to show. Its sender cell shows the
message identifier in mono and its subject cell "not in the index" in muted text, and the other
message cells are empty. Leaving such a row out was the alternative, and the failure would then be
missing from its run's list while its counts still count it.

A lens adds columns after these for its own fields. Each lens declares its extra columns with
their widths and the table's minimum width, and below that width the table scrolls horizontally
inside its own container, never the page. Nothing is ever inserted before the scan state. The row
never wraps. It is 36 pixels tall and, being a full-width target, is exempt from the 44 px
minimum that binds standalone controls.

A stored value the wording table does not know (a content flag, a scan state, an error class
outside the expected set) renders as the value itself in muted text with an `unknown` badge,
never hidden. The browser's counterpart to ADR-0042's rule that an unhandled variant denies is
that an unhandled value is shown, not dropped.

The row detail (L4) shows the same fields as a two-column list, then a sensitivity block (sender
class with the rule id that set it, linking to the account's policy searched for that identifier,
which lists the base rule and the account's own rule when both scopes hold it, since the index
records the identifier and not its scope (ADR-0110), content flags with the rule ids that fired, and scan state with
the time scanned and the scanner version), then the audit rows for this message as an L3 list,
then, when reached from a plan, what the plan does to it and why. Never a body, never a snippet. The
database has no column to show (ADR-0016). Each axis shows the rule behind it, as the index records
them (ADR-0016, ADR-0001). Sender class and content flags are separate questions with separate
consequences ([DESIGN.md](../DESIGN.md#sensitivity-is-two-independent-axes)). The sender class
decides whether the body can be read and whether the message can be disposed of, and the content
flags decide only whether it can be read. Tuning a rule starts from seeing which rule fired and on
what ([O4](../USE_CASES.md#o4--the-operator-can-see-and-steer)), and that needs each rule shown
beside the axis it set. One list of every rule id was the alternative. Its case was fewer cells and
the shape the index used to have. It lost because a reader could not tell which rule set which
axis. When no rule set the class (ADR-0016 names when), its rule shows as "none", as an empty list
of rule ids that fired does, never as an empty cell.

The rule id that set the sender class links to the account's policy searched for it,
`/{account}/policy?search={rule-id}`, which lists that identifier's rule in each scope that holds
one, among any other rule whose identifier or suffix contains the text (ADR-0110). A
sender class read `normal` carries the control "Restrict {domain}…", which opens Add a rule at
`/{account}/policy/new?suffix={domain}` ([section 8.7](#87-policy)), so a restriction starts where
the need to make one arises. Both follow the rule above, that the UI links only to screens that
exist.

### 7.2 The sender row, the audit row, the page row, and the policy change row

Four more row types exist, each with the same rendering rules as the message row.

| Row | Fields, in order | Where |
| --- | --- | --- |
| sender row | domain (mono, links to `messages?sender=` this domain), sender class badge, message count, first seen, last seen, list-id ratio as a percent, scan hits, and for a `normal` sender the control "Restrict {domain}…" of [section 7.1](#71-the-message-row) | the `senders` dataset at L3, the review queue's sender statistics |
| audit row | time, actor (mono), action wording, then the message row's fields when the row has a message and nothing when it does not. The detail of a body served whose sender is `normal` carries "Restrict {domain}…" as the message row's detail does | the `audit` dataset at L3, the row detail's audit list |
| page row | page number, error class, attempts, last error time, disposition, and the recovering run where recovered | a run's failures whose item is a page rather than a message |
| policy change row | time, identity (mono), the action's wording, the rule identifier (mono, linking to the rule while it exists), the scope as a badge (`base`, or the account), and the change as suffix chips, `+domain` in the ok color for a suffix an edit added and `−domain` in the restricted color for one an edit or a lift removed, and the rule's suffixes unmarked for a change that created the rule, added or confirmed. A lift, and an edit that removed suffixes, end with "Restore" ([section 8.7](#87-policy)) | the `policy_changes` dataset at L3, a rule's history |

The dimension `sender` is the domain (`from_domain`) everywhere. The filter `from` is the address.

The page row shows no cursor. The provider's page token is provider state that no table keeps for a
failed page ([ADR-0016](./adr/data/0016-schema.md) records the page number), so a cursor cell would
have nothing to read. On the run screen a page row sits in the same table as the message rows, its
page number spanning the message row's fields, so the failure columns stay aligned. Listing page
items in a table of their own was the alternative, and one run's failures would then sit in two
tables, each with its own sort and pages.

### 7.3 Charts

Every chart is inline SVG drawn from the same rows its table shows.

| Form | Rules |
| --- | --- |
| bars (L1 and L2) | horizontal, 20 px tall with 8 px gaps, filling the region's width, one bar per group, sorted by count descending, the top 20 groups then one `other` bar for the rest, which is not clickable. Direct label with the count at the bar's end, a hairline baseline in the border token, no gridlines, bar fill the neutral chart fill with the restricted share drawn as an inner segment in the restricted color, which a dataset that is not message-derived does not draw. The table beside it lists every group, 50 per page, paged in the browser under `page=`, since the endpoint answers every group at once ([section 17.1](#171-the-dataset-endpoint)). A value stored empty is drawn and listed as "empty" and links with the filter word `empty` on a dimension that declares it can hold one ([section 17.1](#171-the-dataset-endpoint)). A stored value the filter grammar of [section 5](#5-information-architecture-and-the-url) cannot name as itself is a group no filter can name, and is drawn and listed without a link, as the `other` bar is. That is a value holding a comma, which the grammar reads as any of, a value starting with `!`, which it reads as an exclusion, a value spelled `none` or `empty`, and a value stored empty on a dimension that does not declare it. When the dimension is an array (labels, content flags, rule ids) a row counts in each of its groups, shares are over the total count, and the chart carries the note "a message with several labels counts in each" |
| flow diagram (a plan's flows) | 320 px tall. Old labels in a column on the left sorted by outflow descending, with a `none` row for flows that remove nothing, new labels in a column on the right sorted by inflow descending, one straight band per flow drawn in the left column's order with 2 px gaps, band width proportional to message count, the selected band in the action color and the rest in the neutral fill |
| run timeline | 120 px tall. x is wall time from the run's start to its finish or to now, marks for start, backoff, retry, failure, resume, and finish drawn from the run's events; for paged runs y is the page index and the progress events trace a line; for runs without pages the marks sit on one line. A run is paged when any of its events records a page. Each mark kind has its own shape, so no kind is told by color alone, and each mark's hover names its kind, its time and the event's detail as recorded, which renders as text |
| progress bar | track in raised, fill in info while running, ok when complete, restricted when failed, the count of total as text beside it |

## 8. The screens

The chosen shape (ADR-0056) is a home organized around the operator's work, a plan reviewer shaped
like code review, the jobs surfaces, and the ladder for everything analytical. Each screen below is
specified as regions in placement order, the fields of each region in order, which endpoint of
[section 17](#17-the-read-api) feeds it, its interactions, and its states. Numbers in examples are
illustration, never a specification. Screen text that the design fixes is written as a template
with placeholders in braces.

### 8.1 Home

`/{account}`. Under the chrome, the running-work strip full width, then two columns, the left
twice the width of the right. Left, "Awaiting your decision" then "Worth a look". Right, "System".

**Running now**, a live strip fed by the jobs summary endpoint and the event stream. One cell per
workload of ADR-0022, in order, each with the workload name, its workload state (the vocabulary in
[section 11](#11-rendering-and-formatting-rules)), and the fields below. The strip's title links
to Jobs.

| Workload | Fields |
| --- | --- |
| Backfill | the pass running or last completed, its run id, started time, heartbeat age, checkpoint page of total pages, percent as a progress bar, estimated time left, the batch class's used of reserved |
| Delta sync | last tick time and outcome, cursor age, cadence, the last change set as added, modified, removed |
| Reorg apply | `idle` with the count of plans in DRAFT, or the plan being applied with operations done of total |
| Heuristics | last run time and duration, candidates emitted |

Estimated time left is pages remaining divided by the pages completed in the last ten minutes of
the run, read from the run's progress events, and is absent until the run has ten minutes of
history.

Backfill's cell shows pass 1 while pass 1's latest run runs, else pass 2 while pass 2's latest run
runs, else the pass whose latest run finished last, and "not started" when neither pass has a run.
The checkpoint and its progress bar are drawn for every backfill run, the text reading "no page yet"
and the bar an empty track until the run's checkpoint records a page of pages, so a pass that starts
while Home is open shows its progress on its first page event, as Jobs' pass 2 bar does. Hiding them
until the first answer held a page was the alternative, and a pass started after the page loaded
would then show no progress until the page was read again. The estimated time left is read from the
jobs endpoint and shows once the run has its ten minutes of history there. The strip reads the same jobs endpoint as Jobs and refreshes live the way
Jobs' cards do ([section 8.3](#83-jobs)). Each cell's run fields, the progress bar and the batch
class's used of reserved show the latest event for their run or for the rate, bound to that object
alone, so an event redraws that text and re-renders nothing else. A run event for a run no cell
shows, or whose state differs from the one shown, reads the jobs endpoint again, and a change of
state also reads the system endpoint again, for the chrome's running mark and the reorg apply
cell's count of plans in DRAFT. The rest of Home is not live.

**Awaiting your decision**, fed by the plans dataset filtered to DRAFT and the candidates dataset
filtered to pending. Plans first, oldest first. Then candidates by score, highest first. The
heading carries the item count and the oldest plan's age, or with no plans the oldest candidate's
age. Each item is one row and its whole surface opens the object. Items carry no decision
controls. Decisions happen on the object's screen, where the sentence is. The item's kind (Plan,
Candidate) is the row's label, not a field.

| Item | Fields, in order | Link text |
| --- | --- | --- |
| Plan | title, plan id (short), status wording, validation result, restricted messages inside with "label and move only", messages, threads, share of corpus, label operations, age of maximum age rendered as "1d 4h of 14d" | Review plan |
| Candidate | domain, the strongest signal in words ([section 8.6](#86-review-queue)'s templates), score, message count, first seen month, time in queue | Review |

A plan's title is the first line of its description, truncated at 80 characters.

The inbox shows the first page of each list, 50 items, and the heading counts every item. The
candidates' read is sorted by score, so the oldest candidate's age comes from a second read of the
same filter sorted by created time ascending, whose first row is the oldest. Taking the oldest
candidate of the page shown was the alternative, and with more than one page of candidates it
would understate how long the queue has waited. A candidate's score shows two decimals, and its
message count and first seen month show "none" when the index holds no sender row for its domain.

**Worth a look**, fed by the attention endpoint, which returns each card complete. Zero to a
handful of cards, newest first. Each card is the what, the number, since when, the sentence for
its rule, and a link into the lens that explains it with the value applied as a filter. The rules,
their starting thresholds, and their sentences are the table below. They are this design's own,
undefined anywhere else, and starting values. Every threshold is a configuration key of
[section 18.1](#181-the-configuration-the-ui-declares), read at start, and a value of 0 disables
the rule. With no card, the region says "Nothing is worth a look right now."

| Rule | Fires when | Sentence | Links to |
| --- | --- | --- | --- |
| Backlog | pending scan above 5% of the corpus | "{count} messages are pending scan ({share}% of the corpus). Every pending message denies its body until scanned, which reads to the agent like a permission problem." | the corpus lens at L3 with `scan_state=pending` |
| Masking | one sender above 20 events in 7 days under one rule | "Masking fired {count} times on {sender} this week, all under {rule}. A sender masked this often under one rule is worth checking for an over-mask." | the masking lens at L3 with `range=7d`, the `rule=` and the `sender=` |
| Body serves | more than twice the 7-day median in 24 hours, the anomaly [A4](../USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything) names | "{count} bodies were served in 24 hours against a 7-day median of {median}. Body-serve volume beyond triage plausibility is the anomaly the design watches for." | the audit lens at L3 with `action=READ_BODY` and `range=24h` |
| Sync gap | one recovered in the last 7 days | "Delta sync recovered from a cursor gap on {time}, re-enumerating a {window} window and reconciling {count} messages. A repeated gap means the cadence or the cursor lifetime needs attention." | Jobs, its runs with `pass=gap_recovery` over the rule's days |
| Expiry | a DRAFT plan within 2 days of its maximum age | "Plan {title} expires in {remaining}. After that the apply job refuses it and the client must propose again." | the plan |

The server words every card, number, since and sentence included, so the browser renders text it
does not compose, and a masking card's sender, a domain an adversary chose, renders inert like
message-derived text ([section 11](#11-rendering-and-formatting-rules)). The card's what is the
rule's name in the table. What each rule reads, and what its number and since are, is below. Each
"above" and "more than" is strict, so a value equal to its threshold does not fire.

| Rule | Reads | Number | Since | Cards |
| --- | --- | --- | --- | --- |
| Backlog | the messages pending scan against every message the index holds, as of the read | the messages pending scan | none, and the card shows "now" | one, and none while the index is empty |
| Masking | the masking events of the last 7 days whose scanner version and revision equal those their subject was masked under ([section 8.5](#85-the-analysis-lenses)), each counted under its rule and the domain of its message's sender, lowered as the `sender` dimension lowers it ([section 17.1](#171-the-dataset-endpoint)) | the pair's events | the pair's first event in the 7 days | one per sender and rule above the threshold |
| Body serves | the `READ_BODY` audit rows of the last 24 hours, against the median of the daily `READ_BODY` counts over the seven whole UTC days before the 24 hours start, a day without one counting 0 | the bodies served in the 24 hours | the first of them | one |
| Sync gap | the delta-sync runs whose pass is `gap_recovery`, that succeeded and started within the rule's days | the recoveries | the first one's start | one, its sentence worded from the latest recovery |

- **Backlog has no since.** No column records when a message became pending, so the card is a
  state as of the read. Showing the oldest pending message's sent time was the alternative, and it
  would date the backlog by when mail was sent, not by when it started waiting.
- **Masking counts an event only through its message.** An event whose message the index no longer
  holds has no sender to count under, so it counts toward no card. An event of a masking a change
  of scanner replaced counts toward none either, so a subject masked again is never counted twice.
  A sender's domain that the filter grammar of
  [section 5](#5-information-architecture-and-the-url) cannot name is left out of the link, which
  then names the rule alone. One card per sender and rule is the rule's own unit, and one card
  naming the worst pair was the alternative, which would hide every other over-mask.
- **A change of scanner can raise masking cards.** Masking every subject again records a new event
  for each mask, under the current scanner and in the week it happens
  ([ADR-0096](./adr/redaction/0096-a-scanner-change-reopens-backfill.md)), so in the week after a
  change of scanner a sender whose subjects now carry more masks under one rule than the count
  raises a card, however old its mail is. That card is true, since the rule did mask those
  subjects that week. Recording which events are re-masks was the alternative, and it is a schema
  change with no consumer but this card. Counting by the message's sent time was the other, and it
  would date mail rather than masking.
- **The body-serve median reads the same days as the audit lens's figure**
  ([section 8.5](#85-the-analysis-lenses)), the seven whole UTC days before the 24 hours start, so
  the card and the lens it links to show one median. Body denials are not serves and are not
  counted. A week without a serve has a median of 0, and twice 0 is 0, so any serve after such a
  week fires the card. That is kept on purpose. A first serve after a quiet week is a change in
  the agent's behavior, the anomaly the rule exists to show, and the sentence names the median of
  0, so the operator reads the card for what it is. It also means every serve fires the card in
  the first week after the index has serves. A floor below which the rule stays silent was the
  alternative, and it would hide exactly the serves after a quiet week.
- **A sync gap counts only recoveries that succeeded**, because a recovery still running or failed
  has not recovered, and Jobs shows it. One card counting the recoveries is the rule's answer to
  "a repeated gap", and one card per recovery was the alternative, which would fill Home during a
  week of repeated gaps while saying what the count says. The time is the latest recovery's finish,
  and the window is its counters' window end less its window start (ADR-0016). A recovery whose
  counters lack the window or the reconciled count is worded without that clause rather than with
  a value it did not record.
- **Newest first** orders the cards by since, newest first, with a card that has no since first,
  since it is as of now. Cards with the same since keep the table's order, and masking cards with
  the same since are ordered by sender, then by rule.
- **A link to a screen that does not exist yet is left out** and the card shows no link, as
  everywhere in the UI. The endpoint sends each card's link as the table states it, and the browser
  decides whether the screen exists.

**System**, the right column, fed by the system endpoint's three blocks. As-of time first, then
one row per value, each a link.

| Block | Value | Links to |
| --- | --- | --- |
| operational | backfill pass 1, complete or its progress | Jobs |
| operational | backfill pass 2, percent and pending count | the corpus lens with `scan_state=pending` |
| operational | sync cursor age and last successful tick | Jobs |
| operational | rate, current of target, cap, backoff wording | Jobs |
| operational | last provider authentication outcome and time | Account settings ([section 8.13](#813-account-settings)) |
| corpus | audit last 24 hours, one row per action (body served, body denied, mutation applied, mutation refused) | the audit lens at L3 with `action=` that value and `range=24h` |
| corpus | messages | the corpus lens at L1 |
| corpus | threads | the corpus lens at L1 |
| corpus | unfiled, with percent | the corpus lens at L3 with `label=none` |
| corpus | restricted messages | the corpus lens at L3 with `sender_class=restricted` |
| corpus | masking events, last 7 days | the masking lens at L1 |
| corpus | gate skips of decisions, last 7 days | the gate lens at L1 with `decision=skip` |

A row whose screen does not exist yet renders its value unlinked, as the system screen's rows do
([section 8.8](#88-system)).

The corpus block's masking count measures masking activity in the window, so it includes the masks a
change of scanner records when it masks every subject again
([ADR-0096](./adr/redaction/0096-a-scanner-change-reopens-backfill.md)). It can then exceed the
masking lens's count, which counts only the masks of each subject as it now stands.

States. With no plans and no candidates, the inbox says "Nothing awaits your decision" and the
column keeps its height. With backfill pass 1 not started, which is pass 1 neither complete nor
with a run, the strip shows the workload as not started, and the partial-index banner of
[section 12](#12-empty-loading-partial-and-error-patterns) says indexing has not started, so the
corpus rows' counts read as counts so far and an empty index never reads as final. While a pass
runs, the banner sits above the columns. While pass 1 runs with no earlier run of it succeeded, the
corpus rows show counts so far, and while a re-opened pass 1 runs they do not, since the index
already holds the whole mailbox ([section 12](#12-empty-loading-partial-and-error-patterns)). A
failed read on one region shows that region's error card and leaves the others alone.

### 8.2 Plan reviewer

`/{account}/plans/{plan-id}?section=…`. A header, a left rail, a main area, and a footer. Fed by
the plan endpoint (header, Summary, Label operations, Validation, History), the `ops` dataset with
this plan as parent (Flows, Restricted inside, and everything the ladder shows), and the sample
endpoint (Sample).

A plan has one operation per message, so messages and operations are one number, called
"messages" everywhere on the plan's screens. Flows are ADR-0020's, recorded per operation when
the plan is saved, and an operation counts in each of its flows. L0 totals count messages, never
flows.

**Header.** Title, plan id, the status timeline, created time, the proposer, age of maximum age
rendered as "1d 4h of 14d" (from the endpoint's `expires_at`), validation result, and
right-aligned the two outcomes. The timeline is DRAFT › APPROVED › APPLYING › APPLIED ›
ROLLED_BACK with two side exits, REJECTED leaving from DRAFT and APPLY_REFUSED leaving from
APPROVED (the apply job refuses before its first write, ADR-0032), the current state marked. A
one-line note under the outcomes says approving writes status, approved time, and approver,
nothing else, and that rejecting writes REJECTED the same way.

**Rail.** The sections in order, with `?section=` tokens. Selecting one shows it in the main area
at that section's default level with no filters. Only list sections carry a count.

| Section | Token | Content |
| --- | --- | --- |
| Summary | `summary` | L0. messages, threads, share of corpus with a mark at 25% (the second-confirmation threshold of ADR-0020), new labels, label operations, restricted messages touched, validation result, age |
| Label operations | `labels` | the label operations from the plan's `label_ops` in plan order, each with its kind (create, rename, delete), the label names, and the count of messages whose operation touches it computed from the plan's operations. A delete says its associations are removed first, per ADR-0020. Count shown |
| Flows | `flows` | L1 by default, the `ops` dataset grouped by flow. The flow diagram of [section 7.3](#73-charts) and the same flows as a table with messages, restricted count, and the old label, showing the removed label, or "none, existing labels kept" for a `none>added` flow. Count shown |
| Restricted inside | `restricted` | the count, the statement that these receive label and move operations only and that the authorizer rechecks every operation at apply time (ADR-0019), and the `ops` dataset at L3 filtered to `sender_class=restricted`. Count shown |
| Sample | `sample` | the stratified sample, its size, its seed shown in the URL, and a re-draw control that changes the seed. Rows are message rows with before → after and reason |
| Validation | `validation` | the creation-time result (`passed` or `failed`) and its findings from the plan's recorded validation. Once apply has run, the apply-time result and, when refused, the refusal reason |
| History | `history` | the events as a list, each with its time and identity, assembled from the plan row (created, with the proposer), its decision columns (approved or rejected, with the recorded identity), the apply run found by its plan reference (apply started, applied, or refused), and the rollback run (rolled back). Count shown |

Share of corpus is the plan's message count over the count of messages in the account's index at
read time.

**The sample.** Size 24 by default (a configuration key). Strata are flow × sender class.
Allocation is proportional to stratum size rounded down, with the remainder going to the largest
strata. When non-empty strata outnumber the size, the largest strata by size each get one row and
the rest none. Strata are drawn in descending size order. Within a stratum, rows are ordered by the
MD5 of the message id concatenated with the seed and the first allocated rows are taken, skipping
any message already drawn for an earlier stratum, so a message appears at most once and the same
seed always draws the same sample. The default seed is the plan id's first eight characters, and
re-draw picks eight random hexadecimal characters.

**Main area.** The selected section. From Flows, clicking a band applies `flow=` and shows L2
grouped by sender (the group-by control offers sender, reason, and month). Clicking a sender row
applies `sender=` and shows L3 rows. Each row opens L4 as a panel. The breadcrumb reads plan ›
flow › sender.

**Footer.** Present in DRAFT only. Its text, in order:

1. "Approving applies {messages} message operations across {threads} threads ({share}% of
   {account}), creates {created} labels, renames {renamed}, and deletes {deleted} after they are
   emptied. The {restricted} restricted messages inside receive label and move operations only.
   The plan is re-validated and age-checked again immediately before its first write, and every
   operation is logged for exact rollback." Zero-count clauses are omitted.
2. "The decision is recorded as {identity}, taken from {source}." where the source is the
   declared identity header's name or "the configured operator name", whichever is in force.
3. The second-confirmation control (below).
4. Reject, then Approve.

**The approve flow.**

1. Approve is enabled only in DRAFT with a recorded creation-time validation whose result is
   `passed`. A plan with no recorded result reads "validation result missing" and Approve stays
   disabled.
2. Under a quarter of the corpus, Approve submits at once. At a quarter or above, the
   second-confirmation control is a text field asking the operator to type the plan's message
   count as shown, and Approve stays disabled until the digits match. Separators typed or omitted
   both match, because the server compares digits only.
3. The request carries the plan id, the status the screen shows, and the typed confirmation when
   required. The server recomputes the share of corpus on approve and requires the confirmation
   at or above a quarter, and the browser mirrors that rule for the control. On a 400 or 409 the
   screen reloads the plan and clears the confirmation field.
4. On success the header shows APPROVED, History gains the event with time and identity, the
   footer disappears, and a note says the apply job will re-validate before its first write.
5. Reject submits at once with the same conflict rule and shows REJECTED the same way.

The status the screen shows, sent as `expected_status`, is the whole concurrency story. Two
operators deciding the same object see the second refused with a conflict and reloaded to the
first's outcome. Nothing else locks.

**While APPLYING.** The header shows the apply run's id and links to it. Summary gains a progress
bar of operations logged in the op log against the message count (read through the plan endpoint,
which scopes the op log through the plan's account), the checkpoint, and failures so far linking to
the run. History gains the apply start.

**After a partial apply.** The state [A3](../USE_CASES.md#a3--bulk-change-is-reversible) calls
describable. Summary shows operations applied of total, the checkpoint, and that the run will
resume or has failed, with the run linked. Flows shows per flow how many operations are applied.
Nothing on the screen offers a retry.

**ROLLED_BACK.** The timeline marks it, History carries the rollback run with its time, and
Summary states that every operation was restored from the op log.

**APPLY_REFUSED.** The timeline marks the side exit, Validation shows the apply-time result and
the refusal reason (a re-validation failure or age past the maximum), and the footer is absent.
The refusal produces the apply run's row with state `failed`, zero operations done, and the reason
as its last error, so it appears in Jobs like any failed run. The plan is closed. A new plan is
the client's to propose.

### 8.3 Jobs

`/{account}/jobs`. Live. Three regions, fed by the jobs summary endpoint, the rate block it
carries, and the `runs` dataset, with the event stream patching all three.

**Workload cards**, one row of four, each with the workload name, its workload state as a labeled
mark, and its fields. A card reads the workload's runs. Backfill's pass 1 completion mark reads the
account's pass-1 flag, and the run row supplies the numbers.

| Card | Fields |
| --- | --- |
| Backfill | pass 1 with its outcome, message count, page count, duration, and completion time. Pass 2 with its run id, checkpoint page of total as a progress bar, decided of total, pending, scanned this run, skipped, started time, heartbeat age, estimated time left |
| Delta sync | state, cadence, last tick time and duration, cursor age, last change set as added, modified, removed, gap recoveries in the last 7 days with the last one's time, window, and messages reconciled |
| Reorg apply | state, the plan being applied with operations done of total, or `idle` with plans in DRAFT. Last run with its plan title, outcome, operations, failures, duration, date, and whether rollback is available (the plan is APPLIED, per ADR-0020's indefinite rollback) with the op log row count |
| Heuristics | state, cadence, last run time and duration, candidates emitted, candidates awaiting review, next run time |

The count of plans in DRAFT on the reorg apply card and the count of candidates awaiting review
on the heuristics card come from the system endpoint's decisions block, not from the jobs
endpoint. Pass 2's decided of total counts the decided messages against the decided and the pending
together, which is every message the pass has to decide. The progress bars are the one of
[section 7.3](#73-charts), with the count of total written beside each.

Cadence and next run time are configuration displayed, not recorded state. The cadences are
intervals, ADR-0018's sync interval and the heuristics interval, both configuration keys of
[section 18.1](#181-the-configuration-the-ui-declares), and next run is last run plus the
interval.

**Rate budget**, from the rate block. Current of target and the cap in units per second, the
backoff wording, last throttle time. Then one bar per priority class of ADR-0025 (interactive,
sync, batch) showing used of reserved, with the sentence that interactive keeps its reservation
and batch absorbs any reduction first. Each class's bar is the progress bar of
[section 7.3](#73-charts) with its used of reserved written beside it, filled in info, since a
class's spend is a rate that neither completes nor fails.

**Recent runs**, the `runs` dataset, declared like a lens. Its L0 figures are runs in range,
running, failed, and the last failure time. It is groupable by workload, state, and day, and
sortable by started, duration, and failures. The screen shows it at L3 with filters for workload,
state, and range. The default excludes routine delta-sync ticks (`pass=!tick`), shown as a
removable chip, so the table opens on the runs that matter. Sorted by started, newest first, 50
per page. A gap recovery is a run whose pass is `gap_recovery`, with its window and reconciled
count in its counters.

| Column | Content |
| --- | --- |
| run | the run id, shown in full, linking to the run |
| workload | the workload, and the pass or the plan title where one applies |
| started | UTC time |
| duration | elapsed, or "so far" while running |
| state | a labeled mark, the run-state wording in [section 11](#11-rendering-and-formatting-rules) |
| checkpoint or operations | page of pages, operations of operations, or the recovery's window and reconciled count |
| failures | the count, linking to the run when above zero |

**What refreshes live.** Each card's run, the rate budget and each row's state, duration and
checkpoint show the latest event for their run or for the rate, bound to that object alone, so an
event redraws that object's text and re-renders nothing else. A run event for a run that neither
the cards nor the table's page shows, which is a run started since the page loaded, reads the jobs
endpoint and the table's figures and page again. A run event whose state differs from the one a
card or row shows reads the jobs endpoint again too, so a card's workload state and its fields follow
the run. A rate event while the answer shown has no rate state reads the jobs endpoint again, so an
account's first spend brings up the rate budget. Building the new run's card or row, or the budget,
from its event alone was the alternative, and a card's fields come from the jobs endpoint's blocks,
which an event does not carry. A progress bar's fill is bound to the same object as the text beside
it, so an event redraws the fill with the text and re-renders nothing else. Pass 2's bar is drawn
for every pass 2 run, as an empty track until its checkpoint records a page, so a pass that has just
started shows its bar on its first page event. Every bound text follows the answer it is given, so
an answer read again, on an event, a poll of the fallback or a reconnect, reaches the cards and the
rows, and each row stays with its run when a new run takes the table's first place, the row cursor
and keyboard focus with it, so Enter opens the run the operator chose. Keeping the cursor at its
position was the alternative, and a run started meanwhile would then take the operator's place.
Times judged against now, such as "so far" and "in backoff until", read the live clock.

### 8.4 Run

`/{account}/jobs/{run-id}`. Any run opens it. Live while the run or its resumer (the later run
whose `resumed_from` names this one) is running. Five regions, fed by the run summary endpoint
(L0 and the timeline) and the `failures` dataset with this run as parent (everything below). With
zero failures, L1 and L3 collapse to one line, "no failures".

**L0 strip.** Workload and pass, run id, run state, started and finished times, duration, where it
failed (the checkpoint and the retries of it), item failures, how many recovered and by which run,
how many were not found at the provider, and the resuming run with its state.

**Timeline.** The run timeline of [section 7.3](#73-charts), drawn from the run's recorded events.
A legend names the six mark kinds. Start, backoff, retry, failure, resume, finish. Progress events
draw the line and carry no mark. A run with no events shows start and finish only.

**L1.** Failures by error class as bars with count and share, clickable to filter, and a second
small breakdown by disposition. The group-by control offers error class, sender, page, and
disposition.

The breakdown and the failed items are on the screen together and share the view's filters. The
breakdown reads the view at `level=`, which is 1, or 2 once a filter is applied, and the items read
the same filters at level 3 with the view's sort and page. The disposition breakdown reads the same
filters grouped by disposition. A click on a bar applies its value and regroups by the next
dimension as [section 4](#4-the-zoom-ladder) describes, and stays at level 2 where section 4 would
open level 3, since the items are already below. Opening the items as a separate level was the
alternative, and the screen would then hide the breakdown the operator just clicked. `page=` pages
the items there, so the group table beside the bars shows every group of the run on one page.

**L3.** The failed items. Message and operation items render as message rows with these columns
after the standard ones. Page, error class, attempts, last error time, disposition (and the
recovering run where recovered). Page items render as page rows. The filter chip from L1 applies
here.

**L4.** One item in a panel with three parts, in order. What happened ("The run recorded:
{error_summary}"), what it means for the message (the sentence for its disposition, below), and
provenance (run, page, attempts, first and last error time, sender class with the rule that set
it, content flags with the rule ids that fired, the time scanned and the scanner version,
disposition, and the error summary as recorded). The error summary is provider or scanner text
and never a body. The panel ends with two links, the message in the corpus lens and its audit
rows. The panel is the route `/{account}/jobs/{run-id}/failures/{seq}` over the items, read through
the `failures` row detail. Until the corpus and audit lenses exist the two links are left out, and
the panel lists the message's newest 50 audit rows the row detail carries, with their count.

| Disposition | What it means, as shown |
| --- | --- |
| recovered | "{error class} on this item. Run {run} retried it successfully. Its scan state is {scan state}." |
| pending | "{error class} on this item. It has not been retried yet. Its scan state stays pending, so its body is denied until a run reaches it." |
| gone | "{error class}. Its scan state is {scan state}." When the index no longer holds the message, the last sentence is "The index no longer holds it." |
| abandoned | "{error class} on this item after {attempts} attempts. The workload gave up. Its scan state stays pending, so its body is denied until a later run reaches it." |

Both passes of backfill record a `gone` item, and they differ in what follows. The second pass
records one when a body fetch finds the message no longer at the provider, and the message waits
for a scan. The first pass records one when an enumeration did not find a message whose subject it
had to mask again, which may still be at the provider, and leaves its scan state as it was
([ADR-0096](./adr/redaction/0096-a-scanner-change-reopens-backfill.md)). So the screen says only
what holds for both. The error class reads "not found at provider", the disposition "not found",
the L0 strip counts the items not found at the provider, and the sentence reads the scan state from
the row as `recovered` does.
Wording it for each pass was the alternative. It needs the pass beside every failed item, and
the second pass's sentence would promise what delta sync later does to the row, a claim about
another workload.

The screen has no retry request. Resumption is the workload's own behavior, and the UI shows it.

**What refreshes live.** The screen follows the account's stream whenever it is open, and shows the
live indicator while the run or its resumer runs, so a run that resumes a finished one is seen when it
starts. Following the stream only while one of them runs was the alternative, and the screen would
then miss the resumer's start. Another run is another screen, as [section 6](#6-global-chrome)
says, so following a link from one run to another, the resumer, a recovering run or a failed item's
recovering run, starts the screen afresh for that run, its strip, its indicator and its reads. The L0
strip's run fields and the resumer's state show the latest event for their run, bound to it, so an
event redraws that text without re-rendering the screen or the strip. The stream carries no timeline event and no failure
(ADR-0058), so each event for either run also reads the timeline, the failure counts, the breakdown
and the items again, and those regions redraw with their answers.

### 8.5 The analysis lenses

Each is `/{account}/{dataset}` with the ladder of [section 4](#4-the-zoom-ladder), fed by the
dataset endpoint. The L0 strip shows the lens's named figures from the registry's summary query.
Aggregates sort by count descending and rows by time descending, 50 per page, and `sort=`
overrides the rows' order within the registry's sortable columns. The groups of an aggregate keep
the bars' order ([section 17.1](#171-the-dataset-endpoint)). Corpus has two tabs in its L0 strip,
Messages and Senders, each a route.

| Lens | L0 figures | Default view | Dimensions offered | Row |
| --- | --- | --- | --- | --- |
| Corpus, Messages tab (`messages`) | messages, threads, unfiled with percent, restricted messages, pending scan | L1 by sender, all time | sender, label, sender class, scan state, content flag, month | the message row |
| Corpus, Senders tab (`senders`) | senders, restricted senders, senders with scan hits | L3 by message count | sender class, month of first seen | the sender row, with "Restrict {domain}…" on each `normal` sender |
| Masking (`masking`) | events, rules that fired, senders | L1 by rule, last 7 days | rule, tier, sender, day | the message row plus rule and tier |
| Gate (`gate`) | decisions, scans, skips, and pending backlog, which is unranged and labelled "now" | L1 by decision, last 7 days | decision, reason, sender, day | the message row plus decision and reason |
| Audit (`audit`) | body served, body denied, mutation applied, mutation refused, and the body-serve count against its median over the seven whole UTC days before the range's start | L1 by action, last 24 hours | action, actor, sender class, rule, hour | the audit row |

The corpus lens's label grouping shows unfiled as its own group. A sender row's domain links to
`messages?sender=` that domain. The Senders tab also offers "Restrict senders…", which carries its
`search=` to the sender picker at `/{account}/policy/pick` ([section 8.7](#87-policy)). Audit rows
without a message fall into the `none` group of the message-derived dimensions and count as
neither restricted nor flagged. After a change of scanner every subject is masked again, and the
masking events of the maskings it replaced stay in the table, so the masking lens and home's
masking rule count a message's current masks by reading only the events whose scanner version and
revision equal those its subject was masked under
([ADR-0096](./adr/redaction/0096-a-scanner-change-reopens-backfill.md)).

### 8.6 Review queue

`/{account}/candidates`. The `candidates` dataset at L3 by default, sorted by score descending,
with a status filter defaulting to pending. The "decided" control sets `status=confirmed,dismissed`
through the any-of grammar. The L0 strip counts candidates by status. Each row is domain, score,
the strongest signal in words, message count, first seen, time in queue (from the candidate's
created time), and for a decided candidate its decision time and identity, from its reviewed
columns. Rows carry no decision controls. A row opens the candidate. The queue is not live. It
refetches when the tab regains focus.

The strongest signal is the first of the candidate's recorded signals in ADR-0004's table order
(display name, domain clustering, institution keyword, transactional pattern, embedding
similarity), worded by its template from the signal's evidence keys (one entry per heuristic that
fired, with the identifiers and evidence keys [ADR-0016](./adr/data/0016-schema.md) states).

| Heuristic | Identifier | Template |
| --- | --- | --- |
| display-name matching | `display_name` | "display name {name} matches listed {domain}" |
| registrable-domain clustering | `domain_clustering` | "registrable-domain clustering with listed {domain}" |
| institution keyword | `institution_keyword` | "institution keyword {keyword} in domain" |
| transactional pattern | `transactional_pattern` | "transactional pattern (noreply, no List-Id, never labeled)" |
| embedding similarity | `embedding_similarity` | "similar to confirmed {domain} ({score})", the score with two decimals |

The evidence is attacker-written, a display name above all, so it reaches the browser as typed
strings and a number and renders as text ([section 11](#11-rendering-and-formatting-rules)), never
as an object a generic renderer walks. A signal whose identifier the table does not name, or whose
template lacks an evidence key, renders as its identifier in muted text with an `unknown` badge, the
rule of [section 7.1](#71-the-message-row), and "unreadable signal" with the badge when it carries
no identifier. A signal whose identifier is known comes before any whose identifier is not, so the
strongest signal is an unknown one only when nothing else fired, and a candidate with no recorded
signal shows "no signal recorded".

`/{account}/candidates/{domain}`. Three regions and the outcomes.

| Region | Content |
| --- | --- |
| Signals | one row per heuristic that fired, in the order above, with the evidence recorded for it (the display name and the listed domain it matched, the listed domain it clusters with, the keyword, the transactional pattern's three facts, the similarity score and the nearest confirmed sender) |
| Sender statistics | the sender row's fields, then display names seen, sampled local parts, the label distribution as label and count pairs, and the current sender class |
| Messages | the sender's messages as message rows, L3, newest first |

Above the outcomes, one sentence: "Confirming makes {domain} a restricted sender. From each
process's next policy reload its bodies are denied and its messages become organize-only."
Dismiss writes the candidate's status, reviewed time, and reviewer, the columns ADR-0084 grants.
Confirm writes the same three and, in the same transaction, inserts the policy rule row the
confirmation emits (ADR-0004), with rule id `candidate.{account}.{domain}`, the candidate's
account as its account (an overlay rule, never the base policy), the candidate's domain as its
suffix, the restricted class, the source `candidate`, and the operator identity as its creator,
and the rule's row in the policy history (ADR-0102). Both are one transaction written by the UI's
own code, with no database-resident code (ADR-0060). The request carries the domain and the status
the screen shows, pending or dismissed, so a dismissed candidate may be confirmed later. A
confirmed candidate cannot be dismissed from the UI. Undoing a confirmation is a policy edit this
screen does not make. After either outcome the row stays in place with its new status until the operator
navigates, the pending count in the navigation updates from the response, and focus stays where it
was. A confirmed row shows "confirmed, index not yet updated" until the index's sender class for
the domain reads restricted, then "confirmed, index updated", the measure the policy screen's
"index updated" column takes ([section 8.7](#87-policy)).

A confirmed candidate whose signal names a listed domain, as the display-name and clustering
signals do, also shows "Also add to {rule}…", where {rule} is the rule that lists that domain. It
opens Edit domains on that rule with the candidate's domain added
([section 8.7](#87-policy)), so the institution's own rule can learn its domain. Confirm itself is
unchanged.

### 8.7 Policy

`/{account}/policy`, the `rules` dataset. Rows sorted by rule id, the base rules marked "base" and
the account's overlay rules after them, each with rule id, scope (a badge, `base` or the
account), domain suffixes, source (operator, or the candidate it came from), created time
and identity, the count of senders in the index it matched, and "index updated, {k} of {n}", the
matched senders whose stored sender class reads restricted of all it matches. Its hover reads
"Bodies are denied from each process's next policy reload. This counts the senders whose stored
class reads restricted." A
sender matches a rule when its domain, normalized as ADR-0004 states, ends with one of the rule's
suffixes at a label boundary, and a sender counts under every rule it matches. Policy is held as
rows and snapshotted by every process (ADR-0004, ADR-0041), and this screen is the one exception
to per-account rows stated in [section 5](#5-information-architecture-and-the-url). The L0 strip
counts the base rules, the account's rules and the senders restricted in the account, and shows
the time of the latest change with a link to the history. Above the table sit the actions Add a
rule, Restrict senders from the index and History, then Import and Export, which act on the
account's own rules (Import and export, below), and a search box, the `rules` dataset's
`search` filter ([section 5](#5-information-architecture-and-the-url)), which answers which rule
holds a domain. With no overlay rule, the account's part of the table reads "No rules for
{account} alone. The base rules above apply to every account."

A policy edit reaches every process with no manual step and no restart, through the signal between
deployables of ROADMAP's unit [F7](../ROADMAP.md#group-f--foundation), and each process then loads
the policy. This screen's sentences about a process's next policy reload rest on that.

Every change to policy is made through the UI (ADR-0004), and every change is recorded in the
policy history (ADR-0102). This section designs adding, editing and lifting rules, picking stored
senders into the policy, and importing and exporting a scope's rules as a file (ADR-0110).

**A rule.** `/{account}/policy/{rule-id}` shows the account's own rule of that identifier, or the
base rule when the account holds none, and `/{account}/policy/base/{rule-id}` always the base rule,
so a base rule and an account's rule of one identifier are each reachable. Every link to a rule
from a row of the table or the history names its scope this way. A message's rule link searches the
policy for the identifier instead ([section 7.1](#71-the-message-row)). It shows the rule's
header (identifier in mono, scope,
class, source, created time and identity), its domain suffixes, each with the senders and messages
it matches in the account, the senders it matches as sender rows, and its history as policy
change rows, the changes recorded under its identifier and its scope. Three actions, Edit domains,
Change where this applies…, and set apart from them on the right, Lift restriction.

A rule is a panel over the policy screen, as a row detail is over its list
([section 4](#4-the-zoom-ladder)), and so is Add a rule. The list behind either panel shows the
`rules` dataset's default view, since the panel's address carries the panel's own parameters, such
as `suffix=`, which the list would otherwise read as filters. `/{account}/policy/{rule-id}` reads
the account's rule of that identifier, and on a refusal as no such row reads the base rule. A base
rule's panel says "Every account inherits this rule." with a link to it on the base policy screen,
and a rule whose identifier the other scope also holds links to that rule. Change where this
applies… opens its one choice beside it rather than a menu, since a rule has only the other scope
to move to.

**What a write may hold.** Every rule's class is restricted, the one class, shown on the rule's
screen and never chosen. A rule is refused when its identifier is empty, carries surrounding space
or is taken by a rule of the same scope, or when it has no domain suffix or a suffix not shaped
like a domain name. An identifier is unique within its scope, the base policy or one account's
own rules, so the base policy and an account may each hold a rule of the same identifier, and so
may two accounts (ADR-0110).
These are the checks the policy snapshot's validation makes when a process loads the policy
(ADR-0041), and the UI makes them before it writes, so no write the UI makes fails that validation.
A base edit landing while a process reloads makes that reload read the policy once more, and fails
it only when another edit lands during the second read too (ADR-0114). One refusal is the
UI's own. An identifier that is one of this screen's route words, `new`, `pick`, `history`,
`import` and `base`, or is exactly `.` or `..`, or holds a `/`, is refused so every rule can be
reached. A rule's history is the rows recorded under its identifier and its scope, so an
identifier lifted earlier in the same scope may be added again, and its history then reads added,
lifted, added, for one rule. An identifier the scope holds reads "{account} already has a rule
{id}.", or "The base policy already has a rule {id}.". A suffix another rule already matches
is allowed, and says which rule. The scope and the identifier cannot change once a rule is added. A
rule moves scope through Change where this applies…, below.

**Scope is chosen on every add.** "This account, {account}", the default, writes an overlay rule.
"Every account (the base policy)" writes a base rule. The base policy also has an installation
screen of its own, reachable with no account ([section 8.14](#814-base-policy)), and a base rule's
screen here links to it. A base write carries the sentence "Applies
to every account. The counts shown are {account}'s." Every count the screen shows is the account
in view's alone, and nothing is summed across accounts.

**Friction follows what a change lifts** ([section 10](#10-decisions-and-their-friction)).

| Change | It | Asks |
| --- | --- | --- |
| Add a rule, or add a suffix to one | adds restriction only | no confirmation |
| Lift an overlay rule, or remove a suffix from it | lifts a restriction in the account | the lift dialog |
| Lift a base rule, or remove a suffix from it | lifts a restriction in every account | the lift dialog and the rule identifier typed |

- **Add a rule** is the panel `/{account}/policy/new`, 480 px over the policy screen like a row
  detail. Applies to, the rule identifier prefilled `operator.{first suffix}` in either scope and
  editable, so an exported file's identifiers read the same in any scope, the domain suffixes one per
  line, each checked as it is typed and shown with what it matches ("matches {n} senders ·
  {m} messages in {account}"), and the class. The sentence above the button reads "Restricts {n} senders and
  {m} stored messages in {account}. From each process's next policy reload their bodies are
  denied." Add rule stays disabled while any line is refused. A suffix that is itself a public
  suffix, or that matches more than a share of the account's senders, carries a warning on its
  line, "This matches {share} of {account}'s senders. It is likely broader than one institution.",
  which refuses nothing and asks no confirmation. Add a rule opened from the sender picker returns
  to the picker, with its search and selection, on Cancel and on `Escape`.

  The share is a tenth: a suffix matching more than 10% of the account's senders carries the
  warning. One institution's domains are a small part of a mailbox's senders, and the warning only
  advises, so warning early costs a sentence while warning late lets a broad suffix pass
  unremarked. The quarter at which a plan asks a typed restatement was the alternative, and it would
  let a suffix restricting a fifth of the senders through without a word. Each suffix is a field of
  its own, a new empty field always following the last, and a paste of several lines fills one field
  per line, so each verdict is tied to its own field as the accessibility contract below asks. One
  text area was the alternative, and a verdict cannot be tied to one line of it. The lines are
  checked together in one read of the match endpoint once typing pauses for 400 ms, and the
  verdict lists the rules already matching the suffix, "Already matched by the base rule {id}." or
  "Already matched by {account}'s rule {id}.". A line not shaped like a domain name reads "Not
  shaped like a domain name." and is the refused line that keeps Add rule disabled. An identifier
  the checks below refuse is refused here too, before anything is sent. The sentence above the
  button counts the checked suffixes together, so a sender two of them match counts once, rather
  than adding each line's numbers, which would count it twice.

  What opened the panel, the sender picker or Change where this applies…, is recorded in the
  browser's history entry, so a reload keeps it, and Cancel and `Escape` go Back to the picker
  exactly as the operator left it. A parameter in the address was the alternative, and the route
  table gives the panel's address none. Once the add succeeds the browser goes to the new rule,
  whose status region reads "Added {id}.".
- **Edit domains** that only adds suffixes saves at once. An edit that removes any suffix shows the
  change as a diff, `+` for each suffix added and `−` for each removed, and saving goes through the
  lift dialog for the removed ones. An edit that would remove every suffix is not an edit, since a
  rule needs one, and the screen offers Lift restriction instead.

  The suffixes are edited one per line in one text area, each checked for its shape before Save
  enables, with no match count, since the rule's screen above it already shows each suffix's
  numbers. The lift dialog's numbers are read for the removed suffixes together, what keeping
  only the rule's other suffixes would release in the account, so an edit removing several suffixes
  states one sentence for the set, "restricted by these suffixes", and a sender that only the set
  covers is counted. Adding each suffix's own numbers was the alternative, and it would miss a
  sender two removed suffixes both cover. After a save the screen stays on the rule, whose status region
  reads "Added {suffixes} to {rule}." or, after a removal, "Lifted {suffixes}. Put it back".
- **The lift dialog** is titled "Lift the restriction on {suffix or rule}". It states "In {account},
  {n} senders and {m} stored messages are restricted by this {rule or suffix} and by no other rule.
  From each process's next policy reload, their bodies are no longer denied for their sender. A
  message scanned before the rule existed, or skipped by the scan gate, can then be released to the
  agent at once. A message held as restricted goes back to pending scan when a scanning workload
  next compares the index with the policy, and is released once a scan finds no code or link in
  it. A body already released cannot be recalled." (ADR-0002, ADR-0037). The counts are the senders the account's policy without the change would no longer
  restrict. For a base rule it adds "This is a base rule. These counts
  are {account}'s. Each account's policy screen shows its own.", then "Every account loses it:
  {identifiers}.", the account identifiers alone with no count, and asks for the rule identifier
  typed before the button enables. Focus starts on Cancel. The button names the effect, "Lift
  restriction on {suffix or rule}", outlined in the restricted color.
- **Putting a lift back.** After any lift, the screen the operator lands on shows "Lifted {suffix
  or rule}. Put it back", which adds back exactly what was lifted, under the same scope, with no
  confirmation, since it only adds restriction. Bodies released before it lands stay released, so
  the sooner it is put back the less is exposed. A history row whose action is lifted, or an edit
  that removed suffixes, carries "Restore", which opens Add a rule
  prefilled with the removed suffixes, the row's scope and its identifier
  (`/{account}/policy/new?suffix=…&scope=…&id=…`), or Edit domains with the suffixes added when the
  rule still exists.

  Put it back re-sends the writes that add. A rule lifted whole is added again with its suffixes,
  under its identifier and scope, and a removal of suffixes is undone by an edit adding them back
  onto the suffixes the rule holds after the removal. An import's Put back sends one such write per
  rule it lifted and per rule it took suffixes from, in order, and stops at the first refusal,
  which it shows. A request of its own for undoing a lift was the alternative. The add and edit
  requests already carry exactly this, with their checks and history rows, so a new request would
  only duplicate them. After a rule is lifted whole the browser goes to the scope's policy screen,
  since the rule's screen no longer has a rule to show. Each write's outcome is announced in the
  status region of the policy screen it belongs to, the account's or the base policy's, and stays
  there while the tab is open until the next write from that screen replaces it, since a write
  often ends on another screen than the one it was made from.
- **Change where this applies…** moves a rule between scopes, offering "Only {account}" on a base
  rule and "Every account" on an overlay rule. It opens Add a rule prefilled with the rule's
  suffixes, the other scope and the rule's own identifier, which the other scope may also hold.
  Once the add succeeds, it continues to
  the old rule's lift dialog, whose counts are then 0 in the account in view, and which reads
  "Nothing in {account} loses its restriction, since {new rule} covers every suffix." A base rule's
  lift still asks for the identifier typed, since other accounts lose it. The move is two
  transactions, the add and then the lift, each with its history row, and the add always comes
  first, so the account in view never goes without the restriction between the two writes.

A rule added restricts the classes the index has already stored once a scanning workload next
compares the index with the policy, as a lift reaches the delisting transition (ADR-0113,
ADR-0037). The screen claims only what fetch-time re-evaluation guarantees, that bodies are denied
from the next policy reload (ADR-0002), and shows the stored classes catching up through "index
updated".

**Restrict senders from the index.** `/{account}/policy/pick?search=…` is the `senders` dataset at
L3 with its `search` filter, the sender row's fields, a selection box at the left, and for a
restricted sender the rule restricting it. The row cursor and the selection are separate. `x`
toggles the row under the cursor, and so does `Enter`, which here never opens the row, so a key
cannot leave the picker and drop the selection. Opening the corpus stays on the domain's link.
`Shift` with `j` or `k` extends, `Ctrl` or `Cmd` with `a` selects every sender the search matches
across its pages, and `Escape` clears the selection before it closes anything. The selection rides
in the address as `pick=` with the selected domains, or `pick=all` for every match, so Back and a
reload keep it. A restricted sender cannot be selected. A bar at the bottom of the table appears
on the first selection, reading "{n} senders · {m} messages selected", or "all {n} matching
senders" when the selection spans pages, and offers Restrict as one rule, on `r` too, which opens
Add a rule at `/{account}/policy/new?suffix=…` with the selected domains in the address as its
suffixes, so the hand-off is a URL like every other view. A selection of every match hands over
the search instead, as `?search=…`, and the panel lists the domains it matches. The selection makes
one rule because a rule names one institution (ADR-0004), and a search for an institution's name
finds its domains. The match count is announced as the search narrows. A search with no match
reads "No senders match '{search}'".

A sender is restricted, its box disabled, when its stored class reads restricted or when a rule of
the account's policy already restricts it before its stored class catches up, since a second rule
would add nothing. Its row names that rule, linking to the account's policy searched for it, and
carries no "Restrict {domain}…", which a sender no rule restricts carries. Picks made one at a time
are of the page in view, so another page or another search starts with nothing picked, and a
selection spans pages only as every match. The bar counts the messages of the senders picked from
the rows in view, so keeping picks across pages was the alternative, and the bar could then not
count their messages without a read per pick. With every match selected, clearing one row's box
leaves the page's other senders picked. The bar is the live region, its text read as "{n} senders,
{m} messages selected", or "Selected all {n} matching senders", which announces the count
`Ctrl` or `Cmd` with `a` selected. Add a rule handed every match reads every page of the search
and fills one line per sender no rule restricts.

**History.** `/{account}/policy/history` is the `policy_changes` dataset, the base policy's
changes and the account's own, newest first, at L3 by default with the range of
[section 5](#5-information-architecture-and-the-url), groupable by action, scope and identity. Its L0
figures are the changes in range, those that added restriction and those that lifted it. A lifted
rule no longer exists, so its identifier renders without a link, and its row carries Restore.

**Import and export.** One scope's rules are exported to a file, and a file is imported into one
scope (ADR-0110). The account's policy screen exports and imports the account's own rules, and the
base policy screen ([section 8.14](#814-base-policy)) the base rules. A file names no scope, so a
file exported from any scope imports into any other, the base policy or any account. No file holds
two accounts' policies, so the per-account rule of [section 1](#1-what-the-ui-is-for) holds. An
import makes the scope's stored rules equal to the file, after a preview of every change and one
confirmation of every restriction it lifts.

- **The file** is ADR-0004's form, YAML, rules sorted by identifier, each with its identifier, its
  domain suffixes and its class, and nothing else. Its source, its created time and its history
  stay in the database.

  ```yaml
  rules:
    - id: operator.schwab.com
      domain_suffix: [schwab.com, schwabmail.com]
      class: restricted
  ```

- **Export** downloads the scope's rules as they are stored, named `policy-{account}.yaml` or
  `policy-base.yaml`. It asks nothing and changes nothing.
- **Import** is the screen `/{account}/policy/import`, titled "Import {account}'s rules" with "The
  file replaces {account}'s own rules. The base rules are not touched.", or `/setup/policy/import`,
  titled "Import the base policy" with "The file replaces the base rules, which every account
  inherits." The operator drops the file or chooses it, and the server reads it and checks every
  rule with the checks of a write (What a write may hold, above) before anything is shown. A file is
  refused whole, naming each problem with its rule and line, when it is not this form, when a rule
  fails a check, when two rules share an identifier, or when it is over 1 MB. An identifier is
  unique within its scope only, so a file's identifiers never collide with another scope's rules,
  and a rule in the file whose identifier the scope holds is that rule, edited to the file's
  suffixes.
- **The preview** lists the difference between the stored rules and the file, in four groups,
  each with its count. Rules added, suffixes added to a rule, suffixes removed from a rule, and rules
  lifted, which are stored and missing from the file. Unchanged rules show as a count. The lifted
  and removed groups come first, in the restricted color, since they are what the import
  releases. For the account's scope each lifted rule and removed suffix carries the account's
  numbers as the lift dialog states them, the senders and stored messages no other rule restricts.
  For the base scope it carries none, as on [section 8.14](#814-base-policy). A file equal to the
  stored rules reads "The file matches the stored rules. Nothing to import." with no button.
- **Confirming.** {k} counts the restrictions an import lifts, each rule lifted and each suffix
  removed from a rule counting one. With nothing lifted, "Import {n} changes" applies at once,
  since it only adds restriction. With anything lifted, the button reads "Import, lifting {k}
  restrictions" and opens one dialog for every lift, an `alertdialog` titled "This import lifts {k}
  restrictions". It lists each rule and suffix lifted, then states for the account's scope "In
  {account}, {n} senders and {m} stored messages are restricted by what this import lifts and by
  no other rule.", followed by the lift dialog's sentences on what is released and what cannot be
  recalled, or for the base scope "Every account loses these restrictions: {identifiers}." and the
  same sentences. Its button reads "Import and lift {k}". For the base scope, whose lifts every
  account loses, the operator types `lift {k}` before the button enables, and while disabled it
  says "Type lift {k} to enable". For an account's scope the button needs no typing, as a single
  overlay lift needs none. Focus starts on Cancel. Either way the request carries `lift {k}`, so the
  server applies only an import whose confirmation covers the count it lifts.
- **Applying.** The whole import is one transaction, each change with its history row, added,
  edited or lifted, as any write writes it (ADR-0102), so an import that fails leaves neither rules
  nor history. The preview carries the scope's stored rules it was computed against, and the
  import compares them with the scope's rules read in its own transaction, so an import whose scope
  changed since is refused with "The policy changed since this preview. Preview again."
  A rule lifted by an import reaches the delisting transition as any lift does (ADR-0037).
- **After an import** the browser returns to the scope's policy screen, which reads "Imported: {a}
  rules added, {e} rules edited, {k} restrictions lifted.", each count taking the singular for one,
  as every count line of this section does, and, when anything was lifted, "Put back
  the {k} lifted restrictions", which adds back exactly what the
  import lifted with no confirmation, as Put it back does for one lift.

  A file is read and previewed as soon as it is chosen or dropped, and another file is previewed
  afresh. {n} in "Import {n} changes" counts the rules added and the rules edited, one each, as the
  preview groups them. A base import with no account connected says "No account is connected, so
  nothing is released now." in place of the accounts that lose its restrictions, as a base lift
  does on [section 8.14](#814-base-policy). Export is a link the browser downloads, named by the
  server.

**Accessibility of the policy writes.** What the accessibility requirement of
[section 16](#16-framework-requirements) means for these screens:

| Element | Contract |
| --- | --- |
| Lift dialog | an `alertdialog`, labelled by its title and described by its consequence sentence and, for a base rule, its base sentences. Focus is trapped in it, starts on Cancel and returns to the control that opened it. The typed field states the expected value in its label, accepts paste, and compares after trimming surrounding space. The disabled Lift button says why, "Type the identifier to enable" |
| Add a rule panel | the page behind it inert, as behind a row detail. Focus starts on Applies to, or on the Add button when the panel opens with suffixes from a hand-off. Each suffix line's verdict and match count is tied to its line as its description and announced politely once typing pauses, not on every key |
| Sender picker | each selection box a checkbox labelled "Select {domain}". A restricted sender's box is disabled, described by "Already restricted by {rule}". The selection bar is a polite live region announcing "{n} senders, {m} messages selected", and the match count is announced as the search narrows |
| Writes | each save, an add, an edit, a lift or the rate target of [section 8.13](#813-account-settings), announces its outcome in a status region, and a refusal moves focus to the refusal's text |
| Selecting every match | `Ctrl` or `Cmd` with `a`, which selects rows on pages not in view, announces the count it selected |
| Import | the file drop is also a file input labelled "Choose a policy file". The preview's groups are headings with their counts, read lifted first. The import dialog follows the lift dialog's contract, its typed field, on a base import, labelled with the expected `lift {k}` |

### 8.8 System

`/{account}/system`. The account's identifier and provider, then, from the system endpoint's
operational block, the values ADR-0034 exposes to clients, one to one. Backfill pass 1 and pass 2
flags with progress where a pass runs, sync cursor age and last successful tick, scan backlog
depth, rate controller state (current of target, cap, backoff wording, last throttle), and the last
provider authentication outcome with its time. Each links where Home's operational rows link, once
the screen it links to exists, and the rate row also links to the rate target it is changed at.

The as-of time comes first, as in Home's System column, so a tab left open is visibly stale. Then
one row per value, in this order.

| Row | Value | Links to |
| --- | --- | --- |
| account | the identifier in mono and the provider, and "not connected" when the account has no state row, which ADR-0091 reads as not connected | nothing |
| backfill pass 1 | "complete", or while the pass's latest run is running "running, page {page} of {of} ({share})" with the progress bar of [section 7.3](#73-charts), else "not started" while the pass has no run, else "not complete" | Jobs |
| backfill pass 2 | as pass 1 | the corpus lens with `scan_state=pending`, once that lens exists |
| sync cursor age | the age, with the UTC time on hover, or "no cursor yet" | Jobs |
| last successful tick | the UTC time, or "none yet" | Jobs |
| scan backlog | "{count} messages pending scan" | the corpus lens with `scan_state=pending`, once that lens exists |
| rate | "{current} of {target} units/s, cap {cap} units/s", or "no rate state yet" for an account that has never spent | Jobs, and the rate target of Account settings ([section 8.13](#813-account-settings)), where the target is changed |
| backoff | the backoff wording of [section 11](#11-rendering-and-formatting-rules). A `backoff_until` already past reads "not in backoff" | Jobs |
| last throttle | the UTC time, or "never" | Jobs |
| last authentication | the outcome as recorded, then the UTC time, or "none recorded" | Account settings ([section 8.13](#813-account-settings)), where a refused credential is re-authorized |

A row whose screen does not exist yet renders its value unlinked and gains the link when that
screen lands, since the UI links only to screens that exist. Linking ahead of the screen was the
alternative, and it would send the operator to a page that says no screen is there. The scan
backlog links where Home's pass 2 row does, the one Home row that carries the pending count. The
authentication outcome is text a deployable records from what its provider adapter reports, so it
renders inert like message-derived text ([section 11](#11-rendering-and-formatting-rules)).

The screen is not a live surface. It reads the system endpoint through the same cached read as the
chrome's partial-index banner, so a page load sends one request, and while the banner follows the
stream ([section 9](#9-live-surfaces)) the screen's values move with it. Reading the endpoint a
second time for the screen was the alternative, and it would send two requests for one answer. A
failed or slow read shows the region patterns of
[section 12](#12-empty-loading-partial-and-error-patterns).

### 8.9 Plans

`/{account}/plans`. The `plans` dataset at L3, one row per plan, sorted by created time newest
first, 50 per page, with a status filter defaulting to every status. Home shows only plans in
DRAFT. This screen is where APPLIED, ROLLED_BACK, REJECTED, and APPLY_REFUSED history lives.

| Column | Content |
| --- | --- |
| plan | the title, linking to the plan reviewer, and the short plan id |
| status | the wording of [section 11](#11-rendering-and-formatting-rules) as a labeled mark |
| messages | the message count and the share of corpus |
| created | UTC time, and the proposer |
| decided | the approve or reject time and identity, or blank in DRAFT |
| outcome | applied with its run, rolled back with its run, refused with its reason, or blank |

The L0 strip counts plans by status. A row opens the plan reviewer at its Summary section.

### 8.10 Installation

`/setup`, the first of the installation screens, which hold what belongs to no account (ADR-0056).
They are this screen, OAuth client setup ([section 8.11](#811-oauth-client-setup)), connecting an
account ([section 8.12](#812-connect-an-account-and-re-authorize)) and the base policy
([section 8.14](#814-base-policy)). Of what is stored, an installation screen shows only three
kinds of record, which belong to no account's own rows. They are each OAuth client's name,
provider, client identifier and project ID, each account's identifier, provider and the client it
connects through (ADR-0016, ADR-0091, ADR-0106), and the base policy's rules and their history
(ADR-0004, ADR-0102). It reads and shows no stored account's state, counts or rows, so nothing
aggregates across accounts, and an account's health is on its own Account settings
([section 8.13](#813-account-settings)). The one account whose values an installation screen
handles is the one being connected, whose identifier, mailbox and rate target the operator types,
and connecting writes that account's rows alone, in a transaction set to it. Fed by the
installation endpoint ([section 17.4](#174-the-bespoke-endpoints)). Not live.

A title, "Installation", and one sentence saying what lives here. Then three regions.

| Region | Content |
| --- | --- |
| Getting started | shown until an account exists. First a "Before you start" block. The whole setup takes about 15 minutes. It needs a Google account allowed to create Cloud projects, which need not be the mailbox, and the Gmail address to connect. Chrome or Edge keeps the guide on top of Google's console. Google warns that the app is unverified, which is expected. Then three items in order, "Set up a Gmail OAuth client", "Review the base policy" and "Connect an account", each with its status as text and a glyph, done (a filled check), not started (an open circle), optional (an open circle in muted text), or cannot start yet (a dashed circle in faint text, with no link). The base policy item is optional and never blocks the next, so it is never the current item. Its status reads "{n} base rules · optional", "1 base rule · optional" for one, or "No base rules yet · optional", and its title links to the base policy, since a rule in place before the first account connects classifies that account's senders from its first backfill. Connecting reads cannot start yet until a client is set up. The current item carries its action |
| OAuth clients | one row per client, grouped by provider, each with its name in mono, the client identifier in mono, shortened with the full value on hover, the project ID in mono when one is stored, and the accounts connected through it, their identifiers in mono, or "no account yet". Each row carries "Open in Google Cloud console", which opens the console's clients page for its project, Replace, linking to its setup, and Remove. Remove is enabled only while no account connects through the client, and otherwise says why, "{k} accounts connect through this client. Move them to another client from their settings first.", or for one account "1 account connects through this client. Move it to another client from its settings first." It asks a confirmation, "Remove the client {name}? Its sealed secret is deleted here. The client itself stays in Google Cloud console, where you can delete it.", and removes nothing else. Under each provider's rows, "Add a {provider} client" opens a new client's setup. A provider with no client shows its line "No {provider} client yet" with the same action |
| Accounts | one row per account, the identifier in mono linking to the account's Home, the provider, the client it connects through in mono, and a "settings" link to its Account settings, then Connect an account, disabled with its reason while no provider an account can use has a client. While Getting started shows, this region carries no Connect control of its own, since Getting started holds it. With no account, one line, "No account is connected yet." |

Whether accounts share one client or each has its own is the operator's choice
(ADR-0106), and the screen says so once above the clients, "Accounts can share a client, or each use
their own. A client set up in an organization's own Cloud project may choose the Internal
audience, and then serves only that organization's mailboxes." Nothing recommends one over the
other.

`/` sends the browser here when no account exists ([section 5](#5-information-architecture-and-the-url)).

### 8.11 OAuth client setup

`/setup/{provider}/new` sets up a new client, and `/setup/{provider}/{client}` shows the client
named `{client}`, for a provider that authenticates through an OAuth client, Gmail today (ADR-0106,
ADR-0107). Each client is set up the same way, the first or any later one. Everything is done in
Google Cloud console, with no command-line tool, and the UI carries the instructions for every
step, so the operator never needs a guide from anywhere else. Not live.

**The problem the layout answers.** The work happens in Google's tab, while the instructions live
in the UI's. Instructions in full take more space than a step list can give, and an operator
switching tabs to read the next click loses their place in both. So every step has two forms. A
collapsed row of one line, and its full instructions, shown for one step at a time. The full
instructions also go where the operator is looking: beside or over Google's tab.

**The page.** A rail of the seven steps on the left, each with its status, and the steps as rows on
the right, every step reachable in any order. The step in view is expanded and the others are one
line each, its title and status. Opening a step, from the rail or its row, expands it and collapses
the one before. The page opens on the first step not marked done.

**A step expanded** holds, in order:

1. One sentence of why, where the step needs one.
2. "Open in Google Cloud console", which opens the step's console page and keeps this step
   expanded. Every console link of every step opens in one named console tab, so the steps reuse
   one tab rather than opening one each. From step 2 on, the link names the operator's project and
   the step's first action is to check that the project picker at the top of the console shows it,
   since a link may open in whichever project the console last had selected.
3. Its instructions, a numbered list of at most seven actions, each naming the button or field as
   Google labels it, in bold. A value to enter appears in the action as a mono chip with a copy
   control.
4. "Done when you see", one line naming what Google's page shows once the step is complete.
5. "Looks different?", collapsed, giving the console menu path to the same page, since Google moves
   and renames its console pages and its deep links are not documented as stable, and the one value
   that matters on the step.
6. "Done, open step {n+1}", which marks the step, opens the next step's console page in the console
   tab and expands that step, and beside it a plain "Mark done", which marks the step and expands
   the next without opening anything.

**The guide beside Google's tab.** The page offers once, at its top, to put the instructions where
the operator works, "Guide me on top of Google's console", and each expanded step offers it again.

| Browser | Control | What opens |
| --- | --- | --- |
| one that supports Document Picture-in-Picture, as Chrome and Edge do | "Keep the steps on top" | a small window, about 340 by 520 pixels, that stays above every other window, Google's tab included. It shows the current step expanded, its "Open in Google Cloud console", its copy controls, "Done, open step {n+1}", Mark done and Back, and moves to the next step when one is marked done. Step 7's file drop and paste work inside it |
| any other | "Open the steps in a side window" | a narrow window of the same page in a compact layout, opened sized and placed at the right edge of the screen where the browser allows, with the same controls as the small window |

When the browser blocks the side window, the page says "Your browser blocked the window. Allow
pop-ups for this site, or keep using this tab."

The UI's tab follows the guide. A step marked done in either window opens the next step in both,
and closing the small window leaves the page on the step the guide was at. The step in view rides
in the address as `?step=N`, and the side window is the same address with `&guide=side`. The guide
window holds only this page's own text, the step, its values and its controls, and nothing about
any account. It loads the bundle's stylesheet by a link, since the content security policy refuses
inline styles ([section 15](#15-security-of-the-ui-itself)).

**The steps.** The labels and pages below are taken from Google's documentation, and the guide's
wording is checked step by step against the live console when it is built. Where the console's
answer is not documented, the done-state names what holds however the page draws it. The project
identifier captured in step 1 fills the project of every later link.

| Step | Console page (menu path) | Instructions | Done when you see |
| --- | --- | --- | --- |
| 1 Create the project | New project (Menu › IAM & Admin › Create a project) | Use any Google account you own. It does not need to be the mailbox you will connect. Enter **Project name** `mediated-mailbox`. Leave **Location** as **No organization** for a personal account. Press **Create**. Copy the **Project ID** shown under the name, and paste it into the field this step holds. If the console stays in another project, choose the new one in the project picker at the top | the new project's name in the project picker, and its ID pasted here |
| 2 Enable the Gmail API | Gmail API in the API Library (Menu › APIs & Services › Library, search "Gmail API") | Check the project picker shows this project. Press **Enable** | the Gmail API's page no longer offering **Enable** |
| 3 Set up Google Auth Platform | Google Auth Platform (Menu › Google Auth Platform › Overview) | Check the project picker. Press **Get started**. Under **App Information** enter **App name** `mediated mailbox` and choose your address as **User support email**, then **Next**. Under **Audience** choose **External**, then **Next**. Under **Contact Information** enter your address, then **Next**. Under **Finish** tick the agreement to Google's user data policy, then **Continue** and **Create** | Branding, Audience, Data Access and Clients reachable in Google Auth Platform's menu |
| 4 Add the Gmail scope | Data Access (Menu › Google Auth Platform › Data Access) | Check the project picker. Press **Add or remove scopes**. Filter by `gmail.modify` and tick `https://www.googleapis.com/auth/gmail.modify`, or, only once step 2 is done, paste it into the box for adding scopes by hand. Press **Update**, and save the page if it offers to. Add no other scope | `gmail.modify` listed among the project's scopes |
| 5 Publish to production | Audience (Menu › Google Auth Platform › Audience) | Check the project picker. Under **Publishing status**, press **Publish app**, and confirm. Google then marks the app as needing verification, which this client does not need, so leave it | **Publishing status** reading **In production** |
| 6 Create the desktop client | Clients (Menu › Google Auth Platform › Clients) | Check the project picker. Press **Create client**. Choose **Application type** **Desktop app**. Enter **Name** `mediated mailbox`. Press **Create**. In the dialog, press **Download JSON**, or copy the **Client ID** and **Client secret** before closing it | the dialog "OAuth client created" |
| 7 Bring the client back | none | Drop or choose the downloaded file, or paste the client ID and the client secret | Google's answer below |

Each step carries its warnings in the restricted color with a glyph where the step can go quietly
wrong.

- **Step 2.** Google lists the Gmail scope in step 4 only once the API is enabled.
- **Step 3.** Internal is offered only to an organization, and a personal account chooses External.
  "Internal works only if every mailbox that connects through this client is in this organization.
  External always works."
- **Step 4.** The scope lets the system label and move mail and cannot permanently delete anything.
  Google marks it restricted, which is expected.
- **Step 5.** A client left in Testing has every grant expire after 7 days, so each account is
  refused a week after it connects, and nothing here can see the setting. Google shows each consent
  an unverified-app warning, which is expected for a client only its owner uses, and an unverified
  app may have at most 100 users, far more than one installation connects. "Google may email you
  asking to verify the app. Nothing needs doing."
- **Step 6.** Choose Desktop app, never Web application. Google shows the secret only in that
  dialog, and the downloaded file keeps it. A secret lost before it is brought back is replaced
  from the client's page by **Add secret**. "Google deletes a client unused for 6 months, after
  emailing the project's contacts 30 days before."

**The project ID.** Step 1's field takes the project ID, 6 to 30 lowercase letters, digits and
hyphens, starting with a letter. An all-digit value is refused with "That is the project number",
and a value with spaces or capitals with "That is the project name". With no mark in this browser,
step 1 opens with "Started before, in another browser? Paste the project ID, then check each step's
'Done when you see' in the console." For a client already set up, its stored project ID fills the
field. A new client may use the project of a client already set up, and step 1 then offers "Use the
project of {client}", which fills the field and marks steps 1 to 5 done, since they belong to the
project, leaving steps 6 and 7.

**Bringing the client back.** Step 7 takes the file Google offers at step 6, dropped on the step
or chosen with a file control. The file is read in the browser for its client identifier and
secret, the `installed` entry of a desktop client's file, and a file of any other kind is refused
with a line naming what it holds, so a web application's client is caught before it is sent. Or
the operator pastes the client identifier, and the client secret into a password field with a
show control. Paste is never blocked. Step 7 accepts a file or a paste whatever the marks of steps
1 to 6 say. When the file names a project and step 1 holds a project ID that differs, the step says
so before saving, "This client belongs to project {file's project}, but steps 2 to 5 were done in
{step 1's project}. Check the Gmail API and the publishing status in {file's project}." A new
client is also named at step 7, in mono, the project ID proposed as its name and editable. The
name is how every screen and every account refers to the client, and it cannot change once saved.
It is refused when another client holds it, when it is `new`, or when it is not 6 to 30 lowercase
letters, digits and hyphens starting with a letter, the shape of a project ID, so it is always one
path segment. "Check with Google and save" sends the name, the identifier, the
secret and the project ID, taken from step 1 or else the file, and the answer shows in a status
region beside the button. A client identifier another client of the provider already holds is
refused with "This client is already set up as {name}", linking to it.

Steps 1 to 6 carry a mark the operator sets, done or not done, because nothing here can check
them. The marks are kept per browser and per client, a new client's under `new` until it is
saved, as the theme override is kept, and nothing about them is stored by the server. The project
ID is kept the same way until the client is saved, and is then stored with the client
([ADR-0016](./adr/data/0016-schema.md)). Step 7's mark is Google's answer.

| Answer | Shown | Stored |
| --- | --- | --- |
| accepted | "Google accepted this client. Saved as {name}, its secret sealed." The primary button is then "Connect your first account", or "Connect an account through {name}" once an account exists, and goes to [section 8.12](#812-connect-an-account-and-re-authorize) with this client chosen | the name, the provider, the client identifier, the project ID and the secret sealed (ADR-0081) |
| refused | "Google does not recognise this client identifier and secret. Nothing was saved.", with its causes, first "A client created in the last few minutes may not be active yet. Wait five minutes and press Check again.", then a space copied with the secret and a client of another type | nothing |
| no answer | "Could not reach Google. Nothing was saved. Try again." | nothing |

**A client already set up.** The screen opens with "{name} · client {client_id}" and the accounts
connected through it, and step 7 reads Replace {name}, with "Lost the secret? Add a new one to
the same client" above the Replace button. When the client identifier brought back equals the
stored one, the action is "Update the secret". It asks no confirmation and says "Every account on
{name} keeps its grant. Disable the old secret in the console once this is saved." Only when the
identifier differs does replacing ask for confirmation, with the sentence "The grants of the
accounts on {name} were issued to its current client. After replacing it, each of these {n}
accounts is refused at its next token refresh and needs re-authorizing: {identifiers}.", {n} and
the identifiers read from the accounts list, and with none on the client, "No account connects
through {name}." Accounts on other clients are untouched, and the dialog says so when any exist.
After such a replace, the success line lists each affected account's identifier with its
Re-authorize link ([section 8.12](#812-connect-an-account-and-re-authorize)). The secret is never
read back or shown once saved.

### 8.12 Connect an account and re-authorize

`/setup/connect`, an installation screen, and `/{account}/account/reauthorize`, an account's
screen, are one page of three steps, each enabled once the step above is done. Nothing is stored
until the third succeeds (ADR-0091). Not live.

1. **Name the account.** The mailbox address being connected comes first. The identifier, in mono,
   is then proposed from the mailbox's local part, lowercased, with characters an identifier cannot
   hold replaced and a suffix added when an account already holds it, and the operator may edit it.
   Then "Connect through", the client the consent is issued to (ADR-0106). With one client set up,
   it shows as a fact, "Gmail, through {name}", with "Set up another client" beside it. With
   several, it is a list of the clients by provider, each with its name in mono, its project ID
   and the accounts already on it, nothing preselected unless the address carries `client=`, and the
   step does not confirm until one is chosen. The list closes with "Set up another client", which
   opens a new client's setup ([section 8.11](#811-oauth-client-setup)). The provider is the chosen
   client's. A collapsed "Lower this account's rate target" holds the field of
   [section 8.13](#813-account-settings). The identifier is refused when it is exactly `.`, `..` or
   `/` (ADR-0087), when it equals a word the UI's own top-level paths use, which is the router's
   `setup`, the read API's `api`, and the name of every file or directory at the top of the built
   bundle, such as `main.js` and `main.css`, or when an account already holds it. The page
   says the identifier cannot change once connected, since every record of the account is keyed on
   it, and that the mailbox is remembered. Confirming the step starts the consent attempt, so an
   identifier refusal shows here, before anything happens at Google.
2. **Grant access at Google.** One sentence of what Google asks next: to pick the account, past its
   unverified-app warning by Advanced and the link to continue, which is expected, and to allow
   Gmail access, leaving the Gmail permission ticked. The same sentence, never collapsed, adds "If
   Google's warning says the app is being tested, stop: the client is still in Testing. Publish it
   (setup step 5) first, or this account is refused in 7 days." and then "Google may email a security
   alert that mediated mailbox was granted access. That is this connection." Then a picture of what
   the browser shows at the end, a browser address bar reading the redirect address followed by
   `?state=…&code=…` over a page that cannot be reached, captioned "Your
   browser cannot open this page. That is expected. If a page loads instead, something on your own
   computer answered; the address bar still works." The control "Open Google's consent page" is a
   plain link to the consent address the server returned when step 1 was confirmed, opening a new
   tab, so opening it is an ordinary navigation that no pop-up blocker stops. The consent request
   names the step 1 mailbox as Google's login hint, so Google offers that account first. Its
   redirect address is the loopback address the configuration key `consent_redirect` names
   ([section 18.1](#181-the-configuration-the-ui-declares)), never one written into the code, and
   the code exchange, the check of the pasted address and the picture all read that one value. Its
   default names `127.0.0.1` on a high port, 47823, rather than port 80, so a web server on the
   operator's own computer is unlikely to answer, and an installation whose operators' computers
   answer on that port names another. The attempt shows its age and the time left.
   An attempt lasts 15 minutes, this design's starting value.
3. **Paste the address** from "the tab that says it can't reach the site". One field takes the
   whole address and reads its parts itself, with "Paste from clipboard" beside it, and plain paste
   still works. When the UI's tab becomes visible again after the consent page was opened, focus
   moves to the field. Under it, before anything is sent, a line names what it found, the redirect
   address's host and port, the state matching this attempt and a code present. The button reads "Connect
   {identifier}", or "Re-authorize {identifier}", and `Enter` in the field submits.

Step 2 holds a collapsed "Google showed an error instead?", for the mistakes that stop at a page of
Google's and never reach the redirect address. Each line is worded on the code Google prints on its page.

| Google shows | Cause | Fix shown |
| --- | --- | --- |
| `redirect_uri_mismatch` | the client is a Web application | create a Desktop app client at OAuth client setup step 6, then Replace the account's client ([section 8.11](#811-oauth-client-setup)) |
| `deleted_client` or `invalid_client` | the client was deleted, or its secret disabled | restore it within 30 days from the console's deleted credentials, or set the client up again |
| `access_denied`, naming testing or verification | the app is in Testing and this mailbox is not a test user | publish the app at OAuth client setup step 5 |
| `admin_policy_enforced`, or an organization's block | an organization's administrator blocks the app | ask the administrator to trust the client identifier, shown in mono with a copy control, or connect this mailbox through a client set up in the organization's own Cloud project, through "Set up another client" |
| anything else | | open the consent page again, and if Google shows the same page, check each step of OAuth client setup |

"Keep the paste box on top", offered beside step 3, opens a guide window as
[section 8.11](#811-oauth-client-setup)'s does, holding only the paste field and its three checks.
Like that section's guide window, it holds nothing about any account, the identifier and the
mailbox included.

An attempt is held by the server for the session that started it, in a cookie only the server can
open, sealed under a key derived from the request token's and bound to the session (ADR-0111). It
survives which replica answers and a reload of the page, which restores the identifier, the mailbox
and the countdown. A session holds one attempt, and a newer one replaces it, so a tab whose attempt
was replaced shows, when it next has focus, that its attempt was replaced and offers to start again.
The tab tells by comparing its attempt with the one the server reads back. A refusal leaves the
attempt in place until it expires, so opening the consent page again and pasting the new address
finishes it, and a success ends it.

The mailbox Google granted is compared with the one named ignoring case, and for `gmail.com` and
`googlemail.com` ignoring dots and treating the two domains as one, as Google does.

A refusal names its cause and its fix in the status region under the field, and stores nothing.

| Cause | Wording |
| --- | --- |
| the operator declined at Google | "You declined at Google. Open the consent page again." |
| no code in the address | "This address carries no code. Copy the address of the page Google sent you to." |
| not the redirect address | "This is not the address Google sent you to. Copy it from the tab that says it can't reach the site." |
| another attempt's state | "This address belongs to another attempt. Use the latest tab, or start again." |
| no attempt in this session | "No connection is in progress in this browser session. The browser was closed or the UI restarted. Start again from step 1." |
| the attempt expired | "This attempt expired. Open the consent page again." |
| Google refused the code | "Google refused the code. It may already have been used. Open the consent page again." |
| Google did not answer | "Could not reach Google. Nothing was saved. Press Connect again. If Google then refuses the code, open the consent page again." |
| a grant without the Gmail scope | "Google granted no access to Gmail. Tick the Gmail permission on Google's page." |
| a grant holding any scope beside the Gmail scope | "Google granted more than the Gmail permission this system asks for, so nothing was saved. Open the consent page again." |
| the Gmail API not enabled in the client's project | "The Gmail API is not enabled in project {id}. Enable it (OAuth client setup step 2), wait a minute, then open the consent page again.", {id} the client's stored project ID, or "the client's project" when none is stored |
| a grant for another mailbox | "Google granted access to {granted}, not {named}. Sign in to {named} at Google." On a re-authorization it adds "If this mailbox's address changed, the account cannot be re-authorized. Connect it again under a new identifier." |
| an account took the identifier after step 1 | "An account named {identifier} was connected while this one was in progress. Nothing was saved. Start again from step 1 with another identifier." |
| the chosen client was replaced or removed after step 1 | "The client {name} changed while this was in progress. Nothing was saved. Start again from step 1." |

On success, connecting writes the account's two rows in one transaction (ADR-0091), the client it
connects through (ADR-0106), its credential sealed (ADR-0081), the mailbox the consent confirmed
(ADR-0080), and the code exchange's
authentication attempt as the account's latest (ADR-0097). The page reads "Connected
{identifier} ({mailbox})." and says what comes next, "Every workload is told of {identifier} and
serves it, and backfill starts indexing it. Nothing more needs doing." Each workload learns of the
account with no manual step through the signal between deployables of ROADMAP's unit
[F7](../ROADMAP.md#group-f--foundation). The page then adds "The base policy's {n} rules apply to
{identifier} from the start. Rules made for one account do not. Review {identifier}'s policy.",
linking to the account's policy ([section 8.7](#87-policy)), {n} the count of base rules, which
belongs to no account. It links to the account's Home.

**Re-authorizing** is the same page with step 1 as a read-only summary of the identifier, the
provider, the client the account connects through and the mailbox the account remembers. The grant
is checked against that mailbox and the mailbox is never asked for (ADR-0080, ADR-0107).

**Moving to another client.** Step 1's summary carries "Connect through another client", offered
when the provider has another client, which opens Connect through's list without the account's
current client and sets `client=`. The consent is then issued to the chosen client, and step 3's
button reads "Move {identifier} to {name}". Success writes the new client and the new credential
together in one transaction, since a grant works only with the client it was issued to (ADR-0106),
and reads "Moved {identifier} to {name}." A move that fails leaves the account on its old client
with its old credential.

A re-authorization's success, a move's included, replaces the stored credential, records the code
exchange's authentication attempt as the account's latest (ADR-0097), so a refused
credential's banner clears, and reads "Re-authorized {identifier}. Every workload is told of the
new credential and uses it from its next call." Each workload takes it up without waiting for a
refusal, through the signal of [F7](../ROADMAP.md#group-f--foundation). An account with no
mailbox remembered, one with no state row or one stored before the mailbox was kept, is connected
here too: step 1 then asks for the mailbox once, the grant is checked against it, and success
writes the state row, or the credential and the mailbox into it, so the mailbox the consent
confirmed is remembered from then on (ADR-0080).

### 8.13 Account settings

`/{account}/account`, reached from the account selector's menu and from the last authentication
rows of Home's System column and the System screen. The refused-credential banner of
[section 12](#12-empty-loading-partial-and-error-patterns) goes straight to re-authorization
([section 8.12](#812-connect-an-account-and-re-authorize)). Fed by the account endpoint
([section 17.4](#174-the-bespoke-endpoints)). Not live. The identifier in mono and the provider as
the title, the mailbox address under it, and "through {name}", the client the account connects
through in mono, linking to its setup ([section 8.11](#811-oauth-client-setup)), with "Move to
another client" when the provider has another, which opens the account's re-authorization with the
client list ([section 8.12](#812-connect-an-account-and-re-authorize)). Then three regions.

**Credential.** Its health is the latest authentication any workload, or the UI's own consent,
recorded (ADR-0097), as a badge and a sentence, with the time.

| Recorded | Badge | Sentence | Re-authorize |
| --- | --- | --- | --- |
| `succeeded` | accepted, in the ok color | "Gmail accepted the credential at {time}." | an outlined control |
| `refused` | refused, in the restricted color | "Gmail refused the credential. Every workload that calls Gmail fails for this account until it is re-authorized." with the usual causes, access revoked in the Google account, a password change, a client left in Testing, and the account's client, {name}, deleted or replaced, which links to the client's setup ([section 8.11](#811-oauth-client-setup)) | the primary control, in the action color |
| `failed` | no answer, in muted text | "The last attempt got no answer Gmail could read. Workloads retry on their own, so nothing is needed unless this persists. If this lasts past an hour, check that the deployment reaches Google's token endpoint." | an outlined control |
| none recorded | not used yet, in muted text | "No workload has authenticated yet." | an outlined control |
| no state row | not connected, in muted text | "This account is not connected." (ADR-0091) | Connect, in the action color, which opens the account's re-authorization with the mailbox asked for once ([section 8.12](#812-connect-an-account-and-re-authorize)) |

**Rate target.** "Default, half of {provider}'s declared ceiling", or "Lowered to {p}% of
{provider}'s declared ceiling", with the current target in units per second from the rate state
once the account has spent. A percent field, refused unless above 5% and at most 50%, the range
ADR-0024 allows, saved by its Save button, and Reset to default, which clears the lowered target.
While the operator types, the field shows the percent alone, with no preview in units per second.
The target in units per second shows only as the rate state reports it after a save, since the UI
does not compute ADR-0024's fractions. Each workload uses it from its next account reload.
Lowering asks no confirmation, since a lower target only slows the account's work.

**What this account's rows hold.** A two-column list of what the UI reads of the account's two
rows: identifier, provider, client, mailbox, whether it is connected, the last authentication,
the rate target, the two backfill passes' flags and when the sync cursor was written. It never shows
the credential, which the UI cannot read (ADR-0084). A link goes to System for the live view.

### 8.14 Base policy

`/setup/policy`, an installation screen ([section 8.10](#810-installation)), holds the base policy,
the rules every account inherits (ADR-0004). It needs no account to exist, so the base policy can
be written before the first account connects, and a rule in place then classifies that account's
senders from its first backfill. Base rules are also written from each account's policy screen
([section 8.7](#87-policy)), which shows what a rule matches in that account. This screen shows
nothing of any account but its identifier, so it carries no count of senders or messages. Fed by
the base policy endpoints ([section 17.4](#174-the-bespoke-endpoints)). Not live.

**The list.** A title, "Base policy", and the sentence "These rules restrict senders in every
account, those connected later included." followed by the accounts today, their identifiers in
mono, each linking to that account's policy screen, where its numbers are, or "No account is
connected yet." An L0 strip counts the base rules and shows the time of the latest change with a
link to the history. Above the table sit Add a base rule, History, Import and Export, which act on the base rules as
[section 8.7](#87-policy)'s Import and export states, and a search box over the
rule identifiers and the domain suffixes. The table's rows are the base rules sorted by rule id,
each with rule id, domain suffixes, source (operator, or the candidate it came from) and created
time and identity, each row opening its rule. With no rule, the table reads "No base rules yet. A
base rule names one institution's mail domains and restricts its senders in every account." with
Add a base rule.

**What a write may hold** is [section 8.7](#87-policy)'s, with the base policy's history in place
of the account's. An identifier is refused when it is empty, carries surrounding space, is taken
by a base rule, is `new`, `history` or `import`, is exactly `.` or `..`, or holds a `/`. An
account's rule may hold the same identifier as a base rule (ADR-0110). A suffix is refused when it is not shaped like a domain name. Every rule's
class is restricted. Each write is recorded in the policy history in the same transaction
(ADR-0102). This screen, the installation endpoint's count of base rules, and every base write
from an account's policy screen read and write the base policy in a base-policy transaction, which
names no account and reaches no account's rows (ADR-0112).

**Add a base rule** is the panel `/setup/policy/new?suffix=…`, 480 px over the list. The rule
identifier, prefilled `operator.{first suffix}` until the operator edits it, the domain suffixes one
per line, each checked for its shape as it is typed, and the class. A suffix that is itself a
public suffix carries "This is a public suffix. It restricts every sender under it." on its line,
which refuses nothing. The panel says "Match counts are per account. Each account's policy screen
shows what this would restrict there." The sentence above the button reads "Restricts these domains
in every account. From each process's next policy reload their bodies are denied." Adding asks no
confirmation, since it only adds restriction.

**A base rule.** `/setup/policy/{rule-id}` shows the rule's header (identifier in mono, the scope
`base`, class, source, created time and identity), its domain suffixes, and its history as policy
change rows. Two actions, Edit domains, and set apart from it on the right, Lift restriction. A
line under the header reads "Only for one account? Open that account's policy and use Change where
this applies…", since moving a rule to one account's overlay names an account.

- **Edit domains** that only adds suffixes saves at once. An edit that removes any suffix shows the
  diff and saves through the lift dialog for the removed ones, as in
  [section 8.7](#87-policy).
- **The lift dialog** is titled "Lift the restriction on {suffix or rule}". It states "Every
  account loses this restriction: {identifiers}." with each identifier linking to that account's
  policy screen, then "From each process's next policy reload, the senders it alone restricted no
  longer have their bodies denied. A message scanned before the rule existed, or skipped by the scan
  gate, can then be released to the agent at once. A message held as restricted goes back to
  pending scan when a scanning workload next compares the index with the policy, and is released
  once a scan finds no code or link in it. A body already released cannot be recalled." (ADR-0002,
  ADR-0037). With no account connected it states "No account is connected, so nothing is released
  now." It asks for the rule identifier typed before the button enables, focus starts on Cancel,
  and the button reads "Lift restriction on {suffix or rule}", outlined in the restricted color.
- **Putting a lift back** is [section 8.7](#87-policy)'s, "Lifted {suffix or rule}. Put it back",
  adding back exactly what was lifted with no confirmation.

**History.** `/setup/policy/history` lists the base policy's changes alone, newest first, as policy
change rows with the range of [section 5](#5-information-architecture-and-the-url), a lifted rule's
row carrying Restore, which opens Add a base rule prefilled with its suffixes and identifier.
Its ranges are the presets as links in the screen's body, writing `range=`, since the frame's range
control belongs to an account's screens ([section 6](#6-global-chrome)).

A base rule and Add a base rule are panels over the list, as on an account's policy screen. A base
rule's panel reads the base policy's list and its history narrowed to the rule over all time,
since no base policy read answers one rule. Add a base rule checks the lines typed, once typing
pauses, through the base policy's own match read, which names no account and counts nothing. It
answers each suffix's shape, whether it is a public suffix, and the base rules already matching it,
so a line reads "Not shaped like a domain name.", the public-suffix warning above, or "Already
matched by the base rule {id}.", and only the first refuses. Focus starts on the identifier, the
panel having no Applies to.
After a lift or an import the browser returns to this list, whose status region carries Put it back
as [section 8.7](#87-policy)'s does.

The keys and the accessibility contract of the policy writes in [section 8.7](#87-policy) hold
here, the lift dialog's base sentences being this section's.

## 9. Live surfaces

The home's running-work strip, Jobs, and a run while it or its resumer runs update without a
reload. The run screen follows the stream whenever it is open, as [section 8.4](#84-run) says.
Each shows the live indicator in the top bar, a dot in the ok color with "live · updated Ns ago",
and pauses its subscription while the tab is hidden, resuming and refetching when it becomes
visible. The indicator follows the stream of the account in view, so switching accounts shows the
new account's stream and none of the last one's. A dropped stream shows "live · reconnecting" and
the view keeps its last data.
After the fallback to polling of ADR-0058 the indicator reads "live · polling". The view polls at
the polling interval while the stream's reconnection backoff keeps doubling up to its ceiling, so
the view returns to the stream when it opens. Polling until the page reloads was the alternative,
and it would never recover the stream.

The partial-index banner of [section 12](#12-empty-loading-partial-and-error-patterns) also follows
the account's stream while it shows, and reads the system endpoint again on every backfill run event
and on every poll of the fallback, so its figures move with the pass. It is not a live surface and
shows no live indicator. Reading the system endpoint only on page load was the alternative, and it
would leave a pass's progress frozen on a screen left open.

A tab holds one stream connection per account, whichever surfaces follow it. Home's strip and the
banner, Jobs and the banner, and a run and the banner each share the account's one connection, which
opens when the first of them starts following and closes when the last stops. Each event reaches
every surface following the account. A reconnect or a poll of the fallback reads again what each
following surface reads, and a surface that has stopped following reads nothing more. Every surface
sees the connection's one status. One connection per surface was the alternative. Its case was that
each surface's connection lived and died with the surface alone. It was not chosen because every
connection is a poll of the database every two seconds on the server, so two surfaces on one screen
doubled that load for one answer, and under HTTP/1.1, which the dev loop's plain HTTP and a proxy
that downgrades both use, a browser holds at most six connections to one origin, so a few tabs open
during a backfill would leave no connection for the screens' reads. Sharing lives in the stream
client module the next paragraph names, so no surface changes shape.

The transport, the stream's content, and its cadence are ADR-0058's. In the browser only the stream
client module depends on that record, and on the server only the stream endpoint of
[section 17.4](#174-the-bespoke-endpoints) does. The screens read object shapes
([section 17.5](#175-the-streams-event-shape)) that do not change with the transport, so a
different transport replaces one module.

## 10. Decisions and their friction

The two decisions are rare and dangerous, so friction scales with blast radius. Approving restates
what is being approved in one sentence with the numbers. A plan at or over a quarter of the corpus
demands a typed restatement before Approve enables ([section 8.2](#82-plan-reviewer)). Rejecting
and dismissing are plain outlined controls with no confirmation. Confirming a candidate shows what
it will do in one sentence above the control ([section 8.6](#86-review-queue)). Decisions happen
only on the object's screen, never from a list row, so the sentence is always present. Every
request carries the status the screen shows, and a mismatch is refused as a conflict. Every
decision records who made it in the decided row's own columns, the identity coming from the
declared header or the configured operator name (ADR-0084). The plans list and the review queue
show decisions from those rows. No audit row is written for a decision. The audit log holds bodies
served or denied and mailbox mutations, and applying an approved plan is audited by the engine as
the mutation it is. Confirming a candidate inserts a policy rule, so it also writes that rule's row
in the policy history (ADR-0102).

The policy writes follow the same rule, and their direction decides their weight. Adding
restriction can only deny more, so it asks nothing. Lifting a restriction lets a sensitive
sender's bodies be released, at once for mail already scanned and after a scan for the rest, which
cannot be recalled, so it states its consequence with the account's numbers, and on a base rule,
which every account loses, asks for the rule's identifier typed ([section 8.7](#87-policy)). An import
that lifts anything asks once for every lift it makes, typed as `lift {k}` for the base scope as a
base lift is typed. The setups weigh what they replace.
Replacing a client's identifier breaks the grant of every account connected through that client
until each is re-authorized, so it asks for confirmation naming those accounts, while a new secret
for the same client breaks no grant and asks nothing ([section 8.11](#811-oauth-client-setup)).
Removing a client no account connects through deletes only its sealed secret, so it asks a plain
confirmation. Connecting, re-authorizing or moving an account to another client replaces nothing
the operator would miss, so it asks nothing beyond the consent itself.

## 11. Rendering and formatting rules

- Message-derived text (subjects, display names, addresses, labels, reasons, error summaries) is
  rendered as text, never as markup. Subjects are masked at rest but remain text an adversary
  wrote, and the operator's browser must not become the injection target. A rendering path that
  interprets markup for these fields is a defect, and its control is catalogued in
  [docs/VERIFICATIONS.md](./VERIFICATIONS.md).
- Never a body, never a snippet.
- Status and sensitivity never carry meaning by color alone. Every badge has a label. Text wears
  text colors, and a colored mark beside it carries identity.
- Every zoom step is answered from server-side aggregates. The corpus never ships to the browser.

**Formatting.**

| Value | Rule | Example |
| --- | --- | --- |
| counts | thousands separator, no abbreviation | 12,480 |
| shares | one decimal, percent sign | 14.8% |
| rates | one decimal with the unit | 3.1 of 5.0 units/s |
| durations | largest two units | 1h 10m · 6h 41m · 12s |
| ages | largest two units, relative, with the absolute UTC time on hover; an age against a maximum reads "{age} of {maximum}" | 1d 4h · 1d 4h of 14d |
| absolute times | UTC with the `Z` suffix, date and time, or time alone within a day already named | 2026-09-10 10:12Z · 10:12Z |
| plan identifiers | mono. UUIDs shown as their first six characters with an ellipsis, full on hover and in the address line | 7f3a9c… |
| run identifiers | mono, short opaque strings, shown in full | r-0913 |
| column numbers | tabular numerals, right-aligned | |
| before → after labels | the label sets in brackets joined by an arrow | [INBOX] → [INBOX, Finance/Statements] |

**Vocabularies and their on-screen wording.** The stored value is what the URL and the API carry,
spelled as ADR-0016 stores it, and the registry declares each column's value set from there.

| Vocabulary | Stored value | Wording |
| --- | --- | --- |
| plan status | DRAFT · APPROVED · APPLYING · APPLIED · ROLLED_BACK · REJECTED · APPLY_REFUSED | Awaiting approval · Approved, waiting to apply · Applying · Applied · Rolled back · Rejected · Apply refused |
| candidate status | pending · confirmed · dismissed | Awaiting review · Confirmed, index not yet updated (then Confirmed, index updated) · Dismissed |
| scan state | scanned · skipped_restricted · skipped_gate · pending | in cells `scanned` · `restricted` · `gate skip` · `pending`; on hover and in L4 "scanned" · "not scanned, restricted sender" · "released unscanned, gate skip" · "pending content scan" |
| sender class | normal · restricted | normal · restricted |
| content flag | mfa_code · login_link | mfa · link |
| gate decision and reason | SCAN · SKIP, with the reason as ADR-0093's skip branches name it | scan · skip, with the reason as recorded |
| audit action | READ_BODY · DENY_BODY · MUTATE · DENY_MUTATE | body served · body denied · mutation applied · mutation refused |
| run state | running · succeeded · failed | running · succeeded · failed |
| workload state (derived, not stored) | no run yet · a run in running · the last run finished | not started · running · idle |
| backoff state (derived from `backoff_until`) | null · a future time | not in backoff · in backoff until {time} |
| validation result | passed · failed | passed · failed |
| error class | throttled · provider_error · gone · scanner_timeout · validation · authentication | provider throttled · provider error · not found at provider · scanner timeout · validation · authentication |
| disposition | recovered · pending · gone · abandoned | recovered (with the run) · pending · not found · abandoned |
| failure item kind | page · message · op | page · message · operation |
| authentication outcome (ADR-0097) | succeeded · refused · failed · none recorded | accepted · refused · no answer · not used yet |
| policy change action (ADR-0102) | added · edited · lifted · confirmed | added · edited · lifted · confirmed from a candidate |
| policy rule scope | a null account · an account | base · the account's identifier |

The scan-state wording for pending is the denial envelope's own phrase (ADR-0002, ADR-0093), so
the operator and the agent read the same words. The plan statuses are ADR-0020's and the three job
vocabularies are ADR-0016's.

## 12. Empty, loading, partial, and error patterns

| Situation | Pattern |
| --- | --- |
| Backfill pass 1 running, with no earlier run of it succeeded | A partial-index banner under the chrome on every screen, saying which pass is running and how far (pages of pages, percent), with a link to Jobs. Counts on every lens carry "so far" |
| Backfill pass 1 running again, after an earlier run of it succeeded | A change of scanner re-opens pass 1, which enumerates the whole mailbox again to mask every stored subject again under the scanner now in force ([ADR-0096](./adr/redaction/0096-a-scanner-change-reopens-backfill.md)). The banner reads "Backfill pass 1 is running again, page {page} of {of} ({share}), to mask every subject again under the scanner now in force. It last completed at {time}, so the index holds the whole mailbox and no count here is a count so far. A subject it has not reached yet keeps its earlier masks.", without the page clause while the run's checkpoint holds no page, with {time} the finish of pass 1's latest succeeded run as an absolute time ([section 11](#11-rendering-and-formatting-rules)), and with a link to Jobs. Counts on every lens carry no "so far". The UI tells this case from the one above by that succeeded run alone, read from the system endpoint ([section 17.4](#174-the-bespoke-endpoints)), so a re-opened pass 1 resumed after a failure still reads as re-opened, and a first pass 1 resumed after a failure still reads as a first. The banner shows because the subjects' masks are what is in flux, so a subject on any lens may change while the operator reads it. Hiding the banner was the alternative, and it would hide that |
| Backfill pass 2 running | The banner names pass 2 and the pending count, says pending messages deny their bodies until scanned, and links to Jobs as pass 1's does. With both passes running, one banner names both |
| No backfill run yet for an account | While the system endpoint reads backfill pass 1 as not started ([section 8.8](#88-system)), the partial-index banner reads "Indexing has not started yet for {account}. It starts once backfill picks up the account." Counts on every lens carry "so far", so an empty index never reads as final. Backfill picks up a newly connected account with no manual step ([section 8.12](#812-connect-an-account-and-re-authorize)) |
| The backfill state unknown | When the system endpoint's read fails for an account the accounts endpoint lists, the banner says the index's backfill state is unknown, so every count may be a count so far, and counts on every lens carry "so far" while that read is loading or failed. A banner that vanishes on a failed read was the alternative, and it would let a partial index's counts read as final. The banner shows only for a listed account, since for any other the body already says no such account is served. While no banner shows, the banner does not follow the stream, so a pass that starts later appears only when something next reads the system endpoint, at the latest on the next page load |
| A lens with zero rows | The L0 strip with zeros and one line, "No {rows} match", with the chips still shown so the operator can remove one |
| First load of a region | A skeleton of the region's shape (bars for a chart, lines for a table). After one second the skeleton gains the words "still loading". After ten seconds the region shows the error card, "No answer came in time", with a retry link, and the request stays open, so an answer that arrives later still replaces the card. Retry abandons that request and sends a new one. Abandoning the request at ten seconds was the alternative, and it would make any read slower than that impossible to show. A region that moves to another read while one is loading, after a filter, a group or the account changes, starts its second and its ten seconds again for the new read. Never a spinner over the whole page |
| A live surface waiting for its first event | The region renders from its fetch and the indicator reads "live · connecting" |
| A failed read | An error card in the region's place with the origin from the error contract of [section 17.3](#173-the-error-contract). "This request was refused" for the client's fault (with the message), "The UI server failed" for its own, "The database did not answer" for the database, each with the request id and a retry link. Other regions stay |
| A decision refused | The footer or outcome area shows the refusal inline, with the origin and message, and the object reloads on a conflict. A missing declared identity reads "No identity was forwarded, so the decision was not recorded" |
| Search with no result | The corpus lens at L3 with the chip and "No messages match" |
| An account's credential refused | While the account's latest authentication reads `refused` (ADR-0097), a banner under the chrome on every screen of the account, in the partial-index banner's place and above it when both show: "{provider} refused {account}'s credential at {time}. Workloads that call {provider} fail for this account until it is re-authorized.", with a Re-authorize link to [section 8.12](#812-connect-an-account-and-re-authorize). It reads the system endpoint's operational block the partial-index banner already reads, and follows the stream and the polls as that banner does. A `failed` outcome raises no banner, since workloads retry it |
| No account exists | `/` goes to the installation screen ([section 8.10](#810-installation)), whose getting-started list is the whole first run |

## 13. Keyboard map

A starting proposal, marked as design. The builder may change bindings, not the principle that
every navigation in [section 4](#4-the-zoom-ladder) has a key. The map is shown from the settings
menu and on `?`.

| Key | Action |
| --- | --- |
| `j` / `k` | move the row cursor down / up in the table that holds it |
| `Enter` | open the row under the cursor (descend, or open the detail at L3). In the sender picker of [section 8.7](#87-policy) it toggles the row's selection as `x` does |
| `Escape` | clear a selection, else close the detail panel or dialog, else remove the last chip (ascend) |
| `/` | focus search |
| `g` then `h` `p` `r` `j` `c` `m` `t` `u` `o` `s` `a` | go to Home, Plans, Review queue, Jobs, Corpus, Masking, Gate, Audit, Policy, System, Account settings. On an installation screen, which has no account, they do nothing |
| `x` | toggle the selection of the row under the cursor, in a table that selects ([section 8.7](#87-policy)'s sender picker) |
| `Shift`+`j` / `Shift`+`k` | extend the selection down / up |
| `Ctrl`+`a` or `Cmd`+`a` | in a table that selects, while it holds the row cursor, select every row the search matches, across its pages |
| `r` | in the sender picker, while a selection exists, Restrict as one rule |
| `[` / `]` | previous / next page |
| `a` in a plan or candidate | focus the approve or confirm control (never submits) |
| `?` | show the map |

One table on a screen holds the row cursor, the table of the level in view. On the run screen it is
the failed items, so the group table beside the bars takes no key. Letting every table answer was the
alternative, and one key would then move two cursors and Enter open two rows.

While a text field has focus, such as search or the sender picker's search, every key goes to the
field, apart from `Escape`, which leaves it. So `Ctrl`+`a` selects the field's text there and
selects rows only once the table holds the cursor. A selection box or a radio takes no text, so the
map's keys still work while one has focus, and the picker's keys work after a click on a row's box.

In the sender picker, `x` and `Enter` on a restricted sender's row do nothing, since it cannot be
selected. `Shift` with `j` or `k` moves the cursor and selects both the row it leaves and the row it
reaches. `r` acts only while a selection exists, and `Escape` with a selection clears it and nothing
else, so the next `Escape` removes the search's chip as on any lens.

Focus is visible everywhere (the focus ring token). Nothing submits a decision from the keyboard
without the control being focused and activated.

## 14. Visual tokens

### 14.1 Palettes

Two palettes, dark and light, derived and checked as ADR-0059 records. The operator works mostly in
dark. Tokens are stated once here and referenced by role everywhere else.

| Role | Dark | Contrast on surface | Light | Contrast on surface |
| --- | --- | --- | --- | --- |
| ground | `#161311` | | `#f7f5f1` | |
| surface | `#1f1c1a` | | `#fefdfb` | |
| raised | `#292623` | | `#fcfaf6` | |
| border | `#3b3734` | 1.4 | `#d5d0ca` | 1.5 |
| text | `#e1ddd7` | 12.5 | `#241e1a` | 16.2 |
| muted | `#9c9890` | 5.9 | `#6a615b` | 6.0 |
| faint (large or non-text only) | `#6c6863` | 3.1 | `#98918b` | 3.1 |
| action (approve, confirm, primary) | `#ed9c55` | 7.7 | `#a65c20` | 5.0 |
| restricted, denied, failed | `#f47b74` | 6.4 | `#b63132` | 6.0 |
| flagged (mfa code, login link) | `#c39ae2` | 7.3 | `#7b489e` | 6.3 |
| ok, complete | `#6fc082` | 7.7 | `#1d7d3e` | 5.1 |
| info, running | `#7cb3eb` | 7.7 | `#1f6cb0` | 5.4 |
| chart neutral fill | `#59544f` | | `#c9c3bc` | |

Button labels are `ground` on `action` in dark (8.4:1) and `surface` on `action` in light
(4.6:1). The categorical chart series for by-label, by-sender, and by-rule breakdowns are, in
slot order, dark `#3987e5` `#d95926` `#199e70` `#c98500` `#d55181` `#008300` and light `#2a78d6`
`#eb6834` `#1baf7a` `#eda100` `#e87ba4` `#008300`. Series hues are assigned in fixed order and never
cycled. In light mode the third, fourth, and fifth slots sit under 3:1 on the surface, so charts
using them carry visible labels or a table view. Sensitivity and state in charts use the semantic
colors and the neutral fill, never the categorical slots.

### 14.2 Interaction states

| State | Token |
| --- | --- |
| link | info; hover text, underlined |
| focus ring | action, 2 px outside the control |
| hover wash on a row or card | raised |
| selected row, active navigation item, active rail section | raised background with a 2 px action bar on the left |
| disabled control or text | faint |
| progress track | raised; fill info while running, ok when complete, restricted when failed |
| chart hairline, gridline, baseline | border |
| chip | border outline; an applied filter chip is action outline with action text |
| badge | outline in its semantic color with the same color text, never filled |
| decision button | action fill for approve and confirm; border outline with text color for reject and dismiss |
| lift control (removing a restriction) | restricted outline with restricted text, never filled, set apart from the add controls |
| selection box | border outline when clear; action fill with a ground-colored check when selected; faint border and not selectable for a row the action cannot take |
| setup step status | text with a glyph: a filled check in ok for done, an open circle in muted for not started, a dashed circle in faint for cannot start yet |

**Theme switching.** The OS preference selects the palette by default. The override in the
settings menu (system, dark, light) is stored per browser and wins over the OS setting. Every
token is a CSS custom property defined for both palettes, so a screen never names a color.

### 14.3 Typography and spacing

| Token | Value |
| --- | --- |
| UI sans | IBM Plex Sans, then system-ui, sans-serif. Self-hosted |
| mono | IBM Plex Mono, then ui-monospace, monospace. Self-hosted. Identifiers, addresses, numbers in tables |
| type scale | 11 px labels and table headers (uppercase, letter-spaced 0.03em), 12 px table cells and secondary text, 13 px body, 15 px section titles, 20 px screen title |
| weights | 400 body, 500 emphasis and labels, 600 titles |
| line height | 1.45 body, 1.3 titles, tables single-line |
| numerals | tabular in any column that aligns vertically, proportional elsewhere |
| spacing scale | 4, 8, 12, 16, 24 px. Panel padding 12 px, region gap 16 px, screen margin 24 px |
| controls | 28 px tall inline, 36 px tall primary decision buttons and table rows. Standalone controls have a 44 px minimum hit target; table rows are full-width targets and exempt |
| radius | 4 px everywhere, except chips at 10 px |
| borders | 1 px, border token |

No display face and no handwritten face. The annotation face on the mockups is not product.

**The font files.** The bundle carries five woff2 files, IBM Plex Sans at 400, 500 and 600 and IBM
Plex Mono at 400 and 500, each the Latin-1 subset. They are copied from IBM's own releases of the
typeface on the `IBM/plex` repository into `ui/browser/src/fonts/`, with the typeface's OFL licence
and a [README](../ui/browser/src/fonts/README.md) giving each release and each file's checksum. Characters outside the subset, such as the arrow of before → after labels, fall
back to the next face in the stack.

- **Vendored rather than installed.** A font package installed as a production dependency would
  widen the production dependency set ADR-0063 fixes at three packages. A development dependency is
  absent from the image's production install, and IBM's own npm packages depend on a telemetry
  package. The cost is that Renovate does not see the files, so a new release of the typeface is
  picked up by hand. If every asset must be tracked, the files come from the `@fontsource` packages
  as development dependencies instead, with the image's bundle stage installing them.
- **Served as files.** The policy's `font-src 'self'` blocks a font inlined as a `data:` URI, and
  bun's bundler inlines every font a stylesheet's `url()` reaches. So the build leaves the font URLs
  external, as root-relative `/fonts/` paths, and copies the files into the bundle.

## 15. Security of the UI itself

The UI's code never opens a stored credential and never reads mail (ADR-0081, ADR-0084). It
sees a credential in plaintext only while it completes a connection or a re-authorization, seals
it before storing it, and runs under the trust anchor's hardening for that reason (ADR-0028). It
holds the private key, because Google refuses a desktop client's code exchange without the
client's secret, and one isolated package of its server, `ui/internal/clientsecret`, opens with it
the secret of the client a consent was issued to and nothing else. Its import lists admit the
opening half of the credential library there alone, its one operation takes a client's name and
no sealing context, and the plaintext secret reaches no log line, no response and no cookie. A
compromised UI process holds the key that opens every stored refresh token, while its database role
reads none (ADR-0081). What remains is the browser, the transport, the database connection, the two
decisions, the two setups, and the policy writes.

- **TLS, and the same posture as the client surface** (ADR-0084). The UI serves TLS from the
  material its configuration declares ([section 18.1](#181-the-configuration-the-ui-declares)).
  The "own auth" half of ADR-0084's posture is, for the first version, the operator's ruling of
  no authentication of the UI's own with an optional authenticating proxy in front
  ([section 1](#1-what-the-ui-is-for)).
- **Content security policy** (ADR-0062). `default-src 'self'`, `script-src 'self'`,
  `style-src 'self'`, `img-src 'self' data:`, `font-src 'self'`, `connect-src 'self'`,
  `frame-ancestors 'none'`. No inline script, no external origin at runtime, fonts self-hosted.
  The mockups' Google Fonts link is a mockup convenience and is not product. The setup guide's
  window ([section 8.11](#811-oauth-client-setup)) is a document of the same origin under the same
  policy, so it takes its styles by a link to the bundle's stylesheet, never as inline styles.
- **The entry document, the session, and the request token** (ADR-0061). The browser bundle is
  static (ADR-0042), and the one exception is the entry document, which a Go handler renders on
  every page load to place the request token in a `meta` tag. The policy allows it because it is not
  a script. The UI has no authentication of its own, so the session is anonymous and exists solely
  to bind the request token, whether or not an authenticating proxy sits in front. On the first
  response the UI issues the cookie `ui_session`, a random id, `HttpOnly`, `Secure`,
  `SameSite=Strict`, with browser-session lifetime. The token is an HMAC of the session id under a
  key generated at process start, or the key in the file `token_key_file` names when more than
  one replica runs, and its lifetime is the session's. Every state-changing request sends it in the
  `X-Request-Token` header, and the server checks it against the cookie, so a request forged from
  another origin fails. The four decision requests are `POST` and carry the status the screen shows
  for conflict detection. They, the requests of OAuth client setup and account setup, and the
  policy writes are the only state-changing requests. A consent attempt is held by the server for
  the session that started it, one per session, whichever replica answers, so a pasted address from
  another session's attempt, or from an attempt a newer one replaced, is refused
  ([section 8.12](#812-connect-an-account-and-re-authorize), ADR-0111). The check refuses every
  request whose method is not `GET` or `HEAD` before it is routed, so a state-changing route added
  later cannot be mounted without it. A request with no token, and one whose token does not match
  its session, are both refused with `stale_page`. A request whose token no longer
  matches, as a page older than the UI server's last restart sends, is refused with `stale_page`
  ([section 17.3](#173-the-error-contract)).
- **The identity header is trusted only when the deployment declares it** (ADR-0084). With the
  identity header name configured ([section 18.1](#181-the-configuration-the-ui-declares)), the
  UI records that header's value on decisions and policy writes and refuses either when the header
  is absent (403, `identity_missing`, [section 17.3](#173-the-error-contract)). Unset, the UI
  records the configured operator name and trusts nothing from the request. A policy write is
  also refused with `identity_missing` when the operator name is configured empty, so no write is
  recorded with no one named.
- **The database connection carries the account.** Per request the UI opens a transaction and
  sets the transaction-local setting `app.account`, which the row-level security policies of
  ADR-0016's third layer read, by an ordinary statement like every other process.
- **A decision is one transaction, written by the UI's own code.** The status, the decision time,
  and the identity are written together, and a confirmation also inserts its policy rule row and
  that rule's policy history row in the same transaction (ADR-0084, ADR-0102). A policy write is
  one transaction the same way, its rule change and its history row together. No code runs inside the database to complete a decision
  (ADR-0060). A failure anywhere in the transaction fails the decision whole, reported with the
  database origin, and nothing is recorded. The guarantee that no path writes one row without the
  others is carried by the tests of [section 18](#18-repository-and-build-layout).
- **Message-derived text is inert** ([section 11](#11-rendering-and-formatting-rules)).
- **The UI never calls the mediator.** It has no route to it and no credential for it. Every read
  is the UI's own database role against the tables ADR-0084 grants. The only outside endpoints it
  calls are a provider's, to check a client, complete a consent, and confirm which mailbox granted
  it (ADR-0107).
- The dependency tree is small, pinned, and enumerated in a roster (ADR-0063), and the browser
  bundle ships as static files inside the Go binary (ADR-0042), so the runtime has one origin and
  one process.

## 16. Framework requirements

The twelve requirements below are what the browser framework must satisfy. ADR-0063 records the
choice, the candidates weighed, and the spike that proved requirements 1, 2, 5, and 10 on the plan
reviewer's skeleton. The plan reviewer is the proving screen because it exercises nested panels, URL
state, streamed progress during apply, and the shared row component at once.

1. Escapes text by default. No rendering path for message-derived fields may interpret markup.
2. URL routing where the query string is the source of truth for account, dataset, filters,
   level, sort, and page.
3. A component model strong enough for the plan reviewer. Nested drill panels, breadcrumb state,
   and one shared message-row component reused across every dataset.
4. Server-paginated tables of 50 rows, rendered plainly.
5. Streamed updates merged into view state without a full re-render.
6. Theming through CSS custom properties, dark and light, honoring the OS preference with an
   explicit override.
7. Keyboard navigation and focus management as first-class.
8. Charts as inline SVG drawn from data, with no heavy charting dependency. Bars, flows, timelines,
   and progress are the forms in use ([section 7.3](#73-charts)).
9. A small dependency tree, strict TypeScript, bundled by bun, shipped as static files served by
   the Go binary, with no server-side rendering runtime beyond the entry document of
   [section 15](#15-security-of-the-ui-itself) (ADR-0042).
10. Types generated from the read API contract, with drift failing the build (ADR-0042).
11. High fluency for coding agents, since the maintainer is not a UI developer.
12. Accessibility basics on tables, menus, and live regions.

A spike of a candidate builds the plan reviewer's skeleton (the rail, the ladder at two levels, one
streamed progress bar, and the message row) against fixture responses whose subjects carry markup
marker text, served under the policy of [section 15](#15-security-of-the-ui-itself).

## 17. The read API

The UI's Go server exposes one read API to its own browser app, under `/api/{account}/…`, and
nothing else calls it. Every path carries the account, and the account is a path segment passed
to every data-access function (ADR-0047). A request without it does not route (404). `all` or an
unknown account is a client error (400). The unscoped endpoints are the installation's, `GET
/api/accounts`, because the accounts list is what the account selector and the entry redirect are
built from, and those under `/api/setup/`, which serve the installation screens of
[section 8.10](#810-installation) and read no stored account's state rows. The one write among them
into an account's rows, finishing a connection, writes the new account's two rows in a transaction
set to that account. The base policy's writes among them write base rules and their history rows,
which belong to no account. The
shapes below are design and are generated into the contract document the browser's types are
built from (ADR-0042, ADR-0057).

**The contract.** Two enumerated sources feed one generator, the dataset registry and the list of
bespoke handlers. A route outside both fails the build. The generator writes an OpenAPI 3
document, checked in at `ui/contract/` and regenerated in CI, and an OpenAPI-to-TypeScript
generator writes the browser's types from it. The tools and the drift check are ADR-0065's. The
client surface of ADR-0030 already carries its contract as OpenAPI.

### 17.1 The dataset endpoint

`GET /api/{account}/lens?dataset=…&level=N&group=a&range=…&sort=…&page=P&{dimension}={value}…`

A nested dataset carries its required parent filter, `plan={id}` for `ops` and `run={id}` for
`failures`. Level 0 returns the lens's figures. Levels 1 and 2 return aggregates. Level 3 returns
a row page. The level rules and the filter grammar of
[section 5](#5-information-architecture-and-the-url) apply, and a mismatch is a client error.
`as_of` is the UI server's time when the read transaction began.

```json
{ "account": "personal", "dataset": "masking", "level": 0, "as_of": "2026-09-10T10:16:04Z",
  "filters": { "range": "7d" },
  "figures": [
    { "key": "events", "wording": "events", "value": 312, "at": null, "unit": null, "link": "/personal/masking?level=3&range=7d" }
  ] }
```

```json
{ "account": "personal", "dataset": "messages", "level": 1, "as_of": "2026-09-10T10:16:04Z",
  "group": "label", "filters": { "range": "all" },
  "total": { "count": 84212, "restricted": 1242, "flagged": 412 },
  "rows": [ { "key": { "label": "INBOX" }, "count": 42110, "restricted": 1204, "flagged": 380 },
            { "key": { "label": null }, "count": 31405, "restricted": 0, "flagged": 12 } ] }
```

```json
{ "account": "personal", "dataset": "masking", "level": 3, "as_of": "2026-09-10T10:16:04Z",
  "filters": { "range": "7d", "rule": "content.mfa.subject_numeric_6" }, "sort": "sent_at,desc",
  "page": 1, "pages": 5, "total": { "count": 201, "restricted": 0, "flagged": 201 },
  "rows": [ { "id": 88123, "message_id": "m_3f9e4", "from_email": "receipts@acme.io", "subject": "…", "sent_at": "2026-09-06T14:02:00Z", "labels": ["INBOX"], "sender_class": "normal", "content_flags": ["mfa_code"], "scan_state": "scanned", "rule_id": "content.mfa.subject_numeric_6", "tier": 1, "masked_at": "…" } ] }
```

Aggregate rows and the total carry `count`, and for message-derived datasets (`messages`,
`masking`, `gate`, `audit`, `ops`, `failures`) also `restricted` (rows whose message's sender is
restricted) and `flagged` (rows whose message carries a content flag), so every chart splits by
sensitivity without a second request. Both read `messages.sender_class` and
`messages.content_flags` at read time, the index's current values. `senders` carries neither, and
its rows carry the sender's class. Group keys are the dimension values as stored, with `null` for
the unfiled label group and for an audit row with no message. A case-insensitive dimension, such as
`sender`, groups every spelling of a value together, so its key is the value as the database lowers
it, by the function its case-insensitive comparison uses, and a filter naming that key matches every
spelling. Sending one stored spelling was the alternative, and which spelling names the group would
then be the database's arbitrary pick. A value stored empty, such as an empty domain, is a group of
its own keyed by the empty string, apart from the null group, and a filter names it `empty`, since
`dim=` with no value is a removed default filter
([section 5](#5-information-architecture-and-the-url)) and the endpoint refuses an empty value. A
dimension declares whether it can hold a value stored empty, and `empty` is refused on one that
cannot, as `none` is on one with no null group. Of the dimensions served, `sender` alone can,
because a domain is free text taken from whatever follows an address's last `@`, and every other
text dimension holds a closed vocabulary. The same free text can spell a domain `none` or `empty`,
as an address at a host whose name has no dot does. Such a domain's group is keyed by that
spelling, and no filter names it, since the word names the null or the empty group. Row pages are
offset-paginated, 50 rows per page, with the page count, and no other page size exists. Every sort
ends with the row's own identity as its final key (ADR-0057). A row is the dataset's row type, which
begins with the message-row fields of [section 7.1](#71-the-message-row) where the dataset is
message-derived, and every row carries its identity (`message_id`, or `id` for masking and audit
rows, or `seq` for failures). Every level's answer also carries its applied filters as the URL
writes them, the range among them.

`plans` and `candidates` are the lists that the plans screen, the review queue and Home's decision
inbox read ([sections 8.1](#81-home), [8.6](#86-review-queue) and [8.9](#89-plans)). Neither has a
groupable dimension, so each is read at L0 and L3 only and is served by a summary and a rows
statement.

| | `plans` | `candidates` |
| --- | --- | --- |
| Filterable | `status`, with the plan statuses of [section 11](#11-rendering-and-formatting-rules) | `status`, with the candidate statuses of [section 11](#11-rendering-and-formatting-rules) |
| Sortable | `created_at` | `score`, `created_at` |
| Range | over `created_at`, default all time | over `created_at`, default all time |
| Default | L3, `sort=created_at,desc`, no filter | L3, `sort=score,desc`, `status=pending` |
| L0 figures | one per status, worded as [section 11](#11-rendering-and-formatting-rules) words the status, each linking to the plans screen with that status | one per status, worded as section 11 words the status apart from `confirmed`, worded Confirmed because the count holds candidates whose index is updated and those whose index is not yet, each linking to the review queue with that status |
| Row | plan id, description, status, proposer, created time, decision time and identity, refusal reason, message count, and the latest apply run and rollback run found by their plan reference | domain, score, the recorded signals, status, created time, reviewed time and identity, and the sender's message count and first-seen time from its sender statistics |

A candidate's recorded signals reach the browser typed, as a list of entries each carrying
`heuristic`, the stored identifier as a string, and `evidence`, an object of the four evidence keys
of [ADR-0016](./adr/data/0016-schema.md), `name`, `domain` and `keyword` as strings and `score` as a
number, each `null` where the entry records none. The server reads each stored entry on its own and
never refuses the row over one. A key of the wrong type is sent as `null`, an entry with no string
identifier is sent with an empty one, and a stored value that is not a list is sent as one entry
with an empty identifier, so the screen shows that something it cannot word was recorded
([section 8.6](#86-review-queue)). Refusing the read was the alternative, and one malformed row
would then take down the review queue and Home's inbox.

Levels 1 and 2 answer with the request's group, its applied filters, the total of the rows those
filters match, and the groups. The groups are ordered by count descending, then by their key with
`null` last, which is the order the bars of [section 7.3](#73-charts) draw. For `runs` and `failures`
the answer holds every group, and the table beside the bars pages them 50 at a time in the browser.
Paging their groups on the server as rows page was the alternative. Their groups are few, since a
range bounds a run's days and a run bounds its failures, and the statement check cannot prove a
page of groups over a derived value such as a day totally ordered. Every other dataset decides where
it is built whether its groups are sent whole or paged. `sort=` orders level 3's rows and leaves the
groups in that order. Letting `sort=` order the groups too was the
alternative, and it would reorder the table away from the bars beside it, which draw the same
groups. The null group of a dimension that has one is written `none` in a filter, as the label
dimension's is, and the group of a value stored empty is written `empty`. A filter on a `day`
bucket names UTC dates, and a filter on a number names whole numbers, and any other value is refused
as the client's fault before a statement runs.

A figure is a count or a time. A count carries its `value` and a `null` time, and a time, such as the
last failure below, carries its instant in `at` and a `null` value. A time with nothing to show
carries both as `null`, and the strip shows "none". Writing a time as a count of seconds with a unit
was the alternative, and every reader of a figure would then have to know which units hold a time.

`runs` and `failures` are the jobs screen's table and the run screen's ladder
([sections 8.3](#83-jobs) and [8.4](#84-run)). A run's failures are read under the required parent
filter `run={id}`.

| | `runs` | `failures` |
| --- | --- | --- |
| Filterable | `workload`, `state`, `pass` and `day`, with the vocabularies of ADR-0016 and a UTC date for `day`. A run that records no pass, the heuristics run, passes every exclusion of `pass` | `error_class`, `disposition`, `sender` and `page_number`. `sender` is the domain of the item's message, and `page_number` the page the item was processed on, each with a `none` group for an item that has none, such as a page item or a message the index no longer holds, and `sender` with an `empty` group for a message stored with an empty domain |
| Groupable | `workload`, `state`, `day` | `error_class`, `sender`, `page_number`, `disposition` |
| Sortable | `started_at`, `duration` (to the finish, or to `as_of` while running), `failures` | `last_at`, `attempts` |
| Range | over `started_at`, default the last 7 days | none, bounded by the run |
| Default | L3, `sort=started_at,desc`, `pass=!tick` | L1 grouped by `error_class`, `sort=last_at,desc` |
| L0 figures | runs, running and failed, each linking to the jobs screen with that state, the first with none, and last failure, the finish of the latest failed run as a time, linking to that run | failures, then one per disposition in the order of [section 11](#11-rendering-and-formatting-rules) and in its wording, each linking to the run's L3 with that disposition |
| Row | the run as the jobs endpoint sends it, with its plan's description and status, and its count of item failures | `seq`, the item's kind and identifier, the message-row fields of the item's message, `null` for a page item or a message the index no longer holds, the page, the error class, attempts, first and last error time, the disposition and the recovering run |

A page item counts as neither restricted nor flagged.

`policy_changes` is the policy history of [section 8.7](#87-policy), `senders` gains the
`search` filter the sender picker reads, and `rules` gains the `search` filter the policy screen's
search box reads, a substring match over identifiers and suffixes.

| | `policy_changes` |
| --- | --- |
| Filterable | `action`, with the actions of [section 11](#11-rendering-and-formatting-rules), `scope`, the account or the base policy's changes, which are the dimension's null group, written `none` in a filter and worded base, `actor`, `rule`, and `day` |
| Groupable | `action`, `scope`, `actor`, `day` |
| Sortable | `ts` |
| Range | over `ts`, default the last 30 days |
| Default | L3, `sort=ts,desc` |
| L0 figures | changes, those that added restriction (added, confirmed, and edits that only added suffixes) and those that lifted it (lifted, and edits that removed a suffix), each linking to the history with that filter |
| Row | the policy change row of [section 7.2](#72-the-sender-row-the-audit-row-the-page-row-and-the-policy-change-row), with `id` as its identity |

The account's history is its own rows and the base policy's, as row-level security confines it
(ADR-0102). A row carries no account other than the one in the path or none.

`GET /api/{account}/{dataset}/{row-id}` (with the parent filter for a nested dataset, and no other
parameter) returns one row with its provenance for L4, including the message's audit rows, for the
datasets that declare a provenance query. The registry adds one such path for each of those
datasets. `senders` declares none. `plans`, `candidates` and `runs` declare none, because their row
paths belong to the bespoke handlers, and the contract generator refuses a path claimed by both
sources. A row the account's parent does not hold is refused with the client's 404, `unknown_row`.
`failures` declares one. Its detail is the row with the error summary as recorded, the item's
message's sensitivity block of [section 7.1](#71-the-message-row), which is the rule that set its
class as `class_rule_id`, the content rules that fired as `rule_ids`, the time it was scanned and the
scanner version, and the newest 50 audit rows of the message with their count. `class_rule_id` is
`null` when no rule set the class, and every field of the block is `null` for a page item or a
message the index no longer holds.

The page dimension is `page_number`, because `page=` is the common parameter that pages rows.
Naming it `page`, as the run screen's group-by control words it, was the alternative, and a URL
could not tell the filter from the page parameter. A dimension named as a common parameter or as its
dataset's parent filter is refused by the statement-set check.

### 17.2 The registry entry

Each dataset is one entry in Go, from which the contract, the types, and the browser's dataset
catalogue are generated.

| Field | Content |
| --- | --- |
| name | the dataset name used in URLs |
| parent | the required parent filter, if any |
| row type | the Go type of one row, from which the browser type is generated |
| dimensions | name, storage type, whether groupable, whether filterable, whether sortable, the wording shown for the dimension, the wording for its `null` group where one exists, and for a time column the bucket dimensions `hour`, `day`, `month` it derives. `search` is declared as a filter-only entry of kind text |
| values | for a column with a closed set of values, the values as stored, from which the wording table is keyed |
| default | the default group, level, range, and sort, and the default filters. The browser's router fills every default when it canonicalizes a URL. The endpoint fills a missing group, level, range, or sort from the entry and never a filter, because an absent filter there means no filter |
| summary | the query for the L0 figures, each with its key, wording, count or time, unit, and link |
| queries | one statement per groupable dimension for the aggregate levels, plus one for rows, each parameterized by account, filters, sort and page, written against the schema per ADR-0047 and enumerated rather than composed per ADR-0066 |
| provenance | optional. The query that fetches one row's detail. `senders`, `plans`, `candidates` and `runs` declare none |

Event datasets (`masking`, `gate`, `audit`, `failures`) join their event table to `messages` by
account and message id for the sender, subject, class, and flags, reading the current values.
`senders` reads the senders table. `ops` reads the normalized plan operations joined to `messages`.
Every query runs in a transaction that has set `app.account` ([section 15](#15-security-of-the-ui-itself), ADR-0016).

A request naming a dataset, dimension, filter, or sort column the entry does not declare is refused
with the client-fault error. The control's row is in [docs/VERIFICATIONS.md](./VERIFICATIONS.md).

### 17.3 The error contract

Every failure is one shape, and the origin mirrors
[O5](../USE_CASES.md#o5--clients-can-tell-failures-apart)'s distinction for the client surface.

```json
{ "error": { "origin": "client", "code": "unknown_dimension", "message": "…", "request_id": "…" } }
```

| Origin | Status | Meaning |
| --- | --- | --- |
| `client` | 400, 404, 409 | the request was malformed, named something undeclared, or carried a stale status (409 conflict) |
| `client` | 403, code `identity_missing` | the deployment declares an identity header and the request carries none, or a policy write would record an empty identity |
| `client` | 403, code `stale_page` | the request token does not match the session, as on a page older than the UI server's last restart. The status region reads "This page is older than the UI server's last restart. Reload, then try again." and the values typed stay in their fields |
| `ui` | 500 | the UI server failed on its own |
| `database` | 503 | the database did not answer, refused, or a decision's transaction failed |
| `provider` | 502 | the provider did not answer a setup request, checking a client or exchanging a consent's code |

401 is unused. The UI has no authentication of its own.

### 17.4 The bespoke endpoints

| Endpoint | Returns or carries |
| --- | --- |
| `GET /api/accounts` | every account's identifier and provider. Unscoped |
| `GET /api/{account}/plans/{id}` | the header fields, the proposer, `expires_at` (created time plus the configured maximum plan age), the L0 summary with the share of corpus, the label operations with their touched counts, the validation results (creation and, if run, apply) and refusal reason, the history events assembled with time and identity, the apply and rollback run ids, and while applying the op-log progress, scoped through the plan's account |
| `GET /api/{account}/plans/{id}/sample?seed=` | the stratified sample, the seed used, and the strata sizes |
| `POST /api/{account}/plans/{id}/approve` with `{ "expected_status": "DRAFT", "confirmation": "12480" }` | 200 with the updated plan, 409 on a status mismatch, 400 when the confirmation is required and its digits do not match |
| `POST /api/{account}/plans/{id}/reject` with `{ "expected_status": "DRAFT" }` | as approve |
| `POST /api/{account}/candidates/{domain}/confirm` with `{ "expected_status": "pending" }` (or `dismissed`) | 200 with the candidate and the pending count, 409 on mismatch |
| `POST /api/{account}/candidates/{domain}/dismiss` with `{ "expected_status": "pending" }` | as confirm |
| `GET /api/{account}/jobs` | one block per workload with its derived workload state and the card fields of [section 8.3](#83-jobs), the rate block (current, target, cap, backoff, last throttle, and reserved and used per class), and the cadences from configuration |
| `GET /api/{account}/jobs/{run}` | the run as the jobs endpoint sends it, the latest run whose `resumed_from` names it, its item failures counted in all and per disposition, the runs that recovered its items with how many each, and every event of its timeline (kind, time, page, detail) in the order recorded. An unknown run is refused with the client's 404, `unknown_run` |
| `GET /api/{account}/attention` | the "worth a look" cards of [section 8.1](#81-home), in their order, each with its rule id (`backlog`, `masking`, `body_serves`, `sync_gap`, `expiry`), what, number, since (`null` when the rule has none), the sentence, and the link the rule's table names |
| `GET /api/{account}/system` | three blocks. `operational`, the values of [section 8.8](#88-system) and the finish time of backfill pass 1's latest succeeded run, which tells the partial-index banner a re-opened pass 1 from a first one ([section 12](#12-empty-loading-partial-and-error-patterns)); `corpus`, the at-a-glance figures of Home's System column; `decisions`, the counts of plans in DRAFT, candidates pending, and workloads running, which the chrome's counters read |
| `GET /api/{account}/events` | the live stream, `text/event-stream` |
| `GET /api/setup` | the installation endpoint. The providers whose accounts connect through an OAuth client, each OAuth client's name, provider, client identifier and project ID with the accounts connected through it, every account's identifier, provider and the client it connects through, and the count of base rules. Unscoped, and reads nothing of any account's state, so the getting-started list of [section 8.10](#810-installation) reads whether an account exists, never whether one is connected |
| `POST /api/setup/{provider}/clients` with `{ "name": "…", "client_id": "…", "client_secret": "…", "project_id": "…" }` | checks a new client with the provider and stores it under its name, the secret sealed and the project ID beside it, `null` when neither step 1 nor the file gave one. 200 when stored, the client's 400 `client_refused` when the provider refuses it or the identifier or the secret is empty, the provider's 502 `provider_unreachable` when it does not answer, 400 `name_refused` for a name [section 8.11](#811-oauth-client-setup) refuses, 400 `project_refused` for a project ID not shaped like one, 409 `name_taken` or `client_exists`, the latter naming the client that holds the identifier, and 404 `unknown_provider` for a provider that authenticates through no OAuth client. Unscoped |
| `POST /api/setup/{provider}/clients/{client}` with `{ "client_id": "…", "client_secret": "…", "project_id": "…" }` | replaces the named client's identifier and secret, or with the stored `client_id` updates its secret alone, which keeps every grant ([section 8.11](#811-oauth-client-setup)). Answers as the add, and the client's 404 `unknown_client`. Unscoped |
| `POST /api/setup/{provider}/clients/{client}/remove` | removes a client no account connects through. The client's 409 `client_in_use` with the accounts that do, and 404 `unknown_client`. Unscoped |
| `POST /api/setup/connect` with `{ "account": "…", "client": "…", "mailbox": "…", "lowered_target": null }` | starts a consent attempt for the session through the named client after checking the identifier, replacing any attempt the session held ([section 8.12](#812-connect-an-account-and-re-authorize)), 400 `identifier_refused`, `mailbox_refused` for a mailbox with no local part or no domain, or `target_refused` for a target outside the range of [section 8.13](#813-account-settings), 409 `identifier_taken` or 404 `unknown_client` otherwise. The provider is the client's. 200 with the consent page's address and the attempt's expiry. Unscoped |
| `GET /api/setup/connect` | the session's attempt, its identifier, mailbox, client, expiry and consent page's address, with its state as the attempt's name, which a reload of the page restores and a tab compares with its own to tell it was replaced, or none. Unscoped |
| `POST /api/setup/connect/finish` with `{ "address": "…" }` | finishes the session's attempt from the pasted address and writes the account's two rows. 200 with the account, or the client's 400 with one code per cause of [section 8.12](#812-connect-an-account-and-re-authorize)'s refusals, `consent_declined`, `no_code`, `wrong_address`, `wrong_attempt`, `no_attempt`, `attempt_expired`, `code_refused`, `scope_missing`, `scope_refused`, `api_disabled`, `wrong_mailbox`, the client's 409 `identifier_taken` when an account took the identifier after the attempt started, 409 `client_changed` when the attempt's client was replaced or removed after it started, and the provider's 502 `provider_unreachable` when Google does not answer. Unscoped |
| `GET /api/{account}/account` | the values of [section 8.13](#813-account-settings), the identifier, provider, the client it connects through and the provider's other clients, mailbox, whether a state row exists, the last authentication, the lowered target as a fraction, the current target from the rate state, the backfill flags and when the sync cursor was written. Never the credential |
| `POST /api/{account}/account/reauthorize`, `GET /api/{account}/account/reauthorize` and `POST /api/{account}/account/reauthorize/finish` | as the connect requests, for the account in the path, checked against its remembered mailbox, and replacing its credential and recording the code exchange's attempt on success. The start request carries `{ "mailbox": "…" }` only when the account remembers none, a mailbox in the request of an account that remembers one is refused with 400 `mailbox_remembered`, and a request naming none for an account that remembers none with 400 `mailbox_required`. It carries `{ "client": "…" }` to move the account to another client of its provider, refused with 400 `client_wrong_provider` for a client of another provider, and success then writes the client and the credential in one transaction |
| `GET /api/setup/policy?search=…` | the base rules, each with its identifier, suffixes, source and created time and identity, and every account's identifier, which the base policy screen of [section 8.14](#814-base-policy) lists. No count of any account's senders or messages. Unscoped |
| `GET /api/setup/policy/history?range=…&rule=…` | the base policy's history rows alone, newest first, `rule=` narrowing them to one rule's for its screen ([section 8.14](#814-base-policy)). Unscoped. This endpoint, the one above and the base rule count of `GET /api/setup` read in a base-policy transaction (ADR-0112) |
| `POST /api/setup/policy/rules`, `POST /api/setup/policy/rules/{rule-id}` and `POST /api/setup/policy/rules/{rule-id}/lift` | add, edit and lift a base rule with its history row, with the bodies and answers of the account's policy writes below, `scope` absent since it is always `base`, and a lift or a suffix removal always needing `confirmation`. Unscoped |
| `POST /api/{account}/account/target` with `{ "lowered_target": 0.3 }` | stores the lowered target, or clears it with `null`. 400 `target_refused` outside the range of [section 8.13](#813-account-settings), and the client's 409 `not_connected` for an account with no state row, which holds no target until it is connected |
| `GET /api/{account}/policy/match?suffix=…` | for each suffix, whether it is a valid domain suffix, whether it is itself a public suffix, the senders and messages it matches in the account, and any rule that already matches it, which the add panel reads as each line is typed, with `together`, the senders and messages every suffix matches counted together, a sender two of them match counting once, and the account's count of senders, which a share is taken of |
| `GET /api/{account}/policy/release?scope=…&rule=…&keep=…` | the senders and messages keeping only the `keep` suffixes of the rule, or none for a whole lift, would release in the account, the senders its policy restricts now and would not after, which the lift dialog of an edit removing several suffixes reads, since a sender two removed suffixes both match is released only by removing both. The client's 404 `unknown_rule` for a rule the account's policy does not hold |
| `GET /api/setup/policy/match?suffix=…` | for each suffix typed in Add a base rule, whether it is a valid domain suffix, whether it is itself a public suffix, and the base rules already matching it, with no count, since counts are each account's. Unscoped, and read in a base-policy transaction |
| `POST /api/{account}/policy/rules` with `{ "scope": "account", "rule_id": "…", "suffixes": ["…"] }` | adds a rule, `scope` being `account` or `base`, with its history row. 400 `rule_refused` with each problem, 409 `identifier_taken` |
| `POST /api/{account}/policy/rules/{rule-id}` with `{ "scope": "account", "suffixes_before": ["…"], "suffixes": ["…"], "confirmation": null }` | edits the rule's suffixes with its history row. The client's 404 `unknown_rule` for a rule the account's policy does not hold, read before the write, so a rule another account holds is never reported as a conflict. 409 when `suffixes_before` differs from what is stored. An edit that removes a suffix of a base rule needs `confirmation`, the rule identifier typed, refused with 400 `confirmation_required` otherwise, and an edit removing every suffix is refused with 400 `rule_refused` |
| `POST /api/{account}/policy/rules/{rule-id}/lift` with `{ "scope": "account", "suffixes_before": ["…"], "confirmation": null }` | removes the rule with its history row. 404 and 409 as edit, and `confirmation` as edit for a base rule |
| `GET /api/{account}/policy/export` and `GET /api/setup/policy/export` | the scope's rules in the file form of [section 8.7](#87-policy), as a download. The base one is unscoped and reads in a base-policy transaction |
| `POST /api/{account}/policy/import/preview` and `POST /api/setup/policy/import/preview` with the file | reads and checks the file and answers the four groups of the preview, the account's numbers for the account's scope, and the scope's stored rules it was computed against. 400 `file_refused` with each problem, its rule and its line. Writes nothing. The base one is unscoped and reads in a base-policy transaction |
| `POST /api/{account}/policy/import` and `POST /api/setup/policy/import` with `{ "file": "…", "computed_against": "…", "confirmation": null }` | applies the file as one transaction, each change with its history row. 409 `stale_preview` when the scope's rules read in the import's transaction differ from `computed_against`. An import that lifts anything needs `confirmation`, `lift {k}` with {k} the count it lifts, which the dialog sends, typed by the operator for the base scope, refused with 400 `confirmation_required` otherwise. The base one is unscoped and reads and writes in a base-policy transaction |

The plans and candidates lists are read through the dataset endpoint's `plans` and `candidates`
datasets ([section 17.1](#171-the-dataset-endpoint)), and the rules and the policy history through
its `rules` and `policy_changes` datasets. The bespoke endpoints above serve one plan, its sample,
the decisions on plans and candidates, the installation, the setups, an account's settings, and
the policy writes.

Every policy write records the identity of [section 15](#15-security-of-the-ui-itself) as a
decision does, and is refused with `identity_missing` as a decision is, and also when the
identity it would record is empty, as an operator name configured empty leaves it. A policy write refused by the checks of
[section 8.7](#87-policy) writes nothing. Its `rule_refused`, and a file's `file_refused`, carry the
error contract's shape and beside it `problems`, each with its kind, its rule's identifier, its line
in a file and the suffix an invalid suffix names, so the screen names every problem at once.
Answering the first problem alone was the alternative, and an operator fixing a file would then meet
its problems one import at a time. The lift counts the dialog states are the senders the
account's policy would no longer restrict without the change. A whole rule's lift and the removal
of one suffix read them with the rule's row detail, and an edit removing several suffixes reads the
set's together from the release endpoint, since a sender two removed suffixes both match is
released only by removing both.

Every `POST` carries the `X-Request-Token` header of
[section 15](#15-security-of-the-ui-itself), and a body that is not the JSON its request takes,
a field it does not declare included, is refused with 400 `malformed_body`. A decision's body never carries anything the server
does not already hold except the typed confirmation.

### 17.5 The stream's event shape

One event per changed object, with the whole current state of that object, never a delta, so a
missed event costs nothing.

```text
event: run
data: {"account": "personal", "run_id": "r-0914", "state": "running", "checkpoint": {"page": 3065, "of": 3368}, "counters": {…}, "heartbeat_at": "…"}

event: rate
data: {"account": "personal", "current": 3.1, "target": 5.0, "cap": 8.0, "backoff_until": null, "classes": {"interactive": {"reserved": 1.5, "used": 0.4}, "sync": {"reserved": 1.0, "used": 0.8}, "batch": {"reserved": 2.5, "used": 1.9}}}

event: plan
data: {"account": "personal", "plan_id": "…", "status": "APPLYING", "applied": 4120, "of": 12480}
```

The cadence, reconnection, and fallback rules are ADR-0058's. A poll that changed nothing writes a comment line instead of an event, so a proxy in front never closes the stream as idle.

## 18. Repository and build layout

Per ADR-0054 the UI is one top-level directory holding both halves, and per ADR-0042 the server
half is Go and the browser half is TypeScript. How much of it is built is tracked in ROADMAP.md.

```text
ui/
  main.go               the composition root. It embeds browser/dist and hands it to the server
  Dockerfile            the bundle stage, then the Go stage, then a minimal base
  internal/
    registry/           the dataset registry entries, each pointing at the data-access accessors it reads
    api/                the server. The entry document, static files, the read API, the decisions, the stream,
                        the bespoke handlers, the error contract, the request token, identity
    contract/           builds the OpenAPI document from the registry and the handler list
    clientsecret/       opens an OAuth client's sealed secret for a consent's code exchange, the one part
                        of the UI that opens a stored value (ADR-0081)
    core/               pure-core packages private to the UI
    devloop/            whether the binary was built with the devloop build tag
  contract/             the generated OpenAPI document (checked in, regenerated in CI)
  browser/              the TypeScript app, with its manifest, lock file, runner, compiler, lint and format settings
    codegen/            the type generation step's own package (ADR-0065)
    scripts/            the build and the dataset descriptor table's generator
    rules/              the ast-grep rules (ADR-0072)
    src/
      generated/        types and the dataset descriptor table, from ui/contract, never edited
      app/              routing, the URL grammar, the cache, theme, the stream client
      lens/             the ladder shell, chart, cohort table, rows table, row detail
      row/              the message row, the sender row, the audit row, the page row, badges
      screens/          home, plan, plans, jobs, run, candidates, policy, system, installation, client
                        setup, connect and account settings
      fonts/            the vendored font files with their licence (section 14.3)
    test/               the browser tests, the DOM shim's preload, the fixture modules
    dist/               the bundle, embedded into the Go binary, not checked in except its placeholder
  design/               the mockup sources and their notes
```

- **Build.** `bun build` produces `browser/dist`, the Go binary embeds it, and the image of ADR-0049
  carries the binary and nothing else. A placeholder in `browser/dist` keeps Go's build, lint and
  tests working before any bundle exists, and the image build refuses a bundle that is only the
  placeholder. Where tests, violation files and tooling sit follows
  [CLAUDE.md](../CLAUDE.md#code-layout-and-conventions). Every build passes the production define
  ADR-0063 names. The contract document is generated from the registry and the handler list in CI,
  and the browser types and the descriptor table from the document. A stale document, stale types,
  or a stale descriptor table fails the build (ADR-0065).
- **Dev loop.** The Go server runs against a local Postgres with the synthetic fixtures, in plain
  HTTP under `insecure_http: true`, which the server refuses unless its binary is built with the
  `devloop` build tag. No image build sets the tag. The browser app runs under
  bun's dev server proxying `/api` to the Go server, with the same request token and identity
  rules in force. The dev server serves no policy header, so the policy of
  [section 15](#15-security-of-the-ui-itself) is exercised only against the built output the Go
  server serves (ADR-0064).
- **Tests.** The Go side tests the registry, the queries, the error contract, the decisions, and the
  stream against a real Postgres with synthetic fixtures, never a mock (ADR-0043, ADR-0044). The
  browser side has example-based tests of the rows, the ladder, and each screen, run under bun
  against a DOM shim, on fixture responses recorded from the real server. Their subjects, display
  names, and reasons carry designed marker text of the same kind ADR-0044 puts in fixture bodies,
  including markup markers, and every test asserts the marker arrives inert on every surface in the
  form ADR-0064 requires. One integration test drives a fixture plan from DRAFT to APPROVED through
  the real server. The decision transactions are proven against the real database. Every path that
  writes a status is exercised with a fault injected between its writes and must leave nothing
  behind (ADR-0060).

### 18.1 The configuration the UI declares

The UI knows its environment contract and never its platform (ADR-0051), and standing it up
requires only what its artifacts declare ([O6](../USE_CASES.md#o6--deployable)). This is the
declaration. Each key is its configuration path under ADR-0078, its one name, from which its
environment name and flag derive, so `listen` is also `MEDIATED_MAILBOX_LISTEN` and `--listen`, and
each key can come from the file, the environment or a flag. Every key is read at start, and no key
carries secret material, which arrives as a mounted file whose path a key names (ADR-0079). A key
lands in the binary with the work that first reads it, and until then the binary refuses it as it
refuses any key it does not declare (ADR-0078). Which keys the binary reads today is build state,
tracked in [ROADMAP.md](../ROADMAP.md).

Four keys are the browser's, `default_theme`, `stream_reconnect_max`, `stream_poll_interval` and
`consent_redirect`. The Go handler renders each into the entry document on every page load as a
`meta` tag named `mediated-mailbox.` followed by the key, with the value as its content and a
duration written in whole milliseconds. The request token rides beside them as
`mediated-mailbox.request_token`, the placement ADR-0061 gives it. The browser's composition root,
`main.ts`, is their one reader, and a tag it cannot read takes the record's default, apart from
`consent_redirect`, which the browser holds no copy of, so a page without it pictures no address
and finds no pasted address to be the redirect's.

| Key | Value | Required |
| --- | --- | --- |
| `database` | the section ADR-0078 declares for every deployable, the Postgres connection settings for the UI's own role (ADR-0084) rendered into the connection string, with the password read from the file `database.password_file` names and `database.user` defaulting to that role | yes |
| `listen` | the address and port the UI serves on | no, defaults to `:8443`, as the mediator's does |
| `probe_listen` | the address and port the health and readiness probes and the metrics endpoint serve on, in plain HTTP | no, defaults to `:8080`, as the mediator's does |
| `tls_cert`, `tls_key` | paths to the TLS material, mounted as files (ADR-0079's convention) and read again on each handshake, as the mediator's are, so a renewed certificate needs no restart | yes, unless `insecure_http` |
| `seal_public_key_file` | the path of the mounted public key the UI seals credentials to (ADR-0081) | yes |
| `private_key_files` | the paths of every mounted private key, the keyring the deployables that call a provider hold, with which the UI's client-secret package opens a client's secret for a consent's code exchange (ADR-0081, ADR-0092). The start is refused unless the public key matches one of them | yes |
| `insecure_http` | `true` serves plain HTTP, refused unless the binary is built with the `devloop` build tag ([section 18](#18-repository-and-build-layout)) | no |
| `identity_header` | the header name an authenticating proxy forwards. When it is set, its value is recorded on decisions and policy writes, and a decision or a policy write without it is refused | no |
| `operator_name` | the identity recorded on decisions and policy writes when no header is declared | no, defaults to `operator` |
| `consent_redirect` | the loopback address a consent redirects the browser to, where nothing listens, which the consent request and the code exchange name, the pasted address is checked against, and the browser reads from the entry document to picture it and check a paste ([section 8.12](#812-connect-an-account-and-re-authorize)). The start is refused unless it is an `http` address whose host is a loopback IP literal, `127.0.0.1` or another in `127.0.0.0/8`, or `[::1]`, with an explicit port, and no path but `/`, no query, fragment or user. A desktop client accepts a loopback redirect on any port, so changing it needs nothing at Google | no, defaults to `http://127.0.0.1:47823/`, a high port a web server on the operator's computer is unlikely to answer on |
| `token_key_file` | the path of a mounted file holding the key behind the request token and the consent attempt's seal (ADR-0111), at least 32 bytes, replacing the per-process key when more than one replica runs | no |
| `max_plan_age` | the maximum plan age, from which `expires_at` is computed. The value's home is the roadmap's open decision, and this key mirrors it | no, defaults to that value |
| `sample_size` | the plan sample's size | no, defaults to 24 |
| `sync_interval`, `heuristics_interval` | the intervals displayed on the jobs cards (ADR-0018's sync interval of 5 minutes, and the heuristics job's daily run of ADR-0022) | no, defaults to the records' values |
| `attention_backlog_share`, `attention_mask_count`, `attention_serve_factor`, `attention_gap_days`, `attention_expiry_days` | the "worth a look" thresholds of [section 8.1](#81-home), one key per rule, 0 disabling the rule and a negative value refused. The backlog's is a percent of the corpus, the masking rule's a count of events, the body-serve rule's a multiple of the median, and the sync-gap and expiry rules' a number of days | no, defaulting to the starting values of section 8.1, 5, 20, 2, 7 and 2 |
| `default_theme` | `system`, `dark`, or `light` | no, defaults to `system` |
| `stream_interval`, `stream_reconnect_max`, `stream_poll_interval` | the poll cadence behind the event stream, the reconnection backoff ceiling, and the polling fallback interval (ADR-0058) | no, defaults to the record's values |

### 18.2 The UI's own observability

The UI emits and never collects (ADR-0051). Every request is logged with its request id, the
account, the route, and the outcome classified by the error contract's origin. Metrics carry
read latency by dataset and level, stream subscriber count, and decision outcomes by decision and
result, as `mediated_mailbox_ui_read_duration_seconds` by `dataset` and `level`,
`mediated_mailbox_ui_stream_subscribers`, and the decision outcomes' series where the decisions are
built. The health and readiness probes and the metrics endpoint serve on their own plain-HTTP
listener, `probe_listen`, apart from the UI's TLS listener, and the UI reports ready only while the
database answers its role. No log line and no metric label ever carries message-derived text. This is the UI's share of [O2](../USE_CASES.md#o2--observable).

## 19. Building it

**Order of work.** The order the UI's pieces are built in is build state. The units that carry
them live in [ROADMAP.md](../ROADMAP.md), and the order lives in their tickets, never here.

**State model.** Route state is the URL. Data state is per request, keyed by the URL, cached for
a few seconds. Live surfaces hold one stream subscription whose events replace the matching
object in the data state. No global mutable store beyond these three. Which mechanism carries
each is ADR-0063's decision.

**What not to do.** Do not add a rendering path that interprets message-derived text. Do not add a
decision, however small, without a record that widens ADR-0084's grant. Do not let any of the UI's
code but `ui/internal/clientsecret` open a stored value. Do not aggregate across accounts. Do not proxy anything
through the mediator. Do not fetch a corpus into the browser to
group it there. Do not paint a fake status bar or keyboard in any layout. Do not introduce a
color outside the token tables without re-running the palette derivation and, for a chart series,
its validator. Do not read a header for identity unless the deployment declared it. Do not keep
view state outside the URL. What is kept per browser is none of it view state: the theme override,
the account last used, and the OAuth client setup's marks and project identifier, per client, of
[section 8.11](#811-oauth-client-setup). Do not put a trigger, a procedure, or a function in the database for
anything (ADR-0060).

**Tests the UI owes.** Every control the UI carries is dispositioned in
[docs/VERIFICATIONS.md](./VERIFICATIONS.md), as an injection row keyed to the unit that delivers it or
as a standing disposition, and that catalogue, not this document, is the list.
[TESTING.md](../TESTING.md) decides the kinds.

## 20. What remains open

Everything this design relies on about the system is stated by a record and cited where it is
used. What is still open, and where it is tracked:

| Open | Tracked in |
| --- | --- |
| The maximum plan age value, which `expires_at` and the expiry rule of [section 8.1](#81-home) read from configuration | [ROADMAP.md's open decisions](../ROADMAP.md#open-decisions) |
| The "worth a look" rules and thresholds of [section 8.1](#81-home), which are this design's starting values and nothing else defines | this document, until traffic tunes them |
| How a newly connected account, a credential replaced by re-authorization and a policy edit reach the workloads with no manual step and no restart, which [sections 8.7](#87-policy) and [8.12](#812-connect-an-account-and-re-authorize) assume | ROADMAP.md's unit [F7](../ROADMAP.md#group-f--foundation), the signals between deployables |
| A feedback verb on masking and gate events, which would be a third decision and needs its own record before it exists | [ROADMAP.md's open decisions](../ROADMAP.md#open-decisions), gated to the unit that builds the learned tier |

## 21. The mockups

Mockup sources and the notes a design session needs are at [ui/design/](../ui/design/README.md).
This document does not depend on them.
