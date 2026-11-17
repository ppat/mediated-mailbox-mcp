package migrate

import (
	"io/fs"
	"log/slog"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/jackc/pgx/v5"
)

// TestTheApplicationUserTakesThePlaceholdersPlace reads a chain through the replacement and requires
// the quoted name in place of every whole-word occurrence of the placeholder in a SQL file, and
// nothing else changed. A name holding a quote is quoted so it cannot end the identifier, a name
// spelled as the placeholder is quoted once, and a longer name holding the placeholder, a file that
// is not SQL and a directory read as the chain holds them (ADR-0048).
func TestTheApplicationUserTakesThePlaceholdersPlace(t *testing.T) {
	chain := fstest.MapFS{
		"00001_grants.sql": {Data: []byte("GRANT SELECT ON a TO application_user;\nGRANT SELECT ON b TO application_user, x_application_user, application_user_x;\nREVOKE INSERT ON c FROM application_user")},
		"notes.txt":        {Data: []byte("application_user")},
		"sub/00002.sql":    {Data: []byte("(application_user)")},
	}
	for _, c := range []struct {
		name string
		want map[string]string
	}{
		{"mailbox_app", map[string]string{
			"00001_grants.sql": "GRANT SELECT ON a TO \"mailbox_app\";\nGRANT SELECT ON b TO \"mailbox_app\", x_application_user, application_user_x;\nREVOKE INSERT ON c FROM \"mailbox_app\"",
			"notes.txt":        "application_user",
			"sub/00002.sql":    "(\"mailbox_app\")",
		}},
		{`a"; DROP TABLE a; --`, map[string]string{
			"sub/00002.sql": `("a""; DROP TABLE a; --")`,
		}},
		{"application_user", map[string]string{
			"sub/00002.sql": `("application_user")`,
		}},
		{"Mixed$Case", map[string]string{
			"sub/00002.sql": `("Mixed$Case")`,
		}},
	} {
		g := granting{chain: chain, grantee: pgx.Identifier{c.name}.Sanitize()}
		for path, want := range c.want {
			got, err := fs.ReadFile(g, path)
			if err != nil {
				t.Fatal(err)
			}
			if string(got) != want {
				t.Errorf("with %q, %s reads %q, want %q", c.name, path, got, want)
			}
			info, err := fs.Stat(g, path)
			if err != nil {
				t.Fatal(err)
			}
			if info.Size() != int64(len(want)) {
				t.Errorf("with %q, %s reports %d bytes, want %d", c.name, path, info.Size(), len(want))
			}
		}
	}
	entries, err := fs.ReadDir(granting{chain: chain, grantee: `"x"`}, ".")
	if err != nil || len(entries) != 3 {
		t.Errorf("the chain's directory reads %v, %v, want its three entries", entries, err)
	}
}

// TestAnEmptyApplicationUserIsRefused requires Apply to refuse an empty application user's name before
// it reaches the database, since a chain applied with it would grant to no one (ADR-0048). No database
// is given, so a refusal from anywhere past the check would fail differently.
func TestAnEmptyApplicationUserIsRefused(t *testing.T) {
	err := Apply(t.Context(), nil, fstest.MapFS{}, "", slog.New(slog.DiscardHandler))
	if err == nil || !strings.Contains(err.Error(), "the application user's name is empty") {
		t.Errorf("Apply with an empty name returned %v, want its refusal of the empty name", err)
	}
}
