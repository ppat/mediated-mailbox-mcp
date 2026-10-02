# 0111. A consent attempt travels in a cookie the UI server seals, bound to the session that started it

**Status:** Accepted ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

Connecting or re-authorizing an account runs a consent at the provider from the UI
([ADR-0107](../provider/0107-gmail-through-an-installed-app-oauth-client-set-up-in-the-ui.md)). The
UI issues the consent's state and PKCE verifier when the operator confirms the first step, and needs
them again when the operator pastes back the address Google redirected to, minutes later. The
attempt also carries what the operator named at the first step, the account identifier, the mailbox,
the client chosen and a lowered rate target, and whether it connects, re-authorizes or moves an
account.

[docs/UI.md section 8.12](../../UI.md#812-connect-an-account-and-re-authorize) holds an attempt for
the session that started it, one per session with a newer one replacing it, across a reload of the
page and whichever replica answers. The UI may run more than one replica, which is why its request
token key can come from a mounted file
([ADR-0061](./0061-ui-browser-security-posture.md)). Its database grant is closed to the writes
[ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md) names.

## Decision

- **The attempt is held in a cookie whose value the UI server seals.** The value is the attempt's
  fields encrypted with an authenticated cipher, so the browser carries it and can neither read it
  nor alter it. The cookie is `HttpOnly`, `Secure` and `SameSite=Strict`, sent only to the read API's
  paths, with browser-session lifetime.
- **The seal is bound to the session.** Its additional data is the `ui_session` identifier
  ([ADR-0061](./0061-ui-browser-security-posture.md)), so an attempt sealed for one session opens in
  no other.
- **The sealing key is derived from the request token's key**, under a label of its own, so the two
  keys never coincide. A replica holding the key the `token_key_file` names opens an attempt any
  replica sealed. A replica using a key generated at its own start opens only its own.
- **A session holds one attempt.** Starting an attempt sets the cookie, replacing any value it held,
  so only the newest attempt's state can finish. A finish that opens no attempt answers as no attempt
  in progress, and one past its expiry, which the sealed value carries, answers as expired before
  the pasted address is read.
- **A finish that stores the account clears the cookie.** A refused finish leaves the attempt in
  place until it expires, so the operator opens the consent page again and pastes the new address,
  as the refusals' wording tells them to.

The attempt carries its state, its PKCE verifier, the account identifier, the mailbox and whether
the account already remembers it, the client's name with the client identifier it held when the
attempt started, the lowered target, its expiry, and whether it connects, re-authorizes or moves an
account.

## Alternatives considered

- **A table the replicas share.** For it, the browser carries nothing but its session. Against it,
  [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)'s grant names no such write,
  so the record would widen it, and a table needs expiry and a cleanup of attempts abandoned
  mid-way.
- **Each replica's process memory.** For it, nothing is written anywhere. Against it, a paste
  answered by another replica than the one that started the attempt finds nothing, which the
  design's "whichever replica answers" refuses.
- **Sticky sessions at the ingress.** For it, process memory would then work. Against it, it is a
  requirement on the deployment the artifacts would have to declare, which
  [O6](../../../USE_CASES.md#o6--deployable) counts against, and it still loses every attempt on a
  restart.

## Consequences

- A UI that restarts with a key generated at its start cannot open the attempts it sealed before,
  so an operator mid-way through a consent starts again. The page's request token goes stale at the
  same restart ([docs/UI.md section 17.3](../../UI.md#173-the-error-contract)).
- The PKCE verifier and the state travel to the browser sealed. Anyone who can read the browser's
  cookie jar already holds the session the attempt is bound to, so sealing protects the attempt from
  scripts and from other sites, not from the browser's owner.
- A copy of an earlier attempt's cookie, saved from the cookie jar and put back, opens until its
  expiry, since nothing server-side remembers which attempt is newest. Putting it back takes the
  same access to the cookie jar.
- The guard against a second account connecting under an identifier checked free at the first step
  is the finish's own check in the transaction that writes the account, not the attempt
  ([docs/UI.md section 8.12](../../UI.md#812-connect-an-account-and-re-authorize)).
- Assumptions about other components. Every replica of one installation mounts the same
  `token_key_file` when more than one runs
  ([docs/UI.md section 18.1](../../UI.md#181-the-configuration-the-ui-declares)), and the browser
  keeps a cookie of a few hundred bytes for the session.
