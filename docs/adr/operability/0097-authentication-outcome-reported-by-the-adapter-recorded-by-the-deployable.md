# 0097. The adapter reports its latest authentication attempt, and the deployable records it at the end of each unit of work, the latest attempt winning

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O5](../../../USE_CASES.md#o5--clients-can-tell-failures-apart)

## Context

The system status operation returns each account's last provider authentication outcome, read from
`last_auth_at` and `last_auth_outcome` on the account's state row
([ADR-0034](./0034-system-status-operation.md), [ADR-0016](../data/0016-schema.md)). The provider
adapter is the code that sees each authentication attempt. It has no database access and is the
only code that knows a provider's concepts ([ADR-0010](../provider/0010-one-provider-port.md)). The
deployables that call a provider hold a database role and already read what the adapter holds at
the end of each unit of work, to write back a rotated credential
([ADR-0082](./0082-rotation-writeback-to-the-database.md)). Several deployables, and several job
kinds inside the worker that runs every background job
([ADR-0117](./0117-one-background-worker-runs-every-job-kind.md)), call a provider for the same
account, so they all record the same two columns. The UI is a deployable too, and
calls the provider's token endpoint while it completes a consent, exchanging the consent's code for
the credential it stores
([ADR-0107](../provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md),
[ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)).

## Decision

- **The adapter reports, and writes nothing.** It holds its latest authentication attempt, the time
  it started and its outcome, in a provider-neutral shape the canonical model defines, so every
  adapter reports the same shape.
- **The outcome is one of a closed set of three, stored as these spellings.**
  - `succeeded` is a credential the provider accepted.
  - `refused` is an answer from the provider refusing the request, which for an OAuth token
    endpoint is the 400 or 401 that
    [RFC 6749 section 5.2](https://datatracker.ietf.org/doc/html/rfc6749#section-5.2) gives an
    error response. It needs the operator to act, re-authorizing the account or, for a refused
    client, replacing the OAuth client the account connects through
    ([ADR-0106](../provider/0106-accounts-of-a-provider-connect-through-any-of-its-oauth-clients.md)).
  - `failed` is an attempt that got no answer the adapter could read as either, such as a
    transport error, a server error, a body it could not parse, or no answer before the caller's
    deadline passed. It is transient and needs no operator action.
- **Only a request to the provider's authentication endpoint is an attempt.** For Gmail that is a
  request to the token endpoint. Using an access token the adapter already holds is not an attempt,
  and neither is a mailbox request the provider refuses while that access token is still valid. A
  request its caller cancelled before an answer arrived has no outcome and is not reported.
- **The deployable records what the adapter holds at the end of each unit of work**, beside the
  rotation hand-over of [ADR-0082](./0082-rotation-writeback-to-the-database.md), a unit that failed
  included. In the worker, the job kind whose unit of work it was records it, under that job kind's
  own role. An adapter that has made no attempt records nothing.
- **The UI records the attempt its own code exchange made** when it completes a consent, connecting
  or re-authorizing an account, with the same statement, in the transaction that stores the
  credential. A completed consent's exchange is an attempt the provider answered with a credential,
  so it records `succeeded`, and a re-authorization's success replaces a `refused` outcome a
  workload recorded earlier. An exchange that fails stores nothing, as every refused consent does,
  so the UI records no other outcome.
- **The latest attempt wins.** The statement writes the row only when it holds no attempt or an
  older one, so a deployable recording an attempt older than one another deployable recorded
  changes nothing.

## Alternatives considered

- **The adapter writes the row itself.** For it, every attempt recorded the moment it ends. Against
  it, the adapter would need a database connection and the account's write grant, braiding storage
  into the one component that should know only its provider.
- **The deployable infers the outcome from the errors the port returns.** For it, no new surface on
  the adapter. Against it, the port's errors say whether a credential was refused but not whether
  the token endpoint refused it, since a mailbox request the provider refuses is the same error,
  and a cached access token means no attempt was made, so success cannot be told from silence.
- **A callback the adapter calls on each attempt, which writes the row.** For it, no delay. Against
  it, database input and output inside the adapter's token path and under its lock, so a slow
  database slows every provider call.
- **The provider's own error text as the outcome.** For it, more detail for the operator. Against
  it, unbounded provider text in a field every client reads, and nothing a client can branch on.
- **The last writer wins.** For it, no predicate. Against it, a deployable whose unit of work ended
  late would put back an older outcome over a newer one another deployable recorded.

## Consequences

- The recorded outcome can be up to one unit of work late, and a process killed during a unit loses
  that unit's attempt. The next unit's attempt replaces it.
- A grant revoked while an access token is still valid reads `succeeded` until the next refresh,
  which for Gmail is within the hour, since a refused API request is not an attempt.
- "Latest" is by the recording processes' clocks, so it assumes their clocks agree to within the
  time between two attempts.
- A refused credential's outcome clears as soon as the operator re-authorizes the account, rather
  than when a workload next makes an attempt with the new credential.
- Assumptions about other components. Each deployable that calls a provider reads its adapter's
  latest attempt at the end of each unit of work. The UI's role holds the write grant on the two
  columns ([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)). The database,
  not the deployable, decides which attempt is the latest, through the statement's predicate.
  Row-level security scopes the write to the account the transaction set.
