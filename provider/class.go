package provider

import ratecore "github.com/ppat/mediated-mailbox-mcp/provider/ratelimit/core"

// Class is the priority class a caller's leases are asked in (ADR-0025). The zero value names none.
type Class uint8

// The three classes, from the most urgent.
const (
	_ Class = iota
	// Interactive is the client surface's work, which never yields.
	Interactive
	// Sync is delta sync's work.
	Sync
	// Batch is backfill and reorg apply, which absorbs a cut first.
	Batch
)

// limiterClass returns the rate limiter's class for c, and whether c names one.
func limiterClass(c Class) (ratecore.Class, bool) {
	switch c {
	case Interactive:
		return ratecore.Interactive, true
	case Sync:
		return ratecore.Sync, true
	case Batch:
		return ratecore.Batch, true
	default:
		return 0, false
	}
}
