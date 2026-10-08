# 0023. The adapter declares what operations cost; the rate limiter is provider-agnostic

**Status:** Accepted ·
**Pillar:** [Everything above the port speaks canonical](../../../DESIGN.md#everything-above-the-port-speaks-canonical) ·
**Serves:** [O1](../../../USE_CASES.md#o1--rate-limited-politely), [P1](../../../USE_CASES.md#p1--one-contract)

## Context

The bottleneck is provider quota, not classification CPU. A regex pass over a 30 KB body costs
tens of microseconds, while a Gmail call costs 50–200ms and spends quota. So rate limiting is
load-bearing, and it must serve two providers whose limit models are incompatible:

| | Gmail | JMAP (Fastmail) |
| --- | --- | --- |
| Unit of cost | quota units, per-operation weight | requests / concurrent connections |
| Published budget | 6,000 units a minute per user per project, 100 a second on average ([usage limits](https://developers.google.com/workspace/gmail/api/reference/quota)) | not published as a number |
| Signal on breach | HTTP 429, or 403 with a reason such as `userRateLimitExceeded` ([error handling](https://developers.google.com/workspace/gmail/api/guides/handle-errors)) | HTTP 429; limits in the session object |
| Discoverable ceiling | documented constant | `maxConcurrentRequests`, `maxCallsInRequest` |
| Batching semantics | HTTP batch, 100 sub-requests, **cost charged per sub-request** | multi-id `Email/get` is **one** request |

The last row is the load-bearing asymmetry. On Gmail, batching saves round trips but not quota. On
JMAP, a multi-id fetch is one request, so batching saves the resource that is actually limited. An
abstraction that models cost as a single universal number will be wrong for one of them.

## Decision

**The adapter declares cost, and everything above sees only a weight and a budget.**

```python
@dataclass(frozen=True)
class OpCost:
    weight:     float      # provider-native cost units
    ops_count:  int        # logical messages covered (throughput accounting)

@dataclass(frozen=True)
class ThrottleSignal:
    retry_after: timedelta | None
    scope:       ThrottleScope     # PER_USER | PER_PROJECT | UNKNOWN

class RateLimitProfile(Protocol):
    def cost(self, op: ProviderOp) -> OpCost: ...
    def budget_per_second(self) -> float: ...            # provider ceiling
    def parse_throttle(self, err: Exception) -> ThrottleSignal | None: ...
    def refresh_limits(self) -> None: ...                # JMAP: re-read session
```

- **The Gmail profile:** `cost` returns the documented unit weights, `budget_per_second` is 100,
  which is the per-minute limit averaged over its minute, `parse_throttle` distinguishes per-user
  from per-project throttling by the reason Gmail gives, and `refresh_limits` is a no-op. While
  one account uses a Google Cloud project, both kinds get the same response, a throttle on that
  account. How a per-project throttle reaches other accounts in the same project is an
  [open decision](../../../ROADMAP.md#open-decisions).
- **The JMAP profile:** `cost` returns weight 1.0 per JMAP method call regardless of id count,
  `budget_per_second` is derived from `maxConcurrentRequests` in the session object times observed
  throughput, and `refresh_limits` re-reads `.well-known/jmap`.

That is the entire abstraction, one interface each provider can answer in its own terms, rather
than forcing JMAP to pretend it has quota units. One subtlety is worth naming. JMAP's ceiling is
not published, so its budget starts as a conservative guess that the adaptive controller
([ADR-0024](./0024-conservative-target-aimd.md)) discovers, while Gmail's is documented, so the
controller mostly holds station. The machinery is the same and only the amount of discovery
differs, which is what an abstraction serving a documented and an undocumented backend should look
like.

The Gmail unit weights are listed below, because two of their consequences shape other decisions.

| Operation | Units | Note |
| --- | --- | --- |
| `messages.list` | 5 | 500 ids/call — enumeration nearly free |
| `messages.get` (METADATA) | 20 | dominant backfill cost |
| `messages.get` (FULL) | 20 | **same as metadata** — body fetch is quota-free relative |
| `threads.get` | 40 | every message of one thread |
| `batchModify` | 50 | 1000 ids/call — bulk reorg cheap per message |

Row three is why the scan gate is a latency-and-exposure optimization, not a quota one
([ADR-0093](../redaction/0093-composite-scan-gate.md)).

A port call's declared cost is its worst case, known before the call, and no call may cost more
than one second's worth at the hard cap, the largest request
[ADR-0024](./0024-conservative-target-aimd.md) issues. Where a call's size depends on what it
returns, such as a page of threads, the adapter sizes the page to fit. Where the caller sets the
size, as with the identifiers of a metadata fetch or the operations of a mutation, the caller
splits its work into calls that fit, learning what fits by asking the profile what a candidate
call costs. Gmail's HTTP batch endpoint collapses up to 100 sub-requests into one round trip, and
each sub-request is still charged. A batch therefore counts as one call, and the one-second bound
limits what it holds. Its sub-requests, together with every other provider request the call
makes, cost no more than one second's worth at the hard cap. Batching then saves round trips at
unchanged quota, which works with conservative rate targeting rather than against it.

## Alternatives considered

- **A universal cost unit mapped onto both providers.** No case was tabled for it. Rejected by the
  batching asymmetry above, since any single number misprices one provider's batches, and mispricing
  is how limiters overrun.
- **Per-provider rate limiters.** No case was tabled for it. Rejected: it duplicates the controller,
  the priority classes, and the coordination machinery per backend, and pushes provider identity
  above the port.
- **No cost model, only reacting to 429s.** No case was tabled for it. Rejected: reactive-only
  limiting guarantees routinely hitting the ceiling, which is the impolite posture the whole rate
  design exists to avoid.

## Consequences

- Everything above the profile, which is the token bucket, the controller and the pipeline, is
  provider-blind, keeping the port abstraction intact even in the least abstractable corner of
  provider behavior.
- Adding a backend means writing one profile of that backend's real costs, not retuning the
  limiter.
