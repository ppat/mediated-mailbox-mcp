# 0012. Fastmail auth: scoped API tokens per protocol category, never the account password

**Status:** Accepted ·
**Pillar:** [Accounts are isolated by structure, not convention](../../../DESIGN.md#accounts-are-isolated-by-structure-not-convention) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [P3](../../../USE_CASES.md#p3--multi-account)

## Context

The Fastmail adapter needs credentials for mail (JMAP) and, separately, calendar (CalDAV — see
[ADR-0027](./0027-calendar-classification.md)). Fastmail app passwords scope to protocol
categories, which is scoping worth exploiting.

## Decision

- **JMAP API token, not the account password.** A token is scoped and revocable; the password is
  neither.
- **Separate tokens for Mail and for Calendar**, so a calendar-path bug cannot read mail. Same
  principle as scope minimalism on Gmail ([ADR-0011](./0011-gmail-auth-installed-app-oauth.md)):
  capability absent from a credential is a guarantee, not a configuration.
- The adapter reads `.well-known/jmap` for session discovery rather than hardcoding endpoints.

## Alternatives considered

- **One app password covering both protocols.** Rejected: it braids two independently-failing
  paths onto one credential for no operational gain.
- **The account password.** Rejected outright — it grants everything a token would be scoped away
  from, and revoking it means rotating the account's password itself.

## Consequences

- Fastmail credentials follow the same storage and mounting rules as all others
  ([ADR-0013](../operability/0013-credentials-and-rotation-writeback.md)).
