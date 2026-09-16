//go:build banproof

package jmap_test

import (
	"os"
	"testing"

	co "github.com/google/go-cmp/cmp/cmpopts"
	_ "github.com/jackc/pgx/v5" //nolint:DEPGUARD // reason // want suppression `names "depguard"`
)

// This file silences findings of linters standing in for controls, in a package that carries no control of
// its own, with every spelling of the directive that golangci-lint honours (ADR-0071). Each silenced finding
// goes unreported, so each line wants only the search's finding on the directive.
func TestDirectives(t *testing.T) {
	p := t.TempDir()
	_ = co.IgnoreUnexported(struct{}{}) //nolint // want suppression "silences every linter"
	_ = os.Remove(p)                    /* want suppression "silences every linter" */ // nolint
	_ = os.Remove(p)                    //nolint reason words // want suppression "silences every linter"
	_ = os.Remove(p)                    //nolint:all // want suppression "silences every linter"
	_ = os.Remove(p)                    //nolint:allow-this // want suppression "silences every linter"
	_ = os.Remove(p)                    //nolint:gosec,all // reason // want suppression "silences every linter"
	_ = os.Remove(p)                    //nolint:gosec, errcheck // reason // want suppression `names "errcheck"`
	_ = os.Remove(p)                    ////nolint:errcheck // reason // want suppression `names "errcheck"`
	//nolint:forbidigo // reason // want suppression `names "forbidigo"`
	_ = co.IgnoreUnexported(struct{}{})
	remove(p)
}

// remove carries the directive in its doc comment, which covers the whole function.
//
//nolint:errcheck // reason // want suppression `names "errcheck"`
func remove(p string) {
	_ = os.Remove(p)
}
