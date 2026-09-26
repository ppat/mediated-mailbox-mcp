// Package connection is the database section of backfill's configuration and its pure validation,
// which checks the merged value before anything else starts (ADR-0078). A required value met by an
// empty text would otherwise let the driver substitute a default the operator never chose, such as
// its Unix socket directory for an empty host.
package connection

import (
	"errors"
	"strconv"
)

// Config is the database section. Every value is rendered into the connection string on every
// start, and the password comes from the mounted file PasswordFile names (ADR-0078, ADR-0079).
type Config struct {
	Host         string `yaml:"host" settings:"required"`
	Port         int    `yaml:"port"`
	Name         string `yaml:"name" settings:"required"`
	User         string `yaml:"user"`
	SSLMode      string `yaml:"sslmode"`
	SSLRootCert  string `yaml:"sslrootcert"`
	PasswordFile string `yaml:"password_file" settings:"required"`
}

// Validate refuses an empty host, database name, user or password file path, a port outside 1 to
// 65535, and a TLS mode the driver does not accept.
func Validate(c Config) error {
	switch {
	case c.Host == "":
		return errors.New("database.host is empty")
	case c.Name == "":
		return errors.New("database.name is empty")
	case c.User == "":
		return errors.New("database.user is empty")
	case c.PasswordFile == "":
		return errors.New("database.password_file is empty")
	case c.Port < 1 || c.Port > 65535:
		return errors.New("database.port " + strconv.Itoa(c.Port) + " is outside 1 to 65535")
	}
	switch c.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return nil
	}
	return errors.New("database.sslmode " + strconv.Quote(c.SSLMode) + " is not disable, allow, prefer, require, verify-ca or verify-full")
}
