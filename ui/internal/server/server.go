// Package server holds what every endpoint of the UI sits behind: the checks and their key, the error
// contract and the route type, with the request plumbing every route of the read API uses, the reading
// of a request's body and the answer to a failed statement (ADR-0142). The checks are the session cookie, the request token
// (ADR-0061), the content security policy (ADR-0062), and the request's log line with its id
// (docs/UI.md section 18.2). Each endpoint the UI's spec lists wraps its handler in them, so the
// process, which mounts each endpoint at its pattern, serves no response of the UI's own without
// them. The checks are built once per process from the key the configuration names, through Get.
package server

import (
	"context"
	"crypto/rand"
	"fmt"
	"log/slog"
	"net/http"
	"os"

	"github.com/ppat/mediated-mailbox-mcp/core/config"
	"github.com/ppat/mediated-mailbox-mcp/process"
)

// MinTokenKey is the fewest bytes the key behind the request token may hold.
const MinTokenKey = 32

// Checks are the checks every endpoint of the UI sits behind, and the key they and the consent
// attempt's seal are derived from (ADR-0061, ADR-0111).
type Checks struct {
	keys   keys
	logger *slog.Logger
}

// held is the slot the process keeps the UI's checks in.
var held = process.NewOnePerProcess[*Checks]("checks")

// Get returns the UI's checks, built the first time an endpoint asks for them from the key
// token_key_file names, or a key generated at the start when it names none, and kept in the process,
// so every endpoint checks a request token against the one key.
func Get(ctx context.Context, p process.Capabilities, c config.UI) (*Checks, error) {
	return held.Get(ctx, p, func(_ context.Context, shared process.Capabilities) (*Checks, func(), error) {
		key, err := loadKey(c.TokenKeyFile)
		if err != nil {
			return nil, nil, err
		}
		checks, err := New(key, shared.Logger())
		return checks, nil, err
	})
}

// New returns the checks over key, logging each request's line through logger. Get builds the
// process's checks through it, and the read API endpoint's tests build theirs.
func New(key []byte, logger *slog.Logger) (*Checks, error) {
	k, err := newKeys(key)
	if err != nil {
		return nil, err
	}
	return &Checks{keys: k, logger: logger}, nil
}

// Wrap puts h behind every check, in the order a request meets them. The request is given its id and
// its log line, the policy is set before anything is written, the request is given its session, and a
// request whose method is not GET or HEAD is refused unless it carries the session's request token,
// all before h sees it.
func (c *Checks) Wrap(h http.Handler) http.Handler {
	return observe(c.logger, withPolicy(withSession(withToken(c.keys, h))))
}

// loadKey reads the key behind the request token from its mounted file, or generates one when no file
// is named, in which case a page from before a restart gets stale_page and an attempt from before it
// is lost (ADR-0061, ADR-0111).
func loadKey(path string) ([]byte, error) {
	if path == "" {
		key := make([]byte, MinTokenKey)
		if _, err := rand.Read(key); err != nil {
			return nil, fmt.Errorf("generating the request token's key: %w", err)
		}
		return key, nil
	}
	key, err := os.ReadFile(path) //nolint:gosec // the path is the mounted key file the configuration names
	if err != nil {
		return nil, fmt.Errorf("reading the request token's key file: %w", err)
	}
	if len(key) < MinTokenKey {
		return nil, fmt.Errorf("the request token's key file holds %d bytes, and it needs at least %d", len(key), MinTokenKey)
	}
	return key, nil
}
