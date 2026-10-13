# 0131. The body the port returns is one rendered part, bounded by one size constant, and a body above it, or one the adapter cannot decode, is refused rather than failed

**Status:** Accepted (supersedes [ADR-0017](../data/0017-two-pass-backfill.md), with [ADR-0133](../data/0133-backfill-is-full-history-and-two-pass.md) and [ADR-0132](../redaction/0132-a-body-refused-for-good-is-recorded-as-refused-not-left-pending.md)) ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught), [O1](../../../USE_CASES.md#o1--rate-limited-politely), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [O5](../../../USE_CASES.md#o5--clients-can-tell-failures-apart)

## Context

The body fetch is the port's one body-returning operation
([ADR-0010](./0010-one-provider-port.md)), and the gate guards every call to it
([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)). Peak memory per body is the one
resource term of the API server and the worker that needs bounding, and the body fetch had no
bound of its own. The conversion refused HTML above 512 KiB
([ADR-0074](../redaction/0074-html-to-markdown-v2-converts-bodies.md)), but only after the fetch had
read and decoded it, under a response cap of 64 MiB, and a text-only body was released and scanned
at any size.

What Gmail does, read from Google's reference pages on 2026-10-12, sets what an adapter can know
before it downloads a part.

- In `format=full`, a part with a file name, which is every real attachment, comes as a reference,
  an `attachmentId`, with no data
  ([`MessagePartBody`](https://developers.google.com/gmail/api/reference/rest/v1/users.messages.attachments#MessagePartBody)).
  A large text or HTML part comes by reference too. Google documents no threshold. One observer saw
  405 KB come inline and about 3 MB by reference
  ([khuisman/mcp-gee-sweet issue #825](https://github.com/khuisman/mcp-gee-sweet/issues/825)).
- Every part reports `body.size`, "Number of bytes for the message part data (encoding
  notwithstanding)", before its data is fetched.
- The `fields` parameter selects fields, not array elements by value, so it cannot select only the
  HTML part ([partial responses](https://developers.google.com/gmail/api/guides/performance#partial)).
- [`messages.attachments.get`](https://developers.google.com/gmail/api/reference/rest/v1/users.messages.attachments/get)
  fetches one referenced part, at 20 quota units, the same as `messages.get`
  ([usage limits](https://developers.google.com/gmail/api/reference/quota)). Its data comes in the
  part's declared charset, while inline data comes transcoded to UTF-8, by the same observer, and
  decoding it is [ADR-0134](./0134-a-held-apart-part-is-decoded-by-the-whatwg-label-table.md)'s.

The adapter decoded the first text part and the first HTML part without a file name, and a part
held by reference made the whole body fail as the provider's failure, even with the other part
inline. That failure reached the rate controller as a server error in the API server and in the
worker, which multiplies the account's shared rate by 0.8 per attempt
([ADR-0024](../operability/0024-conservative-target-aimd.md)), and it repeated on every attempt,
since the message never changes.

Only the part a client receives can leak. A body with an HTML part is released as that part's
conversion and its text part is not released
([ADR-0100](../redaction/0100-message-text-without-html-is-released-as-a-literal-code-block.md)).
No other path releases the text part of such a body. The snippet is the provider's own preview,
carried with the metadata ([ADR-0001](../redaction/0001-redaction-matrix.md)), and a body the
conversion refuses is denied, never served as its text part instead.

## Decision

- **The body the port returns is the message's rendered part.** That is its first HTML part without
  a file name that holds content, data or a reference to data, depth first, when the message has
  one, and otherwise its first text part without a file name that holds content. An empty part is
  passed over, so an empty HTML part beside a text part leaves the text part as the body. The other
  field of the body is empty. Scanning and releasing read this one part, in the
  form release serves it, the HTML part's conversion or the text part's literal form, so the scan
  reads exactly what a client would receive. The text part of a body with an HTML part is neither
  fetched nor scanned, because it is never released, and scanning it protects nothing a client can
  receive. What [C3](../../../USE_CASES.md#c3--content-based-secrets-caught) falsifies still holds:
  a code or a login link in the released part denies the body, since the released part is the part
  scanned.
- **One size constant bounds the rendered part, 512 KiB, `MaxBodyBytes` in the port's own package.**
  The conversion's input limit and the literal form's limit are that constant, so the three cannot
  drift. A text part above it is refused as an HTML part above it is, in the API server's release
  and in the worker's scan. It bounds the rendered part alone. The snippet and attachment filenames
  are metadata fields the literal form renders unbounded, and no attachment ever takes this bound.
  The value stays 512 KiB until [production point 1](../../../ROADMAP.md#production-point-1--the-read-path)'s
  size data, since raising it later is a constant change that fails safe.
- **A body above the bound, or one in a form the adapter cannot decode, is refused, not failed.**
  The port returns its own error for it, `ErrRefused`, which wraps no other port error. The rate
  controller counts only a provider failure as a server error, so a refusal never moves the rate.
  The API server answers a refusal at serve time as a denial with the reason `body_refused`, audited
  and counted like every denial
  ([ADR-0101](../operability/0101-every-failure-names-its-origin-and-a-body-denial-is-a-result.md)),
  not as a failure of any origin, since nothing failed. A worker records the message as refused
  ([ADR-0132](../redaction/0132-a-body-refused-for-good-is-recorded-as-refused-not-left-pending.md)).
- **A part held apart from the message takes a second call of the same operation.**
  `GetMessageBody` takes the part to fetch. Called with the empty part it chooses the rendered
  part. A part that came inline is in hand and returned. A part held by reference whose declared
  size is above the bound is refused with no download. One within the bound is answered with a
  typed error, `HeldApartError`, naming the part, and no body, so no field of the body ever carries
  a reference. A pure decision in the port's package, `NextBodyPart`, says from the first call's
  answer whether a second call is due and which part it names, and nothing else is followed, so a
  body takes at most two calls. The shells that lease make the calls and drive that one decision,
  `ratelimit/lease` beside `Call` for the worker and the API server's own leased path.
- **Each body call declares 20 units, with or without a part.** Gmail charges `messages.get` and
  `messages.attachments.get` 20 units each, and each call sends one of them, so each call's declared
  cost is its worst case ([ADR-0023](../operability/0023-adapter-declares-cost.md)). An inline body
  is one call, a body refused by its declared size is one call with no download of the part, and
  only a part held by reference within the bound takes a second call.
- **Gmail compiles it this way.** The first call is `messages.get` with `format=full` and the field
  mask `id,threadId,payload`, which drops the labels, the snippet, the history identifier, the date
  and the size estimate, and keeps the headers, because decoding a referenced part in its declared
  charset needs the part's `Content-Type`, which for a single-part message is among the message's
  own headers. A part that came inline is decoded and its length checked against the bound. The second
  call is `messages.attachments.get`, its data decoded in the charset the part's `Content-Type`
  declares, and its decoded length checked against the bound too, so a declared size that turns out
  smaller than the data refuses rather than releases. A charset the adapter cannot decode is
  refused. The part names Gmail's reference with the message and the charset, and Gmail hands out a
  new reference on every read of the message, each of which keeps working for a while, which is
  harmless, since the second call follows the first. Which part is rendered, whether it is
  returned, fetched or refused, the decoding of a part's data, each read's cap and the refusal of an
  answer past it are the Gmail adapter's pure core, `provider/gmail/core`, which returns each
  decision as a value. The adapter enacts it with its requests, its reads up to the cap the core
  gives, the part it names and the port's errors
  ([ADR-0040](../engineering/0040-pure-core-decisions-as-values.md)).
- **Each body response has a read cap of its own.** The first response carries every part Gmail
  sends inline, the rendered part and its alternative among them, so it is capped at four thirds of
  two parts at the bound, for base64, plus 256 KiB for the headers and the part tree, about 1.6 MiB,
  and the second at four thirds of the bound plus 64 KiB. A response that would pass its cap is
  refused, never truncated and misread as malformed. Every request the adapter builds names its
  own cap, which travels on the request to the read, and a request with none is refused before it
  is sent, so no answer is read to a default. The cap rides that one request's context, passed with
  the request, not an ambient value, as
  [ADR-0040](../engineering/0040-pure-core-decisions-as-values.md)'s "nothing is ambient" holds, and
  that carrier is what makes the cap a sent request carries observable to a test.

## Alternatives considered

- **Both parts of a body are scanned**, which [ADR-0017](../data/0017-two-pass-backfill.md) decided
  before this record took the rule from it. The case made for it was that the verdict clears the
  body for release, so it covers whichever part release serves, and that a body whose text part
  differs from its HTML part would otherwise be cleared without its released part scanned. Not
  chosen, because release serves exactly one part, the HTML part's conversion when there is one, so
  the part scanned is the part served, and the other part costs a fetch, memory and scanning for
  nothing a client can receive.
- **Scanning only the HTML part's Markdown**, which ADR-0017 weighed and rejected. Its case is that
  the scanner reads Markdown, which is what the converter produces. ADR-0017 rejected it because a
  body with no HTML part would be cleared for release without its released part ever scanned. That
  case does not arise here, since the part scanned is the rendered part in the form it is released
  in, a text-only body's literal form included.
- **Doubling the body call's declared cost to 40 units**, a `messages.get` and an
  `messages.attachments.get` in one call. The case for it is one call per body and no second call to
  sequence. Not chosen, because every body would declare twice its cost to pay for the rare one held
  apart, which would halve the body throughput of backfill's second pass, delta sync's scanning and
  the API server.
- **Refusing every part held by reference.** The case for it is one request per body and no second
  call. Not chosen, because Gmail's threshold for holding a text or HTML part apart is undocumented
  and could sit below the bound, which would refuse bodies the system can release.
- **A second operation on the port that fetches one part.** The case for it is a signature per
  purpose. Not chosen, because the body would then arrive through two operations, and the port's
  structure, checked in its tests, is that one operation alone returns a body
  ([ADR-0010](./0010-one-provider-port.md)).
- **A field on the body carrying the reference.** The case for it is that the first call's answer
  stays a body. Not chosen, because a body would then carry something that is not content, which
  every caller would have to know to look at.
- **A field mask naming no headers.** The case for it is a smaller response. Not chosen, because a
  referenced part's charset is in its headers.
- **A provider error for a body above the bound**, as before. The case for it is no new port error.
  Not chosen, because it cuts the rate on every attempt and names the provider as the origin of
  something the provider did right.
- **A release rule in the adapter, its size and which part is released.** The case for it is one
  place deciding the whole body. Not chosen, because redaction belongs above the port
  ([DESIGN.md](../../../DESIGN.md#one-gate-n-dumb-adapters)). The adapter learns only the port's
  definition of the rendered part and its size constant, and conversion, scanning and release stay
  above it.

## Consequences

- The port's contract changes in a reviewed change of its pinned signatures. `GetMessageBody` takes
  the part to fetch, every implementation and every caller follows, and the test that pins the
  port's methods pins the new signature. It remains the one operation returning a body.
- Throughput of the body fetch is about 1/(1 + f) of a one-call fetch, where f is the share of
  bodies held by reference within the bound. The contract run against real Gmail measures where
  Gmail holds a part apart, sending marked messages with parts of graded sizes, about 400 KB,
  512 KB, 768 KB, 1 MB, 2 MB and 3 MB, and reporting which come inline, and a probe of messages of
  several parts, about 1.0, 1.2 and 1.8 MB in all, and of a text part alone.
  [Its run of 2026-10-13](https://github.com/ppat/mediated-mailbox-mcp/actions/runs/38367242475/attempts/3) fits one threshold on a message's parts in all.
  - A message of 1,000,000 bytes, a 400,000-byte HTML part beside a 600,000-byte text part, came
    inline whole, as did every message of 786,474 bytes or less.
  - A message of 1,048,619 bytes, a 1 MiB HTML part beside a 43-byte text part, had its HTML part
    held apart, as had every larger graded message.
  - Past the threshold Gmail held apart the large parts and kept the smaller one inline. Beside a
    400,000-byte HTML part it held apart an 800,000-byte text part, and both of two 700,000-byte
    text parts, though a 700,000-byte text part alone comes inline.

  So Gmail holds parts apart once a message's parts pass a threshold between 1,000,000 and
  1,048,619 bytes. A threshold per part alone is refuted, since a 700,000-byte text part was held
  apart while a 786,432-byte HTML part came inline beside a short one. A threshold per part as
  well, above 786,432 bytes, is not excluded. The rendered part stayed inline in every message where another part was larger, so for
  the shapes measured f is 0 at 512 KiB. It is not 0 in general. A message past the threshold whose
  largest part is a rendered part within the bound, an HTML part of 500 KB beside several smaller
  parts for example, can have that part held apart, which is not measured. Whatever f is, a body
  takes at most two calls, so the throughput is never below half of a one-call fetch.
- A response cap set from the bound refuses the rare body whose inline parts and headers together
  pass it, even with its rendered part within the bound. That is over-redaction, the direction the
  design accepts, and it shows as `body_refused`. Under the threshold the run measured, what
  comes inline stays near 1 MB of parts, about 1.4 MB of base64, below the cap of about 1.6 MiB,
  so no answer the run saw reaches the cap. The cap holds the read to the bound if Gmail sends more
  inline than it did.
- The worker's scan reads the text part in its literal form rather than as it arrived, which is
  what a client receives.
- Assumptions about other components. Gmail reports a part's size before its data, gives every
  attachment a file name, and charges both methods 20 units. A rate controller counts a server error
  only for the port's provider failure. The conversion refuses above the constant by an error the
  caller can recognize.
- The rules above are controls. Their injections are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
