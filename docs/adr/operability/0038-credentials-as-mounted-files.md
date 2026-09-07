# 0038. Credentials arrive as mounted files

**Status:** Accepted (supersedes [ADR-0013](./0013-credentials-and-rotation-writeback.md),
jointly with [ADR-0039](./0039-rotation-writeback.md)) ·
**Pillar:** [The mediator is the irreducible trust anchor](../../../DESIGN.md#the-mediator-is-the-irreducible-trust-anchor) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The mediator holds the most dangerous credentials in the system — full-mailbox provider tokens.
They must reach it without ever living in the configuration repository, and survive restarts.

## Decision

- **Files, not environment variables.** Environment variables leak through `/proc`, crash dumps,
  and child-process inheritance; file mounts do not.
- **The mediator never fetches credentials itself.**
- **Only the mediator can read the credential files.**
- **No credential ever crosses the client boundary.** Clients authenticate to the mediator with a
  bearer token distinct from every provider credential; the mediator authenticates to providers.
  Two trust domains that never mix.
- **The UI holds no provider credentials at all** — only a scoped database role
  ([ADR-0021](../mutation/0021-approval-surface.md)). It cannot reach a mailbox.

What happens when a provider rotates a credential is the separate decision of
[ADR-0039](./0039-rotation-writeback.md).

## Alternatives considered

- **Credentials in environment variables.** No case was tabled for them. Rejected for the leak
  surfaces above.
- **Credentials baked into the configuration repository (sealed or encrypted).** No case was
  tabled for it. Rejected; the argument turns on rotation write-back and lives with
  [ADR-0039](./0039-rotation-writeback.md).

## Consequences

- Credential delivery happens outside the mediator; if it stalls, the mounted files go stale.
  Accepted.
- Assumptions about other components: something outside the mediator delivers the files and
  enforces the read restriction.
