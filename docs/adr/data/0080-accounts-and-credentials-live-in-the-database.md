# 0080. Accounts and their provider credentials are created and repaired through the UI and live in the database

**Status:** Accepted (supersedes [ADR-0038](../operability/0038-credentials-as-mounted-files.md),
jointly with [ADR-0079](../operability/0079-secrets-arrive-as-mounted-files.md)) ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [P3](../../../USE_CASES.md#p3--multi-account), [O6](../../../USE_CASES.md#o6--deployable)

## Context

Every deployable that calls a provider holds a full-mailbox provider credential. The mediator uses
one to fetch a released body ([ADR-0002](../redaction/0002-fetch-time-re-evaluation.md)), backfill
and delta sync to build and refresh the index ([ADR-0017](./0017-two-pass-backfill.md),
[ADR-0018](./0018-delta-sync-polls.md)), and the reorganization workload to apply a plan
([ADR-0020](../mutation/0020-reorg-plan-approve-apply-rollback.md)).

Without a flow of its own, connecting an account is a manual procedure outside the system, running
a consent command and delivering its output as mounted files alongside an account entry in
configuration. The aim is
that the operator connects a mailbox, and fixes or re-authorizes it when needed, through a guided
flow in the UI rather than a set of manual steps that are hard to follow.

## Decision

- **An account is created through the UI's guided setup.** Its identifier, its provider, the rate
  target the operator may lower ([ADR-0024](../operability/0024-conservative-target-aimd.md)) and
  its provider credential are stored in the database. No account and no provider credential comes
  from configuration or from a mounted file.
- **The credential is stored encrypted**, sealed as
  [ADR-0081](../operability/0081-credentials-sealed-to-a-public-key.md) decides.
- **The OAuth client an installation connects through is not part of any account.** It is set up
  once, through a flow of its own apart from connecting an account, and stored apart from the
  accounts, its secret sealed like a credential
  ([ADR-0083](../provider/0083-gmail-through-an-installation-oauth-client.md)). Every account of
  that provider connects through it, each with its own grant.
- **The UI repairs an account.** When a credential stops working, the operator re-authorizes the
  account through the UI, which replaces the stored credential.
- **The deployables that call a provider read their accounts, the installation's OAuth client and
  the credentials from the database.**
  A provider rotating a credential is written back there, as
  [ADR-0082](../operability/0082-rotation-writeback-to-the-database.md) decides.
- **No provider credential ever crosses the client boundary.** Clients authenticate to the mediator
  with a bearer token distinct from every provider credential. Each deployable that calls a
  provider authenticates to that provider. Two trust domains that never mix.

## Alternatives considered

- **Accounts in configuration, credentials as mounted files** (the superseded design). For it, an
  account is declared alongside the rest of a deployment and applied the same way, and no process
  outside the provider-calling deployables ever handles a credential. Against it, connecting a
  mailbox is a manual procedure outside the system, and repairing one repeats it.
- **Accounts in the database, credentials still as mounted files.** No case was tabled for it. It
  keeps the manual procedure for the part that is hardest to follow.

## Consequences

- Account setup is state in the database rather than something declared with the deployment.
- The database, and every backup of it, holds each account's credential in sealed form.
- An account can be added or re-authorized while the deployables run. How a running deployable
  learns of a new account or a replaced credential is open in
  [ROADMAP.md](../../../ROADMAP.md#open-decisions).
- The schema of [ADR-0016](./0016-schema.md) gains what the account row must hold.
- Assumptions about other components. Row-level security on the account tables lets a transaction
  that set the account write that account's own rows, and nothing else.
