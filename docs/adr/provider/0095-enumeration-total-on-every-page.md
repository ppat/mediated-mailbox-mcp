# 0095. Every page of an enumeration carries the total and the page limit together, as one optional field of the port's page

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [P1](../../../USE_CASES.md#p1--one-contract)

## Context

[ADR-0010](./0010-one-provider-port.md) has `enumerate_all`'s page carry an optional total, the
number of messages a full enumeration returns as the provider counts them, and the most items any
page of the listing holds, so backfill's pass 1 can record a page count the backfill card turns
into page of total pages and an estimated time left
([docs/UI.md section 8.1](../../UI.md#81-home)). That leaves open how the two numbers travel on the
port, which pages carry them, how pass 1 derives its page count from them, and how the provider
fake and the contract suite cover a total and its absence. Pass 1 is their first consumer.

## Decision

- **One optional field on the port's one page type.** `Page[T]` carries `Total`, nil when the
  implementation does not count the listing. A total holds two numbers, the items the whole listing
  holds as the provider counts them and the page limit, the most items any page of that listing
  holds. The field means the same on every listing, and an implementation fills it where it counts.
  Gmail's thread listing reports none.
- **The page limit travels only with a total.** A listing that reports no total promises nothing
  about the size of its pages. Pass 1 needs the limit only to turn a total into pages, so the port
  promises a limit only where a total gives it a consumer.
- **Every page of an enumeration carries it,** not only the first. Each page brings a fresh count,
  so a mailbox that grows or shrinks during a pass, and a run that resumes in the middle of one, get
  a current figure with no state kept. For Gmail it is one `users.getProfile` in every
  `EnumerateAll`, so the call's declared cost stays one constant, 67 units. They are a
  `messages.list`, a `messages.get` for each of its three messages, a read of the label table and
  the `getProfile`.
  That is under the 80 units of one second's worth at the hard cap
  ([ADR-0023](../operability/0023-adapter-declares-cost.md),
  [ADR-0024](../operability/0024-conservative-target-aimd.md)).
- **Pass 1 derives its page count, `of`, from the page it made durable, using the total, the page
  limit and the next-page token only.** At the page with no next token, `of` is that page's number,
  since the enumeration has ended. Before it, `of` is the total divided by the page limit, rounded
  up, and never less than one more than the current page, since a next token means at least one
  more page. A page with no total gives no `of`.
- **The checkpoint stores `of` only when it has one.** A checkpoint written without it, by an
  earlier run or for a page with no total, loads and resumes unchanged, and the card shows no
  estimate for it. Starting an enumeration over drops `of` with the token. Whether an enumeration
  has ended still reads only the page and the token.
- **`of` is an estimate.** The page limit is a ceiling, so pages that come up short, as a Gmail page
  does when a message is deleted between its listing and its read, make the enumeration take more
  pages than `of`, and the next page's figure corrects it. Gmail's `messagesTotal` counts the
  mailbox, and the enumeration lists it with spam and trash. Gmail's documentation does not say
  whether the two count drafts and chats alike, so the total can be off in either direction, and
  the floor of one more page keeps `of` ahead of the current page while pages remain.
- **The provider fake reports a total only when configured to.** Its total is the number of
  messages it holds and its page limit is its page size. **The contract suite takes a switch saying
  whether the implementation reports a total.** With the switch set, every page of an enumeration
  carries one, no page holds more items than its page limit, and the total counts at least the
  messages the case added. With it unset, no page carries one. The fake passes the suite both ways,
  and Gmail's run against the test account sets it. A real account holds mail the run did not add,
  so a total is checked only as a bound there, never as an exact count
  ([ADR-0043](../engineering/0043-no-mocking.md)).

## Alternatives considered

- **An operation of its own on the port.** The case for it is one role per call. An enumeration
  page would hold only messages, and a count would come from a call that does nothing else. It
  needs a new operation in every rate profile, the fake, its throttle wrapper, the contract suite's
  per-operation cases and the assertion pinning the port's methods. Its answer could also come from
  a different moment than the page pass 1 makes durable with it, which the field on the page avoids
  by construction.
- **A page type of the enumeration's own.** The case for it is that the field would never sit on a
  listing that leaves it empty. It changes the signature ADR-0010 states, `Page[MessageMetadata]`,
  and every caller's page, while the field means the same thing on any listing.
- **The total on the first page only.** The case for it is cost, since Gmail would call
  `getProfile` once a pass rather than once a page. The declared cost would then depend on the page
  token, a run resuming in the middle of an enumeration would never see a total, and a count taken
  once would not follow a mailbox that changes during a pass.
- **A page limit on every page, with or without a total.** The case for it is that every adapter
  knows the limit it sizes its pages to. Without a total no caller reads it, so it would be a
  promise in the contract with no consumer.
- **A page count derived from the adapter's page size, or from the length of the pages seen so far,
  with no page limit on the port.** The case for it is that the port would need nothing beyond the
  total, since the adapter knows its page size and a caller sees every page's length. The adapter's
  page size is not a promise the port makes, and
  [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts)
  licenses no other knowledge. A guess from page lengths assumes later pages look like earlier ones,
  which the port does not promise either, and a short Gmail page, one whose message was deleted
  between its listing and its read, breaks it.
- **`of` as the total over the limit with no floor.** The case for it is that it is the plain
  quotient. A total lower than what the enumeration returns would put the current page past `of`,
  and the card past its whole bar, while pages remain.

## Consequences

- The card's estimate reads `of` as a number of pages from the run's checkpoint, which is what the
  UI's jobs read already expects of the `checkpoint` column of
  [ADR-0016](../data/0016-schema.md). How the card renders it is the UI's.
- A full Gmail pass spends 67 units per page, one of them for the `getProfile` that brings the
  total.
- An adapter whose provider counts a different set than its enumeration lists still passes the
  contract, which checks only a bound. Its card's estimate is as good as its count.
- Pass 1 knows nothing of how an adapter sizes its pages beyond the page limit the page states, so
  an adapter may change its page size without pass 1 changing.
