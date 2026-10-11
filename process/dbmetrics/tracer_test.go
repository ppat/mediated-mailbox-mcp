package dbmetrics_test

import (
	"testing"

	"github.com/ppat/mediated-mailbox-mcp/process/dbmetrics"
)

// A statement is labelled by the name its name comment gives it, by its transaction keyword when it
// has none, and as unnamed otherwise, so no label carries the SQL's text.
func TestLabel(t *testing.T) {
	cases := []struct{ sql, want string }{
		{"-- name: Accounts :many\nSELECT account_id FROM accounts", "Accounts"},
		{"  -- name: SetTransactionAccount :exec\nSELECT set_config('app.account', $1, true)", "SetTransactionAccount"},
		{"-- name: \nSELECT 1", "unnamed"},
		{"-- name: Drop'; --\nSELECT 1", "unnamed"},
		{"begin", "begin"},
		{"BEGIN ISOLATION LEVEL REPEATABLE READ READ ONLY", "begin"},
		{"commit", "commit"},
		{"rollback", "rollback"},
		{"SELECT * FROM accounts WHERE account_id = 'personal'", "unnamed"},
		{"-- a comment that names nothing\nSELECT 1", "unnamed"},
		{"", "unnamed"},
	}
	for _, c := range cases {
		if got := dbmetrics.Label(c.sql); got != c.want {
			t.Errorf("Label(%q) = %q, want %q", c.sql, got, c.want)
		}
	}
}
