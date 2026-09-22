// Package property stubs the functions of testsupport/property the analyser recognises.
package property

import (
	"testing"

	"pgregory.net/rapid"
)

func Check[A any](t *testing.T, draw func(*rapid.T) A, prop func(rapid.TB, A)) {}

func Report[A any](t *testing.T, draw func(*rapid.T) A, classify func(A) string, minimums map[string]float64) {}
