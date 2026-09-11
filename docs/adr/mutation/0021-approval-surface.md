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
| **Jobs** | Every batch workload live, its progress, the rate budget by priority class, recent runs |
| **A run** | One run, and a failed one down to its individual failures |

The screens themselves, how they are organized, and what they read are the UI's design in
[docs/UI.md](../../UI.md).

Constraints that keep it safe to exist:

- **Writes go directly to Postgres, never through the client surface** — preserving every
  client's structural inability to approve its own plans.
- **Separate Deployment, ServiceAccount, and database role**: read-only on most tables; its write
  grant is exactly `reorg_plans(status, approved_at, approved_by)`,
  `policy_candidates(status, reviewed_at, reviewed_by)`, and insert on `policy_rules`. That is
  the columns each verb sets, plus the one row confirming a candidate emits
  ([ADR-0004](../classification/0004-sender-list-decides.md)), and nothing else. The limit is
  enforced by database permissions, so a UI bug cannot become a mailbox mutation. A decision is
  recorded in those columns by the UI's own code in one transaction, with no database-resident
  code ([ADR-0060](../engineering/0060-no-code-in-the-database.md)). The identity it records is
  the value of a header the deployment declares an authenticating proxy sets, else a configured
  operator name.
- **No provider credentials.** The UI cannot reach a mailbox at all.
- **It never displays message bodies** — structurally, because it reads a database with no body
  columns ([ADR-0016](../data/0016-schema.md)). Stated here so nobody later adds a "preview"
  feature by proxying through the mediator.
- **TLS**, same posture as the client surface. **Authentication is not in the first version.**
  The operator may place the UI behind an ingress that forwards to an authentication service and
  sets a cookie, with the identity header above. The UI's own authentication (OpenID Connect,
  say) may come later.

The shape is a small single-page app over a thin read API — deliberately unambitious, since its
value is legibility plus two buttons. Its design is [docs/UI.md](../../UI.md).

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

- Compromise of the UI yields two verbs. The first approves a plan, which the Mutation
  Authorizer still constrains at apply time. The second confirms a candidate, whose rule insert
  can only add a restriction and whose invalid rules never take effect
  ([ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md)). A compromised UI cannot
  read a mailbox, serve a body, or mutate mail directly.
- A write that changed nothing must not reach the operator as a successful one. The two ways a
  write can fail here behave oppositely. A grant the role does not hold raises, so it is visible on
  its own. A policy that excludes the row instead empties the statement, which succeeds having
  changed nothing, and without care that reaches the operator as the stale-status conflict the
  expected-status check produces, which is the wrong account of what happened. Telling those apart
  is a mechanism question and belongs to
  [ADR-0066](../data/0066-data-access-generated-from-sql.md). One consequence of the grant being
  the control is that its refusal names the table and not the column, so it carries nothing the
  operator could act on and maps to the internal-fault origin of the error contract rather than to
  a client fault.
- The UI is where the operator's standing duties live — plan review, candidate review, masking
  review — so its legibility is a safety property, not a nicety.
