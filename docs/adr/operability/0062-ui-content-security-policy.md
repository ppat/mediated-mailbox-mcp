# 0062. The UI serves under a content security policy that allows one origin and no inline script

**Status:** Accepted ·
**Serves:** [O4](../../../USE_CASES.md#o4--the-operator-can-see-and-steer)

## Context

The UI renders text an adversary wrote, the subjects, display names, labels, and reasons that
are masked at rest and still hostile. [ADR-0056](./0056-ui-organized-around-the-operators-work.md)
rules that such text renders as text and never as markup. A content security policy is the
second line behind that rule. If a rendering path ever interprets markup, the policy stops the
markup from running script or reaching another origin. The operator delegated the UI's design
to the designing session on 2026-09-09, and this is the policy that session chose.

## Decision

- **The policy is** `default-src 'self'`, `script-src 'self'`, `style-src 'self'`,
  `img-src 'self' data:`, `font-src 'self'`, `connect-src 'self'`, `frame-ancestors 'none'`.
- **No inline script, no external origin at runtime, and fonts self-hosted.** The mockups'
  Google Fonts link is a design-time convenience and not product.

## Alternatives considered

- **A policy allowing a font host.** Its case was keeping the mockups' font link. Rejected
  because every allowed origin is an origin a leaked subject could reach, and a font is two
  files to ship.
- **No content security policy, relying on the rendering rule alone.** No case was tabled.

## Consequences

- The browser toolchain's development server serves no policy header and injects an inline
  script this policy would block, so the policy is exercised only against the built output the Go
  handler serves. Its proof is split into permanent Go tests and one browser drill, as
  [ADR-0064](../engineering/0064-browser-tests-run-under-bun-against-a-dom-shim.md) records.
- The policy is a control. Its violation injection is catalogued in
  [docs/VERIFICATIONS.md](../../VERIFICATIONS.md).
- Assumptions about other components: the browser bundle has one origin, the UI's own, which
  [ADR-0042](../engineering/0042-implementation-stack.md)'s static-files shape gives it.
