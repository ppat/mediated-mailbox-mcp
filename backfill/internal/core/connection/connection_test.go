package connection_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/connection"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

func valid() connection.Config {
	//nolint:gosec // A mounted file's path, not a credential.
	return connection.Config{Host: "db.example", Port: 5432, Name: "mailbox", User: "mediated_mailbox_backfill", SSLMode: "verify-full", PasswordFile: "/run/secrets/password"}
}

// A value that would let the driver substitute a default the operator never chose is refused.
func TestAnUnusableValueIsRefused(t *testing.T) {
	cases := []struct {
		name   string
		change func(*connection.Config)
		want   string
	}{
		{"an empty host", func(c *connection.Config) { c.Host = "" }, "database.host is empty"},
		{"an empty database name", func(c *connection.Config) { c.Name = "" }, "database.name is empty"},
		{"an empty user", func(c *connection.Config) { c.User = "" }, "database.user is empty"},
		{"an empty password file path", func(c *connection.Config) { c.PasswordFile = "" }, "database.password_file is empty"},
		{"port zero", func(c *connection.Config) { c.Port = 0 }, "database.port 0 is outside 1 to 65535"},
		{"a port above 65535", func(c *connection.Config) { c.Port = 65536 }, "database.port 65536 is outside 1 to 65535"},
		{"an unknown TLS mode", func(c *connection.Config) { c.SSLMode = "verify_full" }, `database.sslmode "verify_full" is not disable, allow, prefer, require, verify-ca or verify-full`},
		{"an empty TLS mode", func(c *connection.Config) { c.SSLMode = "" }, `database.sslmode "" is not disable, allow, prefer, require, verify-ca or verify-full`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			config := valid()
			c.change(&config)
			err := connection.Validate(config)
			if err == nil {
				t.Fatalf("Validate accepted it, want %q", c.want)
			}
			if diff := cmp.Diff(c.want, err.Error(), compare.Options); diff != "" {
				t.Errorf("error (-want +got):\n%s", diff)
			}
		})
	}
}

// Every value in range is accepted, the ports at either end and every TLS mode the driver takes.
func TestAUsableValueIsAccepted(t *testing.T) {
	for _, port := range []int{1, 65535} {
		config := valid()
		config.Port = port
		if err := connection.Validate(config); err != nil {
			t.Errorf("port %d: %v", port, err)
		}
	}
	for _, mode := range []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		config := valid()
		config.SSLMode = mode
		if err := connection.Validate(config); err != nil {
			t.Errorf("sslmode %s: %v", mode, err)
		}
	}
}

// The database section is pinned field by field, so a new value, one meant to hold a secret
// included, is a visible change (ADR-0078).
func TestTheSectionIsPinned(t *testing.T) {
	mustnotcompile.RequireFields(t, "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/connection", "Config",
		"Host string",
		"Port int",
		"Name string",
		"User string",
		"SSLMode string",
		"SSLRootCert string",
		"PasswordFile string",
	)
}
