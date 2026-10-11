//go:build integration

package api_test

import (
	"errors"
	"testing"

	"github.com/jackc/pgx/v5/pgconn"
)

// The database's check on accounts.account_id gives every case of the shared table of identifier
// cases the table's verdict, refusing by the grammar's check alone, and stores every proposal, so
// the database never refuses an identifier the server accepted (ADR-0128, VERIFICATIONS, the
// identifier grammar row and the identifier agreement row). Each insert runs in a transaction of its own
// that is rolled back, so no case sees another.
func TestTheDatabaseKeepsToTheIdentifierTable(t *testing.T) {
	r := newRig(t)
	store := func(id string) (bool, error) {
		tx, err := r.db.Begin(t.Context())
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			if err := tx.Rollback(t.Context()); err != nil {
				t.Fatal(err)
			}
		}()
		_, err = tx.Exec(t.Context(), "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", id)
		var pgErr *pgconn.PgError
		switch {
		case err == nil:
			return true, nil
		case errors.As(err, &pgErr) && pgErr.Code == "23514" && pgErr.ConstraintName == "accounts_account_id_grammar":
			return false, nil
		default:
			return false, err
		}
	}
	for _, c := range identifierCases {
		got, err := store(c.Identifier)
		if err != nil {
			t.Errorf("%q: storing it failed other than by the grammar's check: %v", c.Identifier, err)
		} else if got != c.Database {
			t.Errorf("%q: the database stores it %v, the table says %v", c.Identifier, got, c.Database)
		}
		if got, err := store(c.Proposal); !got || err != nil {
			t.Errorf("%q: the proposal %q is not stored: %v", c.Identifier, c.Proposal, err)
		}
	}
}
