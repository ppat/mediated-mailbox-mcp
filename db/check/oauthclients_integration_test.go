//go:build integration

package check_test

import (
	"context"
	"errors"
	"fmt"
	neturl "net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/postgres"
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

// keyedOnNameVersion is the migration that keys oauth_clients on the client's name. The chain before it
// keys the table on the provider.
const keyedOnNameVersion = 21

// The migration that keys oauth_clients on the client's name names the client already stored after
// its provider, leaves its sealed secret's bytes as they were, so the secret stays bound to the row
// its name keys, and points every account of that provider at it. An account of a provider with no
// client names none (ADR-0106, ADR-0088, ADR-0048).
func TestTheStoredClientIsNamedAfterItsProviderAndItsAccountsPointAtIt(t *testing.T) {
	ctx := t.Context()
	admin := os.Getenv(postgres.EnvAdminURL)
	if admin == "" {
		t.Fatalf("run under pgrun, which sets %s", postgres.EnvAdminURL)
	}
	migrations, err := filepath.Glob(filepath.Join("..", "migrations", "*.sql"))
	if err != nil || len(migrations) == 0 {
		t.Fatalf("no migrations found: %v", err)
	}
	before := t.TempDir()
	for _, path := range migrations {
		var version int
		if _, err := fmt.Sscanf(filepath.Base(path), "%d_", &version); err != nil {
			t.Fatal(err)
		}
		if version >= keyedOnNameVersion {
			continue
		}
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		//nolint:gosec // The name is a checked-in migration's, written into the test's own directory.
		if err := os.WriteFile(filepath.Join(before, filepath.Base(path)), src, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	migrate, err := neturl.Parse(admin)
	if err != nil {
		t.Fatal(err)
	}
	// A space encoded as a plus sign reaches the server as a plus sign.
	migrate.RawQuery += "&options=" + strings.ReplaceAll(neturl.QueryEscape("-c role="+postgres.MigrationRole), "+", "%20")
	name := fmt.Sprintf("check_clients_by_name_%d", os.Getpid())
	adminConn, err := pgx.Connect(ctx, admin)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if _, err := adminConn.Exec(context.Background(), "DROP DATABASE IF EXISTS "+pgx.Identifier{name}.Sanitize()+" WITH (FORCE)"); err != nil {
			t.Error(err)
		}
		if err := adminConn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	})
	if err := postgres.ApplyChain(ctx, admin, migrate.String(), name, filepath.Join("..", "bootstrap", "extensions.sql"), before); err != nil {
		t.Fatal(err)
	}
	database := *migrate
	database.Path = "/" + name
	conn, err := pgx.Connect(ctx, database.String())
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		if err := conn.Close(context.Background()); err != nil {
			t.Error(err)
		}
	}()
	sealed := []byte("the sealed secret's bytes")
	for _, sql := range []string{
		"INSERT INTO oauth_clients (provider, client_id, client_secret) VALUES ('gmail', 'gmail-id', $1)",
		"INSERT INTO accounts (account_id, provider) VALUES ('personal', 'gmail'), ('work', 'gmail'), ('other', 'fastmail')",
	} {
		args := []any{}
		if strings.Contains(sql, "$1") {
			args = append(args, sealed)
		}
		if _, err := conn.Exec(ctx, sql, args...); err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
	}

	//nolint:gosec // The URL is the one pgrun exported for this run's own database, and the command is goose.
	goose := exec.CommandContext(ctx, "goose", "-dir", filepath.Join("..", "migrations"), "postgres", database.String(), "up")
	goose.Env = append(os.Environ(), "GOOSE_DRIVER=", "GOOSE_DBSTRING=", "GOOSE_MIGRATION_DIR=")
	if out, err := goose.CombinedOutput(); err != nil {
		t.Fatalf("migrating the stored rows: %v\n%s", err, out)
	}

	var clientName, provider, clientID string
	var secret []byte
	if err := conn.QueryRow(ctx, "SELECT name, provider, client_id, client_secret FROM oauth_clients").Scan(&clientName, &provider, &clientID, &secret); err != nil {
		t.Fatal(err)
	}
	if clientName != "gmail" || provider != "gmail" || clientID != "gmail-id" || string(secret) != string(sealed) {
		t.Errorf("the stored client migrated to %q, %q, %q with secret %q, want it named gmail with its bytes unchanged", clientName, provider, clientID, secret)
	}
	rows, err := conn.Query(ctx, "SELECT account_id, coalesce(oauth_client, 'none') FROM accounts ORDER BY account_id")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	var account, client string
	if _, err := pgx.ForEachRow(rows, []any{&account, &client}, func() error { got[account] = client; return nil }); err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"personal": "gmail", "work": "gmail", "other": "none"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("the clients the accounts name (-want +got):\n%s", diff)
	}
}
