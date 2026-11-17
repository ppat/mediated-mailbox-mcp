//go:build banproof

package jmap_test

import (
	"os"
	"testing"

	co "github.com/google/go-cmp/cmp/cmpopts"
	_ "github.com/jackc/pgx/v5/pgconn" //nolint:gosec // reason // want depguard "import 'github.com/jackc/pgx/v5/pgconn' is not allowed from list 'provider'"
)

// This file names only ordinary linters in suppression directives on violations of controls. The search
// allows every directive that gives a reason, and golangci-lint still reports every control finding, so a
// directive naming only ordinary linters cannot reach a control (ADR-0071). The last directive gives no
// reason, which the search refuses.
func TestOrdinaryDirectives(t *testing.T) {
	p := t.TempDir()
	_ = os.Remove(p)                    //nolint:gosec // reason // want errcheck "Error return value of `os.Remove` is not checked"
	_ = co.IgnoreUnexported(struct{}{}) //nolint:staticcheck, revive // reason // want forbidigo `use of .co\.IgnoreUnexported. forbidden`
	//nolint:gosec // reason
	if p != "" {
		_ = co.IgnoreUnexported(struct{}{}) // want forbidigo `use of .co\.IgnoreUnexported. forbidden`
	}
	_ = os.Remove(p) /* want errcheck "Error return value" suppression "gives no reason" */ //nolint:gosec
}
