# 0002. Body release re-evaluates at fetch time, and a gate denial never contacts the provider

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

Sensitivity is computed during backfill and sync and stored in the index. That creates a choice
about what a body-fetch request trusts, whether the stored classification as of enumeration time,
context supplied by the agent, or a fresh evaluation. Meanwhile policy is live, and the operator who
adds a domain to the deny list expects it to bind immediately.

## Decision

`get_message_body` re-classifies from the index at fetch time, against current policy, trusting
nothing a client supplies.

```
Gate re-classifies from the index at fetch time
  │  (never trusts caller-supplied sensitivity context)
  ├─ content_flags non-empty,
  │  and scan_state ≠ SCANNED            → DENY ("invalid stored state")
  ├─ scan_state = PENDING                → DENY ("pending content scan")
  ├─ scan_state = SKIPPED_RESTRICTED     → DENY (policy)
  ├─ re-derived class = restricted       → DENY (policy)
  ├─ content_flags non-empty             → DENY (policy)
  │     all gate denials: provider never contacted, audit row written
  └─ otherwise (SCANNED clean, or SKIPPED_GATE)
        → adapter.get_message_body() → sanitize
          (SKIPPED_GATE additionally: serve-time pattern check —
           a hit denies; the fetched body is discarded, audit row written;
           every release: the check also reads the attachment filenames)
        → audit ALLOW → return
```

Two properties are the point.

- **On a gate deny, the provider is never contacted**, so no gate-denied body ever enters
  mediator memory. The denial is decided entirely from the index.
- **Policy changes bind on the next call, not the next sync.** A newly deny-listed domain denies
  immediately, because classification is re-derived against current policy at fetch time. How the
  mediator holds current policy at each call is
  [ADR-0099](../engineering/0099-a-body-request-loads-the-policy-before-it-decides.md)'s.

`SKIPPED_GATE` allowing is deliberate and visible in the flow rather than hidden. It is the
accepted residual of [ADR-0093](./0093-composite-scan-gate.md).

The gate reads the content flags and the scan state the index stores, and never its stored sender
class. The first branch catches a stored state no message can be in, content flags on a message the
scanner never read, and names it rather than reporting it as a pending scan.

**The serve-time pattern check.** This record governs whether a body is released. When it says
release and the message is `SKIPPED_GATE` (never scanned, by the gate's accepted skip), the body
additionally passes the cheap deterministic pattern tier (login-link shapes, one-time codes), run
on the already-fetched body during sanitization
([ADR-0036](./0036-released-bodies-are-clean-markdown.md)'s step). A hit denies, so the fetched
body is discarded and reaches no client, with the same audit row every denial writes. This is not
the scanner, and it does not sit under the scanner's out-of-band placement rule
([ADR-0009](./0009-scanner-verdicts-carry-no-content.md)). Those reasons bind the full scanner
(timeouts, latency, fail-open pressure), while a pure pattern check on a body already in memory has
no external calls, no backlog, and bounded microsecond cost. In effect, the accepted residual of
[ADR-0093](./0093-composite-scan-gate.md) shrinks precisely for the highest-value secret class, at
the exact moment of exposure. The pattern tier is the Content Scanner's first tier run alone,
without its scoring tier ([ADR-0005](../classification/0005-tiered-detection.md)). A
`SKIPPED_GATE` body with no built scanner to check it is withheld, and a scanned body is not
checked again. The check reads everything the release hands the client from the message, the body
and the snippet and attachment filenames that follow it under
[ADR-0001](./0001-redaction-matrix.md)'s matrix, so a hit in any of them withholds all of them.
Attachment filenames are the one released field no scanner reads on any scan state, so the same
reasons apply to them on every release. A scanned message's filenames pass the pattern tier too,
and a hit withholds its body, snippet and filenames with the same audit row, while its body and
snippet are not checked again.

## Alternatives considered

- **Trust the classification cached at enumeration time.** No case was tabled for it. Rejected: a
  deny-list addition would keep leaking until a re-sync, which is exactly the failure
  [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released) names as falsifying.
- **Count the stored sender class toward denial as well.** The case for it is that a stored state
  nobody built, whose sender class is restricted, would be denied as a restricted sender rather than
  under another reason. Rejected, because the stored class is rewritten only when a sender is
  delisted ([ADR-0037](./0037-delisting-transition.md)) or an added rule reaches it by its effect
  ([ADR-0113](./0113-an-added-rule-reaches-the-stored-classes-by-its-effect.md)), each through the
  comparison a scanning workload makes, so it lags the policy in force and cannot decide a denial.
- **Accept caller-supplied sensitivity context** (e.g. the sensitivity block a client received with
  the metadata). No case was tabled for it. Rejected: anything a client supplies, a suborned client
  can forge. Inputs to the release decision must come only from state no client can write.
- **Fetch the body first, then decide.** No case was tabled for it. Rejected: it moves every denied
  body through mediator memory for no benefit, enlarging the blast radius of any spill or logging
  bug. The serve-time pattern check is not this. It never fetches in order to decide, since the
  gate's release decision triggered the fetch, and the check can only subtract from what that
  decision would release.
- **No check at release for unscanned bodies.** Its case: the residual was already accepted and
  measured. Displaced: microseconds spent on an already-fetched body remove the highest-value
  secret class from the residual.
- **Full scanning at release.** No case was tabled for it. Rejected: the out-of-band placement
  reasons ([ADR-0009](./0009-scanner-verdicts-carry-no-content.md)) bind the full scanner, which are
  timeouts, latency, and fail-open pressure in the serving path.

## Consequences

- Every fetch decision, allow and deny alike, writes an audit row. The denial envelope carries a
  reason distinct enough that "pending content scan" reads as backlog, not as a permissions bug the
  agent will misreport.
- The gate's deny branches depend only on the index, so gate behavior is fully testable offline
  with fixtures, with no provider and no network.
- Fetch-time re-evaluation means the index's *cached classification* never decides release on its
  own, because sender class is re-derived against current policy at fetch time. Scan state is index
  state and does gate release, re-read at fetch time and never trusted from enumeration.
- Assumptions about other components: the pattern tier is a pure function usable outside the
  scanner's batch context (no external calls, no state), and the sanitization step
  ([ADR-0036](./0036-released-bodies-are-clean-markdown.md)) sees every released body, so hosting
  the check there covers every `SKIPPED_GATE` release. The check's violation injection is
  catalogued in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
