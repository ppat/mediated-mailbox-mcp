package database_test

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process/database"
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

func connection(t *testing.T, password string) config.Database {
	t.Helper()
	return config.Database{Host: "db.example", Port: 5433, Name: "mailbox", User: "mediated_mailbox_app", SSLMode: "verify-full", PasswordFile: passwordFile(t, password), PoolSize: 7}
}

// The connection takes every value from the configuration, whatever the PG* variables say, and the
// pool opens at most the configured pool size.
func TestTheConnectionIsTheConfigurations(t *testing.T) {
	t.Setenv("PGHOST", "elsewhere")
	t.Setenv("PGPORT", "6543")
	t.Setenv("PGDATABASE", "other")
	t.Setenv("PGUSER", "someone")
	t.Setenv("PGSSLMODE", "disable")
	t.Setenv("PGSSLROOTCERT", filepath.Join(t.TempDir(), "absent.crt"))
	pool, err := database.PoolConfig(connection(t, "secret"))
	if err != nil {
		t.Fatal(err)
	}
	conn := pool.ConnConfig
	type settings struct {
		Host, Database, User, Password, ServerName string
		Port                                       uint16
		MaxConns                                   int32
	}
	got := settings{Host: conn.Host, Port: conn.Port, Database: conn.Database, User: conn.User, Password: conn.Password, MaxConns: pool.MaxConns}
	if conn.TLSConfig != nil {
		got.ServerName = conn.TLSConfig.ServerName
	}
	want := settings{Host: "db.example", Port: 5433, Database: "mailbox", User: "mediated_mailbox_app", Password: "secret", ServerName: "db.example", MaxConns: 7}
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
	c := connection(t, "secret")
	c.SSLRootCert = filepath.Join(t.TempDir(), `it's a \ path' sslmode='disable`)
	_, err := database.PoolConfig(c)
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
		pool, err := database.PoolConfig(connection(t, c.text))
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
		c := connection(t, text)
		_, err := database.PoolConfig(c)
		want := "the password file " + c.PasswordFile + " holds no password"
		if err == nil || err.Error() != want {
			t.Errorf("%q: PoolConfig returned %v, want %q", text, err, want)
		}
	}
}

// The driver's pool size core/config defaults the API server's, the UI's and the migrator's to, on
// this node's processors, is the driver's own default, so a deployable whose configuration sets no
// pool size opens as many connections as the driver would have opened by itself.
func TestTheDefaultPoolSizeIsTheDriversOwn(t *testing.T) {
	driver, err := pgxpool.ParseConfig("")
	if err != nil {
		t.Fatal(err)
	}
	if got, want := config.DriverPoolSize(runtime.NumCPU()), int(driver.MaxConns); got != want {
		t.Errorf("DriverPoolSize(runtime.NumCPU()) = %d, want the driver's %d", got, want)
	}
}
