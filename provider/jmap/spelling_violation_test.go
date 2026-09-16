//go:build banproof

package jmap_test

import (
	"os"
	"testing"

	co "github.com/google/go-cmp/cmp/cmpopts"
)

// This file carries spellings of suppression directives that golangci-lint and the control linters do not
// honour today, each on a violation of a control. The search refuses each, and the control finding is still
// reported. If a later release starts honouring a spelling, that line's control finding goes unreported and
// banproof turns red (ADR-0071).
func TestUnhonouredSpellings(t *testing.T) {
	p := t.TempDir()
	_ = os.Remove(p)                    /* want errcheck "Error return value" suppression "does not honour" */                                 //NOLINT
	_ = os.Remove(p)                    /* want errcheck "Error return value" suppression "does not honour" */                                 //Nolint:errcheck // reason
	_ = os.Remove(p)                    /* nolint */                                                                                           /* want errcheck "Error return value" suppression "does not honour" */
	_ = os.Remove(p)                    /* want errcheck "Error return value" suppression "does not honour" */                                 //	nolint
	_ = co.IgnoreUnexported(struct{}{}) /* want forbidigo `use of .co\.IgnoreUnexported. forbidden` suppression "forbidigo's own directive" */ //permit:co.IgnoreUnexported
}
