package config_test

import (
	"testing"

	"github.com/google/go-cmp/cmp"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/compare"
	"github.com/ppat/mediated-mailbox-mcp/testsupport/mustnotcompile"
)

const pkg = "github.com/ppat/mediated-mailbox-mcp/core/config"

// refused runs each case's change on a copy of valid and requires check to refuse it with want.
func refused[T any](t *testing.T, valid func() T, check func(T) error, cases []refusal[T]) {
	t.Helper()
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := valid()
			c.change(&v)
			err := check(v)
			if err == nil {
				t.Fatalf("Check accepted it, want %q", c.want)
			}
			if diff := cmp.Diff(c.want, err.Error(), compare.Options); diff != "" {
				t.Errorf("error (-want +got):\n%s", diff)
			}
		})
	}
}

type refusal[T any] struct {
	name   string
	change func(*T)
	want   string
}

// A log_level that names no level is refused, the spellings slog's own parser accepts included, and
// each level spelled exactly is accepted (ADR-0122).
func TestTheProcessPartIsChecked(t *testing.T) {
	for _, level := range []string{"debug", "info", "warn", "error"} {
		if err := (config.Process{LogLevel: level}).Check(); err != nil {
			t.Errorf("%s was refused: %v", level, err)
		}
	}
	refused(t, config.DefaultProcess, config.Process.Check, []refusal[config.Process]{
		{"an empty level", func(p *config.Process) { p.LogLevel = "" }, `log_level "" is not one of debug, info, warn and error`},
		{"an upper-case level", func(p *config.Process) { p.LogLevel = "INFO" }, `log_level "INFO" is not one of debug, info, warn and error`},
		{"a level with an offset", func(p *config.Process) { p.LogLevel = "info+2" }, `log_level "info+2" is not one of debug, info, warn and error`},
		{"a word slog does not know", func(p *config.Process) { p.LogLevel = "warning" }, `log_level "warning" is not one of debug, info, warn and error`},
	})
}

// An empty or blank probe_listen is refused.
func TestTheOperationsPartIsChecked(t *testing.T) {
	if err := config.DefaultOperations().Check(); err != nil {
		t.Errorf("the default was refused: %v", err)
	}
	refused(t, config.DefaultOperations, config.Operations.Check, []refusal[config.Operations]{
		{"an empty probe_listen", func(o *config.Operations) { o.ProbeListen = "" }, "probe_listen is empty"},
		{"a blank probe_listen", func(o *config.Operations) { o.ProbeListen = " " }, "probe_listen is empty"},
	})
}

// An empty listen is refused, and so is one of the two TLS files set without the other, a blank path
// counting as unset. Both files set serve TLS, and neither, or both blank, plain HTTP (ADR-0127).
func TestTheServingPartIsChecked(t *testing.T) {
	both := "the process serves TLS with both or plain HTTP with neither"
	refused(t, config.DefaultServing, config.Serving.Check, []refusal[config.Serving]{
		{"an empty listen", func(s *config.Serving) { s.Listen = " " }, "listen is empty"},
		{"a certificate without its key", func(s *config.Serving) { s.TLSCert = "/tls/cert" }, "tls_cert is set and tls_key is not, and " + both},
		{"a key without its certificate", func(s *config.Serving) { s.TLSKey = "/tls/key" }, "tls_key is set and tls_cert is not, and " + both},
		{"a certificate with a blank key", func(s *config.Serving) { s.TLSCert, s.TLSKey = "/tls/cert", "  " }, "tls_cert is set and tls_key is not, and " + both},
		{"a key with a blank certificate", func(s *config.Serving) { s.TLSCert, s.TLSKey = " ", "/tls/key" }, "tls_key is set and tls_cert is not, and " + both},
	})
	for name, files := range map[string][2]string{"both files": {"/tls/cert", "/tls/key"}, "neither file": {"", ""}, "both blank": {" ", "\t"}} {
		s := config.Serving{Listen: ":8443", TLSCert: files[0], TLSKey: files[1]}
		if err := s.Check(); err != nil {
			t.Errorf("%s was refused: %v", name, err)
		}
		if got, want := s.ServesTLS(), name == "both files"; got != want {
			t.Errorf("%s serves TLS %v, want %v", name, got, want)
		}
	}
}

func validDatabase() config.Database {
	//nolint:gosec // A mounted file's path, not a credential.
	return config.Database{Host: "db.example", Port: 5432, Name: "mailbox", User: "mediated_mailbox_app", SSLMode: "verify-full", PasswordFile: "/run/secrets/password", PoolSize: 4}
}

// A value that would let the driver substitute a default the operator never chose is refused.
func TestAnUnusableDatabaseValueIsRefused(t *testing.T) {
	refused(t, validDatabase, config.Database.Check, []refusal[config.Database]{
		{"an empty host", func(c *config.Database) { c.Host = "" }, "database.host is empty"},
		{"an empty database name", func(c *config.Database) { c.Name = "" }, "database.name is empty"},
		{"an empty user", func(c *config.Database) { c.User = "" }, "database.user is empty"},
		{"an empty password file path", func(c *config.Database) { c.PasswordFile = "" }, "database.password_file is empty"},
		{"port zero", func(c *config.Database) { c.Port = 0 }, "database.port 0 is outside 1 to 65535"},
		{"a port above 65535", func(c *config.Database) { c.Port = 65536 }, "database.port 65536 is outside 1 to 65535"},
		{"a pool size of zero", func(c *config.Database) { c.PoolSize = 0 }, "database.pool_size 0 is not positive"},
		{"a negative pool size", func(c *config.Database) { c.PoolSize = -1 }, "database.pool_size -1 is not positive"},
		{"an unknown TLS mode", func(c *config.Database) { c.SSLMode = "verify_full" }, `database.sslmode "verify_full" is not disable, allow, prefer, require, verify-ca or verify-full`},
		{"an empty TLS mode", func(c *config.Database) { c.SSLMode = "" }, `database.sslmode "" is not disable, allow, prefer, require, verify-ca or verify-full`},
	})
}

// Every database value in range is accepted, the ports at either end, the smallest pool and every
// TLS mode the driver takes.
func TestAUsableDatabaseValueIsAccepted(t *testing.T) {
	for _, port := range []int{1, 65535} {
		c := validDatabase()
		c.Port = port
		if err := c.Check(); err != nil {
			t.Errorf("port %d: %v", port, err)
		}
	}
	c := validDatabase()
	c.PoolSize = 1
	if err := c.Check(); err != nil {
		t.Errorf("pool size 1: %v", err)
	}
	for _, mode := range []string{"disable", "allow", "prefer", "require", "verify-ca", "verify-full"} {
		c := validDatabase()
		c.SSLMode = mode
		if err := c.Check(); err != nil {
			t.Errorf("sslmode %s: %v", mode, err)
		}
	}
}

// The database part's defaults are the port, the TLS mode that fails closed and the pool size given,
// and no user, since the project names no database user (ADR-0143). The driver's own pool size is
// the larger of 4 and the processor count.
func TestTheDatabaseDefaults(t *testing.T) {
	want := config.Database{Port: 5432, SSLMode: "verify-full", PoolSize: 7}
	if diff := cmp.Diff(want, config.DefaultDatabase(7), compare.Options); diff != "" {
		t.Errorf("defaults (-want +got):\n%s", diff)
	}
	for processors, want := range map[int]int{1: 4, 4: 4, 5: 5, 64: 64} {
		if got := config.DriverPoolSize(processors); got != want {
			t.Errorf("DriverPoolSize(%d) = %d, want %d", processors, got, want)
		}
	}
}

func validKeys() config.Keys {
	return config.Keys{PublicKeyFile: "/run/keys/public", PrivateKeyFiles: []string{"/run/keys/old", "/run/keys/new"}}
}

// A credential value that names no key file is refused, so the keyring is never built from an empty
// path.
func TestAnUnusableKeysValueIsRefused(t *testing.T) {
	refused(t, validKeys, config.Keys.Check, []refusal[config.Keys]{
		{"an empty public key path", func(c *config.Keys) { c.PublicKeyFile = "" }, "credential.public_key_file is empty"},
		{"no private key", func(c *config.Keys) { c.PrivateKeyFiles = nil }, "credential.private_key_files names no file"},
		{"an empty list of private keys", func(c *config.Keys) { c.PrivateKeyFiles = []string{} }, "credential.private_key_files names no file"},
		{"an empty first private key path", func(c *config.Keys) { c.PrivateKeyFiles[0] = "" }, "credential.private_key_files entry 0 is empty"},
		{"an empty later private key path", func(c *config.Keys) { c.PrivateKeyFiles[1] = "" }, "credential.private_key_files entry 1 is empty"},
	})
}

// One private key, and several during a key replacement, are accepted.
func TestAUsableKeysValueIsAccepted(t *testing.T) {
	for _, keys := range [][]string{{"/run/keys/current"}, {"/run/keys/old", "/run/keys/new"}} {
		c := validKeys()
		c.PrivateKeyFiles = keys
		if err := c.Check(); err != nil {
			t.Errorf("%v: %v", keys, err)
		}
	}
}

// The accounts' reload interval is a minute by default, and one that is not positive is refused.
func TestTheAccountsPartIsChecked(t *testing.T) {
	if got, want := config.DefaultAccounts().AccountReloadInterval, config.Duration(60_000_000_000); got != want {
		t.Errorf("the default reload interval is %d, want %d", got, want)
	}
	refused(t, config.DefaultAccounts, config.Accounts.Check, []refusal[config.Accounts]{
		{"a reload interval of zero", func(a *config.Accounts) { a.AccountReloadInterval = 0 }, "account_reload_interval is not positive"},
		{"a negative reload interval", func(a *config.Accounts) { a.AccountReloadInterval = -1 }, "account_reload_interval is not positive"},
	})
	if err := (config.Accounts{AccountReloadInterval: 1}).Check(); err != nil {
		t.Errorf("a reload interval of one nanosecond was refused: %v", err)
	}
}

// Each part is pinned field by field, so a new value, one meant to hold a secret included, is a
// visible change, and the scanner's part carries no revision an operator could set (ADR-0078).
func TestThePartsArePinned(t *testing.T) {
	mustnotcompile.RequireFields(t, pkg, "Process", "LogLevel string")
	mustnotcompile.RequireFields(t, pkg, "Operations", "ProbeListen string")
	mustnotcompile.RequireFields(t, pkg, "Serving", "Listen string", "TLSCert string", "TLSKey string")
	mustnotcompile.RequireFields(t, pkg, "Database",
		"Host string",
		"Port int",
		"Name string",
		"User string",
		"SSLMode string",
		"SSLRootCert string",
		"PasswordFile string",
		"PoolSize int",
	)
	mustnotcompile.RequireFields(t, pkg, "Keys", "PublicKeyFile string", "PrivateKeyFiles []string")
	mustnotcompile.RequireFields(t, pkg, "Accounts", "AccountReloadInterval Duration")
	mustnotcompile.RequireFields(t, pkg, "Scanner",
		"Triggers map[string][]string",
		"LinkWords []string",
		"LinkParams []string",
		"Window int",
		"DenseLength int",
		"DenseEntropy float64",
		"Weights Weights",
		"Threshold float64",
		"SubjectThreshold float64",
	)
	mustnotcompile.RequireFields(t, pkg, "Weights",
		"Entropy float64", "Mix float64", "Length float64", "Proximity float64", "Position float64")
}
