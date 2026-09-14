---
paths:
  - "ui/browser/**"
---

# Rules for the browser app

You are touching the UI's TypeScript half. Its design is [docs/UI.md](../../docs/UI.md), and the
conventions it shares with every component are
[CLAUDE.md](../../CLAUDE.md#code-layout-and-conventions). These are the tripwires:

- **The browser's bans hold everywhere in it**, tests and fixtures included, and each keeps its
  violation file beside the code the ban covers (ADR-0063, ADR-0064, ADR-0072). `go tool banproof
  -browser` stays green.
- **A ban is never suppressed or switched off in configuration**, apart from the sanctioned `value`
  read ADR-0063 names. An ordinary oxlint rule is suppressed only by a directive naming it with its
  reason.
- **Generated files under `src/generated/` and the contract document are never edited by hand**
  (ADR-0065). The type generation step keeps its own package in `codegen/`, pinned to the TypeScript
  version `openapi-typescript` supports, and nothing else uses that copy.
- **No property-based tests, no mocks, and no second JavaScript runtime in the test path**
  (ADR-0064).
- **Every direct dependency has its roster line** (ADR-0063).
- **Nothing here depends on `ui/design/`.**
