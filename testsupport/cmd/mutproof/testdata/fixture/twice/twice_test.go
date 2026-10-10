package twice_test

import (
	"testing"

	"fixture/twice"
)

func TestAdmitRefusesFlagged(t *testing.T) {
	if twice.Admit(true) {
		t.Error("Admit let a flagged item in")
	}
}
