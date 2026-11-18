// Package core holds the decisions the worker's gate-and-scan loop makes the same way in both job
// kinds, as values (ADR-0040, ADR-0117). Counts counts what the loop decided of the messages waiting
// for a scan, as backfill's second pass and a delta sync tick both record it (ADR-0016).
package core

import "github.com/ppat/mediated-mailbox-mcp/core/index"

// Counts count the waiting messages a job kind read. Decided counts the messages the gate decided,
// Scanned those whose scan verdict was recorded, Skipped those recorded as skipped, Refused those
// whose body was refused for its size or by its conversion (ADR-0132), and Pending those left waiting
// for a scan, because the gate could not decide or their body could not be fetched.
type Counts struct {
	Decided, Scanned, Skipped, Refused, Pending int
}

// Add returns c with each of outcomes counted.
func (c Counts) Add(outcomes []index.Outcome) Counts {
	for _, o := range outcomes {
		if o.Verdict.Decided() {
			c.Decided++
		}
		switch {
		case o.Scanned != nil:
			c.Scanned++
		case o.Refused:
			c.Refused++
		case o.Verdict.Decided() && !o.Verdict.Scans():
			c.Skipped++
		default:
			c.Pending++
		}
	}
	return c
}
