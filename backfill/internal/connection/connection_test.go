package connection_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/connection"
	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/connection"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
)

func passwordFile(t *testing.T, text string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "password")
	if err := os.WriteFile(path, []byte(text), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func config(t *testing.T, password string) core.Config {
	t.Helper()
	return core.Config{Host: "db.example", Port: 5433, Name: "mailbox", User: "mediated_mailbox_backfill", SSLMode: "verify-full", PasswordFile: passwordFile(t, password)}
}

// A start refuses while PGPASSWORD or PGSSLPASSWORD is set, an empty value included.
func TestAPasswordVariableIsRefused(t *testing.T) {
	cases := []struct{ entry, name string }{
		{"PGPASSWORD=secret", "PGPASSWORD"},
		{"PGPASSWORD=", "PGPASSWORD"},
		{"PGSSLPASSWORD=secret", "PGSSLPASSWORD"},
		{"PGSSLPASSWORD=", "PGSSLPASSWORD"},
	}
	for _, c := range cases {
		t.Run(c.entry, func(t *testing.T) {
			err := connection.RefusePasswordVariables([]string{"PGHOST=elsewhere", c.entry})
			want := "the environment sets " + c.name + ", and the database password comes only from the mounted password file"
			if err == nil || err.Error() != want {
				t.Errorf("RefusePasswordVariables returned %v, want %q", err, want)
			}
		})
	}
}

// Every other variable leaves the start alone.
func TestOtherVariablesAreNotRefused(t *testing.T) {
	if err := connection.RefusePasswordVariables([]string{"PGHOST=elsewhere", "PGPASSFILE=/tmp/pass", "PGSSLMODE=disable", "HOME=/"}); err != nil {
		t.Errorf("the start was refused: %v", err)
	}
}

// The connection takes every value from the configuration, whatever the PG* variables say.
func TestTheConnectionIsTheConfigurations(t *testing.T) {
	t.Setenv("PGHOST", "elsewhere")
	t.Setenv("PGPORT", "6543")
	t.Setenv("PGDATABASE", "other")
	t.Setenv("PGUSER", "someone")
	t.Setenv("PGSSLMODE", "disable")
	t.Setenv("PGSSLROOTCERT", filepath.Join(t.TempDir(), "absent.crt"))
	pool, err := connection.PoolConfig(config(t, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.ConnConfig
	type settings struct {
		Host, Database, User, Password, ServerName string
		Port                                       uint16
	}
	got := settings{Host: conn.Host, Port: conn.Port, Database: conn.Database, User: conn.User, Password: conn.Password}
	if conn.TLSConfig != nil {
		got.ServerName = conn.TLSConfig.ServerName
	}
	want := settings{Host: "db.example", Port: 5433, Database: "mailbox", User: "mediated_mailbox_backfill", Password: "secret", ServerName: "db.example"}
	if diff := cmp.Diff(want, got, compare.Options); diff != "" {
		t.Errorf("connection (-want +got):\n%s", diff)
	}
	if conn.TLSConfig == nil || conn.TLSConfig.InsecureSkipVerify {
		t.Errorf("TLS %+v, want the server's certificate verified", conn.TLSConfig)
	}
}

// A value holding a quote and a backslash reaches the driver intact and cannot add a setting of its
// own, such as a TLS mode that would win over the configured one.
func TestAValueCannotChangeAnotherSetting(t *testing.T) {
	c := config(t, "secret")
	c.SSLRootCert = filepath.Join(t.TempDir(), `it's a \ path' sslmode='disable`)
	_, err := connection.PoolConfig(c)
	if err == nil {
		t.Fatal("the connection was configured, so the TLS mode was changed and the CA file never read")
	}
	if !strings.Contains(err.Error(), "open "+c.SSLRootCert+":") {
		t.Errorf("error %q, want the driver to open %q", err, c.SSLRootCert)
	}
}

// The password is the file's text less one trailing newline, written \n or \r\n.
func TestThePasswordIsTheFilesText(t *testing.T) {
	for _, c := range []struct{ text, want string }{
		{"it's a \\ secret", "it's a \\ secret"},
		{"secret\n", "secret"},
		{"secret\r\n", "secret"},
		{"secret\n\n", "secret\n"},
		{"secret\r\n\r\n", "secret\r\n"},
	} {
		pool, err := connection.PoolConfig(config(t, c.text))
		if err != nil {
			t.Fatalf("%q: %v", c.text, err)
		}
		if got := pool.ConnConfig.Password; got != c.want {
			t.Errorf("%q gave the password %q, want %q", c.text, got, c.want)
		}
	}
}

// A password file holding no password, once its newline is trimmed, refuses the start.
func TestAnEmptyPasswordIsRefused(t *testing.T) {
	for _, text := range []string{"", "\n", "\r\n"} {
		c := config(t, text)
		_, err := connection.PoolConfig(c)
		want := "the password file " + c.PasswordFile + " holds no password"
		if err == nil || err.Error() != want {
			t.Errorf("%q: PoolConfig returned %v, want %q", text, err, want)
		}
	}
}
