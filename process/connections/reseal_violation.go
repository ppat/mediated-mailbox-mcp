//go:build banproof

package connections

import "context"

// This file re-seals the stored credentials from outside delta sync's package, on purpose, and
// banproof requires the findings below from go vet. Only delta sync re-seals (ADR-0071, ADR-0092).
func resealsFromHere(ctx context.Context, c *Connections) Scan {
	reseal := c.Reseal // want vetcheck "uses the connections' Reseal outside delta sync's package"
	_ = reseal
	return c.Reseal(ctx) // want vetcheck "uses the connections' Reseal outside delta sync's package"
}
