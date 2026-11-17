# 0082. A rotated provider credential is sealed and written back to the account's state row by the deployable that received it

**Status:** Accepted (supersedes [ADR-0039](./0039-rotation-writeback.md)) ·
**Pillar:** [The mediation layer is the irreducible trust anchor](../../../DESIGN.md#the-mediation-layer-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes)

## Context

A provider may rotate a credential it issued. When that happens, the new value must reach where the
credential is stored, or the next restart resumes from the stale credential and mailbox access is
gone, the most common way systems like this die quietly. Credentials are stored sealed in the
database ([ADR-0080](../data/0080-accounts-and-credentials-live-in-the-database.md),
[ADR-0081](./0081-credentials-sealed-to-a-public-key.md)), and the deployables that call a provider
already connect to it.

## Decision

- **The deployable that receives a rotated credential seals it and writes it to the account's
  state row** ([ADR-0091](../data/0091-accounts-listed-apart-from-their-state.md)), in the same
  database it reads credentials from. Its write of the account's credential sets that one column,
  nothing more.
- **No other store takes part.** No deployable holds a credential for a secret store, because no
  account credential lives in one.
- **The process keeps the new value in memory**, so a failed write loses access only if the process
  restarts before a later write succeeds. A failed write is loud in that deployable's logs.
- Rotations arriving at two deployables close together are reconciled by the compare-and-set of
  [ADR-0089](./0089-sealed-values-written-by-compare-and-set.md).

## Alternatives considered

- **Write back through a secret store synced into mounted files**
  ([ADR-0039](./0039-rotation-writeback.md)). For it, no deployable writes where credentials are
  stored except through one narrow location. Against it, account credentials live in the database
  and not in a secret store, so the loop has nothing to carry.
- **No write-back, re-authorize through the UI when a rotation is lost.** For it, the UI already
  re-authorizes an account. Against it, the failure is silent and delayed, since everything works
  until the next restart, which is the shape of failure found weeks later mid-incident.

## Consequences

- Rotation write-back stays the deliberately exercised day-one drill. Force a rotation, restart,
  confirm access survives. The drill and its injection are catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- The application user, which the API server, the UI and the worker connect as, can update the
  credential column of every account's row
  ([ADR-0143](../data/0143-the-programs-connect-as-one-application-user-beside-the-migrators.md)).
  So a compromised provider-calling deployable can overwrite a stored credential, as any
  compromised program can. A provider-calling deployable already holds the credential in plaintext.
- Assumptions about other components. The update's statement compares the account with the
  transaction's, so it reaches only the account the transaction set
  ([ADR-0135](../data/0135-every-statement-reads-its-account-from-the-transaction.md)).
