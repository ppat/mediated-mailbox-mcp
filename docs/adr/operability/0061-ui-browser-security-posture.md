# 0061. A state-changing request is accepted only with a token bound to the anonymous session that loaded the page

**Status:** Accepted ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

The UI carries the two decisions no client may reach, OAuth client setup and account setup
([ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)), and in its first version
has no authentication of its own. A page on another origin, opened in the operator's browser, could
otherwise submit a decision or replace a client or an account's credential with the operator's
cookies. The operator delegated the UI's design to the designing session on 2026-09-09, and this
is the mechanism that session chose, recorded so the control resting on it has a decision behind
it.

## Decision

- **The UI issues an anonymous session cookie on first response**, a random identifier,
  `HttpOnly`, `Secure`, `SameSite=Strict`, with browser-session lifetime, whether or not a
  proxy authenticates in front of the UI.
- **The entry document carries a token derived from that session.** The entry document is the
  one page a Go handler renders. The token is an HMAC of the session identifier under a key
  generated at process start, or a configured key when more than one replica runs. Everything
  else the UI serves is static.
- **Every state-changing request sends the token in a header, and the server checks it against the
  cookie.** A request without a matching token is refused before any write. The requests of the
  decisions, OAuth client setup and account setup are the only state-changing requests.

## Alternatives considered

- **A double-submit cookie, echoed by the browser, instead of a server-issued token.** Its
  case was no server-side derivation. Rejected because the entry document is rendered anyway,
  and a token bound to the session under a key the browser never sees is stronger than a value
  the browser echoes.
- **The cookie's `SameSite` attribute alone, with no token.** Its case was one less
  mechanism. Rejected because a cookie attribute is a browser policy, not a server check, and
  the state-changing requests deserve a check the server performs.

## Consequences

- The browser bundle ships as static files inside the Go binary
  ([ADR-0042](../engineering/0042-implementation-stack.md)), with the entry document as the
  one rendered page.
- The token check is a control. Its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- Assumptions about other components: the identity a decision records comes from
  [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)'s rule and is independent
  of the token, which proves only that the request came from a page the UI served.
