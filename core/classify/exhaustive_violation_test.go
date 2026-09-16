//go:build banproof

package classify_test

import "testing"

type verdict int

const (
	allow verdict = iota
	deny
)

// This file leaves a verdict unhandled in a switch that carries the deny-defaulting default branch,
// on purpose.
func decide(v verdict) bool {
	switch v { // want exhaustive "missing cases in switch of type classify_test.verdict: classify_test.allow"
	case deny:
		return false
	default:
		return false
	}
}

func TestDecide(t *testing.T) { _ = decide(deny) }
