# 0021. The approval surface is a separate UI that writes the database directly — two verbs, no credentials

**Status:** Accepted ·
**Pillar:** [Approval is not in any client's vocabulary](../../../DESIGN.md#approval-is-not-in-any-clients-vocabulary) ·
**Serves:** [G3](../../../USE_CASES.md#g3--reorganization), [A3](../../../USE_CASES.md#a3--bulk-change-is-reversible), [O2](../../../USE_CASES.md#o2--observable)

## Context

Two human workflows need a surface no client can reach: approving reorganization plans
([ADR-0020](./0020-reorg-plan-approve-apply-rollback.md)) and confirming sensitive-sender
candidates ([ADR-0004](../classification/0004-sender-list-decides.md)). The same surface is where
the operator reads what the system is doing — the audits and measurements the design keeps
insisting on are only real if a human can see them.

## Decision

**A read-mostly web UI with exactly two write verbs**, deployed and privileged separately from the
mediator:

| View | Purpose |
| --- | --- |
| **Corpus overview** | Volume by sender, label distribution, unfiled counts, classification breakdown — makes backfill output legible to the operator, not just the agent |
| **Reorg plans** | List, diff, sample of affected messages, per-message reasoning, **approve/reject** — the load-bearing screen |
| **Review queue** | Ranked heuristic candidates with the signal that flagged them; **confirm/dismiss** into the policy store — what keeps the sender list from going stale |
| **Masking events** | What was masked, why, which rule — tunes the masking posture from real traffic |
| **Scan gate decisions** | Skip rates by reason and sender — makes the accepted residual auditable |
| **Audit log** | Every body served, every denial, every mutation |

Constraints that keep it safe to exist:

- **Writes go directly to Postgres, never through the client surface** — preserving every
  client's structural inability to approve its own plans.
- **Separate Deployment, ServiceAccount, and database role**: read-only on most tables; its write
  grant is exactly `reorg_plans(status, approved_at, approved_by)` and
  `policy_candidates(status, reviewed_at)` — the columns each verb sets, nothing else. The limit
  is enforced by database permissions, so a UI bug cannot become a mailbox mutation.
- **No provider credentials.** The UI cannot reach a mailbox at all.
- **It never displays message bodies** — structurally, because it reads a database with no body
  columns ([ADR-0016](../data/0016-schema.md)). Stated here so nobody later adds a "preview"
  feature by proxying through the mediator.
- **TLS and its own auth, LAN-only**, same posture as the client surface
  ([ADR-0014](../operability/0014-lan-only-transport.md)).

The shape is a small single-page app over a thin read API — deliberately unambitious, since its
value is legibility plus two buttons.

## Alternatives considered

- **Approval through the mediator's client surface (a privileged human token).** Rejected:
  it puts the approval verb back into the surface clients speak, one credential-handling bug
  away from a client's reach. Separate surface, separate process, separate identity.
- **CLI-only approval, no UI.** Workable for the approve verb alone, and acceptable as an interim
  — but it fails the legibility half: skip rates, masking events, and candidate evidence need
  visual review, and an unmeasured accepted risk is the thing this design refuses to carry.
- **A full-featured mail client UI.** Rejected: every feature added to a surface with write access
  is blast radius; the two-verb constraint is what makes the UI boring, and boring is the goal.

## Consequences

- Compromise of the UI yields two verbs: approving a plan — which the Mutation Authorizer still
  constrains at apply time — and confirming policy candidates. It cannot read a mailbox, serve a
  body, or mutate mail directly.
- The UI is where the operator's standing duties live — plan review, candidate review, masking
  review — so its legibility is a safety property, not a nicety.
