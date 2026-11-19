// Package migrate applies a migration chain to the database with goose linked as a library
// (ADR-0139). The migrator applies the chain its binary embeds, db/migrations, and the integration
// runs apply the same chain through the same code. It is the one package goose is admitted to.
//
// The chain's grants name the application user through Placeholder, and Apply puts the name it is
// given in its place as goose reads each file (ADR-0048, ADR-0143). db/check holds the placeholder to
// the grantee of a grant or a revoke, so the replacement changes nothing else.
package migrate

import (
	"bytes"
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/pressly/goose/v3"
	"github.com/pressly/goose/v3/lock"
)

// Placeholder is the grantee every grant and revoke of the chain names in place of the application
// user, whose name the project does not fix (ADR-0048, ADR-0143). It is an ordinary unquoted role
// name, so the code generator and sqlfluff read the migration files as plain SQL.
const Placeholder = "application_user"

// Apply applies every migration of chain the database does not yet hold, in order, each in its own
// transaction, connected through db as the role that owns the schema, with applicationUser, quoted as
// an identifier, in place of every occurrence of Placeholder in the chain's SQL files. It refuses an
// empty applicationUser. It first takes goose's PostgreSQL advisory lock on a connection of its own,
// so a second run at the same time waits, then finds the migrations already applied and applies
// nothing. goose is handed logger and, since it is not made verbose, writes no record through it.
func Apply(ctx context.Context, db *sql.DB, chain fs.FS, applicationUser string, logger *slog.Logger) error {
	if applicationUser == "" {
		return errors.New("the application user's name is empty, so the chain's grants name no one")
	}
	locker, err := lock.NewPostgresSessionLocker()
	if err != nil {
		return fmt.Errorf("building the migration lock: %w", err)
	}
	provider, err := goose.NewProvider(goose.DialectPostgres, db, granting{chain: chain, grantee: pgx.Identifier{applicationUser}.Sanitize()},
		goose.WithSessionLocker(locker),
		goose.WithSlog(logger),
		goose.WithDisableGlobalRegistry(true),
	)
	if err != nil {
		return fmt.Errorf("reading the migration chain: %w", err)
	}
	if _, err := provider.Up(ctx); err != nil {
		return fmt.Errorf("applying the migration chain: %w", err)
	}
	return nil
}

// granting is a migration chain read with grantee, a quoted identifier, in place of the placeholder
// in each SQL file. Every other file and every directory reads as the chain holds it.
type granting struct {
	chain   fs.FS
	grantee string
}

// Open opens name in the chain, and for a SQL file returns its text with the grantee in place of
// every occurrence of the placeholder Occurrences finds.
func (g granting) Open(name string) (fs.File, error) {
	f, err := g.chain.Open(name)
	if err != nil || !strings.HasSuffix(name, ".sql") {
		return f, err
	}
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		return f, err
	}
	text, err := io.ReadAll(f)
	if err = errors.Join(err, f.Close()); err != nil {
		return nil, fmt.Errorf("reading %s: %w", name, err)
	}
	replaced := replace(text, g.grantee)
	return &grantedFile{Reader: bytes.NewReader(replaced), info: grantedInfo{FileInfo: info, size: int64(len(replaced))}}, nil
}

// grantedFile is a SQL file of the chain with the grantee in place, read from memory.
type grantedFile struct {
	*bytes.Reader
	info grantedInfo
}

func (f *grantedFile) Stat() (fs.FileInfo, error) { return f.info, nil }
func (f *grantedFile) Close() error               { return nil }

// grantedInfo is the chain's file information with the size of the replaced text.
type grantedInfo struct {
	fs.FileInfo
	size int64
}

func (i grantedInfo) Size() int64 { return i.size }

// replace returns text with grantee in place of every occurrence Occurrences finds.
func replace(text []byte, grantee string) []byte {
	var out bytes.Buffer
	last := 0
	for _, at := range Occurrences(text) {
		out.Write(text[last:at])
		out.WriteString(grantee)
		last = at + len(Placeholder)
	}
	out.Write(text[last:])
	return out.Bytes()
}

// Occurrences returns the offset of every occurrence of the placeholder in text that stands as a whole
// word, bounded on each side by the text's start or end or a byte that cannot continue an unquoted
// identifier, as PostgreSQL reads one, which are the occurrences Apply replaces. An occurrence inside a
// longer name is not one. db/check holds each to the grantee of a grant or a revoke.
func Occurrences(text []byte) []int {
	var out []int
	for at := 0; ; {
		i := bytes.Index(text[at:], []byte(Placeholder))
		if i < 0 {
			return out
		}
		start, end := at+i, at+i+len(Placeholder)
		at = end
		if (start == 0 || !continuesIdentifier(text[start-1])) && (end == len(text) || !continuesIdentifier(text[end])) {
			out = append(out, start)
		}
	}
}

// continuesIdentifier reports whether c can stand inside an unquoted identifier, a letter, a digit,
// an underscore, a dollar sign or a byte of a multi-byte character.
func continuesIdentifier(c byte) bool {
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c == '_' || c == '$' || c >= 0x80
}
