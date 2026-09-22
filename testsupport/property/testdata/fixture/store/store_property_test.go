// Package store_test is the property the failing-case store's tests run. N at or above 500 is the
// planted fault. FIXTURE_VARIANT=extradraw adds a draw ahead of N, which is an edit to the draw
// function, FIXTURE_VARIANT=reshaped changes the arguments' shape, and FIXTURE_VARIANT=unrelated
// draws only passing cases and then fails the test for another reason.
package store_test

import (
	"os"
	"testing"

	"pgregory.net/rapid"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/property"
)

type args struct{ N int }

type wider struct {
	N     int
	Label string
}

func TestPlantedFault(t *testing.T) {
	if os.Getenv("FIXTURE_VARIANT") == "unrelated" {
		property.Check(t, func(t *rapid.T) args {
			return args{N: rapid.IntRange(0, 499).Draw(t, "n")}
		}, func(t rapid.TB, a args) {
			if a.N >= 500 {
				t.Fatalf("N %d reaches the planted fault", a.N)
			}
		})
		t.Error("a failure unrelated to the property")
		return
	}
	if os.Getenv("FIXTURE_VARIANT") == "reshaped" {
		property.Check(t, func(t *rapid.T) wider {
			return wider{N: rapid.IntRange(0, 1000).Draw(t, "n"), Label: rapid.String().Draw(t, "label")}
		}, func(t rapid.TB, a wider) {
			if a.N >= 500 {
				t.Fatalf("N %d reaches the planted fault", a.N)
			}
		})
		return
	}
	property.Check(t, func(t *rapid.T) args {
		if os.Getenv("FIXTURE_VARIANT") == "extradraw" {
			rapid.Int().Draw(t, "extra")
		}
		return args{N: rapid.IntRange(0, 1000).Draw(t, "n")}
	}, func(t rapid.TB, a args) {
		if a.N >= 500 {
			t.Fatalf("N %d reaches the planted fault", a.N)
		}
	})
}
