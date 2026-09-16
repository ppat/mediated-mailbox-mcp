# mediated-mailbox-mcp

A self-hosted mediation layer between an AI agent and a complete mailbox. The agent gets full
organizational visibility — every thread, sender, subject, label, and timestamp — and does real
whole-mailbox work: triage, analysis, and operator-approved reorganization. What it never gets is
the body content of messages from a defined set of sensitive senders, or the MFA codes and login
links that grant account access. The mediator exposes an API — with MCP as a thin protocol
adapter over it — and enforces that line in code, at the last hop before any client, because no
mailbox credential can be scoped to express it.

**Status: design phase.** No code exists yet; the design, its decisions, the delivery plan, the
testing strategy, the verification catalogue, and the mutation ledger are authored and are what
this repository currently contains.

## Where everything lives

| Document | What it holds |
| --- | --- |
| [USE_CASES.md](./USE_CASES.md) | The outcomes the system exists to deliver, each with a falsifiable acceptance criterion |
| [DESIGN.md](./DESIGN.md) | The pillars and invariants — and the Glossary, the single home for vocabulary |
| [docs/UI.md](./docs/UI.md) | The UI's design: how it is organized, its screens, palettes, framework requirements, and build guidance |
| [CLAUDE.md](./CLAUDE.md) | Orientation for agents, and the code's layout and conventions. The data-access library and the narrow shared libraries each describe themselves in the README in their own directory |
| [ROADMAP.md](./ROADMAP.md) | All the work: delivery posture, value path, work units with their finish lines, production points, dependencies, open decisions |
| [docs/adr/README.md](./docs/adr/README.md) | The decision records: every reversible decision with its context, alternatives, and consequences |
| [TESTING.md](./TESTING.md) | What tests a piece of work must have and what proves it done, linking the records that decide it |
| [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) | Every control's proving injection — the violation that must fire, and what firing proves |
| [docs/MUTATIONS.md](./docs/MUTATIONS.md) | The mutation ledger. Per-control proof that tests go red when the mechanism is removed |
