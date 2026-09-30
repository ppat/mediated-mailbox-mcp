# 0094. The scan gate evaluates every message, and no gate decision is memoized

**Status:** Accepted (supersedes [ADR-0007](./0007-composite-scan-gate.md)) ·
**Pillar:** [Fail closed, everywhere](../../../DESIGN.md#fail-closed-everywhere) ·
**Serves:** [C3](../../../USE_CASES.md#c3--content-based-secrets-caught)

## Context

The Scan Gate's predicate ([ADR-0093](./0093-composite-scan-gate.md)) reads two kinds of input. From
the sender it reads the class under the policy in force, the sender's volume and whether the sender
has a prior scan hit. From the message it reads whether pass 1 masked the subject, whether the
message carries its own `List-Id`, the sender's local part, and the message's size and age. The
predicate itself is a pure function over those values
([ADR-0040](../engineering/0040-pure-core-decisions-as-values.md)).

A memo would replay a decision already made for one message onto another message of the same
sender, so the predicate's cost amortizes across the sender's traffic. What it would save is the
work of gathering the sender's inputs, since evaluating the predicate over values already held costs
a few comparisons. Backfill's pass 2 reads each page of messages waiting for a scan together with
each message's sender volume and prior hits, so that work is done once per page whatever the gate
does.

## Decision

- **The gate evaluates every message it decides, from that message's own inputs and its sender's
  current ones.** No gate decision is memoized, and nothing replays one message's decision onto
  another.

The deciding argument. The page read already carries every per-sender input, so a memo saves
nothing measurable, and it is the only path by which a skip decided for one message could be applied
to a message the predicate would scan, or kept after the sender's first scan hit.

## Alternatives considered

- **A memo keyed on the sender plus the subject's shape, the shape being whether pass 1 masked the
  subject.** Its case is the amortization above. Rejected, because the predicate also reads each
  message's own `List-Id`, size and age, so a memo keyed on those two alone replays a
  `high_volume_no_hits` skip onto a message of the same sender that carries no `List-Id` and that
  the predicate would scan. A sender's prior hits also grow as bodies are scanned, so a memoized
  skip goes stale at the sender's first hit unless the hit clears it.
- **A memo whose key carries every per-message input**, the sender's address, whether the subject
  was masked, whether the message carries a `List-Id` and whether it is small and recent, cleared
  for a sender's domain on every scan hit and held in memory for one run. Its case is that a hit
  then returns exactly what the predicate would. Rejected, because it saves only the comparisons the
  predicate makes, and it stays correct only while its key holds every input the predicate reads, a
  condition a later change to the predicate can break without anything failing.
- **A subject shape as a normalized template of the subject text.** Its case is grouping a sender's
  messages by what they say. Rejected, because the predicate never reads the subject text, only
  whether pass 1 masked it, and the text of a masked subject no longer holds what was masked.

## Consequences

- A gate decision depends only on the inputs it was evaluated from, so the reason recorded with it
  ([ADR-0093](./0093-composite-scan-gate.md)) is always the reason the predicate gives for that
  message.
- A sender's first scan hit reaches the next message of that sender the gate evaluates, since the
  prior-hit input is read as it stands when that message is evaluated.
- Assumptions about other components: whatever evaluates the gate gathers each sender's inputs in a
  read that does not grow with the number of messages per sender, as pass 2's page read does. A
  workload whose gathering did grow that way would re-argue this record.
