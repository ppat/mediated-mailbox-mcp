package core_test

import (
	"errors"
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/db/tx/core"
)

// The account setting must read back as the account the transaction set, and an empty account never
// passes, whatever it read back as (ADR-0047).
func TestAccountHeld(t *testing.T) {
	for _, c := range []struct {
		name, account, readBack string
		want                    error
	}{
		{"the account read back", "acct-a", "acct-a", nil},
		{"another account read back", "acct-a", "acct-b", core.ErrAccountNotSet},
		{"nothing read back", "acct-a", "", core.ErrAccountNotSet},
		{"an empty account read back empty", "", "", core.ErrAccountNotSet},
		{"an empty account read back as another", "", "acct-b", core.ErrAccountNotSet},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := core.AccountHeld(c.account, c.readBack); !errors.Is(got, c.want) || (c.want == nil) != (got == nil) {
				t.Errorf("core.AccountHeld(%q, %q) = %v, want %v", c.account, c.readBack, got, c.want)
			}
		})
	}
}

// The base policy's transaction holds no account, so the setting must read back empty (ADR-0112).
func TestBaseHeld(t *testing.T) {
	if err := core.BaseHeld(""); err != nil {
		t.Errorf("BaseHeld of an empty setting = %v, want nil", err)
	}
	if err := core.BaseHeld("acct-a"); !errors.Is(err, core.ErrBaseNotSet) {
		t.Errorf("BaseHeld of an account = %v, want core.ErrBaseNotSet", err)
	}
}
