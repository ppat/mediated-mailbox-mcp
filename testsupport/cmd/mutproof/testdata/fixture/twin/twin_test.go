package twin_test

import (
	"testing"

	"fixture/twin"
)

func TestRefusesFlagged(t *testing.T) {
	if !twin.Refuse(true) {
		t.Fatal("a flagged item passed")
	}
}
