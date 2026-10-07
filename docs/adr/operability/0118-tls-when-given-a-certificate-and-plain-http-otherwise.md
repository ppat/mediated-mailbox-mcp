# 0118. The mediator and the UI serve TLS when given a certificate and key, and plain HTTP when given neither

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [O6](../../../USE_CASES.md#o6--deployable)

## Context

The mediator's client surface and the UI each listen for HTTP requests. Where TLS ends is a fact
about the platform. Some platforms terminate TLS in front of the app, at an ingress or a gateway.
Others hand the app a certificate. [ADR-0030](./0030-api-core-mcp-thin-adapter.md) already allows
TLS at the listener or at an ingress in front of it, and
[ADR-0084](../mutation/0084-ui-writes-decisions-and-account-setup.md) gives the UI the same posture.

Before this record, each deployable held an opinion about which kind of platform it ran on. The
mediator served TLS unless a configuration key declared that an ingress in front terminated it. The
UI refused plain HTTP outside a binary built for the dev loop. The app does not know its platform
([ADR-0051](../engineering/0051-environment-contract.md)), so neither opinion is the app's to hold.

## Decision

- **Each of the two deployables serves TLS when its configuration names a certificate and a key,
  and plain HTTP when it names neither.** The keys are `tls_cert` and `tls_key`, paths of mounted
  files ([ADR-0079](./0079-secrets-arrive-as-mounted-files.md)), with no default, and the files are
  read again on each handshake, so a renewed certificate is served without a restart. No other key
  says which mode applies, and the app serves what it is given without knowing which kind of
  platform it is on.
- **A configuration naming one of the two files and not the other is refused at start.** A
  half-mounted Secret going plain without a word is the failure this refusal prevents.
- **Every request still reaches the same checks in either mode.** The mediator's bearer check runs
  before both roots ([ADR-0030](./0030-api-core-mcp-thin-adapter.md)), and the UI's session and
  its request token ([ADR-0061](./0061-ui-browser-security-posture.md),
  [ADR-0111](./0111-a-consent-attempt-travels-in-a-cookie-the-ui-server-seals.md)) work as before.
  `HttpOnly`, `SameSite=Strict`, the session binding, the request token check and the attempt
  cookie's sealing hold in both modes.
- **The UI's cookies carry `Secure` exactly when the UI serves TLS.** The mode has one source, both
  TLS files named in the configuration, and the same value decides the listener and the cookies.
  Behind a platform that terminates TLS, the browser's hop is still HTTPS, so a cookie without
  `Secure` is stored and sent as before. Over a plain browser hop the UI works, writes included.

## Alternatives considered

- **TLS required, as the UI had it.** Its case was that the app guaranteed an encrypted transport
  whatever platform ran it. Rejected because it makes a platform that terminates TLS in front of the
  app supply a second certificate the app does not need, which is an opinion about the platform
  ([ADR-0051](../engineering/0051-environment-contract.md)).
- **A key declaring that something in front terminates TLS, as the mediator had it.** Its case was
  that plain HTTP happens only when the operator says so, never because material is missing.
  Rejected because the key states a fact about the platform. Part of its protection holds without
  it. The mode follows the configured keys and not the presence of files, so a missing mount behind
  a configured path fails the start or readiness rather than going plain, and naming one file alone
  is refused. The part given up is accepted. A deployment that names neither file by mistake serves
  plain HTTP without complaint, because the app cannot tell a platform that terminates TLS in front
  of it from a forgotten input.
- **A key switching TLS on or off, beside the two paths.** Its case was a mode stated apart from the
  paths. Rejected because the two paths already say it, so a second key is a second source for one
  fact, and every pairing where the two disagree would need its own refusal.
- **`Secure` on the UI's cookies always, which forces an HTTPS browser hop on the deployment.** Its
  case was that the cookie never travels in clear. Rejected because a browser stores no `Secure`
  cookie over a plain hop, so the choice would make an HTTPS browser hop a requirement on the
  platform, which is an opinion about the platform
  ([ADR-0051](../engineering/0051-environment-contract.md)).

## Consequences

- The mediator's `tls_at_ingress` key and the UI's `insecure_http` key are gone, and the build tag
  that admitted the UI's plain HTTP to the dev loop alone guards nothing and is gone with them. The
  dev loop runs the UI with neither file named.
- Serving plain HTTP is not a safety disposition under
  [ADR-0051](../engineering/0051-environment-contract.md), because the gate, the bearer check and
  the request token are unchanged in either mode.
- In plain HTTP the bearer token travels in clear between whatever terminates TLS and the app. The
  hop is the platform's to protect, as the network the app runs in is
  ([DESIGN.md](../../../DESIGN.md#network-position-never-substitutes-for-the-gate)).
- In plain mode a cookie can travel in clear wherever the browser's own hop is plain. That cost is
  accepted, since the browser's hop is the platform's, as the hop in front of the app is. The
  session and the request token still bind every write to a page the UI served.
- The mediator reports ready only once the key pair loads when one is configured, and once the
  bearer token loads in either mode ([ADR-0051](../engineering/0051-environment-contract.md)).
- The chart declares the TLS material as an optional input for the mediator and the UI
  ([ADR-0052](../engineering/0052-kubernetes-deployment-helm-chart.md)), and renders both keys only
  when the input is given.
- Assumptions about other components: the platform either hands each deployable a certificate and
  key as mounted files and keeps them current, or terminates TLS in front of it and forwards plain
  HTTP over a hop it protects.
