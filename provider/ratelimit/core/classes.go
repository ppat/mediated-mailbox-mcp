package core

// Class is a priority class (ADR-0025). The zero value is no class.
type Class uint8

const (
	_ Class = iota
	// Interactive is the client surface's work, which never yields.
	Interactive
	// Sync is delta sync's work.
	Sync
	// Batch is backfill and reorg apply, which absorbs a cut first.
	Batch
)

// The reservations, as fractions of the target. Batch has the rest.
const (
	interactiveFraction = 0.30
	syncFraction        = 0.20
)

// Share returns a class's share of a budget. Interactive reserves 30% of the target and sync 20%,
// and batch has the rest. A budget below the target is cut from batch first, then from sync, then
// from interactive. A value no constant names has no share.
func Share(c Class, budget, target float64) float64 {
	interactive := min(budget, interactiveFraction*target)
	sync := min(budget-interactive, syncFraction*target)
	switch c {
	case Interactive:
		return max(interactive, 0)
	case Sync:
		return max(sync, 0)
	case Batch:
		return max(budget-interactive-sync, 0)
	default:
		return 0
	}
}
