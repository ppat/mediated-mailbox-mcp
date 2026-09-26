// Package connection builds backfill's database connection from its configuration's database
// section, and refuses a start whose environment could override it (ADR-0078, ADR-0079). The pure
// validation of the section is backfill/internal/core/connection's.
package connection

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	core "github.com/ppat/mediated-mailbox-mcp/backfill/internal/core/connection"
)

// RefusePasswordVariables refuses a start while PGPASSWORD or PGSSLPASSWORD is set, an empty value
// included, since the driver reads either on every parse and a stray one would win over the mounted
// password file (ADR-0078).
func RefusePasswordVariables(environ []string) error {
	for _, entry := range environ {
		name, _, _ := strings.Cut(entry, "=")
		if name == "PGPASSWORD" || name == "PGSSLPASSWORD" {
			return fmt.Errorf("the environment sets %s, and the database password comes only from the mounted password file", name)
		}
	}
	return nil
}

// PoolConfig renders every value of c into the connection string, an empty sslrootcert included,
// so the driver takes none of them from its PG* variables. It sets the password read from the
// password file, less one trailing newline, and refuses an empty one.
func PoolConfig(c core.Config) (*pgxpool.Config, error) {
	password, err := readPassword(c.PasswordFile)
	if err != nil {
		return nil, err
	}
	pairs := []string{
		"host=" + quote(c.Host),
		"port=" + quote(strconv.Itoa(c.Port)),
		"dbname=" + quote(c.Name),
		"user=" + quote(c.User),
		"sslmode=" + quote(c.SSLMode),
		"sslrootcert=" + quote(c.SSLRootCert),
	}
	config, err := pgxpool.ParseConfig(strings.Join(pairs, " "))
	if err != nil {
		return nil, err
	}
	config.ConnConfig.Password = password
	return config, nil
}

// readPassword reads the file holding the password alone and trims one trailing newline, written
// \n or \r\n, since editors and kubectl create secret --from-file commonly end a file with one.
func readPassword(path string) (string, error) {
	text, err := os.ReadFile(path) //nolint:gosec // The path is the mounted file the deployment names.
	if err != nil {
		return "", fmt.Errorf("reading the password file: %w", err)
	}
	password := string(text)
	if trimmed, ok := strings.CutSuffix(password, "\r\n"); ok {
		password = trimmed
	} else {
		password = strings.TrimSuffix(password, "\n")
	}
	if password == "" {
		return "", fmt.Errorf("the password file %s holds no password", path)
	}
	return password, nil
}

// quote writes a value of the keyword/value connection string, quoted, with a backslash or a quote
// escaped, so a value cannot end its quoted string and add a setting of its own.
func quote(value string) string {
	return "'" + strings.NewReplacer(`\`, `\\`, `'`, `\'`).Replace(value) + "'"
}
