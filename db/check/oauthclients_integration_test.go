//go:build integration

package check_test

import (
	"errors"
	"strings"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// attempt runs one statement in a savepoint of tx, keeping its effect when it succeeds and undoing it
// when it fails, and returns the SQLSTATE of its failure, empty for none.
func attempt(t *testing.T, tx pgx.Tx, sql string, args ...any) string {
	t.Helper()
	ctx := t.Context()
	sp, err := tx.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sp.Exec(ctx, sql, args...); err != nil {
		if rbErr := sp.Rollback(ctx); rbErr != nil {
			t.Fatal(rbErr)
		}
		var pgErr *pgconn.PgError
		if !errors.As(err, &pgErr) {
			t.Fatalf("%s: %v", sql, err)
		}
		return pgErr.Code
	}
	if err := sp.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	return ""
}

// clientOf returns the client the account names, "none" for NULL.
func clientOf(t *testing.T, tx pgx.Tx, account string) string {
	t.Helper()
	var name *string
	if err := tx.QueryRow(t.Context(), "SELECT oauth_client FROM accounts WHERE account_id = $1", account).Scan(&name); err != nil {
		t.Fatal(err)
	}
	if name == nil {
		return "none"
	}
	return *name
}

// The SQLSTATE codes the schema's refusals raise.
const (
	foreignKeyViolation = "23503"
	uniqueViolation     = "23505"
)

// F6's row for an account naming another provider's client. The account's reference to its client
// carries its provider, so inserting an account that names a client of another provider, and changing
// an existing account's client to one, are refused by the schema and write nothing. Naming a client of
// the account's own provider is accepted, so the refusal is the provider's and not the reference's
// alone (ADR-0106, ADR-0016).
func TestAnAccountCannotNameAnotherProvidersClient(t *testing.T) {
	tx := seeded(t)
	for _, sql := range []string{
		"INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'household-id', 's')",
		"INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('mailer', 'fastmail', 'mailer-id', 's')",
	} {
		if code := attempt(t, tx, sql); code != "" {
			t.Fatalf("%s: %s", sql, code)
		}
	}

	if code := attempt(t, tx, "INSERT INTO accounts (account_id, provider, oauth_client) VALUES ('acct-crossed', 'gmail', 'mailer')"); code != foreignKeyViolation {
		t.Errorf("inserting a Gmail account naming a Fastmail client: %q, want %s", code, foreignKeyViolation)
	}
	var n int
	if err := tx.QueryRow(t.Context(), "SELECT count(*) FROM accounts WHERE account_id = 'acct-crossed'").Scan(&n); err != nil || n != 0 {
		t.Errorf("the refused account is stored %d times, error %v", n, err)
	}
	if code := attempt(t, tx, "UPDATE accounts SET oauth_client = 'household' WHERE account_id = $1", accountA); code != "" {
		t.Fatalf("naming a client of the account's own provider: %q, want it accepted", code)
	}
	if code := attempt(t, tx, "UPDATE accounts SET oauth_client = 'mailer' WHERE account_id = $1", accountA); code != foreignKeyViolation {
		t.Errorf("changing a Gmail account's client to a Fastmail client: %q, want %s", code, foreignKeyViolation)
	}
	if got := clientOf(t, tx, accountA); got != "household" {
		t.Errorf("after the refused change the account names %s, want household", got)
	}
}

// F6's row for removing a client. A client an account connects through cannot be removed, and the
// client and the account are unchanged. One no account connects through is removed, so the refusal
// comes from the account's reference and not from removal being refused outright (ADR-0106,
// ADR-0016).
func TestAClientAnAccountConnectsThroughCannotBeRemoved(t *testing.T) {
	tx := seeded(t)
	for _, sql := range []string{
		"INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'household-id', 's')",
		"INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('spare', 'gmail', 'spare-id', 's')",
		"UPDATE accounts SET oauth_client = 'household' WHERE account_id = '" + accountA + "'",
	} {
		if code := attempt(t, tx, sql); code != "" {
			t.Fatalf("%s: %s", sql, code)
		}
	}

	if code := attempt(t, tx, "DELETE FROM oauth_clients WHERE name = 'household'"); code != foreignKeyViolation {
		t.Errorf("removing the client an account connects through: %q, want %s", code, foreignKeyViolation)
	}
	var n int
	if err := tx.QueryRow(t.Context(), "SELECT count(*) FROM oauth_clients WHERE name = 'household'").Scan(&n); err != nil || n != 1 {
		t.Errorf("after the refused removal the client is stored %d times, error %v", n, err)
	}
	if got := clientOf(t, tx, accountA); got != "household" {
		t.Errorf("after the refused removal the account names %s, want household", got)
	}
	if code := attempt(t, tx, "DELETE FROM oauth_clients WHERE name = 'spare'"); code != "" {
		t.Errorf("removing a client no account connects through: %q, want it removed", code)
	}
}

// F6's row for storing one client twice. A provider's client identifier is held by one client only,
// and a name by one client in the whole installation, so a second client of the provider with an
// identifier another holds, or a client of any provider under a name another holds, is refused by the
// schema. The same identifier under another provider
// is accepted, since the uniqueness is per provider (ADR-0106, ADR-0016).
func TestOneClientIsNeverStoredTwice(t *testing.T) {
	tx := seeded(t)
	if code := attempt(t, tx, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'shared-id', 's')"); code != "" {
		t.Fatalf("storing the first client: %s", code)
	}

	if code := attempt(t, tx, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('second', 'gmail', 'shared-id', 's')"); code != uniqueViolation {
		t.Errorf("storing the provider's client identifier under another name: %q, want %s", code, uniqueViolation)
	}
	if code := attempt(t, tx, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'gmail', 'other-id', 's')"); code != uniqueViolation {
		t.Errorf("storing a client under a name another holds: %q, want %s", code, uniqueViolation)
	}
	if code := attempt(t, tx, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('household', 'fastmail', 'mailer-id', 's')"); code != uniqueViolation {
		t.Errorf("storing another provider's client under a name a client holds: %q, want %s", code, uniqueViolation)
	}
	if code := attempt(t, tx, "INSERT INTO oauth_clients (name, provider, client_id, client_secret) VALUES ('mailer', 'fastmail', 'shared-id', 's')"); code != "" {
		t.Errorf("storing the identifier under another provider: %q, want it accepted", code)
	}
	var n int
	if err := tx.QueryRow(t.Context(), "SELECT count(*) FROM oauth_clients").Scan(&n); err != nil || n != 2 {
		t.Errorf("%d clients are stored, error %v, want the first and the other provider's", n, err)
	}
}

// M7's row for an account identifier the mediator's API root cannot address, and F11's grammar that
// replaced its check. Storing an account whose identifier is exactly ., .. or /, or any identifier
// outside the grammar of one lowercase DNS label, is refused by the schema, whatever writes it, and an
// identifier inside the grammar is stored, so the refusal is the grammar's alone (ADR-0087, ADR-0016).
func TestNoAccountIdentifierIsAPathSegmentThePathCannotHold(t *testing.T) {
	tx := seeded(t)
	label := strings.Repeat("a", 63)
	for _, id := range []string{".", "..", "/", "...", "jo.smith", "a/b", "Personal", "-personal", "personal-", "per_sonal", "", label + "a"} {
		if code := attempt(t, tx, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", id); code != checkViolation {
			t.Errorf("storing the account %q: %q, want %s", id, code, checkViolation)
		}
	}
	for _, id := range []string{"personal", "a", "7", "work-2", "9lives", label} {
		if code := attempt(t, tx, "INSERT INTO accounts (account_id, provider) VALUES ($1, 'gmail')", id); code != "" {
			t.Errorf("storing the account %q: %q, want it stored", id, code)
		}
	}
}
