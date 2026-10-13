# 0130. The API server bounds what one request can make it hold

**Status:** Accepted ·
**Serves:** [G1](../../../USE_CASES.md#g1--whole-mailbox-visibility)

## Context

Gmail's quota caps the API server's CPU, network and database load far below any container limit,
so the one resource that needs bounding is peak memory, the size of an item times the items held
at once. A body's serve runs in stages, the fetch and decode, the HTML parse, the conversion to
Markdown, the release step and the framing of the answer, and each stage's working set is garbage
before the next starts. One body's peak is its single largest stage plus about one copy of the
body, the bodies served at once multiply it, and the Go collector's headroom multiplies it again,
mapped memory measured at 2.2 to 3 times live for large bodies.

Measured by running the real body operation through both roots, with the real Gmail adapter
decoding canned responses, the largest stage was the release step's delimiter neutralizer
([ADR-0036](../redaction/0036-released-bodies-are-clean-markdown.md)). It listed the start and end
offset of every letter of the text, 16 bytes a letter, before sliding a window over the list. That
list was about 7.8 times an HTML body and 14 to 22 times a text body, so a 4 MiB text body peaked
at 72 MB live, and the step allocated 68 to 83 times the text.

Some of the terms are a client's to choose.

- **A JSON-RPC batch.** The MCP SDK accepts a batch, an array of calls in one POST, from a client
  on a protocol revision before 2025-06-18, and reads a request naming no revision as 2025-03-26,
  which allowed batches. It runs every call of a batch at once and holds every answer until the
  last completes. Fifty calls in one POST with no revision named got fifty answers. The SDK's body
  limit of 4 MiB holds about 30,000 calls, which would all wait for leases at once, each asking
  the database again every 20 to 500 ms, and whose answers would take the API server past a 128 MiB
  limit within minutes. Interactive leases outrank backfill's and delta sync's
  ([ADR-0025](./0025-priority-classes-and-leases.md)), so both would starve meanwhile. Revision
  2025-06-18 removed batches.
- **An abandoned call.** The SDK ties a tool handler's context to its HTTP request only when the
  server asks it to, and only for a client on revision 2026-07-28 or later. Claude Code
  ([MCP documentation](https://code.claude.com/docs/en/mcp), read 2026-10-12) gives up on a
  request to an HTTP server that has not sent its first response byte within 60 seconds, unless
  the server's tool timeout or `MCP_TIMEOUT` is set longer. It also abandons a tool call that has
  had no answer and no progress for five minutes, and moves a call still running after two minutes
  to the background. The MCP root answers in JSON rather than as a stream, so its first response
  byte is the whole answer. A handler whose client has gone keeps waiting for its lease and then
  spends a body's quota on an answer nobody reads, and a retry adds a second. Claude Code's newer
  client runtime speaks 2026-07-28 to a server that does.
- **The wait for a lease.** The provider timeout bounds each provider call
  ([ADR-0101](./0101-every-failure-names-its-origin-and-a-body-denial-is-a-result.md)), not the
  lease wait before it, so on an earlier revision nothing bounds how long a request waits.
- **A connection.** The client surface's server set only a deadline on reading a request's
  headers, so an idle keep-alive connection was held forever and a client could send a body as
  slowly as it liked.

## Decision

- **The release step's neutralizer keeps a window of the last 16 letters, not a list of every
  letter.** It holds the byte ranges of the last letters since its last match in a fixed ring,
  returns a text with no match as it is, and copies a text with a match once. What it replaces is
  exactly what the full list replaced, which a differential test holds against the full list kept
  in the test. Measured, live memory at the peak falls from 7.8 to 4.6 times an HTML body, and for
  a text body from about 17 times to 2.7 over the API root and about 8 over the MCP root. The 4 MiB
  text body peaks at 11 MB, and the step allocates about 1.05 times the text.
- **A released body's metadata is fetched before its body**, so the body is held only while it is
  converted, decided and served, not across the wait for the second lease. A client sees no
  difference.
- **The MCP root refuses a JSON-RPC batch with 400 before the SDK reads it, at every revision.** A
  POST whose body's first byte after JSON's whitespace, space, tab, line feed and carriage return,
  is an opening bracket is a batch. The check decides on that byte alone, never reads a method,
  which only the SDK parses ([ADR-0086](../engineering/0086-mcp-root-on-the-official-go-sdk.md)),
  and hands the SDK the body from that byte on as it was sent, without the whitespace before it,
  which JSON ignores. The whitespace it reads before the byte is bounded by the SDK's body limit,
  and past it the request is refused as too large.
- **The MCP root asks the SDK to follow a request's cancellation**, so a call its client abandons
  at revision 2026-07-28 or later has its operation's context cancelled, and stops waiting for its
  lease.
- **A body request waits for its leases until one deadline, 45 seconds after the request
  arrives**, whatever revision its client speaks. The request arrives when the service layer starts
  the operation, once the root has read it, so the policy load and the gate's read before the
  provider session opens count against the deadline, as they count against the client's own
  timer. A wait cut off by it fails the request as the API server's, since the rate limiter is
  inside the system
  ([ADR-0101](./0101-every-failure-names-its-origin-and-a-body-denial-is-a-result.md)), and the
  failure points the client at the system status. A wait that starts once the deadline has passed
  is cut off at once. 45 seconds is far beyond an ordinary wait, which
  [ADR-0025](./0025-priority-classes-and-leases.md) bounds at about a second for the interactive
  class, so only a controller held back, such as by a throttle's backoff, or a policy load or gate
  read held up reaches it. It ends before Claude Code's 60-second timer on a request's first
  response byte, so a request no lease comes for is answered with the API server's failure while
  the client is still waiting, rather than the client timing out with nothing.
- **The deadline bounds the waits and not the provider calls after them.** A call granted its lease
  before the deadline runs for up to the provider timeout
  ([ADR-0101](./0101-every-failure-names-its-origin-and-a-body-denial-is-a-result.md)), 30 seconds
  by default and at most 5 minutes as configured. The account session calls the provider once more
  after re-reading a credential the provider refused, and that call waits for its own lease under
  the same deadline and runs with a provider timeout of its own. No lease is granted after the
  deadline, so a request is answered within about 45 seconds and one provider timeout of its
  arrival, which at the default is 75 seconds. A request whose last lease comes just before the
  deadline can therefore be answered after the client's timer has fired. A call on an earlier
  revision that nobody reads stops waiting 45 seconds after it arrived.
- **The API server's HTTP servers bound a connection and not an answer.** A request's headers have
  10 seconds to arrive and the whole request 30, and a keep-alive connection idle for two minutes is
  closed. 30 seconds carries the 4 MiB body limit at about 140 KB a second. Two minutes keeps an
  agent's connection open between the calls of one task and releases a connection its client
  abandoned. Nothing bounds writing the answer, since a request legitimately waits for its leases
  and its provider calls. The limits apply to the client surface's server and the probes' server
  alike.
- **The lease-wait deadline and the connection limits are constants in code, not settings.** Each
  bounds what a client can make the API server hold, so no deployment has a reason of its own to
  tune it, and a setting would need a range a start refuses outside of. A change to one is a change
  in review.

## Alternatives considered

- **A neutralizer whose work is deferred until the answer is written.** The case for it is that
  the neutralizer is hardening, not part of the gate, so it could run lazily as the answer goes
  out. It loses because both roots hold the whole result at once, the API root marshalling it and
  the MCP root handing the SDK a complete value, so deferring saves at most one copy on the API
  root, while the window already allocates nothing when nothing matches.
- **Batches left to the SDK, which refuses them from revision 2025-06-18 on.** The case for it is
  that clients on the older revisions keep a feature the protocol once had. It loses on the
  operator's ruling, "if the latest MCP spec doesn't allow batches all together, then why should
  we? isn't that simpler?". No client the API server serves needs them.
- **A batch refused after the SDK parses it, in its receiving middleware.** The case for it is that
  the refusal would sit where the root's other refusal does. It loses because the middleware sees
  each call of a batch alone, after the SDK has read and started them all.
- **The lease wait bounded by the provider timeout.** The case for it is one value fewer. It loses
  because the two bound different things, the provider's answer and the rate controller's hold,
  and a failure of each has a different origin.
- **A deadline on each lease wait rather than one for the request.** The case for it is that each
  call gets the same wait. It loses because a request makes two calls, so its whole wait would be
  twice the deadline.
- **The deadline counted from the provider session's opening.** The case for it is that it counts
  only the time spent waiting for leases. It loses because the client's timer runs from when it
  sent the request, so the policy load and the gate's read before the session opens are time the
  client has already waited, and a deadline that leaves them out fires after the client has gone.
- **A deadline of a minute.** The case for it is more room for a throttle's backoff to clear. It
  loses because Claude Code has given up on the request by then, so the API server's failure
  reaches nobody and only the API server's own work is bounded.
- **The deadline and the limits as configuration values with defaults.** The case for it is that a
  deployment could tune them without a build. It loses for the reasons the last bullet of the
  Decision gives, and no deployment has asked to.
- **A write timeout on the servers.** The case for it is that it bounds a client that reads its
  answer slowly. It loses because it also cuts off a request waiting for its leases, which the
  lease-wait deadline already bounds.

## Consequences

- A client that sends a batch at any revision gets 400 with a sentence saying batches are not
  accepted, and no call of the batch runs. A client on revision 2025-06-18 or later was already
  refused one by the SDK.
- A request whose leases do not come within 45 seconds of its arrival is answered as the
  API server's failure, and logged at error. A policy load or a gate read that takes long leaves
  the waits less of the deadline, and a lease asked for after a slow provider call can find it
  passed, which fails the request as the API server's even when a lease is free. The lease wait
  series of
  [ADR-0125](../engineering/0125-performance-is-measured-once-at-each-place-time-goes.md) times
  only the waits that end in a grant, so a cut-off wait shows in the operation's duration and in
  the log, not in that series.
- Assumptions about other components. The SDK reads a body whose first byte after JSON's
  whitespace is not an opening bracket as one message, never as a batch, and follows a request's
  cancellation only from revision 2026-07-28 on. The rate limiter's `Acquire` returns once its
  context ends. Go's HTTP server falls back to the read limit for an idle connection when no idle
  limit is set, and applies the read limit only to reading the request, not to the handler's run.
