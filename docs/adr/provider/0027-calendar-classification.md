# 0027. Calendar sensitivity keys on any participant, not organizer-only — and Fastmail speaks CalDAV

**Status:** Accepted ·
**Pillar:** [Sensitivity is two independent axes](../../../DESIGN.md#sensitivity-is-two-independent-axes) ·
**Serves:** [C1](../../../USE_CASES.md#c1--metadata-always-visible), [C2](../../../USE_CASES.md#c2--sensitive-sender-content-never-released), [P1](../../../USE_CASES.md#p1--one-contract)

## Context

Calendar joins mail behind the same mediation layer, with the same invariant: structure always
visible, sensitive content never released. But calendar sensitivity semantics are not mail
semantics — a meeting's sensitivity is a property of the room, not of who sent the invite.

## Decision

Same layering, calendar-specific classification:

- **Always visible:** id, title, start/end, recurrence, location, attendee addresses and response
  status, organizer, calendar name, busy/free, visibility class. Titles follow the same rule as
  mail subjects — visible, because "Attorney call" is exactly the organizational signal the agent
  needs.
- **Gated:** description body, attachments, conferencing join links, private notes.

| Signal | Treatment |
| --- | --- |
| Organizer domain on the deny list | Restricted |
| **Any attendee** domain on the deny list | Restricted |
| `visibility: private` on the event | Restricted regardless of domain |
| Description contains a tokenized join link | Content-flagged — the link is a credential |

The asymmetry with mail is intentional: mail classifies on *sender*; calendar classifies on *any
participant*, because sensitivity attaches to the meeting, not the inviter.

Mutation rights mirror the mail matrix ([ADR-0019](../mutation/0019-asymmetric-mutation.md)):
restricted events may be recategorized or moved between calendars, never deleted — and never
declined on the operator's behalf.

**Providers:** Google Calendar API (separate scope, same OAuth grant as mail) and **CalDAV** for
Fastmail — not JMAP, because JMAP's calendar extension is not broadly deployed there. That makes
three calendar adapter implementations to plan for, and CalDAV's sync semantics (`sync-token`,
RFC 6578) differ enough from the mail feeds to warrant its own adapter rather than a shim.

## Alternatives considered

- **Classify on organizer only, like mail's sender.** Rejected: an unrestricted organizer inviting
  the operator's attorney produces a meeting exactly as sensitive as one the attorney organized.
  Participant-set classification captures what the event actually exposes.
- **Gate titles like bodies.** Rejected for the same reason mail subjects stay visible
  ([ADR-0001](../redaction/0001-redaction-matrix.md)): the title is the organizational handle, and
  hiding it turns restricted events into unidentifiable blobs.
- **Fastmail calendar via JMAP for symmetry with mail.** Rejected on deployment reality; symmetry
  is not worth building against an extension the provider has not shipped broadly.

## Consequences

- The canonical model grows a calendar half with its own classification inputs (participant set,
  visibility, link detection) feeding the same Redaction Gate and Mutation Authorizer.
- Join links are treated as credentials everywhere, which keeps the "link is an account-takeover
  primitive" rule uniform across mail and calendar.
