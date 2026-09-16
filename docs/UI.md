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
[O4](../USE_CASES.md#o4--the-operator-can-see-and-steer). The rules that bound it are ADR-0021's,
via the [decision-record index](./adr/README.md). The UI carries two decisions as its only writes,
is read-mostly, and never displays a message body. Build state lives in
[ROADMAP.md](../ROADMAP.md), never here. Vocabulary is defined in
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

The operator's standing duties live here. Plan review, candidate review, and masking review are the
duties ADR-0021 names, and the operator added two more on 2026-09-10. Watching batch work as it runs
(backfill, delta sync, reorg apply, the heuristics job) and inspecting a run, failed or not, down to
its individual failures. The loop the UI is organized around is the operator's. Open it, see what
needs a decision, see what is worth a look, decide, and leave.

Every view is per account. The account is chosen explicitly, is always visible, and nothing is ever
aggregated across accounts. This is the operator's ruling, not a consequence of
[P3](../USE_CASES.md#p3--multi-account), whose isolation binds clients and credentials.

The UI has no authentication of its own in the first version. A deployment may place it behind
an ingress that forwards to an authenticating proxy and passes the identity in a declared header,
and the UI's own authentication may come later (ADR-0021). TLS holds
([section 15](#15-security-of-the-ui-itself)).

The UI is desktop-first. It is designed at 1440 pixels wide with a floor of about 1280, and the
first version has no mobile layout.

## 2. Reading order for the builder

The reading list is this document plus every decision record it cites. The order below is where
to start. Any record cited in a section and not listed here is read when that section is built.

1. [USE_CASES.md](../USE_CASES.md), the fixed point and
   [O4](../USE_CASES.md#o4--the-operator-can-see-and-steer). Ten minutes. It says what the UI is
   judged on and what it must never do.
2. ADR-0021, ADR-0016, ADR-0042, ADR-0047, and ADR-0051, via the [index](./adr/README.md). The
   UI's shape and grants, the schema it reads, the languages it is written in, how queries are
   written, the environment contract.
3. This document, in order. Sections 3 to 7 are the model every screen instantiates. Section 8 is
   the screens. Sections 14 to 19 are what the code must be. Section 20 is what remains open.
4. ADR-0056, ADR-0057, ADR-0058, ADR-0059, ADR-0060, ADR-0061, ADR-0062, ADR-0063, ADR-0064, and
   ADR-0065, via the [index](./adr/README.md), for why the shape, the read API, the live surfaces,
   the palettes, the no-code-in-the-database rule, the request token, the content security policy,
   the browser framework, the browser's tests, and the contract pipeline are what they are, and
   what was rejected.
5. ADR-0020, ADR-0032, ADR-0019, ADR-0004, ADR-0007, ADR-0005, ADR-0003, ADR-0002, ADR-0034,
   ADR-0022, ADR-0025, ADR-0018, for the mechanisms the screens display. Read each when building
   the screen that shows it.
6. [TESTING.md](../TESTING.md), ADR-0043, ADR-0044, and the UI's rows in
   [docs/VERIFICATIONS.md](./VERIFICATIONS.md), before writing a test.
7. [ROADMAP.md's open decisions](../ROADMAP.md#open-decisions) and
   [section 20](#20-what-remains-open), before touching anything either lists.

## 3. The lens model

Every view ADR-0021 names is the same shape. An account-scoped dataset, viewed at some aggregation
level, sliced by a few dimensions, drilled to individual rows, and for two of them a decision
attached to the row in view. That shape is a **lens**. The unit of design is the lens, not the
page.

| ADR-0021 view | Dataset | Natural dimensions | Row | Decision |
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
| L4 | Row detail | one row with its provenance, as a panel over the L3 list it came from | one message with both sensitivity axes, the rule that fired, scan state, audit trail, and what the plan does to it |

The navigation rules that make this analysis rather than page-hopping:

- Clicking any aggregate (a bar, a count, a group row) applies it as a filter and descends one
  level. The applied filters are a breadcrumb of chips under the top bar, in the order applied.
  The breadcrumb is a stack. Removing a chip removes it and every chip applied after it, and the
  view returns to the level that chip was applied at.
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
| account | mandatory, the first path segment, never implicit, never `all` |
| time range | `range=` with the grammar below. UTC on the wire and in the display, with the `Z` suffix, local time on hover |
| search | a filter named `search` on the `messages` dataset only, declared in the registry as a filter-only entry of kind text, legal at every level. A case-insensitive substring match over the masked subject through the index's trigram index. The search box submits `/{account}/messages?level=3&search=…`. Sender and label search are the ordinary `sender` and `label` filters |

**The route table.** The entry route first, then the fixed screens, the object screens, and the
lens routes, which exist only for the five analysis datasets. The URL is the view state. The
common parameters `sort=` and `page=` apply to every route that shows rows, and `level=`,
`group=`, and the dimension filters to every lens route and to `ops` and `failures`, and `range=` to
every dataset the default-range table gives a range.

| Route | Screen |
| --- | --- |
| `/` | redirects to `/{account}` for the account last used in this browser, else the first account by identifier |
| `/{account}` | Home ([8.1](#81-home)) |
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
| `/{account}/policy` and `/{account}/policy/{rule-id}` | Policy ([8.7](#87-policy)) |
| `/{account}/system` | System ([8.8](#88-system)) |
| `/{account}/{dataset}?group=a&level=N&range=…&{dimension}={value}…` | an analysis lens ([8.5](#85-the-analysis-lenses)), `{dataset}` one of `messages`, `senders`, `masking`, `gate`, `audit` |
| `/{account}/{dataset}/{row-id}` | one row, the panel over the lens's L3. Not for `senders`, which has no row detail; its rows link out to `messages?sender=` |

The fixed and object routes are matched before the lens route, so `plans`, `candidates`, `jobs`,
`policy`, and `system` are never read as dataset names. Switching account from an object route
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
audit row with no message falls into the `none` group of every message-derived dimension.

**Default range per dataset:**

| Dataset | Default range |
| --- | --- |
| messages, senders, candidates, plans, rules | all time |
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
| `rules` | policy rules | none | no |

The op log, the accounts table, and the rate state are read by the bespoke endpoints of
[section 17.4](#174-the-bespoke-endpoints), never as datasets. Row detail exists for `messages`,
`masking`, `gate`, `audit`, `ops`, `failures`, `runs`, and `rules`. `senders` has none. `plans` and
`candidates` have none as datasets, because their row paths belong to the bespoke screens.

**Areas**, the way the datasets and the bespoke reads group for the operator:

| Area | Datasets | Bespoke reads |
| --- | --- | --- |
| Corpus | messages, senders | |
| Sensitivity | masking, gate | |
| Policy | candidates, rules | |
| Change | plans, ops | the op log, through the plan endpoint |
| Audit | audit | |
| Jobs | runs, failures | the jobs summary, the run summary |
| System | | accounts, rate state, the system summary |

**Policy is the one exception to per-account rows.** Policy is configuration, not account data
(ADR-0004's base policy and ADR-0026's per-account overlays). `/{account}/policy` shows the base
rules, marked "base", and the account's overlay rules (ADR-0004). No row of another account's data
is shown, so the per-account rule of [section 1](#1-what-the-ui-is-for) is not breached. Nothing
else on any screen shows a row outside the account in the path.

**Decisions.** Two, each with two outcomes. A plan is approved or rejected, a candidate is
confirmed or dismissed. Four requests carry them, all by database grant (ADR-0021), which calls
them its two write verbs. No request exists for retrying a job, triggering a rollback, or editing
policy.

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
| Top bar | the account selector, then the primary navigation (Home · Plans · Jobs · Corpus · Masking · Gate · Audit · Review queue · Policy · System), then search, then the settings menu, then the live indicator on live surfaces | The account selector shows the account identifier and its provider. Its menu lists every account from the accounts endpoint, each with its provider, and nothing else. Switching accounts keeps a lens's dataset and level and drops its filters, and follows [section 5](#5-information-architecture-and-the-url)'s rule from an object route. The settings menu holds the theme override (system, dark, light) and the keyboard map |
| Address line | the URL of the current view, as text, selectable | It is the share handle. It updates on every navigation |
| Breadcrumb | the applied filter chips in the order applied, each removable, preceded by the lens or object name | Absent on Home. On a plan or a run it starts with the object |
| Range control | on every dataset with a range, the presets (last 24 hours, 7 days, 30 days, 90 days, all time) and a custom UTC range with a start and an end, writing `range=` | Sits at the right of the breadcrumb line |
| Group-by control | the dataset's groupable dimensions, in the registry's order, writing `group=` | Sits beside the range control, on levels 1 and 2 |
| Body | the screen | |

The primary navigation marks the current screen. Review queue shows the count of pending
candidates and Jobs shows a running mark when any workload is running, both read from the system
endpoint's decisions block ([section 17.4](#174-the-bespoke-endpoints)) on every page load and
updated from decision responses and the stream. Nothing else in the chrome carries a number.

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
class with the rule id that assigned it, content flags with the rule ids that fired, scan state
with the time scanned and the scanner version), then the audit rows for this message as an L3
list, then, when reached from a plan, what the plan does to it and why. Never a body, never a
snippet. The database has no column to show (ADR-0016).

### 7.2 The sender row, the audit row, and the page row

Three more row types exist, each with the same rendering rules as the message row.

| Row | Fields, in order | Where |
| --- | --- | --- |
| sender row | domain (mono, links to `messages?sender=` this domain), sender class badge, message count, first seen, last seen, list-id ratio as a percent, scan hits | the `senders` dataset at L3, the review queue's sender statistics |
| audit row | time, actor (mono), action wording, then the message row's fields when the row has a message and nothing when it does not | the `audit` dataset at L3, the row detail's audit list |
| page row | page number, cursor (mono), error class, attempts, last error time, disposition | a run's failures whose item is a page rather than a message |

The dimension `sender` is the domain (`from_domain`) everywhere. The filter `from` is the address.

### 7.3 Charts

Every chart is inline SVG drawn from the same rows its table shows.

| Form | Rules |
| --- | --- |
| bars (L1 and L2) | horizontal, 20 px tall with 8 px gaps, filling the region's width, one bar per group, sorted by count descending, the top 20 groups then one `other` bar for the rest, which is not clickable. Direct label with the count at the bar's end, a hairline baseline in the border token, no gridlines, bar fill the neutral chart fill with the restricted share drawn as an inner segment in the restricted color. The table beside it lists every group, 50 per page. When the dimension is an array (labels, content flags, rule ids) a row counts in each of its groups, shares are over the total count, and the chart carries the note "a message with several labels counts in each" |
| flow diagram (a plan's flows) | 320 px tall. Old labels in a column on the left sorted by outflow descending, with a `none` row for flows that remove nothing, new labels in a column on the right sorted by inflow descending, one straight band per flow drawn in the left column's order with 2 px gaps, band width proportional to message count, the selected band in the action color and the rest in the neutral fill |
| run timeline | 120 px tall. x is wall time from the run's start to its finish or to now, marks for start, backoff, retry, failure, resume, and finish drawn from the run's events; for paged runs y is the page index and the progress events trace a line; for runs without pages the marks sit on one line |
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

**Worth a look**, fed by the attention endpoint, which returns each card complete. Zero to a
handful of cards, newest first. Each card is the what, the number, since when, the sentence for
its rule, and a link into the lens that explains it with the value applied as a filter. The rules,
their starting thresholds, and their sentences are the table below. They are this design's own,
undefined anywhere else, and starting values. Every threshold is a configuration key of
[section 18.1](#181-the-configuration-the-ui-declares), read at start, and a value of 0 disables
the rule.

| Rule | Fires when | Sentence |
| --- | --- | --- |
| Backlog | pending scan above 5% of the corpus | "{count} messages are pending scan ({share}% of the corpus). Every pending message denies its body until scanned, which reads to the agent like a permission problem." |
| Masking | one sender above 20 events in 7 days under one rule | "Masking fired {count} times on {sender} this week, all under {rule}. A sender masked this often under one rule is worth checking for an over-mask." |
| Body serves | more than twice the 7-day median in 24 hours, the anomaly [A4](../USE_CASES.md#a4--released-bodies-are-clean-markdown-that-cannot-do-anything) names | "{count} bodies were served in 24 hours against a 7-day median of {median}. Body-serve volume beyond triage plausibility is the anomaly the design watches for." |
| Sync gap | one recovered in the last 7 days | "Delta sync recovered from a cursor gap on {time}, re-enumerating a {window} window and reconciling {count} messages. A repeated gap means the cadence or the cursor lifetime needs attention." |
| Expiry | a DRAFT plan within 2 days of its maximum age | "Plan {title} expires in {remaining}. After that the apply job refuses it and the client must propose again." |

**System**, the right column, fed by the system endpoint's three blocks. As-of time first, then
one row per value, each a link.

| Block | Value | Links to |
| --- | --- | --- |
| operational | backfill pass 1, complete or its progress | Jobs |
| operational | backfill pass 2, percent and pending count | the corpus lens with `scan_state=pending` |
| operational | sync cursor age and last successful tick | Jobs |
| operational | rate, current of target, cap, backoff wording | Jobs |
| operational | last provider authentication outcome and time | System |
| corpus | audit last 24 hours, one row per action (body served, body denied, mutation applied, mutation refused) | the audit lens at L3 with `action=` that value and `range=24h` |
| corpus | messages | the corpus lens at L1 |
| corpus | threads | the corpus lens at L1 |
| corpus | unfiled, with percent | the corpus lens at L3 with `label=none` |
| corpus | restricted messages | the corpus lens at L3 with `sender_class=restricted` |
| corpus | masking events, last 7 days | the masking lens at L1 |
| corpus | gate skips of decisions, last 7 days | the gate lens at L1 with `decision=skip` |

States. With no plans and no candidates, the inbox says "Nothing awaits your decision" and the
column keeps its height. With backfill pass 1 not started, the strip shows the workload as not
started and the corpus rows say the index is empty. While a pass runs, the partial-index banner
of [section 12](#12-empty-loading-partial-and-error-patterns) sits above the columns and the corpus
rows show counts so far. A failed read on one region shows that region's error card and leaves
the others alone.

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
endpoint.

Cadence and next run time are configuration displayed, not recorded state. The cadences are
intervals, ADR-0018's sync interval and the heuristics interval, both configuration keys of
[section 18.1](#181-the-configuration-the-ui-declares), and next run is last run plus the
interval.

**Rate budget**, from the rate block. Current of target and the cap in units per second, the
backoff wording, last throttle time. Then one bar per priority class of ADR-0025 (interactive,
sync, batch) showing used of reserved, with the sentence that interactive keeps its reservation
and batch absorbs any reduction first.

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

### 8.4 Run

`/{account}/jobs/{run-id}`. Any run opens it. Live while the run or its resumer (the later run
whose `resumed_from` names this one) is running. Five regions, fed by the run summary endpoint
(L0 and the timeline) and the `failures` dataset with this run as parent (everything below). With
zero failures, L1 and L3 collapse to one line, "no failures".

**L0 strip.** Workload and pass, run id, run state, started and finished times, duration, where it
failed (the checkpoint and the retries of it), item failures, how many recovered and by which run,
how many permanently gone, and the resuming run with its state.

**Timeline.** The run timeline of [section 7.3](#73-charts), drawn from the run's recorded events.
A legend names the six mark kinds. Start, backoff, retry, failure, resume, finish. Progress events
draw the line and carry no mark. A run with no events shows start and finish only.

**L1.** Failures by error class as bars with count and share, clickable to filter, and a second
small breakdown by disposition. The group-by control offers error class, sender, page, and
disposition.

**L3.** The failed items. Message and operation items render as message rows with these columns
after the standard ones. Page, error class, attempts, last error time, disposition (and the
recovering run where recovered). Page items render as page rows. The filter chip from L1 applies
here.

**L4.** One item in a panel with three parts, in order. What happened ("The run recorded:
{error_summary}"), what it means for the message (the sentence for its disposition, below), and
provenance (run, page, attempts, first and last error time, sender class, content flags,
disposition, and the error summary as recorded). The error summary is provider or scanner text
and never a body. The panel ends with two links, the message in the corpus lens and its audit
rows.

| Disposition | What it means, as shown |
| --- | --- |
| recovered | "{error class} on this item. Run {run} retried it successfully. Its scan state is {scan state}." |
| pending | "{error class} on this item. It has not been retried yet. Its scan state stays pending, so its body is denied until a run reaches it." |
| gone | "{error class}. The message was removed at the provider after it was indexed. Its scan state stays pending, so its body is denied, and the next delta-sync tick will remove the row from the index." |
| abandoned | "{error class} on this item after {attempts} attempts. The workload gave up. Its scan state stays pending, so its body is denied until a later run reaches it." |

The screen has no retry request. Resumption is the workload's own behavior, and the UI shows it.

### 8.5 The analysis lenses

Each is `/{account}/{dataset}` with the ladder of [section 4](#4-the-zoom-ladder), fed by the
dataset endpoint. The L0 strip shows the lens's named figures from the registry's summary query.
Aggregates sort by count descending, rows by time descending, 50 rows per page, and `sort=`
overrides both within the registry's sortable columns. Corpus has two tabs in its L0 strip,
Messages and Senders, each a route.

| Lens | L0 figures | Default view | Dimensions offered | Row |
| --- | --- | --- | --- | --- |
| Corpus, Messages tab (`messages`) | messages, threads, unfiled with percent, restricted messages, pending scan | L1 by sender, all time | sender, label, sender class, scan state, content flag, month | the message row |
| Corpus, Senders tab (`senders`) | senders, restricted senders, senders with scan hits | L3 by message count | sender class, month of first seen | the sender row |
| Masking (`masking`) | events, rules that fired, senders | L1 by rule, last 7 days | rule, tier, sender, day | the message row plus rule and tier |
| Gate (`gate`) | decisions, scans, skips, and pending backlog, which is unranged and labelled "now" | L1 by decision, last 7 days | decision, reason, sender, day | the message row plus decision and reason |
| Audit (`audit`) | body served, body denied, mutation applied, mutation refused, and the body-serve count against its median over the seven whole UTC days before the range's start | L1 by action, last 24 hours | action, actor, sender class, rule, hour | the audit row |

The corpus lens's label grouping shows unfiled as its own group. A sender row's domain links to
`messages?sender=` that domain. Audit rows without a message fall into the `none` group of the
message-derived dimensions and count as neither restricted nor flagged.

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
fired, ADR-0016).

| Heuristic | Template |
| --- | --- |
| display-name matching | "display name {name} matches listed {domain}" |
| registrable-domain clustering | "registrable-domain clustering with listed {domain}" |
| institution keyword | "institution keyword {keyword} in domain" |
| transactional pattern | "transactional pattern (noreply, no List-Id, never labeled)" |
| embedding similarity | "similar to confirmed {domain} ({score})" |

`/{account}/candidates/{domain}`. Three regions and the outcomes.

| Region | Content |
| --- | --- |
| Signals | one row per heuristic that fired, in the order above, with the evidence recorded for it (the display name and the listed domain it matched, the listed domain it clusters with, the keyword, the transactional pattern's three facts, the similarity score and the nearest confirmed sender) |
| Sender statistics | the sender row's fields, then display names seen, sampled local parts, the label distribution as label and count pairs, and the current sender class |
| Messages | the sender's messages as message rows, L3, newest first |

Above the outcomes, one sentence: "Confirming makes {domain} a restricted sender. Its bodies deny
from the next classification and its messages become organize-only." Dismiss writes the candidate's
status, reviewed time, and reviewer, the columns ADR-0021 grants. Confirm writes the same three and,
in the same transaction, inserts the policy rule row the confirmation emits (ADR-0004), with rule id
`candidate.{account}.{domain}`, the candidate's account as its account (an overlay rule, never the
base policy), the candidate's domain as its suffix, the restricted class, the source `candidate`,
and the operator identity as its creator. Both are one transaction written by the UI's own code,
with no database-resident code (ADR-0060). The request carries the domain and the status the screen
shows, pending or dismissed, so a dismissed candidate may be confirmed later. A confirmed candidate
cannot be dismissed from the UI. Undoing a confirmation is a policy edit outside the UI. After
either outcome the row stays in place with its new status until the operator navigates, the pending
count in the navigation updates from the response, and focus stays where it was. A confirmed row
shows "confirmed, not yet in effect" until the index's sender class for the domain reads restricted,
then "in effect".

### 8.7 Policy

`/{account}/policy`. Read-only, the `rules` dataset. Rows sorted by rule id, the base rules
marked "base" and the account's overlay rules after them, each with rule id, class, domain
suffixes, source (operator, or the candidate it came from), created time and identity, and the
count of senders in the index it matched. A rule opens to the senders it matched as sender rows.
A sender matches a rule when its domain, normalized as ADR-0004 states, ends with one of the
rule's suffixes at a label boundary, and a sender counts under every rule it matches. Policy is
held as rows and snapshotted by every process (ADR-0004, ADR-0041), and this screen is the one
exception to per-account rows stated in [section 5](#5-information-architecture-and-the-url).

### 8.8 System

`/{account}/system`. The account's identifier and provider, then the system endpoint's
operational block, the values ADR-0034 exposes to clients, one to one. Backfill pass 1 and pass 2
flags with progress where a pass runs, sync cursor age and last successful tick, scan backlog
depth, rate controller state (current of target, cap, backoff wording, last throttle), and the last
provider authentication outcome with its time. Each links where Home's operational rows link.

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

## 9. Live surfaces

The home's running-work strip, Jobs, and a run while it or its resumer runs update without a
reload. Each shows the live indicator in the top bar, a dot in the ok color with "live · updated
Ns ago", and pauses its subscription while the tab is hidden, resuming and refetching when it
becomes visible. A dropped stream shows "live · reconnecting" and the view keeps its last data.
The transport, the stream's content, and its cadence are ADR-0058, proposed and not yet ratified.
Only the stream client module depends on that record. The screens read object shapes
([section 17.5](#175-the-streams-event-shape)) that do not change with the transport, so a
different ratification replaces one module.

## 10. Decisions and their friction

The two decisions are rare and dangerous, so friction scales with blast radius. Approving restates
what is being approved in one sentence with the numbers. A plan at or over a quarter of the corpus
demands a typed restatement before Approve enables ([section 8.2](#82-plan-reviewer)). Rejecting
and dismissing are plain outlined controls with no confirmation. Confirming a candidate shows what
it will do in one sentence above the control ([section 8.6](#86-review-queue)). Decisions happen
only on the object's screen, never from a list row, so the sentence is always present. Every
request carries the status the screen shows, and a mismatch is refused as a conflict. Every
decision records who made it in the decided row's own columns, the identity coming from the
declared header or the configured operator name (ADR-0021). The plans list and the review queue
show decisions from those rows. No audit row is written for a decision. The audit log holds bodies
served or denied and mailbox mutations, and applying an approved plan is audited by the engine as
the mutation it is.

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
| candidate status | pending · confirmed · dismissed | Awaiting review · Confirmed, not yet in effect (then In effect) · Dismissed |
| scan state | scanned · skipped_restricted · skipped_gate · pending | in cells `scanned` · `restricted` · `gate skip` · `pending`; on hover and in L4 "scanned" · "not scanned, restricted sender" · "released unscanned, gate skip" · "pending content scan" |
| sender class | normal · restricted | normal · restricted |
| content flag | mfa_code · login_link | mfa · link |
| gate decision and reason | SCAN · SKIP, with the reason as ADR-0007's skip branches name it | scan · skip, with the reason as recorded |
| audit action | READ_BODY · DENY_BODY · MUTATE · DENY_MUTATE | body served · body denied · mutation applied · mutation refused |
| run state | running · succeeded · failed | running · succeeded · failed |
| workload state (derived, not stored) | no run yet · a run in running · the last run finished | not started · running · idle |
| backoff state (derived from `backoff_until`) | null · a future time | not in backoff · in backoff until {time} |
| validation result | passed · failed | passed · failed |
| error class | throttled · provider_error · gone · scanner_timeout · validation · authentication | provider throttled · provider error · gone at provider · scanner timeout · validation · authentication |
| disposition | recovered · pending · gone · abandoned | recovered (with the run) · pending · gone · abandoned |
| failure item kind | page · message · op | page · message · operation |

The scan-state wording for pending is the denial envelope's own phrase (ADR-0002, ADR-0007), so
the operator and the agent read the same words. The plan statuses are ADR-0020's and the three job
vocabularies are ADR-0016's.

## 12. Empty, loading, partial, and error patterns

| Situation | Pattern |
| --- | --- |
| Backfill pass 1 running | A partial-index banner under the chrome on every screen, saying which pass is running and how far (pages of pages, percent), with a link to Jobs. Counts on every lens carry "so far" |
| Backfill pass 2 running | The banner names pass 2 and the pending count, and says pending messages deny their bodies until scanned. With both passes running, one banner names both |
| A lens with zero rows | The L0 strip with zeros and one line, "No {rows} match", with the chips still shown so the operator can remove one |
| First load of a region | A skeleton of the region's shape (bars for a chart, lines for a table). After one second the skeleton gains the words "still loading". After ten seconds the region shows the error card with a retry link. Never a spinner over the whole page |
| A live surface waiting for its first event | The region renders from its fetch and the indicator reads "live · connecting" |
| A failed read | An error card in the region's place with the origin from the error contract of [section 17.3](#173-the-error-contract). "This request was refused" for the client's fault (with the message), "The UI server failed" for its own, "The database did not answer" for the database, each with the request id and a retry link. Other regions stay |
| A decision refused | The footer or outcome area shows the refusal inline, with the origin and message, and the object reloads on a conflict. A missing declared identity reads "No identity was forwarded, so the decision was not recorded" |
| Search with no result | The corpus lens at L3 with the chip and "No messages match" |

## 13. Keyboard map

A starting proposal, marked as design. The builder may change bindings, not the principle that
every navigation in [section 4](#4-the-zoom-ladder) has a key. The map is shown from the settings
menu and on `?`.

| Key | Action |
| --- | --- |
| `j` / `k` | move the row cursor down / up in any table |
| `Enter` | open the row under the cursor (descend, or open the detail at L3) |
| `Escape` | close the detail panel, or remove the last chip (ascend) |
| `/` | focus search |
| `g` then `h` `p` `r` `j` `c` `m` `t` `u` `o` `s` | go to Home, Plans, Review queue, Jobs, Corpus, Masking, Gate, Audit, Policy, System |
| `[` / `]` | previous / next page |
| `a` in a plan or candidate | focus the approve or confirm control (never submits) |
| `?` | show the map |

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

## 15. Security of the UI itself

The UI holds no provider credential and cannot reach a mailbox (ADR-0021). What remains is the
browser, the transport, the database connection, and the two decisions.

- **TLS, and the same posture as the client surface** (ADR-0021). The UI serves TLS from the
  material its configuration declares ([section 18.1](#181-the-configuration-the-ui-declares)).
  The "own auth" half of ADR-0021's posture is, for the first version, the operator's ruling of
  no authentication of the UI's own with an optional authenticating proxy in front
  ([section 1](#1-what-the-ui-is-for)).
- **Content security policy** (ADR-0062). `default-src 'self'`, `script-src 'self'`,
  `style-src 'self'`, `img-src 'self' data:`, `font-src 'self'`, `connect-src 'self'`,
  `frame-ancestors 'none'`. No inline script, no external origin at runtime, fonts self-hosted.
  The mockups' Google Fonts link is a mockup convenience and is not product.
- **The entry document, the session, and the request token** (ADR-0061). The browser bundle is
  static (ADR-0042), and the one exception is the entry document, which a Go handler renders on
  every page load to place the request token in a `meta` tag. The policy allows it because it is not
  a script. The UI has no authentication of its own, so the session is anonymous and exists solely
  to bind the request token, whether or not an authenticating proxy sits in front. On the first
  response the UI issues the cookie `ui_session`, a random id, `HttpOnly`, `Secure`,
  `SameSite=Strict`, with browser-session lifetime. The token is an HMAC of the session id under a
  key generated at process start, or the configured `UI_TOKEN_KEY` when more than one replica runs,
  and its lifetime is the session's. Every decision request sends it in the `X-Request-Token`
  header, and the server checks it against the cookie, so a request forged from another origin
  fails. The four decision requests are `POST`, carry the status the screen shows for conflict
  detection, and are the only state-changing requests.
- **The identity header is trusted only when the deployment declares it** (ADR-0021). With the
  identity header name configured ([section 18.1](#181-the-configuration-the-ui-declares)), the
  UI records that header's value on decisions and refuses a decision when the header is absent (403,
  `identity_missing`, [section 17.3](#173-the-error-contract)). Unset, the UI records the
  configured operator name and trusts nothing from the request.
- **The database connection carries the account.** Per request the UI opens a transaction and
  sets the transaction-local setting `app.account`, which the row-level security policies of
  ADR-0016's third layer read, by an ordinary statement like every other process.
- **A decision is one transaction, written by the UI's own code.** The status, the decision time,
  and the identity are written together, and a confirmation also inserts its policy rule row in
  the same transaction (ADR-0021). No code runs inside the database to complete a decision
  (ADR-0060). A failure anywhere in the transaction fails the decision whole, reported with the
  database origin, and nothing is recorded. The guarantee that no path writes one row without the
  others is carried by the tests of [section 18](#18-repository-and-build-layout).
- **Message-derived text is inert** ([section 11](#11-rendering-and-formatting-rules)).
- **The UI never calls the mediator.** It has no route to it and no credential for it. Every read
  is the UI's own database role against the tables ADR-0021 grants.
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
unknown account is a client error (400). Exactly one endpoint is unscoped, `GET /api/accounts`,
because the accounts list is what the account selector and the entry redirect are built from. The
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
`as_of` is the read transaction's start time.

```json
{ "account": "personal", "dataset": "masking", "level": 0, "as_of": "2026-09-10T10:16:04Z",
  "figures": [
    { "key": "events", "wording": "events", "value": 312, "unit": null, "link": "/personal/masking?level=3&range=7d" }
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
the unfiled label group and for an audit row with no message. Row pages are offset-paginated, 50
rows per page, with the page count, and no other page size exists. Every sort ends with the row's
own identity as its final key (ADR-0057). A row is the dataset's row type, which begins with the
message-row fields of [section 7.1](#71-the-message-row) where the dataset is message-derived, and
every row carries its identity (`message_id`, or `id` for masking and audit rows, or `seq` for
failures).

`GET /api/{account}/{dataset}/{row-id}` (with the parent filter for a nested dataset) returns one
row with its provenance for L4, including the message's audit rows, for the datasets that declare a
provenance query. `senders` declares none. `plans` and `candidates` declare none, because their row
paths belong to the bespoke handlers, and the contract generator refuses a path claimed by both
sources.

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
| default | the default group, level, range, and sort |
| summary | the query for the L0 figures, each with its key, wording, unit, and link |
| queries | one statement per groupable dimension for the aggregate levels, plus one for rows, each parameterized by account, filters, sort and page, written against the schema per ADR-0047 and enumerated rather than composed per ADR-0066 |
| provenance | optional. The query that fetches one row's detail. `senders`, `plans`, and `candidates` declare none |

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
| `client` | 403, code `identity_missing` | the deployment declares an identity header and the request carries none |
| `ui` | 500 | the UI server failed on its own |
| `database` | 503 | the database did not answer, refused, or a decision's transaction failed |

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
| `GET /api/{account}/jobs/{run}` | the L0 fields, the resumer found by `resumed_from`, and the timeline events (kind, time, page, detail) |
| `GET /api/{account}/attention` | the "worth a look" cards, each with rule id, what, number, since, the sentence, and the lens URL |
| `GET /api/{account}/system` | three blocks. `operational`, the values of [section 8.8](#88-system); `corpus`, the at-a-glance figures of Home's System column; `decisions`, the counts of plans in DRAFT, candidates pending, and workloads running, which the chrome's counters read |
| `GET /api/{account}/events` | the live stream, `text/event-stream` |

Every `POST` carries the `X-Request-Token` header of
[section 15](#15-security-of-the-ui-itself). A decision's body never carries anything the server
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

The cadence, reconnection, and fallback rules are ADR-0058's.

## 18. Repository and build layout

Per ADR-0054 the UI is one top-level directory holding both halves, and per ADR-0042 the server
half is Go and the browser half is TypeScript. Design, not yet built.

```text
ui/
  main.go               the composition root. It embeds browser/dist and hands it to the server
  Dockerfile            the bundle stage, then the Go stage, then a minimal base
  internal/
    registry/           the dataset registry entries, each pointing at the data-access accessors it reads
    api/                the server. The entry document, static files, the read API, the decisions, the stream,
                        the bespoke handlers, the error contract, the request token, identity
    contract/           builds the OpenAPI document from the registry and the handler list
    core/               pure-core packages private to the UI
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
      screens/          home, plan, plans, jobs, run, candidates, policy, system
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
  HTTP under `UI_INSECURE_HTTP=true`, which the server refuses outside the dev loop (a build
  tag or the presence of the fixtures database, the builder's pick). The browser app runs under
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
declaration, as design, with illustrative key names. The names are settled when the chart of
ADR-0052 carries them. Every key is read at start.

| Key | Value | Required |
| --- | --- | --- |
| `UI_DATABASE_URL` | the Postgres connection for the UI's own role (ADR-0021) | yes |
| `UI_LISTEN` | the address and port to serve on | yes |
| `UI_TLS_CERT`, `UI_TLS_KEY` | paths to the TLS material, mounted as files (ADR-0038's convention) | yes, unless `UI_INSECURE_HTTP` |
| `UI_INSECURE_HTTP` | `true` serves plain HTTP, refused outside the dev loop | no |
| `UI_IDENTITY_HEADER` | the header name an authenticating proxy forwards; when set, its value is recorded on decisions and a decision without it is refused | no |
| `UI_OPERATOR_NAME` | the identity recorded on decisions when no header is declared | yes when the header is unset |
| `UI_TOKEN_KEY` | the key behind the request token, replacing the per-process key when more than one replica runs | no |
| `UI_MAX_PLAN_AGE` | the maximum plan age, from which `expires_at` is computed. The value's home is the roadmap's open decision, and this key mirrors it | yes |
| `UI_SAMPLE_SIZE` | the plan sample's size | no, defaults to 24 |
| `UI_SYNC_INTERVAL`, `UI_HEURISTICS_INTERVAL` | the intervals displayed on the jobs cards (ADR-0018's sync interval, the heuristics interval) | no, defaults to the records' values |
| `UI_ATTENTION_BACKLOG_SHARE`, `UI_ATTENTION_MASK_COUNT`, `UI_ATTENTION_SERVE_FACTOR`, `UI_ATTENTION_GAP_DAYS`, `UI_ATTENTION_EXPIRY_DAYS` | the "worth a look" thresholds of [section 8.1](#81-home), one key per rule, 0 disabling the rule | no, defaults apply |
| `UI_DEFAULT_THEME` | `system`, `dark`, or `light` | no, defaults to `system` |
| `UI_STREAM_INTERVAL`, `UI_STREAM_RECONNECT_MAX`, `UI_STREAM_POLL_INTERVAL` | the poll cadence behind the event stream, the reconnection backoff ceiling, and the polling fallback interval (ADR-0058) | no, defaults to the record's values |

### 18.2 The UI's own observability

The UI emits and never collects (ADR-0051). Every request is logged with its request id, the
account, the route, and the outcome classified by the error contract's origin. Metrics carry
read latency by dataset and level, stream subscriber count, and decision outcomes by decision and
result. No log line and no metric label ever carries message-derived text. This is the UI's share
of [O2](../USE_CASES.md#o2--observable).

## 19. Building it

**Order of work.** The order the UI's pieces are built in is build state. The unit that carries
them lives in [ROADMAP.md](../ROADMAP.md), and the order lives in that unit's tickets, never here.

**State model.** Route state is the URL. Data state is per request, keyed by the URL, cached for
a few seconds. Live surfaces hold one stream subscription whose events replace the matching
object in the data state. No global mutable store beyond these three. Which mechanism carries
each is ADR-0063's decision.

**What not to do.** Do not add a rendering path that interprets message-derived text. Do not add a
decision, however small, without a record that widens ADR-0021's grant. Do not aggregate across
accounts. Do not proxy anything through the mediator. Do not fetch a corpus into the browser to
group it there. Do not paint a fake status bar or keyboard in any layout. Do not introduce a
color outside the token tables without re-running the palette derivation and, for a chart series,
its validator. Do not read a header for identity unless the deployment declared it. Do not keep
view state outside the URL. Do not put a trigger, a procedure, or a function in the database for
anything (ADR-0060).

**Tests the UI owes.** Every control the UI carries is dispositioned in
[docs/VERIFICATIONS.md](./VERIFICATIONS.md), as an injection row keyed to M3, M5 or F4 or as a
standing disposition, and that catalogue, not this document, is the list.
[TESTING.md](../TESTING.md) decides the kinds.

## 20. What remains open

Everything this design relies on about the system is stated by a record and cited where it is
used. What is still open, and where it is tracked:

| Open | Tracked in |
| --- | --- |
| The live-update transport. ADR-0058 is Proposed; only the stream client depends on it ([section 9](#9-live-surfaces)) | [ROADMAP.md's open decisions](../ROADMAP.md#open-decisions) |
| The maximum plan age value, which `expires_at` and the expiry rule of [section 8.1](#81-home) read from configuration | the same table |
| The "worth a look" rules and thresholds of [section 8.1](#81-home), which are this design's starting values and nothing else defines | this document, until traffic tunes them |
| A feedback verb on masking and gate events, which would be a third decision and needs its own record before it exists | [ROADMAP.md's open decisions](../ROADMAP.md#open-decisions), gated to the unit that builds the learned tier |

## 21. The mockups

Mockup sources and the notes a design session needs are at [ui/design/](../ui/design/README.md).
This document does not depend on them.
