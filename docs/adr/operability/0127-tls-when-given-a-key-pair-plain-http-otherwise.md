# 0127. The API server and the UI serve TLS when given a certificate and key, and plain HTTP when given neither

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

The API server's client surface and the UI each serve their clients over HTTP. Where TLS ends is a
property of the platform. Some platforms terminate TLS in front of the app, at an ingress or a
gateway, and hand it plain HTTP. Others hand the app a certificate.
[ADR-0051](../engineering/0051-environment-contract.md) says the app knows its environment
contract and never its platform, so a deployable that asks to be told which kind of platform it is
on, or refuses to run on one kind, knows its platform.

The bearer check in front of the client surface is
[ADR-0030](./0030-api-core-mcp-thin-adapter.md)'s, and the UI's lack of authentication of its own
is [ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md)'s.

## Decision

- **Each of the two deployables serves TLS when its configuration names both a certificate file
  and a key file, and plain HTTP when it names neither.** They are `tls_cert` and `tls_key` on both,
  mounted files as [ADR-0079](./0079-secrets-arrive-as-mounted-files.md) has secret material
  arrive. A path that is empty or only spaces counts as unset.
- **A configuration naming exactly one of the two files is refused at start**, before any key
  file is read, with a message naming the file that is missing.
- **No other setting chooses the transport.** There is no flag declaring an ingress, no flag
  asking for plain HTTP, and no build of the binary that serves differently from another.
- **Serving TLS, the key pair is read from its files on each handshake**, so a renewed certificate
  serves without a restart. The API server reports ready only once the key pair loads, and the UI
  loads it before it serves anything. Serving plain HTTP, neither reads a key file.
- **Each deployable's start log records which transport it serves**, as an info record, beside the
  TLS file paths the effective configuration logs. Neither deployable warns when it serves plain
  HTTP, since a warning would be an opinion about the platform.
- **What holds in either mode.** The bearer check stands in front of both of the
  API server's roots in either mode ([ADR-0030](./0030-api-core-mcp-thin-adapter.md)). The probes
  and the metrics endpoint are on their own plain-HTTP listener
  ([ADR-0051](../engineering/0051-environment-contract.md)). The UI's session cookie and the
  consent attempt's cookie are `Secure`, `HttpOnly` and `SameSite=Strict`, and the request token is
  the same, in either mode ([ADR-0061](./0061-ui-browser-security-posture.md),
  [ADR-0111](./0111-a-consent-attempt-travels-in-a-cookie-the-ui-server-seals.md)).

## Alternatives considered

- **TLS required at the app, with plain HTTP only on an explicit declaration.** Each deployable
  requires a key pair, and serves plain HTTP only when its configuration declares that something in
  front of it terminates TLS, or only from a build made for local development. The case for it is
  that a deployable never serves plain HTTP undeclared, and that a setting weakening the transport
  is not an ordinary option. It loses because the declaration and the development build are each
  an opinion about the platform, which [ADR-0051](../engineering/0051-environment-contract.md)
  says the app does not hold.
- **The UI's cookies without `Secure` when it serves plain HTTP.** The case for it is that a
  browser then keeps the cookies over plain HTTP at any address, so the UI writes there too. It
  loses because it weakens the cookies in the common deployment, plain HTTP behind a platform that
  terminates TLS.
- **The UI choosing `Secure` from the `X-Forwarded-Proto` request header.** The case for it is that
  the cookies would follow the scheme the browser used. It loses because it trusts a header any
  client can send, and gives the app knowledge of the platform in front of it.

## Consequences

- A deployment that mounts neither file serves plain HTTP. A deployment that meant to serve TLS and
  mounted neither file is not refused, and the bearer token then crosses the network in clear. The
  start log's transport record and the effective configuration show which mode was taken.
- The `Secure` cookie attribute describes the browser's connection, not the UI's listener. Behind a
  platform that terminates TLS the browser connects over HTTPS, so the cookies hold. MDN's
  reference for `Set-Cookie` states that a site served over plain `http:` cannot set a `Secure`
  cookie, and that the `https:` requirement is ignored when the cookie is set by localhost
  ([MDN, `Set-Cookie`, `Secure`](https://developer.mozilla.org/en-US/docs/Web/HTTP/Reference/Headers/Set-Cookie#secure)).
  MDN's page on secure contexts counts `localhost` and the loopback addresses as potentially
  trustworthy origins
  ([MDN, Secure contexts](https://developer.mozilla.org/en-US/docs/Web/Security/Secure_Contexts)).
  So a local run over plain HTTP at `localhost` keeps the cookies. At any other address over plain
  HTTP end to end, the browser sets no `Secure` cookie, so every state-changing request is refused
  as a stale page before any write, and the UI only reads.
- Assumptions about other components: the platform either terminates TLS in front of the app or
  mounts both files, and the Helm chart
  ([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)) declares the TLS material
  of each deployable as an optional input.
- Each rule above that a test can break is a control, with its violation injection in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
