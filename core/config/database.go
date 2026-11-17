package config

import (
	"errors"
	"strconv"
)

// Database is the database part, the connection settings of every process that connects. Every
// value but the pool size is rendered into the connection string on every start, the password comes
// from the mounted file PasswordFile names, and PoolSize is the most connections a pool opens
// (ADR-0078, ADR-0079). The user has no default, since a database user's name is configuration the
// deployment platform provides (ADR-0143).
type Database struct {
	Host         string `yaml:"host" settings:"required"`
	Port         int    `yaml:"port"`
	Name         string `yaml:"name" settings:"required"`
	User         string `yaml:"user" settings:"required"`
	SSLMode      string `yaml:"sslmode"`
	SSLRootCert  string `yaml:"sslrootcert"`
	PasswordFile string `yaml:"password_file" settings:"required"`
	PoolSize     int    `yaml:"pool_size"`
}

// DefaultDatabase is the database part's defaults with poolSize as its pool size. The TLS mode is
// the one that fails closed. The host, the name, the user and the password file have no default.
func DefaultDatabase(poolSize int) Database {
	return Database{Port: 5432, SSLMode: "verify-full", PoolSize: poolSize}
}

// DriverPoolSize is the database driver's own default pool size on a node with processors
// processors, the larger of 4 and the processor count, which the API server, the UI and the
// migrator default their pool size to, so leaving the value unset opens as many connections as the
// driver would by itself (ADR-0078).
func DriverPoolSize(processors int) int {
	return max(4, processors)
}

// Check refuses an empty host, database name, user or password file path, a port outside 1 to
// 65535, a pool size that is not positive, and a TLS mode the driver does not accept. A required
// value met by an empty text would otherwise let the driver substitute a default the operator never
// chose, such as its Unix socket directory for an empty host.
func (d Database) Check() error {
	switch {
	case d.Host == "":
		return errors.New("database.host is empty")
	case d.Name == "":
		return errors.New("database.name is empty")
	case d.User == "":
		return errors.New("database.user is empty")
	case d.PasswordFile == "":
		return errors.New("database.password_file is empty")
	case d.Port < 1 || d.Port > 65535:
		return errors.New("database.port " + strconv.Itoa(d.Port) + " is outside 1 to 65535")
	case d.PoolSize < 1:
		return errors.New("database.pool_size " + strconv.Itoa(d.PoolSize) + " is not positive")
	}
	switch d.SSLMode {
	case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		return nil
	}
	return errors.New("database.sslmode " + strconv.Quote(d.SSLMode) + " is not disable, allow, prefer, require, verify-ca or verify-full")
}
