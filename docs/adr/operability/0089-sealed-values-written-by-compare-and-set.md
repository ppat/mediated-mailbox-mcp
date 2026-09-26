# 0089. A deployable writes a sealed value only if the stored bytes are still the ones it last knew

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

A stored credential and an OAuth client's secret are sealed values
([ADR-0081](./0081-credentials-sealed-to-a-public-key.md)). Three kinds of write replace one. The UI
replaces a value when the operator re-authorizes an account or sets up an OAuth client again
([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)). A deployable that calls a
provider writes back a rotated credential
([ADR-0082](./0082-rotation-writeback-to-the-database.md)). Delta sync re-seals a value to a new key
([ADR-0092](./0092-key-replacement-by-keyring-and-re-seal.md)). Every deployable that calls a
provider holds the credentials in memory while any of these writes changes the stored one.

Gmail does not rotate refresh tokens, as its documentation of the refresh response shows
([Google's OAuth for installed apps](https://developers.google.com/identity/protocols/oauth2/native-app)),
so two rotations racing has no trigger with the providers chosen today. A deployable writing back
or re-sealing a value based on bytes the operator has since replaced does. It would put the old
grant back over the one the operator just connected, and if the old grant was revoked, the next
restart would lose the mailbox, which is
[O3](../../../USE_CASES.md#o3--survives-its-failure-modes)'s failure. Each account's grant is its
own ([ADR-0085](../provider/0085-multi-account-contexts-with-an-installation-client.md)), so the
rule holds per account.

## Decision

- **A deployable replaces a sealed value only if the stored bytes are still the bytes it last read
  or wrote.** Every write of a sealed value by a deployable, a rotation write-back or a re-seal, is
  a compare-and-set on those bytes. A write that finds other bytes changes nothing, and the
  deployable reads the stored value again, opens it and uses it, dropping its own.
- **The UI's writes replace unconditionally**, because they are the operator's decision and the
  newest grant.
- **The bytes a deployable last knew are also what a reload compares against**
  ([ADR-0090](./0090-accounts-reach-deployables-as-reloaded-snapshots.md)), so a reload never
  replaces a rotated credential the deployable holds while the stored one is unchanged.

## Alternatives considered

- **Last writer wins.** For it, no predicate and the lowest cost. Against it, a write-back landing
  after the operator re-authorized puts the old grant back, silently, until the next restart fails.
  It would also give the column two write rules beside the compare-and-set a re-seal needs.
- **Refreshes of one account serialized across processes by an advisory lock**, following
  `db/ratestate`'s lock. For it, at most one refresh of a grant is in flight anywhere, which also
  answers a provider that revokes a grant when an old refresh token is reused. Against it, no
  chosen provider rotates, it holds a transaction open across a call to the token endpoint, and a
  process killed between the provider's answer and the commit still leaves the old token stored.
  It needs this record's compare-and-set anyway, so it can be added on top.
- **One grant per deployable per account.** For it, no two processes ever share a token. Against it,
  it breaks ADR-0085's one grant per account, and the operator would consent up to four times for
  each account.

## Consequences

- A rotation write-back or a re-seal that loses to a re-authorization is discarded, and the
  deployable carries on with the value the operator stored.
- A provider that rotates refresh tokens and revokes a grant when an old one is reused cannot be
  repaired by a compare-and-set after the fact. Such a provider needs the advisory lock above, and
  the kill window it leaves open is recorded against it then.
- Assumptions about other components. The write goes through a statement whose predicate names the
  stored bytes, so the database, not the application, decides who won.
