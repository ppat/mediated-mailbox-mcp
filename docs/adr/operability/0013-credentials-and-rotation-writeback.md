# 0013. Credentials arrive as mounted files via the external secret store — and rotation writes back

**Status:** Superseded — **Superseded by:**
[ADR-0038](./0038-credentials-as-mounted-files.md) (delivery and containment) and
[ADR-0039](./0039-rotation-writeback.md) (rotation write-back) ·
**Pillar:** [The mediator is the irreducible trust anchor](../../../DESIGN.md#the-mediator-is-the-irreducible-trust-anchor) ·
**Serves:** [O3](../../../USE_CASES.md#o3--survives-its-failure-modes), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The mediator holds the most dangerous credentials in the system — full-mailbox provider tokens.
They must reach the pod without living in git, survive pod restarts, and survive the provider
rotating them. The platform is Kubernetes under GitOps, with Bitwarden Secrets Manager as the
external secret store and External Secrets Operator syncing it into the cluster.

## Decision

```
Bitwarden Secrets Manager
        │ (store access token in a bootstrap Secret)
        ▼
External Secrets Operator ──► Secret/mail-mediator-creds
        │                       └─ accounts/personal/gmail_refresh_token ← mutable
        ▼
Deployment mounts as files (not env vars — env leaks via /proc and crash dumps)
```

- **Files, not environment variables.** Environment variables leak through `/proc`, crash dumps,
  and child-process inheritance; file mounts do not.
- **Refresh-token rotation writeback.** Google may rotate refresh tokens; when it does, the pod
  must write the new value back to the secret store via its API — otherwise the next restart
  resumes from the stale token and mailbox access is gone. This is the most common way systems
  like this die quietly, so the writeback path is exercised deliberately, first, before anything
  is built on top of it (its proving check is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md)).
- **A Kyverno admission policy fences the Secret:** no pod in the namespace may mount the
  credentials Secret except the mediator's own ServiceAccount.
- **No credential ever crosses the client boundary.** Clients authenticate to the mediator with a
  bearer token distinct from every provider credential; the mediator authenticates to providers.
  Two trust domains that never mix.
- **The UI holds no provider credentials at all** — only a scoped database role
  ([ADR-0021](../mutation/0021-approval-surface.md)). It cannot reach a mailbox.

## Alternatives considered

- **Credentials in environment variables.** Rejected for the leak surfaces above.
- **Credentials baked into the GitOps repo (sealed or encrypted).** Rejected: rotation writeback
  needs a mutable store the *pod* can write; a git-mediated loop would put a commit-and-reconcile
  cycle inside an authentication failure window.
- **No writeback — re-consent manually when rotation happens.** Rejected: the failure is silent
  and delayed (everything works until the next restart), which is precisely the shape of failure
  that gets discovered weeks later, mid-incident.

## Consequences

- The secret store is a hard runtime dependency of credential rotation, and the bootstrap token
  for it is itself a credential to manage — accepted as the standard cost of external secret
  management.
