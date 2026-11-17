//go:build banproof

package jmap_test

import "testing"

type verdict int

const (
	allow verdict = iota
	deny
)

// This file silences the exhaustiveness check with exhaustive's own directive, above a switch that leaves a
// verdict unhandled and carries the deny-defaulting default branch.
func decide(v verdict) bool {
	//exhaustive:ignore // want suppression "exhaustive's own directive"
	switch v {
	case deny:
		return false
	default:
		return false
	}
}

func TestExhaustiveDirective(t *testing.T) { _ = decide(allow) }
