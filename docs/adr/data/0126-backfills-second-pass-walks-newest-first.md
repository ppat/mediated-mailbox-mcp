# 0126. Backfill's second pass reads the messages waiting for a scan newest first, keyed on the send time and the identifier

**Status:** Accepted ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Backfill's second pass decides and scans the messages its first pass stored pending, and a pending
message's body is denied as pending its content scan
([ADR-0017](./0017-two-pass-backfill.md), [ADR-0093](../redaction/0093-composite-scan-gate.md)).
The pass starts for an account only once the first pass has ended, because the gate reads each
sender's volume and prior hits from the first pass's statistics, and with partial statistics a
sender can show high volume and no hits only because its earlier hits are not stored yet. A pending
body is denied rather than scanned at release, because the full scanner stays out of the serving
path ([ADR-0009](../redaction/0009-scanner-verdicts-carry-no-content.md)) and a body is never
fetched in order to decide ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)). Running the
passes at once would not shorten the total either, since both spend the same quota. So the order in
which the pass walks the waiting messages is what decides which bodies become readable first.

The pass fetches one body per gated-in message in the batch class. At the batch class's 50 units a
second when no other class asks, that is 2.5 bodies a second. For 100,000 messages the first pass
takes about 12.4 hours, and with 60,000 to 90,000 bodies gated in the second pass takes 6.7 to 10
hours. Read in the order of their identifiers, which for Gmail grow with time, the newest mail is
the last to become readable, about 19 to 22 hours after the start, and again after every change of
scanner ([ADR-0120](../redaction/0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)).
The newest mail is what an agent most likely asks for first.

## Decision

- **The second pass reads the messages waiting for a scan newest first**, ordered by `sent_at`
  descending and then by `message_id` descending, so two messages sent at the same instant have one
  order and every waiting message has one place in the walk.
- **A page reads the messages after the position of the last message the durable pages read**, the
  pair of that message's `sent_at` and `message_id`, written in PostgreSQL's row-comparison form
  `(m.sent_at, m.message_id) < (@after_sent_at, @after_message_id)`. In that form the planner uses
  `sent_at` as an index condition on the existing `messages (account_id, sent_at DESC)` index, so a
  late page costs what an early page costs. No index is added.
- **The first page reads from the position `('infinity', '')`**, which every stored message sorts
  before, so one statement serves every page and the comparison stays an index condition in custom
  and generic plans alike.
- **The pass reads through a statement of its own.** Delta sync's scanning of what waits keeps the
  order of the identifiers and its own checkpoint
  ([ADR-0104](../redaction/0104-once-pass-2-has-ended-each-delta-sync-tick-scans-what-waits.md)).
- **The pass's checkpoint holds both halves of the position**, the identifier and the send time in
  milliseconds since the Unix epoch, and a resumed run reads from both. No checkpoint in an earlier
  shape is read.
- **A pass that starts over starts from the newest waiting message**, which is the first position,
  whether the delisting transition marked messages during the run
  ([ADR-0037](../redaction/0037-delisting-transition.md)) or backfill's run-start step marked the
  pass to start over
  ([ADR-0120](../redaction/0120-a-scanner-change-re-masks-stored-subjects-from-the-store.md)).

The deciding argument. With the pass walking newest first, at 2.5 bodies a second the newest 9,000
gated-in bodies, plausibly the last several months, become readable within the first hour of the
second pass, about 13.5 hours after the start instead of 20 hours or more. The same holds after
every change of scanner.

## Alternatives considered

- **The order of the identifiers.** For it, the order the pass already had, sharing one statement
  with delta sync's scanning. Against it, the identifiers grow with time, so the newest mail is
  readable last, nearly a day after the start. Measured on a seed of 100,000 pending messages, a page
  in the middle of the walk took 127 to 149 ms in that order, against 0.13 to 0.15 ms newest first in
  row-comparison form.
- **The keyset written with `OR`**, `m.sent_at < @after_sent_at OR (m.sent_at = @after_sent_at AND
  m.message_id < @after_message_id)`. For it, the same rows, in the form the index reads' search
  keyset uses. Against it, measured on a seed of 100,000 pending messages, the planner applied it
  as a filter while walking the index from the newest row, so a late page read almost the whole
  account and its cost grew with progress.
- **A nullable position, `@after_sent_at IS NULL OR` the comparison**, for the first page. For it,
  the form the index reads' message listing uses, with no sentinel value. Against it, once the
  server switches the prepared statement to a generic plan the `IS NULL` test cannot be folded away,
  so the comparison becomes a filter. Measured on a seed of 99,000 waiting messages at a position
  about four fifths through the walk, the generic plan filtered 91,753 rows and took 12 ms, against
  0.12 ms with the sentinel, whose comparison stayed an index condition.
- **A partial index on the waiting messages.** For it, an index that holds only what the pass
  reads. Against it, measured on the seed of 100,000 pending messages it added nothing over the
  existing index.
- **One statement shared with delta sync, both walking newest first.** For it, one statement for
  the two readers of what waits. Against it, it changes delta sync's order and the checkpoint
  [ADR-0104](../redaction/0104-once-pass-2-has-ended-each-delta-sync-tick-scans-what-waits.md)
  states, a change this decision does not need, and ties together two job kinds' walk orders that
  can change apart.
- **A body request denied as pending brings that message's scan forward.** For it, an agent waiting
  on one old message gets it sooner. Deferred, because it hands a client a lever on the batch
  budget's order, and with the pass walking newest first what is left is old mail asked for during
  the pass, a small share of what an agent asks.

## Consequences

- **Mail that arrives while the pass runs is newer than the walk's start**, so the walk does not
  reach it. Delta sync scans it once the pass has ended, as it scans anything left pending
  ([ADR-0104](../redaction/0104-once-pass-2-has-ended-each-delta-sync-tick-scans-what-waits.md)).
- **The gate's prior hits counted within the read work in either order**
  ([ADR-0094](../redaction/0094-scan-gate-decisions-are-not-memoized.md)). A sender's hit still
  reaches the sender's next message on the same page, which is now an older one.
- **The position assumes every stored `sent_at` is a whole millisecond.** Every writer of
  `messages` that ships, the first pass and delta sync's insert, stores the canonical model's date,
  which is milliseconds since the Unix epoch. The position carries milliseconds, both the one a run
  holds between its pages and the one its checkpoint stores, so it names the stored value exactly. A
  writer storing a finer `sent_at` would make a position name an earlier instant than the message
  it was taken from, and the next page, in the same run or a resumed one, could skip waiting
  messages, which over-redacts as the next consequence states.
- **A wrong keyset skips messages rather than releasing them.** A skipped message stays pending and
  its body stays denied, which over-redacts, and the account's scan backlog series shows it
  ([ADR-0093](../redaction/0093-composite-scan-gate.md)).
