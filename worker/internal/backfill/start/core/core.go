// Package core holds the decision of backfill's run-start step, as a value (ADR-0040). Overturned
// decides which stored gate skips the step returns to pending, those the gate no longer decides as the
// same skip (ADR-0121). The subjects the step masks again from the store are masked as the first pass
// masks them, by worker/internal/backfill/pass1/core's Remasked (ADR-0120).
package core

import (
	"github.com/ppat/mediated-mailbox-mcp/core/classify"
	"github.com/ppat/mediated-mailbox-mcp/core/index"
	"github.com/ppat/mediated-mailbox-mcp/core/mail"
	"github.com/ppat/mediated-mailbox-mcp/core/scangate"
)

// Overturned returns the identifiers of the stored gate skips among skips that the gate, deciding each
// again, no longer decides as the same skip, in the order given (ADR-0121). It decides as Gate does,
// with the sender's class under the account policy p, the thresholds c and the message's age measured
// at now. A skip stands only while the gate decides it again as high_volume_no_hits, so a message the
// gate would now scan, one under thresholds that cannot decide and one whose sender p now restricts
// are all overturned.
func Overturned(p classify.Policy, c scangate.Config, now mail.UnixMilli, skips []index.Waiting) []string {
	var out []string
	for _, m := range skips {
		if index.Gate(p, c, now, m, 0).Reason() != scangate.HighVolumeNoHits {
			out = append(out, m.ID)
		}
	}
	return out
}
