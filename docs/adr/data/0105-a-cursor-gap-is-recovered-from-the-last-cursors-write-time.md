# 0105. A cursor gap re-enumerates from an hour before the last cursor was written, with no cap, and an account with no cursor is reconciled over a configured first window

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox), [O2](../../../USE_CASES.md#o2--observable)

## Context

A cursor gap is an expected condition. Delta sync detects it, re-enumerates over a bounded window and
alerts ([ADR-0018](./0018-delta-sync-polls.md)). The Provider Port returns `ErrCursorGap` for a
cursor the provider cannot calculate changes from, and full enumeration is the backfill primitive,
which delta sync does not call ([ADR-0010](../provider/0010-one-provider-port.md)). The port's
thread listing selects by a message's date, and Gmail's date is the instant Gmail received the
message. The account's state row records when delta sync last wrote the cursor, in `sync_cursor_at`
([ADR-0016](./0016-schema.md)).

[G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox) is falsified by an invalidated
cursor leaving messages permanently missing from the index, and by a recovery that succeeds silently.
Backfill's first pass is skipped once it has ended for an account, unless a stored subject was masked
under another scanner ([ADR-0096](../redaction/0096-a-scanner-change-reopens-backfill.md)), so a
backfill run adds no message the index lacks after the first pass has ended.

An account delta sync has never ticked for holds no cursor. Backfill enumerates the mailbox as it
stands when each page is read, so mail that arrives after the enumeration passed its place is not in
it, and delta sync's first cursor covers only what arrives after it is taken.

## Decision

- **A gap starts a recovery, recorded as a run of its own whose pass is `gap_recovery`.** The
  recovery takes the provider's current cursor first. It then lists the threads holding a message
  dated at or after the window's start, and applies every message they hold as a tick applies an
  added message, adding one the index lacks and updating the labels and flags of one it holds. Once
  it has read every page of the listing, it removes from the index each stored message dated at or
  after the window's start that the listing did not return and that the provider no longer returns
  when asked for it by its identifier. A listing that fails on any page fails the recovery, which
  then removes nothing. Last, in one transaction, it stores the cursor it took with its write time
  and records the run as succeeded. Its counters carry `window_start`, `window_end` and
  `reconciled`, the messages it added, removed, or whose labels or flags it changed.
- **The window starts an hour before `sync_cursor_at` and ends when the recovery took its cursor.**
  The hour covers a provider's dates drifting from delta sync's clock. The window has no cap, so it
  is bounded by how long the cursor went unwritten.
- **A recovery stopped before it ends stores nothing of its cursor**, so the next tick meets the
  same gap and recovers over the same window again. Applying a message twice leaves the index as
  applying it once.
- **Every gap counts on `mediated_mailbox_sync_cursor_gaps_total`**, a counter labelled by account,
  and an alerting rule fires for an hour after any increase
  ([ADR-0077](../operability/0077-conditions-raised-as-alerting-rules.md)). A recovery that fails
  leaves the gap in place, so the next tick counts it again and the rule keeps firing.
- **An account with no cursor is reconciled once, inside its tick.** The tick takes the current
  cursor first, applies every message of the threads holding a message dated within the first
  window, a value of delta sync's configuration whose default is seven days, and stores the cursor.
  It removes nothing. The tick's counters carry the same three window fields, `reconciled` counting
  the messages it added or whose labels or flags it changed. It counts no gap and raises no alert,
  since no cursor was lost.

The deciding argument. Starting from the last cursor's own write time covers exactly the time the
change feed was not read, and a cap would leave mail older than the cap missing for good, since
nothing else adds a message to the index once backfill's first pass has ended. The same holds for a
message removed while the feed was not read. The feed never names it again and backfill removes
nothing, so the recovery is the one point at which it can leave the index. A message missing from
the listing is removed only once the provider does not return it by its identifier, because the
listing selects through the provider's search, and whether Gmail's search compares the date the
model maps or the message's own Date header is unverified
([provider/gmail/doc.go](../../../provider/gmail/doc.go)). A listing that leaves out a message the
mailbox holds then removes nothing from the index.

## Alternatives considered

- **A window capped by a configured maximum, with backfill as the repair beyond it.** For it, a
  recovery's cost is bounded by configuration whatever the outage. Against it, a backfill run does
  not add a message the index lacks once the first pass has ended, so mail beyond the cap stays
  missing, which falsifies [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox).
- **A full enumeration on every gap.** For it, the index is reconciled whole. Against it, full
  traversal is the backfill primitive delta sync does not call, at corpus cost per gap
  ([ADR-0018](./0018-delta-sync-polls.md)).
- **An account with no cursor takes the current cursor and nothing else.** For it, the smallest
  first tick. Against it, mail that arrived after backfill's enumeration passed its place and before
  that cursor is never added.
- **An account with no cursor reconciled from the newest stored message's date.** For it, no
  configured window. Against it, an index backfill has not yet written to has no newest message,
  and a message whose date the provider does not set on receipt can sit in the future.
- **A recovery that removes nothing, with the messages removed during a gap stated as a residual.**
  For it, a recovery only adds and updates, and no statement deletes on the strength of a listing.
  Against it, every message removed during a gap stays in the index for good, listed and searchable
  though the mailbox no longer holds it, which falsifies
  [G4](../../../USE_CASES.md#g4--the-index-tracks-the-live-mailbox).
- **Removing every message dated in the window that the listing did not return.** For it, no
  further provider call. Against it, a listing that leaves out a message the mailbox holds, such as
  a search comparing a Date header older than the window, would remove that message from the index
  for good.
- **Recording an account's first reconciliation as a gap recovery.** For it, one kind of run for
  every re-enumeration. Against it, the operator's sync-gap card and the gap alert would report a
  lost cursor that never existed.

## Consequences

- A recovery's cost grows with the outage it follows. After a long outage the recovery is long, and
  the alert fires when the gap is found, before the recovery ends.
- The window catches mail that arrived in it, mail whose labels or flags changed while it is in a
  thread the window lists, and mail dated in it that was removed. A change during the gap to a
  message dated before the window is not seen until a later change names it. A message dated before
  the window and removed during the gap stays in the index for good, since the change feed never
  names it again, no recovery lists it and backfill removes nothing. So does a message whose removal
  a tick applied in the seconds between backfill reading the page that holds it and committing that
  page. Production point 1's reconciliation of counts against the provider is where either shows.
- The stored messages the listing left out are asked for in metadata calls sized as a change set's
  are, so a recovery whose listing leaves out many of them spends that much more of the sync class's
  budget.
- An account whose first tick comes more than the first window after backfill's enumeration began
  can miss the mail between. The default assumes the deployment starts delta sync with backfill.
- Assumptions about other components. The provider's thread listing selects by a date within an
  hour of when the provider received the message, and lists the trash and spam like any other mail.
  The Gmail adapter keeps only messages whose receipt instant the window holds, but whether Gmail's
  search, which it narrows by first, compares that instant or the message's own Date header is
  unverified ([provider/gmail/doc.go](../../../provider/gmail/doc.go)). If it compares the header, a
  message received during the gap whose Date header is older than the window is never listed, so
  the recovery never adds it, and production point 1's reconciliation of counts is where that
  shows. The metadata read returns every message the mailbox holds,
  the trash and spam included, and leaves out one it no longer holds
  ([ADR-0010](../provider/0010-one-provider-port.md)).
- The rules above are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
