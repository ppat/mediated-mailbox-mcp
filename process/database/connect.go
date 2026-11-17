package database

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
)

// poolConfig renders every connection value of c into the connection string, an empty sslrootcert
// included, so the driver takes none of them from its PG* variables, and caps the pool at c's pool
// size. It sets the password read from the password file, less one trailing newline, and refuses an
// empty one.
func poolConfig(c config.Database) (*pgxpool.Config, error) {
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
	config.MaxConns = int32(min(c.PoolSize, 1<<31-1)) //nolint:gosec // Bounded above.
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
