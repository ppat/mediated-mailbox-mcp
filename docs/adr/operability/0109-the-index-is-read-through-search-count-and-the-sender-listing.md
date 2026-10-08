# 0109. The index is read through three task-shaped operations, a search, a count that groups, and the sender listing

**Status:** Accepted ·
**Pillar:** [Metadata always flows; sensitive bodies never do](../../../DESIGN.md#metadata-always-flows-sensitive-bodies-never-do) ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility), [G2](../../../USE_CASES.md#g2--historical-understanding)

## Context

[G1](../../../USE_CASES.md#g1--whole-mailbox-visibility) asks that the agent enumerate, count, sort,
search and reason over the entire mailbox as structure, restricted mail included.
[G2](../../../USE_CASES.md#g2--historical-understanding) asks that it derive the patterns already in
the mailbox, label distributions, senders with no label and the senders behind most unfiled volume,
from metadata alone. [ADR-0017](../data/0017-two-pass-backfill.md) builds per-sender aggregates in
backfill's first pass for this, held in the `senders` table of [ADR-0016](../data/0016-schema.md).

The message and thread listings page the whole index newest first. Selecting, sorting, counting
and grouping, and reading the sender aggregates, need operations beyond them, and which operations
carry them is this record's.
[ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md)'s R8 asks
for tools shaped around tasks and few enough to choose between, and its registry gives each
operation one route, a structured read a `:search` route, and a read with only scalar arguments a
`GET`. How the operations select messages is
[ADR-0108](./0108-index-reads-select-by-an-index-query-of-the-surfaces-own.md)'s index query.

## Decision

- **Three operations read the index, and none calls a provider.**

  | Operation | Effect, route | Takes | Returns |
  | --- | --- | --- | --- |
  | `search_messages` | Structured read, `POST …/messages:search` | `query`, `sort` of `date`, `sender` or `subject` with `date` the default, `order` of `descending` or `ascending` with `descending` the default, `cursor` | A page of the selected messages, each the message the other reads serve |
  | `count_messages` | Structured read, `POST …/message-counts:search` | `query`, optional `group_by` of `sender`, `sender_domain`, `label`, `sender_class` or `month`, `cursor` | A `summary` of the whole selection, the `group_by` given, a page of `groups`, and the next cursor |
  | `list_senders` | Read, `GET …/senders` | `cursor` | A page of sender statistics, one per sender domain |

- **A summary and each group carry the same counts.** Messages, distinct threads, unread messages,
  messages with attachments, and the dates of the oldest and the newest message, null when nothing
  is selected. One call without `group_by` answers
  [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)'s own question, how many unread financial
  notices, how many with attachments, and the oldest date.
- **Group keys.** A sender group's key is the address, a domain group's the domain, a label group's
  the label, a class group's `normal` or `restricted`, and a month group's the UTC instant the month
  starts. A message counts in the group of every label it carries, and the messages with no label
  form one group whose key is null. A group with no message is not returned.
- **Paging.** Groups come the one with the most messages first, ties broken by key, with the
  no-label group first of all. Messages come in the order asked, ties broken by message identifier.
  Sender statistics come the sender with the most messages first, ties broken by domain. Every page
  holds up to a hundred rows, and its cursor is bound to the operation, the account, the query, the
  sort and order, and the grouping, so a cursor given with any of them changed is refused, as
  [ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md) binds a
  cursor to its listing and filter. The statements page the messages and the groups by sender and by
  domain, which can be as many as the corpus has. The groups by label, month and class are as many
  as the account's labels, the months its mail spans and the two classes, so the service layer reads
  them whole and pages them.
- **The sender listing serves the statistics backfill builds**, the domain, its number of messages,
  its first and last message, the share of its messages carrying a `List-Id`, its label
  distribution, and its sampled display names and address local parts. Its sender class is the
  Redaction Gate's under the policy in force, restricted when any address of the domain the index
  holds is restricted, and never the class stored with the statistics. A domain whose statistics
  outlive every message the index holds for it is classified by its domain alone, as the classifier
  classifies any address at that domain, and is restricted when the classifier cannot read the
  domain. The prior scan hits are not served. The hits are a signal derived from bodies that no
  outcome needs a client to read. The statistics hold no embedding
  ([ADR-0016](../data/0016-schema.md)), and one the heuristics store later serves the heuristics
  alone, so it is not served either.
- **Everything served is metadata the redaction matrix shows for every sensitivity state**
  ([ADR-0001](../redaction/0001-redaction-matrix.md)). A searched message is the same served
  message, with its sender class and body availability decided under the policy the call took once,
  the same policy any sender class term or grouping of the call uses
  ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)). Counts, group keys and sender
  statistics carry no body-derived field.
- **Each operation reads one snapshot of the index.** A search, a count and a page of the sender
  listing run their statements, the read of the sender addresses the classification takes included,
  in one transaction at the repeatable-read isolation level, read-only, which the transaction helper
  opens for them ([db/README.md](../../../db/README.md)). The class a call selects or groups by, the
  class it serves, a count's summary and its groups and the page they sit beside all describe the
  same index, while a workload commits meanwhile. In the sender listing the snapshot changes nothing
  a client can see, since a domain the address read missed is classified by its domain alone, as its
  addresses would be, so its use there is held by review rather than by a test.
- **Every timestamp crossing the three is UTC with the `Z` suffix**, and one given with any other
  offset is refused ([ADR-0033](./0033-utc-only-timestamps.md)).
- **The mediator reads the statistics through the data-access subsection for senders, and the
  rebuild of the statistics and the counts of prior scan hits sit in a subsection below it**, which
  only the deployables that build the statistics admit, as
  [ADR-0066](../data/0066-data-access-generated-from-sql.md) places a statement a role admitted to
  its table's subsection may not be granted. Delta sync's removal of the statistics of a sender with
  no stored message sits in its own subsection for a message's changes, which the mediator does not
  admit either. The mediator's role holds a read of the sender domain on messages and of the
  statistics' columns, apart from the stored sender class
  ([ADR-0075](../data/0075-one-runtime-role-per-deployable.md)) and the prior scan hits.

## Alternatives considered

- **One tool per question**, such as a count of unread messages, a label distribution and a list of
  unfiled senders. For it, each tool's name says what it answers. Against it, the questions an agent
  asks of its mailbox are open-ended combinations of the same terms, so the tools multiply past what
  R8 allows an agent to choose between, and each repeats the selection.
- **Counting folded into the search**, a search returning counts beside its page. For it, one tool
  fewer. Against it, the counts of a whole selection and a page of its messages are different reads,
  and a grouped count has no page of messages to sit beside. The count also needs its own route,
  since the registry gives each operation one, and a structured read's route ends in `:search`.
- **A grouping by an arbitrary column the client names.** For it, open-ended analysis in one
  argument. Against it, an undeclared dimension would reach the database, which
  [ADR-0066](../data/0066-data-access-generated-from-sql.md) refuses, and every dimension the closed
  set leaves out is one no outcome asks for.
- **Sender aggregates computed from the messages on each call.** For it, the statistics follow the
  index exactly and could take the index query. Against it, it duplicates backfill's aggregation in a
  second place, and the table exists to keep those reads cheap
  ([ADR-0016](../data/0016-schema.md)). A grouping of a selection by sender domain is
  `count_messages`'s already.
- **Sender statistics serving the stored class.** For it, the statistics read as one row each, with no
  classification on the call. Rejected for the reason
  [ADR-0108](./0108-index-reads-select-by-an-index-query-of-the-surfaces-own.md) rejects the stored
  class as a term, a domain listed after its statistics were built would read as normal.
- **Every group set paged by its statement.** For it, one paging mechanism. Against it, a label group
  comes from expanding each message's labels and a month group from truncating its date, and neither
  is a column a statement's sort can be shown to end with, which
  [ADR-0066](../data/0066-data-access-generated-from-sql.md)'s statement check requires of a paged
  read. Those sets are small, so paging them in the service layer costs nothing.

## Consequences

- A label distribution is `count_messages` grouped by label over any selection, and a sender's is in
  its statistics. The senders with no label are `count_messages` grouped by sender with
  `labels_within` empty, and the senders behind most unfiled volume the same with the labels the
  client does not count as filing.
- The statements the three run are one rows statement, one summary, one statement per grouping
  dimension apart from the class, which runs the summary once per class, the summary once more for
  the no-label group of a grouping by label, the read of the distinct sender addresses the
  classification needs, and one page of the statistics.
- Assumptions about other components. Backfill builds the statistics from the stored messages and
  every workload that adds or changes messages rebuilds the statistics of each domain it touched, so
  the listing follows the index. The index keeps a message's labels as an array and its flags as
  `read` and `starred` booleans. The policy snapshot the call takes is the one the reads of messages
  take ([ADR-0099](../engineering/0099-a-body-request-loads-the-policy-before-it-decides.md) governs
  only the body request).
- The controls this record states are catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md),
  the cursor's binding and the UTC rule as extensions of the rows the other reads already prove.
