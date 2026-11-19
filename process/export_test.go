package process

import (
	"testing"
	"time"
)

// SetWedgeBound shortens how long past due a job's loop may go before the health probe fails, for
// the test t, so a test sees the probe fail without waiting minutes.
func SetWedgeBound(t testing.TB, d time.Duration) {
	old := wedgeBound
	wedgeBound = d
	t.Cleanup(func() { wedgeBound = old })
}
