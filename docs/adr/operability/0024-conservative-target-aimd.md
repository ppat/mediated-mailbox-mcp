# 0024. Target half the stated ceiling, hard-cap at 80%, and adapt below with AIMD

**Status:** Accepted ·
**Pillar:** [An accepted risk that is not measured is an unmeasured risk](../../../DESIGN.md#an-accepted-risk-that-is-not-measured-is-an-unmeasured-risk) ·
**Serves:** [O1](../../../USE_CASES.md#o1--rate-limited-politely), [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

Documented ceilings are not real ceilings. Gmail's 250 units/sec/user interacts with per-project
quotas, mailbox size, and account age — accounts hit 429s below the documented number. JMAP
publishes no number at all ([ADR-0023](./0023-adapter-declares-cost.md)). And the worst outcome is
not slowness but a provider-side account restriction from sustained abuse — on the operator's
personal mailbox. Deliberate politeness is the posture; the question is its mechanism.

## Decision

**A conservative target with an adaptive controller free to go below it and structurally unable to
go above the hard cap:**

```
budget_ceiling = profile.budget_per_second()        # 250 for Gmail
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

Two details matter more than the algorithm choice:

- **Honor `Retry-After` when present; use full jitter when absent** —
  `random(0, base * 2^n)`, never fixed backoff. Gmail sometimes sends `Retry-After`; Fastmail's
  behavior gets discovered when its adapter is built. Fixed backoff from a resuming batch job
  produces synchronized retry waves against yourself.
- **Decrease on latency, not only on errors.** A 429 means the budget was already overshot;
  latency degradation precedes it. Tracking a rolling median against baseline and backing off at
  roughly 2× is the difference between a job that occasionally trips limits and one that
  essentially never does — worth having here specifically because these are long batch jobs where
  no individual request has a waiting user.

**The failure modes are named and watched.** *Collapse:* repeated throttling pins the rate at the
floor and it never recovers, turning an hours-long backfill into days — alert on rate pinned at
floor beyond a few minutes. *Runaway:* a coordination bug lets workers collectively exceed the
budget — alert on aggregate observed request rate exceeding `hard_cap`, and page rather than
dashboard, because runaway is the failure that risks the account restriction.

What the numbers mean for the one big job: at the 50% target, Gmail metadata fetches run at about
25 messages/sec, roughly 90k/hour. For a 100k corpus, backfill pass 1 is about 65–75 minutes; pass
2 fetches only the gated-in, non-restricted fraction (roughly 40–60% of the corpus), another
30–45 minutes — **about 2.5–3 hours, once**, roughly double the full-rate estimate and an easy
trade for never antagonizing the provider on a job with no deadline.

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
