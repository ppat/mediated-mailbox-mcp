# 0044. Test data is synthetic, and fixture bodies carry designed marker text

**Status:** Accepted ·
**Serves:** [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released)

## Context

The tests need mail to run against. A committed real message is the very content this system
exists to protect, and git history keeps whatever lands in it forever. Separately, the leak
checks in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) need a deterministic way to
recognize protected content in any output surface.

## Decision

- **Every mail fixture is synthetic.** The made-up messages, senders, and bodies the tests run
  against never include real mail. The operator's real mail contributes only formats and
  patterns to reconstruct synthetically. The messages themselves never land.
- **Fixture bodies carry designed marker text.** A test can search any output surface for
  leaked content and get a deterministic answer.

## Alternatives considered

- **Real mail as fixtures, redacted or anonymized.** No case was tabled for it. Rejected
  because a committed real message is the very content this system exists to protect, and git
  history keeps it forever.

## Consequences

- Scope: the rules govern mail-shaped test data, the messages, senders, and bodies, wherever
  any kind of test uses them, for controls and ordinary tests alike.
- The whole-surface leak searches in [docs/VERIFICATIONS.md](../../VERIFICATIONS.md) are
  built on the marker technique, so this discipline is load-bearing for the catalogue's
  absence rows.
- Fixtures never pretend to carry the job of validating the canonical mapping against messy
  real data. The first backfill run keeps that job, where the roadmap placed it.
- The real MFA-format corpus is built from the operator's own mail during backfill under this
  record's rule, as formats and patterns only.
- The header names C2 because a committed real message would itself be the leak the system
  exists to prevent.
