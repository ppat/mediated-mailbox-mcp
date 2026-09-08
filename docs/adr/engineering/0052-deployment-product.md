# 0052. The repository ships a self-contained deployment artifact that assumes nothing

**Status:** Accepted ·
**Pillar:** [Concerns stay un-braided; components know only their contracts](../../../DESIGN.md#concerns-stay-un-braided-components-know-only-their-contracts) ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

[ADR-0038](../operability/0038-credentials-as-mounted-files.md) rules that something outside the
mediator delivers the credential files, and
[ADR-0051](./0051-environment-contract.md) generalizes the boundary: the app knows its
environment contract, never its platform. The deployment artifact is where that boundary either
becomes structural or erodes — an artifact wired to one platform's machinery would know exactly
what the records refuse to know.

## Decision

- **This repository ships one deployment artifact: a chart that stands up all the workloads —
  and only the workloads.** Platform services are inputs, not contents: the database, the
  credential material, and the live policy configuration arrive from outside.
- **The chart assumes nothing about the cluster it lands on.** Its inputs are values and
  pre-existing configuration and secret objects the user supplies; it assumes no secret-sync,
  certificate, or admission machinery exists. The chart therefore structurally cannot know
  about the secret machinery — [ADR-0038](../operability/0038-credentials-as-mounted-files.md)'s
  boundary made concrete.
- **Project-owned policy objects ship in the chart; their enforcement is the platform's.**
  The egress restriction ([ADR-0014](../operability/0014-lan-only-transport.md)) ships as a
  standard policy object any cluster accepts; whether it is enforced depends on the platform's
  network fabric. Being deployed somewhere does not mean the controls hold there — the
  verification catalogue, not the chart, carries the proof obligations.
- **Distribution: an artifact in the same registry as the images, signed and verified under
  the same machinery ([ADR-0028](../operability/0028-trust-anchor-hardening.md)), at the
  lockstep version ([ADR-0049](./0049-image-per-component-lockstep.md)) — the chart's version
  is the app's version.**
- **The chart's tests travel with the product.** The chart carries its own standard test
  resources, inert at install; and this repository carries a system-level suite that deploys
  the chart on a disposable cluster and exercises the assembled system — referencing nothing of
  the operator's own platform repositories. Optional integration conveniences (metrics
  scrape objects, dashboards) ship flag-gated and off by default: enabling a flag is the user
  asserting their platform has the machinery, so the no-assumptions rule holds.
- **Consumption is the platform side's business.** Whoever deploys the system wires their own
  platform machinery around the chart's inputs and runs their own acceptance, exactly as they
  would for any third-party chart.

## Alternatives considered

- **Infrastructure as a module in the operator's platform repositories** — the form factor the
  operator's other projects use (code in the project repo, deployment wiring in the platform's).
  The case for it: it is the established pattern, and the wiring lives next to the machinery it
  uses. Rejected for this project deliberately, as a form-factor experiment the operator may
  later extend to other projects: a self-contained chart is testable in isolation on a
  disposable cluster, and its no-assumptions boundary is exactly what an external consumer
  would need — which serves the eventual open-sourcing intention for free, without planning
  for it.
- **A chart wired to the operator's platform** (assuming the secret-sync and certificate
  machinery that actually runs there). The case for it: less input-wiring for the one
  deployment that exists today. Rejected: it braids the product with one platform, and it would
  let the chart know precisely what
  [ADR-0038](../operability/0038-credentials-as-mounted-files.md) says is no one's business.
- **Per-component charts.** No case was tabled for them; one chart matches the lockstep rule,
  and independent charts would recreate the compatibility question
  [ADR-0049](./0049-image-per-component-lockstep.md) closes.
- **A hosted chart repository with its own index.** No case was tabled for it. Rejected:
  distribution rides the same registry and signing machinery as the images, so a separate
  hosting surface would add an artifact channel for nothing.

## Consequences

- The disposable-cluster test layer cannot prove policy *enforcement* — its network fabric does
  not enforce the shipped policy objects. That proof lives with the platform (the parked
  disposition in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) records where), and the chart's
  tests prove the system's own behavior, not the platform's.
- The chart grows with the system: deployment manifests land with the unit they deploy, per
  [ROADMAP.md](../../../ROADMAP.md)'s operational rule.
- Assumptions about other components: the consuming platform supplies the database, the secret
  material, the live policy data, and the enforcement machinery for the shipped policy
  objects; the app behind the chart honors [ADR-0051](./0051-environment-contract.md)'s
  contract, which is what makes the chart's inputs sufficient.
