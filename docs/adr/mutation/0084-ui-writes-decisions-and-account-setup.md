# 0084. The UI is a separate surface that writes the database directly, for two decisions, OAuth client setup and account setup, and seals credentials it can never open

**Status:** Accepted (supersedes [ADR-0021](./0021-approval-surface.md)) ·
**Pillar:** [Approval is not in any client's vocabulary](../../../DESIGN.md#approval-is-not-in-any-clients-vocabulary) ·
**Serves:** [G3](../../../USE_CASES.md#g3--reorganization), [A3](../../../USE_CASES.md#a3--bulk-change-is-reversible), [O2](../../../USE_CASES.md#o2--observable), [O6](../../../USE_CASES.md#o6--deployable)

## Context

Two human workflows need a surface no client can reach, approving reorganization plans
([ADR-0020](./0020-reorg-plan-approve-apply-rollback.md)) and confirming sensitive-sender
candidates ([ADR-0004](../classification/0004-sender-list-decides.md)). The same surface is where
the operator reads what the system is doing, since the audits and measurements the design keeps
insisting on are only real if a human can see them. An installation's OAuth client is set up
through the UI ([ADR-0083](../provider/0083-gmail-through-an-installation-oauth-client.md)), and
accounts are connected and repaired through it
([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md)), so the UI also writes
both.

## Decision

**A read-mostly web UI, deployed and privileged separately from the mediator. Its writes are two
decision verbs, OAuth client setup and account setup.**

| View | Purpose |
| --- | --- |
| **Corpus overview** | Volume by sender, label distribution, unfiled counts, classification breakdown. It makes backfill output legible to the operator, not just the agent |
| **Reorg plans** | List, diff, sample of affected messages, per-message reasoning, **approve/reject**. The load-bearing screen |
| **Review queue** | Ranked heuristic candidates with the signal that flagged them, **confirm/dismiss** into the policy store. What keeps the sender list from going stale |
| **Masking events** | What was masked, why, which rule. Tunes the masking posture from real traffic |
| **Scan gate decisions** | Skip rates by reason and sender. Makes the accepted residual auditable |
| **Audit log** | Every body served, every denial, every mutation |
| **Jobs** | Every batch workload live, its progress, the rate budget by priority class, recent runs |
| **A run** | One run, and a failed one down to its individual failures |
| **OAuth client setup** | Setting up the installation's OAuth client for a provider through the guided flow, once, apart from any account |
| **Account setup** | Connecting an account through that client, setting what the account's rows hold, and re-authorizing an account whose credential stopped working |

The screens themselves, how they are organized, and what they read are the UI's design in
[docs/UI.md](../../UI.md).

Constraints that keep it safe to exist:

- **Writes go directly to Postgres, never through the client surface**, preserving every client's
  structural inability to approve its own plans.
- **Separate Deployment, ServiceAccount, and database role.** It is read-only on most tables. Its
  read of `account_state` arrives with the statement that needs it and never covers `credential`
  ([ADR-0091](../data/0091-accounts-listed-apart-from-their-state.md)). Its write grant is exactly the columns each decision verb sets, `reorg_plans(status, approved_at,
  approved_by)` and `policy_candidates(status, reviewed_at, reviewed_by)`, insert on
  `policy_rules` for the one row confirming a candidate emits
  ([ADR-0004](../classification/0004-sender-list-decides.md)), and the columns OAuth client setup
  and account setup write, and the writes on `policy_rules` that policy management makes, which is
  importing, adding and editing rules (ADR-0004), and nothing else. The setup columns are named
  where the tables holding the client and the account are designed, and the policy writes where
  policy management is built, and each is added to this grant then. The limit is enforced by
  database permissions, so a UI bug cannot become a mailbox mutation. A write is made by the UI's own code in one transaction, with no
  database-resident code ([ADR-0060](../engineering/0060-no-code-in-the-database.md)). The
  identity a decision records is the value of a header the deployment declares an authenticating
  proxy sets, else a configured operator name.
- **It seals credentials and can never open one.** The UI runs a provider's consent exchange when
  an account is connected or re-authorized
  ([ADR-0083](../provider/0083-gmail-through-an-installation-oauth-client.md)), seals the grant and
  the client's secret to the public key, and stores them. It holds no private key
  ([ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md)). It calls a provider only
  to check a client, complete a consent, and confirm which mailbox granted it.
- **While it completes a consent it is part of the trust anchor.** It holds a full-mailbox grant in
  plaintext until it seals it, so it runs under the same hardening as the deployables that call a
  provider ([ADR-0028](../operability/0028-trust-anchor-hardening.md)).
- **It never displays message bodies**, structurally, because it reads a database with no body
  columns ([ADR-0016](../data/0016-schema.md)). Stated here so nobody later adds a "preview"
  feature by proxying through the mediator.
- **TLS**, same posture as the client surface. **Authentication is not in the first version.** The
  operator may place the UI behind an ingress that forwards to an authentication service and sets
  a cookie, with the identity header above. The UI's own authentication (OpenID Connect, say) may
  come later.

The shape is a small single-page app over a thin read API. Its value is legibility, two decisions,
and connecting accounts. Its design is [docs/UI.md](../../UI.md).

## Alternatives considered

- **Approval through the mediator's client surface (a privileged human token).** Rejected. It puts
  the approval verb back into the surface clients speak, one credential-handling bug away from a
  client's reach. Separate surface, separate process, separate identity.
- **CLI-only approval, no UI.** Workable for the approve verb alone, and acceptable as an interim.
  It fails the legibility half, since skip rates, masking events, and candidate evidence need
  visual review, and an unmeasured accepted risk is the thing this design refuses to carry.
- **A full-featured mail client UI.** Rejected. Every feature added to a surface with write access
  is blast radius, and a small write surface is what keeps the UI boring, which is the goal.
- **A separate deployable owning provider and account setup.** For it, the UI stays free of every
  credential. Against it, one more component, image and chart entry, holding the same plaintext
  moment the UI would.
- **The UI with a symmetric key that opens credentials too.** Rejected in
  [ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md).

## Consequences

- Compromise of the UI yields two decision verbs, OAuth client setup and account setup. An
  approved plan is still constrained by the Mutation Authorizer at apply time. A confirmed
  candidate's rule insert can only add a restriction, and invalid rules never take effect
  ([ADR-0041](../engineering/0041-policy-as-immutable-snapshots.md)). Setup lets it replace a
  client or an account's credential, or capture a grant while a connection or re-authorization is
  under way. It cannot open a stored credential, serve a body, or mutate mail directly.
- With no authentication in the first version, anyone who reaches the UI can set up a client,
  connect an account or replace a credential. The request token of
  [ADR-0061](../operability/0061-ui-browser-security-posture.md) stops another site from forging
  those requests through the operator's browser. It does not stop a person on the network.
- The UI reaches the provider's token endpoint and the mailbox profile it checks a grant against,
  so its egress includes those endpoints.
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
- The UI is where the operator's standing duties live, plan review, candidate review, masking
  review and keeping accounts connected, so its legibility is a safety property, not a nicety.
