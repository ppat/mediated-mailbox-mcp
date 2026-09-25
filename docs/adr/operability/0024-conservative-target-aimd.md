# 0024. Target half the stated ceiling, hard-cap at 80%, and adapt below with AIMD

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [O1](../../../USE_CASES.md#o1--rate-limited-politely), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Documented ceilings are not real ceilings. Gmail's 6,000 units a minute per user per project interacts with
per-project quotas, mailbox size, and account age — accounts hit 429s below the documented number.
JMAP publishes no number at all ([ADR-0023](./0023-adapter-declares-cost.md)). And the worst outcome
is not slowness but a provider-side account restriction from sustained abuse — on the operator's
personal mailbox. Deliberate politeness is the posture; the question is its mechanism.

## Decision

**A conservative target with an adaptive controller free to go below it and structurally unable to
go above the hard cap:**

```
budget_ceiling = profile.budget_per_second()        # 100 for Gmail
target         = budget_ceiling * 0.50              # the polite default
hard_cap       = budget_ceiling * 0.80              # never exceeded, ever
floor          = budget_ceiling * 0.05              # controller's lower bound
current_rate   ∈ [floor, target]
```

Why a range rather than a constant: a fixed 50% still breaks when the *actual* limit differs from
the documented one, and a static setting cannot know that. The controller can. `hard_cap` exists as
a separate constant from `target` so that no code path — a future tuning knob, a config override,
an "urgent backfill" flag — can push past 80% even if someone raises the target. Ceilings that
live only in a default value get raised eventually. The cap is enforced at the point of lease
issuance ([ADR-0025](./0025-priority-classes-and-leases.md)), not only inside the controller, so a
controller bug cannot exceed it.

The target is the one value here an operator may tune. An account's configuration may lower it,
to any value above the floor, for an account whose provider quota other applications share, and
nothing may raise it above half the ceiling. A target that would be stored equal to the floor is
refused, because a rate held at the floor is the collapse the alert pages on. A lowered target
lowers the sustained rate and not the burst, since the bucket still holds one second at the hard
cap, so a minute can spend up to sixty seconds at the target plus one second at the hard cap. The
other fractions and every rule below are the same for every provider and every account. What
differs by provider is the ceiling they are fractions of, which the adapter declares
([ADR-0023](./0023-adapter-declares-cost.md)).

**The controller is AIMD — additive increase, multiplicative decrease** — the same algorithm TCP
uses, for the same reason: it converges on an unknown ceiling without needing to know it, and
degrades politely under contention.

```python
class AdaptiveRateController:
    def on_success(self):
        self.rate = min(self.rate + self.step, self.target)

    def on_throttle(self, signal: ThrottleSignal):
        self.rate = max(self.rate * 0.5, self.floor)
        self.backoff_until = now() + (signal.retry_after or self._jittered_backoff())

    def on_server_error(self):              # 5xx — decrease, less aggressively
        self.rate = max(self.rate * 0.8, self.floor)

    def on_latency_sample(self, p50: float):
        if p50 > self.baseline_p50 * 2:     # degradation precedes throttling
            self.rate = max(self.rate * 0.9, self.floor)
```

`step` is 2% of the target, times the succeeding call's cost, divided by the current rate. While
the rate is fully spent, the costs of a second's successes add up to the rate, so the rate grows by
2% of the target each second whatever the rate, and climbs from the floor to the target in 45
seconds. A fixed step per success would grow it exponentially, because the number of successes a
second grows with the rate. TCP scales its per-acknowledgement increase by the window for the same
reason ([RFC 5681 section 3.1](https://www.rfc-editor.org/rfc/rfc5681.html#section-3.1)).

Tokens are issued from a token bucket that refills at the current rate and holds at most one
second's worth at `hard_cap`, the committed information rate and committed burst size of
[RFC 2697](https://www.rfc-editor.org/rfc/rfc2697.html)'s meter. Beside the bucket, the tokens
issued inside any one-second window sum to at most `hard_cap`, so the hard cap holds as a rate per
second. Over a longer window, except as the clock and the stored record allow below, the bucket
holds issuance to the current rate plus one second's worth at `hard_cap`, which a provider limit
counted per minute absorbs. Tokens build up while the rate is low, so a call costing more than one
second's worth at the current rate is still issued, and the rate can recover from the floor. The
bucket does not fill during a backoff. A request costing more than one second's worth at `hard_cap`
is refused, never left waiting, so an adapter keeps each request it makes within that size. The
window counts every grant of the last second, so the store keeps them all, whole, beside the
bucket's level and the instant it was last filled. A grant lost from that record widens issuance,
and the rules cannot see its absence. Every one of these limits is measured on the database's clock,
which every process shares. A forward step of that clock, such as a virtual machine resuming or a
time server correcting it, reads as idle time, so it can release up to one more bucket inside one
real second. A stale instant in the stored record reads as idle time too, and the one-second window
still holds it to `hard_cap`, while a longer window can see one more bucket for each stale read, and
at worst `hard_cap` sustained. The rules cannot tell either from real idle time, and a provider
limit counted per minute absorbs both.

Two details matter more than the algorithm choice:

- **Honor `Retry-After` when present; use full jitter when absent** — `random(0, base * 2^n)`, never
  fixed backoff. Gmail's [error
  guide](https://developers.google.com/workspace/gmail/api/guides/handle-errors) does not say it
  sends `Retry-After`, so it is read whenever it arrives, and Fastmail's behavior gets discovered
  when its adapter is built. Fixed backoff from a resuming batch job produces synchronized retry
  waves against yourself. A throttle arriving during a backoff never ends it sooner, so a
  `Retry-After` is honored in full. The base is one second, n counts the throttles before this one
  with no success between them, so the first throttle draws with n at zero, and `base * 2^n` stops
  doubling at 32 seconds, within the 32 or 64 seconds Google Cloud's truncated exponential backoff
  uses ([Memorystore](https://docs.cloud.google.com/memorystore/docs/redis/exponential-backoff)).
- **Decrease on latency, not only on errors.** A 429 means the budget was already overshot;
  latency degradation precedes it. Tracking a rolling median against baseline and backing off at
  roughly 2× is the difference between a job that occasionally trips limits and one that
  essentially never does — worth having here specifically because these are long batch jobs where
  no individual request has a waiting user. The median is taken over one-minute windows holding at
  least 20 samples, and the baseline is the lowest of the last ten such medians, so a rise in
  latency becomes the baseline only once it has lasted ten minutes. LEDBAT keeps its base delay
  the same way, as the lowest of ten one-minute minima
  ([RFC 6817](https://www.rfc-editor.org/rfc/rfc6817.html)).

**The failure modes are named and watched.** *Collapse:* repeated throttling pins the rate at the
floor and it never recovers, turning an hours-long backfill into days — alert on rate pinned at
floor beyond a few minutes. *Runaway:* a coordination bug lets workers collectively exceed the
budget — alert on aggregate observed request rate exceeding `hard_cap`, and page rather than
dashboard, because runaway is the failure that risks the account restriction. The rules that
raise both, and exactly what each reads, are
[ADR-0077](./0077-conditions-raised-as-alerting-rules.md)'s.

What the numbers mean for the one big job: at the 50% target, Gmail metadata fetches cost 20 units
each and run at about 2.5 messages/sec, roughly 9,000/hour. For a 100k corpus, backfill pass 1 is
about 11 hours. Pass 2 fetches only the gated-in, non-restricted fraction, roughly 40–60% of the
corpus, which takes another 4.4–6.7 hours. That is **about 15.5–18 hours, once**, roughly double
the full-rate estimate and an easy trade for never antagonizing the provider on a job with no
deadline.

## Alternatives considered

- **A fixed conservative rate, no controller.** Rejected: wrong in both directions — it overruns
  when the real ceiling is lower than documented, and permanently wastes headroom when discovery
  would have found more (which for JMAP, with no published number, is the only way to find any).
- **Target the documented ceiling and back off on 429.** Rejected: routinely brushing the ceiling
  is the impolite posture, and sustained 429s are the overrun pattern that risks a provider-side
  account restriction.
- **Error-only feedback (classic AIMD without the latency signal).** Rejected for these workloads:
  with no user waiting per request, trading a little throughput for staying out of the throttle
  regime entirely is free.

## Consequences

- Rate behavior is observable by design: current rate, throttle events, and backoff state are
  metrics, and the two pathologies have alerts, not just graphs.
- The controller state lives in the shared coordination row
  ([ADR-0025](./0025-priority-classes-and-leases.md)) so all processes converge on one discovered
  ceiling per account.
- The controller is testable against a simulated provider that throttles on schedule —
  convergence and recovery are provable offline, before the first real backfill; those checks are
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
