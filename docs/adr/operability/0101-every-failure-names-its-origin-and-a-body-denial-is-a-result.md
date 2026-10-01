# 0101. Every failure on the client surface names its origin, and a gated body denial is a result, not a failure

**Status:** Accepted ·
**Pillar:** [One gate, N dumb adapters](../../../DESIGN.md#one-gate-n-dumb-adapters) ·
**Serves:** [O5](../../../USE_CASES.md#o5--clients-can-tell-failures-apart), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

[O5](../../../USE_CASES.md#o5--clients-can-tell-failures-apart) is falsified by a client's own
failure that the response does not tell apart from the system's, and by a provider's failure that
surfaces as the client's or the system's. Until body release, the client surface failed one of two
ways. A refused argument answered with its message, and everything else answered "the operation
failed", a refusal of an unknown account included. The body operation is the first to call a
provider, so it brings the third origin.

[ADR-0087](./0087-client-surface-derives-method-and-hints-from-each-operations-effect.md) left
open whether the denial of a gated body read is a successful result carrying the denial or a
failure, for this contract to decide, with either shape derived the same way on both roots.
[ADR-0002](../redaction/0002-fetch-time-re-evaluation.md) requires the denial to carry a reason
distinct enough that a message pending its content scan reads as backlog, not as a permissions bug.
The UI's own read API already tells origins apart in one error shape
([docs/UI.md section 17.3](../../UI.md#173-the-error-contract)).

## Decision

- **Every failure names one of three origins.**

  | Origin | What failed | Message | API status |
  | --- | --- | --- | --- |
  | `client` | The request. Arguments that are not one object, repeat a key, name an argument the operation does not take or break its schema, an account the mediator does not serve, an identifier the account does not hold | Names what the request got wrong, never stored content | 400, or 405 for `HEAD` on a route |
  | `mediator` | Anything inside the system. The database, the rate limiter, the audit write, an account with no connected credential | Says only that the operation failed inside the mediator, and that the system status operation reports the account's operational state ([ADR-0034](./0034-system-status-operation.md)) | 500 |
  | `provider` | The provider's answer or its absence. A throttle, a failure on its side, a refused credential, a message the index holds and the provider no longer does, no answer within the provider timeout | A fixed sentence per kind, never the provider's own text | 502 |

- **One shape on both roots, for every failure of an operation call.** A failure's content is
  `{"error": {"origin": "<origin>", "message": "<text>"}}`, the shape of the UI's error contract
  without its code and request identifier. The service layer derives it once. The API root writes
  it with the status its origin gives, and the MCP root returns it as a tool result marked as an
  error, as structured content and as text. A request that never becomes an operation call is
  outside the shape, and each such answer is the client's by its own status or code. That is a
  path or a method the API root does not route, which its router answers 404 or 405, a protocol
  error on the MCP root, such as a tool the registry does not hold or a request that is not valid
  JSON-RPC, which the protocol's own error answers, and a request the bearer check refuses with 401.
- **Each provider call has a deadline.** A body request's provider call that has not answered
  within the provider timeout is the provider's failure. The timeout is the mediator's
  `provider_timeout` configuration value, 30 seconds by default, refused unless it is positive and
  at most 5 minutes. It is not a safety setting
  ([ADR-0051](../engineering/0051-environment-contract.md)), since it can only fail a release. A
  request its client abandoned is not the provider's failure, and nobody receives its answer.
- **The origin is decided where the failure is known.** The registry's refusals and an operation's
  refusal of an argument are the client's. An error from a provider call is wrapped as the
  provider's at the call. Everything else is the mediator's, so a failure nobody classified never
  reads as the client's or the provider's.
- **A gated body denial is a successful result.** It carries `released: false`, the reason, and a
  fixed note saying the denial is policy and that no argument or setting releases the body. A
  denial is the gate working, and none of the three origins failed. The reasons are the gate's,
  pending content scan, skipped as restricted, restricted sender, content flagged and invalid stored
  state, and the release step's, the serve-time pattern check matched, no scanner for the check, a
  scan state the release step never releases, and the conversion refused the body.

## Alternatives considered

- **One generic failure for everything.** For it, nothing leaks about the system. Against it, O5
  names it as falsifying.
- **A denial as a failure**, a 403 on the API root and a tool error on the MCP root. For it, a
  client that treats any failure as "no body" needs no other branch. Against it, no origin fits.
  The client asked for something it may ask for, the system and the provider both worked, and a
  tool error reads to an agent as a fault to retry or report, which is the misreading ADR-0002's
  reason exists to prevent.
- **A status code per failure kind on the API root**, 404 for an unknown identifier, 429 for a
  throttle. For it, ordinary HTTP clients branch on the status. Against it, the MCP root has no
  status, so the kind a client can branch on has to travel in the body anyway, and the status is
  then a second place for one fact.
- **The provider's own error text in the message.** For it, more detail. Against it, it is
  unbounded text from outside the system in a field every client reads, the reason
  [ADR-0097](./0097-authentication-outcome-reported-by-the-adapter-recorded-by-the-deployable.md)
  gives for keeping it out of the recorded authentication outcome.

## Consequences

- Changing the shape of a failure is a breaking change of the client surface, as every change to
  an operation's output is.
- The transparency half of O5 rests on the system status operation. A provider or mediator failure
  points the client at it.
- Assumptions about other components. The Provider Port wraps every error in one of its own errors
  or the caller's cancellation ([ADR-0010](../provider/0010-one-provider-port.md)), so the service
  layer classifies a provider failure without reading the provider's text. An adapter returns the
  caller's context error once a call's deadline passes, so the mediator, which set the deadline,
  turns a passed deadline of its own into the provider's failure. The router of the API root, the
  MCP SDK and the bearer check each answer the requests outside the shape with their own status or
  code, before any operation runs.
