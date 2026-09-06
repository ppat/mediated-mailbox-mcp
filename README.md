# mediated-mailbox-mcp

A self-hosted mediation layer between an AI agent and a complete mailbox. The agent gets full
organizational visibility — every thread, sender, subject, label, and timestamp — and does real
whole-mailbox work: triage, analysis, and operator-approved reorganization. What it never gets is
the body content of messages from a defined set of sensitive senders, or the MFA codes and login
links that grant account access. The mediator exposes an API — with MCP as a thin protocol
adapter over it — and enforces that line in code, at the last hop before any client, because no
mailbox credential can be scoped to express it.

**Status: design phase.** No code exists yet; the design, its decisions, the delivery plan, and
the verification catalogue are authored and are what this repository currently contains.

## Where everything lives

| Document | What it holds |
| --- | --- |
| [USE_CASES.md](./USE_CASES.md) | The outcomes the system exists to deliver, each with a falsifiable acceptance criterion |
| [DESIGN.md](./DESIGN.md) | The pillars and invariants — and the Glossary, the single home for vocabulary |
| [ROADMAP.md](./ROADMAP.md) | All the work: delivery posture, value path, work units, dependencies, open decisions |
| [docs/adr/README.md](./docs/adr/README.md) | The decision records: every reversible decision with its context, alternatives, and consequences |
| [docs/VERIFICATIONS.md](./docs/VERIFICATIONS.md) | Every control's proving injection — the violation that must fire, and what firing proves |
