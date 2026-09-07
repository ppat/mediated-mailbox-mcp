---
paths:
  - "USE_CASES.md"
---

# Rules for USE_CASES.md

You are touching the outcome contract — the near-frozen document. It changes only when the
understanding of what the system is *for* changes, and only with the operator's explicit
agreement.

- **Every outcome is falsifiable.** The section shape is: a bolded claim, then a *"Falsified by
  any of:"* list, then scope notes where a criterion would otherwise be misread. A criterion that
  cannot fail is not one — for each bullet, ask what concrete observation would make it fire.
- **Scope notes pre-empt misreadings** — including the "this looks like failure but is success"
  kind (a deliberately exempt case reporting nothing is the outcome working, not failing).
- **Deliberate absences go in Non-outcomes**, so a later reader does not mistake them for gaps
  and "fix" them. Things that are neither targeted nor deliberately excluded (maybe-someday
  ideas) go nowhere at all.
- **This document cites no individual decision records.** Traceability runs the other way,
  through each record's `Serves:` header. Outcome identifiers (letter = axis) are defined here
  and used as the coordinate system everywhere else — adding an outcome means adding its
  axis-table row and keeping the identifier scheme coherent.
- Mechanisms live in the design and the records; schedule lives in the roadmap; test plans live
  in the verification catalogue. This document holds only outcomes and what would falsify them.
