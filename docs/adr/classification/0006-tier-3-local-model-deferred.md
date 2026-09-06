# 0006. Tier 3 is a small local model — in scope, deferred until real labeled data exists

**Status:** Accepted ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

After Tiers 1–2 ([ADR-0005](./0005-tiered-detection.md)), a few percent of scanned messages score
ambiguously. A third tier could settle them — but on day one there is no labeled data from the
operator's actual mail to train it on, and a detector trained on synthetic examples would be worse
than no detector: it would add confident wrong answers to exactly the cases the pattern tiers
already found hard.

## Decision

**Tier 3 is in scope and deliberately deferred.**

- The shape: a small local model — a compact sentence-embedding model of the
  `all-MiniLM-L6-v2` class (22M parameters, CPU-viable, ~5ms/message) or a small ONNX
  classifier — fine-tuned on the operator's own confirmed positives.
- It runs inside the scanner workload: no external calls, no data leaving the cluster.
- The training set accumulates from production: every Tier 1–2 hit becomes a labeled positive,
  every gate-passing miss a weak negative, corrected through the UI's masking-events view. Training
  starts once a few hundred confirmed examples exist.
- When it ships, it ships behind a scanner-version flag so rollback is trivial and its effect is
  measurable against the tiers it augments.

## Alternatives considered

- **Train on synthetic examples now.** Rejected: synthetic OTP mail teaches the model the
  generator's idioms, not the operator's traffic, and its errors would land precisely on the
  ambiguous cases Tier 3 exists for.
- **Skip Tier 3 entirely.** Rejected: Tier 3 is in scope, deferred — trained once a few hundred
  confirmed examples exist.
- **A hosted inference API for the ambiguous tail.** Rejected outright: body content to an
  external endpoint is the exposure the system exists to prevent
  ([ADR-0005](./0005-tiered-detection.md)).

## Consequences

- Day-one detection quality rests entirely on Tiers 1–2
  ([ADR-0003](../redaction/0003-subject-masking.md),
  [ADR-0007](../redaction/0007-composite-scan-gate.md)).
- The masking-events review loop is not optional hygiene — it is the labeling pipeline this
  decision depends on.
